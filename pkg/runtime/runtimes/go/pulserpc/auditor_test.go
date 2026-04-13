package pulserpc

import (
	"context"
	"testing"
	"time"
)

func TestNoOpAuditor(t *testing.T) {
	auditor := &NoOpAuditor{}

	result := &VerificationResult{
		Compatible:     true,
		ServerChecksum: "abc123",
		ClientChecksum: "def456",
		Deltas:         []ContractDelta{},
		Timestamp:      time.Now(),
	}

	ctx := context.Background()

	auditor.Audit(ctx, result)

	if auditor.Name() != "NoOp" {
		t.Errorf("expected name NoOp, got %s", auditor.Name())
	}
}

func TestLoggingAuditor(t *testing.T) {
	auditor := &LoggingAuditor{}

	result := &VerificationResult{
		Compatible:     true,
		ServerChecksum: "abc123",
		ClientChecksum: "def456",
		Deltas:         []ContractDelta{},
		Timestamp:      time.Now(),
	}

	ctx := context.Background()

	auditor.Audit(ctx, result)

	if auditor.Name() != "Logging" {
		t.Errorf("expected name Logging, got %s", auditor.Name())
	}
}

func TestFailFastAuditor_Name(t *testing.T) {
	auditor := &FailFastAuditor{}

	if auditor.Name() != "FailFast" {
		t.Errorf("expected name FailFast, got %s", auditor.Name())
	}
}

func TestFailFastAuditor_NoPanicOnCompatible(t *testing.T) {
	auditor := &FailFastAuditor{}

	result := &VerificationResult{
		Compatible:     true,
		ServerChecksum: "abc123",
		ClientChecksum: "def456",
		Deltas:         []ContractDelta{},
		Timestamp:      time.Now(),
	}

	ctx := context.Background()

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("unexpected panic: %v", r)
		}
	}()

	auditor.Audit(ctx, result)
}

func TestFailFastAuditor_PanicsOnIncompatible(t *testing.T) {
	auditor := &FailFastAuditor{}

	result := &VerificationResult{
		Compatible:     false,
		ServerChecksum: "abc123",
		ClientChecksum: "def456",
		Deltas: []ContractDelta{
			{
				EntityType:  EntityStruct,
				EntityName:  "TestStruct",
				ChangeType:  ChangeRemoved,
				Direction:   DirectionClientHasMore,
				Severity:    SeverityError,
				Description: "Struct 'TestStruct' removed",
			},
		},
		Timestamp: time.Now(),
	}

	ctx := context.Background()

	defer func() {
		if r := (recover()); r == nil {
			t.Error("expected panic for incompatible contract")
		}
	}()

	auditor.Audit(ctx, result)
}

func TestFailFastAuditor_NoPanicOnWarningOnly(t *testing.T) {
	auditor := &FailFastAuditor{}

	result := &VerificationResult{
		Compatible:     true,
		ServerChecksum: "abc123",
		ClientChecksum: "def456",
		Deltas: []ContractDelta{
			{
				EntityType:  EntityEnum,
				EntityName:  "TestEnum",
				ChangeType:  ChangeAdded,
				Direction:   DirectionClientHasLess,
				Severity:    SeverityWarning,
				Description: "Enum value added",
			},
		},
		Timestamp: time.Now(),
	}

	ctx := context.Background()

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("unexpected panic for warning-level deltas: %v", r)
		}
	}()

	auditor.Audit(ctx, result)
}
