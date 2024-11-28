package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"bytes"
	"os"
	"sync"
	"time"
	"gopkg.in/yaml.v2"

	"github.com/google/uuid"
	"github.com/vikasavn/attachcloudip/pkg/types"
)

type Config struct {
	Server struct {
		Host   string `yaml:"host"`
		SSH    struct {
			Port     int    `yaml:"port"`
			Username string `yaml:"username"`
			KeyPath  string `yaml:"key_path"`
		} `yaml:"ssh"`
		Ports struct {
			HTTP         int `yaml:"http"`
			GRPC         int `yaml:"grpc"`
			Registration int `yaml:"registration"`
		} `yaml:"ports"`
	} `yaml:"server"`
}

type Client struct {
	ID      string
	TCPConn net.Conn
	TCPPort int
	mu      sync.Mutex
	encoder *json.Encoder
	decoder *json.Decoder
}

func loadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

func (c *Client) SendRequest(req *types.Request) (*types.Response, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Add request metadata
	if req.ID == "" {
		req.ID = uuid.New().String()
	}
	req.Timestamp = time.Now().Unix()

	if err := c.encoder.Encode(req); err != nil {
		return nil, fmt.Errorf("failed to send request: %v", err)
	}

	var resp types.Response
	if err := c.decoder.Decode(&resp); err != nil {
		return nil, fmt.Errorf("failed to read response: %v", err)
	}

	return &resp, nil
}

func (c *Client) StartHeartbeat(interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for range ticker.C {
			req := &types.Request{
				Type: types.RequestTypeHTTP,
				Path: "/health",
			}
			if _, err := c.SendRequest(req); err != nil {
				log.Printf("❌ Heartbeat failed: %v", err)
			}
		}
	}()
}

func main() {
	configPath := flag.String("config", "", "Path to configuration file")
	clientID := flag.String("id", "", "Client ID (optional)")
	serverAddr := flag.String("server", "localhost:8080", "Server address")
	port := flag.Int("port", 0, "Direct port number for local testing (overrides config and server)")
	flag.Parse()

	if *clientID == "" {
		*clientID = uuid.New().String()
	}

	var httpHost string
	var httpPort int

	if *port != 0 {
		// Direct port mode for local testing
		httpHost = "localhost"
		httpPort = *port
		log.Printf("Using direct port %d for local testing", *port)
	} else if *configPath != "" {
		config, err := loadConfig(*configPath)
		if err != nil {
			log.Fatalf("Failed to load config: %v", err)
		}
		httpHost = config.Server.Host
		httpPort = config.Server.Ports.HTTP
		log.Printf("Loaded configuration from %s", *configPath)
	} else {
		// Parse server address if no config file
		host, port, err := net.SplitHostPort(*serverAddr)
		if err != nil {
			log.Fatalf("Invalid server address: %v", err)
		}
		httpHost = host
		fmt.Sscanf(port, "%d", &httpPort)
	}

	// First register via HTTP
	regReq := struct {
		ClientID string   `json:"client_id"`
		Paths    []string `json:"paths"`
		Protocol string   `json:"protocol"`
	}{
		ClientID: *clientID,
		Paths:    []string{"/"},
		Protocol: "tcp",
	}

	regBody, err := json.Marshal(regReq)
	if err != nil {
		log.Fatalf("Failed to marshal registration request: %v", err)
	}

	httpURL := fmt.Sprintf("http://%s:%d/register", httpHost, httpPort)
	log.Printf("Registering with server at %s", httpURL)
	
	resp, err := http.Post(httpURL, "application/json", bytes.NewReader(regBody))
	if err != nil {
		log.Fatalf("Failed to register with server: %v", err)
	}
	defer resp.Body.Close()

	var regResp types.Response
	if err := json.NewDecoder(resp.Body).Decode(&regResp); err != nil {
		log.Fatalf("Failed to decode registration response: %v", err)
	}

	if regResp.Error != "" {
		log.Fatalf("Registration failed: %s", regResp.Error)
	}

	log.Printf("✅ Successfully registered with server")
	log.Printf("🔌 Assigned TCP port: %d", regResp.Port)

	// Connect to the assigned TCP port
	tcpAddr := fmt.Sprintf("%s:%d", httpHost, regResp.Port)
	log.Printf("Connecting to TCP server at %s", tcpAddr)
	
	conn, err := net.Dial("tcp", tcpAddr)
	if err != nil {
		log.Fatalf("Failed to connect to TCP port: %v", err)
	}
	defer conn.Close()

	// Create client
	client := &Client{
		ID:      *clientID,
		TCPConn: conn,
		TCPPort: regResp.Port,
		encoder: json.NewEncoder(conn),
		decoder: json.NewDecoder(conn),
	}

	// Send TCP registration request
	tcpRegReq := &types.Request{
		ID:   *clientID,
		Type: types.RequestTypeRegister,
	}

	tcpRegResp, err := client.SendRequest(tcpRegReq)
	if err != nil {
		log.Fatalf("Failed to register TCP connection: %v", err)
	}

	if tcpRegResp.Error != "" {
		log.Fatalf("TCP registration failed: %s", tcpRegResp.Error)
	}

	log.Printf("✅ TCP connection established")

	// Start heartbeat
	client.StartHeartbeat(30 * time.Second)

	// Example: Send an HTTP request through the server
	req := &types.Request{
		Type:   types.RequestTypeHTTP,
		Path:   "https://api.example.com/data",
		Method: "GET",
		Headers: map[string]string{
			"Accept": "application/json",
		},
	}

	respitem, err := client.SendRequest(req)
	if err != nil {
		log.Printf("❌ Request failed: %v", err)
		return
	}
	if respitem.StatusCode >= 400 {
		log.Printf("❌ Request failed with status code %d: %s",
			respitem.StatusCode, string(respitem.Body))
	} else {
		log.Printf("✅ Response received: status=%d, body=%s",
			respitem.StatusCode, string(respitem.Body))
	}

	// Keep the connection alive
	select {}
}
