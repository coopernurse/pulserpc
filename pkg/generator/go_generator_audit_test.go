package generator

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func embeddedIDLJSON(t *testing.T, code string) string {
	t.Helper()
	const marker = "const IDL_JSON = "
	idx := strings.Index(code, marker)
	if idx < 0 {
		t.Fatal("generated server has no IDL_JSON constant")
	}
	rest := code[idx+len(marker):]
	end := strings.IndexByte(rest, '\n')
	if end < 0 {
		end = len(rest)
	}
	literal := strings.TrimSpace(rest[:end])
	decoded, err := strconv.Unquote(literal)
	if err != nil {
		t.Fatalf("IDL_JSON is not a valid Go string literal: %v\n%s", err, literal)
	}
	return decoded
}

// core-14: idl.json and the embedded IDL_JSON must both carry the checksum.
func TestGoGeneratorEmbedsChecksumInBothForms(t *testing.T) {
	idl := parseTestIDL(t, `namespace test
interface Svc { Ping() string }
struct Item { name string }
`)
	dir := t.TempDir()
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	idlJSON, err := writeIDLJSONGo(idl, dir, fs)
	if err != nil {
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
	checksum, _ := fromFile["checksum"].(string)
	if checksum == "" {
		t.Fatal("idl.json has no checksum")
	}

	code := generateServerGo(idl, idlJSON, nil, nil, "test", map[string]*NamespaceTypes{}, "pulserpc", "example.com/rpc")
	embedded := embeddedIDLJSON(t, code)
	var fromConst map[string]interface{}
	if err := json.Unmarshal([]byte(embedded), &fromConst); err != nil {
		t.Fatalf("unmarshal IDL_JSON: %v", err)
	}
	embeddedChecksum, _ := fromConst["checksum"].(string)
	if embeddedChecksum == "" {
		t.Fatal("generated IDL_JSON constant has no checksum key")
	}
	if embeddedChecksum != checksum {
		t.Errorf("IDL_JSON checksum %q != idl.json checksum %q", embeddedChecksum, checksum)
	}
}

// core-15: the generated /idl.json handler must serve the JSON object, not a
// JSON-encoded string.
func TestGoGeneratedIDLEndpointServesObject(t *testing.T) {
	idl := parseTestIDL(t, `namespace test
interface Svc { Ping() string }
`)
	// Mirror the real Generate order: idl.json is written before server.go and
	// its bytes are reused for IDL_JSON.
	idlJSON, err := writeIDLJSONGo(idl, t.TempDir(), flag.NewFlagSet("test", flag.ContinueOnError))
	if err != nil {
		t.Fatalf("writeIDLJSONGo: %v", err)
	}
	serverCode := generateServerGo(idl, idlJSON, nil, nil, "test", map[string]*NamespaceTypes{}, "pulserpc", "example.com/rpc")
	testServerCode := generateTestServerGo(idl, nil, nil, "pulserpc", "example.com/rpc", map[string]*NamespaceTypes{}, "")

	if !strings.Contains(testServerCode, "w.Write([]byte(IDL_JSON))") {
		t.Errorf("generated /idl.json handler must write IDL_JSON directly:\n%s", testServerCode)
	}
	if strings.Contains(testServerCode, "json.Marshal(IDL_JSON)") {
		t.Errorf("generated /idl.json handler still double-encodes IDL_JSON")
	}

	// The payload unmarshals into an object with checksum + interfaces.
	var obj map[string]interface{}
	if err := json.Unmarshal([]byte(embeddedIDLJSON(t, serverCode)), &obj); err != nil {
		t.Fatalf("IDL_JSON does not unmarshal into an object: %v", err)
	}
	if _, ok := obj["checksum"].(string); !ok {
		t.Error("IDL_JSON has no string checksum")
	}
	if _, ok := obj["interfaces"]; !ok {
		t.Error("IDL_JSON has no interfaces key")
	}
}
