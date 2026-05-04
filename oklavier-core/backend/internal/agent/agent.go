package agent

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type Agent struct {
	client    *kubernetes.Clientset
	namespace string
}

type SessionInfo struct {
	SessionID   string `json:"session_id"`
	PodName     string `json:"pod_name"`
	ServiceName string `json:"service_name"`
	PodIP       string `json:"pod_ip"`
	Status      string `json:"status"`
	VNCPort     int    `json:"vnc_port"`
	VNCPW       string `json:"vnc_pw"`
}

func New(kubeconfig string, namespace string) (*Agent, error) {
	var config *rest.Config
	var err error

	if kubeconfig != "" {
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
	} else {
		config, err = rest.InClusterConfig()
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get k8s config: %w", err)
	}

	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create k8s client: %w", err)
	}

	log.Printf("K8s agent connected to cluster, namespace: %s", namespace)
	return &Agent{client: client, namespace: namespace}, nil
}

func (a *Agent) CreateSession(imageID, imageName string, cores float64, memory int64) (*SessionInfo, error) {
	ctx := context.Background()
	sessionID := uuid.New().String()
	shortID := sessionID[:20]
	podName := fmt.Sprintf("oklavier-%s", shortID)
	svcName := fmt.Sprintf("oklavier-svc-%s", shortID)

	// VNC password
	vncPW := uuid.New().String()[:16]

	// Pod spec
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: a.namespace,
			Labels: map[string]string{
				"app":            "oklavier-session",
				"oklavier/id":    sessionID,
				"oklavier/image": imageID,
			},
		},
		Spec: corev1.PodSpec{
			SecurityContext: &corev1.PodSecurityContext{
				SeccompProfile: &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeUnconfined},
			},
			Containers: []corev1.Container{
				{
					Name:  "workspace",
					Image: imageName,
					Env: []corev1.EnvVar{
						{Name: "VNC_PW", Value: vncPW},
						{Name: "SESSION_ID", Value: sessionID},
					},
					Ports: []corev1.ContainerPort{
						{Name: "vnc", ContainerPort: 6901, Protocol: corev1.ProtocolTCP},
						{Name: "audio", ContainerPort: 4901, Protocol: corev1.ProtocolTCP},
						{Name: "uploads", ContainerPort: 4902, Protocol: corev1.ProtocolTCP},
					},
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse(fmt.Sprintf("%.0fm", cores*500)),
							corev1.ResourceMemory: resource.MustParse(fmt.Sprintf("%dMi", memory/1048576/2)),
						},
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse(fmt.Sprintf("%.0fm", cores*1000)),
							corev1.ResourceMemory: resource.MustParse(fmt.Sprintf("%dMi", memory/1048576)),
						},
					},
					VolumeMounts: []corev1.VolumeMount{
						{Name: "dshm", MountPath: "/dev/shm"},
					},
				},
			},
			Volumes: []corev1.Volume{
				{
					Name: "dshm",
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{
							Medium:    corev1.StorageMediumMemory,
							SizeLimit: resource.NewQuantity(512*1024*1024, resource.BinarySI),
						},
					},
				},
			},
		},
	}

	// Create pod
	createdPod, err := a.client.CoreV1().Pods(a.namespace).Create(ctx, pod, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to create pod: %w", err)
	}
	log.Printf("Created pod %s for session %s", podName, sessionID)

	// Create service
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      svcName,
			Namespace: a.namespace,
			Labels: map[string]string{
				"app":         "oklavier-session",
				"oklavier/id": sessionID,
			},
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{"oklavier/id": sessionID},
			Ports: []corev1.ServicePort{
				{Name: "vnc", Port: 6901, TargetPort: intstr.FromInt(6901)},
				{Name: "audio", Port: 4901, TargetPort: intstr.FromInt(4901)},
				{Name: "uploads", Port: 4902, TargetPort: intstr.FromInt(4902)},
			},
		},
	}

	_, err = a.client.CoreV1().Services(a.namespace).Create(ctx, svc, metav1.CreateOptions{})
	if err != nil {
		// Cleanup pod on failure
		a.client.CoreV1().Pods(a.namespace).Delete(ctx, podName, metav1.DeleteOptions{})
		return nil, fmt.Errorf("failed to create service: %w", err)
	}
	log.Printf("Created service %s for session %s", svcName, sessionID)

	// Wait for pod IP (up to 30s)
	podIP := ""
	for i := 0; i < 30; i++ {
		p, err := a.client.CoreV1().Pods(a.namespace).Get(ctx, podName, metav1.GetOptions{})
		if err == nil && p.Status.PodIP != "" {
			podIP = p.Status.PodIP
			break
		}
		time.Sleep(1 * time.Second)
	}

	if podIP == "" {
		podIP = createdPod.Name // fallback
	}

	return &SessionInfo{
		SessionID:   sessionID,
		PodName:     podName,
		ServiceName: svcName,
		PodIP:       podIP,
		Status:      "running",
		VNCPort:     6901,
		VNCPW:       vncPW,
	}, nil
}

