package pulserpc

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// core-12: the spec's breaking-change scenario must match the classification
// table and ClassifySeverity. "Server removed a required field" is ClientHasMore
// (Info); "server added a required field" is ClientHasLess/required (Error).
func TestSeveritySpecScenarioMatchesClassifier(t *testing.T) {
	specPath := filepath.Join("..", "..", "..", "..", "..", "openspec", "specs",
		"contract-compatibility-verification", "spec.md")
	spec, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatalf("read spec: %v", err)
	}
	body := string(spec)
	if !strings.Contains(body, "added a required field") {
		t.Errorf("spec breaking-change scenario does not describe a required field being added")
	}
	if strings.Contains(body, "server has removed a required field\n- **THEN** result is returned with `Compatible=false`") {
		t.Errorf("spec still claims a removed required field is an Error-level breaking change")
	}
	if !strings.Contains(body, "Required field added | ClientHasLess | Error") {
		t.Errorf("spec table must classify 'Required field added' as ClientHasLess/Error")
	}

	if got := ClassifySeverity(EntityField, ChangeAdded, DirectionClientHasLess, "required"); got != SeverityError {
		t.Errorf("added required field severity = %v, want Error", got)
	}
	if got := ClassifySeverity(EntityField, ChangeRemoved, DirectionClientHasMore); got != SeverityInfo {
		t.Errorf("removed field severity = %v, want Info", got)
	}
}
