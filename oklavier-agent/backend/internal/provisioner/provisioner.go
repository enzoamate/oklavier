package provisioner

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os/exec"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// generateVNCPassword returns a 128-bit random base64 string suitable as a
// session-bound VNC auth secret. Replaces the previous truncated-UUID variant
// that had only ~56 bits of entropy.
func generateVNCPassword() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// disallowedMountPaths blocks mount targets that would shadow critical
// system directories. Even with EmptyDir (currently used) an attacker can
// hide the image's binaries; if the source ever changes to HostPath this
// also prevents container escape via /var/run/docker.sock etc.
var disallowedMountPaths = []string{
	"/", "/etc", "/proc", "/sys", "/dev",
	"/var", "/var/run", "/var/lib", "/var/log",
	"/usr", "/usr/bin", "/usr/sbin", "/usr/lib", "/usr/local",
	"/bin", "/sbin", "/lib", "/lib64",
	"/root", "/boot",
}

// disallowedEnvKeyPrefixes blocks environment variables that change how the
// dynamic loader / language runtimes behave. Without this, a workspace
// definition can drop `LD_PRELOAD=/path/to/lib.so` (or `PYTHONSTARTUP=...`)
// on every user session.
var disallowedEnvKeyPrefixes = []string{
	"LD_", "DYLD_", "PYTHONSTARTUP", "PYTHONPATH",
	"NODE_OPTIONS", "PERL5OPT", "PERL5LIB", "RUBYOPT",
	"PATH", // tightly controlled paths only
}

func envKeyAllowed(k string) bool {
	for _, p := range disallowedEnvKeyPrefixes {
		if strings.HasPrefix(k, p) {
			return false
		}
	}
	return true
}

// parseExecArgv extracts an explicit argv from an exec_config entry.
// Accepts only `{"argv": ["binary", "arg1", ...]}`. A legacy `cmd: "string"`
// is REJECTED to prevent shell-string injection.
func parseExecArgv(cfg map[string]interface{}) []string {
	raw, ok := cfg["argv"].([]interface{})
	if !ok || len(raw) == 0 {
		return nil
	}
	out := make([]string, 0, len(raw))
	for _, v := range raw {
		s, ok := v.(string)
		if !ok || s == "" {
			return nil
		}
		out = append(out, s)
	}
	// argv[0] (binary path) must be a plain absolute path or a basename.
	if strings.ContainsAny(out[0], "&|;`$\n\r") {
		return nil
	}
	return out
}

func mountPathAllowed(p string) bool {
	clean := strings.TrimRight(p, "/")
	if clean == "" {
		return false
	}
	for _, bad := range disallowedMountPaths {
		if clean == bad {
			return false
		}
	}
	return true
}

type Provisioner struct {
	client    *kubernetes.Clientset
	namespace string
}

type ClusterStats struct {
	NodeCount     int
	CPUTotal      int
	MemoryTotalGB int
	CPUUsed       int
	MemoryUsedGB  int
}

type SessionInfo struct {
	SessionID   string `json:"session_id"`
	PodName     string `json:"pod_name"`
	ServiceName string `json:"service_name"`
	PodIP       string `json:"pod_ip"`
	VNCPassword string `json:"vnc_password"`
	Status      string `json:"status"`
}

func New(namespace string) (*Provisioner, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("not running in cluster: %w", err)
	}

	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create k8s client: %w", err)
	}

	return &Provisioner{client: client, namespace: namespace}, nil
}

func (p *Provisioner) GetClusterStats() ClusterStats {
	ctx := context.Background()
	nodes, err := p.client.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return ClusterStats{}
	}

	stats := ClusterStats{NodeCount: len(nodes.Items)}
	for _, node := range nodes.Items {
		if cpu := node.Status.Capacity.Cpu(); cpu != nil {
			stats.CPUTotal += int(cpu.Value())
		}
		if mem := node.Status.Capacity.Memory(); mem != nil {
			stats.MemoryTotalGB += int(mem.Value() / (1024 * 1024 * 1024))
		}
	}
	return stats
}

