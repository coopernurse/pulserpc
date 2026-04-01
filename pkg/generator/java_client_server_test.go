package generator

import (
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/coopernurse/pulserpc/pkg/parser"
)

func TestJavaGeneratorBasicFiles(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pulserpc-java-gen-")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Build a minimal IDL with one namespace, a struct and an enum
	idl := &parser.IDL{
		Structs: []*parser.Struct{
			{
				Name:      "inc.Req",
				Namespace: "inc",
				Fields:    []*parser.Field{{Name: "msg", Type: &parser.Type{BuiltIn: "string"}}},
			},
		},
		Enums: []*parser.Enum{
			{
				Name:      "inc.Status",
				Namespace: "inc",
				Values:    []*parser.EnumValue{{Name: "ok"}},
			},
		},
		Interfaces: []*parser.Interface{
			{
				Name:      "A",
				Namespace: "",
			},
		},
	}

	p := NewJavaClientServer()
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	// ensure dir flag exists
	fs.String("dir", "", "output dir")
	p.RegisterFlags(fs)
	if err := fs.Set("dir", tmpDir); err != nil {
		t.Fatalf("failed to set dir flag: %v", err)
	}
	if err := fs.Set("package", "com.example"); err != nil {
		t.Fatalf("failed to set base-package flag: %v", err)
	}

	if err := p.Generate(idl, fs); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	// Check namespace Idl file
	nsPath := filepath.Join(tmpDir, "src", "main", "java", "com", "example", "inc", "incIdl.java")
	if _, err := os.Stat(nsPath); err != nil {
		t.Fatalf("expected namespace idl file at %s, missing: %v", nsPath, err)
	}

	// Check Server.java and Client.java in base package
	serverPath := filepath.Join(tmpDir, "src", "main", "java", "com", "example", "Server.java")
	if _, err := os.Stat(serverPath); err != nil {
		t.Fatalf("expected Server.java at %s, missing: %v", serverPath, err)
	}
	clientPath := filepath.Join(tmpDir, "src", "main", "java", "com", "example", "Client.java")
	if _, err := os.Stat(clientPath); err != nil {
		t.Fatalf("expected Client.java at %s, missing: %v", clientPath, err)
	}
}

func TestJavaGeneratorTestFilesWithFlag(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pulserpc-java-gen-")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Build a minimal IDL with an interface
	idl := &parser.IDL{
		Interfaces: []*parser.Interface{
			{
				Name:      "A",
				Namespace: "",
				Methods: []*parser.Method{
					{
						Name:       "add",
						Parameters: []*parser.Parameter{{Name: "a", Type: &parser.Type{BuiltIn: "int"}}, {Name: "b", Type: &parser.Type{BuiltIn: "int"}}},
						ReturnType: &parser.Type{BuiltIn: "int"},
					},
				},
			},
		},
	}

	p := NewJavaClientServer()
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	fs.String("dir", "", "output dir")
	fs.Bool("generate-test-files", false, "generate test files")
	p.RegisterFlags(fs)
	if err := fs.Set("dir", tmpDir); err != nil {
		t.Fatalf("failed to set dir flag: %v", err)
	}
	if err := fs.Set("package", "com.example"); err != nil {
		t.Fatalf("failed to set base-package flag: %v", err)
	}
	if err := fs.Set("generate-test-files", "true"); err != nil {
		t.Fatalf("failed to set generate-test-files flag: %v", err)
	}

	if err := p.Generate(idl, fs); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	// Check that test files are generated when flag is true
	testServerPath := filepath.Join(tmpDir, "src", "test", "java", "com", "example", "TestServer.java")
	if _, err := os.Stat(testServerPath); err != nil {
		t.Fatalf("expected TestServer.java at %s, missing: %v", testServerPath, err)
	}
	testClientPath := filepath.Join(tmpDir, "src", "test", "java", "com", "example", "TestClient.java")
	if _, err := os.Stat(testClientPath); err != nil {
		t.Fatalf("expected TestClient.java at %s, missing: %v", testClientPath, err)
	}
}

