package playground

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestGetFileSymlinkEscape documents the current behavior of session-relative
// file reads (S1-b residual): resolveSessionFile validates the lexically cleaned
// path but does not resolve symlinks before os.ReadFile. A symlink that lives
// inside the session directory yet points at a file outside it is therefore
// followed on read, leaking the outside file's content.
//
// The symlink is created after session generation so it is not part of the
// generated file set; only GetFile's read path is under test here.
func TestGetFileSymlinkEscape(t *testing.T) {
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

	// The name passed to GetFile is session-relative and passes the lexical
	// containment check in resolveSessionFile unchanged.
	data, err := mgr.GetFile(session.ID, "escape.txt")
	if err != nil {
		t.Fatalf("GetFile(symlink) returned error: %v", err)
	}
	if string(data) != secret {
		t.Fatalf("documented S1-b behavior: expected outside content %q, got %q", secret, data)
	}
	t.Logf("S1-b ESCAPE DEMONSTRATED: GetFile followed a session-dir symlink and returned %q from outside the session dir", data)
}
