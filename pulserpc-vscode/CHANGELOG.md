# Changelog

All notable changes to the PulseRPC Language Support extension will be documented in this file.

## [0.1.0] - 2025-01-XX

### Added
- Initial release of Pulse IDL syntax highlighting for VS Code
- Support for `.pulse` file extension
- Syntax highlighting for:
  - Keywords: `namespace`, `interface`, `struct`, `enum`, `extends`, `import`
  - Built-in types: `string`, `int`, `float`, `bool`
  - Array types: `[]Type`, `[][]Type`
  - Map types: `map[string]Type`
  - Qualified names: `Namespace.TypeName`
  - Optional modifier: `[optional]`
  - Single-line comments: `// comment`
  - String literals
- Auto-closing pairs: `{}`, `[]`, `()`, `""`
