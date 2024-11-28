package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

func init() {
	log.SetFlags(log.Llongfile)
}

type Client struct {
	ID         string
	TCPConn    net.Conn
	TCPPort    int
	serverHost string
	path       string
}

func registerClient(serverAddr string, clientID string, path string) (*Client, error) {
	// Prepare registration request
	registrationPayload := struct {
		ClientID string   `json:"client_id"`
		Paths    []string `json:"paths"`
	}{
		ClientID: clientID,
		Paths:    []string{path},
	}

	payloadBytes, err := json.Marshal(registrationPayload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal registration payload: %v", err)
	}

	// Send registration request
	resp, err := http.Post(fmt.Sprintf("http://%s/register", serverAddr),
		"application/json", bytes.NewBuffer(payloadBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to send registration request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("registration failed with status: %d", resp.StatusCode)
	}

	// Parse registration response
	var regResponse struct {
		Port []int `json:"port"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&regResponse); err != nil {
		return nil, fmt.Errorf("failed to decode registration response: %v", err)
	}

	// Extract TCP port from JSON response
	if len(regResponse.Port) == 0 {
		return nil, fmt.Errorf("no TCP port received from server")
	}
	tcpPort := regResponse.Port[0]
	log.Printf("Received TCP port: %+v", regResponse)

	// Extract host from serverAddr
	host, _, err := net.SplitHostPort(serverAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse server address: %v", err)
	}
	log.Printf("Received Host: %+v", host)

	client := &Client{
		ID:         clientID,
		TCPPort:    tcpPort,
		serverHost: host,
		path:       path,
	}

	return client, nil
}

func (c *Client) ConnectTCP() error {
	conn, err := net.Dial("tcp", fmt.Sprintf("%s:%d", c.serverHost, c.TCPPort))
	if err != nil {
		return fmt.Errorf("failed to connect to TCP server: %v", err)
	}
	c.TCPConn = conn

	// Send initial registration message with client ID and path
	registrationMsg := fmt.Sprintf("%s|%s\n", c.ID, c.path)
	log.Println(registrationMsg)
	if _, err := c.TCPConn.Write([]byte(registrationMsg)); err != nil {
		return fmt.Errorf("failed to send registration message: %v", err)
	}
	log.Println("Registration message sent and waiting for confirmation...")

	go func() {
		// Wait for registration confirmation
		buf := make([]byte, 1024)
		n, err := c.TCPConn.Read(buf)
		if err != nil {
			fmt.Printf("failed to read registration confirmation: %v", err)
			return
		}

		response := strings.TrimSpace(string(buf[:n]))
		if response != "registered" {
			fmt.Printf("unexpected registration response: %s", response)
			return
		}

		log.Printf("Successfully registered with server")
	}()
	return nil
}

func (c *Client) sendMessage(message string) error {
	_, err := c.TCPConn.Write([]byte(message + "\n"))
	return err
}

func (c *Client) receiveMessage() (string, error) {
	buf := make([]byte, 1024)
	n, err := c.TCPConn.Read(buf)
	if err != nil {
		return "", err
	}
	return string(buf[:n]), nil
}

func (c *Client) receiveMessages() {
	for {
		message, err := c.receiveMessage()
		if err != nil {
			log.Printf("Failed to receive message: %v", err)
			return
		}

		message = strings.TrimSpace(message)

		// Handle heartbeat acknowledgment
		if message == "heartbeat-ack" {
			log.Printf("Received heartbeat acknowledgment from server")
			continue
		}

		log.Printf("Received message: '%s'", message)
	}
}

func (c *Client) startHeartbeat(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for range ticker.C {
		log.Printf("Sending heartbeat...")
		if err := c.sendMessage("heartbeat"); err != nil {
			log.Printf("Failed to send heartbeat: %v", err)
			return
		}
	}
}

func main() {
	// Command line flags
	serverAddr := flag.String("server", "localhost:9999", "Server address")
	watchPath := flag.String("path", "", "Path to watch for changes")
	flag.Parse()

	if *watchPath == "" {
		log.Fatal("Path is required. Use -path flag to specify the path to watch")
	}

	// Generate a unique client ID
	clientID := uuid.New().String()
	log.Printf("Generated client ID: %s", clientID)

	client, err := registerClient(*serverAddr, clientID, *watchPath)
	if err != nil {
		log.Fatalf("Failed to register client: %v", err)
	}

	log.Println("connecting to TCP server...")
	if err := client.ConnectTCP(); err != nil {
		log.Printf("failed to connect to TCP server: %v", err)
		return
	}
	log.Printf("Received client: %+v", client)
	log.Printf("Client registered with ID: %s", client.ID)

	log.Println("Client started")

	// Start receiving messages in a separate goroutine
	go client.receiveMessages()

	// Start heartbeat in a separate goroutine
	go client.startHeartbeat(2 * time.Second)

	// Keep the main function running
	select {}
}
