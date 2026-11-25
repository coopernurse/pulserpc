package pulserpc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"math/big"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Transport interface for RPC transports
type Transport interface {
	// Request sends a JSON-RPC request and returns the response
	Request(ctx context.Context, method string, params interface{}) (map[string]interface{}, error)
}

// RequestWithID is an optional capability implemented by transports that let
// the caller supply the JSON-RPC request id. A nil id denotes a notification:
// the id member must be omitted and no response is expected. Transports that
// do not implement it keep working through the legacy Request method.
type RequestWithID interface {
	RequestWithID(ctx context.Context, method string, params interface{}, id interface{}) (map[string]interface{}, error)
}

// idsEqual compares a request id with the one echoed in a response. JSON
// decoding may turn numbers into float64 or json.Number, so numeric ids are
// compared by exact rational value rather than by Go type.
func idsEqual(sent, received interface{}) bool {
	if sent == nil || received == nil {
		return sent == nil && received == nil
	}
	if s, ok := sent.(string); ok {
		r, _ := received.(string)
		return r == s
	}

	// Keep the common cases allocation-free. Both json.Number values compare
	// exactly; a mixed json.Number/float64 pair falls back to float64 to match
	// v0.4.0 behavior for fractional ids.
	if s, ok := sent.(json.Number); ok {
		if r, ok := received.(json.Number); ok {
			return jsonNumbersEqual(s, r)
		}
		if r, ok := received.(float64); ok {
			return jsonNumberFloatEqual(s, r)
		}
	}
	if r, ok := received.(json.Number); ok {
		if s, ok := sent.(float64); ok {
			return jsonNumberFloatEqual(r, s)
		}
		if s, ok := sent.(uint64); ok {
			if i, err := r.Int64(); err == nil && i >= 0 {
				return s == uint64(i)
			}
			if u, err := strconv.ParseUint(r.String(), 10, 64); err == nil {
				return s == u
			}
		}
	}
	switch s := sent.(type) {
	case float64:
		if r, ok := received.(float64); ok {
			return s == r
		}
	case int:
		if r, ok := received.(int); ok {
			return s == r
		}
	case int64:
		if r, ok := received.(int64); ok {
			return s == r
		}
	case uint64:
		if r, ok := received.(uint64); ok {
			return s == r
		}
	}

	sv, sok := numericID(sent)
	rv, rok := numericID(received)
	return sok && rok && sv.Cmp(rv) == 0
}

// jsonNumbersEqual compares two json.Number values without a float64 round trip
// when either value is not an int64.
func jsonNumbersEqual(a, b json.Number) bool {
	if ai, err := a.Int64(); err == nil {
		if bi, err := b.Int64(); err == nil {
			return ai == bi
		}
	}
	ar, aok := new(big.Rat).SetString(a.String())
	br, bok := new(big.Rat).SetString(b.String())
	if aok && bok {
		return ar.Cmp(br) == 0
	}
	af, aerr := a.Float64()
	bf, berr := b.Float64()
	return aerr == nil && berr == nil && af == bf
}

// jsonNumberFloatEqual preserves the old float64 comparison for a mixed
// json.Number/float64 id pair.
func jsonNumberFloatEqual(n json.Number, f float64) bool {
	nf, err := n.Float64()
	return err == nil && nf == f
}

// DecodeResponse parses a JSON-RPC response body. The top-level id keeps its
// original numeric text as json.Number so large ids compare exactly; every
// other number is normalized for the v0.4.0 client view. Values that cannot
// round-trip through float64 stay json.Number.
func DecodeResponse(data []byte) (map[string]interface{}, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var response map[string]interface{}
	if err := decoder.Decode(&response); err != nil {
		return nil, err
	}
	if _, err := decoder.Token(); err != io.EOF {
		return nil, fmt.Errorf("trailing data after response")
	}
	for key, value := range response {
		if key == "id" {
			continue
		}
		response[key] = normalizeJSONNumbers(value)
	}
	return response, nil
}

