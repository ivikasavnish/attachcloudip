package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/vikasavn/attachcloudip/pkg/types"
)

var (
	clientRegistry = make(map[string]*ClientInfo)
	registryMutex  = &sync.RWMutex{}
	availablePorts = make(chan int, 100)
)

func init() {
	// Pre-populate available ports
	go func() {
		for port := 10000; port < 11000; port++ {
			availablePorts <- port
		}
	}()
}

func HomeHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Welcome to AttachCloudIP Server")
}

func RegisterHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse registration request
	var req struct {
		ClientID string   `json:"client_id"`
		Paths    []string `json:"paths"`
		Protocol string   `json:"protocol"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Generate client ID if not provided
	if req.ClientID == "" {
		req.ClientID = uuid.New().String()
	}

	// Allocate a port
	port := <-availablePorts

	// Create client info
	clientInfo := &ClientInfo{
		ID:                   req.ClientID,
		Type:                 req.Protocol,
		Paths:                req.Paths,
		LastHeartbeat:        time.Now(),
		TCPPort:              port,
		ActiveTCPConnections: 0,
		Status:               "active",
		Metadata:             map[string]string{},
	}

	// Store client in registry
	registryMutex.Lock()
	clientRegistry[req.ClientID] = clientInfo
	registryMutex.Unlock()

	// Send response
	resp := types.Response{
		RequestID:  uuid.New().String(),
		StatusCode: http.StatusCreated,
		ClientID:   req.ClientID,
		Port:       port,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(resp)

	log.Printf("Registered client: ID=%s, Paths=%v, Port=%d",
		req.ClientID, req.Paths, port)
}

func StatusHandler(w http.ResponseWriter, r *http.Request) {
	registryMutex.RLock()
	defer registryMutex.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(clientRegistry)
}

func ClientListHandler(w http.ResponseWriter, r *http.Request) {
	registryMutex.RLock()
	defer registryMutex.RUnlock()

	clients := make([]string, 0, len(clientRegistry))
	for id := range clientRegistry {
		clients = append(clients, id)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(clients)
}

func HealthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func handleTCPConnection(conn net.Conn) {
	defer conn.Close()

	decoder := json.NewDecoder(conn)
	encoder := json.NewEncoder(conn)

	// Read registration request
	var req types.Request
	if err := decoder.Decode(&req); err != nil {
		log.Printf("Failed to decode request: %v", err)
		return
	}

	if req.Type != types.RequestTypeRegister {
		log.Printf("Invalid request type: %v", req.Type)
		encoder.Encode(types.Response{Error: "Invalid request type"})
		return
	}

	// Generate client ID if not provided
	if req.ID == "" {
		req.ID = uuid.New().String()
	}

	// Allocate a port
	port := <-availablePorts

	// Create client info
	clientInfo := &ClientInfo{
		ID:                   req.ID,
		Type:                 "tcp",
		LastHeartbeat:        time.Now(),
		TCPPort:              port,
		ActiveTCPConnections: 0,
		Status:               "active",
		Metadata:             map[string]string{},
	}

	// Store client in registry
	registryMutex.Lock()
	clientRegistry[req.ID] = clientInfo
	registryMutex.Unlock()

	// Send response
	resp := types.Response{
		RequestID:  uuid.New().String(),
		StatusCode: http.StatusCreated,
		ClientID:   req.ID,
		Port:       port,
	}

	if err := encoder.Encode(resp); err != nil {
		log.Printf("Failed to send response: %v", err)
		return
	}

	log.Printf("Registered TCP client: ID=%s, Port=%d", req.ID, port)

	// Handle subsequent requests
	for {
		var req types.Request
		if err := decoder.Decode(&req); err != nil {
			if err != io.EOF {
				log.Printf("Failed to decode request: %v", err)
			}
			break
		}

		resp, err := handleRequest(clientInfo, &req)
		if err != nil {
			log.Printf("Failed to handle request: %v", err)
			break
		}

		if err := encoder.Encode(resp); err != nil {
			log.Printf("Failed to send response: %v", err)
			break
		}
	}

	// Clean up
	registryMutex.Lock()
	delete(clientRegistry, req.ID)
	registryMutex.Unlock()
	availablePorts <- port
}

func handleRequest(client *ClientInfo, req *types.Request) (*types.Response, error) {
	client.LastHeartbeat = time.Now()

	switch req.Type {
	case types.RequestTypeHTTP:
		// Forward HTTP request
		return &types.Response{
			RequestID:  req.ID,
			StatusCode: http.StatusOK,
			Body:      []byte("HTTP request handled"),
		}, nil

	case types.RequestTypeTCP:
		// Handle TCP request
		return &types.Response{
			RequestID:  req.ID,
			StatusCode: http.StatusOK,
			Body:      []byte("TCP request handled"),
		}, nil

	default:
		return nil, fmt.Errorf("unsupported request type: %v", req.Type)
	}
}
