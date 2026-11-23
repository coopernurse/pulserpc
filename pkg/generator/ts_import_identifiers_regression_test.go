package generator

import (
	"encoding/json"
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestCrossNamespaceFixtureTypeScriptIdentifiers guards the imported-interface
// identifier fix for both ESM and CJS output. The committed fixture declares
// interface CommonOps in namespace "common" and imports it from namespace
// "app", so the parser hands the generator the qualified IDL name
// "common.CommonOps". That name must be reduced to its base name at every
// TypeScript identifier site, while JSON-RPC method strings and the runtime
// registration/proxy keys stay qualified.
func TestCrossNamespaceFixtureTypeScriptIdentifiers(t *testing.T) {
	idl := parseCrossNamespaceFixture(t)

	for _, style := range []string{"esm-node", "cjs"} {
		t.Run(style, func(t *testing.T) {
			outDir := t.TempDir()
			gen := NewTSClientServer()
			fs := flag.NewFlagSet("ts", flag.ContinueOnError)
			fs.String("dir", outDir, "")
			fs.Bool("generate-test-files", true, "")
			gen.RegisterFlags(fs)
			for name, value := range map[string]string{
				"dir":                 outDir,
				"ts-module":           style,
				"ts-gen-package-json": "true",
				"ts-gen-tsconfig":     "true",
			} {
				if err := fs.Set(name, value); err != nil {
					t.Fatalf("set %s=%s: %v", name, value, err)
				}
			}
			if err := gen.Generate(idl, fs); err != nil {
				t.Fatalf("Generate (%s): %v", style, err)
			}

			commonServer := readCrossNSFile(t, filepath.Join(outDir, "common", "server.ts"))
			commonClient := readCrossNSFile(t, filepath.Join(outDir, "common", "client.ts"))
			appServer := readCrossNSFile(t, filepath.Join(outDir, "app", "server.ts"))
			appClient := readCrossNSFile(t, filepath.Join(outDir, "app", "client.ts"))
			testServer := readCrossNSFile(t, filepath.Join(outDir, "app", "test_server.ts"))
			testClient := readCrossNSFile(t, filepath.Join(outDir, "app", "test_client.ts"))

			// Identifier sites: abstract server class, client class, test
			// implementation class, and its import from the other namespace.
			abstractKeyword := "export abstract class"
			classKeyword := "export class"
			implImport := "import { CommonOps } from '../common/server.js'"
			if style == "cjs" {
				abstractKeyword = "abstract class"
				classKeyword = "class"
				implImport = "const { CommonOps } = require('../common/server')"
			}
			requireContains(t, "common/server.ts", commonServer,
				abstractKeyword+" CommonOps {")
			requireContains(t, "common/client.ts", commonClient,
				classKeyword+" CommonOpsClient {",
				// Wire method strings stay qualified.
				`method: "common.CommonOps.Ping"`,
				`method: "common.CommonOps.Echo"`,
			)
			requireContains(t, "app/server.ts", appServer,
				abstractKeyword+" AppOps {")
			requireContains(t, "app/client.ts", appClient,
				classKeyword+" AppOpsClient {")
			requireContains(t, "app/test_server.ts", testServer,
				implImport,
				"class CommonOpsImpl extends CommonOps {",
				// Handler registration key is the qualified IDL name; the
				// constructed class is a base-name identifier.
				`rpcServer.addHandler("common.CommonOps", new CommonOpsImpl());`,
			)
			requireContains(t, "app/test_client.ts", testClient,
				// The dynamic proxy is keyed by the qualified IDL name, so the
				// key stays qualified but is no longer a dotted identifier.
				`client["common.CommonOps"].Ping()`,
				`client["common.CommonOps"].Echo(`,
			)

			// No generated TypeScript file may contain a raw dotted identifier
			// derived from the qualified interface name.
			var tsFiles []string
			err := filepath.Walk(outDir, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}
				if !info.IsDir() && strings.HasSuffix(path, ".ts") {
					tsFiles = append(tsFiles, path)
				}
				return nil
			})
			if err != nil {
				t.Fatalf("walk generated tree: %v", err)
			}
			for _, bad := range []string{
				"class common.CommonOps",
				"class common.CommonOpsClient",
				"import { common.CommonOps",
				"const { common.CommonOps",
				"new common.CommonOps",
				"module.exports.common.CommonOps",
			} {
				for _, file := range tsFiles {
					if strings.Contains(readCrossNSFile(t, file), bad) {
						t.Errorf("%s still contains invalid qualified identifier %q", file, bad)
					}
				}
			}

			tsTypecheckGeneratedProject(t, outDir, style)
		})
	}
}

