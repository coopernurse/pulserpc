import { defineConfig } from "tsup";

export default defineConfig({
  entry: ["pulserpc/index.ts"],
  format: ["esm", "cjs"],
  dts: false,
  splitting: false,
  sourcemap: false,
  clean: true,
  target: "es2020",
  outDir: "dist",
  outExtension({ format }) {
    return {
      js: format === "esm" ? ".mjs" : ".cjs",
    };
  },
});
