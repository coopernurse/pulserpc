## 1. Generator

- [x] 1.1 Register `-omit-empty` (default `false`) in `GoClientServer.RegisterFlags`.
- [x] 1.2 Thread the flag through `Generate` to `generateNamespaceGo` and `generateStructTypesGo`.
- [x] 1.3 Emit `,omitempty` for every struct field when enabled; keep optional-only behavior when disabled.
- [x] 1.4 Leave embedded parent fields unchanged.

## 2. Tests

- [x] 2.1 Add a generator test asserting default tags: optional fields have `omitempty`, required fields do not.
- [x] 2.2 Add a generator test asserting `-omit-empty=true` adds `omitempty` to required fields and preserves optional tags.
- [x] 2.3 Ensure generated modules still build in both modes.

## 3. Documentation

- [x] 3.1 Document `-omit-empty` and the required-field zero-value caveat in `docs/languages/go/reference.md`.
- [x] 3.2 Keep the OpenSpec change record in sync with the implementation.
