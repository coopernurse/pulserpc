package webui

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/coopernurse/pulserpc/pkg/generator"
	"github.com/coopernurse/pulserpc/pkg/playground"
)

func TestHandlePlaygroundGenerate(t *testing.T) {
	// Create a test server
	server := createTestServer(t)

	// Test cases
	tests := []struct {
		name         string
		method       string
		requestBody  interface{}
		expectStatus int
		expectError  bool
		validateResp bool
	}{
		{
			name:   "valid generation request",
			method: http.MethodPost,
			requestBody: GenerateRequest{
				IDL: `
namespace test

struct User {
	name string
	age int
}
`,
				Runtime: "go-client-server",
			},
			expectStatus: 200,
			expectError:  false,
			validateResp: true,
		},
		{
			name:   "invalid method",
			method: http.MethodGet,
			requestBody: GenerateRequest{
				IDL:     "test",
				Runtime: "go-client-server",
			},
			expectStatus: 405,
			expectError:  true,
		},
		{
			name:   "missing IDL",
			method: http.MethodPost,
			requestBody: GenerateRequest{
				Runtime: "go-client-server",
			},
			expectStatus: 400,
			expectError:  true,
		},
		{
			name:   "missing runtime",
			method: http.MethodPost,
			requestBody: GenerateRequest{
				IDL: "test idl",
			},
			expectStatus: 400,
			expectError:  true,
		},
		{
			name:   "invalid runtime",
			method: http.MethodPost,
			requestBody: GenerateRequest{
				IDL:     "test idl",
				Runtime: "invalid-runtime",
			},
			expectStatus: 400,
			expectError:  true,
		},
		{
			name:   "invalid IDL",
			method: http.MethodPost,
			requestBody: GenerateRequest{
				IDL: `
namespace test

struct User {
	name string
	age: invalidtype
}
`,
				Runtime: "go-client-server",
			},
			expectStatus: 400,
			expectError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create request
			var body io.Reader
			if tt.requestBody != nil {
				jsonBody, err := json.Marshal(tt.requestBody)
				if err != nil {
					t.Fatalf("failed to marshal request: %v", err)
				}
				body = bytes.NewReader(jsonBody)
			}

			req := httptest.NewRequest(tt.method, "/api/playground/generate", body)
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			// Call handler
			server.handlePlaygroundGenerate(w, req)

			// Check status
			if w.Code != tt.expectStatus {
				t.Errorf("expected status %d, got %d", tt.expectStatus, w.Code)
			}

			// Validate response if needed
			if tt.validateResp && w.Code == 200 {
				var resp GenerateResponse
				if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
					t.Fatalf("failed to unmarshal response: %v", err)
				}

				if resp.ID == "" {
					t.Error("expected session ID to be set")
				}

				if len(resp.Files) == 0 {
					t.Error("expected files to be generated")
				}
			}
		})
	}
}

