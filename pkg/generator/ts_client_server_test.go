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
		assertTsFileContains(t, outputDir, "book/client.ts", "from '../pulserpc/transport'")
		assertTsFileContains(t, outputDir, "book/client.ts", "from '../pulserpc/rpc'")

		// Verify common/client.ts uses correct runtime import path for namespace subdir
		assertTsFileContains(t, outputDir, "common/client.ts", "from '../pulserpc/transport'")
		assertTsFileContains(t, outputDir, "common/client.ts", "from '../pulserpc/rpc'")

		// Verify book/server.ts uses correct runtime import path for namespace subdir
		assertTsFileContains(t, outputDir, "book/server.ts", "from '../pulserpc/rpc'")
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
		assertTsFileContains(t, outputDir, "book/index.ts", "export * from './types'")
		assertTsFileContains(t, outputDir, "book/index.ts", "export * from './server'")
		assertTsFileContains(t, outputDir, "book/index.ts", "export * from './client'")

		// Assert common/index.ts exists and has re-exports too
		assertTsFileExists(t, outputDir, "common/index.ts")
		assertTsFileContains(t, outputDir, "common/index.ts", "export * from './types'")
		assertTsFileContains(t, outputDir, "common/index.ts", "export * from './server'")
		assertTsFileContains(t, outputDir, "common/index.ts", "export * from './client'")
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

			assertTsFileContains(t, outputDir, "book/types.ts", "from '../common'")
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
			assertTsFileContains(t, outputDir, "server.ts", "from './pulserpc/rpc'")
			// Flat output: client.ts should use ./pulserpc/transport and ./pulserpc/rpc
			assertTsFileContains(t, outputDir, "client.ts", "from './pulserpc/transport'")
			assertTsFileContains(t, outputDir, "client.ts", "from './pulserpc/rpc'")
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

			assertTsFileContains(t, outputDir, "book/server.ts", "from '../pulserpc/rpc'")
			assertTsFileContains(t, outputDir, "common/server.ts", "from '../pulserpc/rpc'")
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

			assertTsFileContains(t, outputDir, "book/client.ts", "from '../pulserpc/transport'")
			assertTsFileContains(t, outputDir, "book/client.ts", "from '../pulserpc/rpc'")
			assertTsFileContains(t, outputDir, "common/client.ts", "from '../pulserpc/transport'")
			assertTsFileContains(t, outputDir, "common/client.ts", "from '../pulserpc/rpc'")
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

		// idl.json should be at root (lib/rpc/idl.json), NOT inside any namespace subdir
		assertTsFileExists(t, outputDir, "idl.json")

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
		assertTsFileContains(t, outputDir, "book/types.ts", "from '../common'")

		// Assert cross-namespace import: user/types.ts references common
		assertTsFileContains(t, outputDir, "user/types.ts", "from '../common'")

		// Note: book/types.ts does NOT need a runtime import (../pulserpc) because
		// it only contains struct/enum types, not RPC types. Runtime imports are
		// only needed in server.ts and client.ts which use RPCError, Server, etc.
	})
}

func TestTsPackageFlag(t *testing.T) {
	t.Run("-package flag does not affect class names", func(t *testing.T) {
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

			// Class name should be exactly "UserService", not "MyAppUserService" or any prefixed version
			assertTsFileContains(t, outputDir, "server.ts", "export abstract class UserService")

			// Verify package value is not prepended to class names
			content, err := os.ReadFile(filepath.Join(outputDir, "server.ts"))
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

		assertTsFileContains(t, outputDir, "client.ts", "import { Transport, HttpTransport } from './pulserpc/transport'")
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
		assertTsFileContains(t, outputDir, "book/client.ts", "from '../pulserpc/transport'")
		assertTsFileContains(t, outputDir, "common/client.ts", "from '../pulserpc/transport'")
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

		assertTsFileContains(t, outputDir, "client.ts", "import { RPCError } from './pulserpc/rpc'")
		assertTsFileContains(t, outputDir, "client.ts", "throw new RPCError(resp.error.code, resp.error.message, resp.error.data)")
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
