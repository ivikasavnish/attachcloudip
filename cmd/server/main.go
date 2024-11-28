package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
)

var (
	HTTPPort      = 9999
	TCPPort       = 9998
	tcpmanager    = NewTCPManager()
	clientManager = NewClientManager()
)

func init() {
	// Set up logging with file name and line number
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
}

// loadConfig loads the configuration from a YAML file

func main() {
	log.Println("Starting server...")

	// Start TCP listener on port 8080
	if err := tcpmanager.StartListener(TCPPort); err != nil {
		log.Fatalf("Failed to start TCP listener: %v", err)
	}
	log.Println("TCP Listener started on port", TCPPort)

	// Start handling TCP connections in a goroutine
	go func() {
		log.Println("Starting TCP connection handler...")
		tcpmanager.HandleIncomingRequests()
	}()

	log.Printf("HTTP Server starting on port %d...", HTTPPort)
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})
	http.HandleFunc("/register", RegisterClient)
	http.HandleFunc("/healthz", HealthCheck)
	http.HandleFunc("/clients", ListClients) // Add new route for listing clients

	if err := http.ListenAndServe(fmt.Sprintf(":%d", HTTPPort), nil); err != nil {
		log.Fatalf("Failed to start HTTP server: %v", err)
		os.Exit(1)
	}
}
