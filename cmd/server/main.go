package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	"gopkg.in/yaml.v2"
)

type Config struct {
	Server struct {
		Host string `yaml:"host"`
		SSH  struct {
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

type ClientInfo struct {
	ID                   string
	Type                 string
	Paths                []string
	LastHeartbeat        time.Time
	TCPPort              int
	ActiveTCPConnections int
	Status               string
	Metadata             map[string]string
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

func startTCPServer(port int) {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		log.Fatalf("Failed to start TCP server: %v", err)
	}
	defer listener.Close()

	log.Printf("TCP server listening on port %d", port)

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("Failed to accept connection: %v", err)
			continue
		}

		go handleTCPConnection(conn)
	}
}

func main() {
	configPath := flag.String("config", "", "Path to configuration file")
	httpPort := flag.String("port", "9999", "HTTP port to run the server on")
	flag.Parse()

	var httpPortNum int
	var regPortNum int

	if *configPath != "" {
		config, err := loadConfig(*configPath)
		if err != nil {
			log.Fatalf("Failed to load config: %v", err)
		}
		httpPortNum = config.Server.Ports.HTTP
		regPortNum = config.Server.Ports.Registration
		log.Printf("Loaded configuration from %s", *configPath)
	} else {
		fmt.Sscanf(*httpPort, "%d", &httpPortNum)
		regPortNum = httpPortNum + 1
	}

	// Start TCP server for registration in a goroutine
	go startTCPServer(regPortNum)

	// Set up HTTP routes
	http.HandleFunc("/", HomeHandler)
	http.HandleFunc("/status", StatusHandler)
	http.HandleFunc("/clients", ClientListHandler)
	http.HandleFunc("/health", HealthHandler)
	http.HandleFunc("/register", RegisterHandler)

	// Start HTTP server
	log.Printf("HTTP server listening on port %d", httpPortNum)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", httpPortNum), nil))
}
