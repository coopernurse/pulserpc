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

// C2/C4/C5: idl.json and the embedded IDL_JSON must carry the same schema and
// the same checksum, including rootNamespace.
func TestGoIDLJSONSchemaAndChecksumAgree(t *testing.T) {
	idl := parseTestIDL(t, `namespace test
interface Svc { Ping() string }
struct Item { name string }
enum Status { active inactive }
`)
	dir := t.TempDir()
	if err := writeIDLJSONGo(idl, dir, flag.NewFlagSet("test", flag.ContinueOnError)); err != nil {
		t.Fatalf("writeIDLJSONGo: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "idl.json"))
	if err != nil {
		t.Fatalf("read idl.json: %v", err)
	}
	var fromFile map[string]interface{}
	if err := json.Unmarshal(data, &fromFile); err != nil {
		t.Fatalf("unmarshal idl.json: %v", err)
	}

	serverCode := generateServerGo(idl, nil, nil, "test", map[string]*NamespaceTypes{}, "pulserpc", "example.com/rpc")
	var fromConst map[string]interface{}
	if err := json.Unmarshal([]byte(embeddedIDLJSON(t, serverCode)), &fromConst); err != nil {
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
		"http.MaxBytesReader(w, r.Body, maxRequestBodySize)",
		"\"Request body too large\"",
		"len(requests) > maxBatchSize",
		"\"Batch exceeds maximum size\"",
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
	serverCode := generateServerGo(idl, nil, nil, "test", map[string]*NamespaceTypes{}, "pulserpc", "example.com/rpc")
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

	first := generateNamespaceGo("test", types, structMap, map[string]*parser.Enum{}, "example.com/rpc")
	for i := 0; i < 40; i++ {
		if got := generateNamespaceGo("test", types, structMap, map[string]*parser.Enum{}, "example.com/rpc"); got != first {
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
// float maps to float64.
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
	code := generateNamespaceGo("test", namespaceMap["test"], structMap, enumMap, "example.com/rpc")

	for _, marker := range []string{
		"`json:\"name\"`",
		"`json:\"note,omitempty\"`",
		"type Status string",
		"StatusActive Status = \"active\"",
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
