package httpfrontend

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/vikasavn/attachcloudip/pkg/registry"
	"github.com/vikasavn/attachcloudip/pkg/tcpmanager"
	"github.com/vikasavn/attachcloudip/pkg/worker"
	"github.com/vikasavn/attachcloudip/pkg/service"
	"github.com/google/uuid"
)

// HTTPFrontend manages HTTP routing and registration
type HTTPFrontend struct {
	registry    *registry.Registry
	tcpManager  *tcpmanager.TCPManager
	dispatcher  *worker.Dispatcher
	logger      *log.Logger
	tunnelService *service.TunnelService
}

// NewHTTPFrontend creates a new HTTP frontend
func NewHTTPFrontend(reg *registry.Registry, tcpMgr *tcpmanager.TCPManager, dispatcher *worker.Dispatcher, ports []int) *HTTPFrontend {
	log.Printf("Creating new HTTP frontend with %d pre-allocated ports: %v", len(ports), ports)
	return &HTTPFrontend{
		registry:    reg,
		tcpManager:  tcpMgr,
		dispatcher:  dispatcher,
		logger:      log.Default(),
		tunnelService: service.NewTunnelService(ports),
	}
}

// RegisterHandler handles client registration
func (f *HTTPFrontend) RegisterHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse registration request
	var req struct {
		Paths    []string          `json:"paths"`
		Metadata map[string]string `json:"metadata"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Create stream request
	streamReq := &service.StreamRequest{
		Type:     service.StreamRequestType_HTTP_REQUEST,
		RequestId: uuid.New().String(),
		Protocol: "tcp",
		HttpRequest: &service.HttpRequest{
			Path: req.Paths[0],
		},
	}

	// Register with tunnel service
	resp, err := f.tunnelService.Register(r.Context(), streamReq)
	if err != nil {
		f.logger.Printf("Registration failed: %v", err)
		http.Error(w, fmt.Sprintf("Registration failed: %v", err), http.StatusInternalServerError)
		return
	}

	// Log port allocation
	f.logger.Printf("Allocated port %d for client %s", resp.Port, resp.RequestId)

	// Return registration response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"client_id": resp.RequestId,
		"port":      resp.Port,
		"status":    "success",
	})
}

// ProxyHandler routes HTTP requests to registered clients
func (f *HTTPFrontend) ProxyHandler(w http.ResponseWriter, r *http.Request) {
	// Find clients registered for this path
	clients, err := f.registry.FindClientForPath(r.URL.Path)
	if err != nil {
		http.Error(w, "No client found for path", http.StatusNotFound)
		return
	}

	// Select first matching client (could be enhanced with load balancing)
	client := clients[0]

	// Create a job to handle the HTTP request
	job := &HTTPProxyJob{
		Request:    r,
		Response:   w,
		ClientID:   client.ID,
		ClientType: client.Type,
	}

	// Submit job to dispatcher
	f.dispatcher.Submit(job)
}

// HTTPProxyJob represents a job for proxying HTTP requests
type HTTPProxyJob struct {
	Request    *http.Request
	Response   http.ResponseWriter
	ClientID   string
	ClientType registry.ClientType
}

// Process implements the Job interface
func (j *HTTPProxyJob) Process() error {
	switch j.ClientType {
	case registry.ClientTypeHTTP:
		return j.processHTTPClient()
	case registry.ClientTypeTCP:
		return j.processTCPClient()
	default:
		return fmt.Errorf("unsupported client type")
	}
}

// Execute implements the worker.Job interface
func (j *HTTPProxyJob) Execute() error {
	return j.Process()
}

// processHTTPClient handles proxying to an HTTP client
func (j *HTTPProxyJob) processHTTPClient() error {
	// Implement HTTP client proxying logic
	// This would involve forwarding the request to the appropriate HTTP client
	log.Printf("Proxying HTTP request to client %s", j.ClientID)
	j.Response.WriteHeader(http.StatusNotImplemented)
	j.Response.Write([]byte("HTTP client proxying not yet implemented"))
	return nil
}

// processTCPClient handles proxying to a TCP client
func (j *HTTPProxyJob) processTCPClient() error {
	// Implement TCP client proxying logic
	log.Printf("Proxying HTTP request to TCP client %s", j.ClientID)
	j.Response.WriteHeader(http.StatusNotImplemented)
	j.Response.Write([]byte("TCP client proxying not yet implemented"))
	return nil
}

// StatusHandler provides server status information
func (f *HTTPFrontend) StatusHandler(w http.ResponseWriter, r *http.Request) {
	clients := f.registry.ListClients()
	clientCount := len(clients)
	
	// Enrich client information with additional details
	enrichedClients := make([]map[string]interface{}, 0, len(clients))
	for _, client := range clients {
		enrichedClient := map[string]interface{}{
			"id":                 client.ID,
			"type":               client.Type,
			"paths":              client.Paths,
			"tcp_port":           client.TCPPort,
			"active_tcp_conns":   client.ActiveTCPConnections,
			"last_heartbeat":     client.LastHeartbeat,
			"metadata":           client.Metadata,
		}
		enrichedClients = append(enrichedClients, enrichedClient)
	}

	status := map[string]interface{}{
		"server_status":       "running",
		"total_clients":       clientCount,
		"timestamp":           time.Now().UTC(),
		"registered_clients": enrichedClients,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

// ClientListHandler returns a list of registered clients
func (f *HTTPFrontend) ClientListHandler(w http.ResponseWriter, r *http.Request) {
	clients := f.registry.ListClients()
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(clients)
}

// HealthHandler provides health check endpoint
func (f *HTTPFrontend) HealthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	
	status := map[string]interface{}{
		"status": "healthy",
		"time": time.Now().UTC(),
	}
	
	json.NewEncoder(w).Encode(status)
}

// StartServer starts the HTTP frontend server
func (f *HTTPFrontend) StartServer(addr string) error {
	http.HandleFunc("/register", f.RegisterHandler)
	http.HandleFunc("/status", f.StatusHandler)
	http.HandleFunc("/health", f.HealthHandler)
	http.HandleFunc("/", f.ProxyHandler)
	
	return http.ListenAndServe(addr, nil)
}
