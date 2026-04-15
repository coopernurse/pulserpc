## 1. Code Generator Fix

- [x] 1.1 Change `const req = {` to `const _req = {` at line 1032
- [x] 1.2 Change `const resp = await this.transport.request(req as any)` to `const _resp = await this.transport.request(_req as any)` at line 1047
- [x] 1.3 Update all `resp.` references to `_resp.` (lines 1048, 1049, 1054, 1058)
