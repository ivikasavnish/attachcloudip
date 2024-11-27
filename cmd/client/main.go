package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/vikasavn/attachcloudip/pkg/config"
	"github.com/vikasavn/attachcloudip/pkg/service"
)

type Logger struct {
	*log.Logger
}

func (l *Logger) Printf(format string, v ...interface{}) {
	_, file, line, _ := runtime.Caller(1)
	l.Logger.Printf(fmt.Sprintf("%s:%d: %s", file, line, format), v...)
}

func (l *Logger) Fatalf(format string, v ...interface{}) {
	_, file, line, _ := runtime.Caller(1)
	l.Logger.Fatalf(fmt.Sprintf("%s:%d: %s", file, line, format), v...)
}

var logger = &Logger{Logger: log.New(os.Stdout, "", log.LstdFlags)}

var (
	configFile = flag.String("config", "config/tunnel.yaml", "Path to config file")
	numWorkers = flag.Int("workers", runtime.NumCPU(), "Number of workers")
	queueSize  = flag.Int("queue", 100, "Job queue size")
)

func main() {
	flag.Parse()

	paths := flag.Args()
	if len(paths) == 0 {
		logger.Fatal("No paths specified. Usage: client -config <config-file> <paths>")
	}

	// Load configuration
	cfg, err := config.LoadConfig(*configFile)
	if err != nil {
		logger.Fatalf("Failed to load config: %v", err)
	}

	// Generate unique client ID
	cfg.Client.ID = uuid.New().String()

	// Create tunnel service
	tunnelService := service.NewTunnelService([]int{}) // Client doesn't need to pre-allocate ports

	// Prepare connection options
	connOpts := &service.ConnectionOptions{
		Protocol:          service.ConnectionOptionsProtocol_HTTP,
		BufferSize:        32 * 1024, // 32KB buffer
		KeepAlive:         true,
		KeepAliveInterval: 30,
		IdleTimeout:       300,
	}

	// Create main context and cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Register client
	logger.Printf("Registering client with ID %s for paths: %v", cfg.Client.ID, paths)
	regResp, err := tunnelService.Register(ctx, &service.StreamRequest{
		Type:      service.StreamRequestType_PATH_UPDATE,
		RequestId: cfg.Client.ID,
		Protocol:  "tcp",
		HttpRequest: &service.HttpRequest{
			Path: paths[0],
		},
	})
	if err != nil {
		logger.Fatalf("Failed to register: %v", err)
	}

	logger.Printf("Registration successful - Session ID: %s, Port: %d", regResp.RequestId, regResp.Port)

	// Complete registration handshake
	if err := completeHandshake(cfg.Client.ID, regResp.Port); err != nil {
		logger.Fatalf("Handshake failed: %v", err)
	}

	// Start heartbeat goroutine
	go func() {
		ticker := time.NewTicker(time.Duration(connOpts.KeepAliveInterval) * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := sendHeartbeat(cfg.Client.ID); err != nil {
					logger.Printf("Failed to send heartbeat: %v", err)
				}
			}
		}
	}()

	// Wait for interrupt signal
	<-sigChan
	logger.Println("Shutdown signal received, closing connection...")
	cancel()

	logger.Println("Client shutdown complete")
}

func completeHandshake(clientID string, port int) error {
	// Connect to the assigned port
	conn, err := net.Dial("tcp", fmt.Sprintf("localhost:%d", port))
	if err != nil {
		return fmt.Errorf("failed to connect for handshake: %w", err)
	}
	defer conn.Close()

	// Send client ID as handshake
	_, err = conn.Write([]byte(clientID))
	if err != nil {
		return fmt.Errorf("failed to send handshake: %w", err)
	}

	return nil
}

func sendHeartbeat(clientID string) error {
	// Implementation of heartbeat
	// This would typically make an HTTP request to the server's heartbeat endpoint
	return nil
}
