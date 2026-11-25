package generator

import (
	"os"
	"path/filepath"
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

		if !strings.Contains(code, `c.transport.Call(ctx, "pulserpc-idl", []interface{}{})`) {
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
// assertFailureBeforeDiff verifies that a generated VerifyCompatibility returns
// an explicit, incompatible result on the failed-fetch path BEFORE reaching the
// diff, so a fetch failure cannot be mistaken for a successful verification.
func assertFailureBeforeDiff(t *testing.T, code, failureMarker, incompatibleMarker, errorMarker, diffMarker string) {
	t.Helper()
	fail := strings.Index(code, failureMarker)
	if fail < 0 {
		t.Errorf("missing failed-fetch marker %q", failureMarker)
		return
	}
	if !strings.Contains(code, incompatibleMarker) {
		t.Errorf("failed-fetch path does not set an incompatible result (%q)", incompatibleMarker)
	}
	if !strings.Contains(code, errorMarker) {
		t.Errorf("failed-fetch path does not set an explicit error (%q)", errorMarker)
	}
	// Match the actual call, not the explanatory comment that mentions DiffIDL.
	diff := strings.Index(code, diffMarker)
	if diff < 0 {
		t.Fatalf("generated client no longer calls DiffIDL on the success path (%q)", diffMarker)
	}
	if fail > diff {
		t.Errorf("failed-fetch return (index %d) must precede DiffIDL (index %d)", fail, diff)
	}
}

// Review finding 1: a failed pulserpc-idl fetch must be reported explicitly in
// all three generated clients, never as Compatible=true.
func TestVerifyCompatibilityFailedFetchIsExplicit(t *testing.T) {
	t.Run("go", func(t *testing.T) {
		var sb strings.Builder
		writeVerifyCompatibilityMethod(&sb, "TestClient")
		code := sb.String()

		if !strings.Contains(code, `fetchErr = err`) || !strings.Contains(code, "server returned error") {
			t.Errorf("Go client ignores transport errors or error responses:\n%s", code)
		}
		assertFailureBeforeDiff(t, code,
			`if c.serverIDL == nil {`,
			"Compatible: false,",
			"failed to fetch server IDL via pulserpc-idl",
			"deltas := pulserpc.DiffIDL(")
		if !strings.Contains(code, "Error:") {
			t.Errorf("Go VerificationResult failure has no Error field set:\n%s", code)
		}
	})

	t.Run("java", func(t *testing.T) {
		code := generateClientJava(nil, map[string]*NamespaceTypes{}, "com.example", "package com.example;", "{}")

		if !strings.Contains(code, "fetchError") || !strings.Contains(code, "server returned error") {
			t.Errorf("Java client ignores transport errors or error responses")
		}
		fail := strings.Index(code, "if (fetchError != null || serverIDL == null)")
		if fail < 0 {
			t.Fatal("Java client has no failed-fetch branch")
		}
		if !strings.Contains(code, "false, \"\", extractChecksum(clientIDL), new ArrayList<>(), System.currentTimeMillis(), error)") {
			t.Errorf("Java failed-fetch result is not explicitly incompatible with an error")
		}
		diff := strings.Index(code, "DiffEngine.diffIDL(clientIDL, serverIDL)")
		if diff < 0 {
			t.Fatal("generated Java client no longer diffs on the success path")
		}
		if fail > diff {
			t.Errorf("Java failed-fetch return (%d) must precede diff (%d)", fail, diff)
		}
	})

	t.Run("csharp", func(t *testing.T) {
		idl := parseTestIDL(t, "namespace test\ninterface Svc { Ping() string }\n")
		code := generateClientCs(idl, nil, nil, map[string]*NamespaceTypes{}, "{}", CSharpNamespacePaths{})

		if !strings.Contains(code, "fetchError") || !strings.Contains(code, "server returned error") {
			t.Errorf("C# client ignores transport errors or error responses")
		}
		assertFailureBeforeDiff(t, code,
			"if (fetchError != null || _serverIDL == null)",
			"Compatible = false,",
			"Error = fetchError ?? \"server IDL is empty\"",
			"var deltas = DiffEngine.DiffIDL(")
	})
}

// Review finding 1: the runtime VerificationResult types must carry the
// explicit error so callers can distinguish "inconclusive" from "compatible".
func TestRuntimeVerificationResultHasErrorField(t *testing.T) {
	read := func(path string) string {
		t.Helper()
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		return string(b)
	}
	goSrc := read(filepath.Join("..", "runtime", "runtimes", "go", "pulserpc", "types.go"))
	if !strings.Contains(goSrc, "Error          string") {
		t.Errorf("Go VerificationResult has no Error field")
	}
	javaSrc := read(filepath.Join("..", "runtime", "runtimes", "java", "com", "bitmechanic", "pulserpc", "VerificationResult.java"))
	if !strings.Contains(javaSrc, "getError()") || !strings.Contains(javaSrc, "List<ContractDelta> deltas, long timestamp, String error)") {
		t.Errorf("Java VerificationResult has no error-aware constructor/getter")
	}
	csSrc := read(filepath.Join("..", "runtime", "runtimes", "csharp", "PulseRPC", "VerificationResult.cs"))
	if !strings.Contains(csSrc, "public string Error") {
		t.Errorf("C# VerificationResult has no Error property")
	}
}

// Review finding 2: untrusted server error.code must not be parsed with a
// throwing GetInt32() call.
func TestGeneratedCSharpErrorCodeConversionIsGuarded(t *testing.T) {
	idl := parseTestIDL(t, "namespace test\ninterface Svc { Ping() string }\n")
	code := generateClientCs(idl, nil, nil, map[string]*NamespaceTypes{}, "{}", CSharpNamespacePaths{})

	if !strings.Contains(code, "codeProp.TryGetInt32(out var parsedCode)") {
		t.Errorf("C# client does not guard error.code conversion with TryGetInt32")
	}
	if strings.Contains(code, "codeProp.GetInt32()") {
		t.Errorf("C# client still uses throwing GetInt32() on untrusted error.code")
	}
	if !strings.Contains(code, "var errorCode = -32603;") {
		t.Errorf("C# client has no -32603 fallback for malformed error.code")
	}
}

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
