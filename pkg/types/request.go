package types

type RequestType string

const (
	RequestTypeHTTP     RequestType = "http"
	RequestTypeTCP      RequestType = "tcp"
	RequestTypeRegister RequestType = "register"
)

type Request struct {
	ID          string      `json:"id"`
	Type        RequestType `json:"type"`
	Path        string      `json:"path"`
	Method      string      `json:"method"`
	Headers     map[string]string `json:"headers"`
	Body        []byte      `json:"body"`
	Timestamp   int64       `json:"timestamp"`
}

type Response struct {
	RequestID   string            `json:"request_id"`
	StatusCode  int               `json:"status_code"`
	Headers     map[string]string `json:"headers"`
	Body        []byte           `json:"body"`
	Error       string           `json:"error,omitempty"`
	Timestamp   int64            `json:"timestamp"`
	Port        int              `json:"port,omitempty"`      // TCP port for client connections
	ClientID    string           `json:"client_id,omitempty"` // Assigned client ID
}
