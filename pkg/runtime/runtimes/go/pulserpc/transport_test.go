package pulserpc

import (
	"net/http"
	"net/http/httptest"
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
