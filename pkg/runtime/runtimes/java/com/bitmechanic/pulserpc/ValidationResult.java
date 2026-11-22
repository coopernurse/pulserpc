package com.bitmechanic.pulserpc;

import java.util.List;
import java.util.ArrayList;
import java.util.Map;
import java.util.LinkedHashMap;

/**
 * Structured validation result, mirroring the Python/TypeScript ValidationResult.
 * Serializes to a Map suitable for JSON-RPC error.data:
 * {"valid": false, "error": "<joined>", "invalid_fields": [...]}
 * invalid_fields is omitted when empty.
 */
public class ValidationResult {
    public boolean valid;
    public String error;
    public List<String> invalidFields;

    public ValidationResult(boolean valid, String error, List<String> invalidFields) {
        this.valid = valid;
        this.error = error;
        this.invalidFields = invalidFields;
    }

    public static ValidationResult ok() {
        return new ValidationResult(true, null, null);
    }

    public static ValidationResult fail(List<ValidationError> errors) {
        StringBuilder sb = new StringBuilder();
        List<String> fields = new ArrayList<>();
        for (ValidationError e : errors) {
            if (sb.length() > 0) sb.append("; ");
            String p = (e.path == null || e.path.isEmpty()) ? "" : e.path;
            if (!p.isEmpty()) {
                sb.append(p).append(": ");
            }
            sb.append(e.message);
            if (e.path != null && !e.path.isEmpty()) {
                fields.add(e.path);
            }
        }
        String err = sb.length() == 0 ? "Validation failed" : sb.toString();
        return new ValidationResult(false, err, fields.isEmpty() ? null : fields);
    }

    public Map<String, Object> toMap() {
        Map<String, Object> m = new LinkedHashMap<>();
        m.put("valid", valid);
        m.put("error", error);
        if (invalidFields != null && !invalidFields.isEmpty()) {
            m.put("invalid_fields", invalidFields);
        }
        return m;
    }
}