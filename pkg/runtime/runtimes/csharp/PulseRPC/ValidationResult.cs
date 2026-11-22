using System.Collections.Generic;

namespace PulseRPC
{
    /// <summary>
    /// Structured validation result mirroring Python/Java/TypeScript.
    /// </summary>
    public class ValidationResult
    {
        public bool Valid { get; set; }
        public string? Error { get; set; }
        public List<string>? InvalidFields { get; set; }

        public static ValidationResult Ok() => new ValidationResult { Valid = true };

        public static ValidationResult Fail(List<ValidationError> errors)
        {
            var sb = new System.Text.StringBuilder();
            var fields = new List<string>();
            foreach (var e in errors)
            {
                if (sb.Length > 0) sb.Append("; ");
                if (!string.IsNullOrEmpty(e.Path))
                {
                    sb.Append(e.Path).Append(": ").Append(e.Message);
                    fields.Add(e.Path);
                }
                else
                {
                    sb.Append(e.Message);
                }
            }
            return new ValidationResult
            {
                Valid = false,
                Error = sb.Length == 0 ? "Validation failed" : sb.ToString(),
                InvalidFields = fields.Count == 0 ? null : fields
            };
        }

        public Dictionary<string, object?> ToDictionary()
        {
            var d = new Dictionary<string, object?>();
            d["valid"] = Valid;
            d["error"] = Error;
            if (InvalidFields != null && InvalidFields.Count > 0)
            {
                d["invalid_fields"] = InvalidFields;
            }
            return d;
        }
    }
}
