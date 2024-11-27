package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

var logger = log.New(os.Stdout, "\x1b[32m[TunnelService]\x1b[0m ", log.LstdFlags|log.Lmicroseconds)

type HttpRequest struct {
	Method  string
	Path    string
	Headers map[string]string
	Body    []byte
}

type HttpResponse struct {
	StatusCode int
	Headers    map[string]string
	Body       []byte
}

type StreamRequestType int
type StreamResponseType int

const (
	StreamRequestType_HTTP_REQUEST StreamRequestType = iota
	StreamRequestType_HTTP_RESPONSE
	StreamRequestType_HEARTBEAT
	StreamRequestType_PATH_UPDATE
)

const (
	StreamResponseType_HTTP_REQUEST StreamResponseType = iota
	StreamResponseType_HTTP_RESPONSE
	StreamResponseType_ERROR
	StreamResponseType_REGISTRATION_SUCCESS
)

type StreamRequest struct {
	Type        StreamRequestType
	RequestId   string
	HttpRequest *HttpRequest
	Protocol    string
}

type StreamResponse struct {
	Type         StreamResponseType
	RequestId    string
	HttpRequest  *HttpRequest
	HttpResponse *HttpResponse
	Message      string
	Port         int
}

type ResponseWaiter struct {
	Response chan *StreamResponse
	Done     chan struct{}
}

type ClientInfo struct {
	ID          string
	SessionID   string
	Paths       []string
	Description string
	Metadata    map[string]string
	LastSeen    time.Time
	Status      string
	Protocol    string
	Port        int
}

type ConnectionOptionsProtocol int

const (
	ConnectionOptionsProtocol_HTTP ConnectionOptionsProtocol = iota
	ConnectionOptionsProtocol_TCP
)

type ConnectionOptions struct {
	Protocol          ConnectionOptionsProtocol
	BufferSize        int
	KeepAlive         bool
	KeepAliveInterval int
	IdleTimeout       int
}

type PortManager struct {
	ports map[int]bool
	mu    sync.RWMutex
}

func NewPortManager() *PortManager {
	return &PortManager{
		ports: make(map[int]bool),
	}
}

func (pm *PortManager) AllocatePort() (int, error) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	for port := 1024; port < 65536; port++ {
		if !pm.ports[port] {
			pm.ports[port] = true
			return port, nil
		}
	}

	return 0, fmt.Errorf("no available ports")
}

func (pm *PortManager) ReleasePort(port int) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	pm.ports[port] = false
}

type TunnelService struct {
	clients         map[string]*ClientInfo
	pathClients     map[string][]string
	responseWaiters *sync.Map
	mu              sync.RWMutex
	availablePorts  []int
}

func NewTunnelService(ports []int) *TunnelService {
	return &TunnelService{
		clients:         make(map[string]*ClientInfo),
		pathClients:     make(map[string][]string),
		responseWaiters: &sync.Map{},
		availablePorts:  ports,
	}
}

func (s *TunnelService) Register(ctx context.Context, req *StreamRequest) (*StreamResponse, error) {
	logger.Printf("📝 Received registration request from client: %s", req.RequestId)

	if req.RequestId == "" {
		return nil, fmt.Errorf("client ID is required")
	}

	if len(req.HttpRequest.Path) == 0 {
		return nil, fmt.Errorf("at least one path is required")
	}

	if len(s.availablePorts) == 0 {
		return nil, fmt.Errorf("no available ports for registration")
	}

	sessionID := uuid.New().String()

	s.mu.Lock()
	defer s.mu.Unlock()

	// Remove existing client if any
	if oldClient, exists := s.clients[req.RequestId]; exists {
		logger.Printf("⚠️  Removing existing client: %s", req.RequestId)
		s.removeClientPaths(oldClient)
		delete(s.clients, req.RequestId)
	}

	// Get an available port
	port := s.availablePorts[0]
	s.availablePorts = s.availablePorts[1:] // Remove used port

	// Create new client info
	client := &ClientInfo{
		ID:          req.RequestId,
		SessionID:   sessionID,
		Paths:       make([]string, 0, 1),
		Description: "",
		Metadata:    make(map[string]string),
		LastSeen:    time.Now(),
		Status:      "connecting",
		Protocol:    req.Protocol,
		Port:        port,
	}

	// Process paths
	normalizedPath := normalizePath(req.HttpRequest.Path)
	if !contains(client.Paths, normalizedPath) {
		client.Paths = append(client.Paths, normalizedPath)
		s.pathClients[normalizedPath] = append(s.pathClients[normalizedPath], req.RequestId)
	}

	// Store client
	s.clients[req.RequestId] = client

	logger.Printf("✅ Client registered successfully:")
	logger.Printf("   - Client ID: %s", req.RequestId)
	logger.Printf("   - Session ID: %s", sessionID)
	logger.Printf("   - Assigned Port: %d", port)
	logger.Printf("   - Paths: %v", client.Paths)

	return &StreamResponse{
		Type:      StreamResponseType_REGISTRATION_SUCCESS,
		RequestId: req.RequestId,
		Port:      port,
	}, nil
}

func (s *TunnelService) SendToClient(clientID string, msg *StreamResponse) error {
	s.mu.RLock()
	_, ok := s.clients[clientID]
	s.mu.RUnlock()

	if !ok {
		return fmt.Errorf("client not found: %s", clientID)
	}

	// In a non-gRPC world, this would involve a different communication mechanism
	// For now, we'll just log that a message would be sent
	fmt.Printf("Would send message to client %s: %+v\n", clientID, msg)

	return nil
}

