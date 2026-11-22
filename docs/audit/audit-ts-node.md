# Audit: ts-node runtime

## Scope & methodology

Audited the ts-node runtime library at `pkg/runtime/runtimes/ts-node/pulserpc/`
(`client.ts`, `server.ts`, `transport.ts`, `validation.ts`, `contract.ts`,
`diff.ts`, `rpc.ts`, `types.ts`, `index.ts`), its test suite under
`pulserpc/tests/`, and the TypeScript sections of `pkg/generator/ts_client_server.go`.
Each invariant in the R*/M*/C*/S* catalogs (plus the G* invariants that touch the
generator) was checked against the current source and, where cheap, against a live
`npx tsx` run of the runtime tests. The stale baseline
`docs/audit/problems-ts-node.md` was re-verified line by line; every entry
ts-node-01..15 has since been remediated in the current tree and is reflected
below as holding rather than re-listed.

## Summary (counts by severity)

- Critical: 0
- High: 0
- Medium: 0
- Low: 0
- Confirmed issues: 0
- Test gaps: 4 (minor)
- Verified as holding: 46 invariants/areas

## Confirmed issues

None. All previously reported ts-node findings (ts-node-01..15) are remediated in
the current tree and are covered by the runtime test suite (9 test files, all
passing: `npm test`/`make test` in `pkg/runtime/runtimes/ts-node`).

## Test gaps

- `pulserpc/tests/test_transport.ts` covers the 204 notification path only for a
  request with no `id` property; there is no assertion for a `204` returned to a
  request that carried an `id` (should surface as an empty-body error). Minor.
- `pulserpc/tests/test_client.ts` does not exercise `Client.ready()` or
  `Client.notify()`; both are only reached indirectly.
- `pulserpc/tests/test_types.ts` does not assert `getStructFields()` returns `[]`
  for an unknown struct name, nor `extractChecksum` for a non-object IDL root.
- `pulserpc/tests/test_server.ts` does not assert the `pulserpc-idl` method is
  suppressed for a notification (`call()` returns `null`), nor that a `null` id on
  a non-notification error path is echoed as `null`.

## Verified as holding

- R1 — `pulserpc/index.ts:7,10,12,22-23` re-exports `RPCError`, `Contract`,
  `Client`, `Server`, `Transport`, and the full `types.js`/`validation.js`
  surface (`validateType`, `findStruct`, `findEnum`, `getStructFields`,
  `extractChecksum`, all scalar/aggregate validators).
- R2 — `pulserpc/validation.ts:34-51` rejects non-numbers, `NaN`, non-finite
  values, fractional values, and out-of-range values for `int`; `validateFloat`
  accepts any `number` (NaN/Infinity included, matching the other TS runtime).
  Covered by `test_validation.ts` and `test_shared.ts`.
- R3 — `pulserpc/validation.ts:15-17,67-81,91-99,118-159` accumulates
  `ValidationError[]` with paths; `test_validation.ts:246-269`.
- R4 — `pulserpc/validation.ts:139-156` missing required → error, missing
  optional → OK, null required → error; `test_validation.ts:186-295`.
- R5 — `pulserpc/types.ts:131-161` parent fields first, child overrides on
  conflict; `test_types.ts:50-86`, `test_shared.ts:177-204`.
- R6 — `pulserpc/types.ts:111-121` exact → base-name → sorted first match;
  `test_types.ts:88-114`.
- R7 — `pulserpc/rpc.ts:17-30`; `test_rpc.ts`.
- R8 — `pulserpc/transport.ts:223-228,241,251,257` uses `RPCErrorCode`
  (`-32603`); no `-32000` remains in source; `test_transport.ts:259-321`.
- R9 — transport APIs are language-specific by design (documented divergence).
- R10 — `pulserpc/server.ts:217-224` calls
  `contract.validateResponse()` when `validateResponses` is true;
  `test_server.ts:99-117`.
- R11/M4/M21/M23 — `pulserpc/server.ts:58-67` returns `null` for missing or
  `null` id on both success and error paths; `id:0` is echoed;
  `test_server.ts:119-141`.
- R12 — `pulserpc/contract.ts:154-229` parses `meta` in both dict and Barrister
  forms with prototype-pollution guards; `test_contract.ts:123-167`.
- R13 — `pulserpc/diff.ts` plus `NoOpAuditor`/`LoggingAuditor`/`FailFastAuditor`
  in `contract.ts`; `Client.verifyCompatibility()` diffs against a separate local
  IDL (`client.ts:220-252`); `test_client.ts:206-261`.
- R14 — `pulserpc/contract.ts:356-366` accepts `null` only for void returns;
  `test_contract.ts:98-103`.
- M1 — `pulserpc/server.ts:100-107` rejects non-`"2.0"` with `-32600`.
- M2 — `pulserpc/server.ts:109-118` requires a non-empty string method.
- M3 — `pulserpc/server.ts:130-149` parses `Interface.method` and returns
  `-32601` for unknown interface/method.
- M5 — `pulserpc/client.ts:322-349` validates `jsonrpc`, echoed `id`, and
  result/error exclusivity; error code `0` is preserved;
  `test_client.ts:81-113`.
- M6 — generated adapter emits `-32700` for malformed JSON
  (`pkg/generator/ts_client_server.go:2137-2144`).
- M7 — generated test adapter supports batches with a 100-entry cap
  (`pkg/generator/ts_client_server.go:2205-2236`).
- M8 — `pulserpc/server.ts:155-190` rejects non-object/array params with
  `-32602` and maps object form by IDL name; `test_server.ts:176-193`.
