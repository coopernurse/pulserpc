# Wave 4: Dependabot `@eslint/js` 10.0.1 CI Failure Audit

- PR: #13 — `chore(deps-dev): bump @eslint/js from 9.39.2 to 10.0.1 in /pkg/webui`
- Branch: `dependabot/npm_and_yarn/pkg/webui/eslint/js-10.0.1`
- Failing runs: CI `45093731427`, CI Conformance `45093731482`

## Root cause

The PR bumps only `@eslint/js` to 10.0.1. That major release declares a peer
dependency on `eslint@^10.0.0`, but `pkg/webui/package.json` still pins
`eslint@^9.39.2`. `npm ci` in the **"Build web UI assets"** step therefore fails
with `ERESOLVE` before any linting even runs. This is a dependency-resolution
conflict caused directly by the `@eslint/js` major bump; it is not a flaky test
or an unrelated config/plugin incompatibility at runtime.

## Evidence

From `gh run view 45093731427 --log-failed`, job `Go unit + harness tests`,
step `Build web UI assets`:

```
npm error code ERESOLVE
npm error ERESOLVE could not resolve
npm error While resolving: @eslint/js@10.0.1
npm error Found: eslint@9.39.2
npm error   dev eslint@"^9.39.2" from the root project
npm error Could not resolve dependency:
npm error peerOptional eslint@"^10.0.0" from @eslint/js@10.0.1
npm error Conflicting peer dependency: eslint@10.15.0
npm error Fix the upstream dependency conflict, or retry this command with
npm error --force or --legacy-peer-deps
make: *** [Makefile:17: build-webui] Error 1
##[error]Process completed with exit code 2.
```

The `CI Conformance` run `45093731482` fails on the same dependency resolution.

## Recommendation

**(a) Close the PR as an incompatible major.**

`@eslint/js` 10 belongs to the ESLint 10 line and cannot resolve against the
pinned ESLint 9 toolchain; a single-package Dependabot bump cannot land on its
own. Adopting v10 (option b) would require a coordinated migration — upgrading
`eslint` to 10 plus auditing the flat config, plugins and rule sets for v10
breaking changes — which is out of scope for this automated PR. Using
`--legacy-peer-deps` to force it through would only mask a broken dependency
graph. Rebase (option c) will not help because the conflict is version-based,
not stale-state based.
