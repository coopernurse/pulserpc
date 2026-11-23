package generator

import (
	"flag"
	"fmt"
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

// runCSharpGenerateWithPackage runs the C# generator with an explicit -package base.
func runCSharpGenerateWithPackage(t *testing.T, idl *parser.IDL, outDir, packageBase string) {
	t.Helper()
	fs := flag.NewFlagSet("csharp", flag.ContinueOnError)
	fs.String("dir", outDir, "")
	fs.String("package", packageBase, "")
	fs.Bool("silent", true, "")
	fs.Bool("generate-test-files", true, "")
	if err := NewCSharpClientServer().Generate(idl, fs); err != nil {
		t.Fatalf("Generate with -package %q: %v", packageBase, err)
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

// buildCSharpNamedResolutionIDL is a multi-namespace fixture with
// cross-namespace references, two structs sharing a base name, and several
// interfaces/enums. A single-namespace fixture cannot expose map-ordered
// emission (C1/D12) or D16 resolution.
func buildCSharpNamedResolutionIDL() *parser.IDL {
	return &parser.IDL{
		RootNamespace: "app",
		Structs: []*parser.Struct{
			{
				Name:      "common.Address",
				Namespace: "common",
				Fields: []*parser.Field{
					{Name: "city", Type: &parser.Type{BuiltIn: "string"}},
					{Name: "zip", Type: &parser.Type{BuiltIn: "string"}, Optional: true},
				},
			},
			{
				Name:      "zzz.Address",
				Namespace: "zzz",
				Fields: []*parser.Field{
					{Name: "line1", Type: &parser.Type{BuiltIn: "string"}},
					{Name: "line2", Type: &parser.Type{BuiltIn: "string"}, Optional: true},
				},
			},
			{
				Name:      "app.Order",
				Namespace: "app",
				Fields: []*parser.Field{
					{Name: "id", Type: &parser.Type{BuiltIn: "string"}},
					{Name: "address", Type: &parser.Type{UserDefined: "Address"}},
					{Name: "status", Type: &parser.Type{UserDefined: "common.Status"}},
				},
			},
			{
				Name:      "user.Profile",
				Namespace: "user",
				Fields: []*parser.Field{
					{Name: "name", Type: &parser.Type{BuiltIn: "string"}},
					{Name: "role", Type: &parser.Type{UserDefined: "Role"}},
				},
			},
		},
		Enums: []*parser.Enum{
			{
				Name:      "common.Status",
				Namespace: "common",
				Values:    []*parser.EnumValue{{Name: "new"}, {Name: "done"}},
			},
			{
				Name:      "user.Role",
				Namespace: "user",
				Values:    []*parser.EnumValue{{Name: "admin"}, {Name: "member"}},
			},
		},
		Interfaces: []*parser.Interface{
			{
				Name:      "OrderService",
				Namespace: "app",
				Methods: []*parser.Method{
					{
						Name:       "create",
						Parameters: []*parser.Parameter{{Name: "order", Type: &parser.Type{UserDefined: "Order"}}},
						ReturnType: &parser.Type{UserDefined: "Order"},
					},
					{Name: "ping", ReturnType: &parser.Type{BuiltIn: "string"}},
				},
			},
			{
				Name:      "UserService",
				Namespace: "user",
				Methods: []*parser.Method{
					{
						Name:       "find",
						Parameters: []*parser.Parameter{{Name: "name", Type: &parser.Type{BuiltIn: "string"}}},
						ReturnType: &parser.Type{UserDefined: "Profile"},
					},
				},
			},
			{
				Name:      "StatusService",
				Namespace: "common",
				Methods: []*parser.Method{
					{Name: "status", ReturnType: &parser.Type{UserDefined: "Status"}},
				},
			},
		},
	}
}

// C1/D12: every map-derived emission site (namespaces, cross-namespace
// imports, IdlData registration) must be byte-deterministic for a
// multi-namespace contract.
func TestCSharpMultiNamespaceGenerationIsByteDeterministic(t *testing.T) {
	idl := buildCSharpNamedResolutionIDL()
	base := t.TempDir()

	var trees []map[string][]byte
	for i := 0; i < 4; i++ {
		dir := filepath.Join(base, fmt.Sprintf("run%d", i))
		runCSharpGenerate(t, idl, dir)
		trees = append(trees, readTree(t, dir))
	}

	first := trees[0]
	if len(first) == 0 {
		t.Fatal("generator produced no files")
	}
	var paths []string
	for p := range first {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	for i, tree := range trees[1:] {
		if len(tree) != len(first) {
			t.Fatalf("run %d file count differs: %d vs %d", i+1, len(tree), len(first))
		}
		for _, p := range paths {
			if string(tree[p]) != string(first[p]) {
				t.Errorf("generated file differs between runs (run %d): %s", i+1, p)
			}
		}
	}

	// Type definitions (and their cross-namespace imports) live in Types.cs;
	// the legacy per-namespace file now holds only IDL metadata.
	typesFile := string(first["App/Types.cs"])
	commonIdx := strings.Index(typesFile, "using Common;")
	userIdx := strings.Index(typesFile, "using User;")
	zzzIdx := strings.Index(typesFile, "using Zzz;")
	if commonIdx < 0 || userIdx < 0 || zzzIdx < 0 {
		t.Fatalf("App/Types.cs missing cross-namespace imports:\n%s", typesFile)
	}
	if !(commonIdx < userIdx && userIdx < zzzIdx) {
		t.Errorf("cross-namespace imports are not sorted: common=%d user=%d zzz=%d", commonIdx, userIdx, zzzIdx)
	}

	// The IdlData registration loop follows the same sorted namespace order.
	contract := string(first["Contract.cs"])
	appReg := strings.Index(contract, ".appIdl.ALL_STRUCTS")
	commonReg := strings.Index(contract, ".commonIdl.ALL_STRUCTS")
	userReg := strings.Index(contract, ".userIdl.ALL_STRUCTS")
	if appReg < 0 || commonReg < 0 || userReg < 0 {
		t.Fatalf("Contract.cs missing IdlData registrations:\n%s", contract)
	}
	if !(appReg < commonReg && commonReg < userReg) {
		t.Errorf("IdlData registration is not sorted: app=%d common=%d user=%d", appReg, commonReg, userReg)
	}
}

// D16: type-name resolution is exact key -> exact base-name key -> first key
// in sorted order sharing the base name, in both helpers.
func TestCSharpTypeNameResolutionFollowsD16(t *testing.T) {
	structMap := map[string]*parser.Struct{
		"beta.Address":  {Name: "beta.Address", Namespace: "beta"},
		"alpha.Address": {Name: "alpha.Address", Namespace: "alpha"},
	}
	enumMap := map[string]*parser.Enum{}

	// 1. An exact key match wins even though a sorted fallback exists.
	if got := getStructOrEnumTypeName("beta.Address", structMap, enumMap, "app", nil); got != "Beta.Address" {
		t.Errorf("exact qualified key resolved to %q, want Beta.Address", got)
	}

	// 2. An exact base-name key wins over the sorted fallback.
	structMap["Address"] = &parser.Struct{Name: "Address", Namespace: "app"}
	if got := getStructOrEnumTypeName("Address", structMap, enumMap, "app", nil); got != "Address" {
		t.Errorf("exact base-name key resolved to %q, want Address", got)
	}

	// 3. Ambiguous base names resolve to the first key in sorted order, every
	// run (never a bare base name with no candidate).
	delete(structMap, "Address")
	for i := 0; i < 200; i++ {
		if got := getStructOrEnumTypeName("Address", structMap, enumMap, "app", nil); got != "Alpha.Address" {
			t.Fatalf("sorted fallback resolved to %q on run %d, want Alpha.Address", got, i)
		}
	}

	// Enums use the same ordering.
	enumKeys := map[string]*parser.Enum{
		"zeta.Status":  {Name: "zeta.Status", Namespace: "zeta"},
		"delta.Status": {Name: "delta.Status", Namespace: "delta"},
	}
	for i := 0; i < 200; i++ {
		if got := getStructOrEnumTypeName("Status", structMap, enumKeys, "app", nil); got != "Delta.Status" {
			t.Fatalf("enum sorted fallback resolved to %q on run %d, want Delta.Status", got, i)
		}
	}

	// The namespace-free helper still returns a bare (deterministic) base name.
	if got := getStructOrEnumTypeNameSimple("Status", structMap, enumKeys); got != "Status" {
		t.Errorf("getStructOrEnumTypeNameSimple(Status) = %q, want Status", got)
	}
}

// D16: an unqualified field reference to a qualified type resolves to the
// qualified C# type in generated code.
func TestCSharpGeneratedCodeResolvesUnqualifiedQualifiedType(t *testing.T) {
	idl := buildCSharpNamedResolutionIDL()
	outDir := t.TempDir()
	runCSharpGenerate(t, idl, outDir)

	data, err := os.ReadFile(filepath.Join(outDir, "App", "Types.cs"))
	if err != nil {
		t.Fatalf("read App/Types.cs: %v", err)
	}
	code := string(data)

	// `address Address` is unqualified; candidates common.Address and
	// zzz.Address share the base name, so the sorted-first one wins.
	if !strings.Contains(code, "public Common.Address Address { get; set; }") {
		t.Errorf("unqualified Address did not resolve to Common.Address:\n%s", code)
	}
	if strings.Contains(code, "public zzz.Address") || strings.Contains(code, "public Zzz.Address") {
		t.Error("unqualified Address resolved to the non-first candidate")
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

// C10/D7: optional built-ins and collections must be nullable, and generated
// files must enable nullable reference types so `string?` is meaningful.
func TestCSharpOptionalBuiltInsAreNullable(t *testing.T) {
	idl := parseTestIDL(t, `
namespace demo

struct OptionalFields {
  opt_int int [optional]
  opt_bool bool [optional]
  opt_float float [optional]
  opt_string string [optional]
  opt_list []string [optional]
  opt_map map[string]int [optional]
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
		"public List<string>? OptList { get; set; }",
		"public Dictionary<string, int>? OptMap { get; set; }",
	} {
		if !strings.Contains(code, want) {
			t.Errorf("generated Types.cs missing %q\n%s", want, code)
		}
	}
}

// C10/D7: both type-mapping helpers (with and without namespace awareness)
// agree that optional collections are nullable.
func TestCSharpOptionalCollectionsAreNullableInBothMappers(t *testing.T) {
	listType := &parser.Type{Array: &parser.Type{BuiltIn: "string"}}
	mapType := &parser.Type{MapValue: &parser.Type{BuiltIn: "int"}}

	if got := mapTypeToCsType(listType, nil, nil, true); got != "List<string>?" {
		t.Errorf("mapTypeToCsType(optional []string) = %q, want List<string>?", got)
	}
	if got := mapTypeToCsType(mapType, nil, nil, true); got != "Dictionary<string, int>?" {
		t.Errorf("mapTypeToCsType(optional map[string]int) = %q, want Dictionary<string, int>?", got)
	}
	if got := mapTypeToCsTypeWithNamespace(listType, nil, nil, true, "", nil); got != "List<string>?" {
		t.Errorf("mapTypeToCsTypeWithNamespace(optional []string) = %q, want List<string>?", got)
	}
	if got := mapTypeToCsTypeWithNamespace(mapType, nil, nil, true, "", nil); got != "Dictionary<string, int>?" {
		t.Errorf("mapTypeToCsTypeWithNamespace(optional map[string]int) = %q, want Dictionary<string, int>?", got)
	}

	// Non-optional collections stay non-nullable.
	if got := mapTypeToCsType(listType, nil, nil, false); got != "List<string>" {
		t.Errorf("mapTypeToCsType([]string) = %q, want List<string>", got)
	}
	if got := mapTypeToCsType(mapType, nil, nil, false); got != "Dictionary<string, int>" {
		t.Errorf("mapTypeToCsType(map[string]int) = %q, want Dictionary<string, int>", got)
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
		// The bounded read is shared with the runtime instead of duplicated in
		// the generated adapter (quality follow-up).
		"HttpTransport.ReadLimitedAsync(context.Request.Body, MaxRequestBodyBytes)",
	} {
		if !strings.Contains(code, want) {
			t.Errorf("generated server missing limit handling %q", want)
		}
	}

	// The generated copy of the bounded reader must be gone.
	if strings.Contains(code, "ReadBodyWithLimitAsync") {
		t.Error("generated server still defines its own ReadBodyWithLimitAsync")
	}
	// Dead helper removed.
	if strings.Contains(code, "WriteErrorResponse") {
		t.Error("generated server still declares the unused WriteErrorResponse helper")
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

// R17/R19/D6: the generated C# client tree copies the runtime Transport.cs,
// which caps response bodies at 10 MiB and bounds both the request and connect
// phases of every HTTP call.
func TestCSharpGeneratedTransportHardening(t *testing.T) {
	idl := parseTestIDL(t, "namespace test\ninterface Svc { Ping() string }\n")
	outDir := t.TempDir()
	runCSharpGenerate(t, idl, outDir)

	tree := readTree(t, outDir)
	var transport string
	for rel, data := range tree {
		if filepath.Base(rel) == "Transport.cs" {
			transport = string(data)
			break
		}
	}
	if transport == "" {
		t.Fatal("generated client tree has no PulseRPC/Transport.cs")
	}

	for _, want := range []string{
		"MaxResponseBytes = 10L * 1024 * 1024",
		"RequestTimeout = TimeSpan.FromSeconds(30)",
		"ConnectTimeout = TimeSpan.FromSeconds(10)",
		"Timeout = RequestTimeout",
		"ReadLimitedAsync(responseStream, MaxResponseBytes",
		"Response body exceeds maximum size of",
	} {
		if !strings.Contains(transport, want) {
			t.Errorf("generated C# transport missing %q", want)
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

// D8: -package must qualify every generated namespace declaration, including
// the shared sources emitted at the output root. A single bare
// `namespace PulseRPC` would put Contract/Server/Client outside the rewritten
// runtime namespace and break `using <base>.PulseRPC;` resolution.
func TestCSharpPackageNamespaceAppliedToAllGeneratedFiles(t *testing.T) {
	const base = "MyApp.Lib.Rpc"
	const runtimeNS = base + ".PulseRPC"

	idl := buildCSharpNamedResolutionIDL()
	outDir := t.TempDir()
	runCSharpGenerateWithPackage(t, idl, outDir, base)

	tree := readTree(t, outDir)
	csCount := 0
	sawRuntimeImport := false
	for rel, data := range tree {
		if !strings.HasSuffix(rel, ".cs") {
			continue
		}
		csCount++
		code := string(data)
		if strings.Contains(code, "\nnamespace PulseRPC\n") {
			t.Errorf("%s still declares bare `namespace PulseRPC`", rel)
		}
		if strings.Contains(code, "\nusing PulseRPC;") {
			t.Errorf("%s still imports bare `using PulseRPC;`", rel)
		}
		if strings.Contains(code, "using "+runtimeNS+";") {
			sawRuntimeImport = true
		}
	}
	if csCount == 0 {
		t.Fatal("generator produced no .cs files")
	}
	if !sawRuntimeImport {
		t.Errorf("no generated file imports the configured runtime namespace %q", runtimeNS)
	}

	// The root-level shared sources must all live in the configured runtime
	// namespace, matching the copied/rewritten runtime sources.
	for _, name := range []string{"Contract.cs", "Server.cs", "Client.cs"} {
		data, ok := tree[name]
		if !ok {
			t.Fatalf("missing root file %s", name)
		}
		if !strings.Contains(string(data), "\nnamespace "+runtimeNS+"\n") {
			t.Errorf("root file %s does not declare `namespace %s`:\n%s", name, runtimeNS, data)
		}
	}

	// Namespace-scoped sources keep their namespace under the configured base.
	appData, ok := tree["MyApp/Lib/Rpc/App/App.cs"]
	if !ok {
		t.Fatal("missing MyApp/Lib/Rpc/App/App.cs")
	}
	if !strings.Contains(string(appData), "\nnamespace "+base+".") {
		t.Errorf("App.cs namespace is not scoped under %s:\n%s", base, appData)
	}

	// The runtime sources were copied into the package tree and rewritten to
	// the same namespace, so the generated `using <base>.PulseRPC;` resolves.
	runtimeData, ok := tree["MyApp/Lib/Rpc/PulseRPC/RPCError.cs"]
	if !ok {
		t.Fatal("missing MyApp/Lib/Rpc/PulseRPC/RPCError.cs")
	}
	// Copied runtime files retain the source file's line endings (CRLF on
	// Windows), unlike generated files which are written with LF.
	if !strings.Contains(string(runtimeData), "namespace "+runtimeNS) {
		t.Errorf("copied runtime source does not declare `namespace %s`:\n%s", runtimeNS, runtimeData)
	}
}

// Preserve the no -package behavior: sources remain in the bare PulseRPC
// namespace and the runtime stays at PulseRPC/.
func TestCSharpNoPackageKeepsBareNamespace(t *testing.T) {
	idl := buildCSharpNamedResolutionIDL()
	outDir := t.TempDir()
	runCSharpGenerate(t, idl, outDir)

	tree := readTree(t, outDir)
	for _, name := range []string{"Contract.cs", "Server.cs", "Client.cs"} {
		data, ok := tree[name]
		if !ok {
			t.Fatalf("missing root file %s", name)
		}
		code := string(data)
		if !strings.Contains(code, "\nnamespace PulseRPC\n") {
			t.Errorf("root file %s lost bare `namespace PulseRPC`:\n%s", name, code)
		}
		if !strings.Contains(code, "\nusing PulseRPC;") {
			t.Errorf("root file %s lost bare `using PulseRPC;`:\n%s", name, code)
		}
	}
	if _, ok := tree["PulseRPC/RPCError.cs"]; !ok {
		t.Error("runtime source was not copied to PulseRPC/ without -package")
	}
}
