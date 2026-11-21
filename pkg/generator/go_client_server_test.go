package generator

import (
	"strings"
	"testing"
)

func generatedGoClient(t *testing.T, src string) string {
	t.Helper()
	idl := parseTestIDL(t, src)
	return generateClientGo(idl, nil, nil, "test", map[string]*NamespaceTypes{}, "pulserpc", "example.com/rpc")
}

// go-08: the generated validation options must gate real IDL validation calls.
func TestGoGeneratedClientUsesValidationFlags(t *testing.T) {
	code := generatedGoClient(t, "namespace test\ninterface Svc { add(a int, b int) string }\n")

	for _, marker := range []string{
		"if c.validateRequests && c.contract != nil {",
		`c.contract.ValidateRequest("Svc", "add", params)`,
		"if c.validateResponses && c.contract != nil {",
		`c.contract.ValidateResponse("Svc", "add", result)`,
		`request validation failed: %w`,
		`response validation failed: %w`,
	} {
		if !strings.Contains(code, marker) {
			t.Errorf("generated client is missing %q", marker)
		}
	}

	// Restrict the ordering check to the generated method: VerifyCompatibility
	// also uses c.transport.Call but deliberately has no param validation.
	methodAt := strings.Index(code, "func (c *SvcClient) Add(")
	if methodAt < 0 {
		t.Fatal("generated client has no Add method")
	}
	methodCode := code[methodAt:]
	requestValidation := strings.Index(methodCode, "c.contract.ValidateRequest(")
	transportCall := strings.Index(methodCode, "c.transport.Call(")
	responseValidation := strings.Index(methodCode, "c.contract.ValidateResponse(")
	if requestValidation < 0 || transportCall < 0 || responseValidation < 0 ||
		requestValidation > transportCall || transportCall > responseValidation {
		t.Errorf("validation calls must surround the transport call (request=%d transport=%d response=%d)",
			requestValidation, transportCall, responseValidation)
	}
}

// go-10: the core commits fixed self-comparison; the remaining acceptance
// criterion is a non-zero Timestamp on every generated verification result.
func TestGoGeneratedVerificationResultHasTimestamp(t *testing.T) {
	var sb strings.Builder
	writeVerifyCompatibilityMethod(&sb, "TestClient")
	code := sb.String()

	if count := strings.Count(code, "Timestamp:"); count < 2 {
		t.Errorf("generated VerifyCompatibility sets Timestamp %d times, want at least 2 (success + failed fetch)", count)
	}
	if !strings.Contains(code, "time.Now()") {
		t.Error("generated VerifyCompatibility does not populate Timestamp with time.Now()")
	}
	if !strings.Contains(code, "pulserpc.VerificationResult{") {
		t.Error("generated VerifyCompatibility does not return a VerificationResult")
	}

	clientCode := generatedGoClient(t, "namespace test\ninterface Svc { Ping() string }\n")
	if !strings.Contains(clientCode, "\t\"time\"\n") {
		t.Error("generated client does not import time for the verification timestamp")
	}
}

// go-02 / go-03: the generated transport must have an id-less notify path and
// unique, validated request ids instead of a length-derived constant id.
func TestGoGeneratedTransportNotificationAndIDs(t *testing.T) {
	var transport strings.Builder
	writeHTTPTransportGo(&transport)
	transportCode := transport.String()

	var iface strings.Builder
	writeTransportInterfaceGo(&iface)
	if !strings.Contains(iface.String(), "type NotifyTransport interface") {
		t.Error("generated client does not declare NotifyTransport")
	}

	for _, marker := range []string{
		"func (t *HTTPTransport) Notify(",
		"t.call(method, params, nil)",
		"if id != nil {\n\t\trequest[\"id\"] = id\n\t}",
		"atomic.AddUint64(&t.requestID, 1)",
		"response id %v does not match request id %v",
		"both result and error are present",
	} {
		if !strings.Contains(transportCode, marker) {
			t.Errorf("generated transport is missing %q", marker)
		}
	}
	if strings.Contains(transportCode, "len(method)+len(params)") {
		t.Error("generated transport still derives request ids from payload lengths")
	}

	clientCode := generatedGoClient(t, "namespace test\ninterface Svc { Ping() string }\n")
	for _, marker := range []string{"\t\"io\"\n", "\t\"sync/atomic\"\n"} {
		if !strings.Contains(clientCode, marker) {
			t.Errorf("generated client is missing import %q", marker)
		}
	}
	if !strings.Contains(clientCode, "transport.(NotifyTransport)") {
		t.Error("generated client has no Notify helper using NotifyTransport")
	}
}

// audit-go-3: the generated client must bound the HTTP response body read so a
// misbehaving server cannot force unbounded allocations.
func TestGoGeneratedClientBoundsResponseRead(t *testing.T) {
	code := generatedGoClient(t, "namespace test\ninterface Svc { Ping() string }\n")

	for _, marker := range []string{
		"const maxResponseSize = 10 * 1024 * 1024",
		"io.ReadAll(io.LimitReader(resp.Body, maxResponseSize+1))",
		"response exceeds %d bytes",
	} {
		if !strings.Contains(code, marker) {
			t.Errorf("generated client is missing %q", marker)
		}
	}
	if strings.Contains(code, "io.ReadAll(resp.Body)") {
		t.Error("generated client still uses an unbounded io.ReadAll(resp.Body)")
	}
}

// go-15: the generated HTTP adapter implements batching. Assert the generated
// code covers mixed batches, empty batches and notification-only batches.
func TestGoGeneratedServerBatchHandling(t *testing.T) {
	var sb strings.Builder
	writeServerHandleRequestGoRefactored(&sb)
	code := sb.String()

	for _, marker := range []string{
		"if requests, ok := requestData.([]interface{}); ok {",
		"len(requests) == 0",
		"-32600",
		"if resp != nil {",
		"if len(responses) == 0 {",
		"http.StatusNoContent",
		"json.NewEncoder(w).Encode(responses)",
	} {
		if !strings.Contains(code, marker) {
			t.Errorf("generated batch handler is missing %q", marker)
		}
	}

	// go-01: a nil Server.Call response (any notification, including an error
	// notification) must become HTTP 204.
	if !strings.Contains(code, "if response == nil {") {
		t.Error("generated single-request handler does not suppress responses for notifications")
	}
}