// tsTypecheckGeneratedProject runs tsc --noEmit over the generated project.
//
// The generated tsconfig.json targets TS 5 (moduleResolution Node10 for CJS,
// ignoreDeprecations "5.0"), which TS 6+ rejects outright. A version-neutral
// override config is therefore written beside it and used for the check. Node
// type definitions are picked up from the global npm install when present.
//
// CJS output has two known, pre-existing diagnostics from the ESM-to-CJS
// transform: TS2749 (a value imported via require used as a type) and TS2503
// (a namespace-style type reference after "import * as ns" became "const ns =
// require(...)"). The repo's own TS integration test tolerates exactly those
// two codes; every other diagnostic still fails this test. A missing tsc is
// reported as a precise skip, never as a pass.
func tsTypecheckGeneratedProject(t *testing.T, outDir, style string) {
	t.Helper()
	tsc, err := exec.LookPath("tsc")
	if err != nil {
		t.Skipf("tsc not on PATH; generated %s TypeScript typecheck UNVERIFIED", style)
	}

	compilerOpts := map[string]interface{}{
		"target":          "ES2020",
		"strict":          false,
		"esModuleInterop": true,
		"skipLibCheck":    true,
	}
	if style == "cjs" {
		compilerOpts["module"] = "Node16"
		compilerOpts["moduleResolution"] = "Node16"
		// CJS output is require()/module.exports scripts; force module mode
		// so same-named top-level bindings in different files do not collide.
		compilerOpts["moduleDetection"] = "force"
	} else {
		compilerOpts["module"] = "NodeNext"
		compilerOpts["moduleResolution"] = "NodeNext"
	}
	// Global npm installs keep @types beside the npx/tsc shim.
	typeRoots := filepath.Join(filepath.Dir(tsc), "node_modules", "@types")
	if _, err := os.Stat(typeRoots); err == nil {
		compilerOpts["typeRoots"] = []string{filepath.ToSlash(typeRoots)}
		compilerOpts["types"] = []string{"node"}
	}

	encodedOpts, err := json.MarshalIndent(compilerOpts, "  ", "  ")
	if err != nil {
		t.Fatalf("marshal tsc override options: %v", err)
	}
	config := "{\n  \"compilerOptions\": " + string(encodedOpts) + ",\n  \"include\": [\"**/*.ts\"],\n  \"exclude\": [\"node_modules\"]\n}\n"
	configPath := filepath.Join(outDir, "tsconfig.check.json")
	if err := os.WriteFile(configPath, []byte(config), 0644); err != nil {
		t.Fatalf("write tsc override config: %v", err)
	}

	cmd := exec.Command(tsc, "--noEmit", "--project", configPath)
	cmd.Dir = outDir
	out, err := cmd.CombinedOutput()
	unexpected := []string{}
	for _, line := range strings.Split(string(out), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.Contains(trimmed, "error TS2749:") || strings.Contains(trimmed, "error TS2503:") {
			continue
		}
		unexpected = append(unexpected, trimmed)
	}
	if len(unexpected) > 0 {
		t.Fatalf("generated %s TypeScript has unexpected diagnostics from %s:\n%s", style, tsc, strings.Join(unexpected, "\n"))
	}
	if err != nil && style != "cjs" {
		t.Fatalf("generated %s TypeScript typecheck failed with %s: %v\n%s", style, tsc, err, out)
	}
	if err != nil {
		t.Logf("generated %s TypeScript typechecked with %s; only known CJS-transform diagnostics (TS2749/TS2503) remain", style, tsc)
	}
}
