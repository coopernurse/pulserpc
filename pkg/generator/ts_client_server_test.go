package generator

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/coopernurse/pulserpc/pkg/parser"
)

// withTempOutputDir creates a temporary directory for test output and cleans it up.
func withTempOutputDir(t *testing.T, fn func(dir string)) {
	t.Helper()
	dir := t.TempDir()
	fn(dir)
}

// assertTsFileExists checks that a file exists at the given relative path within dir.
func assertTsFileExists(t *testing.T, dir, relPath string) {
	t.Helper()
	fullPath := filepath.Join(dir, relPath)
	_, err := os.Stat(fullPath)
	if err != nil {
		t.Errorf("expected file %q to exist: %v", relPath, err)
	}
}

// assertTsFileContains checks that a file contains the given substring.
func assertTsFileContains(t *testing.T, dir, relPath, substr string) {
	t.Helper()
	fullPath := filepath.Join(dir, relPath)
	content, err := os.ReadFile(fullPath)
	if err != nil {
		t.Fatalf("failed to read file %q: %v", relPath, err)
	}
	if !strings.Contains(string(content), substr) {
		t.Errorf("expected file %q to contain %q", relPath, substr)
	}
}

// buildMultiNamespaceIDL creates an IDL with two namespaces: common and book.
// book includes a type reference to common (Book references common.Category).
func buildMultiNamespaceIDL() *parser.IDL {
	return &parser.IDL{
		Structs: []*parser.Struct{
			{
				Name:      "Category",
				Namespace: "common",
				Fields: []*parser.Field{
					{
						Name: "id",
						Type: &parser.Type{BuiltIn: "string"},
					},
					{
						Name: "name",
						Type: &parser.Type{BuiltIn: "string"},
					},
				},
			},
			{
				Name:      "Book",
				Namespace: "book",
				Fields: []*parser.Field{
					{
						Name: "title",
						Type: &parser.Type{BuiltIn: "string"},
					},
					{
						Name: "category",
						Type: &parser.Type{UserDefined: "Category"},
					},
				},
			},
		},
		Enums: []*parser.Enum{
			{
				Name:      "Status",
				Namespace: "common",
				Values: []*parser.EnumValue{
					{Name: "ACTIVE"},
					{Name: "INACTIVE"},
				},
			},
		},
		Interfaces: []*parser.Interface{
			{
				Name:      "CommonService",
				Namespace: "common",
				Methods: []*parser.Method{
					{
						Name: "getCategory",
						Parameters: []*parser.Parameter{
							{Name: "id", Type: &parser.Type{BuiltIn: "string"}},
						},
						ReturnType: &parser.Type{UserDefined: "Category"},
					},
				},
			},
			{
				Name:      "BookService",
				Namespace: "book",
				Methods: []*parser.Method{
					{
						Name: "getBook",
						Parameters: []*parser.Parameter{
							{Name: "id", Type: &parser.Type{BuiltIn: "string"}},
						},
						ReturnType: &parser.Type{UserDefined: "Book"},
					},
				},
			},
		},
	}
}

func TestTsMultiNamespace(t *testing.T) {
	withTempOutputDir(t, func(outputDir string) {
		idl := buildMultiNamespaceIDL()

		gen := NewTSClientServer()
		fs := flag.NewFlagSet("test", flag.ContinueOnError)
		fs.String("dir", "", "output dir")
		gen.RegisterFlags(fs)
		if err := fs.Set("dir", outputDir); err != nil {
			t.Fatalf("failed to set dir flag: %v", err)
		}

		err := gen.Generate(idl, fs)
		if err != nil {
			t.Fatalf("Generate() failed: %v", err)
		}

		// Assert book namespace files exist
		assertTsFileExists(t, outputDir, "book/types.ts")
		assertTsFileExists(t, outputDir, "book/server.ts")
		assertTsFileExists(t, outputDir, "book/client.ts")

		// Assert common namespace files exist
		assertTsFileExists(t, outputDir, "common/types.ts")
		assertTsFileExists(t, outputDir, "common/server.ts")
		assertTsFileExists(t, outputDir, "common/client.ts")

		// Assert runtime exists
		assertTsFileExists(t, outputDir, "pulserpc/index.ts")

		// Assert idl.json is at root
		assertTsFileExists(t, outputDir, "idl.json")
	})
}

