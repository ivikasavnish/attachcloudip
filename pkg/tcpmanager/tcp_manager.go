package tcpmanager

import (
	"fmt"
	"io"
	"log"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/vikasavn/attachcloudip/pkg/registry"
	"github.com/vikasavn/attachcloudip/pkg/worker"
)

// TCPManager handles TCP connections and routing
type TCPManager struct {
	registry    *registry.Registry
	dispatcher  *worker.Dispatcher
	listeners   map[int]net.Listener
	connections map[string]net.Conn
	portInventory []int
	maxPorts     int
	mu          sync.RWMutex
	logger      *log.Logger
}

// NewTCPManager creates a new TCP manager
func NewTCPManager(registry *registry.Registry, dispatcher *worker.Dispatcher) *TCPManager {
	return &TCPManager{
		registry:    registry,
		dispatcher:  dispatcher,
		listeners:   make(map[int]net.Listener),
		connections: make(map[string]net.Conn),
		portInventory: make([]int, 0, 10),
		maxPorts:     10,
		logger:      log.Default(),
	}
}

// StartListener starts a TCP listener for a specific port
func (m *TCPManager) StartListener(port int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.listeners[port]; exists {
		return fmt.Errorf("listener already exists on port %d", port)
	}

	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return fmt.Errorf("failed to start listener on port %d: %v", port, err)
	}

	m.listeners[port] = listener
	m.logger.Printf("TCP Listener started on port %d", port)

	go m.acceptConnections(listener, port)
	return nil
}

// acceptConnections handles incoming TCP connections
func (m *TCPManager) acceptConnections(listener net.Listener, port int) {
	for {
		conn, err := listener.Accept()
		if err != nil {
			m.logger.Printf("Error accepting connection on port %d: %v", port, err)
			return
		}

		go m.handleConnection(conn, port)
	}
}

// handleConnection processes an individual TCP connection
func (m *TCPManager) handleConnection(conn net.Conn, port int) {
	defer func() {
		conn.Close()
		// Find and decrement TCP connection for the client
		clients := m.registry.ListClients()
		for _, client := range clients {
			if client.TCPPort == port {
				m.registry.DecrementTCPConnection(client.ID)
				break
			}
		}
	}()

	// Read initial request
	buffer := make([]byte, 1024)
	n, err := conn.Read(buffer)
	if err != nil {
		m.logger.Printf("Error reading initial request: %v", err)
		return
	}
	initialRequest := string(buffer[:n])

	// Find client for this port
	clients := m.registry.ListClients()
	var selectedClient *registry.ClientRegistration
	for _, client := range clients {
		if client.TCPPort == port {
			selectedClient = client
			// Increment TCP connection count
			m.registry.IncrementTCPConnection(client.ID)
			break
		}
	}

	if selectedClient == nil {
		m.logger.Printf("No client found for port %d", port)
		return
	}

	// Create connection routing job
	job := &UpdatedTCPConnectionJob{
		Connection:     conn,
		ClientID:       selectedClient.ID,
		InitialRequest: initialRequest,
		RoutingKey:     extractRoutingKey(initialRequest),
	}

	// Submit job to dispatcher
	m.dispatcher.Submit(job)
}

// RouteConnection attempts to route a connection to the appropriate client
func (m *TCPManager) RouteConnection(conn net.Conn, initialRequest string) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Extract routing information from the initial request
	routingKey := extractRoutingKey(initialRequest)

	// Find clients registered for this routing key
	clients, err := m.registry.FindClientForPath(routingKey)
	if err != nil || len(clients) == 0 {
		log.Printf("No clients found for routing key: %s", routingKey)
		return fmt.Errorf("no clients found for routing key: %s", routingKey)
	}

	// Select client for routing
	selectedClient := selectClientForRouting(clients)

	// Create connection routing job
	job := &UpdatedTCPConnectionJob{
		Connection:     conn,
		ClientID:       selectedClient.ID,
		InitialRequest: initialRequest,
		RoutingKey:     routingKey,
	}

	// Submit job to dispatcher
	m.dispatcher.Submit(job)

	return nil
}

// extractRoutingKey parses the initial request to determine routing information
func extractRoutingKey(request string) string {
	// Trim whitespace and convert to lowercase for consistent matching
	request = strings.TrimSpace(strings.ToLower(request))

	// Extract first word or path-like segment
	parts := strings.SplitN(request, " ", 2)
	if len(parts) > 0 {
		return parts[0]
	}

	return ""
}

// selectClientForRouting chooses a client for routing
func selectClientForRouting(clients []*registry.ClientRegistration) *registry.ClientRegistration {
	if len(clients) == 0 {
		return nil
	}

	// Simple round-robin or random selection could be implemented here
	return clients[0]
}

// MonitorConnectionHealth periodically checks and manages connection health
func (m *TCPManager) MonitorConnectionHealth(checkInterval, maxIdleTime time.Duration) {
	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

	for range ticker.C {
		m.mu.Lock()

		for clientID, conn := range m.connections {
			// Check connection idle time
			if isConnectionStale(conn, maxIdleTime) {
				log.Printf("Closing stale connection for client %s", clientID)
				conn.Close()
				delete(m.connections, clientID)
			}
		}

		m.mu.Unlock()
	}
}

// isConnectionStale checks if a connection has been idle for too long
func isConnectionStale(conn net.Conn, maxIdleTime time.Duration) bool {
	tcpConn, ok := conn.(*net.TCPConn)
	if !ok {
		return false
	}

	file, err := tcpConn.File()
	if err != nil {
		return false
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return false
	}

	return time.Since(stat.ModTime()) > maxIdleTime
}

