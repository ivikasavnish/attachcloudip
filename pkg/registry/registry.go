package registry

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ClientType defines the type of client
type ClientType int

const (
	ClientTypeHTTP ClientType = iota
	ClientTypeTCP
)

// ClientRegistration represents a registered client
type ClientRegistration struct {
	ID                   string
	Type                 ClientType
	Paths                []string
	LastHeartbeat        time.Time
	TCPPort              int
	ActiveTCPConnections int
	Status               string
	Metadata             map[string]string
	mu                   sync.Mutex
}

// Registry manages client registrations
type Registry struct {
	mu               sync.RWMutex
	clients          map[string]*ClientRegistration
	pathToClientIDs  map[string][]string
	availableTCPPort int
}

// NewRegistry creates a new client registry
func NewRegistry(startPort int) *Registry {
	return &Registry{
		clients:          make(map[string]*ClientRegistration),
		pathToClientIDs:  make(map[string][]string),
		availableTCPPort: startPort,
	}
}

// AllocateTCPPort allocates a TCP port for a client
func (r *Registry) AllocateTCPPort(clientType ClientType) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Allocate port
	tcpPort := r.availableTCPPort
	r.availableTCPPort++

	return tcpPort, nil
}

// RegisterClient adds a new client to the registry
func (r *Registry) RegisterClient(paths []string, clientType ClientType, metadata map[string]string) (*ClientRegistration, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Generate unique client ID
	clientID := uuid.New().String()

	// Allocate TCP port if needed
	var tcpPort int
	if clientType == ClientTypeTCP {
		var err error
		tcpPort, err = r.AllocateTCPPort(clientType)
		if err != nil {
			return nil, fmt.Errorf("failed to allocate TCP port: %w", err)
		}
	}

	client := &ClientRegistration{
		ID:                   clientID,
		Type:                 clientType,
		Paths:                paths,
		LastHeartbeat:        time.Now(),
		TCPPort:              tcpPort,
		ActiveTCPConnections: 0,
		Status:               "active",
		Metadata:             metadata,
	}

	// Store client
	r.clients[clientID] = client

	// Map paths to client IDs
	for _, path := range paths {
		r.pathToClientIDs[path] = append(r.pathToClientIDs[path], clientID)
	}

	// Debug logging
	fmt.Printf("Registered client: ID=%s, Type=%v, Paths=%v, TCPPort=%d\n",
		clientID, clientType, paths, tcpPort)
	fmt.Printf("Current clients: %d\n", len(r.clients))

	return client, nil
}

// FindClientForPath finds clients registered for a specific path
func (r *Registry) FindClientForPath(path string) ([]*ClientRegistration, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	clientIDs, exists := r.pathToClientIDs[path]
	if !exists || len(clientIDs) == 0 {
		return nil, fmt.Errorf("no clients found for path: %s", path)
	}

	clients := make([]*ClientRegistration, 0, len(clientIDs))
	for _, clientID := range clientIDs {
		if client, ok := r.clients[clientID]; ok {
			clients = append(clients, client)
		}
	}

	return clients, nil
}

// UpdateHeartbeat updates the client's heartbeat timestamp and status
func (r *Registry) UpdateHeartbeat(clientID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	client, exists := r.clients[clientID]
	if !exists {
		return fmt.Errorf("client not found: %s", clientID)
	}

	client.mu.Lock()
	defer client.mu.Unlock()

	client.LastHeartbeat = time.Now()
	client.Status = "active"

	// Log heartbeat with connection info
	log.Printf("❤️  Heartbeat from client %s (TCP Connections: %d, Last Seen: %s)",
		clientID, client.ActiveTCPConnections, client.LastHeartbeat.Format(time.RFC3339))

	return nil
}

// StartHeartbeatMonitor starts monitoring client heartbeats
func (r *Registry) StartHeartbeatMonitor(interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for range ticker.C {
			r.mu.Lock()
			now := time.Now()

			for id, client := range r.clients {
				client.mu.Lock()
				lastBeat := client.LastHeartbeat
				client.mu.Unlock()

				if now.Sub(lastBeat) > interval*2 {
					log.Printf("⚠️  Client %s hasn't sent heartbeat in %v", id, now.Sub(lastBeat))
					client.mu.Lock()
					client.Status = "inactive"
					client.mu.Unlock()
				}
			}

			r.mu.Unlock()
		}
	}()
}

// CleanupStaleClients removes clients that haven't sent a heartbeat in a while
func (r *Registry) CleanupStaleClients(timeout time.Duration) []string {
	r.mu.Lock()
	defer r.mu.Unlock()

	staleClientIDs := make([]string, 0)
	now := time.Now()

	for clientID, client := range r.clients {
		if now.Sub(client.LastHeartbeat) > timeout {
			// Remove from clients
			delete(r.clients, clientID)

			// Remove from path mappings
			for _, path := range client.Paths {
				clientList := r.pathToClientIDs[path]
				for i, id := range clientList {
					if id == clientID {
						r.pathToClientIDs[path] = append(clientList[:i], clientList[i+1:]...)
						break
					}
				}
				// Remove path if no clients
				if len(r.pathToClientIDs[path]) == 0 {
					delete(r.pathToClientIDs, path)
				}
			}

			staleClientIDs = append(staleClientIDs, clientID)
		}
	}

	return staleClientIDs
}

// ClientCount returns the total number of registered clients
func (r *Registry) ClientCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	count := len(r.clients)
	fmt.Printf("ClientCount called: %d clients\n", count)
	return count
}

// ListClients returns a copy of all registered clients
func (r *Registry) ListClients() map[string]*ClientRegistration {

	return r.clients
}

// IncrementTCPConnection increments the active TCP connection count for a client
func (r *Registry) IncrementTCPConnection(clientID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	client, exists := r.clients[clientID]
	if !exists {
		return fmt.Errorf("client not found: %s", clientID)
	}

	client.mu.Lock()
	client.ActiveTCPConnections++
	client.mu.Unlock()

	fmt.Printf("Client %s TCP connections: %d\n", clientID, client.ActiveTCPConnections)
	return nil
}

// DecrementTCPConnection decrements the active TCP connection count for a client
func (r *Registry) DecrementTCPConnection(clientID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	client, exists := r.clients[clientID]
	if !exists {
		return fmt.Errorf("client not found: %s", clientID)
	}

	client.mu.Lock()
	if client.ActiveTCPConnections > 0 {
		client.ActiveTCPConnections--
	}
	client.mu.Unlock()

	fmt.Printf("Client %s TCP connections: %d\n", clientID, client.ActiveTCPConnections)
	return nil
}

// GetNextTCPPort returns the next available TCP port
func (r *Registry) GetNextTCPPort() int {
	r.mu.Lock()
	defer r.mu.Unlock()

	port := r.availableTCPPort
	r.availableTCPPort++
	return port
}

// RemoveClient removes a client from the registry
func (r *Registry) RemoveClient(clientID string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Remove client from the registry
	delete(r.clients, clientID)

	// Remove client ID from path mappings
	for path, ids := range r.pathToClientIDs {
		for i, id := range ids {
			if id == clientID {
				r.pathToClientIDs[path] = append(ids[:i], ids[i+1:]...)
				break
			}
		}
	}

	log.Printf("[REGISTRY] Removed client %s from registry", clientID)
}
