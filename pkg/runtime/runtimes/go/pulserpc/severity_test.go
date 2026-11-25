package pulserpc

import "testing"

// "Server removed a required field" is ClientHasMore (Info); "server added a
// required field" is ClientHasLess/required (Error).
func TestSeverityClassificationScenarios(t *testing.T) {
	if got := ClassifySeverity(EntityField, ChangeAdded, DirectionClientHasLess, "required"); got != SeverityError {
		t.Errorf("added required field severity = %v, want Error", got)
	}
	if got := ClassifySeverity(EntityField, ChangeRemoved, DirectionClientHasMore); got != SeverityInfo {
		t.Errorf("removed field severity = %v, want Info", got)
	}
}
