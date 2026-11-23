package playground

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestGetFileRejectsSymlinkEscape verifies that a session-relative name which
// is lexically inside the session directory but resolves through a symlink to
// a file outside it is rejected (S1-b). The symlink is created after session
// generation so only GetFile's read path is under test.
func TestGetFileRejectsSymlinkEscape(t *testing.T) {
	baseDir, err := os.MkdirTemp("", "pulserpc-test-symlink-")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(baseDir)

	mgr, err := NewManager(baseDir, nil)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	session := &Session{
		ID:      "symlink-session",
		Created: time.Now(),
		Dir:     filepath.Join(baseDir, "symlink-session"),
		Runtime: "test",
	}
	if err := os.MkdirAll(session.Dir, 0755); err != nil {
		t.Fatalf("failed to create session dir: %v", err)
	}
	mgr.sessions[session.ID] = session

	// The secret lives OUTSIDE the session directory, in the parent base dir.
	secretPath := filepath.Join(baseDir, "secret.txt")
	const secret = "outside-secret"
	if err := os.WriteFile(secretPath, []byte(secret), 0644); err != nil {
		t.Fatalf("failed to write secret: %v", err)
	}

	linkPath := filepath.Join(session.Dir, "escape.txt")
	if err := os.Symlink(secretPath, linkPath); err != nil {
		t.Skipf("symlink creation not supported here, skipping S1-b check: %v", err)
	}
	defer os.Remove(linkPath)

	// The name is session-relative and passes the lexical containment check;
	// the symlink re-check in resolveSessionFile must reject the read.
	data, err := mgr.GetFile(session.ID, "escape.txt")
	if err == nil {
		t.Fatalf("GetFile(symlink) must be rejected, got content %q", data)
	}
	if len(data) != 0 {
		t.Fatalf("rejected read must return no content, got %q", data)
	}
	t.Logf("GetFile(symlink) rejected as expected: %v", err)
}
