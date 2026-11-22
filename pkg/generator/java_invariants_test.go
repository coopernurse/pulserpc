package generator

import (
	"encoding/json"
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/coopernurse/pulserpc/pkg/parser"
)

// generateJavaFixture runs the Java generator into dir and returns nothing;
// failures are fatal.
func generateJavaFixture(t *testing.T, idl *parser.IDL, dir string, testFiles bool) {
	t.Helper()
	generateJavaFixtureWithLib(t, idl, dir, testFiles, "jackson")
}

// generateJavaFixtureWithLib is generateJavaFixture with an explicit JSON
// library selection.
func generateJavaFixtureWithLib(t *testing.T, idl *parser.IDL, dir string, testFiles bool, jsonLib string) {
	t.Helper()
	p := NewJavaClientServer()
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	fs.String("dir", "", "output dir")
	fs.Bool("generate-test-files", false, "generate test files")
	p.RegisterFlags(fs)
	if err := fs.Set("dir", dir); err != nil {
		t.Fatalf("set dir: %v", err)
	}
	if err := fs.Set("package", "com.example.rpc"); err != nil {
		t.Fatalf("set package: %v", err)
	}
	if err := fs.Set("json-lib", jsonLib); err != nil {
		t.Fatalf("set json-lib: %v", err)
	}
	if testFiles {
		if err := fs.Set("generate-test-files", "true"); err != nil {
			t.Fatalf("set generate-test-files: %v", err)
		}
	}
	if err := p.Generate(idl, fs); err != nil {
		t.Fatalf("Generate: %v", err)
	}
}

func snapshotJavaTree(t *testing.T, root string) map[string]string {
	t.Helper()
	files := make(map[string]string)
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		files[filepath.ToSlash(rel)] = string(data)
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
	return files
}

func determinismFixtureIDL() *parser.IDL {
	return &parser.IDL{
		RootNamespace: "shop",
		Structs: []*parser.Struct{
			{
				Name:      "common.Address",
				Namespace: "common",
				Fields: []*parser.Field{
					{Name: "street", Type: &parser.Type{BuiltIn: "string"}},
					{Name: "zip", Type: &parser.Type{BuiltIn: "string"}, Optional: true},
				},
			},
			{
				Name:      "book.Book",
				Namespace: "book",
				Fields: []*parser.Field{
					{Name: "title", Type: &parser.Type{BuiltIn: "string"}},
					{Name: "pages", Type: &parser.Type{BuiltIn: "int"}},
				},
			},
			{
				Name:      "shop.Order",
				Namespace: "shop",
				Fields: []*parser.Field{
					{Name: "ship", Type: &parser.Type{UserDefined: "common.Address"}},
					{Name: "book", Type: &parser.Type{UserDefined: "book.Book"}},
					{Name: "status", Type: &parser.Type{UserDefined: "shop.Status"}},
					{Name: "qty", Type: &parser.Type{BuiltIn: "int"}, Optional: true},
					{Name: "tags", Type: &parser.Type{Array: &parser.Type{BuiltIn: "string"}}},
				},
			},
			{
				Name:      "user.User",
				Namespace: "user",
				Fields: []*parser.Field{
					{Name: "name", Type: &parser.Type{BuiltIn: "string"}},
					{Name: "home", Type: &parser.Type{UserDefined: "common.Address"}},
				},
			},
		},
		Enums: []*parser.Enum{
			{Name: "shop.Status", Namespace: "shop", Values: []*parser.EnumValue{{Name: "open"}, {Name: "closed"}}},
		},
		Interfaces: []*parser.Interface{
			{
				Name:      "shop.Catalog",
				Namespace: "shop",
				Methods: []*parser.Method{
					{
						Name:       "place",
						Parameters: []*parser.Parameter{{Name: "order", Type: &parser.Type{UserDefined: "shop.Order"}}},
						ReturnType: &parser.Type{UserDefined: "book.Book"},
					},
					{
						Name:           "find",
						Parameters:     []*parser.Parameter{{Name: "id", Type: &parser.Type{UserDefined: "user.User"}}},
						ReturnType:     &parser.Type{BuiltIn: "int"},
						ReturnOptional: true,
					},
				},
			},
		},
	}
}

