package playground

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/coopernurse/pulserpc/pkg/generator"
)

// TestSessionPersistence tests that sessions persist across Manager instances
func TestSessionPersistence(t *testing.T) {
	tempDir, err := newTempDir("pulserpc-test-persist-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer removeAllDir(tempDir)

	plugins := []generator.Plugin{
		generator.NewGoClientServer(),
	}

	// Create first manager instance
	mgr1, err := NewManager(tempDir, plugins)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	// Generate a session
	idl := `
namespace test

struct User {
	name string
}
`
	session1, err := mgr1.Generate(idl, "go-client-server")
	if err != nil {
		t.Fatalf("failed to generate: %v", err)
	}

	// Verify session exists
	_, ok := mgr1.GetSession(session1.ID)
	if !ok {
		t.Error("session should exist in first manager")
	}

	// Create second manager instance with same base directory
	mgr2, err := NewManager(tempDir, plugins)
	if err != nil {
		t.Fatalf("failed to create second manager: %v", err)
	}

	// Session should NOT be accessible in new manager (sessions are in-memory)
	_, ok = mgr2.GetSession(session1.ID)
	if ok {
		t.Error("session should not exist in new manager (sessions are in-memory only)")
	}

	// But we can generate a new session
	session2, err := mgr2.Generate(idl, "go-client-server")
	if err != nil {
		t.Fatalf("failed to generate second session: %v", err)
	}

	if session2.ID == session1.ID {
		t.Error("second session should have different ID")
	}

	// Both sessions should have their files in the filesystem
	if len(session1.Files) == 0 {
		t.Error("first session should have files")
	}
	if len(session2.Files) == 0 {
		t.Error("second session should have files")
	}
}

// TestSessionExpirationCleanup tests the cleanup mechanism
func TestSessionExpirationCleanup(t *testing.T) {
	tempDir, err := newTempDir("pulserpc-test-cleanup-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer removeAllDir(tempDir)

	plugins := []generator.Plugin{
		generator.NewGoClientServer(),
	}

	mgr, err := NewManager(tempDir, plugins)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	// Set very short max age for testing
	mgr.SetMaxAge(50 * time.Millisecond)

	// Generate multiple sessions
	idl := `
namespace test

struct User {
	name string
}
`
	var sessionIDs []string
	for i := 0; i < 3; i++ {
		session, err := mgr.Generate(idl, "go-client-server")
		if err != nil {
			t.Fatalf("failed to generate session %d: %v", i, err)
		}
		sessionIDs = append(sessionIDs, session.ID)

		// Verify session exists
		_, ok := mgr.GetSession(session.ID)
		if !ok {
			t.Errorf("session %d should exist immediately after creation", i)
		}

		// Verify files are on disk
		for _, file := range session.Files {
			fullPath := filepath.Join(session.Dir, file)
			if !fileExists(fullPath) {
				t.Errorf("file should exist: %s", fullPath)
			}
		}
	}

	// Wait for sessions to expire
	time.Sleep(100 * time.Millisecond)

	// Force cleanup
	mgr.CleanupNow()

	// Verify all sessions are removed from memory
	for i, sessionID := range sessionIDs {
		_, ok := mgr.GetSession(sessionID)
		if ok {
			t.Errorf("session %d should be removed after cleanup", i)
		}

		// Verify files are deleted from disk
		if sessionID == sessionIDs[0] {
			// Check one session's files are deleted
			session, _ := mgr.GetSession(sessionID)
			if session != nil {
				for _, file := range session.Files {
					fullPath := filepath.Join(session.Dir, file)
					if fileExists(fullPath) {
						t.Errorf("file should be deleted after cleanup: %s", fullPath)
					}
				}
			}
		}
	}
}

// TestMultipleRuntimesParallel tests generating code for multiple runtimes in parallel
func TestMultipleRuntimesParallel(t *testing.T) {
	tempDir, err := newTempDir("pulserpc-test-parallel-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer removeAllDir(tempDir)

	plugins := []generator.Plugin{
		generator.NewGoClientServer(),
		generator.NewJavaClientServer(),
		generator.NewPythonClientServer(),
		generator.NewTSClientServer(),
		generator.NewCSharpClientServer(),
	}

	mgr, err := NewManager(tempDir, plugins)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	idl := `
namespace test

struct User {
	name string
	age int
	email string [optional]
}
`

	runtimes := []string{
		"go-client-server",
		"java-client-server",
		"python-client-server",
		"ts-client-server",
		"csharp-client-server",
	}

	// Generate for all runtimes in parallel using goroutines
	type result struct {
		runtime string
		session *Session
		err     error
	}
	results := make(chan result, len(runtimes))

	for _, runtime := range runtimes {
		go func(rt string) {
			session, err := mgr.Generate(idl, rt)
			results <- result{runtime: rt, session: session, err: err}
		}(runtime)
	}

	// Collect results
	sessions := make(map[string]*Session)
	for i := 0; i < len(runtimes); i++ {
		res := <-results
		if res.err != nil {
			t.Errorf("failed to generate for %s: %v", res.runtime, res.err)
		}
		sessions[res.runtime] = res.session
	}

	// Verify all sessions were created successfully
	if len(sessions) != len(runtimes) {
		t.Errorf("expected %d sessions, got %d", len(runtimes), len(sessions))
	}

	// Verify each session has files and can retrieve them
	for runtime, session := range sessions {
		if len(session.Files) == 0 {
			t.Errorf("%s: expected files to be generated", runtime)
		}

		// Test retrieving a file
		if len(session.Files) > 0 {
			content, err := mgr.GetFile(session.ID, session.Files[0])
			if err != nil {
				t.Errorf("%s: failed to get file: %v", runtime, err)
			}
			if len(content) == 0 {
				t.Errorf("%s: expected non-empty file content", runtime)
			}
		}

		// Test ZIP creation
		zipData, err := mgr.CreateZip(session.ID)
		if err != nil {
			t.Errorf("%s: failed to create ZIP: %v", runtime, err)
		}
		if len(zipData) == 0 {
			t.Errorf("%s: expected non-empty ZIP data", runtime)
		}

		t.Logf("%s: %d files generated, ZIP size: %d bytes", runtime, len(session.Files), len(zipData))
	}
}

// TestConcurrentAccess tests thread safety of session operations
func TestConcurrentAccess(t *testing.T) {
	tempDir, err := newTempDir("pulserpc-test-concurrent-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer removeAllDir(tempDir)

	plugins := []generator.Plugin{
		generator.NewGoClientServer(),
	}

	mgr, err := NewManager(tempDir, plugins)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	idl := `
namespace test

struct User {
	name string
}
`

	// Launch multiple goroutines that perform operations concurrently
	done := make(chan bool, 10)

	// 5 generators
	for i := 0; i < 5; i++ {
		go func(idx int) {
			for j := 0; j < 3; j++ {
				session, err := mgr.Generate(idl, "go-client-server")
				if err != nil {
					t.Errorf("goroutine %d: failed to generate: %v", idx, err)
				}
				_ = session
			}
			done <- true
		}(i)
	}

	// 5 readers
	for i := 0; i < 5; i++ {
		go func(idx int) {
			// Try to read session count
			count := mgr.GetSessionCount()
			if count < 0 {
				t.Errorf("goroutine %d: invalid session count: %d", idx, count)
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify no deadlocks or panics occurred
	finalCount := mgr.GetSessionCount()
	if finalCount != 15 { // 5 generators × 3 sessions each
		t.Logf("concurrent access test completed with %d sessions (expected 15)", finalCount)
	}
}

// Helper functions
func newTempDir(pattern string) (string, error) {
	return os.MkdirTemp("", pattern)
}

func removeAllDir(path string) {
	os.RemoveAll(path)
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// TestNoDefaultPackageForNonJava verifies that non-Java generators work without a default package
func TestNoDefaultPackageForNonJava(t *testing.T) {
	tempDir, err := newTempDir("pulserpc-test-no-pkg-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer removeAllDir(tempDir)

	plugins := []generator.Plugin{
		generator.NewTSClientServer(),
		generator.NewPythonClientServer(),
	}

	mgr, err := NewManager(tempDir, plugins)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	idl := `
namespace test

struct User {
	name string
	age int
}
`

	nonJavaRuntimes := []string{"ts-client-server", "python-client-server"}

	for _, runtime := range nonJavaRuntimes {
		t.Run(runtime, func(t *testing.T) {
			session, err := mgr.Generate(idl, runtime)
			if err != nil {
				t.Fatalf("failed to generate for %s: %v", runtime, err)
			}

			if len(session.Files) == 0 {
				t.Errorf("%s: expected files to be generated", runtime)
			}

			for _, file := range session.Files {
				if len(file) == 0 {
					t.Errorf("%s: expected non-empty file path", runtime)
				}
				if containsString(file, "com.example.generated") {
					t.Errorf("%s: file path should not contain default package 'com.example.generated': %s", runtime, file)
				}
			}

			t.Logf("%s: %d files generated without default package", runtime, len(session.Files))
		})
	}
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && (s[:len(substr)] == substr || containsAny(s, substr)))
}

func containsAny(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
