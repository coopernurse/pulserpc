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

// RequestWithID is an optional capability implemented by transports that let
// the caller supply the JSON-RPC request id. A nil id denotes a notification:
// the id member must be omitted and no response is expected. Transports that
// do not implement it keep working through the legacy Request method.
type RequestWithID interface {
	RequestWithID(method string, params interface{}, id interface{}) (map[string]interface{}, error)
}

// idsEqual compares a request id with the one echoed in a response. JSON
// decoding turns numbers into float64, so numeric ids must be compared by
// value rather than by Go type.
func idsEqual(sent, received interface{}) bool {
	if sent == nil || received == nil {
		return sent == nil && received == nil
	}
	if s, ok := sent.(string); ok {
		r, _ := received.(string)
		return r == s
	}
	sv, sok := numericID(sent)
	rv, rok := numericID(received)
	return sok && rok && sv == rv
}

func numericID(value interface{}) (float64, bool) {
	switch v := value.(type) {
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case uint64:
		return float64(v), true
	case float64:
		return v, true
	case json.Number:
		f, err := v.Float64()
		return f, err == nil
	default:
		return 0, false
	}
}

// validateEnvelope enforces the response-envelope rules that are the client's
// responsibility: a matching id and exactly one of result or error.
func validateEnvelope(response map[string]interface{}, sentID interface{}) error {
	if response == nil {
		return fmt.Errorf("empty response")
	}
	if sentID != nil {
		if received, ok := response["id"]; !ok || !idsEqual(sentID, received) {
			return fmt.Errorf("response id %v does not match request id %v", response["id"], sentID)
		}
	}
	_, hasResult := response["result"]
	_, hasError := response["error"]
	if hasResult && hasError {
		return fmt.Errorf("invalid response: both result and error are present")
	}
	if !hasResult && !hasError {
		return fmt.Errorf("invalid response: neither result nor error is present")
	}
	return nil
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

// Request sends a JSON-RPC request via HTTP POST with a default id of 1.
func (t *HTTPTransport) Request(method string, params interface{}) (map[string]interface{}, error) {
	return t.RequestWithID(method, params, 1)
}

// RequestWithID sends a JSON-RPC request via HTTP POST with the supplied id.
// A nil id omits the id member entirely, producing a notification.
func (t *HTTPTransport) RequestWithID(method string, params interface{}, id interface{}) (map[string]interface{}, error) {
	// Build request. Notifications have no id member.
	reqBody := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  method,
	}
	if params != nil {
		reqBody["params"] = params
	}
	if id != nil {
		reqBody["id"] = id
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

	// An empty body is the expected outcome for a notification (HTTP 204).
	if len(respBody) == 0 {
		if id == nil {
			return map[string]interface{}{}, nil
		}
		return nil, fmt.Errorf("empty response")
	}

	// Parse JSON response
	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if err := validateEnvelope(result, id); err != nil {
		return nil, err
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

// Request sends a request directly to the server with a default id of 1.
func (t *InProcTransport) Request(method string, params interface{}) (map[string]interface{}, error) {
	return t.RequestWithID(method, params, 1)
}

// RequestWithID sends a request directly to the server with the supplied id.
// A nil id produces a notification, which the server answers with nil.
func (t *InProcTransport) RequestWithID(method string, params interface{}, id interface{}) (map[string]interface{}, error) {
	req := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  method,
	}
	if params != nil {
		req["params"] = params
	}
	if id != nil {
		req["id"] = id
	}

	response := t.Server.Call(req)
	if response == nil {
		if id == nil {
			return map[string]interface{}{}, nil
		}
		return nil, fmt.Errorf("empty response (notification?)")
	}

	if err := validateEnvelope(response, id); err != nil {
		return nil, err
	}

	return response, nil
}
