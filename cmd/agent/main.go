// Kolibri Mesh Agent
// Runs on each server and communicates with coordinator
package main

import (
	"bytes"
	"context"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/kolibri/kolibri-mesh/pkg/awg"
	"github.com/kolibri/kolibri-mesh/pkg/mesh"
)

var (
	node       *mesh.Node
	agent      *mesh.Agent
	awgConfig  *awg.Config
	obfuscator *awg.Obfuscator
)

const agentVersion = "0.2.0"

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
		Meta:     loadNodeMeta(),
	}

	go startLocalServer()

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

	// Wait for shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	log.Println("Shutting down...")
	agent.Stop()
}

// fetchAWGConfig fetches AWG config from coordinator
func fetchAWGConfig(coordinatorURL string) (*awg.Config, error) {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(coordinatorURL + "/api/config")
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

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Post(coordinatorURL+"/api/nodes", "application/json", bytes.NewReader(data))
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
	mux.HandleFunc("/api/exec", handleExec)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8081"
	}

	addr := ":" + port
	log.Printf("Agent listening on %s", addr)
	server := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	if err := server.ListenAndServe(); err != nil {
		log.Printf("Local server error: %v", err)
	}
}

func loadNodeMeta() map[string]string {
	hostname, _ := os.Hostname()
	meta := map[string]string{
		"agent_version":  agentVersion,
		"autoscale":      "enabled",
		"capacity_slots": fmt.Sprintf("%d", runtime.NumCPU()),
		"exec_auth":      "token",
		"hostname":       hostname,
		"os":             runtime.GOOS,
		"arch":           runtime.GOARCH,
		"startup_mode":   "local-first",
		"transport":      "mesh-api",
	}
	if execToken() == "" {
		meta["exec_auth"] = "insecure-bootstrap"
	}
	if slots := strings.TrimSpace(os.Getenv("KOLIBRI_MESH_CAPACITY_SLOTS")); slots != "" {
		meta["capacity_slots"] = slots
	}
	if pool := strings.TrimSpace(os.Getenv("KOLIBRI_MESH_POOL")); pool != "" {
		meta["pool"] = pool
	}
	return meta
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
		"version":   agentVersion,
		"timestamp": time.Now().Format(time.RFC3339),
	})
}

type ExecRequest struct {
	Command string   `json:"command"`
	Args    []string `json:"args,omitempty"`
	Timeout int      `json:"timeout,omitempty"` // seconds, default 30
}

type ExecResponse struct {
	ExecID   string `json:"exec_id"`
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	ExitCode int    `json:"exit_code"`
	Duration string `json:"duration"`
	Error    string `json:"error,omitempty"`
}

type execAuditRecord struct {
	Time       string   `json:"time"`
	ExecID     string   `json:"exec_id"`
	NodeID     string   `json:"node_id,omitempty"`
	NodeName   string   `json:"node_name,omitempty"`
	RemoteAddr string   `json:"remote_addr"`
	Command    string   `json:"command,omitempty"`
	Args       []string `json:"args,omitempty"`
	Timeout    int      `json:"timeout"`
	Authorized bool     `json:"authorized"`
	ExitCode   int      `json:"exit_code,omitempty"`
	Error      string   `json:"error,omitempty"`
	Duration   string   `json:"duration,omitempty"`
}

func execToken() string {
	if token := strings.TrimSpace(os.Getenv("KOLIBRI_MESH_EXEC_TOKEN")); token != "" {
		return token
	}
	return strings.TrimSpace(os.Getenv("MESH_EXEC_TOKEN"))
}

func bearerToken(header string) string {
	const prefix = "Bearer "
	if strings.HasPrefix(header, prefix) {
		return strings.TrimSpace(strings.TrimPrefix(header, prefix))
	}
	return ""
}

func execAuthorized(r *http.Request) bool {
	token := execToken()
	if token == "" {
		return os.Getenv("KOLIBRI_MESH_EXEC_ALLOW_INSECURE") == "1"
	}
	got := strings.TrimSpace(r.Header.Get("X-Kolibri-Mesh-Token"))
	if got == "" {
		got = bearerToken(r.Header.Get("Authorization"))
	}
	return subtle.ConstantTimeCompare([]byte(got), []byte(token)) == 1
}

