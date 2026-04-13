package pulserpc

import (
	"context"
	"log"
)

type ContractAuditor interface {
	Audit(ctx context.Context, result *VerificationResult)
	Name() string
}

type NoOpAuditor struct{}

func (a *NoOpAuditor) Audit(ctx context.Context, result *VerificationResult) {}

func (a *NoOpAuditor) Name() string {
	return "NoOp"
}

type LoggingAuditor struct{}

func (a *LoggingAuditor) Audit(ctx context.Context, result *VerificationResult) {
	if !result.Compatible {
		log.Printf("[ERROR] Contract incompatibility detected: %d deltas found", len(result.Deltas))
	}
	for _, delta := range result.Deltas {
		switch delta.Severity {
		case SeverityError:
			log.Printf("[ERROR] %s: %s", delta.EntityType, delta.Description)
		case SeverityWarning:
			log.Printf("[WARNING] %s: %s", delta.EntityType, delta.Description)
		case SeverityInfo:
			log.Printf("[INFO] %s: %s", delta.EntityType, delta.Description)
		}
	}
	if result.Compatible && len(result.Deltas) == 0 {
		log.Printf("[INFO] Contract compatibility verified: client and server IDLs are identical")
	}
}

func (a *LoggingAuditor) Name() string {
	return "Logging"
}

type FailFastAuditor struct{}

func (a *FailFastAuditor) Audit(ctx context.Context, result *VerificationResult) {
	if !result.Compatible {
		panic("Contract compatibility verification failed: " + formatDeltas(result.Deltas))
	}
}

func (a *FailFastAuditor) Name() string {
	return "FailFast"
}

func formatDeltas(deltas []ContractDelta) string {
	if len(deltas) == 0 {
		return "no deltas"
	}
	msg := ""
	for _, delta := range deltas {
		if delta.Severity == SeverityError {
			msg += string(delta.EntityType) + ": " + delta.Description + "; "
		}
	}
	return msg
}
