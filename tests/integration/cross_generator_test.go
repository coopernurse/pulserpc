package integration

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/coopernurse/pulserpc/pkg/generator"
	"github.com/coopernurse/pulserpc/pkg/parser"
)

type generatorVariant struct {
	name string
	new  func() generator.Plugin
	// configure applies extra flags to a fresh FlagSet after RegisterFlags.
	configure func(*testing.T, *flag.FlagSet)
}

// setVariantFlag sets a generator flag, failing the test when the flag is not
// registered instead of panicking on a nil fs.Lookup result.
func setVariantFlag(t *testing.T, fs *flag.FlagSet, name, value string) {
	t.Helper()
	f := fs.Lookup(name)
	if f == nil {
		t.Fatalf("%s: flag %q is not registered", fs.Name(), name)
	}
	if err := f.Value.Set(value); err != nil {
		t.Fatalf("%s: set flag %q=%q: %v", fs.Name(), name, value, err)
	}
}

func crossGeneratorVariants() []generatorVariant {
	return []generatorVariant{
		{name: "go", new: func() generator.Plugin { return generator.NewGoClientServer() }, configure: func(t *testing.T, fs *flag.FlagSet) {
			setVariantFlag(t, fs, "go-module", "example.com/crosscheck")
			setVariantFlag(t, fs, "inline-runtime", "true")
		}},
		{name: "java", new: func() generator.Plugin { return generator.NewJavaClientServer() }, configure: func(t *testing.T, fs *flag.FlagSet) {
			setVariantFlag(t, fs, "package", "com.example.generated")
			setVariantFlag(t, fs, "json-lib", "jackson")
		}},
		{name: "python", new: func() generator.Plugin { return generator.NewPythonClientServer() }},
		{name: "python2", new: func() generator.Plugin { return generator.NewPythonClientServer() }, configure: func(t *testing.T, fs *flag.FlagSet) {
			setVariantFlag(t, fs, "python-version", "2")
		}},
		{name: "ts-node", new: func() generator.Plugin { return generator.NewTSClientServer() }, configure: func(t *testing.T, fs *flag.FlagSet) {
			setVariantFlag(t, fs, "ts-module", "esm-node")
		}},
		{name: "ts-cjs", new: func() generator.Plugin { return generator.NewTSClientServer() }, configure: func(t *testing.T, fs *flag.FlagSet) {
			setVariantFlag(t, fs, "ts-module", "cjs")
			setVariantFlag(t, fs, "ts-gen-package-json", "true")
			setVariantFlag(t, fs, "ts-gen-tsconfig", "true")
		}},
		{name: "csharp", new: func() generator.Plugin { return generator.NewCSharpClientServer() }},
	}
}

func loadCrossCheckerIDL(t *testing.T) *parser.IDL {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "examples", "conform.pulse"))
	if err != nil {
		t.Fatalf("read conform.pulse: %v", err)
	}
	filename := filepath.Join("..", "..", "examples", "conform.pulse")
	idl, err := parser.ParseIDL(filename, string(data))
	if err != nil {
		t.Fatalf("parse conform.pulse: %v", err)
	}
	return idl
}

func generateVariant(t *testing.T, variant generatorVariant, dir string) map[string][]byte {
	t.Helper()
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}

	plugin := variant.new()
	fs := flag.NewFlagSet(variant.name, flag.ContinueOnError)
	fs.String("dir", dir, "Output directory")
	fs.String("base-dir", "", "Base directory")
	plugin.RegisterFlags(fs)
	if variant.configure != nil {
		variant.configure(t, fs)
	}
	if err := fs.Parse(nil); err != nil {
		t.Fatalf("parse flags: %v", err)
	}
	if err := plugin.Generate(loadCrossCheckerIDL(t), fs); err != nil {
		t.Fatalf("generate: %v", err)
	}

	files := make(map[string][]byte)
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
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
	if len(files) == 0 {
		t.Fatalf("%s generated no files", variant.name)
	}
	return files
}

// C1/D12: generating the same IDL twice must be byte-identical.
func TestCrossGeneratorDeterminism(t *testing.T) {
	for _, variant := range crossGeneratorVariants() {
		variant := variant
		t.Run(variant.name, func(t *testing.T) {
			// Generate twice into the same path. Some generators derive a
			// package/module path from the output location; comparing different
			// paths would report that as nondeterminism even though the output
			// is stable for a fixed location.
			dir := filepath.Join(t.TempDir(), "generated")
			first := generateVariant(t, variant, dir)
			if err := os.RemoveAll(dir); err != nil {
				t.Fatalf("remove %s: %v", dir, err)
			}
			second := generateVariant(t, variant, dir)

			if len(first) != len(second) {
				t.Fatalf("file count differs: %d vs %d", len(first), len(second))
			}
			for name, want := range first {
				got, ok := second[name]
				if !ok {
					t.Errorf("second run is missing %s", name)
					continue
				}
				if string(got) != string(want) {
					t.Errorf("%s is not byte-deterministic", name)
				}
			}
		})
	}
}