func TestHandlePlaygroundFiles(t *testing.T) {
	// Create a test server
	server := createTestServer(t)

	// First, generate a session
	genReq := GenerateRequest{
		IDL: `
namespace test

struct User {
	name string
}
`,
		Runtime: "go-client-server",
	}

	// Generate a session using the manager directly
	session, err := server.playgroundMgr.Generate(genReq.IDL, genReq.Runtime)
	if err != nil {
		t.Fatalf("failed to generate session: %v", err)
	}

	// Test cases
	tests := []struct {
		name         string
		method       string
		url          string
		expectStatus int
		expectError  bool
		expectBody   string
	}{
		{
			name:         "valid file request",
			method:       http.MethodGet,
			url:          "/api/playground/files/" + session.ID + "/" + session.Files[0],
			expectStatus: 200,
			expectError:  false,
		},
		{
			name:         "invalid method",
			method:       http.MethodPost,
			url:          "/api/playground/files/" + session.ID + "/test.txt",
			expectStatus: 405,
			expectError:  true,
		},
		{
			name:         "missing session ID",
			method:       http.MethodGet,
			url:          "/api/playground/files/",
			expectStatus: 400,
			expectError:  true,
		},
		{
			name:         "missing file path",
			method:       http.MethodGet,
			url:          "/api/playground/files/" + session.ID,
			expectStatus: 400,
			expectError:  true,
		},
		{
			name:         "non-existent session",
			method:       http.MethodGet,
			url:          "/api/playground/files/nonexistent/file.txt",
			expectStatus: 404,
			expectError:  true,
		},
		{
			name:         "non-existent file",
			method:       http.MethodGet,
			url:          "/api/playground/files/" + session.ID + "/nonexistent.txt",
			expectStatus: 404,
			expectError:  true,
			expectBody:   "File not found\n",
		},
		{
			name:         "parent traversal",
			method:       http.MethodGet,
			url:          "/api/playground/files/" + session.ID + "/../../secret.txt",
			expectStatus: 404,
			expectError:  true,
		},
		{
			name:         "encoded parent traversal",
			method:       http.MethodGet,
			url:          "/api/playground/files/" + session.ID + "/%2e%2e%2f%2e%2e%2fsecret.txt",
			expectStatus: 404,
			expectError:  true,
		},
		{
			name:         "absolute path",
			method:       http.MethodGet,
			url:          "/api/playground/files/" + session.ID + "//etc/passwd",
			expectStatus: 404,
			expectError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.url, nil)
			w := httptest.NewRecorder()

			// Call handler
			server.handlePlaygroundFiles(w, req)

			// Check status
			if w.Code != tt.expectStatus {
				t.Errorf("expected status %d, got %d. Body: %s", tt.expectStatus, w.Code, w.Body.String())
			}

			if tt.expectBody != "" && w.Body.String() != tt.expectBody {
				t.Errorf("expected body %q, got %q", tt.expectBody, w.Body.String())
			}

			// Check content type for successful requests
			if !tt.expectError && w.Code == 200 {
				contentType := w.Header().Get("Content-Type")
				if contentType == "" {
					t.Error("expected Content-Type header to be set")
				}
			}
		})
	}
}

func TestHandlePlaygroundZip(t *testing.T) {
	// Create a test server
	server := createTestServer(t)

	// First, generate a session
	genReq := GenerateRequest{
		IDL: `
namespace test

struct User {
	name string
}
`,
		Runtime: "go-client-server",
	}

	session, err := server.playgroundMgr.Generate(genReq.IDL, genReq.Runtime)
	if err != nil {
		t.Fatalf("failed to generate session: %v", err)
	}

	// Test cases
	tests := []struct {
		name         string
		method       string
		url          string
		expectStatus int
		expectError  bool
	}{
		{
			name:         "valid zip request",
			method:       http.MethodGet,
			url:          "/api/playground/zip/" + session.ID,
			expectStatus: 200,
			expectError:  false,
		},
		{
			name:         "invalid method",
			method:       http.MethodPost,
			url:          "/api/playground/zip/" + session.ID,
			expectStatus: 405,
			expectError:  true,
		},
		{
			name:         "missing session ID",
			method:       http.MethodGet,
			url:          "/api/playground/zip/",
			expectStatus: 400,
			expectError:  true,
		},
		{
			name:         "non-existent session",
			method:       http.MethodGet,
			url:          "/api/playground/zip/nonexistent",
			expectStatus: 404,
			expectError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.url, nil)
			w := httptest.NewRecorder()

			// Call handler
			server.handlePlaygroundZip(w, req)

			// Check status
			if w.Code != tt.expectStatus {
				t.Errorf("expected status %d, got %d. Body: %s", tt.expectStatus, w.Code, w.Body.String())
			}

			// Check content type for successful requests
			if !tt.expectError && w.Code == 200 {
				contentType := w.Header().Get("Content-Type")
				if contentType != "application/zip" {
					t.Errorf("expected Content-Type application/zip, got %s", contentType)
				}

				contentDisposition := w.Header().Get("Content-Disposition")
				if contentDisposition == "" {
					t.Error("expected Content-Disposition header to be set")
				}
			}
		})
	}
}

