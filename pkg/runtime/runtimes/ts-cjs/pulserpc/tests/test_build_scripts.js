/**
 * Build-script / source-hygiene checks (CommonJS variant).
 *
 * - ts-cjs-10: `make clean` must not delete checked-in .js test sources, and
 *   both test targets must list every test file.
 * - ts-cjs-11: the CJS integration harness must not swallow tsc errors.
 * - ts-cjs-17: no raw severity string comparisons remain in the runtime.
 */

const { strict: assert } = require("assert");
const { readFileSync, readdirSync } = require("fs");
const { resolve } = require("path");

const runtimeRoot = resolve(__dirname, "..", ".."); // .../runtimes/ts-cjs
const repoRoot = resolve(runtimeRoot, "..", "..", "..", "..");
const makefile = readFileSync(resolve(runtimeRoot, "Makefile"), "utf-8");
const integrationScript = readFileSync(
  resolve(repoRoot, "tests", "integration", "test_generator_ts.sh"),
  "utf-8"
);

function testCleanDoesNotDeleteJsSources() {
  const cleanBody = makefile.split(/^clean:/m)[1] || "";
  assert.ok(cleanBody.length > 0, "Makefile must define a clean target");
  assert.ok(
    !/name\s+["']\*\.js["']/.test(cleanBody),
    "clean target must not delete *.js sources"
  );
  assert.ok(cleanBody.includes('*.js.map'), "clean target must still remove *.js.map");
  assert.ok(cleanBody.includes('node_modules'), "clean target must still remove node_modules");
  console.log("✓ testCleanDoesNotDeleteJsSources");
}

function testAllTestFilesRegistered() {
  const testFiles = readdirSync(resolve(__dirname)).filter(
    (f) => f.endsWith(".js") || f.endsWith(".ts")
  );
  assert.ok(testFiles.length >= 13, `expected all test files to exist, found ${testFiles.length}`);

  const testTarget = makefile.split(/^test: install/m)[1].split(/^test-docker:/m)[0];
  const dockerTarget = makefile.split(/^test-docker:/m)[1].split(/^test-integration:/m)[0];
  for (const f of testFiles) {
    assert.ok(
      testTarget.includes(`tests/${f}`),
      `local test target must run tests/${f}`
    );
    assert.ok(
      dockerTarget.includes(`tests/${f}`),
      `test-docker target must run tests/${f}`
    );
  }
  console.log("✓ testAllTestFilesRegistered");
}

function testTscErrorsNotSwallowed() {
  assert.ok(
    integrationScript.includes('npx tsc --project tsconfig.json > tsc.out 2>&1'),
    "integration harness must capture tsc diagnostics"
  );
  const tscLines = integrationScript
    .split("\n")
    .filter((line) => line.includes("npx tsc --project tsconfig.json"));
  assert.ok(tscLines.length > 0, "integration harness must invoke tsc");
  for (const line of tscLines) {
    assert.ok(!line.includes("|| true"), `tsc invocation must not swallow errors: ${line}`);
  }
  assert.ok(
    integrationScript.includes("error TS(2749|2503)"),
    "integration harness must document the explicit type-only diagnostic allowlist"
  );
  console.log("✓ testTscErrorsNotSwallowed");
}

function testNoRawSeverityComparisons() {
  const sourceFiles = ["client.ts", "contract.ts", "server.ts", "diff.ts", "transport.ts", "validation.ts"];
  for (const file of sourceFiles) {
    const body = readFileSync(resolve(runtimeRoot, "pulserpc", file), "utf-8");
    assert.ok(
      !/severity\s*===?\s*["']/.test(body),
      `${file} must compare severity with the Severity constant`
    );
  }
  console.log("✓ testNoRawSeverityComparisons");
}

function main() {
  testCleanDoesNotDeleteJsSources();
  testAllTestFilesRegistered();
  testTscErrorsNotSwallowed();
  testNoRawSeverityComparisons();
  console.log("\nAll build-script tests passed!");
}

main();