// normalizeJSONNumbers converts json.Number values to float64 recursively when
// the value fits the v0.4.0 numeric view: integers in the JavaScript
// safe-integer range and fractional values. Exact wide integers and
// whole-valued decimal/exponent tokens outside that range stay json.Number so
// int64 precision is not lost. It is used for request params, response results,
// and error data so middleware and handlers see the same Go types as v0.4.0 for
// ordinary values. Negative zero keeps its sign.
func normalizeJSONNumbers(value interface{}) interface{} {
	switch v := value.(type) {
	case json.Number:
		if i, err := v.Int64(); err == nil {
			if i == 0 && strings.HasPrefix(v.String(), "-") {
				return math.Copysign(0, -1)
			}
			if intFitsFloatExactly(i) {
				return float64(i)
			}
			return v
		}
		// Integer-form text outside int64 cannot be represented exactly as a
		// float64 either; keep the exact text.
		if isIntegerFormJSONNumber(v) {
			return v
		}
		// Check the exact value before trusting float parsing. A large
		// exponent can compensate for an equally long fractional part.
		if i, ok := exactJSONInt64(v); ok {
			if intFitsFloatExactly(i) {
				if i == 0 && strings.HasPrefix(v.String(), "-") {
					return math.Copysign(0, -1)
				}
				return float64(i)
			}
			return json.Number(strconv.FormatInt(i, 10))
		}
		if f, err := v.Float64(); err == nil && math.Trunc(f) != f {
			return f
		}
		// A fractional or out-of-range token can look integral as a float.
		return v
	case []interface{}:
		for i := range v {
			v[i] = normalizeJSONNumbers(v[i])
		}
		return v
	case map[string]interface{}:
		for key := range v {
			v[key] = normalizeJSONNumbers(v[key])
		}
		return v
	default:
		return value
	}
}

// numericID converts a request id to an exact rational value. Integers are
// exact; json.Number keeps its decimal text; float64 is interpreted as the
// exact binary value it holds. Big integers therefore never lose precision by
// passing through float64.
func numericID(value interface{}) (*big.Rat, bool) {
	switch v := value.(type) {
	case int:
		return new(big.Rat).SetInt64(int64(v)), true
	case int8:
		return new(big.Rat).SetInt64(int64(v)), true
	case int16:
		return new(big.Rat).SetInt64(int64(v)), true
	case int32:
		return new(big.Rat).SetInt64(int64(v)), true
	case int64:
		return new(big.Rat).SetInt64(v), true
	case uint:
		return new(big.Rat).SetUint64(uint64(v)), true
	case uint8:
		return new(big.Rat).SetUint64(uint64(v)), true
	case uint16:
		return new(big.Rat).SetUint64(uint64(v)), true
	case uint32:
		return new(big.Rat).SetUint64(uint64(v)), true
	case uint64:
		return new(big.Rat).SetUint64(v), true
	case float64:
		r := new(big.Rat).SetFloat64(v)
		return r, r != nil
	case float32:
		r := new(big.Rat).SetFloat64(float64(v))
		return r, r != nil
	case json.Number:
		if r, ok := new(big.Rat).SetString(v.String()); ok {
			return r, true
		}
		f, err := v.Float64()
		if err != nil {
			return nil, false
		}
		r := new(big.Rat).SetFloat64(f)
		return r, r != nil
	default:
		return nil, false
	}
}