func defaultAuditPath() string {
	if path := strings.TrimSpace(os.Getenv("KOLIBRI_MESH_EXEC_AUDIT_LOG")); path != "" {
		return path
	}
	if os.Geteuid() == 0 {
		return "/var/log/kolibri/mesh-agent-exec.jsonl"
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return "mesh-agent-exec.jsonl"
	}
	return filepath.Join(home, ".config", "kolibri-mesh", "exec-audit.jsonl")
}

func redactArgs(args []string) []string {
	redacted := make([]string, len(args))
	copy(redacted, args)
	redactNext := false
	for i, arg := range redacted {
		lower := strings.ToLower(arg)
		if redactNext {
			redacted[i] = "[redacted]"
			redactNext = false
			continue
		}
		if strings.Contains(lower, "token=") || strings.Contains(lower, "secret=") ||
			strings.Contains(lower, "password=") || strings.Contains(lower, "key=") {
			parts := strings.SplitN(arg, "=", 2)
			redacted[i] = parts[0] + "=[redacted]"
			continue
		}
		if lower == "--token" || lower == "--secret" || lower == "--password" || lower == "--key" ||
			lower == "-token" || lower == "-secret" || lower == "-password" || lower == "-key" {
			redactNext = true
		}
	}
	return redacted
}

func writeExecAudit(rec execAuditRecord) {
	path := defaultAuditPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		log.Printf("mesh exec audit mkdir failed: %v", err)
		return
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		log.Printf("mesh exec audit open failed: %v", err)
		return
	}
	defer f.Close()
	if err := json.NewEncoder(f).Encode(rec); err != nil {
		log.Printf("mesh exec audit write failed: %v", err)
	}
}

func newExecID() string {
	nodeID := "node"
	if node != nil && node.ID != "" {
		nodeID = node.ID
	}
	return fmt.Sprintf("%s-%d", nodeID, time.Now().UnixNano())
}

func nodeIdentity() (string, string) {
	if node == nil {
		return "", ""
	}
	return node.ID, node.Name
}

// handleExec executes a shell command and returns output
func handleExec(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	execID := newExecID()
	nodeID, nodeName := nodeIdentity()
	if !execAuthorized(r) {
		writeExecAudit(execAuditRecord{
			Time:       time.Now().Format(time.RFC3339),
			ExecID:     execID,
			NodeID:     nodeID,
			NodeName:   nodeName,
			RemoteAddr: r.RemoteAddr,
			Authorized: false,
			ExitCode:   -1,
			Error:      "unauthorized",
		})
		http.Error(w, "unauthorized mesh exec", http.StatusUnauthorized)
		return
	}

	var req ExecRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	if req.Command == "" {
		http.Error(w, "command is required", http.StatusBadRequest)
		return
	}

	timeout := 30
	if req.Timeout > 0 && req.Timeout <= 300 {
		timeout = req.Timeout
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	defer cancel()

	start := time.Now()

	// Split command if it contains spaces and no args provided
	cmdStr := req.Command
	args := req.Args
	if len(args) == 0 {
		parts := strings.Fields(cmdStr)
		if len(parts) > 1 {
			cmdStr = parts[0]
			args = parts[1:]
		}
	}

	cmd := exec.CommandContext(ctx, cmdStr, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	duration := time.Since(start)

	exitCode := 0
	errorMessage := ""
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1
		}
		if ctx.Err() == context.DeadlineExceeded {
			errorMessage = "timeout"
		} else {
			errorMessage = err.Error()
		}
	}

	writeExecAudit(execAuditRecord{
		Time:       time.Now().Format(time.RFC3339),
		ExecID:     execID,
		NodeID:     nodeID,
		NodeName:   nodeName,
		RemoteAddr: r.RemoteAddr,
		Command:    cmdStr,
		Args:       redactArgs(args),
		Timeout:    timeout,
		Authorized: true,
		ExitCode:   exitCode,
		Error:      errorMessage,
		Duration:   duration.String(),
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ExecResponse{
		ExecID:   execID,
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		ExitCode: exitCode,
		Duration: duration.String(),
		Error:    errorMessage,
	})
}
