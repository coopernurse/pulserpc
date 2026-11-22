package generator

import (
	"flag"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/coopernurse/pulserpc/pkg/parser"
)

// runCSharpGenerate runs the C# generator into outDir with the standard test
// flags enabled.
func runCSharpGenerate(t *testing.T, idl *parser.IDL, outDir string) {
	t.Helper()
	fs := flag.NewFlagSet("csharp", flag.ContinueOnError)
	fs.String("dir", outDir, "")
	fs.String("package", "", "")
	fs.Bool("silent", true, "")
	fs.Bool("generate-test-files", true, "")
	if err := NewCSharpClientServer().Generate(idl, fs); err != nil {
		t.Fatalf("Generate: %v", err)
	}
}

// readTree returns relative path -> contents for every file under dir.
func readTree(t *testing.T, dir string) map[string][]byte {
	t.Helper()
	files := make(map[string][]byte)
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		files[filepath.ToSlash(rel)] = data
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", dir, err)
	}
	return files
}

// C1/D12: generating the same IDL twice must produce byte-identical output.
func TestCSharpGenerationIsByteDeterministic(t *testing.T) {
	idl := parseTestIDL(t, `
namespace demo

struct Thing {
  name string
  count int
}

interface Svc {
  Do(thing Thing) Thing
}
`)
	base := t.TempDir()
	first := filepath.Join(base, "first")
	second := filepath.Join(base, "second")

	runCSharpGenerate(t, idl, first)
	runCSharpGenerate(t, idl, second)

	a := readTree(t, first)
	b := readTree(t, second)

	if len(a) == 0 {
		t.Fatal("generator produced no files")
	}
	if len(a) != len(b) {
		t.Fatalf("file count differs: %d vs %d", len(a), len(b))
	}
	var paths []string
	for p := range a {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	for _, p := range paths {
		if string(a[p]) != string(b[p]) {
			t.Errorf("generated file differs between runs: %s", p)
		}
	}
}

// C1/D12: the unqualified-type fallback used by writeTypeDictCs must resolve
// deterministically (first key in sorted order) rather than by map iteration.
func TestCSharpTypeDictFallbackIsDeterministic(t *testing.T) {
	enumMap := map[string]*parser.Enum{
		"z.MathOp": {Name: "z.MathOp"},
		"a.MathOp": {Name: "a.MathOp"},
		"m.MathOp": {Name: "m.MathOp"},
	}
	structMap := map[string]*parser.Struct{}
	typ := &parser.Type{UserDefined: "MathOp"}

	var first string
	for i := 0; i < 200; i++ {
		var sb strings.Builder
		writeTypeDictCs(&sb, typ, structMap, enumMap)
		got := sb.String()
		if i == 0 {
			first = got
			continue
		}
		if got != first {
			t.Fatalf("writeTypeDictCs is not deterministic: %q vs %q", first, got)
		}
	}

	want := `new Dictionary<string, object> { { "userDefined", "a.MathOp" } }`
	if first != want {
		t.Errorf("fallback resolution = %q, want %q", first, want)
	}
}

// C10/D7: optional built-ins must be nullable, and generated files must enable
// nullable reference types so `string?` is meaningful.
func TestCSharpOptionalBuiltInsAreNullable(t *testing.T) {
	idl := parseTestIDL(t, `
namespace demo

struct OptionalFields {
  opt_int int [optional]
  opt_bool bool [optional]
  opt_float float [optional]
  opt_string string [optional]
}

interface Svc {
  Ping() string
}
`)
	outDir := t.TempDir()
	runCSharpGenerate(t, idl, outDir)

	data, err := os.ReadFile(filepath.Join(outDir, "Demo", "Types.cs"))
	if err != nil {
		t.Fatalf("read Types.cs: %v", err)
	}
	code := string(data)

	for _, want := range []string{
		"#nullable enable",
		"public int? OptInt { get; set; }",
		"public bool? OptBool { get; set; }",
		"public double? OptFloat { get; set; }",
		"public string? OptString { get; set; }",
	} {
		if !strings.Contains(code, want) {
			t.Errorf("generated Types.cs missing %q\n%s", want, code)
		}
	}
}

// R26: IDL float maps to C# double.
func TestCSharpFloatMapsToDouble(t *testing.T) {
	typ := &parser.Type{BuiltIn: "float"}
	if got := mapTypeToCsType(typ, nil, nil, false); got != "double" {
		t.Errorf("mapTypeToCsType(float) = %q, want double", got)
	}
}

// R15: internal error paths must not put exception text or stack traces on the
// wire; validation errors (-32602) keep their messages.
func TestCSharpGeneratedServerRedactsInternalErrors(t *testing.T) {
	idl := parseTestIDL(t, "namespace test\ninterface Svc { Ping() string }\n")
	paths := NewCSharpNamespacePaths("", "")
	code := generateServerCs(idl, map[string]*NamespaceTypes{}, nil, nil, "{}", paths)

	for _, forbidden := range []string{
		"e.StackTrace",
		`Exception: {e.Message}`,
		`Failed to deserialize IDL JSON: {e.Message}`,
		`Response validation failed: {e.Message}`,
	} {
		if strings.Contains(code, forbidden) {
			t.Errorf("generated server leaks internal detail: %q", forbidden)
		}
	}
	for _, want := range []string{
		`ErrorResponse(requestId, -32603, "Internal error");`,
		`ErrorResponse(requestId, -32602, "Invalid params"`,
		`_logger?.LogError(e, "Exception invoking`,
	} {
		if !strings.Contains(code, want) {
			t.Errorf("generated server missing %q", want)
		}
	}
}

// R16/R18: the generated server caps request bodies and batch size and rejects
// both with -32600 before dispatch.
func TestCSharpGeneratedServerEnforcesLimits(t *testing.T) {
	idl := parseTestIDL(t, "namespace test\ninterface Svc { Ping() string }\n")
	paths := NewCSharpNamespacePaths("", "")
	code := generateServerCs(idl, map[string]*NamespaceTypes{}, nil, nil, "{}", paths)

	for _, want := range []string{
		"private const int MaxRequestBodyBytes = 1024 * 1024;",
		"private const int MaxBatchSize = 100;",
		`"Request body too large"`,
		`"Batch exceeds maximum size"`,
		"batchLength > MaxBatchSize",
		"ReadBodyWithLimitAsync",
	} {
		if !strings.Contains(code, want) {
			t.Errorf("generated server missing limit handling %q", want)
		}
	}
}

// R22/D3: non-POST requests return 405 with a JSON-RPC error body.
func TestCSharpGeneratedServerNonPostReturnsJsonRpc405(t *testing.T) {
	idl := parseTestIDL(t, "namespace test\ninterface Svc { Ping() string }\n")
	paths := NewCSharpNamespacePaths("", "")
	code := generateServerCs(idl, map[string]*NamespaceTypes{}, nil, nil, "{}", paths)

	for _, want := range []string{
		"context.Response.StatusCode = 405;",
		`"Invalid Request - only POST allowed"`,
	} {
		if !strings.Contains(code, want) {
			t.Errorf("generated server missing 405 JSON-RPC body %q", want)
		}
	}
}

// R24: ids other than string/number/null are invalid requests.
func TestCSharpGeneratedServerValidatesIdType(t *testing.T) {
	idl := parseTestIDL(t, "namespace test\ninterface Svc { Ping() string }\n")
	paths := NewCSharpNamespacePaths("", "")
	code := generateServerCs(idl, map[string]*NamespaceTypes{}, nil, nil, "{}", paths)

	for _, want := range []string{
		"requestId is bool || requestId is System.Collections.IList || requestId is System.Collections.IDictionary",
		`"id must be a string, number, or null"`,
	} {
		if !strings.Contains(code, want) {
			t.Errorf("generated server missing id validation %q", want)
		}
	}
}

// M18/M19/M20/M28: batch and params edge cases are handled explicitly.
func TestCSharpGeneratedServerBatchAndParamsShape(t *testing.T) {
	idl := parseTestIDL(t, "namespace test\ninterface Svc { Ping() string }\n")
	paths := NewCSharpNamespacePaths("", "")
	code := generateServerCs(idl, map[string]*NamespaceTypes{}, nil, nil, "{}", paths)

	for _, want := range []string{
		`"Batch must not be empty"`,
		`"Batch entry must be an object"`,
		`"Request must be an object"`,
		"paramsObj is System.Collections.IDictionary namedParams",
		`"Unknown parameter '{keyName}'"`,
		"// M28: an all-notification batch produces no body.",
	} {
		if !strings.Contains(code, want) {
			t.Errorf("generated server missing batch/params handling %q", want)
		}
	}

	// Malformed JSON must be -32700 with a fixed message (M26/R15).
	if !strings.Contains(code, `ErrorResponse(null, -32700, "Parse error")`) {
		t.Error("generated server missing -32700 parse error handling")
	}
}

// C4/C5: the embedded IDL is a JSON object carrying rootNamespace and checksum,
// and pulserpc-idl serves the parsed object rather than a string.
func TestCSharpEmbeddedIdlCarriesMetadata(t *testing.T) {
	idl := parseTestIDL(t, "namespace demo\ninterface Svc { Ping() string }\n")
	outDir := t.TempDir()
	runCSharpGenerate(t, idl, outDir)

	// Generate computes and assigns the checksum; the embedded IDL must carry it.
	if idl.Checksum == "" {
		t.Fatal("Generate did not assign a checksum")
	}

	for _, file := range []string{"Server.cs", "Client.cs"} {
		data, err := os.ReadFile(filepath.Join(outDir, file))
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}
		code := string(data)
		for _, want := range []string{"rootNamespace", "demo", idl.Checksum, "checksum"} {
			if !strings.Contains(code, want) {
				t.Errorf("%s missing IDL metadata %q", file, want)
			}
		}
	}

	client, err := os.ReadFile(filepath.Join(outDir, "Client.cs"))
	if err != nil {
		t.Fatalf("read Client.cs: %v", err)
	}
	if !strings.Contains(string(client), "public static string GetIDLJson() => IDL_JSON;") {
		t.Error("generated client missing IdlHolder.GetIDLJson()")
	}

	server, err := os.ReadFile(filepath.Join(outDir, "Server.cs"))
	if err != nil {
		t.Fatalf("read Server.cs: %v", err)
	}
	if !strings.Contains(string(server), `JsonSerializer.Deserialize<object>(_idlJson)`) {
		t.Error("generated server does not serve the IDL as an object")
	}
}