// C2/C4: every emitted idl.json is a JSON object with the canonical schema and
// a checksum that matches the parser.
func TestCrossGeneratorIDLArtifacts(t *testing.T) {
	idl := loadCrossCheckerIDL(t)
	wantChecksum, err := parser.ComputeChecksum(idl)
	if err != nil {
		t.Fatalf("compute checksum: %v", err)
	}

	requiredKeys := []string{"rootNamespace", "checksum", "interfaces", "structs", "enums", "errors"}

	for _, variant := range crossGeneratorVariants() {
		variant := variant
		t.Run(variant.name, func(t *testing.T) {
			files := generateVariant(t, variant, t.TempDir())

			found := false
			for name, data := range files {
				if filepath.Base(name) != "idl.json" {
					continue
				}
				found = true

				var doc map[string]any
				if err := json.Unmarshal(data, &doc); err != nil {
					t.Errorf("%s is not a JSON object: %v", name, err)
					continue
				}
				for _, key := range requiredKeys {
					if _, ok := doc[key]; !ok {
						t.Errorf("%s is missing %q", name, key)
					}
				}
				if doc["rootNamespace"] != "conform" {
					t.Errorf("%s rootNamespace = %v, want conform", name, doc["rootNamespace"])
				}
				if doc["checksum"] != wantChecksum {
					t.Errorf("%s checksum = %v, want %s", name, doc["checksum"], wantChecksum)
				}
			}

			// Generators that embed IDL metadata instead of writing idl.json
			// are covered by the served pulserpc-idl probe in test_http_api.sh.
			if !found {
				t.Logf("%s does not emit idl.json (embedded metadata is checked over the wire)", variant.name)
			}
		})
	}
}

// embeddedIDLAssn finds assignments to the generated IDL metadata constants.
// Go emits `const IDL_JSON = "..."`, Java emits `String IDL_JSON = "..."`, and
// C# emits `string IDL_JSON = @"..."` (also `_idlJson`). Group 1 is the first
// character of the string literal.
var embeddedIDLAssn = regexp.MustCompile(`\b(?:IDL_JSON|_idlJson|idlJson)\s*=\s*(@?")`)

// decodeEmbeddedIDL decodes a language string literal starting at text[0].
// Go and Java literals are quoted (with compatible escape sequences); C#
// literals are verbatim strings (@"...") with doubled quotes.
func decodeEmbeddedIDL(text string) (string, error) {
	if strings.HasPrefix(text, `@"`) {
		var sb strings.Builder
		for i := 2; i < len(text); {
			if text[i] == '"' {
				if i+1 < len(text) && text[i+1] == '"' {
					sb.WriteByte('"')
					i += 2
					continue
				}
				return sb.String(), nil
			}
			sb.WriteByte(text[i])
			i++
		}
		return "", fmt.Errorf("unterminated verbatim string literal")
	}
	if text[0] != '"' {
		return "", fmt.Errorf("literal does not start with a quote")
	}
	i := 1
	for i < len(text) {
		if text[i] == '\\' {
			i += 2
			continue
		}
		if text[i] == '"' {
			break
		}
		i++
	}
	if i >= len(text) {
		return "", fmt.Errorf("unterminated string literal")
	}
	literal := text[:i+1]
	if decoded, err := strconv.Unquote(literal); err == nil {
		return decoded, nil
	}
	var decoded string
	if err := json.Unmarshal([]byte(literal), &decoded); err != nil {
		return "", err
	}
	return decoded, nil
}

// C5: embedded IDL metadata must be a JSON object (not a double-encoded
// string), and its checksum must agree with the parser. Variants that only
// write idl.json are already checked by TestCrossGeneratorIDLArtifacts.
func TestCrossGeneratorEmbeddedIDLIsObject(t *testing.T) {
	idl := loadCrossCheckerIDL(t)
	wantChecksum, err := parser.ComputeChecksum(idl)
	if err != nil {
		t.Fatalf("compute checksum: %v", err)
	}

	for _, variant := range crossGeneratorVariants() {
		variant := variant
		t.Run(variant.name, func(t *testing.T) {
			files := generateVariant(t, variant, t.TempDir())

			found := false
			for name, data := range files {
				text := string(data)
				for _, loc := range embeddedIDLAssn.FindAllStringSubmatchIndex(text, -1) {
					found = true
					idlText, err := decodeEmbeddedIDL(text[loc[2]:])
					if err != nil {
						t.Errorf("%s: decode embedded IDL literal: %v", name, err)
						continue
					}
					var doc map[string]any
					if err := json.Unmarshal([]byte(idlText), &doc); err != nil {
						t.Errorf("%s: embedded IDL is not a JSON object (possibly double-encoded): %v", name, err)
						continue
					}
					if got, ok := doc["checksum"]; ok {
						if got != wantChecksum {
							t.Errorf("%s: embedded IDL checksum = %v, want %s", name, got, wantChecksum)
						}
					}
				}
			}

			if !found {
				t.Logf("%s does not embed IDL metadata (idl.json is checked by TestCrossGeneratorIDLArtifacts)", variant.name)
			}
		})
	}
}
