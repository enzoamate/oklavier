package agent

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type SessionReadiness struct {
	Phase      string `json:"phase"`       // creating, starting, vnc_waiting, ready, error
	Progress   int    `json:"progress"`    // 0-100
	Message    string `json:"message"`
	PodIP      string `json:"pod_ip"`
	PodReady   bool   `json:"pod_ready"`
	VNCReady   bool   `json:"vnc_ready"`
}

func (a *Agent) CheckReadiness(podName string, podIP string) *SessionReadiness {
	ctx := context.Background()

	// Phase 1: Check if pod exists
	pod, err := a.client.CoreV1().Pods(a.namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return &SessionReadiness{Phase: "creating", Progress: 10, Message: "Création du pod..."}
	}

	// Phase 2: Check pod phase
	switch pod.Status.Phase {
	case "Pending":
		// Check if image is pulling
		for _, cs := range pod.Status.ContainerStatuses {
			if cs.State.Waiting != nil {
				reason := cs.State.Waiting.Reason
				if reason == "ContainerCreating" || reason == "PodInitializing" {
					return &SessionReadiness{Phase: "starting", Progress: 25, Message: "Téléchargement de l'image..."}
				}
				if reason == "ErrImagePull" || reason == "ImagePullBackOff" {
					return &SessionReadiness{Phase: "error", Progress: 0, Message: "Erreur: image introuvable"}
				}
			}
		}
		return &SessionReadiness{Phase: "starting", Progress: 20, Message: "En attente de ressources..."}

	case "Running":
		// Check container ready
		allReady := true
		for _, cs := range pod.Status.ContainerStatuses {
			if !cs.Ready {
				allReady = false
			}
		}
		if !allReady {
			return &SessionReadiness{Phase: "starting", Progress: 40, Message: "Démarrage de l'application...", PodIP: pod.Status.PodIP}
		}

		// Phase 3: Check VNC readiness
		ip := pod.Status.PodIP
		if ip == "" {
			ip = podIP
		}
		if ip != "" {
			vncReady := checkVNC(ip)
			if vncReady {
				return &SessionReadiness{
					Phase: "ready", Progress: 100, Message: "Prêt !",
					PodIP: ip, PodReady: true, VNCReady: true,
				}
			}
			return &SessionReadiness{
				Phase: "vnc_waiting", Progress: 70, Message: "Connexion au bureau virtuel...",
				PodIP: ip, PodReady: true, VNCReady: false,
			}
		}
		return &SessionReadiness{Phase: "starting", Progress: 50, Message: "En attente de l'adresse réseau...", PodReady: true}

	case "Failed", "Unknown":
		return &SessionReadiness{Phase: "error", Progress: 0, Message: "Le pod a échoué"}
	}

	return &SessionReadiness{Phase: "creating", Progress: 5, Message: "Initialisation..."}
}

func checkVNC(podIP string) bool {
	client := &http.Client{Timeout: 2 * time.Second}

	resp, err := client.Get(fmt.Sprintf("https://%s:6901/", podIP))
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	io.ReadAll(resp.Body)

	if resp.StatusCode == 200 || resp.StatusCode == 401 {
		log.Printf("VNC ready on %s (status %d)", podIP, resp.StatusCode)
		return true
	}
	return false
}

// WaitForReady polls until the session is ready or timeout
func (a *Agent) WaitForReady(podName string, podIP string, timeout time.Duration) *SessionReadiness {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		status := a.CheckReadiness(podName, podIP)
		if status.Phase == "ready" || status.Phase == "error" {
			return status
		}
		time.Sleep(500 * time.Millisecond)
	}
	return &SessionReadiness{Phase: "error", Progress: 0, Message: "Timeout"}
}
