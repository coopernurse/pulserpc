## 1. Add sorting to generateTypesTs

- [x] 1.1 Import `sort` package in `ts_client_server.go`
- [x] 1.2 Sort enums by name before iteration in `generateTypesTs`
- [x] 1.3 Sort structs by name before iteration in `generateTypesTs`

## 2. Add sorting to generateTypesTsForNamespace

- [x] 2.1 Sort enums by name before iteration in `generateTypesTsForNamespace`
- [x] 2.2 Sort structs by name before iteration in `generateTypesTsForNamespace`

## 3. Add sorting to generateServerTs and writeInterfaceStubTs

- [x] 3.1 Sort interfaces by name before iteration in `generateServerTs`
- [x] 3.2 Sort methods by name before iteration in `writeInterfaceStubTs`

## 4. Add sorting to generateServerTsForNamespace and writeInterfaceStubTsForNamespace

- [x] 4.1 Sort interfaces by name before iteration in `generateServerTsForNamespace`
- [x] 4.2 Sort methods by name before iteration in `writeInterfaceStubTsForNamespace`

## 5. Add sorting to generateClientTs

- [x] 5.1 Sort interfaces by name before iteration in `generateClientTs`

## 6. Add sorting to generateClientTsForNamespace

- [x] 6.1 Sort interfaces by name before iteration in `generateClientTsForNamespace`

## 7. Verification

- [x] 7.1 Run existing tests to ensure no regression
- [x] 7.2 Verify generated TypeScript output is deterministic across regenerations