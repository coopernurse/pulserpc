## 1. Parser Checksum Implementation

- [x] 1.1 Create `pkg/parser/checksum.go` with `ComputeChecksum(idl *IDL) (string, error)` function
- [x] 1.2 Implement canonical form builder that traverses IDL in normalized order
- [x] 1.3 Implement namespace resolution for type references (resolve bare names to FQN)
- [x] 1.4 Implement type serialization (built-in, array, map, user-defined) in canonical form
- [x] 1.5 Implement sorting for interfaces, methods, parameters, fields, enum values, errors

## 2. Unit Tests

- [x] 2.1 Add `pkg/parser/checksum_test.go` with basic tests
- [x] 2.2 Add test for order invariance (shuffle declarations)
- [x] 2.3 Add test for whitespace invariance (add/remove whitespace)
- [x] 2.4 Add test for parameter sort invariance
- [x] 2.5 Add test for field sort invariance
- [x] 2.6 Add test for error message exclusion (change message, same checksum)
- [x] 2.7 Add test for error code inclusion (change code, different checksum)
- [x] 2.8 Add regression test with known checksum for `book.pulse`
- [ ] 2.9 Add property-based tests using `github.com/flyingmutant/rapid` for:
  - Order invariance
  - Whitespace invariance
  - Type reference resolution equivalence

## 3. Generator Integration

- [x] 3.1 Update Go generator to compute and include checksum in idl.json
- [x] 3.2 Update Python generator to compute and include checksum in idl.json
- [x] 3.3 Update TypeScript generator to compute and include checksum in idl.json
- [x] 3.4 Update Java generator to compute and include checksum in idl.json
- [x] 3.5 Update C# generator to compute and include checksum in idl.json

## 4. Regenerate Examples

- [x] 4.1 Regenerate `examples/quickstart/python/idl.json` with checksum
- [x] 4.2 Verify all example idl.json files contain valid checksums
