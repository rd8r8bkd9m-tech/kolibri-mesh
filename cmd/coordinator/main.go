// Kolibri Mesh Coordinator
// Manages the mesh network and coordinates all nodes
package main

import (
	"encoding/json"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/kolibri/kolibri-mesh/pkg/awg"
	"github.com/kolibri/kolibri-mesh/pkg/mesh"
)

var (
	meshNetwork *mesh.Mesh
	coordinator *mesh.Coordinator
	awgConfig   *awg.Config
)

const coordinatorVersion = "0.2.0"

func main() {
	log.Println("Starting Kolibri Mesh Coordinator...")

	// Initialize AWG config
	awgConfig = awg.DefaultConfig()

	// Create local node
	localNode := &mesh.Node{
		ID:       "coordinator",
		Name:     "home",
		IP:       net.ParseIP("10.99.0.1"),
		Role:     "coordinator",
		Status:   "online",
		LastSeen: time.Now(),
	}

	// Create mesh network
	meshNetwork = mesh.NewMesh(localNode)

	// Create coordinator
	coordinator = mesh.NewCoordinator(meshNetwork, 30*time.Second)
	coordinator.Start()

	// Setup HTTP routes
	mux := http.NewServeMux()
	mux.HandleFunc("/api/status", handleStatus)
	mux.HandleFunc("/api/nodes", handleNodes)
	mux.HandleFunc("/api/heartbeat", handleHeartbeat)
	mux.HandleFunc("/api/config", handleConfig)
	mux.HandleFunc("/api/health", handleHealth)

	// Start server
	addr := ":8080"
	if port := os.Getenv("PORT"); port != "" {
		addr = ":" + port
	}

	log.Printf("Coordinator listening on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

// handleStatus returns mesh status
func handleStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(meshNetwork)
}

// handleNodes handles node operations
func handleNodes(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(meshNetwork.GetNodes())
	case http.MethodPost:
		var node mesh.Node
		if err := json.NewDecoder(r.Body).Decode(&node); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		node.Status = "online"
		node.LastSeen = time.Now()
		meshNetwork.AddNode(&node)
		log.Printf("Node registered: %s (%s)", node.Name, node.ID)
		w.WriteHeader(http.StatusCreated)
	case http.MethodPut:
		// Update node (heartbeat)
		var node mesh.Node
		if err := json.NewDecoder(r.Body).Decode(&node); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		meshNetwork.UpdateNodeStatus(node.ID, "online")
		w.WriteHeader(http.StatusOK)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleHeartbeat handles heartbeat from agents
func handleHeartbeat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var hb mesh.Heartbeat
	if err := json.NewDecoder(r.Body).Decode(&hb); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	response := coordinator.HandleHeartbeat(&hb)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleConfig returns AWG config for nodes
func handleConfig(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(awgConfig)
}

// handleHealth returns health status
func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":    "ok",
		"app":       "Kolibri Mesh Coordinator",
		"version":   coordinatorVersion,
		"timestamp": time.Now().Format(time.RFC3339),
	})
}
