// Package mesh implements Kolibri Mesh networking
package mesh

import (
	"bytes"
	"encoding/json"
	"net"
	"net/http"
	"sync"
	"time"
)

// Node represents a mesh node
type Node struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	IP          net.IP    `json:"ip"`
	PublicKey   string    `json:"public_key"`
	Endpoint    string    `json:"endpoint"`
	Role        string    `json:"role"` // coordinator, agent, mikrotik
	Status      string    `json:"status"` // online, offline, connecting
	LastSeen    time.Time `json:"last_seen"`
	Meta        map[string]string `json:"meta,omitempty"`
}

// Mesh represents the mesh network
type Mesh struct {
	mu    sync.RWMutex
	nodes map[string]*Node
	local *Node
}

// NewMesh creates a new mesh network
func NewMesh(localNode *Node) *Mesh {
	return &Mesh{
		nodes: make(map[string]*Node),
		local: localNode,
	}
}

// AddNode adds a node to the mesh
func (m *Mesh) AddNode(node *Node) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.nodes[node.ID] = node
}

// RemoveNode removes a node from the mesh
func (m *Mesh) RemoveNode(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.nodes, id)
}

// GetNode gets a node by ID
func (m *Mesh) GetNode(id string) *Node {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.nodes[id]
}

// GetNodes gets all nodes
func (m *Mesh) GetNodes() []*Node {
	m.mu.RLock()
	defer m.mu.RUnlock()
	nodes := make([]*Node, 0, len(m.nodes))
	for _, node := range m.nodes {
		nodes = append(nodes, node)
	}
	return nodes
}

// GetOnlineNodes gets all online nodes
func (m *Mesh) GetOnlineNodes() []*Node {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var nodes []*Node
	for _, node := range m.nodes {
		if node.Status == "online" {
			nodes = append(nodes, node)
		}
	}
	return nodes
}

// UpdateNodeStatus updates node status
func (m *Mesh) UpdateNodeStatus(id string, status string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if node, ok := m.nodes[id]; ok {
		node.Status = status
		node.LastSeen = time.Now()
	}
}

// GetLocalNode gets the local node
func (m *Mesh) GetLocalNode() *Node {
	return m.local
}

// MarshalJSON marshals mesh to JSON
func (m *Mesh) MarshalJSON() ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return json.Marshal(struct {
		Local *Node            `json:"local"`
		Nodes map[string]*Node `json:"nodes"`
	}{
		Local: m.local,
		Nodes: m.nodes,
	})
}

// Heartbeat sends heartbeat to coordinator
type Heartbeat struct {
	NodeID    string    `json:"node_id"`
	Status    string    `json:"status"`
	Timestamp time.Time `json:"timestamp"`
	Meta      map[string]string `json:"meta,omitempty"`
}

// HeartbeatResponse is the response to a heartbeat
type HeartbeatResponse struct {
	OK      bool      `json:"ok"`
	Nodes   []*Node   `json:"nodes,omitempty"`
	Message string    `json:"message,omitempty"`
}

// Coordinator represents the mesh coordinator
type Coordinator struct {
	mesh      *Mesh
	heartbeat time.Duration
	stopCh    chan struct{}
}

// NewCoordinator creates a new coordinator
func NewCoordinator(mesh *Mesh, heartbeat time.Duration) *Coordinator {
	return &Coordinator{
		mesh:      mesh,
		heartbeat: heartbeat,
		stopCh:    make(chan struct{}),
	}
}

// Start starts the coordinator
func (c *Coordinator) Start() {
	go c.cleanupLoop()
}

// Stop stops the coordinator
func (c *Coordinator) Stop() {
	close(c.stopCh)
}

// HandleHeartbeat handles heartbeat from agent
func (c *Coordinator) HandleHeartbeat(hb *Heartbeat) *HeartbeatResponse {
	c.mesh.UpdateNodeStatus(hb.NodeID, hb.Status)
	return &HeartbeatResponse{
		OK:    true,
		Nodes: c.mesh.GetOnlineNodes(),
	}
}

// cleanupLoop removes offline nodes
func (c *Coordinator) cleanupLoop() {
	ticker := time.NewTicker(c.heartbeat * 3)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.cleanup()
		case <-c.stopCh:
			return
		}
	}
}

// cleanup removes nodes that haven't sent heartbeat
func (c *Coordinator) cleanup() {
	c.mesh.mu.Lock()
	defer c.mesh.mu.Unlock()

	// Mark offline if not seen in 10 minutes
	threshold := time.Now().Add(-10 * time.Minute)
	for _, node := range c.mesh.nodes {
		if node.LastSeen.Before(threshold) {
			node.Status = "offline"
		}
	}
}

// Agent represents a mesh agent
type Agent struct {
	node        *Node
	coordinator string
	heartbeat   time.Duration
	mesh        *Mesh
	stopCh      chan struct{}
}

// NewAgent creates a new agent
func NewAgent(node *Node, coordinator string, heartbeat time.Duration) *Agent {
	return &Agent{
		node:        node,
		coordinator: coordinator,
		heartbeat:   heartbeat,
		mesh:        NewMesh(node),
		stopCh:      make(chan struct{}),
	}
}

// Start starts the agent
func (a *Agent) Start() {
	go a.heartbeatLoop()
}

// Stop stops the agent
func (a *Agent) Stop() {
	close(a.stopCh)
}

// heartbeatLoop sends heartbeats to coordinator
func (a *Agent) heartbeatLoop() {
	ticker := time.NewTicker(a.heartbeat)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			a.sendHeartbeat()
		case <-a.stopCh:
			return
		}
	}
}

// sendHeartbeat sends heartbeat to coordinator
func (a *Agent) sendHeartbeat() {
	hb := &Heartbeat{
		NodeID:    a.node.ID,
		Status:    "online",
		Timestamp: time.Now(),
	}

	data, err := json.Marshal(hb)
	if err != nil {
		return
	}

	resp, err := http.Post(
		a.coordinator+"/api/heartbeat",
		"application/json",
		bytes.NewReader(data),
	)
	if err != nil {
		return
	}
	defer resp.Body.Close()
}

// GetMesh gets the mesh network
func (a *Agent) GetMesh() *Mesh {
	return a.mesh
}
