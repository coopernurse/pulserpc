package parser

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// core-05: the archived spec and the implementation must agree on the checksum
// format (43-character unpadded base64url of a SHA-256 digest).
func TestChecksumFormatMatchesArchivedSpec(t *testing.T) {
	idl := &IDL{RootNamespace: "test", Structs: []*Struct{{Name: "S", Namespace: "test"}}}
	checksum, err := ComputeChecksum(idl)
	if err != nil {
		t.Fatalf("ComputeChecksum: %v", err)
	}
	if len(checksum) != 43 {
		t.Fatalf("checksum length = %d, want 43", len(checksum))
	}
	if strings.Contains(checksum, "=") {
		t.Errorf("checksum must be unpadded base64url, got %q", checksum)
	}
	raw, err := base64.URLEncoding.WithPadding(base64.NoPadding).DecodeString(checksum)
	if err != nil {
		t.Fatalf("checksum is not unpadded base64url: %v", err)
	}
	if len(raw) != 32 {
		t.Errorf("decoded checksum is %d bytes, want 32", len(raw))
	}

	specPath := filepath.Join("..", "..", "openspec", "changes", "archive",
		"2026-04-12-idl-checksum", "specs", "idl-checksum", "spec.md")
	spec, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatalf("read spec: %v", err)
	}
	if !strings.Contains(string(spec), "43 base64url characters") || strings.Contains(string(spec), "44 base64url characters") {
		t.Errorf("archived spec still claims a 44-character checksum")
	}
}