func (a *Agent) DestroySession(sessionID string) error {
	ctx := context.Background()

	// Find and delete pod
	pods, err := a.client.CoreV1().Pods(a.namespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("oklavier/id=%s", sessionID),
	})
	if err == nil {
		for _, pod := range pods.Items {
			a.client.CoreV1().Pods(a.namespace).Delete(ctx, pod.Name, metav1.DeleteOptions{})
			log.Printf("Deleted pod %s", pod.Name)
		}
	}

	// Find and delete service
	svcs, err := a.client.CoreV1().Services(a.namespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("oklavier/id=%s", sessionID),
	})
	if err == nil {
		for _, svc := range svcs.Items {
			a.client.CoreV1().Services(a.namespace).Delete(ctx, svc.Name, metav1.DeleteOptions{})
			log.Printf("Deleted service %s", svc.Name)
		}
	}

	return nil
}

func (a *Agent) DestroySessionByPodName(podName string) error {
	ctx := context.Background()
	a.client.CoreV1().Pods(a.namespace).Delete(ctx, podName, metav1.DeleteOptions{})
	svcName := "oklavier-svc-" + podName[len("oklavier-"):]
	a.client.CoreV1().Services(a.namespace).Delete(ctx, svcName, metav1.DeleteOptions{})
	log.Printf("Deleted pod %s and service %s", podName, svcName)
	return nil
}

func (a *Agent) GetSessionStatus(sessionID string) (*SessionInfo, error) {
	ctx := context.Background()

	pods, err := a.client.CoreV1().Pods(a.namespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("oklavier/id=%s", sessionID),
	})
	if err != nil || len(pods.Items) == 0 {
		return nil, fmt.Errorf("session not found")
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

	return &SessionInfo{
		SessionID: sessionID,
		PodName:   pod.Name,
		PodIP:     pod.Status.PodIP,
		Status:    status,
		VNCPort:   6901,
	}, nil
}

func (a *Agent) ListSessions() ([]SessionInfo, error) {
	ctx := context.Background()

	pods, err := a.client.CoreV1().Pods(a.namespace).List(ctx, metav1.ListOptions{
		LabelSelector: "app=oklavier-session",
	})
	if err != nil {
		return nil, err
	}

	var sessions []SessionInfo
	for _, pod := range pods.Items {
		sessionID := pod.Labels["oklavier/id"]
		status := "unknown"
		switch pod.Status.Phase {
		case corev1.PodRunning:
			status = "running"
		case corev1.PodPending:
			status = "starting"
		case corev1.PodFailed:
			status = "failed"
		}
		sessions = append(sessions, SessionInfo{
			SessionID: sessionID,
			PodName:   pod.Name,
			PodIP:     pod.Status.PodIP,
			Status:    status,
			VNCPort:   6901,
		})
	}
	return sessions, nil
}
