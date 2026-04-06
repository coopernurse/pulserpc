package playground

import (
	"crypto/rand"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/coopernurse/pulserpc/pkg/generator"
	"github.com/coopernurse/pulserpc/pkg/parser"
	"github.com/oklog/ulid/v2"
)

const (
	// DefaultMaxAge is the default maximum age for a session before cleanup
	DefaultMaxAge = 2 * time.Hour
	// CleanupInterval is how often the cleanup goroutine runs
	CleanupInterval = 15 * time.Minute
)

// Manager manages generation sessions and temp file cleanup
type Manager struct {
	baseDir  string        // temp base dir for all playground sessions
	maxAge   time.Duration // max session age before cleanup
	mu       sync.RWMutex
	sessions map[string]*Session // keyed by ULID

	plugins map[string]generator.Plugin // available generator plugins
}

// Session represents a single generation run
type Session struct {
	ID      string // ULID identifier
	Created time.Time
	Runtime string   // e.g., "go-client-server"
	IDL     string   // original IDL text
	Files   []string // relative file paths
	Dir     string   // absolute path to temp dir
}

// NewManager creates a new playground manager
func NewManager(baseDir string, plugins []generator.Plugin) (*Manager, error) {
	// Create base directory if it doesn't exist
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create base directory: %w", err)
	}

	// Build plugin map
	pluginMap := make(map[string]generator.Plugin)
	for _, p := range plugins {
		pluginMap[p.Name()] = p
	}

	m := &Manager{
		baseDir:  baseDir,
		maxAge:   DefaultMaxAge,
		sessions: make(map[string]*Session),
		plugins:  pluginMap,
	}

	// Start cleanup goroutine
	go m.cleanupLoop()

	return m, nil
}

// Generate creates a new session and generates code for the given IDL and runtime
func (m *Manager) Generate(idl string, runtime string) (*Session, error) {
	// Validate runtime
	plugin, ok := m.plugins[runtime]
	if !ok {
		return nil, fmt.Errorf("unknown runtime: %s", runtime)
	}

	// Parse IDL with strict validation (fail fast on errors)
	parsedIDL, err := parser.ParseIDL("", idl)
	if err != nil {
		return nil, fmt.Errorf("IDL parse error: %w", err)
	}

	// Generate ULID for session
	t := time.Now()
	entropy := ulid.Monotonic(rand.Reader, 0)
	id := ulid.MustNew(ulid.Timestamp(t), entropy).String()

	// Create session directory
	sessionDir := filepath.Join(m.baseDir, id)
	if err := os.MkdirAll(sessionDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create session directory: %w", err)
	}

	// Create session
	session := &Session{
		ID:      id,
		Created: t,
		Runtime: runtime,
		IDL:     idl,
		Dir:     sessionDir,
	}

	// Create a FlagSet for the plugin with -dir pointing to session dir
	fs := new(flag.FlagSet)
	fs.String("dir", sessionDir, "Output directory")
	fs.String("base-dir", "", "Base directory for namespace packages")

	// Call the plugin's RegisterFlags
	plugin.RegisterFlags(fs)

	// Provide a default Go module for playground generation to avoid go.mod dependency
	if runtime == "go-client-server" {
		if goModuleFlag := fs.Lookup("go-module"); goModuleFlag != nil {
			if goModuleFlag.Value.String() == "" {
				goModuleFlag.Value.Set("example.com/pulserpc-playground")
			}
		}
		// Set inline-runtime to true for playground mode
		if inlineRuntimeFlag := fs.Lookup("inline-runtime"); inlineRuntimeFlag != nil {
			inlineRuntimeFlag.Value.Set("true")
		}
	}

	// Set default values for Java-specific flags if they exist and are empty
	// Only Java requires a default package; other runtimes should work without one
	if runtime == "java-client-server" {
		if pkgFlag := fs.Lookup("package"); pkgFlag != nil {
			if pkgFlag.Value.String() == "" {
				pkgFlag.Value.Set("com.example.generated")
			}
		}
		if jsonFlag := fs.Lookup("json-lib"); jsonFlag != nil {
			if jsonFlag.Value.String() == "" {
				jsonFlag.Value.Set("jackson")
			}
		}
	}

	// Parse the FlagSet (this is needed to set the flag values)
	if err := fs.Parse([]string{}); err != nil {
		return nil, fmt.Errorf("failed to parse flags: %w", err)
	}

	// Generate code
	if err := plugin.Generate(parsedIDL, fs); err != nil {
		// Clean up session directory on error
		os.RemoveAll(sessionDir)
		return nil, fmt.Errorf("generation failed: %w", err)
	}

	// Collect all generated files
	files, err := m.collectFiles(sessionDir)
	if err != nil {
		// Clean up session directory on error
		os.RemoveAll(sessionDir)
		return nil, fmt.Errorf("failed to collect generated files: %w", err)
	}

	session.Files = files

	// Store session
	m.mu.Lock()
	m.sessions[id] = session
	m.mu.Unlock()

	return session, nil
}

// GetSession retrieves a session by ID
func (m *Manager) GetSession(id string) (*Session, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	session, ok := m.sessions[id]
	return session, ok
}

// collectFiles walks the session directory and collects all file paths
func (m *Manager) collectFiles(sessionDir string) ([]string, error) {
	var files []string

	err := filepath.WalkDir(sessionDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if d.IsDir() {
			return nil
		}

		// Get relative path from session directory
		relPath, err := filepath.Rel(sessionDir, path)
		if err != nil {
			return err
		}

		files = append(files, relPath)
		return nil
	})

	if err != nil {
		return nil, err
	}

	return files, nil
}

// cleanupLoop runs periodically to clean up old sessions
func (m *Manager) cleanupLoop() {
	ticker := time.NewTicker(CleanupInterval)
	defer ticker.Stop()

	for range ticker.C {
		m.cleanup()
	}
}

// cleanup removes sessions older than maxAge
func (m *Manager) cleanup() {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	var toDelete []string

	// Find expired sessions
	for id, session := range m.sessions {
		if now.Sub(session.Created) > m.maxAge {
			toDelete = append(toDelete, id)
		}
	}

	// Delete expired sessions
	for _, id := range toDelete {
		session := m.sessions[id]
		os.RemoveAll(session.Dir)
		delete(m.sessions, id)
	}
}
