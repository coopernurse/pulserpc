package pulserpc

// ValidationError carries a field path and a human-readable message, mirroring
// ValidationResult is the structured shape emitted as a JSON-RPC error's data
// field by accumulating validators, mirroring Python/TypeScript/Java.
type ValidationResult struct {
	Valid         bool     `json:"-"`
	Error         string   `json:"-"`
	InvalidFields []string `json:"-"`
}

// NewValidationResult builds a ValidationResult from a list of errors.
func NewValidationResult(errors []ValidationError) ValidationResult {
	if len(errors) == 0 {
		return ValidationResult{Valid: true}
	}
	parts := make([]string, 0, len(errors))
	fields := make([]string, 0, len(errors))
	for _, e := range errors {
		if e.Path != "" {
			parts = append(parts, e.Path+": "+e.Message)
			fields = append(fields, e.Path)
		} else {
			parts = append(parts, e.Message)
		}
	}
	vr := ValidationResult{Valid: false, Error: joinStrings(parts, "; ")}
	if len(fields) > 0 {
		vr.InvalidFields = fields
	}
	return vr
}

func joinStrings(parts []string, sep string) string {
	if len(parts) == 0 {
		return ""
	}
	out := parts[0]
	for i := 1; i < len(parts); i++ {
		out += sep + parts[i]
	}
	return out
}

// ToMap returns the JSON-serializable map shape: {"valid":..., "error":...,
// "invalid_fields":[...]} (invalid_fields omitted when empty).
func (vr ValidationResult) ToMap() map[string]interface{} {
	m := map[string]interface{}{
		"valid": vr.Valid,
		"error": vr.Error,
	}
	if len(vr.InvalidFields) > 0 {
		m["invalid_fields"] = vr.InvalidFields
	}
	return m
}
