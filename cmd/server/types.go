package main

type Client struct {
	ClientId string   `json:"client_id"`
	Paths    []string `json:"paths"`
	Protocol string   `json:"protocol"`
}

type ClientList struct {
	Clients []Client `json:"clients"`
}

type ClientRegisterRequest struct {
	ClientId string   `json:"client_id"`
	Paths    []string `json:"paths"`
	Protocol string   `json:"protocol"`
}

// ServerConfig represents the configuration for the server
type ServerConfig struct {
	Host string `yaml:"host"`
	SSH  struct {
		Port     int    `yaml:"port"`
		Username string `yaml:"username"`
		KeyPath  string `yaml:"key_path"`
	} `yaml:"ssh"`
	Ports struct {
		HTTP         int `yaml:"http"`
		Grpc         int `yaml:"grpc"`
		Registration int `yaml:"registration"`
	} `yaml:"ports"`
	Routing struct {
		PathMatching struct {
			CaseSensitive bool   `yaml:"case_sensitive"`
			TrailingSlash string `yaml:"trailing_slash"`
		} `yaml:"path_matching"`
		Paths []struct {
			Pattern      string `yaml:"pattern"`
			Description  string `yaml:"description"`
			RequiredAuth bool   `yaml:"required_auth"`
		} `yaml:"paths"`
	} `yaml:"routing"`
}

// ClientConfig represents the configuration for the client
type ClientConfig struct {
	ID    string `yaml:"id"`
	Ports struct {
		HTTPStart   int `yaml:"http_start"`
		TunnelStart int `yaml:"tunnel_start"`
	} `yaml:"ports"`
	Registration struct {
		RetryInterval int `yaml:"retry_interval"`
		Timeout       int `yaml:"timeout"`
		Paths         []struct {
			Path        string `yaml:"path"`
			Description string `yaml:"description"`
			Metadata    []struct {
				Version  string `yaml:"version"`
				Provider string `yaml:"provider"`
			} `yaml:"metadata,omitempty"`
		} `yaml:"paths"`
	} `yaml:"registration"`
	Heartbeat struct {
		Interval int `yaml:"interval"`
		Timeout  int `yaml:"timeout"`
	} `yaml:"heartbeat"`
}

// Config represents the overall configuration
// containing both server and client configurations
type Config struct {
	Server ServerConfig `yaml:"server"`
	Client ClientConfig `yaml:"client"`
}

type ClientRegisterResponse struct {
	Client *Client `json:"client"`
	Port   []int   `json:"port"`
}
