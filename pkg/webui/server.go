package webui

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/coopernurse/pulserpc/pkg/generator"
	"github.com/coopernurse/pulserpc/pkg/playground"
)

// Server represents the web UI HTTP server
type Server struct {
	port          int
	playgroundMgr *playground.Manager
}

// NewServer creates a new web UI server
func NewServer(port int) *Server {
	// Create playground manager with temp directory
	tempDir := filepath.Join(os.TempDir(), "pulserpc-playground")
	plugins := []generator.Plugin{
		generator.NewGoClientServer(),
		generator.NewJavaClientServer(),
		generator.NewPythonClientServer(),
		generator.NewTSClientServer(),
		generator.NewCSharpClientServer(),
	}

	mgr, err := playground.NewManager(tempDir, plugins)
	if err != nil {
		log.Fatalf("Failed to create playground manager: %v", err)
	}

	return &Server{
		port:          port,
		playgroundMgr: mgr,
	}
}

// Start starts the HTTP server
func (s *Server) Start() error {
	mux := http.NewServeMux()

	// Serve static files
	mux.HandleFunc("/", s.handleStatic)
	mux.HandleFunc("/app.js", s.handleJS)
	mux.HandleFunc("/css/", s.handleCSS)

	// API endpoints
	mux.HandleFunc("/api/proxy", s.handleProxy)

	// Playground API endpoints
	mux.HandleFunc("/api/playground/generate", s.handlePlaygroundGenerate)
	mux.HandleFunc("/api/playground/files/", s.handlePlaygroundFiles)
	mux.HandleFunc("/api/playground/zip/", s.handlePlaygroundZip)

	addr := fmt.Sprintf(":%d", s.port)
	log.Printf("PulseRPC Web UI server starting on http://localhost%s", addr)
	log.Printf("Open http://localhost%s in your browser", addr)

	return http.ListenAndServe(addr, mux)
}

// handleStatic serves the index.html file
func (s *Server) handleStatic(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	data, err := WebUIFiles.ReadFile("dist/index.html")
	if err != nil {
		http.Error(w, "Failed to read index.html", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if _, err := w.Write(data); err != nil {
		log.Printf("Failed to write response: %v", err)
	}
}

// handleJS serves the bundled JavaScript file
func (s *Server) handleJS(w http.ResponseWriter, r *http.Request) {
	data, err := WebUIFiles.ReadFile("dist/app.js")
	if err != nil {
		http.Error(w, "Failed to read app.js", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	if _, err := w.Write(data); err != nil {
		log.Printf("Failed to write response: %v", err)
	}
}

// handleCSS serves CSS files
func (s *Server) handleCSS(w http.ResponseWriter, r *http.Request) {
	// Extract the CSS file path. Named cssPath to avoid shadowing the
	// imported "path" package used below for embed.FS paths.
	cssPath := strings.TrimPrefix(r.URL.Path, "/css/")
	if cssPath == "" {
		http.NotFound(w, r)
		return
	}

	// embed.FS paths always use forward slashes regardless of the host OS.
	// filepath.Join emits backslashes on Windows, which embed.FS rejects.
	filePath := path.Join("dist", "css", cssPath)
	data, err := WebUIFiles.ReadFile(filePath)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "text/css; charset=utf-8")
	if _, err := w.Write(data); err != nil {
		log.Printf("Failed to write response: %v", err)
	}
}

// handleProxy proxies JSON-RPC requests to the target endpoint
func (s *Server) handleProxy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get target endpoint from header
	targetEndpoint := r.Header.Get("X-Target-Endpoint")
	if targetEndpoint == "" {
		http.Error(w, "X-Target-Endpoint header is required", http.StatusBadRequest)
		return
	}

	// Read request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to read request body: %v", err), http.StatusBadRequest)
		return
	}
	defer func() {
		if err := r.Body.Close(); err != nil {
			log.Printf("Failed to close request body: %v", err)
		}
	}()

	// Validate JSON
	var jsonReq map[string]interface{}
	if err := json.Unmarshal(body, &jsonReq); err != nil {
		http.Error(w, fmt.Sprintf("Invalid JSON: %v", err), http.StatusBadRequest)
		return
	}

	// Create request to target endpoint
	proxyReq, err := http.NewRequest(http.MethodPost, targetEndpoint, strings.NewReader(string(body)))
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to create proxy request: %v", err), http.StatusInternalServerError)
		return
	}

	// Copy relevant headers
	proxyReq.Header.Set("Content-Type", "application/json")
	for key, values := range r.Header {
		if strings.HasPrefix(strings.ToLower(key), "x-") && key != "X-Target-Endpoint" && key != "X-PulseRPC-Headers" {
			for _, value := range values {
				proxyReq.Header.Add(key, value)
			}
		}
	}

	// Add custom headers from X-PulseRPC-Headers
	if hJson := r.Header.Get("X-PulseRPC-Headers"); hJson != "" {
		var customHeaders map[string]string
		if err := json.Unmarshal([]byte(hJson), &customHeaders); err == nil {
			for k, v := range customHeaders {
				proxyReq.Header.Set(k, v)
			}
		}
	}

	// Make request
	client := &http.Client{}
	resp, err := client.Do(proxyReq)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to proxy request: %v", err), http.StatusBadGateway)
		return
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Printf("Failed to close response body: %v", err)
		}
	}()

	// Read response
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to read response: %v", err), http.StatusBadGateway)
		return
	}

	// Set CORS headers
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Target-Endpoint")
	w.Header().Set("Content-Type", "application/json")

	// Copy status code
	w.WriteHeader(resp.StatusCode)
	if _, err := w.Write(respBody); err != nil {
		log.Printf("Failed to write response: %v", err)
	}
}