// Invariant C1/D12: generating the same IDL twice must produce byte-identical
// output, including import order and namespace registration order.
func TestJavaGenerationIsByteDeterministic(t *testing.T) {
	var snapshots []map[string]string
	for i := 0; i < 3; i++ {
		dir := t.TempDir()
		idl := determinismFixtureIDL()
		generateJavaFixture(t, idl, dir, true)
		snapshots = append(snapshots, snapshotJavaTree(t, dir))
	}

	first := snapshots[0]
	if len(first) == 0 {
		t.Fatal("generator produced no files")
	}
	for i := 1; i < len(snapshots); i++ {
		got := snapshots[i]
		if len(got) != len(first) {
			t.Fatalf("run %d produced %d files, want %d", i, len(got), len(first))
		}
		for rel, want := range first {
			have, ok := got[rel]
			if !ok {
				t.Fatalf("run %d is missing %s", i, rel)
			}
			if have != want {
				t.Fatalf("run %d differs for %s\n--- first ---\n%s\n--- got ---\n%s", i, rel, want, have)
			}
		}
	}
}

// optionalBuiltInsIDL is the C10 fixture: required and optional built-ins in a
// struct and in interface signatures.
func optionalBuiltInsIDL(t *testing.T) *parser.IDL {
	t.Helper()
	return parseTestIDL(t, `
namespace metrics

struct Sample {
  reqInt int
  reqBool bool
  reqFloat float
  reqString string
  optInt int [optional]
  optBool bool [optional]
  optFloat float [optional]
  optString string [optional]
}

interface Probe {
  requiredCount() int
  optionalCount() int [optional]
  optionalFlag() bool [optional]
  optionalRatio() float [optional]
}
`)
}

// Invariant C10/D7: optional built-in fields, parameters, and returns are boxed
// so absence is representable; required built-ins stay primitive. R26: float is
// a Java double.
func TestJavaOptionalBuiltInsAreBoxed(t *testing.T) {
	idl := optionalBuiltInsIDL(t)
	dir := t.TempDir()
	generateJavaFixture(t, idl, dir, false)

	base := filepath.Join(dir, "src", "main", "java", "com", "example", "rpc", "metrics")
	sample := readFileString(t, filepath.Join(base, "Sample.java"))
	for _, want := range []string{
		"private int reqInt;",
		"private boolean reqBool;",
		"private double reqFloat;",
		"private String reqString;",
		"private Integer optInt;",
		"private Boolean optBool;",
		"private Double optFloat;",
		"private String optString;",
		"public Integer getOptInt()",
		"public void setOptInt(Integer optInt)",
		"public Boolean getOptBool()",
		"public Double getOptFloat()",
	} {
		if !strings.Contains(sample, want) {
			t.Errorf("Sample.java missing %q:\n%s", want, sample)
		}
	}

	iface := readFileString(t, filepath.Join(base, "Probe.java"))
	for _, want := range []string{
		"public int requiredCount();",
		"public Integer optionalCount();",
		"public Boolean optionalFlag();",
		"public Double optionalRatio();",
	} {
		if !strings.Contains(iface, want) {
			t.Errorf("Probe.java missing %q:\n%s", want, iface)
		}
	}

	client := readFileString(t, filepath.Join(base, "ProbeClient.java"))
	if !strings.Contains(client, "public Integer optionalCount()") {
		t.Errorf("ProbeClient.java should box the optional return type:\n%s", client)
	}
}