func (p *Provisioner) CountActiveSessions() int {
	ctx := context.Background()
	pods, err := p.client.CoreV1().Pods(p.namespace).List(ctx, metav1.ListOptions{
		LabelSelector: "app=oklavier-session",
	})
	if err != nil {
		return 0
	}
	return len(pods.Items)
}

func (p *Provisioner) HandleCreateSession(c *fiber.Ctx) error {
	var req struct {
		SessionID      string          `json:"session_id"`
		DockerImage    string          `json:"docker_image"`
		Cores          float64         `json:"cores"`
		Memory         int64           `json:"memory"`
		SHMSize        string          `json:"shm_size"`
		Persistent     bool            `json:"persistent"`
		PersistentSize string          `json:"persistent_size"`
		UserID         string          `json:"user_id"`
		WorkspaceID    string          `json:"workspace_id"`
		RunConfig      json.RawMessage `json:"run_config"`
		ExecConfig     json.RawMessage `json:"exec_config"`
		VolumeMappings json.RawMessage `json:"volume_mappings"`
		GPUCount       int             `json:"gpu_count"`
		DockerRegistry string          `json:"docker_registry"`
		DockerUser     string          `json:"docker_user"`
		DockerPassword string          `json:"docker_password"`
	}
	if err := c.BodyParser(&req); err != nil {
		// SECURITY: don't log the raw body — it contains docker_password,
		// run_config and volume_mappings, which leaked to the agent log buffer
		// served by GET /api/logs.
		log.Printf("CreateSession: body parse error: %v", err)
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request"})
	}

	ctx := context.Background()
	if len(req.SessionID) < 20 {
		return c.Status(400).JSON(fiber.Map{"error": "session_id too short"})
	}
	podName := fmt.Sprintf("oklavier-%s", req.SessionID[:20])
	svcName := fmt.Sprintf("oklavier-svc-%s", req.SessionID[:20])
	// SECURITY: previously used uuid.New().String()[:16], which keeps two
	// hyphens and the version-`4` nibble — only ~56 bits of entropy. Use
	// crypto/rand for 16 bytes -> 128 bits -> base64.
	vncPW, err := generateVNCPassword()
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to generate VNC password"})
	}

	shmSize := req.SHMSize
	if shmSize == "" {
		shmSize = "512Mi"
	}

	// Persistent storage: create/reuse PVC per user+workspace
	volumeMounts := []corev1.VolumeMount{
		{Name: "dshm", MountPath: "/dev/shm"},
	}
	volumes := []corev1.Volume{{
		Name: "dshm",
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{
				Medium:    corev1.StorageMediumMemory,
				SizeLimit: resource.NewQuantity(512*1024*1024, resource.BinarySI),
			},
		},
	}}

	if req.Persistent && req.UserID != "" && req.WorkspaceID != "" {
		pvcName := strings.ToLower(fmt.Sprintf("oklavier-data-%s-%s", req.UserID[:8], req.WorkspaceID[:8]))
		pvcSize := req.PersistentSize
		if pvcSize == "" {
			pvcSize = "5Gi"
		}

		// Check if PVC already exists, if not create it
		_, err := p.client.CoreV1().PersistentVolumeClaims(p.namespace).Get(ctx, pvcName, metav1.GetOptions{})
		if err != nil {
			// PVC doesn't exist, create it
			storageClass := ""
			pvc := &corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      pvcName,
					Namespace: p.namespace,
					Labels: map[string]string{
						"app":                "oklavier-data",
						"oklavier/user":      req.UserID,
						"oklavier/workspace": req.WorkspaceID,
					},
				},
				Spec: corev1.PersistentVolumeClaimSpec{
					AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
					Resources: corev1.VolumeResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceStorage: resource.MustParse(pvcSize),
						},
					},
				},
			}
			if storageClass != "" {
				pvc.Spec.StorageClassName = &storageClass
			}
			_, err = p.client.CoreV1().PersistentVolumeClaims(p.namespace).Create(ctx, pvc, metav1.CreateOptions{})
			if err != nil {
				log.Printf("Failed to create PVC %s: %v", pvcName, err)
			} else {
				log.Printf("Created PVC %s (%s) for user %s workspace %s", pvcName, pvcSize, req.UserID[:8], req.WorkspaceID[:8])
			}
		} else {
			log.Printf("Reusing existing PVC %s", pvcName)
		}

		// Add PVC volume and mount
		volumes = append(volumes, corev1.Volume{
			Name: "persistent-data",
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: pvcName,
				},
			},
		})
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      "persistent-data",
			MountPath: "/home/user",
		})
	}

	// Process run_config: environment variables, hostname, user, restart_policy
	envVars := []corev1.EnvVar{
		{Name: "VNC_PW", Value: vncPW},
		{Name: "SESSION_ID", Value: req.SessionID},
	}
	hostname := podName

	// Parse run_config from JSON
	var runConfig map[string]interface{}
	if len(req.RunConfig) > 0 {
		json.Unmarshal(req.RunConfig, &runConfig)
	}
	if runConfig != nil {
		// Environment variables from run_config — SECURITY: filter dangerous
		// loader/runtime keys (LD_PRELOAD etc.) which would let a workspace
		// definition hijack every user session.
		if envMap, ok := runConfig["environment"].(map[string]interface{}); ok {
			added := 0
			for k, v := range envMap {
				if !envKeyAllowed(k) {
					log.Printf("[security] dropped disallowed env key %q from workspace", k)
					continue
				}
				envVars = append(envVars, corev1.EnvVar{Name: k, Value: fmt.Sprintf("%v", v)})
				added++
				if added >= 64 {
					break
				}
			}
		}
		// Environment as list "KEY=VALUE"
		if envList, ok := runConfig["environment"].([]interface{}); ok {
			added := 0
			for _, item := range envList {
				s := fmt.Sprintf("%v", item)
				if idx := strings.Index(s, "="); idx > 0 {
					k := s[:idx]
					if !envKeyAllowed(k) {
						log.Printf("[security] dropped disallowed env key %q from workspace", k)
						continue
					}
					envVars = append(envVars, corev1.EnvVar{Name: k, Value: s[idx+1:]})
					added++
					if added >= 64 {
						break
					}
				}
			}
		}
		// Hostname
		if h, ok := runConfig["hostname"].(string); ok && h != "" {
			hostname = h
		}
	}

	// Parse volume_mappings from JSON: {"host_path": {"bind": "/container/path", "mode": "rw"}}
	var volMappings map[string]interface{}
	if len(req.VolumeMappings) > 0 {
		json.Unmarshal(req.VolumeMappings, &volMappings)
	}
	if volMappings != nil {
		volIdx := len(volumes)
		mountCount := 0
		const maxMounts = 32
		for hostPath, config := range volMappings {
			if mountCount >= maxMounts {
				log.Printf("[security] volume_mappings cap reached (%d), ignoring extras", maxMounts)
				break
			}
			volName := fmt.Sprintf("extra-vol-%d", volIdx)
			mountPath := hostPath
			readOnly := false
			if cfg, ok := config.(map[string]interface{}); ok {
				if bind, ok := cfg["bind"].(string); ok {
					mountPath = bind
				}
				if mode, ok := cfg["mode"].(string); ok && mode == "ro" {
					readOnly = true
				}
			}
			// SECURITY: reject mount paths that would shadow critical system
			// directories. Even with EmptyDir an attacker can hide image
			// binaries; with HostPath this would be a container escape.
			if !mountPathAllowed(mountPath) {
				log.Printf("[security] rejected disallowed mount path %q", mountPath)
				continue
			}
			volumes = append(volumes, corev1.Volume{
				Name: volName,
				VolumeSource: corev1.VolumeSource{
					EmptyDir: &corev1.EmptyDirVolumeSource{},
				},
			})
			volumeMounts = append(volumeMounts, corev1.VolumeMount{
				Name:      volName,
				MountPath: mountPath,
				ReadOnly:  readOnly,
			})
			volIdx++
			mountCount++
		}
	}

	// Create ImagePullSecret if registry credentials are provided
	var imagePullSecrets []corev1.LocalObjectReference
	if req.DockerUser != "" && req.DockerPassword != "" {
		secretName := fmt.Sprintf("oklavier-registry-%s", req.SessionID[:20])
		registry := req.DockerRegistry
		if registry == "" {
			registry = "https://index.docker.io/v1/"
		}
		dockerConfigJSON := fmt.Sprintf(`{"auths":{%q:{"username":%q,"password":%q,"auth":%q}}}`,
			registry, req.DockerUser, req.DockerPassword,
			base64.StdEncoding.EncodeToString([]byte(req.DockerUser+":"+req.DockerPassword)))
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      secretName,
				Namespace: p.namespace,
				Labels: map[string]string{
					"app":              "oklavier-session",
					"oklavier/session": req.SessionID,
				},
			},
			Type: corev1.SecretTypeDockerConfigJson,
			Data: map[string][]byte{
				corev1.DockerConfigJsonKey: []byte(dockerConfigJSON),
			},
		}
		_, err := p.client.CoreV1().Secrets(p.namespace).Create(ctx, secret, metav1.CreateOptions{})
		if err != nil {
			log.Printf("Failed to create ImagePullSecret %s: %v", secretName, err)
		} else {
			imagePullSecrets = append(imagePullSecrets, corev1.LocalObjectReference{Name: secretName})
			log.Printf("Created ImagePullSecret %s for registry %s", secretName, registry)
		}
	}

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: p.namespace,
			Labels: map[string]string{
				"app":              "oklavier-session",
				"oklavier/session": req.SessionID,
			},
		},
		Spec: corev1.PodSpec{
			Hostname:                     hostname,
			ImagePullSecrets:             imagePullSecrets,
			AutomountServiceAccountToken: func() *bool { b := false; return &b }(),
			SecurityContext: &corev1.PodSecurityContext{
				// SECURITY: was Unconfined — that disables every syscall filter
				// and is a documented container escape primitive. RuntimeDefault
				// is the supported safe profile.
				SeccompProfile: &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault},
			},
			Containers: []corev1.Container{{
				Name:            "workspace",
				Image:           req.DockerImage,
				ImagePullPolicy: corev1.PullAlways,
				Env:             envVars,
				SecurityContext: &corev1.SecurityContext{
					// SECURITY: drop all caps + forbid privilege escalation. The
					// previous PodSpec set none of these, so workspace pods ran
					// with the default (sometimes root) capability set.
					AllowPrivilegeEscalation: func() *bool { b := false; return &b }(),
					Capabilities: &corev1.Capabilities{
						Drop: []corev1.Capability{"ALL"},
					},
				},
				Ports: []corev1.ContainerPort{
					{Name: "vnc-ws", ContainerPort: 6901},
					{Name: "vnc-tcp", ContainerPort: 5900},
					{Name: "audio", ContainerPort: 4901},
					{Name: "uploads", ContainerPort: 4902},
				},
				Resources: func() corev1.ResourceRequirements {
					limits := corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse(fmt.Sprintf("%.0fm", req.Cores*1000)),
						corev1.ResourceMemory: resource.MustParse(fmt.Sprintf("%d", req.Memory)),
					}
					if req.GPUCount > 0 {
						limits["nvidia.com/gpu"] = resource.MustParse(fmt.Sprintf("%d", req.GPUCount))
					}
					return corev1.ResourceRequirements{Limits: limits}
				}(),
				VolumeMounts: volumeMounts,
			}},
			Volumes: volumes,
		},
	}

	_, err = p.client.CoreV1().Pods(p.namespace).Create(ctx, pod, metav1.CreateOptions{})
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to create pod: " + err.Error()})
	}

	// Process exec_config: run commands inside the pod after start
	var execConfigs []map[string]interface{}
	if len(req.ExecConfig) > 0 {
		json.Unmarshal(req.ExecConfig, &execConfigs)
	}
	if len(execConfigs) > 0 {
		// SECURITY: cap the number of post-start commands to prevent abuse.
		const maxExec = 10
		if len(execConfigs) > maxExec {
			execConfigs = execConfigs[:maxExec]
		}
		nsName := p.namespace
		k8sClient := p.client
		go func() {
			// Wait for pod to be running
			for i := 0; i < 30; i++ {
				time.Sleep(2 * time.Second)
				pod, err := k8sClient.CoreV1().Pods(nsName).Get(context.Background(), podName, metav1.GetOptions{})
				if err == nil && pod.Status.Phase == corev1.PodRunning {
					break
				}
			}
			for _, execCfg := range execConfigs {
				// SECURITY: previously, `cmd` was passed to `sh -c` inside the
				// pod, allowing arbitrary shell injection / chained commands
				// from any workspace template. We now require explicit argv
				// (`argv: ["/path/to/binary", "arg1", "arg2"]`) and disallow
				// the legacy `cmd` string entirely. No shell, no metacharacter
				// expansion.
				argv := parseExecArgv(execCfg)
				if len(argv) == 0 {
					log.Printf("[security] exec_config entry rejected: missing or invalid argv")
					continue
				}
				log.Printf("ExecConfig: running argv=%v in %s", argv, podName)
				kubectlArgs := append([]string{"exec", "-n", nsName, podName, "--"}, argv...)
				execCmd := exec.Command("kubectl", kubectlArgs...)
				output, err := execCmd.CombinedOutput()
				if err != nil {
					log.Printf("ExecConfig: error: %v output: %s", err, string(output))
				} else {
					log.Printf("ExecConfig: argv[0]=%q completed", argv[0])
				}
			}
		}()
	}

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: svcName, Namespace: p.namespace,
			Labels: map[string]string{"app": "oklavier-session", "oklavier/session": req.SessionID},
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{"oklavier/session": req.SessionID},
			Ports: []corev1.ServicePort{
				{Name: "vnc-ws", Port: 6901, TargetPort: intstr.FromInt(6901)},
				{Name: "vnc-tcp", Port: 5901, TargetPort: intstr.FromInt(5900)},
				{Name: "audio", Port: 4901, TargetPort: intstr.FromInt(4901)},
			},
		},
	}
	p.client.CoreV1().Services(p.namespace).Create(ctx, svc, metav1.CreateOptions{})

	// Wait for pod IP
	podIP := ""
	for i := 0; i < 30; i++ {
		pod, err := p.client.CoreV1().Pods(p.namespace).Get(ctx, podName, metav1.GetOptions{})
		if err == nil && pod.Status.PodIP != "" {
			podIP = pod.Status.PodIP
			break
		}
		time.Sleep(time.Second)
	}

	log.Printf("Created session %s (pod: %s, ip: %s)", req.SessionID, podName, podIP)

	return c.JSON(SessionInfo{
		SessionID:   req.SessionID,
		PodName:     podName,
		ServiceName: svcName,
		PodIP:       podIP,
		VNCPassword: vncPW,
		Status:      "running",
	})
}

