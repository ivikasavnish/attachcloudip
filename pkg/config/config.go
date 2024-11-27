package config

import (
	"fmt"
	"io/ioutil"

	"gopkg.in/yaml.v2"
)

type SSHConfig struct {
	Port     int    `yaml:"port"`
	Username string `yaml:"username"`
	KeyPath  string `yaml:"key_path"`
}

type ServerPortConfig struct {
	HTTP         int `yaml:"http"`         // External HTTP API port
	GRPC         int `yaml:"grpc"`         // Internal gRPC communication port
	Registration int `yaml:"registration"` // TCP registration port
	StartPort    int `yaml:"start_port"`   // Starting port for dynamic port allocation
}

type ClientPortConfig struct {
	HTTPStart   int `yaml:"http_start"`   // Starting port for HTTP server
	TunnelStart int `yaml:"tunnel_start"` // Starting port for SSH tunnel
}

type RegistrationConfig struct {
	RetryInterval int `yaml:"retry_interval"` // Seconds between registration attempts
	Timeout       int `yaml:"timeout"`        // Registration timeout in seconds
}

type HeartbeatConfig struct {
	Interval int `yaml:"interval"` // Heartbeat interval in seconds
	Timeout  int `yaml:"timeout"`  // Heartbeat timeout in seconds
}

type ServerConfig struct {
	Host  string           `yaml:"host"`
	SSH   SSHConfig        `yaml:"ssh"`
	Ports ServerPortConfig `yaml:"ports"`
}

type ClientConfig struct {
	ID           string             `yaml:"id"`         // Client identifier
	Path         string             `yaml:"path"`       // Path to handle requests for
	TargetURL    string             `yaml:"target_url"` // URL to forward requests to
	Ports        ClientPortConfig   `yaml:"ports"`
	Registration RegistrationConfig `yaml:"registration"`
	Heartbeat    HeartbeatConfig    `yaml:"heartbeat"`
}

type Config struct {
	Server ServerConfig `yaml:"server"`
	Client ClientConfig `yaml:"client"`
}

func LoadConfig(path string) (*Config, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %v", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %v", err)
	}

	return &config, nil
}

// GetHTTPServerURL returns the HTTP server URL
func (c *Config) GetHTTPServerURL() string {
	return fmt.Sprintf("http://%s:%d", c.Server.Host, c.Server.Ports.HTTP)
}

// GetGRPCServerURL returns the gRPC server URL
func (c *Config) GetGRPCServerURL() string {
	return fmt.Sprintf("%s:%d", c.Server.Host, c.Server.Ports.GRPC)
}

// GetRegistrationServerURL returns the registration server URL
func (c *Config) GetRegistrationServerURL() string {
	return fmt.Sprintf("%s:%d", c.Server.Host, c.Server.Ports.Registration)
}

// GetSSHServerURL returns the SSH server URL
func (c *Config) GetSSHServerURL() string {
	return fmt.Sprintf("%s:%d", c.Server.Host, c.Server.SSH.Port)
}
