package main

import (
	"fmt"
	"log"
	"net"
	"strings"
	"sync"
	"time"
)

func init() {
	log.SetFlags(log.Llongfile)
}

type clientInfo struct {
	conn       net.Conn
	path       string
	clientID   string
	lastActive time.Time
}

type TCPManager struct {
	listener *net.Listener
	clients  map[string]clientInfo // Map client ID to client info
	Ports    []int
	sync.RWMutex
}

func NewTCPManager() *TCPManager {
	return &TCPManager{
		clients: make(map[string]clientInfo),
	}
}

func (m *TCPManager) StartListener(port int) error {
	log.Printf("Starting TCP listener on port %d...", port)
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		log.Printf("Failed to start TCP listener on port %d: %v", port, err)
		return err
	}
	
	log.Printf("TCP listener started successfully on port %d", port)
	m.listener = &listener
	m.Ports = append(m.Ports, port)
	return nil
}

func (m *TCPManager) AcceptConnection() (net.Conn, error) {
	return (*m.listener).Accept()
}

func (m *TCPManager) RegisterClient(clientID, path string, conn net.Conn) {
	m.Lock()
	defer m.Unlock()
	
	log.Printf("Registering client ID: %s, path: %s", clientID, path)
	m.clients[clientID] = clientInfo{
		conn:       conn,
		path:       path,
		clientID:   clientID,
		lastActive: time.Now(),
	}
	log.Printf("Registered client %s with path %s", clientID, path)
}

func (m *TCPManager) UpdateClientActivity(clientID string) {
	m.Lock()
	defer m.Unlock()
	if client, exists := m.clients[clientID]; exists {
		client.lastActive = time.Now()
		m.clients[clientID] = client
		log.Printf("Updated activity for client %s", clientID)
	}
}

func (m *TCPManager) RemoveClient(clientID string) {
	m.Lock()
	defer m.Unlock()
	if client, exists := m.clients[clientID]; exists {
		client.conn.Close()
		delete(m.clients, clientID)
		log.Printf("Removed client %s", clientID)
	}
}

func (m *TCPManager) HandleIncomingRequests() {
	log.Println("TCP Manager: Starting to handle incoming requests...")
	for {
		log.Println("TCP Manager: Waiting for new connection...")
		conn, err := m.AcceptConnection()
		if err != nil {
			log.Printf("TCP Manager: Error accepting connection: %v\n", err)
			continue
		}
		
		log.Printf("TCP Manager: New connection accepted from: %s", conn.RemoteAddr().String())
		
		go m.handleClient(conn)
	}
}

func (m *TCPManager) handleClient(c net.Conn) {
	remoteAddr := c.RemoteAddr().String()
	log.Printf("TCP Manager: Starting client handler for connection from %s", remoteAddr)
	
	defer func() {
		c.Close()
		log.Printf("TCP Manager: Connection closed for: %s", remoteAddr)
	}()

	// First message should be client ID and path separated by |
	buf := make([]byte, 1024)
	log.Printf("TCP Manager: Waiting for registration message from %s", remoteAddr)
	n, err := c.Read(buf)
	if err != nil {
		log.Printf("TCP Manager: Error reading registration message from %s: %v", remoteAddr, err)
		return
	}

	// Parse client ID and path from first message (format: "clientID|path")
	initialMsg := strings.TrimSpace(string(buf[:n]))
	log.Printf("TCP Manager: Received registration message from %s: '%s'", remoteAddr, initialMsg)
	
	parts := strings.Split(initialMsg, "|")
	if len(parts) != 2 {
		log.Printf("TCP Manager: Invalid registration format from %s. Expected 'clientID|path', got: %s", remoteAddr, initialMsg)
		return
	}

	clientID := strings.TrimSpace(parts[0])
	path := strings.TrimSpace(parts[1])

	// Remove any newlines from path
	path = strings.ReplaceAll(path, "\n", "")

	if clientID == "" || path == "" {
		log.Printf("TCP Manager: Invalid registration from %s: empty clientID or path. Message: %s", remoteAddr, initialMsg)
		return
	}

	log.Printf("TCP Manager: Registering client. ID: %s, Path: %s, Address: %s", clientID, path, remoteAddr)
	m.RegisterClient(clientID, path, c)

	// Send registration confirmation
	log.Printf("TCP Manager: Sending registration confirmation to client %s at %s", clientID, remoteAddr)
	_, err = c.Write([]byte("registered\n"))
	if err != nil {
		log.Printf("TCP Manager: Error sending registration confirmation to %s at %s: %v", clientID, remoteAddr, err)
		m.RemoveClient(clientID)
		return
	}
	log.Printf("TCP Manager: Registration confirmation sent to client %s at %s", clientID, remoteAddr)

	// Handle incoming messages
	for {
		n, err := c.Read(buf)
		if err != nil {
			log.Printf("TCP Manager: Error reading from client %s at %s: %v", clientID, remoteAddr, err)
			m.RemoveClient(clientID)
			return
		}

		message := strings.TrimSpace(string(buf[:n]))
		log.Printf("TCP Manager: Received message from client %s at %s: '%s'", clientID, remoteAddr, message)

		// Handle heartbeat
		if message == "heartbeat" {
			m.UpdateClientActivity(clientID)
			log.Printf("TCP Manager: Sending heartbeat-ack to client %s at %s", clientID, remoteAddr)
			_, err := c.Write([]byte("heartbeat-ack\n"))
			if err != nil {
				log.Printf("TCP Manager: Error sending heartbeat acknowledgment to %s at %s: %v", clientID, remoteAddr, err)
				m.RemoveClient(clientID)
				return
			}
			log.Printf("TCP Manager: Heartbeat acknowledgment sent to %s at %s", clientID, remoteAddr)
			continue
		}

		// Handle other messages here
		log.Printf("TCP Manager: Received other message from %s at %s: %s", clientID, remoteAddr, message)
	}
}

func (m *TCPManager) GetClients() []clientInfo {
	m.RLock()
	defer m.RUnlock()
	clients := make([]clientInfo, 0, len(m.clients))
	for _, client := range m.clients {
		clients = append(clients, client)
	}
	return clients
}

func (m *TCPManager) SendMessageToClient(path string, message string) error {
	for _, client := range m.GetClients() {
		if client.path == path {
			_, err := client.conn.Write([]byte(message + "\n"))
			if err != nil {
				return fmt.Errorf("failed to send message to client at path %s: %v", path, err)
			}
			return nil
		}
	}
	return fmt.Errorf("no client found with path %s", path)
}

func (m *TCPManager) ReceiveMessageFromClient(path string) (string, error) {
	for _, client := range m.GetClients() {
		if client.path == path {
			buf := make([]byte, 1024)
			n, err := client.conn.Read(buf)
			if err != nil {
				return "", fmt.Errorf("failed to receive message from client at path %s: %v", path, err)
			}
			return strings.TrimSpace(string(buf[:n])), nil
		}
	}
	return "", fmt.Errorf("no client found with path %s", path)
}