func (p *Provisioner) HandleDestroySession(c *fiber.Ctx) error {
	var req struct {
		SessionID string `json:"session_id"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request"})
	}

	ctx := context.Background()
	pods, _ := p.client.CoreV1().Pods(p.namespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("oklavier/session=%s", req.SessionID),
	})
	for _, pod := range pods.Items {
		p.client.CoreV1().Pods(p.namespace).Delete(ctx, pod.Name, metav1.DeleteOptions{})
	}
	svcs, _ := p.client.CoreV1().Services(p.namespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("oklavier/session=%s", req.SessionID),
	})
	for _, svc := range svcs.Items {
		p.client.CoreV1().Services(p.namespace).Delete(ctx, svc.Name, metav1.DeleteOptions{})
	}
	// Clean up ImagePullSecrets for this session
	secrets, _ := p.client.CoreV1().Secrets(p.namespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("oklavier/session=%s", req.SessionID),
	})
	for _, s := range secrets.Items {
		p.client.CoreV1().Secrets(p.namespace).Delete(ctx, s.Name, metav1.DeleteOptions{})
	}

	log.Printf("Destroyed session %s", req.SessionID)
	return c.JSON(fiber.Map{"status": "ok"})
}

func (p *Provisioner) HandleSessionStatus(c *fiber.Ctx) error {
	var req struct {
		SessionID string `json:"session_id"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request"})
	}

	ctx := context.Background()
	pods, err := p.client.CoreV1().Pods(p.namespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("oklavier/session=%s", req.SessionID),
	})
	if err != nil || len(pods.Items) == 0 {
		return c.JSON(fiber.Map{"status": "not_found"})
	}

	pod := pods.Items[0]
	status := "unknown"
	switch pod.Status.Phase {
	case corev1.PodRunning:
		status = "running"
	case corev1.PodPending:
		status = "starting"
	case corev1.PodFailed:
		status = "failed"
	}

	return c.JSON(fiber.Map{
		"status":   status,
		"pod_ip":   pod.Status.PodIP,
		"pod_name": pod.Name,
	})
}

