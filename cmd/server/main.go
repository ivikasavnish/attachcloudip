package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/vikasavn/attachcloudip/pkg/config"
	"github.com/vikasavn/attachcloudip/pkg/httpfrontend"
	"github.com/vikasavn/attachcloudip/pkg/registry"
	"github.com/vikasavn/attachcloudip/pkg/tcpmanager"
	"github.com/vikasavn/attachcloudip/pkg/worker"
)

func main() {
	// Configure logging
	logger := log.New(os.Stdout, "server: ", log.LstdFlags|log.Lshortfile)

	// Parse command-line flags
	configFile := flag.String("config", "config/tunnel.yaml", "Path to config file")
	numWorkers := flag.Int("workers", 10, "Number of worker goroutines")
	queueSize := flag.Int("queue", 100, "Job queue size")
	flag.Parse()

	// Load configuration
	cfg, err := config.LoadConfig(*configFile)
	if err != nil {
		logger.Fatalf("Failed to load config: %v", err)
	}

	// Create context for graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(),
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT)
	defer stop()

	// Create worker dispatcher
	dispatcher := worker.NewDispatcher(*numWorkers, *queueSize)
	dispatcher.Start()
	defer dispatcher.Stop()

	// Create registry starting from the registration port in config
	reg := registry.NewRegistry(cfg.Server.Ports.Registration)

	// Start heartbeat monitor
	reg.StartHeartbeatMonitor(30 * time.Second)
	logger.Println("Started heartbeat monitor with 30s interval")

	// Create TCP manager
	tcpMgr := tcpmanager.NewTCPManager(reg, dispatcher)

	// Start TCP listener on registration port
	logger.Printf("Starting TCP listener on registration port %d", cfg.Server.Ports.Registration)
	_, err = tcpMgr.StartOrReuseListener(cfg.Server.Ports.Registration)
	if err != nil {
		logger.Printf("Warning: Failed to start TCP listener on registration port %d: %v", cfg.Server.Ports.Registration, err)
	} else {
		logger.Printf("Successfully started TCP listener on port %d", cfg.Server.Ports.Registration)
	}

	// Pre-allocate additional ports for the port inventory
	logger.Println("Pre-allocating TCP ports for inventory")
	startPort := cfg.Server.Ports.Registration + 1
	allocatedPorts := []int{cfg.Server.Ports.Registration}  // Start with registration port
	
	for portsAllocated := 0; portsAllocated < 9; { // We already have one port, so allocate 9 more
		// Skip the HTTP and gRPC ports
		if startPort == cfg.Server.Ports.HTTP || startPort == cfg.Server.Ports.GRPC {
			startPort++
			continue
		}
		
		port, err := tcpMgr.GetAvailablePort(startPort)
		if err != nil {
			logger.Printf("Warning: Failed to pre-allocate port starting from %d: %v", startPort, err)
			startPort++
			continue
		}
		
		_, err = tcpMgr.StartOrReuseListener(port)
		if err != nil {
			logger.Printf("Warning: Failed to start listener on pre-allocated port %d: %v", port, err)
			startPort = port + 1
			continue
		} else {
			logger.Printf("Pre-allocated and started TCP listener on port %d", port)
			allocatedPorts = append(allocatedPorts, port)
			portsAllocated++
			startPort = port + 1
		}
	}

	logger.Printf("Successfully pre-allocated %d ports: %v", len(allocatedPorts), allocatedPorts)

	// Create HTTP frontend with allocated ports
	httpFrontend := httpfrontend.NewHTTPFrontend(reg, tcpMgr, dispatcher, allocatedPorts)

	// Prepare servers
	var wg sync.WaitGroup

	// HTTP Server
	httpServer := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Server.Ports.HTTP),
		Handler: nil, // Use default mux
	}

	// Start HTTP server
	wg.Add(1)
	go func() {
		defer wg.Done()
		logger.Printf("Starting HTTP server on %s", httpServer.Addr)
		if err := httpFrontend.StartServer(httpServer.Addr); err != nil && err != http.ErrServerClosed {
			logger.Printf("HTTP server error: %v", err)
		}
	}()

	// Wait for shutdown signal
	<-ctx.Done()
	logger.Println("Shutdown signal received, gracefully shutting down...")

	// Create shutdown context with timeout
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Gracefully shutdown HTTP server
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		logger.Printf("HTTP server shutdown error: %v", err)
	}

	// Close all TCP connections
	tcpMgr.CloseAllConnections()

	// Stop dispatcher
	dispatcher.Stop()

	// Wait for all goroutines to finish
	wg.Wait()

	logger.Println("Server shutdown complete")
}
