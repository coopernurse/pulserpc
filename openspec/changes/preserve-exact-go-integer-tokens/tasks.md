## Implementation

- [x] Share exact signed integer parsing across normalization, validation, and conversion.
- [x] Preserve optional numeric fields and float negative zero, including integer-form `-0` tokens.
- [x] Keep wide and whole-valued decimal/exponent numbers as `json.Number` for middleware.
- [x] Test bounds, decimal and exponent forms, and exactness against rational arithmetic.
- [x] Verify real generated HTTP dispatch, including optional integers and float fields.
- [x] Reconcile the `go-server-options` numeric-ids requirement with the exact number view.
