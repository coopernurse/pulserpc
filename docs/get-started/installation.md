---
title: Installation
parent: Get Started
nav_order: 1
layout: default
---

# Installing PulseRPC

PulseRPC can be installed in several ways depending on your workflow.

## Installation Methods

### Method 1: Download Pre-built Binary

Download the latest release from the [GitHub Releases page](https://github.com/coopernurse/pulserpc/releases).

```bash
# Example for Linux AMD64
wget https://github.com/coopernurse/pulserpc/releases/latest/download/pulserpc-linux-amd64 -O pulserpc
chmod +x pulserpc
mv pulserpc /usr/local/bin/
```

### Method 2: Docker

Pull the latest Docker image:

```bash
docker pull ghcr.io/coopernurse/pulserpc:latest
```

Run PulseRPC via Docker:

```bash
docker run --rm -v $(pwd):/work ghcr.io/coopernurse/pulserpc:latest -h
```

### Method 3: Build from Source

Clone and build:

```bash
git clone https://github.com/coopernurse/pulserpc.git
cd pulserpc
make build
```

The binary will be created at `./target/pulserpc`.

## Verify Installation

```bash
pulserpc -h
```

You should see usage output that documents the supported command line flags. For a complete reference of all flags and usage examples, see the [CLI Reference](cli-reference.html).

## Troubleshooting

> **Need Help?** If you encounter issues not covered here, please [open an issue on GitHub](https://github.com/coopernurse/pulserpc/issues).

### Command not found

If you get `pulserpc: command not found`:

1. Check your PATH: `echo $PATH`
2. Add Go bin to PATH (if using Go install):
   ```bash
   echo 'export PATH=$PATH:$(go env GOPATH)/bin' >> ~/.bashrc
   source ~/.bashrc
   ```

### Wrong Go version

PulseRPC requires Go 1.21 or later. Check your version:

```bash
go version
```

### Permission denied

> **Common Issue**: This usually happens when downloading pre-built binaries. Make sure to set executable permissions.

If you get permission errors, make the binary executable:

```bash
chmod +x pulserpc
```

### macOS "developer cannot be verified"

On macOS, downloaded binaries may be blocked by Gatekeeper with the error "developer cannot be verified". To allow the binary to run:

```bash
xattr -cr /path/to/pulserpc-darwin-amd64
# Or right-click the file and select "Open Anyway" in System Settings
```

## Next Steps

- [Quickstart Overview](quickstart-overview.html) - Learn what you'll build
- [IDL Syntax](../idl-guide/syntax.html) - IDL language reference
- [Language Quickstarts](../languages/go/quickstart.html) - Jump to your language