// C11/R27: generated client and server use the string enum converter.
func TestCSharpGeneratedCodeUsesStringEnums(t *testing.T) {
	idl := parseTestIDL(t, "namespace test\nenum Color { red green }\ninterface Svc { Ping() string }\n")
	paths := NewCSharpNamespacePaths("", "")
	server := generateServerCs(idl, map[string]*NamespaceTypes{}, nil, nil, "{}", paths)
	client := generateClientCs(idl, nil, nil, map[string]*NamespaceTypes{}, "{}", paths)

	for name, code := range map[string]string{"server": server, "client": client} {
		if !strings.Contains(code, "JsonStringEnumConverter") {
			t.Errorf("generated %s does not configure string enums", name)
		}
	}
}

// R31: the generated client fetches pulserpc-idl and reports a failed fetch as
// incompatible rather than diffing against itself.
func TestCSharpGeneratedClientVerifiesAgainstFetchedIdl(t *testing.T) {
	idl := parseTestIDL(t, "namespace test\ninterface Svc { Ping() string }\n")
	paths := NewCSharpNamespacePaths("", "")
	code := generateClientCs(idl, nil, nil, map[string]*NamespaceTypes{}, "{}", paths)

	for _, want := range []string{
		`_transport.CallAsync("pulserpc-idl", Array.Empty<object>())`,
		`fetchError = "failed to fetch server IDL: "`,
		"if (fetchError != null || _serverIDL == null)",
		"Compatible = false",
	} {
		if !strings.Contains(code, want) {
			t.Errorf("generated client missing verification behavior %q", want)
		}
	}
}
