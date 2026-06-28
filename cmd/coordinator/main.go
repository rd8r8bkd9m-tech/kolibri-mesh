// Kolibri Mesh Coordinator
// Manages the mesh network and coordinates all nodes
package main

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
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
	mux.HandleFunc("/v1/", handleControlPlaneProxy)

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

func controlPlaneURL() string {
	target := strings.TrimSpace(os.Getenv("KOLIBRI_FACTORY_CONTROL_URL"))
	if target == "" {
		target = "http://10.99.0.2:9101"
	}
	return strings.TrimRight(target, "/")
}

func allowedControlCIDRs() []*net.IPNet {
	raw := strings.TrimSpace(os.Getenv("KOLIBRI_MESH_CONTROL_ALLOWED_CIDRS"))
	if raw == "" {
		raw = "127.0.0.0/8,10.99.0.0/24"
	}
	var networks []*net.IPNet
	for _, item := range strings.Split(raw, ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		_, network, err := net.ParseCIDR(item)
		if err == nil {
			networks = append(networks, network)
			continue
		}
		if ip := net.ParseIP(item); ip != nil {
			bits := 128
			if ip.To4() != nil {
				bits = 32
			}
			network = &net.IPNet{IP: ip, Mask: net.CIDRMask(bits, bits)}
			networks = append(networks, network)
		}
	}
	return networks
}

func sourceAllowed(remoteAddr string) bool {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		host = remoteAddr
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	for _, network := range allowedControlCIDRs() {
		if network.Contains(ip) {
			return true
		}
	}
	return false
}

func controlPathAllowed(method, path string) bool {
	if method == http.MethodGet && (path == "/v1/health" || path == "/v1/filesystem" || path == "/v1/nodes") {
		return true
	}
	if method == http.MethodPost && (path == "/v1/nodes/register" || path == "/v1/tasks/lease" || path == "/v1/tasks") {
		return true
	}
	if method == http.MethodPost && strings.HasPrefix(path, "/v1/nodes/") && strings.HasSuffix(path, "/heartbeat") {
		return true
	}
	if method == http.MethodPost && strings.HasPrefix(path, "/v1/tasks/") {
		return strings.HasSuffix(path, "/heartbeat") || strings.HasSuffix(path, "/complete") || strings.HasSuffix(path, "/fail")
	}
	return false
}

func handleControlPlaneProxy(w http.ResponseWriter, r *http.Request) {
	if !sourceAllowed(r.RemoteAddr) {
		http.Error(w, "source not allowed", http.StatusForbidden)
		return
	}
	if !controlPathAllowed(r.Method, r.URL.Path) {
		http.NotFound(w, r)
		return
	}
	target := controlPlaneURL() + r.URL.Path
	if r.URL.RawQuery != "" {
		target += "?" + r.URL.RawQuery
	}
	payload, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	req, err := http.NewRequestWithContext(r.Context(), r.Method, target, bytes.NewReader(payload))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	req.Header.Set("Content-Type", r.Header.Get("Content-Type"))
	req.ContentLength = int64(len(payload))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	w.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
	w.WriteHeader(resp.StatusCode)
	if _, err := io.Copy(w, resp.Body); err != nil {
		log.Printf("control proxy write failed: %v", err)
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
