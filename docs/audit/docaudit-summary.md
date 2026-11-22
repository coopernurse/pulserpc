# Documentation vs Implementation Audit — Summary

Companion to the per-runtime reports `docaudit-<runtime>.md`. Seven auditors
compared each runtime's implementation to the documentation that describes it
(`docs/languages/*/reference.md`, `quickstart.md`, `docs/RUNTIME_IMPLEMENTATION_GUIDE.md`,
`docs/idl-guide/*`, and the relevant `openspec/specs/*`).

**Recommendation convention:** `doc` = change the documentation to match the code;
`code` = change the code to match the documentation; `either` = a deliberate choice
is needed. The user's bias is that the code is correct, so most items are `doc`.

## Totals

| Runtime | High | Medium | Low | Total | Report |
|---|---:|---:|---:|---:|---|
| Go | 1 | 3 | 1 | 5 | [docaudit-go.md](docaudit-go.md) |
| Java | 0 | 2 | 2 | 4 | [docaudit-java.md](docaudit-java.md) |
| Python 3 | 1 | 4 | 2 | 7 | [docaudit-python.md](docaudit-python.md) |
| Python 2 | 0 | 0 | 5 | 5 | [docaudit-python2.md](docaudit-python2.md) |
| ts-node | 0 | 2 | 3 | 5 | [docaudit-ts-node.md](docaudit-ts-node.md) |
| ts-cjs | 8 | 9 | 9 | 26 | [docaudit-ts-cjs.md](docaudit-ts-cjs.md) |
| C# | 0 | 6 | 8 | 14 | [docaudit-csharp.md](docaudit-csharp.md) |
| **Total** | **10** | **26** | **30** | **66** | |

## Cross-cutting themes (fix once, helps several runtimes)

### 1. Structured validation `data` is now the norm — `docs/idl-guide/validation.md:245`
The doc still says *"`data` is a plain string in the Go, Java and C# runtimes;
TypeScript and Python expose a structured result."* After the M13 convergence all
runtimes emit `{"valid":false,"error":...,"invalid_fields":[...]}`.
**Recommendation: `doc`** — update `validation.md` to one shape for all runtimes.
Affects: Go (go-doc-01), Java (doc-java-01), C# (doc-csharp-03).

### 2. `int` is signed 32-bit, not 64-bit — `docs/idl-guide/types.md:15`
The table says `int` is "64-bit integers" (`int64`/`Long`). Every runtime enforces
`[-2147483648, 2147483647]` (D1/R25).
**Recommendation: `doc`** — change the table and any width prose.
Affects: Go (go-doc-02), Python2 (doc-python2-03), ts-cjs (tscjs-doc), ts-node.

### 3. Generated TypeScript/Python servers DO support batch requests
`docs/RUNTIME_IMPLEMENTATION_GUIDE.md:907-910` says only Go/C#/Java generated servers
support batches. The generated TS and Python adapters implement batching with the
100-entry cap.
**Recommendation: `doc`** — update the guide's batch table.
Affects: Python (python-doc-01, High), ts-node (doc-ts-node-01), ts-cjs.

### 4. Stale generated-API examples in `docs/languages/typescript/reference.md`
The examples describe an older generator: struct **classes** (generator emits
interfaces), `PulseRPCServer` (runtime is `Server` + abstract service classes),
synchronous methods (generated methods are async and take `ctx`), and
`HTTPTransport` (class is `HttpTransport`). `RPCException` is documented but only
`RPCError` exists; `idl.json` is on disk, not embedded.
**Recommendation: `doc`** — regenerate the examples from a current fixture.
Affects: ts-cjs (8 High deltas), ts-node, and partly C# (`RPCException`, PascalCase).
These are the highest user impact: copy-pasted examples do not compile.

### 5. `VerifyOnBootstrap()` is documented but not wired
Go and C# generated clients advertise `VerifyOnBootstrap()` in the quickstart/spec,
but Go does not generate it at all and C#'s is inert.
**Recommendation: mixed** — Go: `code` (implement it); C#: `code` (wire it, already
partially done) or document it as a no-op. Flagged below as a code-side disagreement.

### 6. Contract-verification spec drift
The specs document `new Client(transport, options)`, `IContractAuditor`, a
server-IDL fallback when no local `idl.json` exists, and a public constructor.
The implementations use `Client.create`, `ContractAuditor`, and throw when no local
IDL exists.
**Recommendation: `doc`** (specs are design artifacts; code is the shipped API),
except where the fallback is genuinely desirable.

## Code-side disagreements (where I would not blindly change the docs)

These are the cases where the auditor believes the **code** should change instead of
the docs:

1. **Go: `VerifyOnBootstrap()` is documented but not generated**
   (`pkg/generator/go_client_server.go:1140-1174,1317`). The option is advertised in
   the spec/quickstart. `code` — implement it for parity with Java/C#/TS rather than
   deleting the docs. *(Go report.)*
2. **ts-cjs: generated client stubs skip envelope validation** (`pkg/generator/ts_client_server.go:2029-2048`).
   They only check `_resp.error` truthiness, accept mismatched ids, and treat
   `error: null` as success. This violates the guide's client-envelope rule.
   `code` — generate clients that use the runtime `Client` envelope checks. *(ts-cjs
   report — strongest code-side finding.)*
3. **ts-cjs: `HttpTransport` rejects explicit `id: null` on 204** (`transport.ts:211-221`).
   The documented notification rule treats `id: null` as a notification. `code` —
   accept `204` when `req.id` is absent **or** null. *(ts-cjs.)*
4. **ts-cjs: envelope errors are plain `Error`, unlike ts-node's `RPCError(-32600)`**
   (`client.ts:267-284` vs `ts-node/client.ts:322-337`). `code` — align to `RPCError`
   with a stable code. *(ts-cjs.)*
5. **C#: generated client does not check the response `id`** and `VerifyOnBootstrap()`
   is inert (doc-csharp-04/05). `code` — wire the checks; the docs describe the
   intended contract. *(C# report.)*
6. **Go: `FindStruct`/`FindEnum` doc comments say ambiguous base names resolve to
   nil**, but `typeindex.go` returns the first sorted key. Internal comment/code
   mismatch; user-facing behavior matches the guide. `code` (comment fix).

## Accurate documentation (no action)

The auditors confirmed large areas already match: notification suppression, response
validation, id-type rules, batch caps, body/response limits, scalar acceptance
(including `NaN`/`Infinity` rejection), void returns, `meta` parsing, namespace
resolution, and transport timeouts. Python 2's documented carve-outs (no batch, no
contract verification, no generated stubs) are accurate.

## Recommended sequencing

1. **Single-edit cross-cutting fixes** (highest value per change):
   `validation.md` data shape, `types.md` int width, guide batch table.
2. **Regenerate the TypeScript reference examples** (fixes 8 High ts-cjs deltas and
   several ts-node/low items). This is the biggest user-facing win.
3. **C# and Go doc corrections** for `RPCException`/PascalCase/`VerifyOnBootstrap`.
4. **Decide the code-side items** (Go `VerifyOnBootstrap`, ts-cjs envelope/`id:null`/
   `RPCError`) — these are behavior changes, review separately.
5. **Python 2 / Java low-severity wording** cleanup.

Every per-runtime report lists the exact `file:line` evidence and a concrete edit, so
items 1–3 can be applied directly without re-auditing.
