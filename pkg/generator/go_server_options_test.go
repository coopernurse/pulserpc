package generator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const optionsIDL = `namespace opt

struct Meta {
  note string
}

struct Item {
  name string
  count int
  meta Meta
}

interface Svc {
  ping() string
  add(a int, b int) int
  meta(item Item) string
}
`

// The generated server must expose the configurable constructor, read the
// runtime policy inside the adapter, and keep the body cap ahead of HTTP
// middleware.
func TestGoGeneratedServerOptionsSource(t *testing.T) {
	idl := parseTestIDL(t, optionsIDL)
	code := generateServerGo(idl, marshalTestIDLJSON(t, idl), nil, nil, "opt", map[string]*NamespaceTypes{}, "pulserpc", "example.com/rpc")

	for _, marker := range []string{
		"func NewPulseRPCServer(host string, port int) *PulseRPCServer",
		"pulserpc.DefaultServerConfig()",
		"func NewPulseRPCServerWithConfig(host string, port int, config pulserpc.ServerConfig) *PulseRPCServer",
		"pulserpc.NewServerWithConfig(contract, true, true, config)",
		"config := s.rpcServer.Config()",
		"if config.MaxRequestBodyBytes > 0 {",
		"http.MaxBytesReader(w, r.Body, bodyLimit)",
		"if config.RejectInvalidUTF8 && !utf8.Valid(body) {",
		"decoder.UseNumber()",
		"decoder.Token()",
		"pulserpc.NormalizeRequestNumbers(requestData)",
		"isBatch && !config.AllowBatch",
		`"bytes"`,
		`"unicode/utf8"`,
	} {
		if !strings.Contains(code, marker) {
			t.Errorf("generated server missing %q", marker)
		}
	}

	maxBytes := strings.Index(code, "http.MaxBytesReader(w, r.Body, bodyLimit)")
	middlewareLoop := strings.Index(code, "for _, mw := range s.httpMiddleware")
	if maxBytes < 0 || middlewareLoop < 0 || maxBytes > middlewareLoop {
		t.Errorf("body cap is not installed before HTTP middleware (maxBytes=%d, middleware=%d)", maxBytes, middlewareLoop)
	}

	client := generatedGoClient(t, "namespace opt\ninterface Svc { ping() string }\n")
	if !strings.Contains(client, "pulserpc.DecodeResponse(jsonDataResp)") {
		t.Error("generated client does not decode responses with the exact-id helper")
	}
}

