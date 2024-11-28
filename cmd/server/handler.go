package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

func HealthCheck(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func NotFound(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNotFound)
	w.Write([]byte("Not Found"))
}

func RegisterClient(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var request struct {
		ClientID string   `json:"client_id"`
		Paths    []string `json:"paths"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, fmt.Sprintf("Failed to decode request: %v", err), http.StatusBadRequest)
		return
	}

	log.Printf("Received registration request for client %s with paths: %v", request.ClientID, request.Paths)

	// Return TCP port for client connection
	response := struct {
		Port []int `json:"port"`
	}{
		Port: tcpmanager.Ports,
	}

	// Store the client paths for later use
	// Use first path for now
	clientManager.RegisterClient(&Client{
		ClientId: request.ClientID,
		Paths:    request.Paths,
	})

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

type ClientResponse struct {
	ID         string    `json:"id"`
	Path       string    `json:"path"`
	LastActive time.Time `json:"last_active"`
}

func ListClients(w http.ResponseWriter, r *http.Request) {
	clients := tcpmanager.GetClients()
	response := make([]ClientResponse, 0, len(clients))

	for _, client := range clients {
		response = append(response, ClientResponse{
			ID:         client.clientID,
			Path:       client.path,
			LastActive: client.lastActive,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
