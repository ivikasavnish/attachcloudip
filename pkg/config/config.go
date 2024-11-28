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

type PortConfig struct {
	HTTP         int `yaml:"http"`
	GRPC         int `yaml:"grpc"`
	Registration int `yaml:"registration"`
}

type ServerConfig struct {
	Host  string     `yaml:"host"`
	SSH   SSHConfig  `yaml:"ssh"`
	Ports PortConfig `yaml:"ports"`
}

type ClientConfig struct {
	ID string
}

type Config struct {
	Server           ServerConfig      `yaml:"server"`
	Client           ClientConfig      `yaml:"client"`
	ConnectionOpts   ConnectionOptions `yaml:"connection_opts"`
}

type ConnectionOptionsProtocol int

const (
	ConnectionOptionsProtocol_HTTP ConnectionOptionsProtocol = iota
	ConnectionOptionsProtocol_TCP
)

type ConnectionOptions struct {
	Protocol          ConnectionOptionsProtocol
	BufferSize        int
	KeepAlive         bool
	KeepAliveInterval int
	IdleTimeout       int
}

// GetHTTPServerURL returns the HTTP server URL
func (c *Config) GetHTTPServerURL() string {
	return fmt.Sprintf("http://%s:%d", c.Server.Host, c.Server.Ports.HTTP)
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
