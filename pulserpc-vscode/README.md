# PulseRPC Language Support

Syntax highlighting for Pulse IDL files used by the PulseRPC project.

## Features

- Syntax highlighting for `.pulse` IDL files
- Support for keywords: `namespace`, `interface`, `struct`, `enum`, `extends`, `import`
- Highlighting for built-in types: `string`, `int`, `float`, `bool`
- Support for complex types:
  - Arrays: `[]Type`, `[][]Type`
  - Maps: `map[string]Type`
  - Qualified names: `Namespace.TypeName`
- Optional field modifier: `[optional]`
- Single-line comments: `// comment`

## Installation

### From VSIX Package

1. Download the latest `.vsix` file from the [Releases](https://github.com/pulserpc/pulserpc/releases) page
2. Open VS Code
3. Go to Extensions (Ctrl+Shift+X)
4. Click the "..." menu in the Extensions view
5. Select "Install from VSIX..."
6. Choose the downloaded `.vsix` file

### From Source

```bash
# Install dependencies
npm install

# Build the extension
npm run build

# Package the extension
vsce package

# Install the extension
code --install-extension pulserpc-*.vsix
```

## Development

```bash
# Install dependencies
npm install

# Open the project in VS Code
code .

# Press F5 to launch a new VS Code window with the extension loaded
```

## Testing

Open any `.pulse` file in VS Code to see the syntax highlighting in action. You can also use the "Developer: Inspect Editor Tokens and Scopes" command to verify the token scopes.

## License

MIT

## See Also

- [PulseRPC Documentation](https://github.com/pulserpc/pulserpc)
- [Pulse IDL Specification](https://github.com/pulserpc/pulserpc/blob/main/docs/idl-spec.md)