// validateEnvelope enforces the JSON-RPC 2.0 response-envelope rules that are
// the client's responsibility: a "2.0" version, a matching id, exactly one of
// result or error, and a well-formed error object.
func validateEnvelope(response map[string]interface{}, sentID interface{}) error {
	if response == nil {
		return fmt.Errorf("empty response")
	}
	if version, ok := response["jsonrpc"]; !ok || version != "2.0" {
		return fmt.Errorf("invalid response: jsonrpc version must be '2.0'")
	}
	if sentID != nil {
		if received, ok := response["id"]; !ok || !idsEqual(sentID, received) {
			return fmt.Errorf("response id %v does not match request id %v", response["id"], sentID)
		}
	}
	_, hasResult := response["result"]
	errValue, hasError := response["error"]
	if hasResult && hasError {
		return fmt.Errorf("invalid response: both result and error are present")
	}
	if !hasResult && !hasError {
		return fmt.Errorf("invalid response: neither result nor error is present")
	}
	if hasError {
		errObj, ok := errValue.(map[string]interface{})
		if !ok || errObj == nil {
			return fmt.Errorf("invalid response: error must be an object")
		}
		switch errObj["code"].(type) {
		case float64, int, int64, json.Number:
		default:
			return fmt.Errorf("invalid response: error.code must be a number")
		}
		if _, ok := errObj["message"].(string); !ok {
			return fmt.Errorf("invalid response: error.message must be a string")
		}
	}
	return nil
}

// ValidateResponseEnvelope exposes the envelope rules to generated transports,
// which live outside this package but must enforce an identical contract.
func ValidateResponseEnvelope(response map[string]interface{}, sentID interface{}) error {
	return validateEnvelope(response, sentID)
}

// maxResponseSize is the maximum number of bytes read from an HTTP response
// body. It mirrors the generated transport so a malicious or misbehaving
// server cannot make the client allocate an unbounded amount of memory (R17).
const maxResponseSize = 10 * 1024 * 1024 // 10 MiB

// readCappedBody reads at most limit bytes from body. It reads one byte past
// the limit so an oversized body is reported as an error instead of being
// silently truncated into an invalid JSON payload.
func readCappedBody(body io.Reader, limit int64) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(body, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("response exceeds %d bytes", limit)
	}
	return data, nil
}

// HTTPTransport is an HTTP-based transport
type HTTPTransport struct {
	BaseURL string
	Headers map[string]string
	Client  *http.Client
}

// NewHTTPTransport creates a new HTTP transport. The base URL has any trailing
// slash removed (D13/R30) and requests time out after 30 seconds (D6/R19).
func NewHTTPTransport(baseURL string, headers map[string]string) *HTTPTransport {
	return &HTTPTransport{
		BaseURL: strings.TrimSuffix(baseURL, "/"),
		Headers: headers,
		Client:  &http.Client{Timeout: 30 * time.Second},
	}
}

// Request sends a JSON-RPC request via HTTP POST with a default id of 1.
func (t *HTTPTransport) Request(ctx context.Context, method string, params interface{}) (map[string]interface{}, error) {
	return t.RequestWithID(ctx, method, params, 1)
}

// RequestWithID sends a JSON-RPC request via HTTP POST with the supplied id.
// A nil id omits the id member entirely, producing a notification. The caller
// context bounds the HTTP request and cancellation aborts it.
func (t *HTTPTransport) RequestWithID(ctx context.Context, method string, params interface{}, id interface{}) (map[string]interface{}, error) {
	if ctx == nil {
		ctx = context.Background()
	}

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

	// Create HTTP request bound to the caller context
	httpReq, err := http.NewRequestWithContext(ctx, "POST", t.BaseURL, bytes.NewReader(body))
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

	// Read response body, capped so an oversized response is rejected rather
	// than buffered in full (R17).
	respBody, err := readCappedBody(resp.Body, maxResponseSize)
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

	// Parse JSON response. The id stays exact; other numbers use float64.
	result, err := DecodeResponse(respBody)
	if err != nil {
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
func (t *InProcTransport) Request(ctx context.Context, method string, params interface{}) (map[string]interface{}, error) {
	return t.RequestWithID(ctx, method, params, 1)
}

// RequestWithID sends a request directly to the server with the supplied id.
// A nil id produces a notification, which the server answers with nil. The
// caller context is forwarded to the server so handlers observe cancellation
// and deadlines.
func (t *InProcTransport) RequestWithID(ctx context.Context, method string, params interface{}, id interface{}) (map[string]interface{}, error) {
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

	response := t.Server.CallWithContext(ctx, req)
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
