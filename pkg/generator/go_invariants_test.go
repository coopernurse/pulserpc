package generator

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/coopernurse/pulserpc/pkg/parser"
)

// marshalTestIDLJSON returns the shared idl.json bytes for a parsed IDL, in
// the same shape the generator passes to both writers.
func marshalTestIDLJSON(t *testing.T, idl *parser.IDL) []byte {
	t.Helper()
	data, err := MarshalIDLJSON(idl)
	if err != nil {
		t.Fatalf("marshalIDLMetadataGo: %v", err)
	}
	return data
}

// go-02/R19: the generated HTTP server sets read/write/idle timeouts and a
// header size cap so slow or header-flooding clients cannot pin a connection.
func TestGoGeneratedServerSetsTimeouts(t *testing.T) {
	idl := parseTestIDL(t, "namespace test\ninterface Svc { Ping() string }\n")
	code := generateServerGo(idl, marshalTestIDLJSON(t, idl), nil, nil, "test", map[string]*NamespaceTypes{}, "pulserpc", "example.com/rpc")

	for _, marker := range []string{
		"ReadHeaderTimeout: 10 * time.Second",
		"ReadTimeout:       30 * time.Second",
		"WriteTimeout:      30 * time.Second",
		"IdleTimeout:       60 * time.Second",
		"MaxHeaderBytes:    1 << 20",
	} {
		if !strings.Contains(code, marker) {
			t.Errorf("generated server is missing %q", marker)
		}
	}
	if !strings.Contains(code, "\t\"time\"\n") {
		t.Error("generated server does not import time for the http.Server timeouts")
	}
}

// go-04/R17: namespace lookup keeps D16 determinism without sorting or
// allocating the key set on a miss.
func TestGoNamespaceLookupIsDeterministicAndAllocationFree(t *testing.T) {
	structMap := map[string]*parser.Struct{
		"z.Widget": {Name: "z.Widget", Namespace: "z"},
		"a.Widget": {Name: "a.Widget", Namespace: "a"},
		"m.Widget": {Name: "m.Widget", Namespace: "m"},
		"b.Gadget": {Name: "b.Gadget", Namespace: "b"},
	}
	enumMap := map[string]*parser.Enum{
		"z.Color": {Name: "z.Color", Namespace: "z"},
		"b.Color": {Name: "b.Color", Namespace: "b"},
	}

	for i := 0; i < 100; i++ {
		if got := findNamespaceForType("Widget", structMap, enumMap); got != "a" {
			t.Fatalf("duplicate struct base name resolved to %q, want the sorted-first namespace a", got)
		}
		if got := findNamespaceForType("Color", structMap, enumMap); got != "b" {
			t.Fatalf("enum base name resolved to %q, want the sorted-first namespace b", got)
		}
		// Structs win over enums when both declare the base name.
		if got := findNamespaceForType("Gadget", structMap, map[string]*parser.Enum{"a.Gadget": {Name: "a.Gadget", Namespace: "a"}}); got != "b" {
			t.Fatalf("struct/enum conflict resolved to %q, want struct namespace b", got)
		}
		// Qualified names resolve directly and unknown names miss.
		if got := findNamespaceForType("q.Widget", structMap, enumMap); got != "q" {
			t.Fatalf("qualified name resolved to %q, want q", got)
		}
		if got := findNamespaceForType("Missing", structMap, enumMap); got != "" {
			t.Fatalf("unknown name resolved to %q, want empty", got)
		}
	}

	// The miss path must not allocate: the old implementation built and sorted
	// a key slice on every lookup.
	if allocs := testing.AllocsPerRun(100, func() {
		findNamespaceForType("Missing", structMap, enumMap)
	}); allocs != 0 {
		t.Errorf("namespace miss allocated %v times per run, want 0", allocs)
	}
}