// Invariant C10: the optional-built-in fixture must actually compile, not only
// contain boxed declarations. This runs javac over the whole generated project
// for both JSON libraries, with a minimal stub for the selected library's
// annotation and type token so no third-party jar is needed.
//
// When javac is not on PATH (for example the WSL Go test runner) the test is
// skipped: the Docker-based integration gate
// tests/integration/test_generator_java.sh runs `mvn test-compile` on the full
// generated project for examples/conform.pulse with Jackson and Gson and is the
// authoritative compile check.
func TestJavaOptionalBuiltInsFixtureCompiles(t *testing.T) {
	javac, err := exec.LookPath("javac")
	if err != nil {
		t.Skip("javac not on PATH; tests/integration/test_generator_java.sh (mvn test-compile) is the compile gate")
	}

	fixtures := []struct {
		jsonLib string
		stubs   map[string]string
	}{
		{
			jsonLib: "jackson",
			stubs: map[string]string{
				"com/fasterxml/jackson/annotation/JsonProperty.java": "package com.fasterxml.jackson.annotation;\n\npublic @interface JsonProperty {\n    String value();\n}\n",
				"com/fasterxml/jackson/core/type/TypeReference.java": "package com.fasterxml.jackson.core.type;\n\npublic abstract class TypeReference<T> {\n    public java.lang.reflect.Type getType() {\n        return null;\n    }\n}\n",
			},
		},
		{
			jsonLib: "gson",
			stubs: map[string]string{
				"com/google/gson/annotations/SerializedName.java": "package com.google.gson.annotations;\n\npublic @interface SerializedName {\n    String value();\n}\n",
				"com/google/gson/reflect/TypeToken.java":          "package com.google.gson.reflect;\n\npublic class TypeToken<T> {\n    public java.lang.reflect.Type getType() {\n        return null;\n    }\n}\n",
			},
		},
	}

	for _, fixture := range fixtures {
		t.Run(fixture.jsonLib, func(t *testing.T) {
			dir := t.TempDir()
			generateJavaFixtureWithLib(t, optionalBuiltInsIDL(t), dir, false, fixture.jsonLib)

			stubDir := filepath.Join(dir, "stubs")
			for rel, content := range fixture.stubs {
				stubPath := filepath.Join(stubDir, filepath.FromSlash(rel))
				if err := os.MkdirAll(filepath.Dir(stubPath), 0o755); err != nil {
					t.Fatalf("create stub dir: %v", err)
				}
				if err := os.WriteFile(stubPath, []byte(content), 0o644); err != nil {
					t.Fatalf("write stub %s: %v", rel, err)
				}
			}

			var sources []string
			srcRoot := filepath.Join(dir, "src", "main", "java")
			err := filepath.Walk(srcRoot, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}
				if info.IsDir() || !strings.HasSuffix(path, ".java") {
					return nil
				}
				sources = append(sources, path)
				return nil
			})
			if err != nil {
				t.Fatalf("walk generated sources: %v", err)
			}
			err = filepath.Walk(stubDir, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}
				if info.IsDir() || !strings.HasSuffix(path, ".java") {
					return nil
				}
				sources = append(sources, path)
				return nil
			})
			if err != nil {
				t.Fatalf("walk stubs: %v", err)
			}
			if len(sources) < 10 {
				t.Fatalf("expected the generated project plus stubs, got %d sources", len(sources))
			}

			classesDir := filepath.Join(dir, "classes")
			if err := os.MkdirAll(classesDir, 0o755); err != nil {
				t.Fatalf("create classes dir: %v", err)
			}
			args := append([]string{"-d", classesDir, "-encoding", "UTF-8", "-proc:none", "-nowarn"}, sources...)
			cmd := exec.Command(javac, args...)
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("javac failed for the %s optional-built-in fixture:\n%s", fixture.jsonLib, out)
			}
		})
	}
}