func TestJavaGeneratorTestFilesDisabled(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pulserpc-java-gen-")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Build a minimal IDL with an interface
	idl := &parser.IDL{
		Interfaces: []*parser.Interface{
			{
				Name:      "A",
				Namespace: "",
				Methods: []*parser.Method{
					{
						Name:       "add",
						Parameters: []*parser.Parameter{{Name: "a", Type: &parser.Type{BuiltIn: "int"}}, {Name: "b", Type: &parser.Type{BuiltIn: "int"}}},
						ReturnType: &parser.Type{BuiltIn: "int"},
					},
				},
			},
		},
	}

	p := NewJavaClientServer()
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	fs.String("dir", "", "output dir")
	fs.Bool("generate-test-files", false, "generate test files")
	p.RegisterFlags(fs)
	if err := fs.Set("dir", tmpDir); err != nil {
		t.Fatalf("failed to set dir flag: %v", err)
	}
	if err := fs.Set("package", "com.example"); err != nil {
		t.Fatalf("failed to set base-package flag: %v", err)
	}
	// Explicitly disable test file generation
	if err := fs.Set("generate-test-files", "false"); err != nil {
		t.Fatalf("failed to set generate-test-files flag: %v", err)
	}

	if err := p.Generate(idl, fs); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	// Check that test files are NOT generated when generate-test-files is false
	testServerPath := filepath.Join(tmpDir, "src", "test", "java", "com", "example", "TestServer.java")
	if _, err := os.Stat(testServerPath); err == nil {
		t.Fatalf("TestServer.java should NOT be generated when -generate-test-files=false")
	}
	testClientPath := filepath.Join(tmpDir, "src", "test", "java", "com", "example", "TestClient.java")
	if _, err := os.Stat(testClientPath); err == nil {
		t.Fatalf("TestClient.java should NOT be generated when -generate-test-files=false")
	}
}

func TestJavaNamespacedDirectoryStructure(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pulserpc-java-gen-")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	idl := &parser.IDL{
		Structs: []*parser.Struct{
			{
				Name:      "book.Book",
				Namespace: "book",
				Fields:    []*parser.Field{{Name: "title", Type: &parser.Type{BuiltIn: "string"}}},
			},
		},
		Interfaces: []*parser.Interface{
			{
				Name:      "book.Library",
				Namespace: "book",
				Methods: []*parser.Method{
					{
						Name:       "getBook",
						Parameters: []*parser.Parameter{{Name: "id", Type: &parser.Type{BuiltIn: "string"}}},
						ReturnType: &parser.Type{UserDefined: "book.Book"},
					},
				},
			},
		},
	}

	p := NewJavaClientServer()
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	fs.String("dir", "src/main/java", "output dir")
	p.RegisterFlags(fs)
	if err := fs.Set("dir", tmpDir); err != nil {
		t.Fatalf("failed to set dir flag: %v", err)
	}
	if err := fs.Set("package", "com.myapp.rpc"); err != nil {
		t.Fatalf("failed to set package flag: %v", err)
	}

	if err := p.Generate(idl, fs); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	basePath := filepath.Join(tmpDir, "src", "main", "java")

	bookDir := filepath.Join(basePath, "com", "myapp", "rpc", "book")
	if _, err := os.Stat(bookDir); err != nil {
		t.Fatalf("expected book namespace directory at %s, missing: %v", bookDir, err)
	}

	bookPath := filepath.Join(bookDir, "Book.java")
	if _, err := os.Stat(bookPath); err != nil {
		t.Fatalf("expected Book.java at %s, missing: %v", bookPath, err)
	}

	content, err := os.ReadFile(bookPath)
	if err != nil {
		t.Fatalf("failed to read Book.java: %v", err)
	}
	if !containsString(string(content), "package com.myapp.rpc.book;") {
		t.Fatalf("Book.java should contain 'package com.myapp.rpc.book;' but got:\n%s", string(content))
	}

	clientPath := filepath.Join(bookDir, "LibraryClient.java")
	if _, err := os.Stat(clientPath); err != nil {
		t.Fatalf("expected LibraryClient.java at %s, missing: %v", clientPath, err)
	}

	clientContent, err := os.ReadFile(clientPath)
	if err != nil {
		t.Fatalf("failed to read LibraryClient.java: %v", err)
	}
	if !containsString(string(clientContent), "package com.myapp.rpc.book;") {
		t.Fatalf("LibraryClient.java should contain 'package com.myapp.rpc.book;' but got:\n%s", string(clientContent))
	}
}

