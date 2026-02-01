# Plan: Automatic Quickstart Guide Testing

## Summary

Enable automatic testing of all 5 language quickstart guides (Go, Python, Java, TypeScript, C#) by extracting code examples into testable source files while using Jekyll includes to maintain documentation as the single source of truth.

## Current Problem

- Quickstart guides contain embedded code examples that must be manually tested
- Time-consuming to verify each guide works after code changes
- No automated way to detect when generator changes break quickstart examples

## Proposed Solution

Extract quickstart code into runnable source files in `examples/quickstart/` and update documentation to use Jekyll includes that reference these files. Create automated test scripts that verify each quickstart works end-to-end.

---

## Directory Structure

```
examples/quickstart/
├── checkout.pulse              # Shared IDL (identical across all 5 languages)
├── README.md                   # Overview and how to run
├── go/
│   ├── server.go              # Server implementation (lines 195-343 from docs)
│   ├── client.go              # Client implementation (lines 368-409 from docs)
│   └── Makefile               # Build/run commands
├── python/
│   ├── server.py              # Server implementation (lines 163-306 from docs)
│   └── client.py              # Client implementation (lines 320-392 from docs)
├── java/
│   ├── MyServer.java          # Server implementation (lines 155-286 from docs)
│   ├── MyClient.java          # Client implementation (lines 299-344 from docs)
│   └── pom.xml                # Maven configuration
├── typescript/
│   ├── my_server.ts           # Server implementation (lines 156-272 from docs)
│   ├── my_client.ts           # Client implementation (lines 328-364 from docs)
│   ├── package.json           # npm configuration
│   └── tsconfig.json          # TypeScript config
└── csharp/
    ├── Shared/                # Generated code goes here
    ├── TestServer/
    │   ├── TestServer.csproj  # Project file (lines 162-183 from docs)
    │   └── MyServer.cs        # Server implementation (lines 188-318 from docs)
    └── TestClient/
        ├── TestClient.csproj  # Project file (lines 331-348 from docs)
        └── MyClient.cs        # Client implementation (lines 353-405 from docs)

docs/_includes/quickstart/
├── checkout.idl               # checkout.pulse with IDL syntax highlighting
├── go-server.md               # server.go with Go syntax highlighting
├── go-client.md
├── python-server.md           # server.py with Python syntax highlighting
├── python-client.md
├── java-server.md
├── java-client.md
├── ts-server.md
├── ts-client.md
├── csharp-server.md
└── csharp-client.md

tests/integration/
├── test_quickstart_all.sh     # Orchestrator - runs all language tests
├── test_quickstart_go.sh      # Go-specific test
├── test_quickstart_python.sh
├── test_quickstart_java.sh
├── test_quickstart_ts.sh
└── test_quickstart_csharp.sh
```

---

## Implementation Steps

### Phase 1: Extract Quickstart Code

**Files to create:**

1. `examples/quickstart/checkout.pulse`
   - Extract from any quickstart guide (IDL is identical across all languages)
   - Source: `docs/languages/go/quickstart.md` lines 32-147

2. For each language, extract server and client code:
   - **Go**: `server.go` (lines 195-343), `client.go` (lines 368-409)
   - **Python**: `server.py` (lines 163-306), `client.py` (lines 320-392)
   - **Java**: `MyServer.java` (lines 155-286), `MyClient.java` (lines 299-344)
   - **TypeScript**: `my_server.ts` (lines 156-272), `my_client.ts` (lines 328-364)
   - **C#**: `MyServer.cs` (lines 188-318), `MyClient.cs` (lines 353-405)

3. Create language-specific project files:
   - **Go**: `Makefile` with `run-server`, `run-client`, `test` targets
   - **Java**: `pom.xml` with Maven configuration
   - **TypeScript**: `package.json`, `tsconfig.json`
   - **C#**: `TestServer.csproj`, `TestClient.csproj`, `Shared/` directory structure

### Phase 2: Create Jekyll Includes

For each source file, create a corresponding include file in `docs/_includes/quickstart/`:

**Example include file format:**
```markdown
{% highlight go %}
package main

// ... content from examples/quickstart/go/server.go
{% endhighlight %}
```

**Update quickstart markdown files:**
Replace code blocks with include directives. Example for `docs/languages/go/quickstart.md`:

Before:
```markdown
```go
package main
...
```
```

After:
```markdown
{% include quickstart/go-server.md %}
```

### Phase 3: Create Test Scripts

**Test script pattern** (example: `tests/integration/test_quickstart_go.sh`):

```bash
#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
OUTPUT_DIR="/tmp/pulserpc_quickstart_go_$$"
SERVER_PORT=8100  # Different from integration tests (9000-9004)

cleanup() {
    kill $SERVER_PID 2>/dev/null || true
    rm -rf "$OUTPUT_DIR"
}
trap cleanup EXIT

# 1. Generate code from shared checkout.pulse
mkdir -p "$OUTPUT_DIR"
cd "$OUTPUT_DIR"
go mod init quickstart-test
"$PROJECT_ROOT/target/pulserpc" -plugin go-client-server -dir pkg/checkout \
    "$PROJECT_ROOT/examples/quickstart/checkout.pulse"

# 2. Copy quickstart implementations
mkdir -p cmd/server cmd/client
cp "$PROJECT_ROOT/examples/quickstart/go/server.go" cmd/server/main.go
cp "$PROJECT_ROOT/examples/quickstart/go/client.go" cmd/client/main.go

# 3. Start server
go mod tidy
go run -tags server_only cmd/server/main.go > server.log 2>&1 &
SERVER_PID=$!

# 4. Wait for server ready
for i in {1..30}; do
    if curl -s http://localhost:$SERVER_PORT > /dev/null 2>&1; then
        break
    fi
    sleep 1
done

# 5. Run client and verify output
CLIENT_OUTPUT=$(go run -tags client_only cmd/client/main.go)
echo "$CLIENT_OUTPUT" | grep -q "Products ===" || exit 1
echo "$CLIENT_OUTPUT" | grep -q "Wireless Mouse" || exit 1
echo "$CLIENT_OUTPUT" | grep -q "Order created" || exit 1

echo "✓ Go quickstart test passed!"
```

**Port allocation:**
- Go: 8100
- Python: 8101
- Java: 8102
- TypeScript: 8103
- C#: 8104

**Orchestrator script** (`tests/integration/test_quickstart_all.sh`):
```bash
#!/bin/bash
set -e

LANGUAGES=("go" "python" "java" "typescript" "csharp")
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

for lang in "${LANGUAGES[@]}"; do
    echo "=== Testing $lang Quickstart ==="
    bash "$SCRIPT_DIR/test_quickstart_${lang}.sh" || {
        echo "✗ $lang quickstart test FAILED"
        exit 1
    }
done

echo "✓ All quickstart tests passed!"
```

### Phase 4: Update Build System

**Add to root Makefile:**
```makefile
.PHONY: test-quickstarts
test-quickstarts:
	@echo "Testing all quickstart guides..."
	@bash tests/integration/test_quickstart_all.sh

.PHONY: test-quickstart-go
test-quickstart-go:
	@bash tests/integration/test_quickstart_go.sh

# Similar targets for python, java, typescript, csharp
```

### Phase 5: Documentation

**Create `examples/quickstart/README.md`:**
```markdown
# PulseRPC Quickstart Examples

This directory contains runnable quickstart examples for each supported language.

## Running Examples

### Go
\`\`\`bash
cd go
make test
\`\`\`

### Python
\`\`\`bash
cd python
python3 server.py &
python3 client.py
\`\`\`

### Java
\`\`\`bash
cd java
mvn compile exec:java -Dexec.mainClass="MyServer" &
mvn compile exec:java -Dexec.mainClass="MyClient"
\`\`\`

### TypeScript
\`\`\`bash
cd typescript
npm run build &
node dist/my_server.js &
node dist/my_client.js
\`\`\`

### C#
\`\`\`bash
cd csharp/TestServer
dotnet run &
cd ../TestClient
dotnet run
\`\`\`

## Automated Testing

Run all quickstart tests:
\`\`\`bash
make test-quickstarts
\`\`\`

Run individual language test:
\`\`\`bash
make test-quickstart-go
\`\`\`
```

**Update `CLAUDE.md`:**
Add section about quickstart testing:
```markdown
## Quickstart Testing

The quickstart guides are automatically tested to ensure they work correctly.

- Quickstart source code: `examples/quickstart/`
- Test scripts: `tests/integration/test_quickstart_*.sh`
- Run all: `make test-quickstarts`
- Run single: `make test-quickstart-{lang}`

When updating quickstart documentation:
1. Edit both the markdown guide and the source file in `examples/quickstart/`
2. Run `make test-quickstarts` to verify
3. Commit both changes together
```

---

## Critical Files Reference

### Source Documentation (code to extract)
- `/workspace/docs/languages/go/quickstart.md`
- `/workspace/docs/languages/python/quickstart.md`
- `/workspace/docs/languages/java/quickstart.md`
- `/workspace/docs/languages/typescript/quickstart.md`
- `/workspace/docs/languages/csharp/quickstart.md`

### Test Patterns to Follow
- `/workspace/tests/integration/test_generator_go.sh`
- `/workspace/tests/integration/test_http_api.sh`
- `/workspace/scripts/test-servers.sh`

### Build Files to Update
- `/workspace/Makefile` - Add test-quickstarts targets

---

## Verification

After implementation, verify:

1. **Manual testing**: Each quickstart can be run from `examples/quickstart/{lang}/`
2. **Automated testing**: `make test-quickstarts` passes for all languages
3. **Documentation**: Jekyll site builds correctly with includes
4. **No duplication**: Code exists once in `examples/quickstart/`, referenced via includes in docs

---

## Estimated Effort

- Phase 1 (Extract code): 1-2 hours
- Phase 2 (Jekyll includes): 1 hour
- Phase 3 (Test scripts): 2-3 hours
- Phase 4 (Build system): 30 minutes
- Phase 5 (Documentation): 30 minutes

**Total**: 5-7 hours

---

## Future Enhancements (Optional)

1. **Sync script**: Create `scripts/extract_quickstart_code.sh` to auto-extract code from markdown
2. **Docker tests**: Add `test_quickstart_docker.sh` using Docker images from `scripts/test-servers.sh`
3. **CI integration**: Add to GitHub Actions workflow
