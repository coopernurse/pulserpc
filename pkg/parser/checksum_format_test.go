package parser

import (
	"encoding/base64"
	"strings"
	"testing"
)

// The checksum format is a 43-character unpadded base64url encoding of a
// SHA-256 digest.
func TestChecksumFormat(t *testing.T) {
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
}
