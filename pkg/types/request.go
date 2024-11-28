package types

import (
	"net/http"
)

type RequestType string

const (
	RequestTypeHTTP       RequestType = "http"
	RequestTypeTCP        RequestType = "tcp"
	RequestTypeRegister   RequestType = "register"
	RegisterRequest       RequestType = "register"
	HeartbeatRequest      RequestType = "heartbeat"
	ProxyRequest          RequestType = "proxy"
	PortAllocationRequest RequestType = "port_allocation"
)

type Request struct {
	ID          string            `json:"id"`
	Type        RequestType       `json:"type"`
	Path        string            `json:"path"`
	Method      string            `json:"method"`
	Headers     http.Header       `json:"headers"`
	Body        []byte            `json:"body"`
	Timestamp   int64             `json:"timestamp"`
	QueryParams map[string]string `json:"query_params,omitempty"`
	Host        string            `json:"host,omitempty"`
	Protocol    string            `json:"protocol,omitempty"`
	ClientID    string            `json:"client_id,omitempty"`
	Payload     interface{}       `json:"payload"`
}

type PortAllocationPayload struct {
	ClientID string `json:"client_id"`
	Action   string `json:"action"` // "allocate" or "release"
	Port     int    `json:"port,omitempty"`
}

type Response struct {
	RequestID   string      `json:"request_id"`
	StatusCode  int         `json:"status_code"`
	Headers     http.Header `json:"headers"`
	Body        []byte      `json:"body"`
	Error       string      `json:"error,omitempty"`
	Timestamp   int64       `json:"timestamp"`
	Port        int         `json:"port,omitempty"`      // TCP port for client connections
	ClientID    string      `json:"client_id,omitempty"` // Assigned client ID
	Protocol    string      `json:"protocol,omitempty"`
	ContentType string      `json:"content_type,omitempty"`
}

type Worker interface {
	ProcessRequest(req *Request) (*Response, error)
}

type HTTPWorker struct {
	Client *http.Client
}

type GRPCWorker struct {
	// Add gRPC client configuration here
}

type WorkerPool struct {
	Workers map[RequestType]Worker
}