func TestJavaMultipleNamespacesSamePackageRoot(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pulserpc-java-gen-")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	idl := &parser.IDL{
		Structs: []*parser.Struct{
			{
				Name:      "user.User",
				Namespace: "user",
				Fields:    []*parser.Field{{Name: "name", Type: &parser.Type{BuiltIn: "string"}}},
			},
			{
				Name:      "project.Project",
				Namespace: "project",
				Fields:    []*parser.Field{{Name: "name", Type: &parser.Type{BuiltIn: "string"}}},
			},
		},
	}

	p := NewJavaClientServer()
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	fs.String("dir", "src/main/java", "output dir")
	p.RegisterFlags(fs)
	if err := fs.Set("dir", tmpDir); err != nil {
		t.Fatalf("failed to set dir flag: %v", err)
	}
	if err := fs.Set("package", "com.myapp.rpc"); err != nil {
		t.Fatalf("failed to set package flag: %v", err)
	}

	if err := p.Generate(idl, fs); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	basePath := filepath.Join(tmpDir, "src", "main", "java")

	userDir := filepath.Join(basePath, "com", "myapp", "rpc", "user")
	if _, err := os.Stat(userDir); err != nil {
		t.Fatalf("expected user namespace directory at %s, missing: %v", userDir, err)
	}

	projectDir := filepath.Join(basePath, "com", "myapp", "rpc", "project")
	if _, err := os.Stat(projectDir); err != nil {
		t.Fatalf("expected project namespace directory at %s, missing: %v", projectDir, err)
	}

	userPath := filepath.Join(userDir, "User.java")
	if _, err := os.Stat(userPath); err != nil {
		t.Fatalf("expected User.java at %s, missing: %v", userPath, err)
	}

	projectPath := filepath.Join(projectDir, "Project.java")
	if _, err := os.Stat(projectPath); err != nil {
		t.Fatalf("expected Project.java at %s, missing: %v", projectPath, err)
	}

	userContent, err := os.ReadFile(userPath)
	if err != nil {
		t.Fatalf("failed to read User.java: %v", err)
	}
	if !containsString(string(userContent), "package com.myapp.rpc.user;") {
		t.Fatalf("User.java should contain 'package com.myapp.rpc.user;' but got:\n%s", string(userContent))
	}

	projectContent, err := os.ReadFile(projectPath)
	if err != nil {
		t.Fatalf("failed to read Project.java: %v", err)
	}
	if !containsString(string(projectContent), "package com.myapp.rpc.project;") {
		t.Fatalf("Project.java should contain 'package com.myapp.rpc.project;' but got:\n%s", string(projectContent))
	}
}

func TestJavaNamespaceCasePreserved(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pulserpc-java-gen-")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	idl := &parser.IDL{
		Structs: []*parser.Struct{
			{
				Name:      "UserProfile.Address",
				Namespace: "UserProfile",
				Fields:    []*parser.Field{{Name: "street", Type: &parser.Type{BuiltIn: "string"}}},
			},
		},
	}

	p := NewJavaClientServer()
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	fs.String("dir", "src/main/java", "output dir")
	p.RegisterFlags(fs)
	if err := fs.Set("dir", tmpDir); err != nil {
		t.Fatalf("failed to set dir flag: %v", err)
	}
	if err := fs.Set("package", "com.myapp.rpc"); err != nil {
		t.Fatalf("failed to set package flag: %v", err)
	}

	if err := p.Generate(idl, fs); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	basePath := filepath.Join(tmpDir, "src", "main", "java")

	uppercaseDir := filepath.Join(basePath, "com", "myapp", "rpc", "UserProfile")
	if _, err := os.Stat(uppercaseDir); err != nil {
		t.Fatalf("expected UserProfile namespace directory with preserved case at %s, missing: %v", uppercaseDir, err)
	}

	addressPath := filepath.Join(uppercaseDir, "Address.java")
	if _, err := os.Stat(addressPath); err != nil {
		t.Fatalf("expected Address.java at %s, missing: %v", addressPath, err)
	}

	content, err := os.ReadFile(addressPath)
	if err != nil {
		t.Fatalf("failed to read Address.java: %v", err)
	}
	if !containsString(string(content), "package com.myapp.rpc.UserProfile;") {
		t.Fatalf("Address.java should contain 'package com.myapp.rpc.UserProfile;' but got:\n%s", string(content))
	}

	idlPath := filepath.Join(uppercaseDir, "UserProfileIdl.java")
	if _, err := os.Stat(idlPath); err != nil {
		t.Fatalf("expected UserProfileIdl.java at %s, missing: %v", idlPath, err)
	}
}

