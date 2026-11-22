package pulserpc

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// R19/R30: the runtime HTTP transport sets a 30-second timeout and normalizes
// a trailing slash on the base URL.
func TestRuntimeHTTPTransportTimeoutAndTrailingSlash(t *testing.T) {
	transport := NewHTTPTransport("http://example.com/", nil)
	if transport.Client == nil {
		t.Fatal("NewHTTPTransport returned a nil client")
	}
	if transport.Client.Timeout != 30*time.Second {
		t.Errorf("client timeout = %v, want 30s", transport.Client.Timeout)
	}
	if transport.BaseURL != "http://example.com" {
		t.Errorf("BaseURL = %q, want the trailing slash removed", transport.BaseURL)
	}
}

// D6: a slow server must produce a timeout error when the transport's
// injectable http.Client is given a short timeout. The Client field is
// injectable, so this proves the timeout is actually enforced on requests
// rather than merely present as a default constant.
func TestRuntimeHTTPTransportTimesOutSlowServer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(300 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":"ok"}`))
	}))
	defer server.Close()

	transport := NewHTTPTransport(server.URL, nil)
	// The default is 30 s (asserted above); inject a short one for this test.
	transport.Client = &http.Client{Timeout: 50 * time.Millisecond}

	_, err := transport.RequestWithID("Svc.ping", []interface{}{}, 1)
	if err == nil {
		t.Fatal("expected a timeout error from a server that sleeps past the client timeout")
	}
	if !strings.Contains(err.Error(), "request failed") {
		t.Errorf("error = %v, want a wrapped request failure", err)
	}
	lower := strings.ToLower(err.Error())
	if !strings.Contains(lower, "timeout") && !strings.Contains(lower, "deadline exceeded") {
		t.Errorf("error = %v, want a timeout/deadline exceeded error", err)
	}
}

// R29: custom headers supplied to the runtime transport are sent with every
// request.
func TestRuntimeHTTPTransportSendsCustomHeaders(t *testing.T) {
	var gotHeader string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeader = r.Header.Get("X-Api-Key")
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":"ok"}`))
	}))
	defer server.Close()

	transport := NewHTTPTransport(server.URL+"/", map[string]string{"X-Api-Key": "secret"})
	response, err := transport.RequestWithID("Svc.ping", []interface{}{}, 1)
	if err != nil {
		t.Fatalf("RequestWithID: %v", err)
	}
	if response["result"] != "ok" {
		t.Errorf("result = %#v, want ok", response["result"])
	}
	if gotHeader != "secret" {
		t.Errorf("X-Api-Key = %q, want secret", gotHeader)
	}
}

// R17: response bodies are bounded so a hostile server cannot make the client
// buffer an unbounded payload.
func TestRuntimeHTTPTransportRejectsOversizedResponse(t *testing.T) {
	oversized := bytes.Repeat([]byte("a"), maxResponseSize+1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(oversized)
	}))
	defer server.Close()

	transport := NewHTTPTransport(server.URL, nil)
	_, err := transport.RequestWithID("Svc.ping", []interface{}{}, 1)
	if err == nil {
		t.Fatal("expected an error for a response larger than maxResponseSize")
	}
	if !strings.Contains(err.Error(), "exceeds") {
		t.Errorf("error = %v, want a size-limit error", err)
	}
}

// The bounded reader rejects oversized input instead of truncating it.
func TestReadCappedBody(t *testing.T) {
	if _, err := readCappedBody(strings.NewReader("123456789"), 8); err == nil {
		t.Error("readCappedBody accepted a body one byte over the limit")
	} else if !strings.Contains(err.Error(), "exceeds 8 bytes") {
		t.Errorf("error = %v, want a limit of 8 bytes", err)
	}

	data, err := readCappedBody(strings.NewReader("12345678"), 8)
	if err != nil {
		t.Fatalf("readCappedBody rejected a body at the limit: %v", err)
	}
	if string(data) != "12345678" {
		t.Errorf("data = %q, want 12345678", data)
	}
}