func readFileString(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

// Invariants C4/C5: the embedded IDL_JSON contains rootNamespace and checksum
// and is a JSON object (not a double-encoded string). The pulserpc-idl method
// serves that object.
func TestJavaEmbeddedIDLMetadataAndObjectShape(t *testing.T) {
	idl := parseTestIDL(t, "namespace test\nstruct S { a string }\ninterface Svc { ping() S }\n")
	dir := t.TempDir()
	generateJavaFixture(t, idl, dir, false)

	serverPath := filepath.Join(dir, "src", "main", "java", "com", "example", "rpc", "Server.java")
	server := readFileString(t, serverPath)

	re := regexp.MustCompile(`(?s)private static final String IDL_JSON = (".*?");`)
	match := re.FindStringSubmatch(server)
	if match == nil {
		t.Fatalf("Server.java has no IDL_JSON literal:\n%s", server)
	}
	var idlText string
	if err := json.Unmarshal([]byte(match[1]), &idlText); err != nil {
		t.Fatalf("IDL_JSON literal is not a Java string literal: %v", err)
	}
	var doc map[string]interface{}
	if err := json.Unmarshal([]byte(idlText), &doc); err != nil {
		t.Fatalf("IDL_JSON is not an object: %v\n%s", err, idlText)
	}
	if doc["rootNamespace"] != "test" {
		t.Errorf("IDL_JSON rootNamespace = %v, want test", doc["rootNamespace"])
	}
	if checksum, _ := doc["checksum"].(string); checksum == "" {
		t.Error("IDL_JSON is missing a checksum")
	}
	if _, ok := doc["interfaces"]; !ok {
		t.Error("IDL_JSON is missing interfaces")
	}
	if !strings.Contains(server, `jsonParser.fromJson(IDL_JSON, Object.class)`) ||
		!strings.Contains(server, `return resultEnvelope(idlDoc, id);`) {
		t.Error("pulserpc-idl does not serve the embedded IDL as a JSON object")
	}
}

// Invariants R16-R19, R22, R24, M18/M19: the generated server/client enforce
// resource limits, POST-only 405, id typing, and per-entry batch errors.
func TestGeneratedJavaAdapterInvariants(t *testing.T) {
	idl := parseTestIDL(t, "namespace test\ninterface Svc { ping() string }\n")
	server := generateServerJava(idl, nil, map[string]*NamespaceTypes{}, "com.example", "package com.example;", "{}")
	client := generateClientJava(idl, map[string]*NamespaceTypes{}, "com.example", "package com.example;", "{}")

	t.Run("R16_request_body_cap", func(t *testing.T) {
		for _, want := range []string{
			"HttpLimits.checkDeclaredLength(declaredLength, HttpLimits.MAX_REQUEST_BODY, \"Request body\");",
			"HttpLimits.readBounded(requestBodyStream, HttpLimits.MAX_REQUEST_BODY, \"Request body\");",
			"try (java.io.InputStream requestBodyStream = exchange.getRequestBody()) {",
			"sendError(exchange, 200, -32600, \"Request body too large\");",
		} {
			if !strings.Contains(server, want) {
				t.Errorf("generated server missing request-body cap: %q", want)
			}
		}
		if strings.Contains(server, "readAllBytes()") {
			t.Error("generated server still reads the request body unbounded")
		}
	})

	t.Run("R16_transport_errors_are_not_oversize", func(t *testing.T) {
		// A request-body read failure (client disconnect, corrupt chunked
		// encoding) is a transport error, not a size violation: it must reach
		// the outer handler (-32603), never the "too large" branch.
		if !strings.Contains(server, "catch (HttpLimits.BodyTooLargeException bodyTooLarge) {") {
			t.Error("generated server catches a generic IOException for the body cap")
		}
		if strings.Contains(server, "catch (IOException bodyTooLarge)") {
			t.Error("generated server still mislabels transport read errors as oversize")
		}
	})

	t.Run("R17_response_body_cap", func(t *testing.T) {
		for _, want := range []string{
			"HttpLimits.readResponseBody(response, HttpLimits.MAX_RESPONSE_BODY, \"Response body\")",
			"HttpResponse.BodyHandlers.ofInputStream()",
		} {
			if !strings.Contains(client, want) {
				t.Errorf("generated client missing response-body cap: %q", want)
			}
		}
		// The shared helper owns the status/length checks and closes the body
		// on every path; the generated client must not duplicate them outside a
		// try-with-resources.
		if strings.Contains(client, "response.statusCode() != 200") {
			t.Error("generated client checks the status outside the body-closing helper")
		}
	})

	t.Run("R18_batch_cap", func(t *testing.T) {
		for _, want := range []string{
			"batch.size() > HttpLimits.MAX_BATCH_SIZE",
			`errorEnvelope(-32600, "Batch exceeds maximum size", null, null)`,
		} {
			if !strings.Contains(server, want) {
				t.Errorf("generated server missing batch cap: %q", want)
			}
		}
	})

	t.Run("R19_client_timeout", func(t *testing.T) {
		if !strings.Contains(client, ".timeout(HttpLimits.REQUEST_TIMEOUT)") {
			t.Error("generated client has no request timeout")
		}
		if !strings.Contains(client, ".connectTimeout(HttpLimits.CONNECT_TIMEOUT)") {
			t.Error("generated client does not use the shared connect timeout")
		}
	})

	t.Run("D3_error_envelope_always_has_id", func(t *testing.T) {
		if !strings.Contains(server, "        response.put(\"id\", id);\n") {
			t.Error("generated error envelope must always carry id (null when unknown)")
		}
		if !strings.Contains(server, "InternalErrors.log(") {
			t.Error("generated server does not use the shared internal-error logger")
		}
		if strings.Contains(server, "private void logInternalError(") {
			t.Error("generated server still defines its own logInternalError helper")
		}
	})

	t.Run("R22_post_only_405", func(t *testing.T) {
		for _, want := range []string{
			`sendError(exchange, 405, -32600, "Invalid Request - only POST allowed");`,
			"private void sendError(HttpExchange exchange, int status, int code, String message)",
		} {
			if !strings.Contains(server, want) {
				t.Errorf("generated server missing POST-only 405 handling: %q", want)
			}
		}
	})

	t.Run("R24_id_type", func(t *testing.T) {
		if !strings.Contains(server, "id != null && !(id instanceof String) && !(id instanceof Number)") {
			t.Error("generated server does not reject non-string/non-number ids")
		}
	})

	t.Run("M18_M19_batch_errors", func(t *testing.T) {
		for _, want := range []string{
			"if (item instanceof Map) {",
			`responses.add(errorEnvelope(-32600, "Invalid Request", null, null));`,
			"Invalid Request - empty batch",
		} {
			if !strings.Contains(server, want) {
				t.Errorf("generated server missing batch envelope handling: %q", want)
			}
		}
	})

	t.Run("R30_trailing_slash_normalized", func(t *testing.T) {
		if !strings.Contains(client, "while (normalizedBaseUrl.endsWith(\"/\"))") {
			t.Error("generated client does not normalize a trailing slash on the base URL")
		}
	})
}

// Invariant R15/S6: generated adapters never put exception text or data on the
// wire.
func TestGeneratedJavaInternalErrorsAreRedacted(t *testing.T) {
	idl := parseTestIDL(t, "namespace test\ninterface Svc { ping() string }\n")
	server := generateServerJava(idl, nil, map[string]*NamespaceTypes{}, "com.example", "package com.example;", "{}")

	for _, leaked := range []string{
		`"Internal error: " +`,
		`"Failed to load IDL: " +`,
		`"Internal error: response validation failed: " +`,
	} {
		if strings.Contains(server, leaked) {
			t.Errorf("generated server still leaks internal error detail: %q", leaked)
		}
	}
	if strings.Count(server, `"Internal error"`) < 3 {
		t.Errorf("generated server should return the fixed Internal error message on every internal path:\n%s", server)
	}
	if !strings.Contains(server, `InternalErrors.log("request dispatch", e);`) {
		t.Error("generated server does not log request-dispatch errors server-side")
	}
}

// Invariant C11/R27: generated enums are plain Java enums serialized by name.
func TestGeneratedJavaEnumsAreNamed(t *testing.T) {
	idl := parseTestIDL(t, "namespace test\nenum Colour { red green }\n")
	dir := t.TempDir()
	generateJavaFixture(t, idl, dir, false)

	enumFile := readFileString(t, filepath.Join(dir, "src", "main", "java", "com", "example", "rpc", "test", "Colour.java"))
	if !strings.Contains(enumFile, "public enum Colour {") {
		t.Fatalf("expected a plain Java enum:\n%s", enumFile)
	}
	if strings.Contains(enumFile, "@JsonValue") || strings.Contains(enumFile, "ordinal") {
		t.Errorf("enum should serialize by name, not ordinal:\n%s", enumFile)
	}
	if !strings.Contains(enumFile, "    red,") || !strings.Contains(enumFile, "    green") {
		t.Errorf("enum constants are not the declared names:\n%s", enumFile)
	}
}