// GenerateRequest represents the request body for /api/playground/generate
type GenerateRequest struct {
	IDL     string `json:"idl"`
	Runtime string `json:"runtime"`
}

// GenerateResponse represents the response from /api/playground/generate
type GenerateResponse struct {
	ID    string   `json:"id"`
	Files []string `json:"files"`
}

// handlePlaygroundGenerate handles code generation requests
func (s *Server) handlePlaygroundGenerate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Read request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to read request body: %v", err), http.StatusBadRequest)
		return
	}
	defer func() {
		if err := r.Body.Close(); err != nil {
			log.Printf("Failed to close request body: %v", err)
		}
	}()

	// Parse request
	var req GenerateRequest
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid JSON: %v", err), http.StatusBadRequest)
		return
	}

	// Validate request
	if req.IDL == "" {
		http.Error(w, "IDL is required", http.StatusBadRequest)
		return
	}
	if req.Runtime == "" {
		http.Error(w, "Runtime is required", http.StatusBadRequest)
		return
	}

	// Generate code
	session, err := s.playgroundMgr.Generate(req.IDL, req.Runtime)
	if err != nil {
		http.Error(w, fmt.Sprintf("Generation failed: %v", err), http.StatusBadRequest)
		return
	}

	// Send response
	resp := GenerateResponse{
		ID:    session.ID,
		Files: session.Files,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		log.Printf("Failed to encode response: %v", err)
	}
}

// handlePlaygroundFiles handles file retrieval requests
func (s *Server) handlePlaygroundFiles(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract session ID and file path from URL
	// URL format: /api/playground/files/<session-id>/<file-path>
	prefix := "/api/playground/files/"
	path := strings.TrimPrefix(r.URL.Path, prefix)

	if path == "" {
		http.Error(w, "Session ID and file path are required", http.StatusBadRequest)
		return
	}

	// Split into session ID and file path
	parts := strings.SplitN(path, "/", 2)
	if len(parts) < 2 {
		http.Error(w, "File path is required", http.StatusBadRequest)
		return
	}

	sessionID := parts[0]
	filePath := parts[1]

	// Check if session exists
	_, ok := s.playgroundMgr.GetSession(sessionID)
	if !ok {
		http.Error(w, fmt.Sprintf("Session not found: %s. It may have expired (sessions are deleted after 2 hours)", sessionID), http.StatusNotFound)
		return
	}

	// Check if session is expired
	if s.playgroundMgr.IsExpired(sessionID) {
		http.Error(w, "Session has expired (sessions are deleted after 2 hours)", http.StatusNotFound)
		return
	}

	// Get file contents
	data, err := s.playgroundMgr.GetFile(sessionID, filePath)
	if err != nil {
		log.Printf("Failed to read file for session %s, path %s: %v", sessionID, filePath, err)
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	// Detect content type based on file extension
	contentType := detectContentType(filePath)

	w.Header().Set("Content-Type", contentType)
	if _, err := w.Write(data); err != nil {
		log.Printf("Failed to write response: %v", err)
	}
}

// handlePlaygroundZip handles ZIP archive download requests
func (s *Server) handlePlaygroundZip(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract session ID from URL
	// URL format: /api/playground/zip/<session-id>
	prefix := "/api/playground/zip/"
	sessionID := strings.TrimPrefix(r.URL.Path, prefix)

	if sessionID == "" || sessionID == r.URL.Path {
		http.Error(w, "Session ID is required", http.StatusBadRequest)
		return
	}

	// Check if session exists
	session, ok := s.playgroundMgr.GetSession(sessionID)
	if !ok {
		http.Error(w, fmt.Sprintf("Session not found: %s. It may have expired (sessions are deleted after 2 hours)", sessionID), http.StatusNotFound)
		return
	}

	// Check if session is expired
	if s.playgroundMgr.IsExpired(sessionID) {
		http.Error(w, "Session has expired (sessions are deleted after 2 hours)", http.StatusNotFound)
		return
	}

	// Create ZIP archive
	zipData, err := s.playgroundMgr.CreateZip(sessionID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to create ZIP: %v", err), http.StatusInternalServerError)
		return
	}

	// Send ZIP file
	filename := session.GetZipFilename()
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	if _, err := w.Write(zipData); err != nil {
		log.Printf("Failed to write response: %v", err)
	}
}

// detectContentType returns the appropriate content type for a file based on its extension
func detectContentType(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))

	switch ext {
	case ".go":
		return "text/plain; charset=utf-8"
	case ".java":
		return "text/plain; charset=utf-8"
	case ".py":
		return "text/plain; charset=utf-8"
	case ".ts", ".tsx":
		return "text/plain; charset=utf-8"
	case ".js":
		return "text/javascript; charset=utf-8"
	case ".cs":
		return "text/plain; charset=utf-8"
	case ".json":
		return "application/json"
	case ".md":
		return "text/markdown; charset=utf-8"
	case ".html":
		return "text/html; charset=utf-8"
	case ".css":
		return "text/css; charset=utf-8"
	case ".xml":
		return "text/xml; charset=utf-8"
	case ".txt":
		return "text/plain; charset=utf-8"
	default:
		return "application/octet-stream"
	}
}