// UpdatedTCPConnectionJob represents a job for handling a TCP connection
type UpdatedTCPConnectionJob struct {
	Connection      net.Conn
	ClientID        string
	InitialRequest  string
	RoutingKey      string
	RoutingMetadata map[string]string
}

// Execute implements the worker.Job interface
func (j *UpdatedTCPConnectionJob) Execute() error {
	return j.Process()
}

// Process implements an enhanced job processing method
func (j *UpdatedTCPConnectionJob) Process() error {
	log.Printf("Processing TCP connection for client %s with routing key: %s",
		j.ClientID, j.RoutingKey)

	switch j.RoutingKey {
	case "service":
		return j.processServiceRouting()
	case "stream":
		return j.processStreamRouting()
	default:
		return j.processDefaultRouting()
	}
}

// processServiceRouting handles routing for service-based connections
func (j *UpdatedTCPConnectionJob) processServiceRouting() error {
	log.Printf("Routing service connection for client %s", j.ClientID)

	serviceHandler := func(data []byte) ([]byte, error) {
		return []byte("Service response"), nil
	}

	return j.genericConnectionHandler(serviceHandler)
}

// processStreamRouting handles routing for streaming connections
func (j *UpdatedTCPConnectionJob) processStreamRouting() error {
	log.Printf("Routing stream connection for client %s", j.ClientID)

	streamHandler := func(data []byte) ([]byte, error) {
		return data, nil
	}

	return j.genericConnectionHandler(streamHandler)
}

// processDefaultRouting provides a fallback routing mechanism
func (j *UpdatedTCPConnectionJob) processDefaultRouting() error {
	log.Printf("Using default routing for client %s", j.ClientID)

	defaultHandler := func(data []byte) ([]byte, error) {
		return data, nil
	}

	return j.genericConnectionHandler(defaultHandler)
}

// genericConnectionHandler provides a reusable connection processing template
func (j *UpdatedTCPConnectionJob) genericConnectionHandler(
	handler func([]byte) ([]byte, error),
) error {
	defer j.Connection.Close()

	buffer := make([]byte, 1024)
	for {
		err := j.Connection.SetReadDeadline(time.Now().Add(5 * time.Minute))
		if err != nil {
			return fmt.Errorf("set read deadline: %w", err)
		}

		n, err := j.Connection.Read(buffer)
		if err != nil {
			if err == io.EOF {
				log.Printf("Connection closed by client %s", j.ClientID)
				return nil
			}
			return fmt.Errorf("read error: %w", err)
		}

		response, err := handler(buffer[:n])
		if err != nil {
			log.Printf("Handler error for client %s: %v", j.ClientID, err)
			return err
		}

		_, err = j.Connection.Write(response)
		if err != nil {
			return fmt.Errorf("write error: %w", err)
		}
	}
}

// CloseAllConnections closes all active TCP connections
func (m *TCPManager) CloseAllConnections() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.logger.Println("Closing all active TCP connections")
	
	for connKey, conn := range m.connections {
		m.logger.Printf("Closing connection: %s", connKey)
		if err := conn.Close(); err != nil {
			m.logger.Printf("Error closing connection %s: %v", connKey, err)
		}
	}

	// Clear the connections map
	m.connections = make(map[string]net.Conn)

	// Reset listeners
	for port, listener := range m.listeners {
		m.logger.Printf("Closing listener on port %d", port)
		if err := listener.Close(); err != nil {
			m.logger.Printf("Error closing listener on port %d: %v", port, err)
		}
	}
	m.listeners = make(map[int]net.Listener)
}

// GetAvailablePort finds an available port for TCP listener
func (m *TCPManager) GetAvailablePort(startPort int) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// First check if we have room in our inventory
	if len(m.portInventory) >= m.maxPorts {
		return 0, fmt.Errorf("port inventory is full (max %d ports)", m.maxPorts)
	}

	// Try the start port first if specified
	if startPort > 0 {
		listener, err := net.Listen("tcp", fmt.Sprintf(":%d", startPort))
		if err == nil {
			listener.Close()
			m.portInventory = append(m.portInventory, startPort)
			return startPort, nil
		}
	}

	// Try to find the next available port near the start port
	currentPort := startPort
	if currentPort <= 0 {
		currentPort = 8000 // Default starting port if none specified
	}

	for attempts := 0; attempts < 1000; attempts++ {
		listener, err := net.Listen("tcp", fmt.Sprintf(":%d", currentPort))
		if err == nil {
			listener.Close()
			m.portInventory = append(m.portInventory, currentPort)
			return currentPort, nil
		}
		currentPort++
	}

	return 0, fmt.Errorf("could not find available port after 1000 attempts")
}

// StartOrReuseListener starts a new TCP listener or returns an existing one
func (m *TCPManager) StartOrReuseListener(port int) (net.Listener, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if we already have a listener for this port
	if listener, exists := m.listeners[port]; exists {
		return listener, nil
	}

	// Try to start a listener on the specified port
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		// If the specified port fails, try to get a new available port
		newPort, err := m.GetAvailablePort(port + 1)
		if err != nil {
			return nil, fmt.Errorf("failed to find available port: %v", err)
		}
		
		listener, err = net.Listen("tcp", fmt.Sprintf(":%d", newPort))
		if err != nil {
			return nil, fmt.Errorf("failed to start listener on new port %d: %v", newPort, err)
		}
		port = newPort
	}

	m.listeners[port] = listener
	m.logger.Printf("TCP Listener started on port %d", port)

	go m.acceptConnections(listener, port)
	return listener, nil
}
