/**
 * R9: `Transport` is an abstract base — TypeScript must reject `new Transport()`.
 * Runtime tests cover the concrete HttpTransport/InProcTransport subclasses.
 */

const { strict: assert } = require("assert");
const { spawnSync } = require("child_process");
const { writeFileSync, mkdtempSync, rmSync } = require("fs");
const { tmpdir } = require("os");
const { join } = require("path");

const transportPath = require("path")
  .resolve(__dirname, "..", "transport.ts")
  .replace(/\\/g, "/");

function main() {
  const src = `import { Transport } from "${transportPath}";\nconst t = new Transport();\nvoid t;\n`;
  const dir = mkdtempSync(join(tmpdir(), "pulserpc-abstract-"));
  const file = join(dir, "probe.ts");
  try {
    writeFileSync(file, src, "utf-8");
    const npx = process.platform === "win32" ? "npx.cmd" : "npx";
    const result = spawnSync(
      npx,
      ["tsc", "--noEmit", "--target", "ES2020", "--module", "CommonJS", "--skipLibCheck", file],
      {
        encoding: "utf-8",
        shell: process.platform === "win32",
        // Run from the temp dir so no ambient tsconfig.json is picked up.
        cwd: dir,
      }
    );
    const output = `${result.stdout || ""}${result.stderr || ""}`;
    assert.notStrictEqual(result.status, 0, `tsc unexpectedly accepted new Transport(): ${output}`);
    assert.ok(
      output.includes("TS2511"),
      `expected TS2511 ('Cannot create an instance of an abstract class'), got: ${output}`
    );
    console.log("✓ testAbstractTransportRejectsDirectInstantiation");
    console.log("\nAll abstract transport tests passed!");
  } finally {
    rmSync(dir, { recursive: true, force: true });
  }
}

main();
