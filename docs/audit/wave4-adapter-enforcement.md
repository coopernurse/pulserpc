# Wave 4 — Generated HTTP Adapter Enforcement of Body & Batch Caps

Scope: determine where generated HTTP adapters in `pkg/generator/` enforce request-body
and batch-size limits (left UNVERIFIED by the S3 reports). Distinguish a *declared
constant* from an *actual enforcing check* (with call site). Caps that exist only in a
runtime transport already surveyed are noted as such, not reported as new.

| runtime | inbound body cap (constant + actual check, file:line) | batch cap (constant + actual check, file:line) | generated adapter file | status |
|---|---|---|---|---|
