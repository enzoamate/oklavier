package handlers

import (
	"context"
	"log"

	"github.com/gofiber/fiber/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"oklavier-api/internal/db"
)

type ClusterHandler struct {
	DB *db.DB
}

// RegisterLocalCluster auto-detects and registers the current cluster
func (h *ClusterHandler) RegisterLocalCluster() {
	// Try in-cluster config
	config, err := rest.InClusterConfig()
	if err != nil {
		log.Printf("Not running in K8s cluster, skipping local cluster registration")
		return
	}

	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return
	}

	ctx := context.Background()
	nodes, err := client.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return
	}

	nodeCount := len(nodes.Items)
	totalCPU := 0
	totalMemoryGB := 0
	clusterName := "local"

	for _, node := range nodes.Items {
		if clusterName == "local" {
			// Use first node name as hint
			clusterName = node.Name
		}
		cpu := node.Status.Capacity.Cpu()
		mem := node.Status.Capacity.Memory()
		if cpu != nil {
			totalCPU += int(cpu.Value())
		}
		if mem != nil {
			totalMemoryGB += int(mem.Value() / (1024 * 1024 * 1024))
		}
	}

	// Upsert local cluster in DB
	_, err = h.DB.Exec(`
		INSERT INTO cluster (id, name, description, kubeconfig, namespace, is_default, enabled, status, node_count, cpu_total, memory_total_gb, last_check)
		VALUES ('local', $1, 'Cluster local (auto-détecté)', 'in-cluster', $2, true, true, 'connected', $3, $4, $5, NOW())
		ON CONFLICT (id) DO UPDATE SET status='connected', node_count=$3, cpu_total=$4, memory_total_gb=$5, last_check=NOW()
	`, clusterName, "oklavier", nodeCount, totalCPU, totalMemoryGB)

	if err != nil {
		log.Printf("Failed to register local cluster: %v", err)
		return
	}

	log.Printf("Local cluster registered: %s (%d nodes, %d vCPU, %d GB RAM)", clusterName, nodeCount, totalCPU, totalMemoryGB)
}

func (h *ClusterHandler) TestCluster(c *fiber.Ctx) error {
	var req struct {
		Kubeconfig string `json:"kubeconfig"`
		Namespace  string `json:"namespace"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"connected": false, "error": "Invalid body"})
	}

	// Parse kubeconfig from string
	config, err := clientcmd.RESTConfigFromKubeConfig([]byte(req.Kubeconfig))
	if err != nil {
		log.Printf("Cluster test: invalid kubeconfig: %v", err)
		return c.JSON(fiber.Map{"connected": false, "error": "Invalid kubeconfig: " + err.Error()})
	}

	config.Timeout = 10e9 // 10s

	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return c.JSON(fiber.Map{"connected": false, "error": "Failed to create client: " + err.Error()})
	}

	ctx := context.Background()

	// Test connection by listing nodes
	nodes, err := client.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		log.Printf("Cluster test: connection failed: %v", err)
		return c.JSON(fiber.Map{"connected": false, "error": "Connection failed: " + err.Error()})
	}

	// Calculate total resources
	nodeCount := len(nodes.Items)
	totalCPU := 0
	totalMemoryGB := 0
	for _, node := range nodes.Items {
		cpu := node.Status.Capacity.Cpu()
		mem := node.Status.Capacity.Memory()
		if cpu != nil {
			totalCPU += int(cpu.Value())
		}
		if mem != nil {
			totalMemoryGB += int(mem.Value() / (1024 * 1024 * 1024))
		}
	}

	// Test namespace access
	_, err = client.CoreV1().Pods(req.Namespace).List(ctx, metav1.ListOptions{Limit: 1})
	nsAccess := err == nil

	log.Printf("Cluster test: connected! %d nodes, %d vCPU, %d GB RAM, ns=%s accessible=%v",
		nodeCount, totalCPU, totalMemoryGB, req.Namespace, nsAccess)

	return c.JSON(fiber.Map{
		"connected":       true,
		"node_count":      nodeCount,
		"cpu_total":       totalCPU,
		"memory_total_gb": totalMemoryGB,
		"namespace_ok":    nsAccess,
	})
}
