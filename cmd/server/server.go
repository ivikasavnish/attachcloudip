package main

import (
	"fmt"
	"log"
	"net/http"
)

// Server represents a server with its configuration
type Server struct {
	config *Config
}

// NewServer creates a new Server instance with the provided configuration
func NewServer(config *Config) *Server {
	return &Server{
		config: config,
	}
}

// Start begins the server operations
func (s *Server) Start() error {
	address := fmt.Sprintf("%s:%d", s.config.Server.Host, s.config.Server.Ports.HTTP)
	log.Printf("[SERVER] Starting server on %s", address)
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})
	http.HandleFunc("/register", RegisterClient)
	http.HandleFunc("/healthz", HealthCheck)
	log.Fatal(http.ListenAndServe(address, nil))

	return nil
}
