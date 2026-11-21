package generator

import (
	"strings"
	"testing"

	"github.com/coopernurse/pulserpc/pkg/parser"
)

func parseTestIDL(t *testing.T, src string) *parser.IDL {
	t.Helper()
	idl, err := parser.ParseIDL("test.pulse", src)
	if err != nil {
		t.Fatalf("ParseIDL: %v", err)
	}
	if err := parser.ValidateIDL(idl); err != nil {
		t.Fatalf("ValidateIDL: %v", err)
	}
	return idl
}

// core-09: generated clients must diff the local IDL against the server's IDL,
// not against themselves.
func TestVerifyCompatibilityFetchesServerIDL(t *testing.T) {
	t.Run("go", func(t *testing.T) {
		var sb strings.Builder
		writeVerifyCompatibilityMethod(&sb, "TestClient")
		code := sb.String()

		if !strings.Contains(code, `c.transport.Call("pulserpc-idl", []interface{}{})`) {
			t.Errorf("generated Go VerifyCompatibility does not fetch the server IDL via pulserpc-idl:\n%s", code)
		}
		if strings.Contains(code, "serverIDL = c.contract.GetIDLParsed()") {
			t.Errorf("generated Go VerifyCompatibility still diffs the embedded IDL against itself:\n%s", code)
		}
	})

	t.Run("java", func(t *testing.T) {
		code := generateClientJava(nil, map[string]*NamespaceTypes{}, "com.example", "package com.example;", "{}")
		if !strings.Contains(code, `call("pulserpc-idl", new HashMap<>())`) {
			t.Errorf("generated Java verifyCompatibility does not fetch the server IDL via pulserpc-idl")
		}
		if strings.Contains(code, "Object serverIDL = jsonParser.fromJson(IDL_JSON, Object.class);") {
			t.Errorf("generated Java verifyCompatibility still diffs the embedded IDL against itself")
		}
	})

	t.Run("csharp", func(t *testing.T) {
		idl := parseTestIDL(t, "namespace test\ninterface Svc { Ping() string }\n")
		code := generateClientCs(idl, nil, nil, map[string]*NamespaceTypes{}, "{}", CSharpNamespacePaths{})
		if !strings.Contains(code, `_transport.CallAsync("pulserpc-idl", Array.Empty<object>())`) {
			t.Errorf("generated C# VerifyCompatibility does not fetch the server IDL via pulserpc-idl")
		}
		if strings.Contains(code, `var idlObj = JsonSerializer.Deserialize<object>(IdlHolder.GetIDLJson());`) {
			t.Errorf("generated C# VerifyCompatibility still diffs the embedded IDL against itself")
		}
	})
}

// core-09 acceptance: ServerChecksum and ClientChecksum must be able to differ.
// The generated code reads the checksum from the fetched serverIDL object rather
// than from the same object used for the client.
func TestVerifyCompatibilityChecksumsComeFromDifferentObjects(t *testing.T) {
	t.Run("go", func(t *testing.T) {
		var sb strings.Builder
		writeVerifyCompatibilityMethod(&sb, "TestClient")
		code := sb.String()
		if !strings.Contains(code, `if dict, ok := serverIDL.(map[string]interface{}); ok {`) {
			t.Errorf("generated Go client does not read the checksum from the server IDL object:\n%s", code)
		}
	})

	t.Run("java", func(t *testing.T) {
		code := generateClientJava(nil, map[string]*NamespaceTypes{}, "com.example", "package com.example;", "{}")
		if !strings.Contains(code, "extractChecksum(serverIDL)") {
			t.Errorf("generated Java client does not read the checksum from the server IDL object")
		}
	})

	t.Run("csharp", func(t *testing.T) {
		idl := parseTestIDL(t, "namespace test\ninterface Svc { Ping() string }\n")
		code := generateClientCs(idl, nil, nil, map[string]*NamespaceTypes{}, "{}", CSharpNamespacePaths{})
		if !strings.Contains(code, "ServerChecksum = IdlHolder.ExtractChecksum(_serverIDL)") {
			t.Errorf("generated C# client does not read the checksum from the server IDL object")
		}
	})
}
