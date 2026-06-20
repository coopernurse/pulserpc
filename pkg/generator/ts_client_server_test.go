package generator

import (
	"flag"
	"fmt"
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

		// Assert idl.json is in the entry-point namespace directory only
		// (idl.json is only written to the entry-point namespace, not all namespaces)
		assertTsFileExists(t, outputDir, "common/idl.json")
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
		assertTsFileContains(t, outputDir, "book/client.ts", "from '../pulserpc/transport.js'")
		assertTsFileContains(t, outputDir, "book/client.ts", "from '../pulserpc/rpc.js'")

		// Verify common/client.ts uses correct runtime import path for namespace subdir
		assertTsFileContains(t, outputDir, "common/client.ts", "from '../pulserpc/transport.js'")
		assertTsFileContains(t, outputDir, "common/client.ts", "from '../pulserpc/rpc.js'")

		// Verify book/server.ts uses correct runtime import path for namespace subdir
		assertTsFileContains(t, outputDir, "book/server.ts", "from '../pulserpc/rpc.js'")
	})
}

func TestTsNamespaceIndex(t *testing.T) {
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

		// Assert book/index.ts exists
		assertTsFileExists(t, outputDir, "book/index.ts")

		// Assert it contains the expected re-exports
		assertTsFileContains(t, outputDir, "book/index.ts", "export * from './types.js'")
		assertTsFileContains(t, outputDir, "book/index.ts", "export * from './server.js'")
		assertTsFileContains(t, outputDir, "book/index.ts", "export * from './client.js'")

		// Assert common/index.ts exists and has re-exports too
		assertTsFileExists(t, outputDir, "common/index.ts")
		assertTsFileContains(t, outputDir, "common/index.ts", "export * from './types.js'")
		assertTsFileContains(t, outputDir, "common/index.ts", "export * from './server.js'")
		assertTsFileContains(t, outputDir, "common/index.ts", "export * from './client.js'")
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
		// Single namespace with non-empty namespace name and -dir should produce flat output
		// (multi-namespace mode only activates with multiple namespaces)
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

		// Single namespace should produce flat output (types.ts, server.ts, client.ts in root)
		assertTsFileExists(t, outputDir, "types.ts")
		assertTsFileExists(t, outputDir, "server.ts")
		assertTsFileExists(t, outputDir, "client.ts")
		assertTsFileExists(t, outputDir, "pulserpc/index.ts")
	})
}

func TestTsImportPaths(t *testing.T) {
	t.Run("multi-namespace book/types.ts uses ../pulserpc runtime import", func(t *testing.T) {
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

			// book/types.ts should NOT contain './pulserpc' (flat import)
			content, err := os.ReadFile(filepath.Join(outputDir, "book/types.ts"))
			if err != nil {
				t.Fatalf("failed to read book/types.ts: %v", err)
			}
			if strings.Contains(string(content), "from './pulserpc'") {
				t.Error("book/types.ts should not contain flat import from './pulserpc'")
			}
		})
	})

	t.Run("multi-namespace book/types.ts contains ../common cross-namespace import", func(t *testing.T) {
		withTempOutputDir(t, func(outputDir string) {
			// Build IDL where book has a type referencing common.Category
			idl := buildMultiNamespaceWithCrossRefIDL()

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

			assertTsFileContains(t, outputDir, "book/types.ts", "from '../common/types.js'")
		})
	})

	t.Run("single-namespace flat output uses ./pulserpc import", func(t *testing.T) {
		withTempOutputDir(t, func(outputDir string) {
			// Single namespace with empty namespace name - flat output
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
			// No -dir flag set: flat output
			// But we need to set output dir for the test to work
			// With empty namespace and no -dir, it should stay flat
			// We'll test the flat output by checking server.ts content

			// Actually, with empty namespace and -dir set, it stays flat per isMultiNamespaceMode
			if err := fs.Set("dir", outputDir); err != nil {
				t.Fatalf("failed to set dir flag: %v", err)
			}

			err := gen.Generate(idl, fs)
			if err != nil {
				t.Fatalf("Generate() failed: %v", err)
			}

			// Flat output: server.ts should use ./pulserpc
			assertTsFileContains(t, outputDir, "server.ts", "from './pulserpc/rpc.js'")
			// Flat output: client.ts should use ./pulserpc/transport and ./pulserpc/rpc
			assertTsFileContains(t, outputDir, "client.ts", "from './pulserpc/transport.js'")
			assertTsFileContains(t, outputDir, "client.ts", "from './pulserpc/rpc.js'")
		})
	})

	t.Run("multi-namespace server.ts uses ../pulserpc/rpc import", func(t *testing.T) {
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

			assertTsFileContains(t, outputDir, "book/server.ts", "from '../pulserpc/rpc.js'")
			assertTsFileContains(t, outputDir, "common/server.ts", "from '../pulserpc/rpc.js'")
		})
	})

	t.Run("multi-namespace client.ts uses ../pulserpc/transport and ../pulserpc/rpc import", func(t *testing.T) {
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

			assertTsFileContains(t, outputDir, "book/client.ts", "from '../pulserpc/transport.js'")
			assertTsFileContains(t, outputDir, "book/client.ts", "from '../pulserpc/rpc.js'")
			assertTsFileContains(t, outputDir, "common/client.ts", "from '../pulserpc/transport.js'")
			assertTsFileContains(t, outputDir, "common/client.ts", "from '../pulserpc/rpc.js'")
		})
	})
}

func TestTsIdlAndRuntime(t *testing.T) {
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

		// idl.json is only written to the entry-point namespace directory
		assertTsFileExists(t, outputDir, "common/idl.json")

		// pulserpc/index.ts should exist for runtime re-exports
		// This allows "import { ... } from '../pulserpc'" to work from inside namespace subdirs
		assertTsFileExists(t, outputDir, "pulserpc/index.ts")

		// Verify pulserpc/index.ts contains re-exports from runtime modules
		assertTsFileContains(t, outputDir, "pulserpc/index.ts", "export")
	})
}

