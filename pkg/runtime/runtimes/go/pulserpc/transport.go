package pulserpc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// Transport interface for RPC transports
type Transport interface {
	// Request sends a JSON-RPC request and returns the response
	Request(method string, params interface{}) (map[string]interface{}, error)
}

// HTTPTransport is an HTTP-based transport
type HTTPTransport struct {
	BaseURL string
	Headers map[string]string
	Client  *http.Client
}

// NewHTTPTransport creates a new HTTP transport
func NewHTTPTransport(baseURL string, headers map[string]string) *HTTPTransport {
	return &HTTPTransport{
		BaseURL: baseURL,
		Headers: headers,
		Client:  &http.Client{},
	}
}

// Request sends a JSON-RPC request via HTTP POST
func (t *HTTPTransport) Request(method string, params interface{}) (map[string]interface{}, error) {
	// Build request
	reqBody := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  method,
		"params":  params,
		"id":      1,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	httpReq, err := http.NewRequest("POST", t.BaseURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	httpReq.Header.Set("Content-Type", "application/json")
	for k, v := range t.Headers {
		httpReq.Header.Set(k, v)
	}

	// Send request
	resp, err := t.Client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Handle empty response (notification)
	if len(respBody) == 0 {
		return nil, fmt.Errorf("empty response")
	}

	// Parse JSON response
	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return result, nil
}

// InProcTransport is an in-process transport for testing
type InProcTransport struct {
	Server *Server
}

// NewInProcTransport creates a new in-process transport
func NewInProcTransport(server *Server) *InProcTransport {
	return &InProcTransport{
		Server: server,
	}
}

// Request sends a request directly to the server
func (t *InProcTransport) Request(method string, params interface{}) (map[string]interface{}, error) {
	req := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  method,
		"id":      1,
	}
	if params != nil {
		req["params"] = params
	}

	response := t.Server.Call(req)
	if response == nil {
		return nil, fmt.Errorf("empty response (notification?)")
	}

	return response, nil
}