func TestJavaRuntimeInPulserpcDir(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pulserpc-java-gen-")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	idl := &parser.IDL{
		Structs: []*parser.Struct{
			{
				Name:      "book.Book",
				Namespace: "book",
				Fields:    []*parser.Field{{Name: "title", Type: &parser.Type{BuiltIn: "string"}}},
			},
		},
		Interfaces: []*parser.Interface{
			{
				Name:      "book.Library",
				Namespace: "book",
			},
		},
	}

	p := NewJavaClientServer()
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	fs.String("dir", "src/main/java", "output dir")
	p.RegisterFlags(fs)
	if err := fs.Set("dir", tmpDir); err != nil {
		t.Fatalf("failed to set dir flag: %v", err)
	}
	if err := fs.Set("package", "com.myapp.rpc"); err != nil {
		t.Fatalf("failed to set package flag: %v", err)
	}

	if err := p.Generate(idl, fs); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	// Runtime should be at {dir}/pulserpc/
	runtimeDir := filepath.Join(tmpDir, "pulserpc")
	if _, err := os.Stat(runtimeDir); err != nil {
		t.Fatalf("expected runtime directory at %s, missing: %v", runtimeDir, err)
	}

	rpcErrorPath := filepath.Join(runtimeDir, "RPCError.java")
	if _, err := os.Stat(rpcErrorPath); err != nil {
		t.Fatalf("expected RPCError.java at %s, missing: %v", rpcErrorPath, err)
	}

	rpcErrorContent, err := os.ReadFile(rpcErrorPath)
	if err != nil {
		t.Fatalf("failed to read RPCError.java: %v", err)
	}
	if !containsString(string(rpcErrorContent), "package pulserpc;") {
		t.Fatalf("RPCError.java should contain 'package pulserpc;' but got:\n%s", string(rpcErrorContent))
	}

	// Runtime should NOT be in namespace directory
	namespaceDir := filepath.Join(tmpDir, "src", "main", "java", "com", "myapp", "rpc", "book")
	if _, err := os.Stat(filepath.Join(namespaceDir, "RPCError.java")); err == nil {
		t.Fatalf("runtime files should NOT be in namespace directory")
	}
}

func TestJavaRuntimeLocationWithNestedDirAndPackage(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pulserpc-java-gen-")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	idl := &parser.IDL{
		Structs: []*parser.Struct{
			{
				Name:      "user.User",
				Namespace: "user",
				Fields:    []*parser.Field{{Name: "name", Type: &parser.Type{BuiltIn: "string"}}},
			},
		},
	}

	p := NewJavaClientServer()
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	fs.String("dir", tmpDir, "output dir")
	p.RegisterFlags(fs)
	if err := fs.Set("dir", tmpDir); err != nil {
		t.Fatalf("failed to set dir flag: %v", err)
	}
	if err := fs.Set("package", "org.company.services"); err != nil {
		t.Fatalf("failed to set package flag: %v", err)
	}

	if err := p.Generate(idl, fs); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	// Runtime should be at {dir}/pulserpc/
	runtimeDir := filepath.Join(tmpDir, "pulserpc")
	if _, err := os.Stat(runtimeDir); err != nil {
		t.Fatalf("expected runtime directory at %s, missing: %v", runtimeDir, err)
	}

	rpcErrorPath := filepath.Join(runtimeDir, "RPCError.java")
	if _, err := os.Stat(rpcErrorPath); err != nil {
		t.Fatalf("expected RPCError.java at %s, missing: %v", rpcErrorPath, err)
	}

	// Runtime should NOT be in package hierarchy
	wrongRuntimeDir := filepath.Join(tmpDir, "src", "main", "java", "org", "company", "services", "pulserpc")
	if _, err := os.Stat(wrongRuntimeDir); err == nil {
		t.Fatalf("runtime should NOT be at %s", wrongRuntimeDir)
	}
}
