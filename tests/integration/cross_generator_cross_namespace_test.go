package integration

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/coopernurse/pulserpc/pkg/parser"
)

// The committed two-namespace fixture must be generatable by every runtime
// variant, and each generator must carry the imported interface under its
// qualified IDL name. Runs under `make test-cross-generator` (CI variants job)
// because the test name matches the -run 'TestCrossGenerator' filter.
func TestCrossGeneratorCrossNamespaceFixture(t *testing.T) {
	path := filepath.Join("..", "..", "examples", "cross-ns", "app.pulse")
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read cross-ns fixture: %v", err)
	}
	idl, err := parser.ParseIDL(path, string(src))
	if err != nil {
		t.Fatalf("ParseIDL cross-ns fixture: %v", err)
	}

	for _, variant := range crossGeneratorVariants() {
		variant := variant
		t.Run(variant.name, func(t *testing.T) {
			dir := t.TempDir()
			plugin := variant.new()
			fs := flag.NewFlagSet(variant.name, flag.ContinueOnError)
			fs.String("dir", dir, "")
			fs.String("base-dir", "", "")
			plugin.RegisterFlags(fs)
			if variant.configure != nil {
				variant.configure(t, fs)
			}
			if err := plugin.Generate(idl, fs); err != nil {
				t.Fatalf("generate: %v", err)
			}

			files := 0
			qualified := false
			err := filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
				if err != nil || d.IsDir() {
					return err
				}
				files++
				data, err := os.ReadFile(p)
				if err != nil {
					return err
				}
				if strings.Contains(string(data), "common.CommonOps") {
					qualified = true
				}
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			if files == 0 {
				t.Fatal("generator produced no files")
			}
			if !qualified {
				t.Error("no generated file carries the qualified imported interface name common.CommonOps")
			}
		})
	}
}