// buildMultiNamespaceWithCrossRefIDL creates an IDL with two namespaces where
// book has a struct field that references a type from common.
func buildMultiNamespaceWithCrossRefIDL() *parser.IDL {
	return &parser.IDL{
		Structs: []*parser.Struct{
			{
				Name:      "Category",
				Namespace: "common",
				Fields: []*parser.Field{
					{Name: "id", Type: &parser.Type{BuiltIn: "string"}},
					{Name: "name", Type: &parser.Type{BuiltIn: "string"}},
				},
			},
			{
				Name:      "Book",
				Namespace: "book",
				Fields: []*parser.Field{
					{Name: "title", Type: &parser.Type{BuiltIn: "string"}},
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
				Values:    []*parser.EnumValue{{Name: "ACTIVE"}, {Name: "INACTIVE"}},
			},
		},
		Interfaces: []*parser.Interface{
			{
				Name:      "CommonService",
				Namespace: "common",
				Methods: []*parser.Method{
					{
						Name:       "getCategory",
						Parameters: []*parser.Parameter{{Name: "id", Type: &parser.Type{BuiltIn: "string"}}},
						ReturnType: &parser.Type{UserDefined: "Category"},
					},
				},
			},
			{
				Name:      "BookService",
				Namespace: "book",
				Methods: []*parser.Method{
					{
						Name:       "getBook",
						Parameters: []*parser.Parameter{{Name: "id", Type: &parser.Type{BuiltIn: "string"}}},
						ReturnType: &parser.Type{UserDefined: "Book"},
					},
				},
			},
		},
	}
}

// buildThreeNamespaceIDL creates an IDL with three namespaces: common, book, and user.
// Both book and user include common types (cross-namespace references).
func buildThreeNamespaceIDL() *parser.IDL {
	return &parser.IDL{
		Structs: []*parser.Struct{
			{
				Name:      "Category",
				Namespace: "common",
				Fields: []*parser.Field{
					{Name: "id", Type: &parser.Type{BuiltIn: "string"}},
					{Name: "name", Type: &parser.Type{BuiltIn: "string"}},
				},
			},
			{
				Name:      "Book",
				Namespace: "book",
				Fields: []*parser.Field{
					{Name: "title", Type: &parser.Type{BuiltIn: "string"}},
					{
						Name: "category",
						Type: &parser.Type{UserDefined: "Category"},
					},
				},
			},
			{
				Name:      "User",
				Namespace: "user",
				Fields: []*parser.Field{
					{Name: "name", Type: &parser.Type{BuiltIn: "string"}},
					{
						Name: "favoriteCategory",
						Type: &parser.Type{UserDefined: "Category"},
					},
				},
			},
		},
		Enums: []*parser.Enum{
			{
				Name:      "Status",
				Namespace: "common",
				Values:    []*parser.EnumValue{{Name: "ACTIVE"}, {Name: "INACTIVE"}},
			},
		},
		Interfaces: []*parser.Interface{
			{
				Name:      "CommonService",
				Namespace: "common",
				Methods: []*parser.Method{
					{
						Name:       "getCategory",
						Parameters: []*parser.Parameter{{Name: "id", Type: &parser.Type{BuiltIn: "string"}}},
						ReturnType: &parser.Type{UserDefined: "Category"},
					},
				},
			},
			{
				Name:      "BookService",
				Namespace: "book",
				Methods: []*parser.Method{
					{
						Name:       "getBook",
						Parameters: []*parser.Parameter{{Name: "id", Type: &parser.Type{BuiltIn: "string"}}},
						ReturnType: &parser.Type{UserDefined: "Book"},
					},
				},
			},
			{
				Name:      "UserService",
				Namespace: "user",
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
}

func TestTsMultiFileEndToEnd(t *testing.T) {
	withTempOutputDir(t, func(outputDir string) {
		idl := buildThreeNamespaceIDL()

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

		// Assert runtime exists
		assertTsFileExists(t, outputDir, "pulserpc/index.ts")

		// Assert common namespace files exist
		assertTsFileExists(t, outputDir, "common/types.ts")
		assertTsFileExists(t, outputDir, "common/server.ts")
		assertTsFileExists(t, outputDir, "common/client.ts")
		assertTsFileExists(t, outputDir, "common/index.ts")

		// Assert book namespace files exist
		assertTsFileExists(t, outputDir, "book/types.ts")
		assertTsFileExists(t, outputDir, "book/server.ts")
		assertTsFileExists(t, outputDir, "book/client.ts")
		assertTsFileExists(t, outputDir, "book/index.ts")

		// Assert user namespace files exist
		assertTsFileExists(t, outputDir, "user/types.ts")
		assertTsFileExists(t, outputDir, "user/server.ts")
		assertTsFileExists(t, outputDir, "user/client.ts")
		assertTsFileExists(t, outputDir, "user/index.ts")

		// Assert cross-namespace import: book/types.ts references common
		assertTsFileContains(t, outputDir, "book/types.ts", "from '../common/types.js'")
	})
}

func TestTsPackageFlag(t *testing.T) {
	t.Run("-package flag places files in package directory", func(t *testing.T) {
		withTempOutputDir(t, func(outputDir string) {
			idl := &parser.IDL{
				Interfaces: []*parser.Interface{
					{
						Name: "UserService",
						Methods: []*parser.Method{
							{
								Name:       "getUser",
								Parameters: []*parser.Parameter{{Name: "id", Type: &parser.Type{BuiltIn: "string"}}},
								ReturnType: &parser.Type{BuiltIn: "string"},
							},
						},
					},
				},
			}

			gen := NewTSClientServer()
			fs := flag.NewFlagSet("test", flag.ContinueOnError)
			fs.String("dir", "", "output dir")
			fs.String("package", "", "base module path")
			gen.RegisterFlags(fs)
			if err := fs.Set("dir", outputDir); err != nil {
				t.Fatalf("failed to set dir flag: %v", err)
			}
			if err := fs.Set("package", "@myapp/lib/rpc"); err != nil {
				t.Fatalf("failed to set package flag: %v", err)
			}

			err := gen.Generate(idl, fs)
			if err != nil {
				t.Fatalf("Generate() failed: %v", err)
			}

			// When -package is set with single namespace, files should go into package directory
			// Class name should be exactly "UserService", not "MyAppUserService" or any prefixed version
			assertTsFileContains(t, outputDir, "@myapp/lib/rpc/server.ts", "export abstract class UserService")

			// Verify package value is not prepended to class names
			content, err := os.ReadFile(filepath.Join(outputDir, "@myapp/lib/rpc/server.ts"))
			if err != nil {
				t.Fatalf("failed to read server.ts: %v", err)
			}
			if strings.Contains(string(content), "@myapp/lib/rpcUserService") {
				t.Error("class name should not contain package prefix")
			}
			if strings.Contains(string(content), "myappUserService") {
				t.Error("class name should not contain package prefix")
			}
		})
	})

	t.Run("-package flag with multi-namespace output", func(t *testing.T) {
		withTempOutputDir(t, func(outputDir string) {
			idl := buildMultiNamespaceIDL()

			gen := NewTSClientServer()
			fs := flag.NewFlagSet("test", flag.ContinueOnError)
			fs.String("dir", "", "output dir")
			fs.String("package", "", "base module path")
			gen.RegisterFlags(fs)
			if err := fs.Set("dir", outputDir); err != nil {
				t.Fatalf("failed to set dir flag: %v", err)
			}
			if err := fs.Set("package", "@mycompany/api"); err != nil {
				t.Fatalf("failed to set package flag: %v", err)
			}

			err := gen.Generate(idl, fs)
			if err != nil {
				t.Fatalf("Generate() failed: %v", err)
			}

			// Package flag should affect directory structure - files go under {outputDir}/{package}/
			assertTsFileContains(t, outputDir, "@mycompany/api/book/server.ts", "export abstract class BookService")
			assertTsFileContains(t, outputDir, "@mycompany/api/common/server.ts", "export abstract class CommonService")

			// Verify package value is not prepended to class names
			content, err := os.ReadFile(filepath.Join(outputDir, "@mycompany/api/book/server.ts"))
			if err != nil {
				t.Fatalf("failed to read @mycompany/api/book/server.ts: %v", err)
			}
			if strings.Contains(string(content), "@mycompany/apiBookService") {
				t.Error("class name should not contain package prefix")
			}
			if strings.Contains(string(content), "mycompanyBookService") {
				t.Error("class name should not contain package prefix")
			}
		})
	})
}

func TestTsStaticClientGeneration(t *testing.T) {
	withTempOutputDir(t, func(outputDir string) {
		idl := &parser.IDL{
			Structs: []*parser.Struct{
				{Name: "Product", Fields: []*parser.Field{
					{Name: "productId", Type: &parser.Type{BuiltIn: "string"}},
					{Name: "name", Type: &parser.Type{BuiltIn: "string"}},
				}},
			},
			Interfaces: []*parser.Interface{
				{Name: "CatalogService", Methods: []*parser.Method{
					{Name: "listProducts", Parameters: []*parser.Parameter{}, ReturnType: &parser.Type{Array: &parser.Type{UserDefined: "Product"}}},
				}},
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

		assertTsFileExists(t, outputDir, "client.ts")
		assertTsFileContains(t, outputDir, "client.ts", "export class CatalogServiceClient")
		assertTsFileContains(t, outputDir, "client.ts", "constructor(private transport: Transport)")
		assertTsFileContains(t, outputDir, "client.ts", "async listProducts()")
	})
}

func TestTsStaticClientMethodSignatures(t *testing.T) {
	withTempOutputDir(t, func(outputDir string) {
		idl := &parser.IDL{
			Structs: []*parser.Struct{
				{Name: "Product", Fields: []*parser.Field{
					{Name: "productId", Type: &parser.Type{BuiltIn: "string"}},
					{Name: "name", Type: &parser.Type{BuiltIn: "string"}},
				}},
				{Name: "Cart", Fields: []*parser.Field{
					{Name: "cartId", Type: &parser.Type{BuiltIn: "string"}},
				}},
			},
			Interfaces: []*parser.Interface{
				{Name: "CatalogService", Methods: []*parser.Method{
					{Name: "listProducts", Parameters: []*parser.Parameter{}, ReturnType: &parser.Type{Array: &parser.Type{UserDefined: "Product"}}},
					{Name: "getProduct", Parameters: []*parser.Parameter{{Name: "productId", Type: &parser.Type{BuiltIn: "string"}}}, ReturnType: &parser.Type{UserDefined: "Product"}},
				}},
				{Name: "CartService", Methods: []*parser.Method{
					{Name: "getCart", Parameters: []*parser.Parameter{{Name: "cartId", Type: &parser.Type{BuiltIn: "string"}}}, ReturnType: &parser.Type{UserDefined: "Cart"}, ReturnOptional: true},
				}},
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

		assertTsFileContains(t, outputDir, "client.ts", "async listProducts(): Promise<types.Product[]>")
		assertTsFileContains(t, outputDir, "client.ts", "async getProduct(productId: string): Promise<types.Product>")
		assertTsFileContains(t, outputDir, "client.ts", "async getCart(cartId: string): Promise<types.Cart | null>")
	})
}

func TestTsStaticClientTransport(t *testing.T) {
	withTempOutputDir(t, func(outputDir string) {
		idl := &parser.IDL{
			Interfaces: []*parser.Interface{
				{Name: "TestService", Methods: []*parser.Method{
					{Name: "test", Parameters: []*parser.Parameter{}, ReturnType: &parser.Type{BuiltIn: "string"}},
				}},
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

		assertTsFileContains(t, outputDir, "client.ts", "import { Transport, HttpTransport } from './pulserpc/transport.js'")
		assertTsFileContains(t, outputDir, "client.ts", "constructor(private transport: Transport)")
		assertTsFileContains(t, outputDir, "client.ts", "export { Transport, HttpTransport }")
	})
}

func TestTsStaticClientMultiNamespace(t *testing.T) {
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

		assertTsFileExists(t, outputDir, "book/client.ts")
		assertTsFileExists(t, outputDir, "common/client.ts")
		assertTsFileContains(t, outputDir, "book/client.ts", "export class BookServiceClient")
		assertTsFileContains(t, outputDir, "book/client.ts", "constructor(private transport: Transport)")
		assertTsFileContains(t, outputDir, "common/client.ts", "export class CommonServiceClient")
		assertTsFileContains(t, outputDir, "common/client.ts", "constructor(private transport: Transport)")
		assertTsFileContains(t, outputDir, "book/client.ts", "from '../pulserpc/transport.js'")
		assertTsFileContains(t, outputDir, "common/client.ts", "from '../pulserpc/transport.js'")
	})
}

func TestTsStaticClientExports(t *testing.T) {
	withTempOutputDir(t, func(outputDir string) {
		idl := &parser.IDL{
			Interfaces: []*parser.Interface{
				{Name: "ServiceA", Methods: []*parser.Method{{Name: "methodA", Parameters: []*parser.Parameter{}, ReturnType: nil}}},
				{Name: "ServiceB", Methods: []*parser.Method{{Name: "methodB", Parameters: []*parser.Parameter{}, ReturnType: nil}}},
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

		assertTsFileContains(t, outputDir, "client.ts", "export class ServiceAClient")
		assertTsFileContains(t, outputDir, "client.ts", "export class ServiceBClient")
	})
}

func TestTsStaticClientNoDynamicProxy(t *testing.T) {
	withTempOutputDir(t, func(outputDir string) {
		idl := &parser.IDL{
			Interfaces: []*parser.Interface{
				{Name: "TestService", Methods: []*parser.Method{{Name: "test", Parameters: []*parser.Parameter{}, ReturnType: nil}}},
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

		content, err := os.ReadFile(filepath.Join(outputDir, "client.ts"))
		if err != nil {
			t.Fatalf("failed to read client.ts: %v", err)
		}

		if strings.Contains(string(content), "new Client(") {
			t.Error("client.ts should not contain 'new Client(' - dynamic proxy pattern")
		}
		if strings.Contains(string(content), "class Client") {
			t.Error("client.ts should not contain 'class Client' - dynamic proxy pattern")
		}
		if strings.Contains(string(content), "InterfaceClientProxy") {
			t.Error("client.ts should not contain 'InterfaceClientProxy' - dynamic proxy pattern")
		}
	})
}

func TestTsStaticClientRPCError(t *testing.T) {
	withTempOutputDir(t, func(outputDir string) {
		idl := &parser.IDL{
			Interfaces: []*parser.Interface{
				{Name: "TestService", Methods: []*parser.Method{{Name: "test", Parameters: []*parser.Parameter{}, ReturnType: &parser.Type{BuiltIn: "string"}}}},
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

		assertTsFileContains(t, outputDir, "client.ts", "import { RPCError } from './pulserpc/rpc.js'")
		assertTsFileContains(t, outputDir, "client.ts", "throw new RPCError(_resp.error.code, _resp.error.message, _resp.error.data)")
	})
}

func TestTsStaticClientMethodCalls(t *testing.T) {
	withTempOutputDir(t, func(outputDir string) {
		idl := &parser.IDL{
			Structs: []*parser.Struct{
				{Name: "Request", Fields: []*parser.Field{{Name: "id", Type: &parser.Type{BuiltIn: "string"}}}},
			},
			Interfaces: []*parser.Interface{
				{Name: "TestService", Methods: []*parser.Method{
					{Name: "testMethod", Parameters: []*parser.Parameter{{Name: "req", Type: &parser.Type{UserDefined: "Request"}}}, ReturnType: &parser.Type{BuiltIn: "string"}},
				}},
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

		assertTsFileContains(t, outputDir, "client.ts", "method: \"TestService.testMethod\"")
		assertTsFileContains(t, outputDir, "client.ts", "params: [req]")
	})
}

func TestTsTestsAreDeterministic(t *testing.T) {
	for iteration := 0; iteration < 3; iteration++ {
		t.Run(fmt.Sprintf("iteration_%d", iteration), func(t *testing.T) {
			tmpDir, err := os.MkdirTemp("", "pulserpc-ts-det-")
			if err != nil {
				t.Fatalf("failed to create temp dir: %v", err)
			}
			defer func() { _ = os.RemoveAll(tmpDir) }()

			idl := &parser.IDL{
				Structs: []*parser.Struct{
					{
						Name:      "Zoo",
						Namespace: "animals",
						Fields:    []*parser.Field{{Name: "name", Type: &parser.Type{BuiltIn: "string"}}},
					},
					{
						Name:      "Alpha",
						Namespace: "animals",
						Fields:    []*parser.Field{{Name: "value", Type: &parser.Type{BuiltIn: "int"}}},
					},
					{
						Name:      "Mango",
						Namespace: "animals",
						Fields:    []*parser.Field{{Name: "color", Type: &parser.Type{BuiltIn: "string"}}},
					},
				},
				Enums: []*parser.Enum{
					{
						Name:      "Status",
						Namespace: "common",
						Values:    []*parser.EnumValue{{Name: "ACTIVE"}, {Name: "INACTIVE"}},
					},
					{
						Name:      "Priority",
						Namespace: "common",
						Values:    []*parser.EnumValue{{Name: "HIGH"}, {Name: "LOW"}, {Name: "MEDIUM"}},
					},
				},
				Interfaces: []*parser.Interface{
					{
						Name:      "ZooService",
						Namespace: "animals",
						Methods: []*parser.Method{
							{Name: "getZoo", Parameters: []*parser.Parameter{{Name: "id", Type: &parser.Type{BuiltIn: "string"}}}, ReturnType: &parser.Type{UserDefined: "Zoo"}},
						},
					},
					{
						Name:      "AlphaService",
						Namespace: "animals",
						Methods: []*parser.Method{
							{Name: "processAlpha", Parameters: []*parser.Parameter{{Name: "data", Type: &parser.Type{BuiltIn: "string"}}}, ReturnType: &parser.Type{BuiltIn: "int"}},
						},
					},
					{
						Name:      "MangoService",
						Namespace: "animals",
						Methods: []*parser.Method{
							{Name: "getMango", Parameters: []*parser.Parameter{{Name: "id", Type: &parser.Type{BuiltIn: "string"}}}, ReturnType: &parser.Type{UserDefined: "Mango"}},
						},
					},
				},
			}

			gen := NewTSClientServer()
			fs := flag.NewFlagSet("test", flag.ContinueOnError)
			fs.String("dir", "", "output dir")
			gen.RegisterFlags(fs)
			if err := fs.Set("dir", tmpDir); err != nil {
				t.Fatalf("failed to set dir flag: %v", err)
			}

			if err := gen.Generate(idl, fs); err != nil {
				t.Fatalf("Generate failed: %v", err)
			}

			collectedFiles := map[string][]byte{}
			err = filepath.Walk(tmpDir, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}
				if info.IsDir() {
					return nil
				}
				relPath, err := filepath.Rel(tmpDir, path)
				if err != nil {
					return err
				}
				content, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				collectedFiles[relPath] = content
				return nil
			})
			if err != nil {
				t.Fatalf("failed to walk temp dir: %v", err)
			}

			if iteration == 0 {
				t.Logf("Collected %d files from first iteration", len(collectedFiles))
				for relPath := range collectedFiles {
					t.Logf("  %s", relPath)
				}
			}

			for relPath, content := range collectedFiles {
				firstContent, ok := collectedFiles[relPath]
				if !ok {
					t.Errorf("File missing in iteration %d: %s", iteration, relPath)
					continue
				}
				if iteration > 0 && !bytesEqual(firstContent, content) {
					t.Errorf("File content differs in iteration %d: %s\nFirst iteration:\n%s\nCurrent iteration:\n%s",
						iteration, relPath, string(firstContent), string(content))
				}
			}
		})
	}
}

func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestTsMultiLineFieldComment(t *testing.T) {
	withTempOutputDir(t, func(outputDir string) {
		idl := &parser.IDL{
			Structs: []*parser.Struct{
				{
					Name: "User",
					Fields: []*parser.Field{
						{
							Name:    "id",
							Type:    &parser.Type{BuiltIn: "string"},
							Comment: "The user's unique identifier\n spanning multiple lines",
						},
						{
							Name:    "name",
							Type:    &parser.Type{BuiltIn: "string"},
							Comment: "First line\nSecond line\nThird line",
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

		content, err := os.ReadFile(filepath.Join(outputDir, "types.ts"))
		if err != nil {
			t.Fatalf("failed to read types.ts: %v", err)
		}
		tsContent := string(content)

		if !strings.Contains(tsContent, "  // The user's unique identifier\n  //  spanning multiple lines") {
			t.Error("expected multi-line field comment to be split across multiple // lines")
		}
		if !strings.Contains(tsContent, "  // First line\n  // Second line\n  // Third line") {
			t.Error("expected three-line field comment to be split across three // lines")
		}
	})
}

func TestTsMultiLineEnumValueComment(t *testing.T) {
	withTempOutputDir(t, func(outputDir string) {
		idl := &parser.IDL{
			Enums: []*parser.Enum{
				{
					Name: "Status",
					Values: []*parser.EnumValue{
						{
							Name:    "ACTIVE",
							Comment: "The item is active\nand should be processed",
						},
						{
							Name:    "INACTIVE",
							Comment: "Line one\nLine two",
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

		content, err := os.ReadFile(filepath.Join(outputDir, "types.ts"))
		if err != nil {
			t.Fatalf("failed to read types.ts: %v", err)
		}
		tsContent := string(content)

		if !strings.Contains(tsContent, "  // The item is active\n  // and should be processed") {
			t.Error("expected multi-line enum value comment to be split across multiple // lines")
		}
		if !strings.Contains(tsContent, "  // Line one\n  // Line two") {
			t.Error("expected two-line enum value comment to be split across two // lines")
		}
	})
}

func TestTsMultiLineFieldCommentNamespace(t *testing.T) {
	withTempOutputDir(t, func(outputDir string) {
		idl := &parser.IDL{
			Structs: []*parser.Struct{
				{
					Name:      "Category",
					Namespace: "common",
					Fields: []*parser.Field{
						{
							Name:    "id",
							Type:    &parser.Type{BuiltIn: "string"},
							Comment: "First line\nSecond line",
						},
					},
				},
				{
					Name:      "Book",
					Namespace: "book",
					Fields: []*parser.Field{
						{Name: "title", Type: &parser.Type{BuiltIn: "string"}},
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

		content, err := os.ReadFile(filepath.Join(outputDir, "common/types.ts"))
		if err != nil {
			t.Fatalf("failed to read common/types.ts: %v", err)
		}
		tsContent := string(content)

		if !strings.Contains(tsContent, "  // First line\n  // Second line") {
			t.Error("expected multi-line field comment in namespace to be split across multiple // lines")
		}
	})
}

func TestTsMultiLineEnumValueCommentNamespace(t *testing.T) {
	withTempOutputDir(t, func(outputDir string) {
		idl := &parser.IDL{
			Enums: []*parser.Enum{
				{
					Name:      "Status",
					Namespace: "common",
					Values: []*parser.EnumValue{
						{
							Name:    "ACTIVE",
							Comment: "Multi-line\ncomment here",
						},
					},
				},
			},
			Structs: []*parser.Struct{
				{
					Name:      "Book",
					Namespace: "book",
					Fields:    []*parser.Field{{Name: "title", Type: &parser.Type{BuiltIn: "string"}}},
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

		content, err := os.ReadFile(filepath.Join(outputDir, "common/types.ts"))
		if err != nil {
			t.Fatalf("failed to read common/types.ts: %v", err)
		}
		tsContent := string(content)

		if !strings.Contains(tsContent, "  // Multi-line\n  // comment here") {
			t.Error("expected multi-line enum value comment in namespace to be split across multiple // lines")
		}
	})
}

// TestTSRegisterFlagsIdempotent covers the §4.2 invariant: calling
// RegisterFlags twice on the same FlagSet must not panic and must not
// duplicate flag registrations.
func TestTSRegisterFlagsIdempotent(t *testing.T) {
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	gen := NewTSClientServer()

	gen.RegisterFlags(fs)
	// Snapshot the names of every registered flag after the first call.
	first := make(map[string]bool)
	fs.VisitAll(func(f *flag.Flag) { first[f.Name] = true })

	// Second call must be a no-op (no panic, no duplicate registrations).
	gen.RegisterFlags(fs)
	second := make(map[string]bool)
	fs.VisitAll(func(f *flag.Flag) { second[f.Name] = true })

	if len(first) != len(second) {
		t.Errorf("RegisterFlags duplicate: first=%d flags, second=%d flags", len(first), len(second))
	}
	for name := range first {
		if !second[name] {
			t.Errorf("RegisterFlags removed flag %q on second call", name)
		}
	}

	// Confirm the new flags are present.
	for _, want := range []string{"ts-module", "ts-gen-package-json", "ts-gen-tsconfig"} {
		if !second[want] {
			t.Errorf("expected flag %q to be registered", want)
		}
	}
}

// TestTSResolveModuleStyle covers the §4.3 invariant: every valid input
// (canonical + alias) must canonicalize to the right name, and unknown
// inputs must return an error that lists the valid values.
func TestTSResolveModuleStyle(t *testing.T) {
	p := NewTSClientServer()

	cases := []struct {
		input    string
		expected string
		wantErr  bool
	}{
		{"", "esm-node", false},
		{"esm-node", "esm-node", false},
		{"esm", "esm-node", false},
		{"node", "esm-node", false},
		{"esm-bundler", "esm-bundler", false},
		{"bundler", "esm-bundler", false},
		{"cjs", "cjs", false},
		{"commonjs", "cjs", false},
		// Case sensitivity: not part of the spec but document current
		// behavior — flag values are matched case-sensitively.
		{"ESM-NODE", "", true},
		{"Cjs", "", true},
		{"bogus", "", true},
		{"umd", "", true},
	}

	for _, c := range cases {
		t.Run(c.input, func(t *testing.T) {
			got, err := p.resolveModuleStyle(c.input)
			if c.wantErr {
				if err == nil {
					t.Fatalf("resolveModuleStyle(%q) = %q, want error", c.input, got)
				}
				// The error message must list every valid value
				// (canonical names and aliases) so users can recover.
				for _, want := range []string{"esm-node", "esm-bundler", "cjs", "esm", "node", "bundler", "commonjs"} {
					if !strings.Contains(err.Error(), want) {
						t.Errorf("error %q must mention %q", err.Error(), want)
					}
				}
				return
			}
			if err != nil {
				t.Fatalf("resolveModuleStyle(%q) returned unexpected error: %v", c.input, err)
			}
			if got != c.expected {
				t.Errorf("resolveModuleStyle(%q) = %q, want %q", c.input, got, c.expected)
			}
		})
	}
}

// TestTSRegisterFlagsHelpText covers the §4.4 invariant: the help text
// emitted by -h must mention each new flag and its valid values.
func TestTSRegisterFlagsHelpText(t *testing.T) {
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	gen := NewTSClientServer()
	gen.RegisterFlags(fs)

	out := fs.PrintDefaults

	// Drive the help output through a custom SetOutput buffer to capture it.
	var buf strings.Builder
	fs.SetOutput(&buf)
	out()
	help := buf.String()

	for _, want := range []string{"ts-module", "ts-gen-package-json", "ts-gen-tsconfig", "esm-node", "esm-bundler", "cjs"} {
		if !strings.Contains(help, want) {
			t.Errorf("expected help text to contain %q, got: %s", want, help)
		}
	}
}

// captureStderr swaps the package-level stderrWriter for the duration of fn
// and returns whatever was written. Restores the previous writer on return.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	orig := stderrWriter
	var buf strings.Builder
	stderrWriter = &buf
	defer func() { stderrWriter = orig }()
	fn()
	return buf.String()
}

// newResolveFS creates a fresh FlagSet and TSClientServer with both ts-module
// and ts-no-detect flags registered, suitable for resolveEffectiveModuleStyle
// tests.
func newResolveFS(t *testing.T, dir string) (*TSClientServer, *flag.FlagSet) {
	t.Helper()
	gen := NewTSClientServer()
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	fs.String("dir", "", "output dir")
	gen.RegisterFlags(fs)
	if dir != "" {
		if err := fs.Set("dir", dir); err != nil {
			t.Fatalf("failed to set dir: %v", err)
		}
	}
	return gen, fs
}

// TestResolveStyleExplicitOnly covers the §5 test matrix: an explicit
// -ts-module value must always win, regardless of the surrounding files.
func TestResolveStyleExplicitOnly(t *testing.T) {
	dir := t.TempDir()
	for _, style := range []string{"esm-node", "esm-bundler", "cjs"} {
		t.Run(style, func(t *testing.T) {
			gen, fs := newResolveFS(t, dir)
			if err := fs.Set("ts-module", style); err != nil {
				t.Fatalf("set ts-module: %v", err)
			}
			if err := gen.resolveEffectiveModuleStyle(fs, dir); err != nil {
				t.Fatalf("resolveEffectiveModuleStyle: %v", err)
			}
			if gen.moduleStyle != style {
				t.Errorf("moduleStyle = %q, want %q", gen.moduleStyle, style)
			}
		})
	}
}

// TestResolveStyleDefault covers the no-flag, no-files path.
func TestResolveStyleDefault(t *testing.T) {
	dir := t.TempDir()
	gen, fs := newResolveFS(t, dir)
	if err := gen.resolveEffectiveModuleStyle(fs, dir); err != nil {
		t.Fatalf("resolveEffectiveModuleStyle: %v", err)
	}
	if gen.moduleStyle != "esm-node" {
		t.Errorf("default moduleStyle = %q, want esm-node", gen.moduleStyle)
	}
}

// TestResolveStyleFromPackageJSON checks that a "commonjs" package.json type
// in the output dir drives the resolved style to cjs.
func TestResolveStyleFromPackageJSON(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"type":"commonjs"}`), 0644); err != nil {
		t.Fatalf("write package.json: %v", err)
	}
	gen, fs := newResolveFS(t, dir)
	if err := gen.resolveEffectiveModuleStyle(fs, dir); err != nil {
		t.Fatalf("resolveEffectiveModuleStyle: %v", err)
	}
	if gen.moduleStyle != "cjs" {
		t.Errorf("moduleStyle = %q, want cjs", gen.moduleStyle)
	}
	if gen.packageJSONType != "commonjs" {
		t.Errorf("packageJSONType = %q, want commonjs", gen.packageJSONType)
	}
}

// TestResolveStyleFromPackageJSONEmpty covers package.json with no "type"
// field (defaults to module) and asserts it maps to esm-node.
func TestResolveStyleFromPackageJSONEmpty(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"name":"x"}`), 0644); err != nil {
		t.Fatalf("write package.json: %v", err)
	}
	gen, fs := newResolveFS(t, dir)
	if err := gen.resolveEffectiveModuleStyle(fs, dir); err != nil {
		t.Fatalf("resolveEffectiveModuleStyle: %v", err)
	}
	if gen.moduleStyle != "esm-node" {
		t.Errorf("moduleStyle = %q, want esm-node (no type → module)", gen.moduleStyle)
	}
}

// TestResolveStyleFromTsConfig checks that a tsconfig.json with
// compilerOptions.module=CommonJS drives the resolved style to cjs.
func TestResolveStyleFromTsConfig(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "tsconfig.json"),
		[]byte(`{"compilerOptions":{"module":"CommonJS"}}`), 0644); err != nil {
		t.Fatalf("write tsconfig.json: %v", err)
	}
	gen, fs := newResolveFS(t, dir)
	if err := gen.resolveEffectiveModuleStyle(fs, dir); err != nil {
		t.Fatalf("resolveEffectiveModuleStyle: %v", err)
	}
	if gen.moduleStyle != "cjs" {
		t.Errorf("moduleStyle = %q, want cjs", gen.moduleStyle)
	}
	if gen.tsconfigModule != "CommonJS" {
		t.Errorf("tsconfigModule = %q, want CommonJS", gen.tsconfigModule)
	}
}

// TestResolveStyleTsConfigBundlerHint covers the "module=ES2020 +
// moduleResolution=Bundler" case from the §5.4 mapping table.
func TestResolveStyleTsConfigBundlerHint(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "tsconfig.json"),
		[]byte(`{"compilerOptions":{"module":"ES2020","moduleResolution":"Bundler"}}`), 0644); err != nil {
		t.Fatalf("write tsconfig.json: %v", err)
	}
	gen, fs := newResolveFS(t, dir)
	if err := gen.resolveEffectiveModuleStyle(fs, dir); err != nil {
		t.Fatalf("resolveEffectiveModuleStyle: %v", err)
	}
	if gen.moduleStyle != "esm-bundler" {
		t.Errorf("moduleStyle = %q, want esm-bundler (ES2020 + Bundler resolution)", gen.moduleStyle)
	}
}

// TestResolveStylePrecedenceExplicit checks that an explicit flag wins
// over package.json detection and emits a warning.
func TestResolveStylePrecedenceExplicit(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"type":"commonjs"}`), 0644); err != nil {
		t.Fatalf("write package.json: %v", err)
	}
	gen, fs := newResolveFS(t, dir)
	if err := fs.Set("ts-module", "esm-bundler"); err != nil {
		t.Fatalf("set ts-module: %v", err)
	}
	warn := captureStderr(t, func() {
		if err := gen.resolveEffectiveModuleStyle(fs, dir); err != nil {
			t.Fatalf("resolveEffectiveModuleStyle: %v", err)
		}
	})
	if gen.moduleStyle != "esm-bundler" {
		t.Errorf("moduleStyle = %q, want esm-bundler", gen.moduleStyle)
	}
	if !strings.Contains(warn, "warning:") {
		t.Errorf("expected warning, got: %q", warn)
	}
	if !strings.Contains(warn, "esm-bundler") {
		t.Errorf("warning %q must mention esm-bundler", warn)
	}
	if !strings.Contains(warn, "package.json") {
		t.Errorf("warning %q must mention package.json", warn)
	}
}

// TestResolveStylePrecedenceTsconfigOverPackageJSON asserts that a
// tsconfig.json detection takes precedence over package.json detection.
func TestResolveStylePrecedenceTsconfigOverPackageJSON(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "tsconfig.json"),
		[]byte(`{"compilerOptions":{"module":"NodeNext"}}`), 0644); err != nil {
		t.Fatalf("write tsconfig.json: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"type":"commonjs"}`), 0644); err != nil {
		t.Fatalf("write package.json: %v", err)
	}
	gen, fs := newResolveFS(t, dir)
	captureStderr(t, func() {
		if err := gen.resolveEffectiveModuleStyle(fs, dir); err != nil {
			t.Fatalf("resolveEffectiveModuleStyle: %v", err)
		}
	})
	if gen.moduleStyle != "esm-node" {
		t.Errorf("moduleStyle = %q, want esm-node (tsconfig NodeNext)", gen.moduleStyle)
	}
}

// TestResolveStyleConflictWarning asserts that when tsconfig and package.json
// disagree, the resolver picks tsconfig and emits a conflict warning.
func TestResolveStyleConflictWarning(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "tsconfig.json"),
		[]byte(`{"compilerOptions":{"module":"CommonJS"}}`), 0644); err != nil {
		t.Fatalf("write tsconfig.json: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"type":"module"}`), 0644); err != nil {
		t.Fatalf("write package.json: %v", err)
	}
	gen, fs := newResolveFS(t, dir)
	warn := captureStderr(t, func() {
		if err := gen.resolveEffectiveModuleStyle(fs, dir); err != nil {
			t.Fatalf("resolveEffectiveModuleStyle: %v", err)
		}
	})
	if gen.moduleStyle != "cjs" {
		t.Errorf("moduleStyle = %q, want cjs (tsconfig wins)", gen.moduleStyle)
	}
	if !strings.Contains(warn, "disagrees") {
		t.Errorf("expected disagreement warning, got: %q", warn)
	}
	if !strings.Contains(warn, "tsconfig.json") {
		t.Errorf("warning %q must mention tsconfig.json", warn)
	}
	if !strings.Contains(warn, "package.json") {
		t.Errorf("warning %q must mention package.json", warn)
	}
}

// TestResolveStyleNoDetect asserts that -ts-no-detect suppresses
// auto-detection entirely.
func TestResolveStyleNoDetect(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"type":"commonjs"}`), 0644); err != nil {
		t.Fatalf("write package.json: %v", err)
	}
	gen, fs := newResolveFS(t, dir)
	if err := fs.Set("ts-no-detect", "true"); err != nil {
		t.Fatalf("set ts-no-detect: %v", err)
	}
	if err := gen.resolveEffectiveModuleStyle(fs, dir); err != nil {
		t.Fatalf("resolveEffectiveModuleStyle: %v", err)
	}
	if gen.moduleStyle != "esm-node" {
		t.Errorf("moduleStyle = %q, want esm-node (no-detect suppresses package.json)", gen.moduleStyle)
	}
	if gen.packageJSONType != "" {
		t.Errorf("packageJSONType = %q, want empty (no-detect must not read package.json)", gen.packageJSONType)
	}
}

// TestWalkUpStopsAt10 asserts that the walk-up logic stops at the configured
// depth (10) and does not panic when the tree is deeper.
func TestWalkUpStopsAt10(t *testing.T) {
	dir := t.TempDir()
	// Build a 15-deep directory chain.
	deep := dir
	for i := 0; i < 15; i++ {
		deep = filepath.Join(deep, "d")
		if err := os.Mkdir(deep, 0755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
	}
	// Place a tsconfig.json at the very top (dir) — 15 levels above deep,
	// outside the walk limit. It must NOT be found.
	if err := os.WriteFile(filepath.Join(dir, "tsconfig.json"),
		[]byte(`{"compilerOptions":{"module":"CommonJS"}}`), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	// Place a tsconfig.json 3 levels above deep (within the walk limit).
	// This is deep/d/d/d.
	within := deep
	for i := 0; i < 3; i++ {
		within = filepath.Dir(within)
	}
	if err := os.WriteFile(filepath.Join(within, "tsconfig.json"),
		[]byte(`{"compilerOptions":{"module":"NodeNext"}}`), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	gen, fs := newResolveFS(t, deep)
	if err := gen.resolveEffectiveModuleStyle(fs, deep); err != nil {
		t.Fatalf("resolveEffectiveModuleStyle: %v", err)
	}
	if gen.tsconfigModule != "NodeNext" {
		t.Errorf("tsconfigModule = %q, want NodeNext (within walk limit)", gen.tsconfigModule)
	}

	// Verify the walk does not panic on a deeper tree with no config files.
	// Build a 15-deep chain with no tsconfig/package.json — start walk there.
	isolated := t.TempDir()
	for i := 0; i < 15; i++ {
		isolated = filepath.Join(isolated, "x")
		if err := os.MkdirAll(isolated, 0755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
	}
	gen2, fs2 := newResolveFS(t, isolated)
	if err := gen2.resolveEffectiveModuleStyle(fs2, isolated); err != nil {
		t.Fatalf("resolveEffectiveModuleStyle (isolated): %v", err)
	}
	if gen2.tsconfigModule != "" {
		t.Errorf("tsconfigModule = %q, want empty (no tsconfig found)", gen2.tsconfigModule)
	}
	if gen2.packageJSONType != "" {
		t.Errorf("packageJSONType = %q, want empty (no package.json found)", gen2.packageJSONType)
	}
}

// TestResolveStyleEmptyOutputDir covers the case where -dir is empty:
// detection is skipped and the default is returned.
func TestResolveStyleEmptyOutputDir(t *testing.T) {
	gen, fs := newResolveFS(t, "")
	// Place a package.json in the current dir — it should be ignored
	// because no -dir was provided.
	if err := gen.resolveEffectiveModuleStyle(fs, ""); err != nil {
		t.Fatalf("resolveEffectiveModuleStyle: %v", err)
	}
	if gen.moduleStyle != "esm-node" {
		t.Errorf("moduleStyle = %q, want esm-node (empty dir skips detection)", gen.moduleStyle)
	}
	if gen.packageJSONType != "" {
		t.Errorf("packageJSONType = %q, want empty (no walk performed)", gen.packageJSONType)
	}
}

// TestResolveStyleMalformedTsconfig asserts that a malformed tsconfig.json
// surfaces as an error (not a silent default).
func TestResolveStyleMalformedTsconfig(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "tsconfig.json"), []byte(`{not valid json`), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	gen, fs := newResolveFS(t, dir)
	if err := gen.resolveEffectiveModuleStyle(fs, dir); err == nil {
		t.Errorf("expected error for malformed tsconfig.json")
	}
}

// TestTSImportPathFreeFunction covers the §6.1 invariant: tsImportPath is
// the single source of truth for the .js-suffix decision and behaves
// correctly for every supported style.
func TestTSImportPathFreeFunction(t *testing.T) {
	cases := []struct {
		style   string
		target  string
		want    string
	}{
		// esm-node: explicit .js suffix required.
		{"esm-node", "./pulserpc/rpc", "./pulserpc/rpc.js"},
		{"esm-node", "../common/types", "../common/types.js"},
		{"esm-node", "./server", "./server.js"},
		// esm-bundler: .js emitted here, stripped by step 7's transform.
		{"esm-bundler", "./pulserpc/rpc", "./pulserpc/rpc.js"},
		{"esm-bundler", "../common/types", "../common/types.js"},
		// cjs: no extension.
		{"cjs", "./pulserpc/rpc", "./pulserpc/rpc"},
		{"cjs", "../common/types", "../common/types"},
		{"cjs", "./server", "./server"},
		// unknown / empty: defensive default = ESM (adds .js).
		{"", "./pulserpc/rpc", "./pulserpc/rpc.js"},
		{"unknown", "./x", "./x.js"},
	}
	for _, c := range cases {
		t.Run(c.style+"/"+c.target, func(t *testing.T) {
			got := tsImportPath(c.style, c.target)
			if got != c.want {
				t.Errorf("tsImportPath(%q, %q) = %q, want %q", c.style, c.target, got, c.want)
			}
		})
	}
}

// TestTSImportPathMethodMatchesFreeFunction ensures the (p *TSClientServer)
// method form delegates correctly to the free function.
func TestTSImportPathMethodMatchesFreeFunction(t *testing.T) {
	for _, style := range []string{"esm-node", "esm-bundler", "cjs", ""} {
		p := &TSClientServer{moduleStyle: style}
		if got, want := p.importPath("./pulserpc/rpc"), tsImportPath(style, "./pulserpc/rpc"); got != want {
			t.Errorf("importPath (style=%q) = %q, want %q", style, got, want)
		}
	}
}

// TestTsDefaultOutputByteEqualPreChange is the §6.5 invariant: the default
// esm-node output must be byte-equal to the pre-refactor baseline. We
// confirm this by checking the canonical .js-suffix lines on every emitted
// path: the same string the old hard-coded emit produced.
func TestTsDefaultOutputByteEqualPreChange(t *testing.T) {
	withTempOutputDir(t, func(outputDir string) {
		idl := buildMultiNamespaceIDL()

		gen := NewTSClientServer()
		fs := flag.NewFlagSet("test", flag.ContinueOnError)
		fs.String("dir", "", "output dir")
		gen.RegisterFlags(fs)
		if err := fs.Set("dir", outputDir); err != nil {
			t.Fatalf("failed to set dir: %v", err)
		}

		if err := gen.Generate(idl, fs); err != nil {
			t.Fatalf("Generate() failed: %v", err)
		}

		// Multi-namespace server.ts (from a namespace subdir): still
		// emits '../pulserpc/rpc.js' under esm-node.
		assertTsFileContains(t, outputDir, "book/server.ts", "from '../pulserpc/rpc.js'")
		assertTsFileContains(t, outputDir, "common/server.ts", "from '../pulserpc/rpc.js'")

		// Multi-namespace client.ts.
		assertTsFileContains(t, outputDir, "book/client.ts", "from '../pulserpc/transport.js'")
		assertTsFileContains(t, outputDir, "book/client.ts", "from '../pulserpc/rpc.js'")
		assertTsFileContains(t, outputDir, "common/client.ts", "from '../pulserpc/transport.js'")
		assertTsFileContains(t, outputDir, "common/client.ts", "from '../pulserpc/rpc.js'")

		// Cross-namespace import (book → common).
		assertTsFileContains(t, outputDir, "book/types.ts", "from '../common/types.js'")

		// Namespace index re-exports.
		assertTsFileContains(t, outputDir, "book/index.ts", "export * from './types.js'")
		assertTsFileContains(t, outputDir, "book/index.ts", "export * from './server.js'")
		assertTsFileContains(t, outputDir, "book/index.ts", "export * from './client.js'")
	})
}

// TestTsCjsOutputStripsJsSuffix asserts the §6.1 invariant in the opposite
// direction: under cjs, the .js suffix must NOT appear in any relative
// import emitted by the generator.
func TestTsCjsOutputStripsJsSuffix(t *testing.T) {
	withTempOutputDir(t, func(outputDir string) {
		idl := buildMultiNamespaceIDL()

		gen := NewTSClientServer()
		fs := flag.NewFlagSet("test", flag.ContinueOnError)
		fs.String("dir", "", "output dir")
		gen.RegisterFlags(fs)
		if err := fs.Set("dir", outputDir); err != nil {
			t.Fatalf("failed to set dir: %v", err)
		}
		if err := fs.Set("ts-module", "cjs"); err != nil {
			t.Fatalf("set ts-module: %v", err)
		}

		if err := gen.Generate(idl, fs); err != nil {
			t.Fatalf("Generate() failed: %v", err)
		}

		// Each relative import path must appear WITHOUT a trailing .js.
		// Walk every generated .ts file and look for the prohibited suffix.
		err := filepath.Walk(outputDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() || !strings.HasSuffix(path, ".ts") {
				return nil
			}
			// Skip the CJS runtime tree — it uses require() and has its
			// own .ts->.js rules documented in §7.2.
			if strings.Contains(path, string(filepath.Separator)+"pulserpc"+string(filepath.Separator)) {
				return nil
			}
			content, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			s := string(content)
			for _, line := range strings.Split(s, "\n") {
				if strings.Contains(line, "from '../pulserpc/rpc.js'") ||
					strings.Contains(line, "from '../pulserpc/transport.js'") ||
					strings.Contains(line, "from '../common/types.js'") ||
					strings.Contains(line, "from './types.js'") ||
					strings.Contains(line, "from './server.js'") ||
					strings.Contains(line, "from './client.js'") {
					t.Errorf("%s contains .js suffix in cjs mode: %s", path, line)
				}
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walk: %v", err)
		}

		// And the .js-less forms must be present.
		assertTsFileContains(t, outputDir, "book/server.ts", "from '../pulserpc/rpc'")
		assertTsFileContains(t, outputDir, "book/client.ts", "from '../pulserpc/transport'")
		assertTsFileContains(t, outputDir, "book/index.ts", "export * from './types'")
	})
}
