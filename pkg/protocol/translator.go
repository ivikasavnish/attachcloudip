package protocol

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/vikasavn/attachcloudip/pkg/types"
)

// HTTPToTCPRequest converts an HTTP request to our internal TCP request format
func HTTPToTCPRequest(r *http.Request, clientID string) (*types.Request, error) {
	// Read body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read request body: %v", err)
	}
	defer r.Body.Close()

	// Convert headers
	headers := make(http.Header)
	for key, values := range r.Header {
		headers[key] = values
	}

	// Convert query parameters
	queryParams := make(map[string]string)
	for key, values := range r.URL.Query() {
		if len(values) > 0 {
			queryParams[key] = values[0]
		}
	}

	// Create TCP request
	tcpReq := &types.Request{
		Type:        types.RequestTypeHTTP,
		Path:        r.URL.Path,
		Method:      r.Method,
		Headers:     headers,
		Body:        body,
		Timestamp:   time.Now().Unix(),
		QueryParams: queryParams,
		Host:        r.Host,
		Protocol:    r.Proto,
		ClientID:    clientID,
	}

	return tcpReq, nil
}

// TCPToHTTPResponse converts our internal TCP response to an HTTP response
func TCPToHTTPResponse(tcpResp *types.Response, w http.ResponseWriter) error {
	// Set headers
	for key, values := range tcpResp.Headers {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}

	// Set content type if provided
	if tcpResp.ContentType != "" {
		w.Header().Set("Content-Type", tcpResp.ContentType)
	}

	// Set content length
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(tcpResp.Body)))

	// Set status code
	w.WriteHeader(tcpResp.StatusCode)

	// Write body
	if len(tcpResp.Body) > 0 {
		if _, err := w.Write(tcpResp.Body); err != nil {
			return fmt.Errorf("failed to write response body: %v", err)
		}
	}

	return nil
}

// HTTPResponseToTCP converts an HTTP response to our internal TCP response format
func HTTPResponseToTCP(httpResp *http.Response, requestID string) (*types.Response, error) {
	// Read body
	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}
	defer httpResp.Body.Close()

	// Create TCP response
	tcpResp := &types.Response{
		RequestID:   requestID,
		StatusCode:  httpResp.StatusCode,
		Headers:     httpResp.Header,
		Body:        body,
		Timestamp:   time.Now().Unix(),
		Protocol:    httpResp.Proto,
		ContentType: httpResp.Header.Get("Content-Type"),
	}

	return tcpResp, nil
}

// TCPToHTTPRequest converts our internal TCP request to an HTTP request
func TCPToHTTPRequest(tcpReq *types.Request) (*http.Request, error) {
	// Create URL with query parameters
	url := tcpReq.Path
	if len(tcpReq.QueryParams) > 0 {
		first := true
		for key, value := range tcpReq.QueryParams {
			if first {
				url += "?"
				first = false
			} else {
				url += "&"
			}
			url += fmt.Sprintf("%s=%s", key, value)
		}
	}

	// Create HTTP request
	req, err := http.NewRequest(
		tcpReq.Method,
		url,
		bytes.NewReader(tcpReq.Body),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %v", err)
	}

	// Set headers
	req.Header = tcpReq.Headers

	// Set host if provided
	if tcpReq.Host != "" {
		req.Host = tcpReq.Host
	}

	return req, nil
}
