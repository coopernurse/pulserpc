package integration

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/coopernurse/pulserpc/pkg/generator"
	"github.com/coopernurse/pulserpc/pkg/parser"
)

// Cross-generator checks (C1/C2/C4/C5) are intentionally opt-in until the
// per-runtime remediation plans land. Set PULSERPC_CROSS_GENERATOR=1 to run
// them during the final merge. They are not part of `make test`.
func requireCrossGenerator(t *testing.T) {
	t.Helper()
	if os.Getenv("PULSERPC_CROSS_GENERATOR") == "" {
		t.Skip("set PULSERPC_CROSS_GENERATOR=1 to run cross-generator checks")
	}
}

type generatorVariant struct {
	name string
	new  func() generator.Plugin
	// configure applies extra flags to a fresh FlagSet.
	configure func(*flag.FlagSet)
}

func crossGeneratorVariants() []generatorVariant {
	return []generatorVariant{
		{name: "go", new: func() generator.Plugin { return generator.NewGoClientServer() }, configure: func(fs *flag.FlagSet) {
			fs.Lookup("go-module").Value.Set("example.com/crosscheck")
			fs.Lookup("inline-runtime").Value.Set("true")
		}},
		{name: "java", new: func() generator.Plugin { return generator.NewJavaClientServer() }, configure: func(fs *flag.FlagSet) {
			fs.Lookup("package").Value.Set("com.example.generated")
			fs.Lookup("json-lib").Value.Set("jackson")
		}},
		{name: "python", new: func() generator.Plugin { return generator.NewPythonClientServer() }},
		{name: "python2", new: func() generator.Plugin { return generator.NewPythonClientServer() }, configure: func(fs *flag.FlagSet) {
			fs.Lookup("python-version").Value.Set("2")
		}},
		{name: "ts-node", new: func() generator.Plugin { return generator.NewTSClientServer() }, configure: func(fs *flag.FlagSet) {
			fs.Lookup("ts-module").Value.Set("esm-node")
		}},
		{name: "ts-cjs", new: func() generator.Plugin { return generator.NewTSClientServer() }, configure: func(fs *flag.FlagSet) {
			fs.Lookup("ts-module").Value.Set("cjs")
			fs.Lookup("ts-gen-package-json").Value.Set("true")
			fs.Lookup("ts-gen-tsconfig").Value.Set("true")
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
		variant.configure(fs)
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
	requireCrossGenerator(t)
	for _, variant := range crossGeneratorVariants() {
		variant := variant
		t.Run(variant.name, func(t *testing.T) {
			// Use the same basename under different parents: some generators
			// derive a package/module name from the output directory, which is
			// not nondeterminism for a fixed output location.
			first := generateVariant(t, variant, filepath.Join(t.TempDir(), "generated"))
			second := generateVariant(t, variant, filepath.Join(t.TempDir(), "generated"))

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
	requireCrossGenerator(t)
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
