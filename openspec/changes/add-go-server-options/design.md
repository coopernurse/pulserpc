## Context

The generated Go server is a thin HTTP adapter over `pulserpc.Server`. The
adapter installs a fixed 1 MiB `http.MaxBytesReader`, parses the body with
`json.Unmarshal`, accepts batches, and forwards each request to
`Server.CallWithContext`. The runtime always accepts named and positional
params, treats an absent id as a notification, and has no policy switches.

Downstream applications need a per-server policy without hand-writing an
adapter. The runtime must also stop forcing numeric ids through `float64`: the
adapter is the only place that owns body decoding, so `UseNumber` there fixes
the precision loss for every handler.

## Goals / Non-Goals

**Goals:**

- One `ServerConfig` value that controls batching, id requirements, params form,
  body size, UTF-8, unknown fields, and error mapping.
- Preserve v0.4.0 behavior exactly when the default config is used, including
  the legacy oversize error.
- Keep envelope protocol errors (`-32600`) distinct from argument decode and
  validation failures that `MapError` may rewrite.
- Make numeric ids exact regardless of configuration.
- Keep the generated server API additive: `NewPulseRPCServer` still works.

**Non-Goals:**

- Changing other language runtimes.
- Introducing per-method policy overrides.
- Changing the JSON-RPC envelope, batch response shape, or HTTP status codes.
- Replacing the generated `errorResponse`/`sendErrorResponse` shape (HTTP 200).

## Decisions

### Policy lives on the runtime `Server`

`ServerConfig` is stored on `Server` and exposed through `Server.Config()`,
which returns a copy. `NewServer` delegates to
`NewServerWithConfig(..., DefaultServerConfig())`, so existing callers keep the
lenient behavior. The generated adapter reads `s.rpcServer.Config()` rather than
duplicating policy state, so programmatic `Call`/`CallWithContext` users and the
HTTP adapter share the dispatch policy (`RequireNonNullID`, `ParamsMode`,
`RejectUnknownRequestFields`, `MapError`). Batching and body limits are
HTTP-adapter concerns and are enforced there; `Call` handles one request at a
time and never reads `AllowBatch`.

**Alternative considered:** putting policy on the generated `PulseRPCServer`
only. Rejected: `Call`/`CallWithContext` would then enforce different rules from
the HTTP path, which is surprising and untestable end to end.

### The zero value fails closed on batching

`AllowBatch` is a `bool`, so its zero value is `false`. This makes
`ServerConfig{}` reject batches even though `NewServer` (which uses
`DefaultServerConfig()`) allows them. The asymmetry is deliberate: a config
built from scratch should not silently accept batches, but callers that do not
choose a config must keep the v0.4.0 behavior. Documented on the type and in the
reference.

**Alternative considered:** `DisallowBatch` to make the zero value lenient.
Rejected: the requested API names `AllowBatch`, and a positive option reads
better at call sites.

### RequireNonNullID bypasses notification suppression

An absent id is currently a notification and therefore receives no response,
even for errors. When `RequireNonNullID` is set, the runtime checks after the
nil-request check and again after middleware rewrites the request, and returns
`errorResponse(nil, ...)` directly instead of `respondError`. That writes the
`-32600` error with `"id": null` while the handler never runs.

### Params mode is an envelope decision

`ParamsModePositionalOnly` rejects object params with `-32600` in the params
switch, before named-param conversion. It is intentionally not routed through
`MapError`: the client used a disallowed request shape, not an invalid
parameter value. Missing or `null` params still normalize to an empty list, so a
zero-argument handler keeps working.

### Decode and validation failures carry a phase

An internal `paramError` wraps the underlying error with an `ErrorPhase`:

- `ErrorPhaseDecodeParams`: params shape, parameter count from named
  conversion, unknown named parameters, unknown struct fields.
- `ErrorPhaseValidateParams`: `Contract.ValidateRequest` failures.

