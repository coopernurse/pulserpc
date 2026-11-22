# Developing and Testing on Windows

PulseRPC's test suite compiles and runs native test binaries, generated
servers, and per-runtime toolchains. On Windows 11 with **Smart App Control
(SAC)** or a managed **WDAC** policy, those unsigned binaries are blocked by
Code Integrity before they run. This guide covers how to diagnose that and how
to run the suite reliably.

## 1. Diagnose the block

```powershell
# 0 = off, 1 = enforce, 2 = evaluation
(Get-ItemProperty 'HKLM:\SYSTEM\CurrentControlSet\Control\CI\Policy' `
  -Name VerifiedAndReputablePolicyState).VerifiedAndReputablePolicyState

Get-CimInstance -Namespace root\Microsoft\Windows\DeviceGuard `
  -ClassName Win32_DeviceGuard |
  Select-Object CodeIntegrityPolicyEnforcementStatus,
                UsermodeCodeIntegrityPolicyEnforcementStatus

Get-ChildItem 'C:\Windows\System32\CodeIntegrity\CiPolicies\Active'

Get-WinEvent -LogName 'Microsoft-Windows-CodeIntegrity/Operational' -MaxEvents 30 |
  Where-Object Id -in 3077,3078,3033 |
  Select-Object TimeCreated,Id,Message
```

Typical blocked-file events look like:

```
Code Integrity determined that a process (\...\go.exe) attempted to load
\...\parser.test.exe that did not meet the Enterprise signing level
requirements ... Policy ID:{0283ac0f-fff1-49ae-ada1-8a933130cad6}
```

`{0283AC0F-FFF1-49AE-ADA1-8A933130CAD6}` is the Smart App Control policy.
SAC has **no per-app allowlist**; it evaluates unsigned binaries by Microsoft
cloud reputation. Defender exclusions, self-signing, and running as
Administrator do **not** bypass it.

## 2. Recommended: run tests in WSL2

SAC governs Windows PE binaries, not Linux ELF binaries. WSL2 is the fastest
native-feeling path and keeps SAC enabled.

### Install Ubuntu and toolchains

```powershell
wsl --install -d Ubuntu
wsl -d Ubuntu
```

Inside Ubuntu (no `sudo` required for Go/Node):

```bash
# Go (match go.mod; this repo uses 1.25.4)
mkdir -p ~/tools
curl -fsSL https://go.dev/dl/go1.25.4.linux-amd64.tar.gz -o /tmp/go.tgz
tar -C ~/tools -xzf /tmp/go.tgz
echo 'export PATH=$HOME/tools/go/bin:$PATH' >> ~/.bashrc

# Node 22 + tsx (needed by pkg/generator's TS compile test)
ver=$(curl -fsSL https://nodejs.org/dist/index.json |
  python3 -c 'import json,sys; print([v["version"] for v in json.load(sys.stdin) if v["version"].startswith("v22.")][0])')
curl -fsSL "https://nodejs.org/dist/$ver/node-$ver-linux-x64.tar.xz" -o /tmp/node.tar.xz
tar -C "$HOME/tools" -xJf /tmp/node.tar.xz
mv "$HOME/tools/node-$ver-linux-x64" "$HOME/tools/node"
echo 'export PATH=$HOME/tools/node/bin:$HOME/.npm-global/bin:$PATH' >> ~/.bashrc
npm install -g tsx typescript @types/node --prefix "$HOME/.npm-global" --no-audit --no-fund
```

Optional toolchains (only if you want to run those runtimes in WSL):

```bash
# .NET 8
curl -fsSL https://dot.net/v1/dotnet-install.sh -o /tmp/dotnet-install.sh
bash /tmp/dotnet-install.sh --channel 8.0 --install-dir "$HOME/.dotnet"
echo 'export PATH=$HOME/.dotnet:$PATH' >> ~/.bashrc

# JDK 17 (Temurin tarball; adjust version as needed)
mkdir -p ~/tools
curl -fsSL "https://github.com/adoptium/temurin17-binaries/releases/latest/download/OpenJDK17U-jdk_x64_linux_hotspot.tar.gz" -o /tmp/jdk.tgz
tar -C ~/tools -xzf /tmp/jdk.tgz
echo 'export JAVA_HOME=$(echo $HOME/tools/jdk-17*)' >> ~/.bashrc
```

### Run the suites

From WSL (`/mnt/c` works; clone into `~/src` if you want faster I/O):