const optionsE2ETestFile = `package opt

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"example.com/wave6/rpc/pulserpc"
)

type e2eHandler struct{}

func (e2eHandler) Ping(ctx context.Context) (string, error) { return "pong", nil }
func (e2eHandler) Add(ctx context.Context, a, b int) (int, error) { return a + b, nil }
func (e2eHandler) Meta(ctx context.Context, item Item) (string, error) { return item.Name, nil }

func strictConfig() pulserpc.ServerConfig {
	cfg := pulserpc.DefaultServerConfig()
	cfg.AllowBatch = false
	cfg.RequireNonNullID = true
	cfg.ParamsMode = pulserpc.ParamsModePositionalOnly
	cfg.MaxRequestBodyBytes = 32 * 1024
	cfg.RejectInvalidUTF8 = true
	cfg.RejectUnknownRequestFields = true
	cfg.MapError = func(phase pulserpc.ErrorPhase, err error) *pulserpc.RPCError {
		return pulserpc.NewRPCError(4101, "Invalid input")
	}
	return cfg
}

func newServer(t *testing.T, cfg pulserpc.ServerConfig) *httptest.Server {
	t.Helper()
	server := NewPulseRPCServerWithConfig("0.0.0.0", 0, cfg)
	if err := server.Register("Svc", e2eHandler{}); err != nil {
		t.Fatalf("Register: %v", err)
	}
	return httptest.NewServer(server)
}

func post(t *testing.T, server *httptest.Server, body []byte) (int, []byte) {
	t.Helper()
	resp, err := http.Post(server.URL, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return resp.StatusCode, data
}

func expectCode(t *testing.T, body []byte, want int) map[string]interface{} {
	t.Helper()
	var response map[string]interface{}
	if err := json.Unmarshal(body, &response); err != nil {
		t.Fatalf("unmarshal %s: %v", body, err)
	}
	errObj, ok := response["error"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected an error response, got %s", body)
	}
	code, _ := errObj["code"].(float64)
	if int(code) != want {
		t.Fatalf("code = %v, want %d (%s)", errObj["code"], want, body)
	}
	return response
}

func TestGeneratedServerOptionsEndToEnd(t *testing.T) {
	server := newServer(t, strictConfig())
	defer server.Close()

	t.Run("default constructor allows batches", func(t *testing.T) {
		plain := NewPulseRPCServer("0.0.0.0", 0)
		if err := plain.Register("Svc", e2eHandler{}); err != nil {
			t.Fatalf("Register: %v", err)
		}
		plainServer := httptest.NewServer(plain)
		defer plainServer.Close()

		status, body := post(t, plainServer, []byte(` + "`" + `[{"jsonrpc":"2.0","method":"Svc.ping","params":[],"id":1}]` + "`" + `))
		if status != http.StatusOK {
			t.Fatalf("status = %d, want 200", status)
		}
		if !strings.Contains(string(body), "pong") {
			t.Fatalf("default server rejected a batch: %s", body)
		}
	})

	t.Run("batch disabled", func(t *testing.T) {
		status, body := post(t, server, []byte(` + "`" + `[{"jsonrpc":"2.0","method":"Svc.ping","params":[],"id":1}]` + "`" + `))
		if status != http.StatusOK {
			t.Fatalf("status = %d, want 200", status)
		}
		expectCode(t, body, -32600)
	})

	t.Run("absent id", func(t *testing.T) {
		_, body := post(t, server, []byte(` + "`" + `{"jsonrpc":"2.0","method":"Svc.ping"}` + "`" + `))
		response := expectCode(t, body, -32600)
		if id, exists := response["id"]; !exists || id != nil {
			t.Fatalf("response id = %#v (exists=%t), want null", id, exists)
		}
	})

	t.Run("null id", func(t *testing.T) {
		_, body := post(t, server, []byte(` + "`" + `{"jsonrpc":"2.0","method":"Svc.ping","id":null}` + "`" + `))
		expectCode(t, body, -32600)
	})

	t.Run("named params rejected", func(t *testing.T) {
		_, body := post(t, server, []byte(` + "`" + `{"jsonrpc":"2.0","method":"Svc.add","params":{"a":1,"b":2},"id":1}` + "`" + `))
		expectCode(t, body, -32600)
	})

	t.Run("unknown nested field mapped", func(t *testing.T) {
		_, body := post(t, server, []byte(` + "`" + `{"jsonrpc":"2.0","method":"Svc.meta","params":[{"name":"x","count":1,"meta":{"note":"n","bogus":1}}],"id":1}` + "`" + `))
		response := expectCode(t, body, 4101)
		errObj := response["error"].(map[string]interface{})
		if errObj["message"] != "Invalid input" {
			t.Fatalf("message = %#v, want Invalid input", errObj["message"])
		}
		if _, exists := errObj["data"]; exists {
			t.Fatalf("mapped error carried data: %s", body)
		}
	})

	t.Run("zero-arg method without params", func(t *testing.T) {
		status, body := post(t, server, []byte(` + "`" + `{"jsonrpc":"2.0","method":"Svc.ping","id":7}` + "`" + `))
		if status != http.StatusOK {
			t.Fatalf("status = %d, want 200", status)
		}
		if !strings.Contains(string(body), ` + "`" + `"result":"pong"` + "`" + `) {
			t.Fatalf("zero-arg call failed: %s", body)
		}
	})

	t.Run("oversized body", func(t *testing.T) {
		body := []byte(` + "`" + `{"jsonrpc":"2.0","method":"Svc.ping","params":[],"id":1}` + "`" + ` + strings.Repeat(" ", 33*1024))
		_, response := post(t, server, body)
		expectCode(t, response, -32700)
	})

	t.Run("invalid utf8", func(t *testing.T) {
		body := []byte("{\"jsonrpc\":\"2.0\",\"method\":\"Svc.ping\",\"params\":[],\"id\":1,\"x\":\"\xff\"}")
		_, response := post(t, server, body)
		expectCode(t, response, -32700)
	})

	t.Run("numeric id is exact", func(t *testing.T) {
		_, body := post(t, server, []byte(` + "`" + `{"jsonrpc":"2.0","method":"Svc.ping","params":[],"id":9007199254740993}` + "`" + `))
		if !strings.Contains(string(body), ` + "`" + `"id":9007199254740993` + "`" + `) {
			t.Fatalf("numeric id lost precision: %s", body)
		}
	})
}

// Params keep the v0.4.0 float64 view for middleware and handlers; only the
// top-level id stays json.Number so it can be echoed exactly.
func TestGeneratedServerNormalizesParamsForMiddleware(t *testing.T) {
	plain := NewPulseRPCServerWithConfig("0.0.0.0", 0, pulserpc.DefaultServerConfig())
	if err := plain.Register("Svc", e2eHandler{}); err != nil {
		t.Fatalf("Register: %v", err)
	}

	var paramType, idType string
	plain.UseRPC(func(ctx context.Context, request map[string]interface{}, response map[string]interface{}) (context.Context, map[string]interface{}, map[string]interface{}) {
		if params, ok := request["params"].([]interface{}); ok && len(params) > 0 {
			paramType = fmt.Sprintf("%T", params[0])
		}
		idType = fmt.Sprintf("%T", request["id"])
		return ctx, request, nil
	})

	server := httptest.NewServer(plain)
	defer server.Close()

	status, body := post(t, server, []byte(` + "`" + `{"jsonrpc":"2.0","method":"Svc.add","params":[1,2],"id":1}` + "`" + `))
	if status != http.StatusOK || !strings.Contains(string(body), ` + "`" + `"result":3` + "`" + `) {
		t.Fatalf("call failed: status=%d body=%s", status, body)
	}
	if paramType != "float64" {
		t.Fatalf("middleware param type = %s, want float64", paramType)
	}
	if idType != "json.Number" {
		t.Fatalf("middleware id type = %s, want json.Number", idType)
	}
}
`

// Generated modules exercise every policy end to end against the HTTP adapter.
func TestGoGeneratedServerOptionsEndToEnd(t *testing.T) {
	out := wave6Generate(t, map[string]string{
		"root.pulse": optionsIDL,
	}, "root.pulse", false)

	testPath := filepath.Join(out, "opt", "options_e2e_test.go")
	if err := os.WriteFile(testPath, []byte(optionsE2ETestFile), 0644); err != nil {
		t.Fatalf("write e2e test: %v", err)
	}
	wave6Go(t, out, "test", "./...")
}