A helper consults `MapError` first. A non-nil mapped error is returned as-is
(HTTP 200, no implicit data). A nil return falls back to the pre-change
behavior: `*RPCError` passthrough, otherwise `-32602 "Invalid params"` with the
error string as data. Envelope errors never use this path.

### Unknown-field detection mirrors conversion

The strict check runs in `invokeHandler` before converting each parameter and
walks the raw decoded value against the handler parameter type. It accepts the
same key candidates as `convertMapToStruct`: the `json` tag name, the Go field
name, the lower-first-letter form, and the all-lowercase form. The candidate
helper is shared with `convertMapToStruct` so the two checks cannot drift. It
skips `json:"-"`, unexported fields, and empty-interface targets; it does not
exempt `json.Unmarshaler` types because the runtime's converter populates these
structs by reflection rather than calling `UnmarshalJSON`. It recurses through
pointers, slices/arrays, maps, and embedded structs, adding promoted embedded
keys to the allowed set. The allowed-key schema is a pure function of the
struct type, so it is cached in a `sync.Map` rather than rebuilt for every
element of a slice or map. Unknown keys are sorted only when producing an error
(`<path>: unknown field "<key>"`), keeping the valid path allocation-free.

Because the check runs after contract validation, an unknown key that does not
affect validation reaches it; a missing required field still fails validation
first with the validation phase.

### Body limits and UTF-8 are adapter concerns

The generated adapter reads `config.MaxRequestBodyBytes`; `0` keeps the
existing `maxRequestBodySize` constant and the legacy `-32600 "Request body too
large"` response. A positive value uses that cap and reports oversize as
`-32700 "Parse error"`. `http.MaxBytesReader` is still installed before HTTP
middleware, so a hook cannot buffer an unbounded body.

`RejectInvalidUTF8` checks `utf8.Valid` before decoding. The default `false`
keeps `encoding/json`'s byte replacement behavior.

### Exact numeric ids

The adapter creates a `json.Decoder` over the body, calls `UseNumber()`, decodes
one value, then reads one token and requires `io.EOF` so trailing data is a
`-32700` without materializing a second value. Numeric ids are `json.Number`,
which the runtime accepts and serializes verbatim.

To avoid changing the v0.4.0 middleware/handler view, the adapter then calls
`pulserpc.NormalizeRequestNumbers`, which converts numbers inside `params` and
other request members to `float64` while leaving each request's top-level `id`
as `json.Number`. Batch arrays are walked member by member. Programmatic callers
may still pass `json.Number` params directly; runtime validation accepts
`json.Number` for `int` and `float`, and conversion special-cases it before the
generic `ConvertibleTo` path so it maps to the requested integer or float width
(and never to a string). Fractional and out-of-range ints are still rejected.

On the client side, the runtime and generated `HTTPTransport` decode responses
through `pulserpc.DecodeResponse`, which keeps the response `id` as
`json.Number` and normalizes every other number to `float64`. `numericID`/
`idsEqual` in `transport.go` compare ids exactly: integers are exact,
`json.Number` uses its decimal text, and `float64` uses its exact binary value.
A mixed `json.Number`/`float64` pair falls back to the legacy float64 comparison,
so fractional ids keep v0.4.0 behavior; `idsEqual(7, 7.0)` and string
comparison keep working.

## Risks / Trade-offs

- **[Zero-value surprise]** `ServerConfig{}` disables batching. Mitigation:
  document the requirement to start from `DefaultServerConfig()` on the type,
  the constructor, and the reference.
- **[Unknown-field recursion cost]** Strict mode walks each struct param. It
  runs only when enabled and rewards the common case with a single traversal.
- **[Error mapping can hide detail]** `MapError` replaces validation detail.
  Mitigation: phases let applications distinguish decode from validation, and
  a nil return restores the standard response.
- **[Custom body limit changes the oversize code]** A configured limit returns
  `-32700` instead of the legacy `-32600`. Mitigation: documented in the limits
  table; the default is unchanged.
- **[Generated API change]** The adapter's generated source changes. Mitigation:
  generator tests assert the new strings and build a generated module.