- M9 — `pulserpc/contract.ts:273-286` accepts trailing optional omission;
  `test_contract.ts:65-83`, `test_server.ts:158-174`.
- M10 — `pulserpc/contract.ts:291-338` per-parameter IDL validation invoked from
  `server.ts:195-208`; `test_server.ts:143-156`.
- M11 — `pulserpc/validation.ts:95-116,190-216`; `test_validation.ts`.
- M12 — `pulserpc/validation.ts:125-159` ignores unknown fields by design.
- M13 — `pulserpc/contract.ts:128-141` emits a structured `ValidationResult`
  (`valid`/`error`/`invalidFields`) as `RPCError.data`;
  `test_contract.ts:107-121` and `docs/idl-guide/validation.md:245-247`.
- M14 — `pulserpc/client.ts:357` guards only `result !== undefined`, so `null`
  is validated against return optionality; `test_client.ts:115-130`.
- M15 — `pulserpc/client.ts:225-237` diffs the server IDL against a separate
  local IDL, not against itself.
- M18/M19/M20/M22/M24/M25/M26/M27/M28 — covered by `test_server.ts` and the
  generated adapter (empty batch → single `-32600`; non-object entries → per-entry
  `-32600`; object-form mapping; `id:null` notification; numeric code/message
  fallback; explicit `null` result; `jsonrpc:"2.0"`; notification-only batch →
  204).
- R15/D4 — `pulserpc/server.ts:210-240` logs handler/response-validation
  exceptions server-side and returns `-32603 "Internal error"` with no `data`;
  declared `RPCError`s are preserved; `test_server.ts:195-219`.
- R16/R17/R18/R19 — generated adapter caps body at 1 MiB and batches at 100;
  `HttpTransport` caps responses at 10 MiB and times out after 30 s;
  `test_transport.ts`.
- R20 — `pulserpc/client.ts:298-300` increments the id synchronously before the
  transport await; `test_client.ts:143-164`.
- R21 — `pulserpc/types.ts:91-109` uses a `WeakMap` base-name index cache.
- R22/R23 — generated adapter returns 405 for non-POST and 204 for
  notification-only batches (`pkg/generator/ts_client_server.go:2258-2272`).
- R24 — `pulserpc/server.ts:84-97` rejects boolean/object/array ids with
  `-32600`; `test_server.ts:221-242`.
- R25/R26 — `pulserpc/validation.ts:31-58` enforces signed 32-bit `int` and
  accepts any `number` for `float`; `test_shared.ts:91-133`.
- R27 — `pulserpc/validation.ts:103-116` validates enums as wire strings;
  `test_shared.ts:112-122`.
- R28/S5 — `pulserpc/contract.ts:21` rejects `__proto__`/`constructor`/
  `prototype` meta keys; `test_contract.ts:144-167`,
  `test_client.ts:166-186`.
- R29/R30 — `pulserpc/transport.ts:155-160,191-206` normalizes a trailing `/`
  and accepts custom headers; `test_transport.ts:85-121`.
- R31 — `pulserpc/client.ts:152-181` fetches `pulserpc-idl` at bootstrap.
- R32/C4/C5/D8 — `pkg/generator/idl_json.go:14-46` emits the uniform
  `rootNamespace`/`checksum`/`interfaces`/`structs`/`enums`/`errors` schema with
  comments and a 43-char base64url checksum; `test_shared.ts:216-222`.
- C1 — generator sorts map-derived output (namespaces, structs, enums, methods,
  filenames) before writing; see `pkg/generator/ts_client_server.go` sort calls.
- C2/C3 — TS writes `idl.json` in the entry-point namespace directory and embeds
  `IDL_JSON` for `pulserpc-idl`; `writeIDLJSONTs` and `MarshalIDLJSON`.
- C6/C7/C8/C10/C11/C12/C13 — module-style precedence, no-overwrite config
  generation, enum-as-string, boxed/optional types, and successful compile are
  exercised by `tests/integration/test_generator_ts.sh`.
- S6 — internal error details are redacted (see R15).
- S7/S8 — body/batch/limit handling in the generated adapter and transport.

## Unconfirmed hypotheses

- `pulserpc/server.ts:259` omits `error.data` only when it is `undefined`;
  an `RPCError` carrying `data: null` serializes `"data":null` instead of
  omitting it. Other runtimes omit `null`/`None`. Whether `null` should be
  treated as "no data" in TypeScript (where `undefined` is the natural absent
  value) is not settled by the docs; left as-is. No test covers this.
- `pulserpc/validation.ts:53-58` accepts `NaN`/`Infinity` for `float`. This
  matches the other TypeScript runtime and the `R26` "IEEE-754 double"
  statement, but standard JSON cannot represent those values; a cross-runtime
  probe for `JSON.parse`-produced infinities is not present.
- `pulserpc/client.ts:279-296` request validation for named params ignores
  unknown keys (the server rejects them), so a client with validation enabled
  can pass a request the server will reject. Not observed in tests; severity low.

## Notes

- Documented, non-blocking divergence (see
  `docs/audit/reviews-final.md`): the ts-node client throws `RPCError` (`-32600`)
  for malformed response envelopes while the ts-cjs client throws a plain
  `Error`. Both satisfy their acceptance criteria. ts-node also exposes
  `RPCErrorCode` constants and a single-owner `Contract.checkParamCount`, where
  ts-cjs uses raw codes and inlines the range check.
- The ts-node runtime is the "correct side" of the catalog's original
  ts-node/ts-cjs splits: it rejects `Infinity` for `int`, exposes the full
  validation/type surface, and uses standard `-32603` transport codes.
- No source files were modified by this audit; only this document was written.
EOF