```bash
export PATH="$HOME/tools/go/bin:$HOME/tools/node/bin:$HOME/.npm-global/bin:$PATH"
export GOTOOLCHAIN=local GOTMPDIR=/tmp
cd /mnt/c/Users/james/src/pulserpc

# Go unit + integration (this is fully green under WSL)
go test ./pkg/... ./tests/integration/... -count=1

# Java
(cd pkg/runtime/runtimes/java && mvn -q test)

# Python 3
python3 -m pytest pkg/runtime/runtimes/python/tests \
  pkg/runtime/runtimes/python/pulserpc/tests -q

# TypeScript
(cd pkg/runtime/runtimes/ts-node && npx tsx pulserpc/tests/test_types.ts)  # and the rest
(cd pkg/runtime/runtimes/ts-cjs  && npx tsx pulserpc/tests/test_types.js)   # and the rest

# C# (after installing .NET 8 in WSL)
(cd pkg/runtime/runtimes/csharp && dotnet test tests/PulseRPC.Tests.csproj --nologo)
```

The repo's Docker-based generator suites also work from WSL if Docker Desktop
WSL integration is enabled:

```bash
make test-generators     # generates + compiles + runs Go/Java/C#/TS/Python
make test-all-runtimes   # HTTP suite across the matrix
```

`make` is not preinstalled in a minimal Ubuntu; install it with
`sudo apt update && sudo apt install -y make curl` if you want the Makefile
targets.

## 3. Docker fallback

Docker Desktop is already the least invasive option for C#, Java, and Python 2.
Run a worktree directly in a container:

```powershell
# Go
docker run --rm -v "C:\Users\james\src\pulserpc:/workspace" -w /workspace `
  golang:1.25 go test ./pkg/... ./tests/integration/... -count=1

# Java
docker run --rm -v "C:\Users\james\src\pulserpc:/workspace" `
  -w /workspace/pkg/runtime/runtimes/java `
  maven:3.9-eclipse-temurin-17 mvn -q test

# C#
docker run --rm -v "C:\Users\james\src\pulserpc:/workspace" `
  -w /workspace/pkg/runtime/runtimes/csharp `
  mcr.microsoft.com/dotnet/sdk:8.0 dotnet test tests/PulseRPC.Tests.csproj --nologo

# Python 2
docker run --rm -v "C:\Users\james\src\pulserpc:/workspace" `
  -w /workspace/pkg/runtime/runtimes/python2 `
  python:2.7-slim sh -c "PYTHONPATH=/workspace/pkg/runtime/runtimes/python2 python tests/run_tests.py"
```

For generated e2e, use the repo's existing Docker harnesses
(`tests/integration/test_generator_*.sh`, `make test-generators`) rather than
launching generated `.exe` files on the host.

## 4. What still works natively

Signed host executables are allowed:

- **Java**: `java.exe` / `mvn.cmd` (the JVM host is signed).
- **Node.js / TypeScript**: `node.exe` / `npx.cmd`.
- **Python**: `python.exe` / `pytest`.
- **Go build-only checks**: `go build`, `go vet` (they do not execute the
  unsigned test binary).
- **C# build-only checks**: `dotnet build` (does not launch `testhost.exe` or
  the apphost).

Blocked natively:

- **`go test`** — runs an unsigned `*.test.exe`.
- **`dotnet test`** — runs an unsigned `testhost.exe`.
- **`dotnet run`** and generated C# apphosts — the apphost `.exe` is unsigned.
- **Generated Go servers** — compiled native `.exe`.

### C# apphost workaround

If you must run generated C# natively, set `<UseAppHost>false</UseAppHost>` in
the generated test `.csproj` and run:

```powershell
dotnet exec bin\Debug\net8.0\TestServer.dll
```

This avoids the unsigned apphost. `dotnet test` still launches `testhost.exe`
and is usually blocked; use WSL/Docker for tests.

## 5. Turning Smart App Control off

Only if you accept the trade-off:

1. **Windows Security → App & browser control → Smart App Control settings**
2. Choose **Off**.

**Warning:** Microsoft documents this as irreversible without resetting or
reinstalling Windows. It also removes SAC's protection. Prefer WSL2/Docker.

If the setting is greyed out, the device is managed (Intune/GPO/WDAC). Ask IT
for either a WDAC supplemental policy that allows unsigned developer binaries
in a specific directory (for example `C:\Users\james\src`) or a developer VM.

## 6. Troubleshooting

- **Go temp directory blocked**: set `GOTMPDIR` outside the blocked path, but
  note that generated `.exe` files may still be blocked. WSL is the reliable
  fix.
- **`go build`/`go vet` pass but `go test` fails with an Application Control
  error**: expected; run the tests in WSL/Docker.
- **`mvn test` passes but generated Java e2e fails**: the generated server runs
  under the signed JVM, so this is usually a real code/test failure, not SAC.
- **`dotnet build` passes but `dotnet test` fails**: testhost is unsigned;
  WSL/Docker.
- **Docker bind mount is empty or slow**: ensure the drive is shared in Docker
  Desktop and prefer a path under `C:\Users\<you>\src`.

## 7. Related docs

- [Invariant remediation plans](../plans/README.md)
- [Runtime implementation guide](../RUNTIME_IMPLEMENTATION_GUIDE.md)
- [Integration coverage assessment](../audit/integration-coverage.md)