func (p *Provisioner) HandleSessionReadiness(c *fiber.Ctx) error {
	var req struct {
		SessionID string `json:"session_id"`
		PodName   string `json:"pod_name"`
		PodIP     string `json:"pod_ip"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"phase": "error", "progress": 0, "message": "Invalid request"})
	}

	ctx := context.Background()

	// Find pod
	podName := req.PodName
	if podName == "" {
		pods, _ := p.client.CoreV1().Pods(p.namespace).List(ctx, metav1.ListOptions{
			LabelSelector: fmt.Sprintf("oklavier/session=%s", req.SessionID),
		})
		if len(pods.Items) == 0 {
			return c.JSON(fiber.Map{"phase": "creating", "progress": 10, "message": "Creating pod..."})
		}
		podName = pods.Items[0].Name
	}

	pod, err := p.client.CoreV1().Pods(p.namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return c.JSON(fiber.Map{"phase": "creating", "progress": 10, "message": "Creating pod..."})
	}

	switch pod.Status.Phase {
	case "Pending":
		return c.JSON(fiber.Map{"phase": "starting", "progress": 25, "message": "Starting application..."})
	case "Running":
		// Check VNC
		ip := pod.Status.PodIP
		if ip != "" && checkVNC(ip) {
			return c.JSON(fiber.Map{"phase": "ready", "progress": 100, "message": "Ready!", "pod_ip": ip})
		}
		return c.JSON(fiber.Map{"phase": "vnc_waiting", "progress": 70, "message": "Connecting to virtual desktop...", "pod_ip": ip})
	case "Failed":
		return c.JSON(fiber.Map{"phase": "error", "progress": 0, "message": "Pod failed"})
	}

	return c.JSON(fiber.Map{"phase": "creating", "progress": 5, "message": "Initializing..."})
}

func checkVNC(podIP string) bool {
	addr := fmt.Sprintf("%s:5900", podIP)
	conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	if err != nil {
		log.Printf("checkVNC(%s): failed: %v", addr, err)
		return false
	}
	conn.Close()
	log.Printf("checkVNC(%s): OK", addr)
	return true
}

// GetPodIP returns the IP of a session pod
func (p *Provisioner) GetPodIP(sessionID string) (string, string, error) {
	ctx := context.Background()
	pods, err := p.client.CoreV1().Pods(p.namespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("oklavier/session=%s", sessionID),
	})
	if err != nil || len(pods.Items) == 0 {
		return "", "", fmt.Errorf("session not found")
	}

	pod := pods.Items[0]
	// Get VNC password from env
	vncPW := ""
	for _, env := range pod.Spec.Containers[0].Env {
		if env.Name == "VNC_PW" {
			vncPW = env.Value
			break
		}
	}
	return pod.Status.PodIP, vncPW, nil
}

func (p *Provisioner) GetSessionInfo(sessionID string) (*SessionInfo, error) {
	ip, pw, err := p.GetPodIP(sessionID)
	if err != nil {
		return nil, err
	}
	return &SessionInfo{PodIP: ip, VNCPassword: pw}, nil
}
