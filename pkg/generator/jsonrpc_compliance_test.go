package generator

import (
	"os"
	"strings"
	"testing"
)

// core-10: every generated server must emit -32700 for a malformed JSON body,
// and every generated client must surface the server's error object rather than
// a bogus "missing result" error.
func TestGeneratedAdaptersSurfaceRPCErrors(t *testing.T) {
	t.Run("java_server_parse_error", func(t *testing.T) {
		idl := parseTestIDL(t, "namespace test\ninterface Svc { Ping() string }\n")
		code := generateServerJava(idl, nil, map[string]*NamespaceTypes{}, "com.example", "package com.example;", "{}")
		if !strings.Contains(code, `sendError(exchange, -32700, "Parse error")`) {
			t.Errorf("generated Java server does not return -32700 on malformed JSON")
		}
	})

	t.Run("csharp_client_error_before_result", func(t *testing.T) {
		idl := parseTestIDL(t, "namespace test\ninterface Svc { Ping() string }\n")
		code := generateClientCs(idl, nil, nil, map[string]*NamespaceTypes{}, "{}", CSharpNamespacePaths{})
		errIdx := strings.Index(code, `TryGetValue("error", out var rpcError)`)
		resultIdx := strings.Index(code, `TryGetValue("result", out var result)`)
		if errIdx < 0 {
			t.Fatalf("generated C# client does not inspect response[\"error\"]")
		}
		if resultIdx < 0 || errIdx > resultIdx {
			t.Errorf("generated C# client must check error before result (error@%d result@%d)", errIdx, resultIdx)
		}
	})

	t.Run("go_client_uses_transport_that_checks_error", func(t *testing.T) {
		var sb strings.Builder
		writeHTTPTransportGo(&sb)
		code := sb.String()
		if !strings.Contains(code, `response["error"]`) {
			t.Errorf("generated Go transport does not surface a server error object")
		}
	})

	t.Run("ts_client_checks_error", func(t *testing.T) {
		idl := parseTestIDL(t, "namespace test\ninterface Svc { Ping() string }\n")
		code := generateClientTs(idl, nil, nil, "test", map[string]*NamespaceTypes{}, "esm-node")
		if !strings.Contains(code, "if (_resp.error)") {
			t.Errorf("generated TS client does not surface a server error object")
		}
	})
}

// core-10: batch requests are documented as optional. Go, C#, and Java
// generated servers implement them; TypeScript and Python do not.
func TestRuntimeGuideMarksBatchOptional(t *testing.T) {
	guidePath := "docs/RUNTIME_IMPLEMENTATION_GUIDE.md"
	if _, err := os.Stat(guidePath); err != nil {
		// `go test` runs from the package directory.
		guidePath = "../../docs/RUNTIME_IMPLEMENTATION_GUIDE.md"
	}
	data, err := os.ReadFile(guidePath)
	if err != nil {
		t.Fatalf("read guide: %v", err)
	}
	guide := string(data)
	if !strings.Contains(guide, "Batch requests (an array of requests) are **optional**") {
		t.Errorf("runtime guide still promises batch requests as mandatory")
	}
}
