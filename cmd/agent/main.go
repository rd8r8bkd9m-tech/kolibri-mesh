// Kolibri Mesh Agent
// Runs on each server and communicates with coordinator
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/kolibri/kolibri-mesh/pkg/awg"
	"github.com/kolibri/kolibri-mesh/pkg/mesh"
)

var (
	node        *mesh.Node
	agent       *mesh.Agent
	awgConfig   *awg.Config
	obfuscator  *awg.Obfuscator
)

func main() {
	log.Println("Starting Kolibri Mesh Agent...")

	// Load config from environment
	nodeID := os.Getenv("NODE_ID")
	nodeName := os.Getenv("NODE_NAME")
	coordinatorURL := os.Getenv("COORDINATOR_URL")
	nodeIP := os.Getenv("NODE_IP")

	if nodeID == "" || nodeName == "" || coordinatorURL == "" {
		log.Fatal("NODE_ID, NODE_NAME, and COORDINATOR_URL are required")
	}

	// Create node
	node = &mesh.Node{
		ID:       nodeID,
		Name:     nodeName,
		IP:       net.ParseIP(nodeIP),
		Role:     "agent",
		Status:   "online",
		LastSeen: time.Now(),
	}

	// Fetch AWG config from coordinator
	var err error
	awgConfig, err = fetchAWGConfig(coordinatorURL)
	if err != nil {
		log.Printf("Failed to fetch AWG config: %v, using default", err)
		awgConfig = awg.DefaultConfig()
	}

	// Create obfuscator
	obfuscator, err = awg.NewObfuscator(awgConfig)
	if err != nil {
		log.Fatalf("Failed to create obfuscator: %v", err)
	}

	// Create agent
	agent = mesh.NewAgent(node, coordinatorURL, 30*time.Second)
	agent.Start()

	// Register with coordinator
	if err := registerWithCoordinator(coordinatorURL); err != nil {
		log.Printf("Failed to register: %v", err)
	}

	// Start local services
	go startLocalServer()

	// Wait for shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	log.Println("Shutting down...")
	agent.Stop()
}

// fetchAWGConfig fetches AWG config from coordinator
func fetchAWGConfig(coordinatorURL string) (*awg.Config, error) {
	resp, err := http.Get(coordinatorURL + "/api/config")
	if err != nil {
		return nil, fmt.Errorf("failed to fetch config: %w", err)
	}
	defer resp.Body.Close()

	var config awg.Config
	if err := json.NewDecoder(resp.Body).Decode(&config); err != nil {
		return nil, fmt.Errorf("failed to decode config: %w", err)
	}

	return &config, nil
}

// registerWithCoordinator registers this agent with coordinator
func registerWithCoordinator(coordinatorURL string) error {
	data, err := json.Marshal(node)
	if err != nil {
		return fmt.Errorf("failed to marshal node: %w", err)
	}

	resp, err := http.Post(
		coordinatorURL+"/api/nodes",
		"application/json",
		bytes.NewReader(data),
	)
	if err != nil {
		return fmt.Errorf("failed to register: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("registration failed with status: %d", resp.StatusCode)
	}

	log.Printf("Registered with coordinator as %s (%s)", node.Name, node.ID)
	return nil
}

// startLocalServer starts the local HTTP server
func startLocalServer() {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/status", handleStatus)
	mux.HandleFunc("/api/health", handleHealth)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8081"
	}

	addr := ":" + port
	log.Printf("Agent listening on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Printf("Local server error: %v", err)
	}
}

// handleStatus returns agent status
func handleStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"node":      node,
		"awg":       awgConfig,
		"timestamp": time.Now().Format(time.RFC3339),
	})
}

// handleHealth returns health status
func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":    "ok",
		"app":       "Kolibri Mesh Agent",
		"node":      node.Name,
		"version":   "0.1.0",
		"timestamp": time.Now().Format(time.RFC3339),
	})
}