// C2/C4/C5: idl.json and the embedded IDL_JSON must carry the same schema and
// the same checksum, including rootNamespace.
func TestGoIDLJSONSchemaAndChecksumAgree(t *testing.T) {
	idl := parseTestIDL(t, `namespace test
interface Svc { Ping() string }
struct Item { name string }
enum Status { active inactive }
`)
	dir := t.TempDir()
	idlJSON, err := writeIDLJSONGo(idl, dir, flag.NewFlagSet("test", flag.ContinueOnError))
	if err != nil {
		t.Fatalf("writeIDLJSONGo: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "idl.json"))
	if err != nil {
		t.Fatalf("read idl.json: %v", err)
	}
	if string(data) != string(idlJSON) {
		t.Fatalf("idl.json differs from the bytes returned by writeIDLJSONGo")
	}
	var fromFile map[string]interface{}
	if err := json.Unmarshal(data, &fromFile); err != nil {
		t.Fatalf("unmarshal idl.json: %v", err)
	}

	serverCode := generateServerGo(idl, idlJSON, nil, nil, "test", map[string]*NamespaceTypes{}, "pulserpc", "example.com/rpc")
	embedded := embeddedIDLJSON(t, serverCode)
	// go-05: the IDL is marshalled once; IDL_JSON must be byte-identical to
	// idl.json rather than a second marshalling of the same struct.
	if embedded != string(data) {
		t.Errorf("IDL_JSON is not byte-identical to idl.json:\n--- idl.json ---\n%s\n--- IDL_JSON ---\n%s", data, embedded)
	}
	var fromConst map[string]interface{}
	if err := json.Unmarshal([]byte(embedded), &fromConst); err != nil {
		t.Fatalf("unmarshal IDL_JSON: %v", err)
	}

	for _, key := range []string{"rootNamespace", "checksum", "interfaces", "structs", "enums", "errors"} {
		if _, ok := fromFile[key]; !ok {
			t.Errorf("idl.json is missing top-level key %q", key)
		}
		if _, ok := fromConst[key]; !ok {
			t.Errorf("IDL_JSON is missing top-level key %q", key)
		}
	}
	if fromFile["rootNamespace"] != "test" {
		t.Errorf("idl.json rootNamespace = %#v, want test", fromFile["rootNamespace"])
	}
	if fromConst["rootNamespace"] != "test" {
		t.Errorf("IDL_JSON rootNamespace = %#v, want test", fromConst["rootNamespace"])
	}
	if fromFile["checksum"] == "" || fromFile["checksum"] == nil {
		t.Error("idl.json has no checksum")
	}
	if fromFile["checksum"] != fromConst["checksum"] {
		t.Errorf("checksums differ: file=%#v const=%#v", fromFile["checksum"], fromConst["checksum"])
	}
}

// R16/R18/R22/M18/M19/M22/M26: the generated adapter enforces body, batch, and
// method limits and returns JSON-RPC error bodies with a null id.
func TestGoGeneratedServerLimitsAndErrorShape(t *testing.T) {
	var sb strings.Builder
	writeServerHandleRequestGoRefactored(&sb)
	code := sb.String()

	for _, marker := range []string{
		"const maxRequestBodySize = 1 << 20",
		"const maxBatchSize = 100",
		"const maxBatchResponseSize = 10 * 1024 * 1024",
		"config := s.rpcServer.Config()",
		"bodyLimit := int64(maxRequestBodySize)",
		"if config.MaxRequestBodyBytes > 0 {",
		"http.MaxBytesReader(w, r.Body, bodyLimit)",
		"\"Request body too large\"",
		"if config.RejectInvalidUTF8 && !utf8.Valid(body) {",
		"decoder.UseNumber()",
		"decoder.Token()",
		"pulserpc.NormalizeRequestNumbers(requestData)",
		"isBatch && !config.AllowBatch",
		"len(requests) > maxBatchSize",
		"\"Batch exceeds maximum size\"",
		"len(payload) > maxBatchResponseSize",
		"\"Batch response too large\"",
		"len(requests) == 0",
		"errorResponse(nil, -32600, \"Invalid Request\", nil)",
		"\"Invalid Request - only POST allowed\"",
		"w.WriteHeader(http.StatusMethodNotAllowed)",
		"errorResponse(nil, -32600, \"Invalid Request - only POST allowed\", nil)",
		"-32700, \"Parse error\"",
		"if data != nil {\n\t\terrObj[\"data\"] = data\n\t}",
		"\"id\":      requestID",
	} {
		if !strings.Contains(code, marker) {
			t.Errorf("generated server is missing %q", marker)
		}
	}

	// The generated integration test server must reuse the adapter so batches
	// and the other limit/error behavior are exercised end to end.
	idl := parseTestIDL(t, "namespace test\ninterface Svc { Ping() string }\n")
	serverCode := generateServerGo(idl, marshalTestIDLJSON(t, idl), nil, nil, "test", map[string]*NamespaceTypes{}, "pulserpc", "example.com/rpc")
	if !strings.Contains(serverCode, "func (s *PulseRPCServer) ServeHTTP(") {
		t.Error("generated server does not expose ServeHTTP for adapters/tests")
	}
	testServer := generateTestServerGo(idl, nil, nil, "example.com/rpc/test", "example.com/rpc", nil, "test")
	if !strings.Contains(testServer, `mux.Handle("/", server)`) {
		t.Error("generated test server does not reuse the generated HTTP adapter")
	}
	if strings.Contains(testServer, "json.NewDecoder(r.Body)") {
		t.Error("generated test server still has its own divergent request decoder")
	}
}

// R17/R19/R29/R30: the generated transport bounds responses, sets a 30-second
// timeout, sends custom headers, and normalizes a trailing slash.
func TestGoGeneratedTransportHardening(t *testing.T) {
	var sb strings.Builder
	writeHTTPTransportGo(&sb)
	code := sb.String()

	for _, marker := range []string{
		"const maxResponseSize = 10 * 1024 * 1024",
		"&http.Client{Timeout: 30 * time.Second}",
		"strings.TrimSuffix(baseURL, \"/\")",
		"for k, v := range t.headers {",
		"req.Header.Set(k, v)",
	} {
		if !strings.Contains(code, marker) {
			t.Errorf("generated transport is missing %q", marker)
		}
	}

	clientCode := generatedGoClient(t, "namespace test\ninterface Svc { Ping() string }\n")
	if !strings.Contains(clientCode, "\t\"time\"\n") {
		t.Error("generated client does not import time for the HTTP timeout")
	}
}

// C1/D12/D16: map-derived output must be byte-deterministic and duplicate
// base names must resolve in sorted order.
func TestGoGenerationIsDeterministic(t *testing.T) {
	holder := &parser.Struct{
		Name:      "test.Holder",
		Namespace: "test",
		Fields: []*parser.Field{
			{Name: "one", Type: &parser.Type{UserDefined: "AType"}},
			{Name: "two", Type: &parser.Type{UserDefined: "BType"}},
		},
	}
	structMap := map[string]*parser.Struct{
		"a.AType":     {Name: "a.AType", Namespace: "a"},
		"b.BType":     {Name: "b.BType", Namespace: "b"},
		"test.Holder": holder,
	}
	types := &NamespaceTypes{Structs: []*parser.Struct{holder}}

	first := generateNamespaceGo("test", types, structMap, map[string]*parser.Enum{}, "example.com/rpc", false)
	for i := 0; i < 40; i++ {
		if got := generateNamespaceGo("test", types, structMap, map[string]*parser.Enum{}, "example.com/rpc", false); got != first {
			t.Fatalf("generation is not byte-deterministic on run %d:\n--- first ---\n%s\n--- got ---\n%s", i, first, got)
		}
	}

	duplicate := map[string]*parser.Struct{
		"z.Widget": {Name: "z.Widget", Namespace: "z"},
		"a.Widget": {Name: "a.Widget", Namespace: "a"},
	}
	for i := 0; i < 40; i++ {
		if got := findNamespaceForType("Widget", duplicate, nil); got != "a" {
			t.Fatalf("duplicate base-name resolution = %q, want sorted first key namespace a", got)
		}
	}
}

// C9/C11/R26/R27: optional fields use omitempty, enums are string types, and
// int maps to int64 while float maps to float64.
func TestGoStructTagsEnumsAndFloats(t *testing.T) {
	idl := parseTestIDL(t, `namespace test
enum Status { active inactive }
struct Item {
    name string
    note string [optional]
    count int
    ratio float
    status Status
}
interface Svc { get() Item }
`)
	structMap := make(map[string]*parser.Struct)
	enumMap := make(map[string]*parser.Enum)
	for _, s := range idl.Structs {
		structMap[s.Name] = s
	}
	for _, e := range idl.Enums {
		enumMap[e.Name] = e
	}
	namespaceMap := GroupTypesByNamespace(idl)
	code := generateNamespaceGo("test", namespaceMap["test"], structMap, enumMap, "example.com/rpc", false)

	for _, marker := range []string{
		"`json:\"name\"`",
		"`json:\"note,omitempty\"`",
		"type Status string",
		"StatusActive Status = \"active\"",
		"Count int64",
		"Ratio float64",
		"Status Status",
	} {
		if !strings.Contains(code, marker) {
			t.Errorf("generated types are missing %q:\n%s", marker, code)
		}
	}
	if strings.Contains(code, "`json:\"name,omitempty\"`") {
		t.Error("required field name uses omitempty")
	}
}

// IDL int is signed 64-bit: Go maps it to int64 in every position.
func TestGoIntMapsToInt64(t *testing.T) {
	intType := &parser.Type{BuiltIn: "int"}
	cases := []struct {
		name     string
		typ      *parser.Type
		optional bool
		want     string
	}{
		{"scalar", intType, false, "int64"},
		{"optional", intType, true, "*int64"},
		{"array", &parser.Type{Array: intType}, false, "[]int64"},
		{"optional array", &parser.Type{Array: intType}, true, "[]int64"},
		{"map", &parser.Type{MapValue: intType}, false, "map[string]int64"},
		{"optional map", &parser.Type{MapValue: intType}, true, "map[string]int64"},
	}
	for _, tc := range cases {
		if got := mapTypeToGoType(tc.typ, nil, nil, tc.optional); got != tc.want {
			t.Errorf("%s: mapTypeToGoType = %q, want %q", tc.name, got, tc.want)
		}
		if got := getGoTypeWithNamespace(tc.typ, nil, nil, tc.optional, "test", nil, "example.com/rpc"); got != tc.want {
			t.Errorf("%s: getGoTypeWithNamespace = %q, want %q", tc.name, got, tc.want)
		}
	}
}