func TestTsMultiNamespaceFileContent(t *testing.T) {
	withTempOutputDir(t, func(outputDir string) {
		idl := buildMultiNamespaceIDL()

		gen := NewTSClientServer()
		fs := flag.NewFlagSet("test", flag.ContinueOnError)
		fs.String("dir", "", "output dir")
		gen.RegisterFlags(fs)
		if err := fs.Set("dir", outputDir); err != nil {
			t.Fatalf("failed to set dir flag: %v", err)
		}

		err := gen.Generate(idl, fs)
		if err != nil {
			t.Fatalf("Generate() failed: %v", err)
		}

		// Verify book/types.ts contains the Book struct
		assertTsFileContains(t, outputDir, "book/types.ts", "export interface Book")

		// Verify common/types.ts contains the Category struct and Status enum
		assertTsFileContains(t, outputDir, "common/types.ts", "export interface Category")
		assertTsFileContains(t, outputDir, "common/types.ts", "export enum Status")

		// Verify book/server.ts contains the BookService abstract class
		assertTsFileContains(t, outputDir, "book/server.ts", "export abstract class BookService")

		// Verify common/server.ts contains the CommonService abstract class
		assertTsFileContains(t, outputDir, "common/server.ts", "export abstract class CommonService")

		// Verify book/client.ts uses correct runtime import path for namespace subdir
		assertTsFileContains(t, outputDir, "book/client.ts", "from '../pulserpc'")

		// Verify common/client.ts uses correct runtime import path for namespace subdir
		assertTsFileContains(t, outputDir, "common/client.ts", "from '../pulserpc'")

		// Verify book/server.ts uses correct runtime import path for namespace subdir
		assertTsFileContains(t, outputDir, "book/server.ts", "from '../pulserpc/rpc'")
	})
}

func TestTsBackwardsCompatibleSingleNamespace(t *testing.T) {
	withTempOutputDir(t, func(outputDir string) {
		// Single namespace IDL with no -dir flag should produce flat output
		idl := &parser.IDL{
			Structs: []*parser.Struct{
				{
					Name: "User",
					Fields: []*parser.Field{
						{Name: "name", Type: &parser.Type{BuiltIn: "string"}},
					},
				},
			},
			Interfaces: []*parser.Interface{
				{
					Name: "UserService",
					Methods: []*parser.Method{
						{
							Name:       "getUser",
							Parameters: []*parser.Parameter{{Name: "id", Type: &parser.Type{BuiltIn: "string"}}},
							ReturnType: &parser.Type{UserDefined: "User"},
						},
					},
				},
			},
		}

		gen := NewTSClientServer()
		fs := flag.NewFlagSet("test", flag.ContinueOnError)
		fs.String("dir", "", "output dir")
		gen.RegisterFlags(fs)
		// No -dir flag: should produce flat output in current directory behavior
		// We still need to set -dir to our temp dir for the test
		if err := fs.Set("dir", outputDir); err != nil {
			t.Fatalf("failed to set dir flag: %v", err)
		}

		err := gen.Generate(idl, fs)
		if err != nil {
			t.Fatalf("Generate() failed: %v", err)
		}

		// With single namespace and -dir set, multi-namespace mode should be active
		// (per the isMultiNamespaceMode logic: outputDir != "" && len(namespaceMap) == 1 with non-empty ns)
		// But the namespace here is empty string, so it should stay flat
		assertTsFileExists(t, outputDir, "types.ts")
		assertTsFileExists(t, outputDir, "server.ts")
		assertTsFileExists(t, outputDir, "client.ts")
		assertTsFileExists(t, outputDir, "pulserpc/index.ts")
	})
}

func TestTsBackwardsCompatibleSingleNamespaceFlat(t *testing.T) {
	withTempOutputDir(t, func(outputDir string) {
		// Single namespace with non-empty namespace name and -dir should trigger multi-namespace mode
		idl := &parser.IDL{
			Structs: []*parser.Struct{
				{
					Name:      "user.User",
					Namespace: "user",
					Fields: []*parser.Field{
						{Name: "name", Type: &parser.Type{BuiltIn: "string"}},
					},
				},
			},
			Interfaces: []*parser.Interface{
				{
					Name:      "user.UserService",
					Namespace: "user",
					Methods: []*parser.Method{
						{
							Name:       "getUser",
							Parameters: []*parser.Parameter{{Name: "id", Type: &parser.Type{BuiltIn: "string"}}},
							ReturnType: &parser.Type{UserDefined: "user.User"},
						},
					},
				},
			},
		}

		gen := NewTSClientServer()
		fs := flag.NewFlagSet("test", flag.ContinueOnError)
		fs.String("dir", "", "output dir")
		gen.RegisterFlags(fs)
		if err := fs.Set("dir", outputDir); err != nil {
			t.Fatalf("failed to set dir flag: %v", err)
		}

		err := gen.Generate(idl, fs)
		if err != nil {
			t.Fatalf("Generate() failed: %v", err)
		}

		// Single namespace with -dir and non-empty namespace should produce per-namespace subdir
		assertTsFileExists(t, outputDir, "user/types.ts")
		assertTsFileExists(t, outputDir, "user/server.ts")
		assertTsFileExists(t, outputDir, "user/client.ts")
		assertTsFileExists(t, outputDir, "pulserpc/index.ts")
	})
}