func (s *TunnelService) handleHTTPRequest(ctx context.Context, clientID string, req *HttpRequest, requestID string) error {
	waiter := &ResponseWaiter{
		Response: make(chan *StreamResponse, 1),
		Done:     make(chan struct{}),
	}
	s.responseWaiters.Store(requestID, waiter)
	defer s.responseWaiters.Delete(requestID)

	streamResp := &StreamResponse{
		Type:        StreamResponseType_HTTP_REQUEST,
		RequestId:   requestID,
		HttpRequest: req,
	}

	if err := s.SendToClient(clientID, streamResp); err != nil {
		return fmt.Errorf("failed to send request to client: %v", err)
	}

	select {
	case resp := <-waiter.Response:
		if resp.Type == StreamResponseType_ERROR {
			return fmt.Errorf("client error: %s", resp.Message)
		}
		return nil
	case <-ctx.Done():
		return fmt.Errorf("request cancelled")
	case <-time.After(30 * time.Second):
		return fmt.Errorf("request timeout")
	}
}

func (s *TunnelService) handleHTTPResponse(req *StreamRequest) error {
	if req.RequestId == "" {
		return fmt.Errorf("empty request ID")
	}

	waiterInterface, ok := s.responseWaiters.Load(req.RequestId)
	if !ok {
		return fmt.Errorf("no waiter found for request ID: %s", req.RequestId)
	}

	waiter, ok := waiterInterface.(*ResponseWaiter)
	if !ok {
		return fmt.Errorf("invalid waiter type for request ID: %s", req.RequestId)
	}

	var streamResp *StreamResponse
	switch req.Type {
	case StreamRequestType_HTTP_RESPONSE:
		streamResp = &StreamResponse{
			Type:      StreamResponseType_HTTP_RESPONSE,
			RequestId: req.RequestId,
			HttpResponse: &HttpResponse{
				StatusCode: 200, // Default status
				Headers:    req.HttpRequest.Headers,
				Body:       req.HttpRequest.Body,
			},
		}
	default:
		streamResp = &StreamResponse{
			Type:      StreamResponseType_ERROR,
			RequestId: req.RequestId,
			Message:   "Unexpected stream request type",
		}
	}

	select {
	case waiter.Response <- streamResp:
		return nil
	default:
		return fmt.Errorf("response channel full for request ID: %s", req.RequestId)
	}
}

func (s *TunnelService) StoreResponseWaiter(requestID string, waiter *ResponseWaiter) {
	s.responseWaiters.Store(requestID, waiter)
}

func (s *TunnelService) DeleteResponseWaiter(requestID string) {
	s.responseWaiters.Delete(requestID)
}

func (s *TunnelService) GetResponseWaiter(requestID string) (*ResponseWaiter, bool) {
	waiterInterface, ok := s.responseWaiters.Load(requestID)
	if !ok {
		return nil, false
	}

	waiter, ok := waiterInterface.(*ResponseWaiter)
	return waiter, ok
}

func (s *TunnelService) FindMatchingClient(requestPath string) (*ClientInfo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	normalizedRequestPath := normalizePath(requestPath)
	logger.Printf("Finding client for path: %s (normalized: %s)", requestPath, normalizedRequestPath)

	// First try exact match
	if clientIDs, exists := s.pathClients[normalizedRequestPath]; exists && len(clientIDs) > 0 {
		if client, exists := s.clients[clientIDs[0]]; exists {
			logger.Printf("Found exact match client %s for path %s", client.ID, normalizedRequestPath)
			return client, nil
		}
	}

	// Then try pattern matching
	for path, clientIDs := range s.pathClients {
		if len(clientIDs) > 0 && isPathMatch(path, normalizedRequestPath) {
			if client, exists := s.clients[clientIDs[0]]; exists {
				logger.Printf("Found pattern match client %s for path %s using pattern %s",
					client.ID, normalizedRequestPath, path)
				return client, nil
			}
		}
	}

	logger.Printf("No client found for path: %s", requestPath)
	return nil, fmt.Errorf("no client found for path: %s", requestPath)
}

func (s *TunnelService) removeClient(clientID string) {
	if client, exists := s.clients[clientID]; exists {
		s.removeClientPaths(client)
		delete(s.clients, clientID)
		logger.Printf("Removed client %s", clientID)
	}
}

func (s *TunnelService) removeClientPaths(client *ClientInfo) {
	for _, p := range client.Paths {
		normalizedPath := normalizePath(p)
		clients := s.pathClients[normalizedPath]
		for i, id := range clients {
			if id == client.ID {
				s.pathClients[normalizedPath] = append(clients[:i], clients[i+1:]...)
				logger.Printf("Removed path mapping: %s -> %s", normalizedPath, client.ID)
				break
			}
		}
		if len(s.pathClients[normalizedPath]) == 0 {
			delete(s.pathClients, normalizedPath)
			logger.Printf("Removed empty path mapping for: %s", normalizedPath)
		}
	}
}

func normalizePath(p string) string {
	// Remove trailing slash if present
	p = strings.TrimRight(p, "/")
	// Ensure path starts with /
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	return p
}

func isPathMatch(pattern, requestPath string) bool {
	// For now, just check if the request path starts with the pattern
	// TODO: Implement more sophisticated pattern matching (e.g., glob patterns)
	return strings.HasPrefix(requestPath, pattern)
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func (s *TunnelService) HandleStatusRequest(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	status := make([]map[string]interface{}, 0)
	for _, client := range s.clients {
		status = append(status, map[string]interface{}{
			"id":          client.ID,
			"paths":       client.Paths,
			"description": client.Description,
			"status":      client.Status,
			"port":        client.Port,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"clients": status,
		"paths":   s.pathClients,
	})
}