func TestExpiredSessionHandling(t *testing.T) {
	// Create a test server with short max age
	server := createTestServerWithShortMaxAge(t)

	// Generate a session
	genReq := GenerateRequest{
		IDL: `
namespace test

struct User {
	name string
}
`,
		Runtime: "go-client-server",
	}

	session, err := server.playgroundMgr.Generate(genReq.IDL, genReq.Runtime)
	if err != nil {
		t.Fatalf("failed to generate session: %v", err)
	}

	// Wait for session to expire
	time.Sleep(20 * time.Millisecond)

	// Force cleanup
	server.playgroundMgr.CleanupNow()

	// Try to access files - should get 404
	req := httptest.NewRequest(http.MethodGet, "/api/playground/files/"+session.ID+"/test.txt", nil)
	w := httptest.NewRecorder()
	server.handlePlaygroundFiles(w, req)

	if w.Code != 404 {
		t.Errorf("expected status 404 for expired session, got %d", w.Code)
	}

	// Try to download zip - should get 404
	req = httptest.NewRequest(http.MethodGet, "/api/playground/zip/"+session.ID, nil)
	w = httptest.NewRecorder()
	server.handlePlaygroundZip(w, req)

	if w.Code != 404 {
		t.Errorf("expected status 404 for expired session zip, got %d", w.Code)
	}
}

func TestAllRuntimes(t *testing.T) {
	// Create a test server
	server := createTestServer(t)

	runtimes := []string{
		"go-client-server",
		"java-client-server",
		"python-client-server",
		"ts-client-server",
		"csharp-client-server",
	}

	idl := `
namespace test

struct User {
	name string
	age int
	email string [optional]
}
`

	for _, runtime := range runtimes {
		t.Run(runtime, func(t *testing.T) {
			// Generate code
			session, err := server.playgroundMgr.Generate(idl, runtime)
			if err != nil {
				t.Fatalf("failed to generate for %s: %v", runtime, err)
			}

			// Verify files were generated
			if len(session.Files) == 0 {
				t.Errorf("expected files to be generated for %s", runtime)
			}

			// Verify we can retrieve a file
			if len(session.Files) > 0 {
				data, err := server.playgroundMgr.GetFile(session.ID, session.Files[0])
				if err != nil {
					t.Errorf("failed to get file for %s: %v", runtime, err)
				}
				if len(data) == 0 {
					t.Errorf("expected non-empty file content for %s", runtime)
				}
			}

			// Verify we can create a zip
			zipData, err := server.playgroundMgr.CreateZip(session.ID)
			if err != nil {
				t.Errorf("failed to create zip for %s: %v", runtime, err)
			}
			if len(zipData) == 0 {
				t.Errorf("expected non-empty zip data for %s", runtime)
			}
		})
	}
}

// Helper functions

func createTestServer(t *testing.T) *Server {
	tempDir, err := os.MkdirTemp("", "pulserpc-test-server-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.RemoveAll(tempDir)
	})

	plugins := []generator.Plugin{
		generator.NewGoClientServer(),
		generator.NewJavaClientServer(),
		generator.NewPythonClientServer(),
		generator.NewTSClientServer(),
		generator.NewCSharpClientServer(),
	}

	mgr, err := playground.NewManager(tempDir, plugins)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	return &Server{
		port:          8080,
		playgroundMgr: mgr,
	}
}

func createTestServerWithShortMaxAge(t *testing.T) *Server {
	tempDir, err := os.MkdirTemp("", "pulserpc-test-server-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.RemoveAll(tempDir)
	})

	plugins := []generator.Plugin{
		generator.NewGoClientServer(),
	}

	mgr, err := playground.NewManager(tempDir, plugins)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	// Override max age for testing
	mgr.SetMaxAge(10 * time.Millisecond)

	return &Server{
		port:          8080,
		playgroundMgr: mgr,
	}
}
