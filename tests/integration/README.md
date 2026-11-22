# PulseRPC Generator Integration Testing

This directory contains integration tests that verify generator plugins can produce working client and server code that interoperate correctly.

## Overview

The integration test framework:

1. Generates test server and client code from a test IDL
2. Starts the test server in a Docker container
3. Runs the test client against the server
4. Reports pass/fail results

This ensures that:
- Generated code compiles/runs correctly
- Client and server can communicate via JSON-RPC 2.0
- Type validation works end-to-end
- All IDL features are properly supported

## Test IDL: `conform.pulse`

The test uses `examples/conform.pulse` which is designed to exercise all IDL features:

### Built-in Types
- `string` - Used in various methods
- `int` - Used in `add()`, `repeat_num()`, etc.
- `float` - Used in `calc()`, `sqrt()`
- `bool` - Used in `RepeatRequest.force_uppercase`

### Complex Types
- **Arrays**: `[]string`, `[]int`, `[]float` - Tested in `repeat()`, `repeat_num()`, `calc()`
- **Maps**: Not currently in conform.pulse, but supported by the framework

### User-Defined Types
- **Structs**: `RepeatRequest`, `RepeatResponse`, `HiResponse`, `Person`
- **Struct Inheritance**: `RepeatResponse extends inc.Response`
- **Enums**: `inc.Status`, `inc.MathOp` (namespaced enums)

### Optional Fields
- `Person.email` is marked `[optional]` - tested with `null` value in `putPerson()`

### Optional Returns
- `B.echo()` returns `string [optional]` - tested to return `null` when input is `"return-null"`

### Multiple Interfaces
- Interface `A` with 7 methods
- Interface `B` with 1 method
- Tests that server dispatcher correctly routes to the right interface

### Namespaces
- `inc.Status`, `inc.MathOp`, `inc.Response` - Tests qualified type names

## Test Files Generated

When running with `-test-server` flag, the generator creates:

### `test_server.{ext}`

Concrete implementations of all interface stubs. For Python, this includes:

- `AImpl` - Implements all methods of interface `A`
  - `add(a, b)` - Returns `a + b`
  - `calc(nums, operation)` - Performs operation on array
  - `sqrt(a)` - Returns square root
  - `repeat(req1)` - Echoes string as list
  - `say_hi()` - Returns `{"hi": "hi"}`
  - `repeat_num(num, count)` - Returns array of repeated values
  - `putPerson(p)` - Returns `p.personId`

- `BImpl` - Implements interface `B`
  - `echo(s)` - Returns `s`, or `None` if `s == "return-null"`

### `test_client.{ext}`

Test program that exercises all client methods:

- Creates HTTP transport and client instances
- Waits for server to be ready
- Calls each method with test parameters
- Validates responses match expected values
- Reports pass/fail for each test
- Exits with code 0 on success, 1 on failure

## Running Tests

### Prerequisites

- Docker installed and running
- PulseRPC binary built (`make build`)

### Run Python Generator Tests

From the project root:

```bash
make test-generator-python
```

Or from `pkg/runtime/runtimes/python/`:

```bash
make test-integration
```

### Run All Generator Tests

```bash
make test-generators
```

### Run the shared HTTP protocol harness

`scripts/test-servers.sh` starts all seven runtimes (python, python2, ts-node,
ts-cjs, csharp, java, go). `test_all_runtimes.sh` runs both
`test_http_api.sh` (JSON-RPC envelope and message-validation probes) and
`test_limits.sh` (1 MiB request body and 100-entry batch) against each:

```bash
make test-all-runtimes
```

The limit probes can also target a single server:

```bash
make test-limits SERVER_URL=http://localhost:9000
```

The remaining cross-cutting scripts are reachable through
`make test-integration-scripts` (numeric types, enum case, raises propagation,
static TS client, Java compile, and error declarations).

### Manual Execution

You can also run the test harness script directly:

```bash
bash tests/integration/test_generator.sh
```

Or in Docker:

```bash
docker run --rm \
    -v $(pwd):/workspace \
    -w /workspace \
    python:3.11-slim \
    /bin/bash -c "apt-get update -qq && apt-get install -y -qq curl >/dev/null 2>&1 && bash tests/integration/test_generator.sh"
```

## Test Harness Script

The `test_generator.sh` script:

1. **Builds pulserpc binary** (if needed)
2. **Generates code** with `-test-server` flag
3. **Starts test server** in background
4. **Waits for server** to be ready (polls with timeout)
5. **Runs test client** and captures results
6. **Cleans up** server process and temp files

The script uses:
- Temporary directory: `/tmp/pulserpc_test_$$`
- Server port: `8080`
- Timeout: `30` seconds

## Adding Tests for New Languages

When implementing a new language runtime:

1. **Add test generation** to the generator plugin:
   - Check for `-test-server` flag
   - Generate `test_server.{ext}` with concrete implementations
   - Generate `test_client.{ext}` with test cases

2. **Update Makefiles**:
   - Add `test-integration` target to `pkg/runtime/runtimes/{lang}/Makefile`
   - Add `test-generator-{lang}` target to root `Makefile`
   - Update `test-generators` target

3. **Test the implementation**:
   ```bash
   make test-generator-{lang}
   ```

## Troubleshooting

### Server doesn't start

- Check that the language runtime is available in Docker image
- Verify generated code compiles/runs
- Check server logs in temporary directory

### Tests fail

- Review test client output for specific failures
- Check that test server implementations match IDL comments
- Verify type validation is working correctly

### Timeout errors

- Increase timeout in `test_generator.sh`
- Check that server is binding to `0.0.0.0` (not `localhost`)
- Verify port 8080 is available

## Future Enhancements

- Add more test IDLs for specific scenarios
- Support batch request testing
- Support notification testing
- Performance benchmarking
- Cross-language interoperability tests (Python client → Java server, etc.)

