package generator

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/coopernurse/pulserpc/pkg/parser"
	"github.com/coopernurse/pulserpc/pkg/runtime"
)

// stderrWriter is the destination for module-style resolution warnings.
// It is a package-level variable so tests can capture and assert on the
// emitted warnings by swapping in a buffer.
var stderrWriter io.Writer = os.Stderr

// tsWalkUpMaxDepth caps how far resolveEffectiveModuleStyle walks up the
// directory tree looking for tsconfig.json and package.json. The plan's
// invariants require that the walk never goes past 10 ancestors and never
// outside the outputDir's ancestors.
const tsWalkUpMaxDepth = 10

// TSClientServer is a plugin that generates TypeScript HTTP server and client code from IDL.
//
// Three module styles are supported (see the -ts-module flag and the
// resolveModuleStyle helper):
//   - "esm-node" (default) — Node-flavored ESM with explicit ".js" import
//     suffixes and a runtime tree at pkg/runtime/runtimes/ts-node/.
//   - "esm-bundler" — ESM without ".js" import suffixes, suitable for
//     Vite/webpack/Next; uses the same runtime tree as esm-node, with the
//     generator applying a post-process transform to strip the suffix.
//   - "cjs" — CommonJS, using pkg/runtime/runtimes/ts-cjs/ and a transform
//     that converts ESM imports/exports to require/module.exports.
type TSClientServer struct {
	packageBase    string
	moduleStyle    string
	noDetect       bool
	packageJSONType string
	tsconfigModule  string
}

// NewTSClientServer creates a new TSClientServer plugin instance
func NewTSClientServer() *TSClientServer {
	return &TSClientServer{}
}

// Name returns the plugin identifier
func (p *TSClientServer) Name() string {
	return "ts-client-server"
}

// RegisterFlags registers CLI flags for this plugin
func (p *TSClientServer) RegisterFlags(fs *flag.FlagSet) {
	if fs.Lookup("package") == nil {
		fs.String("package", "", "Base module path for generated imports (e.g., @myapp/lib/rpc). Creates single directory level under -dir.")
	}
	if fs.Lookup("ts-module") == nil {
		fs.String("ts-module", "", "Module style for generated TypeScript code: esm-node (default), esm-bundler, or cjs. Aliases: esm, node, bundler, commonjs. Precedence when unset: tsconfig.json module > package.json type > esm-node.")
	}
	if fs.Lookup("ts-gen-package-json") == nil {
		fs.Bool("ts-gen-package-json", false, "Generate a package.json at -dir matching the resolved module style (errors if one already exists).")
	}
	if fs.Lookup("ts-gen-tsconfig") == nil {
		fs.Bool("ts-gen-tsconfig", false, "Generate a tsconfig.json at -dir matching the resolved module style (errors if one already exists).")
	}
	if fs.Lookup("ts-no-detect") == nil {
		fs.Bool("ts-no-detect", false, "Disable auto-detection of module style from tsconfig.json/package.json. With this flag set, the default is always esm-node unless -ts-module is also set.")
	}
}

// tsImportPath returns the final import-path string for a relative target,
// appending the appropriate extension for the given module style. This is
// the single source of truth for the .js-suffix decision in generated
// TypeScript files.
//
//   - "esm-node" / "esm-bundler": returns "<target>.js". The bundler
//     post-process transform in step 7 strips the .js suffix, but the
//     generator still emits it here so default-style output is byte-equal
//     across both ESM variants.
//   - "cjs": returns "<target>" (CommonJS resolves extensions implicitly).
//   - any other value: treated as ESM (defensive default).
//
// Callers pass a bare relative path with no extension
// (e.g., "./pulserpc/rpc", "../common/types", "./server").
func tsImportPath(moduleStyle, target string) string {
	if moduleStyle == "cjs" {
		return target
	}
	return target + ".js"
}

// importPath is the method form of tsImportPath, used by call sites that
// already hold a *TSClientServer receiver.
func (p *TSClientServer) importPath(target string) string {
	return tsImportPath(p.moduleStyle, target)
}

// resolveModuleStyle normalizes a user-supplied -ts-module value to one of
// the canonical names: "esm-node", "esm-bundler", or "cjs". Recognized
// aliases: "" → "esm-node", "esm" → "esm-node", "node" → "esm-node",
// "bundler" → "esm-bundler", "commonjs" → "cjs". Unknown values return an
// error that lists the valid values and aliases.
func (p *TSClientServer) resolveModuleStyle(raw string) (string, error) {
	switch raw {
	case "", "esm-node", "node", "esm":
		return "esm-node", nil
	case "esm-bundler", "bundler":
		return "esm-bundler", nil
	case "cjs", "commonjs":
		return "cjs", nil
	default:
		return "", fmt.Errorf("invalid module style %q (valid: esm-node, esm-bundler, cjs; aliases: esm, node, bundler, commonjs)", raw)
	}
}

// tsconfigModuleToStyle maps a tsconfig.json compilerOptions.module value to
// the canonical module style. Returns "" if the value is not recognized
// (caller should fall through to package.json detection).
func tsconfigModuleToStyle(module string) string {
	switch module {
	case "Node16", "NodeNext", "ES2022", "ES2020", "ESNext":
		return "esm-node"
	case "Bundler", "Preserve":
		return "esm-bundler"
	case "CommonJS", "Node10":
		return "cjs"
	default:
		return ""
	}
}

// packageJSONTypeToStyle maps a package.json "type" field to the canonical
// module style. The empty string and "module" both map to esm-node; "commonjs"
// maps to cjs. Unknown values are rejected by the caller (treated as no
// detection result).
func packageJSONTypeToStyle(pkgType string) (string, bool) {
	switch pkgType {
	case "", "module":
		return "esm-node", true
	case "commonjs":
		return "cjs", true
	default:
		return "", false
	}
}

// walkUpDirs walks up from start, returning each directory from start
// up to (and including) the filesystem root, but no more than maxDepth
// entries. Returned paths are cleaned (filepath.Clean). The first entry is
// always start. The function does not follow symlinks; it just walks the
// lexical parent chain.
func walkUpDirs(start string, maxDepth int) []string {
	dirs := make([]string, 0, maxDepth+1)
	cur := start
	for i := 0; i <= maxDepth; i++ {
		dirs = append(dirs, filepath.Clean(cur))
		parent := filepath.Dir(cur)
		if parent == cur {
			break
		}
		cur = parent
	}
	return dirs
}

// readTsconfigModule walks up from outputDir looking for tsconfig.json. If
// found, it parses just the compilerOptions.module field and returns it.
// Returns "" (not an error) when no tsconfig.json is found or it has no
// module field. Errors are returned only for malformed JSON in a found file.
func readTsconfigModule(outputDir string) (string, string, error) {
	for _, dir := range walkUpDirs(outputDir, tsWalkUpMaxDepth) {
		path := filepath.Join(dir, "tsconfig.json")
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var doc struct {
			CompilerOptions struct {
				Module            string `json:"module"`
				ModuleResolution  string `json:"moduleResolution"`
			} `json:"compilerOptions"`
		}
		if err := json.Unmarshal(data, &doc); err != nil {
			return "", path, fmt.Errorf("failed to parse %s: %w", path, err)
		}
		// Prefer compilerOptions.moduleResolution="Bundler" hint, then
		// module. This catches the "module=ESNext with moduleResolution=Bundler"
		// pattern from the plan's mapping table.
		module := doc.CompilerOptions.Module
		if doc.CompilerOptions.ModuleResolution == "Bundler" {
			module = "Bundler"
		}
		return module, path, nil
	}
	return "", "", nil
}

// readPackageJSONType walks up from outputDir looking for package.json. If
// found, returns the "type" field. Returns "" when no package.json is found.
// Errors are returned only for malformed JSON in a found file.
func readPackageJSONType(outputDir string) (string, string, error) {
	for _, dir := range walkUpDirs(outputDir, tsWalkUpMaxDepth) {
		path := filepath.Join(dir, "package.json")
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var doc struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal(data, &doc); err != nil {
			return "", path, fmt.Errorf("failed to parse %s: %w", path, err)
		}
		return doc.Type, path, nil
	}
	return "", "", nil
}

// resolveEffectiveModuleStyle determines the effective module style for this
// generation run, following the precedence defined in step 5 of the plan:
//   1. Explicit -ts-module flag (canonicalized via resolveModuleStyle).
//   2. Detected tsconfig.json compilerOptions.module.
//   3. Detected package.json "type".
//   4. Default "esm-node".
//
// The -ts-no-detect flag disables steps 2 and 3 entirely.
//
// It emits a one-line "warning:" to stderrWriter when the explicit flag
// disagrees with a detected source, or when tsconfig.json and package.json
// disagree with each other. Warnings are advisory and do not return errors.
//
// The resolved canonical style is stored in p.moduleStyle. p.packageJSONType
// and p.tsconfigModule are populated for diagnostics.
func (p *TSClientServer) resolveEffectiveModuleStyle(fs *flag.FlagSet, outputDir string) error {
	// 1. Read explicit flag.
	var explicitRaw string
	if f := fs.Lookup("ts-module"); f != nil {
		explicitRaw = f.Value.String()
	}
	explicit, err := p.resolveModuleStyle(explicitRaw)
	if err != nil {
		return err
	}

	// Read the no-detect flag.
	p.noDetect = false
	if f := fs.Lookup("ts-no-detect"); f != nil && f.Value.String() == "true" {
		p.noDetect = true
	}

	// 2 & 3. Auto-detect (unless disabled).
	p.tsconfigModule = ""
	p.packageJSONType = ""
	var tsconfigStyle, packageStyle string
	var tsconfigPath, packagePath string

	if !p.noDetect && outputDir != "" {
		module, path, err := readTsconfigModule(outputDir)
		if err != nil {
			return err
		}
		if path != "" {
			p.tsconfigModule = module
			tsconfigPath = path
			tsconfigStyle = tsconfigModuleToStyle(module)
		}

		pkgType, path, err := readPackageJSONType(outputDir)
		if err != nil {
			return err
		}
		if path != "" {
			p.packageJSONType = pkgType
			packagePath = path
			if style, ok := packageJSONTypeToStyle(pkgType); ok {
				packageStyle = style
			}
		}
	}

	// 4. Pick the effective style.
	resolved := explicit
	type detected struct {
		source string // e.g. "tsconfig.json module"
		value  string // e.g. "commonjs", "NodeNext"
		path   string // filesystem path
	}
	var det *detected

	if explicitRaw != "" {
		// Explicitly set via flag — always wins, but warn if it disagrees.
		if tsconfigStyle != "" && tsconfigStyle != explicit {
			det = &detected{source: "tsconfig.json module", value: p.tsconfigModule, path: tsconfigPath}
		} else if packageStyle != "" && packageStyle != explicit {
			det = &detected{source: "package.json type", value: p.packageJSONType, path: packagePath}
		}
	} else {
		// No explicit flag — use highest-precedence detection, else default (esm-node).
		if tsconfigStyle != "" {
			resolved = tsconfigStyle
		} else if packageStyle != "" {
			resolved = packageStyle
		}
	}

	// Warn when tsconfig and package.json disagree with each other.
	if tsconfigStyle != "" && packageStyle != "" && tsconfigStyle != packageStyle {
		_, _ = fmt.Fprintf(stderrWriter,
			"warning: tsconfig.json module=%s disagrees with package.json type=%s; using tsconfig.json module=%s\n",
			p.tsconfigModule, p.packageJSONType, p.tsconfigModule)
	}

	// Warn when explicit disagrees with detection.
	if det != nil {
		_, _ = fmt.Fprintf(stderrWriter,
			"warning: -ts-module=%s overrides detected %s=%s at %s\n",
			explicit, det.source, det.value, det.path)
	}

	p.moduleStyle = resolved
	return nil
}


// transformFileForStyle rewrites a TypeScript source file's bytes so they
// match the given module style. See step 7 of the plan.
//
//   - "esm-node" (and empty/unknown): bytes are returned unchanged. The
//     generator already emits ".js" import suffixes; the runtime tree at
//     ts-node is already valid Node ESM.
//   - "esm-bundler": the .js suffix is stripped from every RELATIVE import
//     (./x.js, ../x.js). Absolute or package imports are left alone. The
//     runtime tree at ts-node is used and transformed in place; bundlers
//     resolve imports without the suffix.
//   - "cjs": full ESM-to-CJS rewrite. import/export statements are converted
//     to require()/module.exports, interfaces and types are dropped, and
//     class/function/enum definitions are tracked so the matching
//     module.exports.X = X line can be appended.
//
// Both runtime-copied files and user-generated code are passed through this
// function. For the runtime files in ts-cjs, the input is already CJS and
// the transform is effectively a no-op (no import/export lines remain to
// rewrite).
func (p *TSClientServer) transformFileForStyle(content []byte, moduleStyle string) []byte {
	switch moduleStyle {
	case "", "esm-node":
		return content
	case "esm-bundler":
		return transformEsmBundler(content)
	case "cjs":
		return transformCjs(content)
	default:
		return content
	}
}

// bundlerRelativeImportRE matches `from '<relative-path-with-.js>'` and
// captures the relative path (without the `.js` suffix). Relative paths are
// defined as starting with `./` or `../`.
var bundlerRelativeImportRE = regexp.MustCompile(`from\s+['"](\.{1,2}/[^'"]*?)\.js['"]`)

// transformEsmBundler strips `.js` suffixes from every relative import path.
// Other content is preserved verbatim.
func transformEsmBundler(content []byte) []byte {
	return bundlerRelativeImportRE.ReplaceAll(content, []byte("from '$1'"))
}

// cjs transform regular expressions. Each is anchored to the start of the
// trimmed line so we don't accidentally match the word `import` inside a
// string literal or a comment.
var (
	cjsImportNamedRE = regexp.MustCompile(`^import\s*\{\s*([^}]+?)\s*\}\s*from\s+['"]([^'"]+)['"]\s*;?\s*$`)
	cjsImportStarRE  = regexp.MustCompile(`^import\s+\*\s+as\s+(\w+)\s+from\s+['"]([^'"]+)['"]\s*;?\s*$`)
	cjsImportDefaultRE = regexp.MustCompile(`^import\s+(\w+)\s+from\s+['"]([^'"]+)['"]\s*;?\s*$`)
	cjsExportStarFromRE = regexp.MustCompile(`^export\s+\*\s+from\s+['"]([^'"]+)['"]\s*;?\s*$`)
	cjsExportNamedFromRE = regexp.MustCompile(`^export\s*\{\s*([^}]+?)\s*\}\s*from\s+['"]([^'"]+)['"]\s*;?\s*$`)
	cjsExportInterfaceRE = regexp.MustCompile(`^export\s+(abstract\s+)?interface\s+\w+`)
	cjsExportTypeRE = regexp.MustCompile(`^export\s+type\s+`)
	cjsExportAbstractClassRE = regexp.MustCompile(`^export\s+abstract\s+class\s+(\w+)`)
	cjsExportClassRE = regexp.MustCompile(`^export\s+class\s+(\w+)`)
	cjsExportEnumRE = regexp.MustCompile(`^export\s+enum\s+(\w+)`)
	cjsExportFunctionRE = regexp.MustCompile(`^export\s+(async\s+)?function\s+(\w+)`)
	cjsExportConstRE = regexp.MustCompile(`^export\s+const\s+(\w+)\s*=`)
	cjsExportNamedRE = regexp.MustCompile(`^export\s*\{\s*([^}]+?)\s*\}\s*;?\s*$`)
)

// transformCjs converts ESM TypeScript source to CommonJS-compatible
// TypeScript. The runtime still expects .ts files (compile with tsc), but
// the import/export statements become require/module.exports statements
// so the compiled output works as Node CJS.
//
// Line-by-line processing keeps the transform predictable: each line is
// classified by its leading token and rewritten. Multi-line constructs
// (class/function/enum bodies) are tracked via a per-pending-export
// brace counter so the trailing module.exports.X = X line lands at the
// matching close brace.
//
// `export interface` and `export type` declarations are left in place
// unchanged so that downstream `tsc --noEmit` can resolve type-only
// references (e.g., `import * as types from './types'; types.RepeatRequest`).
// Those declarations are erased at compile time by tsc itself.
func transformCjs(content []byte) []byte {
	var out bytes.Buffer
	lines := strings.Split(string(content), "\n")

	// Stack of pending exports whose module.exports.X = X assignment must
	// be emitted when the matching close brace is encountered. Each entry
	// carries its own brace depth so unrelated braces (e.g., a non-export
	// nested class) inside an export body don't accidentally close it.
	var pending []pendingCjsExport

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Classify and rewrite the line.
		switch {
		case cjsImportNamedRE.MatchString(trimmed), cjsImportStarRE.MatchString(trimmed), cjsImportDefaultRE.MatchString(trimmed):
			// Leave import statements unchanged. The runtime tree for the
			// CJS module style uses `export class` etc., so these imports
			// resolve cleanly via `tsc --module CommonJS --esModuleInterop`
			// (and via tsx at runtime). Converting them to `require()`
			// here would shadow the runtime's exported types with local
			// `const` bindings, which breaks `tsc --noEmit`.
			out.WriteString(line)
			out.WriteString("\n")
		case cjsExportStarFromRE.MatchString(trimmed):
			m := cjsExportStarFromRE.FindStringSubmatch(trimmed)
			requirePath := strings.TrimSuffix(m[1], ".js")
			fmt.Fprintf(&out, "module.exports = Object.assign(module.exports, require('%s'));\n", requirePath)
		case cjsExportNamedFromRE.MatchString(trimmed):
			m := cjsExportNamedFromRE.FindStringSubmatch(trimmed)
			requirePath := strings.TrimSuffix(m[2], ".js")
			for _, n := range strings.Split(m[1], ",") {
				n = strings.TrimSpace(n)
				if n == "" {
					continue
				}
				source, local := splitExportName(n)
				fmt.Fprintf(&out, "module.exports.%s = require('%s').%s;\n", local, requirePath, source)
			}
		case cjsExportInterfaceRE.MatchString(trimmed), cjsExportTypeRE.MatchString(trimmed):
			// Leave `export interface` / `export type` declarations in
			// place. They are type-only constructs that tsc erases at
			// compile time, and they must remain visible so that
			// downstream `tsc --noEmit` (and any sibling namespace
			// imports like `import * as types from './types'`) can
			// resolve type references. They fall through to the
			// default branch below.
			out.WriteString(line)
			out.WriteString("\n")
		case cjsExportAbstractClassRE.MatchString(trimmed), cjsExportClassRE.MatchString(trimmed):
			m := cjsExportAbstractClassRE.FindStringSubmatch(trimmed)
			if m == nil {
				m = cjsExportClassRE.FindStringSubmatch(trimmed)
			}
			// Strip the leading `export ` keyword and emit the plain
			// `class Foo { ... }` form, matching the CJS runtime's
			// convention (no top-level `export` in the body).
			pushPendingExport(&pending, m[1], stripExportPrefix(line), &out)
		case cjsExportEnumRE.MatchString(trimmed):
			m := cjsExportEnumRE.FindStringSubmatch(trimmed)
			pushPendingExport(&pending, m[1], stripExportPrefix(line), &out)
		case cjsExportFunctionRE.MatchString(trimmed):
			m := cjsExportFunctionRE.FindStringSubmatch(trimmed)
			pushPendingExport(&pending, m[2], stripExportPrefix(line), &out)
		case cjsExportConstRE.MatchString(trimmed):
			m := cjsExportConstRE.FindStringSubmatch(trimmed)
			// export const is a single statement — emit the line without
			// the leading `export` keyword, then append the
			// module.exports assignment immediately.
			out.WriteString(stripExportPrefix(line))
			out.WriteString("\n")
			fmt.Fprintf(&out, "module.exports.%s = %s;\n", m[1], m[1])
		case cjsExportNamedRE.MatchString(trimmed):
			m := cjsExportNamedRE.FindStringSubmatch(trimmed)
			for _, n := range strings.Split(m[1], ",") {
				n = strings.TrimSpace(n)
				if n == "" {
					continue
				}
				source, local := splitExportName(n)
				fmt.Fprintf(&out, "module.exports.%s = %s;\n", local, source)
			}
		default:
			out.WriteString(line)
			out.WriteString("\n")
			advancePendingExports(&pending, line, &out)
		}
	}

	return out.Bytes()
}

// pendingCjsExport tracks an in-flight export whose module.exports.X = X
// assignment must be appended when its body closes.
type pendingCjsExport struct {
	name  string
	depth int
}

// pushPendingExport records a new pending export for a class, enum, or
// function declaration. It writes the source line, computes the opening
// brace delta, and pushes the entry on the stack.
func pushPendingExport(stack *[]pendingCjsExport, name, line string, out *bytes.Buffer) {
	delta := strings.Count(line, "{") - strings.Count(line, "}")
	out.WriteString(line)
	out.WriteString("\n")
	*stack = append(*stack, pendingCjsExport{name: name, depth: delta})
}

// advancePendingExports walks the pending-export stack and emits
// module.exports.X = X lines for any export whose body just closed on
// this line. Unrelated braces inside an export body are tolerated
// because each entry has its own depth counter.
func advancePendingExports(stack *[]pendingCjsExport, line string, out *bytes.Buffer) {
	delta := strings.Count(line, "{") - strings.Count(line, "}")
	// Apply the brace delta to every entry's depth counter.
	for i := range *stack {
		(*stack)[i].depth += delta
	}
	// Pop entries whose depth has closed. Process innermost-first so
	// nested export classes emit their module.exports in the right order.
	for len(*stack) > 0 {
		top := &(*stack)[len(*stack)-1]
		if top.depth > 0 {
			return
		}
		// depth <= 0: the body has closed on this line (or earlier).
		// Emit the assignment and pop.
		fmt.Fprintf(out, "module.exports.%s = %s;\n", top.name, top.name)
		*stack = (*stack)[:len(*stack)-1]
	}
}

// splitExportName parses an entry from an `export { a, b as c, ... }`
// list, returning the source name (what to read) and the local name (what
// to bind on module.exports). The input is already trimmed.
func splitExportName(s string) (source, local string) {
	if idx := strings.Index(s, " as "); idx >= 0 {
		return strings.TrimSpace(s[:idx]), strings.TrimSpace(s[idx+len(" as "):])
	}
	return s, s
}

// stripExportPrefix removes the leading `export ` keyword from a line
// (with one trailing space), if present. Used by the CJS transform so the
// emitted body uses plain `class` / `function` / `enum` / `const`
// declarations while the module.exports.X = X line carries the export.
func stripExportPrefix(line string) string {
	const prefix = "export "
	if strings.HasPrefix(line, prefix) {
		return line[len(prefix):]
	}
	return line
}

// writeTransformedFile is a helper that applies the active module-style
// transform to content and writes it to path. It centralises the
// transformation step required by step 7.3 of the plan: every user-generated
// file and every runtime file passes through here.
func (p *TSClientServer) writeTransformedFile(path string, content []byte) error {
	transformed := p.transformFileForStyle(content, p.moduleStyle)
	if err := os.WriteFile(path, transformed, 0644); err != nil {
		return fmt.Errorf("failed to write %s: %w", path, err)
	}
	return nil
}

// wipePulserpcDirs removes any leftover runtime directories from a previous
// generation. Different module styles ship different runtime trees, so a
// stale `client.ts` from a prior CJS run could otherwise satisfy a later
// ESM regeneration if the filenames happened to coincide. See step 8.
//
// runtimeDir is the resolved path to the runtime directory for the current
// packageBase (typically <outputDir>/pulserpc or
// <outputDir>/<packageBase>/pulserpc). namespaceDirs lists any per-namespace
// runtime subdirectories the plan asks us to defend against
// (<outputDir>/<namespace>/pulserpc); the existing path helper does not
// place the runtime per-namespace, but a previous run or a custom caller
// might have.
//
// os.RemoveAll is a no-op for non-existent paths, so wiping a directory
// that doesn't exist does not surface as an error.
func wipePulserpcDirs(runtimeDir string, namespaceDirs ...string) {
	if runtimeDir != "" {
		_ = os.RemoveAll(runtimeDir)
	}
	for _, d := range namespaceDirs {
		if d == "" {
			continue
		}
		_ = os.RemoveAll(d)
	}
}

// tsPackageJSON is the JSON shape emitted by generatePackageJSON. The
// struct tags control the field order in the rendered file.
type tsPackageJSON struct {
	Name    string            `json:"name"`
	Version string            `json:"version"`
	Type    string            `json:"type"`
	Main    string            `json:"main,omitempty"`
	Scripts map[string]string `json:"scripts"`
	Exports map[string]string `json:"exports,omitempty"`
}

// sanitizePackageName returns a valid `name` field value for a generated
// package.json, derived from filepath.Base(outputDir). Leading non-
// alphanumeric runes are stripped; if nothing is left (e.g., basename "/"),
// the literal fallback "pulserpc-generated" is returned.
func sanitizePackageName(base string) string {
	name := strings.TrimLeftFunc(base, func(r rune) bool {
		return (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && (r < '0' || r > '9')
	})
	if name == "" {
		return "pulserpc-generated"
	}
	return name
}

// generatePackageJSON writes a minimal package.json at outputDir that
// matches the resolved module style. See step 9 of the plan.
//
// outputDir must be non-empty (the caller should skip generation when no
// -dir was provided). The function refuses to overwrite an existing
// package.json — callers must delete it or omit -ts-gen-package-json.
//
// Field rules:
//   - `type` is "module" for esm-node and esm-bundler, "commonjs" for cjs.
//   - `main` is "./client.js" only in flat (single-namespace, no-package)
//     mode for esm-node and cjs. esm-bundler never gets `main` (bundlers
//     resolve via `exports`). Multi-namespace mode omits `main` because
//     the exports map is the canonical entry point.
//   - `exports` is emitted only in multi-namespace mode, with one entry
//     per non-empty namespace pointing at "./<ns>/index.js".
//
// The file is written via json.MarshalIndent so it round-trips through
// encoding/json (an invariant in step 9).
func (p *TSClientServer) generatePackageJSON(outputDir string, useNamespaceDirs bool, namespaces []string) error {
	if outputDir == "" {
		return fmt.Errorf("cannot generate package.json: outputDir is empty")
	}
	pkgPath := filepath.Join(outputDir, "package.json")
	if _, err := os.Stat(pkgPath); err == nil {
		return fmt.Errorf("package.json already exists at %s; remove it or skip -ts-gen-package-json", pkgPath)
	}

	typeVal := "module"
	if p.moduleStyle == "cjs" {
		typeVal = "commonjs"
	}

	pkg := tsPackageJSON{
		Name:    sanitizePackageName(filepath.Base(outputDir)),
		Version: "0.1.0",
		Type:    typeVal,
		Scripts: map[string]string{"build": "tsc"},
	}

	// `main` is only meaningful in flat mode for the non-bundler styles.
	if p.moduleStyle != "esm-bundler" && !useNamespaceDirs {
		pkg.Main = "./client.js"
	}

	// `exports` is only emitted in multi-namespace mode.
	if useNamespaceDirs {
		pkg.Exports = make(map[string]string, len(namespaces))
		for _, ns := range namespaces {
			if ns == "" {
				continue
			}
			pkg.Exports["./"+ns] = "./" + ns + "/index.js"
		}
	}

	data, err := json.MarshalIndent(pkg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal package.json: %w", err)
	}
	// Trailing newline so editors and POSIX tools are happy.
	data = append(data, '\n')

	if err := os.WriteFile(pkgPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write %s: %w", pkgPath, err)
	}
	return nil
}

// tsConfigJSON is the JSON shape emitted by generateTSConfig. We use a
// free-form compilerOptions map so future tweaks (e.g., adding a new
// compiler option) don't require schema changes here.
type tsConfigJSON struct {
	CompilerOptions map[string]interface{} `json:"compilerOptions"`
	Include         []string               `json:"include"`
	Exclude         []string               `json:"exclude"`
}

// tsconfigStyleValues returns the (module, moduleResolution) pair that
// matches the resolved module style, per the §10.3 mapping table.
func tsconfigStyleValues(moduleStyle string) (moduleVal, resolutionVal string) {
	switch moduleStyle {
	case "cjs":
		return "CommonJS", "Node10"
	case "esm-bundler":
		return "Bundler", "Bundler"
	default:
		// "esm-node" and any defensive default.
		return "NodeNext", "NodeNext"
	}
}

// generateTSConfig writes a minimal tsconfig.json at outputDir that
// matches the resolved module style. See step 10 of the plan.
//
// outputDir must be non-empty. The function refuses to overwrite an
// existing tsconfig.json — callers must delete it or omit
// -ts-gen-tsconfig.
//
// The file includes a base compilerOptions block (target ES2020,
// strict=false, esModuleInterop, etc.), `include: ["**/*.ts"]`, and
// `exclude: ["node_modules"]`. `module` and `moduleResolution` are
// always consistent with the resolved module style.
func (p *TSClientServer) generateTSConfig(outputDir string) error {
	if outputDir == "" {
		return fmt.Errorf("cannot generate tsconfig.json: outputDir is empty")
	}
	tsPath := filepath.Join(outputDir, "tsconfig.json")
	if _, err := os.Stat(tsPath); err == nil {
		return fmt.Errorf("tsconfig.json already exists at %s; remove it or skip -ts-gen-tsconfig", tsPath)
	}

	moduleVal, resolutionVal := tsconfigStyleValues(p.moduleStyle)

	compilerOpts := map[string]interface{}{
		"target":           "ES2020",
		"module":           moduleVal,
		"moduleResolution": resolutionVal,
		"strict":           false,
		"esModuleInterop":  true,
		"skipLibCheck":     true,
	}
	// allowImportingTsExtensions is incompatible with module: CommonJS
	// in TypeScript 5.x (requires noEmit or rewriteRelativeImportExtensions).
	if p.moduleStyle != "cjs" {
		compilerOpts["allowImportingTsExtensions"] = true
	}
	// CJS code uses require(), module.exports, and process; tell tsc where to
	// find type declarations for these Node built-ins.
	if p.moduleStyle == "cjs" {
		compilerOpts["types"] = []string{"node"}
	}
	// moduleResolution=Node10 is deprecated in TS 5.x+; suppress the
	// deprecation diagnostic so tsc --project works without error.
	if resolutionVal == "Node10" {
		compilerOpts["ignoreDeprecations"] = "6.0"
	}
	cfg := tsConfigJSON{
		CompilerOptions: compilerOpts,
		Include:         []string{"**/*.ts"},
		Exclude:         []string{"node_modules"},
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal tsconfig.json: %w", err)
	}
	data = append(data, '\n')

	if err := os.WriteFile(tsPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write %s: %w", tsPath, err)
	}
	return nil
}

// sortedNamespaceNames returns the keys of namespaceMap in deterministic
// (sorted) order, with the empty namespace skipped. Used by
// generatePackageJSON's exports map and by Generate's wiring.
func sortedNamespaceNames(namespaceMap map[string]*NamespaceTypes) []string {
	out := make([]string, 0, len(namespaceMap))
	for k := range namespaceMap {
		if k == "" {
			continue
		}
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// Generate generates TypeScript HTTP server and client code from the parsed IDL
func (p *TSClientServer) Generate(idl *parser.IDL, fs *flag.FlagSet) error {
	// Check silent flag
	silentFlag := fs.Lookup("silent")
	isSilent := func() bool {
		return silentFlag != nil && silentFlag.Value.String() == "true"
	}

	// Access the -dir flag value
	dirFlag := fs.Lookup("dir")
	outputDir := ""
	if dirFlag != nil && dirFlag.Value.String() != "" {
		outputDir = dirFlag.Value.String()
	}

	// Get package prefix flag
	packageFlag := fs.Lookup("package")
	packagePrefix := ""
	if packageFlag != nil && packageFlag.Value.String() != "" {
		packagePrefix = packageFlag.Value.String()
	}
	p.packageBase = packagePrefix

	// Resolve the effective module style (explicit flag + auto-detection).
	// Must run before any code-emit or runtime-copy code reads p.moduleStyle.
	if err := p.resolveEffectiveModuleStyle(fs, outputDir); err != nil {
		return fmt.Errorf("failed to resolve module style: %w", err)
	}

	// Build type registries
	structMap := make(map[string]*parser.Struct)
	enumMap := make(map[string]*parser.Enum)
	interfaceMap := make(map[string]*parser.Interface)

	for _, s := range idl.Structs {
		structMap[s.Name] = s
	}
	for _, e := range idl.Enums {
		enumMap[e.Name] = e
	}
	for _, i := range idl.Interfaces {
		interfaceMap[i.Name] = i
	}

	// Group types by namespace
	namespaceMap := GroupTypesByNamespace(idl)

	// Initialize path helpers with package base
	paths := NewTSNamespacePaths(outputDir, p.packageBase)

	// Step 8: wipe any leftover pulserpc runtime directories from a previous
	// generation before re-creating them. Different module styles ship
	// different runtime trees, so a stale `client.ts` from a prior CJS run
	// could otherwise satisfy a later ESM regeneration if filenames
	// happened to coincide. Wipe the single runtime dir (under packageBase
	// when set) plus any per-namespace runtime subdirs as a defensive
	// measure.
	nsRuntimeDirs := make([]string, 0, len(namespaceMap))
	for ns := range namespaceMap {
		nsRuntimeDirs = append(nsRuntimeDirs, filepath.Join(outputDir, ns, "pulserpc"))
		if packagePrefix != "" {
			nsRuntimeDirs = append(nsRuntimeDirs, filepath.Join(outputDir, packagePrefix, ns, "pulserpc"))
		}
	}
	wipePulserpcDirs(paths.ResolveRuntimeDir(), nsRuntimeDirs...)

	// Ensure runtime directory exists
	if err := paths.EnsureRuntimeDir(); err != nil {
		return fmt.Errorf("failed to create runtime directory: %w", err)
	}

	// Copy runtime library files to the resolved runtime directory
	if err := p.copyRuntimeFiles(paths, isSilent()); err != nil {
		return fmt.Errorf("failed to copy runtime files: %w", err)
	}

	// Determine if multi-namespace mode or package mode is active
	// When -package flag is set, use namespace directories even for single namespace
	hasPackage := packagePrefix != ""
	multiNsMode := isMultiNamespaceMode(outputDir, namespaceMap)
	useNamespaceDirs := multiNsMode || hasPackage

	// Create per-namespace subdirectories when needed
	if useNamespaceDirs {
		for ns := range namespaceMap {
			if err := paths.EnsureNamespaceDir(ns); err != nil {
				return fmt.Errorf("failed to create namespace directory: %w", err)
			}
		}
	}

	// Write IDL JSON file to entry-point namespace directory only
	// The entry-point is the namespace of the root file being parsed
	// This applies in ALL cases, regardless of multi-namespace mode
	entryPointNs := idl.RootNamespace
	if entryPointNs == "" {
		// Fallback: if no RootNamespace set, use the first namespace
		for ns := range namespaceMap {
			entryPointNs = ns
			break
		}
	}
	if entryPointNs != "" {
		entryPointDir := paths.ResolveNamespaceDir(entryPointNs)
		// Ensure entry-point namespace directory exists
		if err := paths.EnsureNamespaceDir(entryPointNs); err != nil {
			return fmt.Errorf("failed to create entry-point namespace directory: %w", err)
		}
		if err := writeIDLJSONTs(idl, entryPointDir, fs); err != nil {
			return fmt.Errorf("failed to write idl.json: %w", err)
		}
	} else {
		// No namespaces at all - write to root (edge case)
		if err := writeIDLJSONTs(idl, outputDir, fs); err != nil {
			return fmt.Errorf("failed to write idl.json: %w", err)
		}
	}

	// Generate per-namespace files when using namespace directories,
	// or flat files for backwards-compatible single-namespace output without package.
	if useNamespaceDirs {
		// Multi-namespace or package mode: generate types.ts, server.ts, client.ts per namespace
		for ns, nsTypes := range namespaceMap {
			nsDir := paths.ResolveNamespaceDir(ns)

			// Build namespace-scoped maps for type resolution
			nsStructMap := make(map[string]*parser.Struct)
			for _, s := range nsTypes.Structs {
				nsStructMap[s.Name] = s
			}
			nsEnumMap := make(map[string]*parser.Enum)
			for _, e := range nsTypes.Enums {
				nsEnumMap[e.Name] = e
			}
			nsInterfaceMap := make(map[string]*parser.Interface)
			for _, i := range nsTypes.Interfaces {
				nsInterfaceMap[i.Name] = i
			}

			// Generate types.ts for this namespace
			typesCode := generateTypesTsForNamespace(nsTypes, ns, nsStructMap, nsEnumMap, true, namespaceMap, p.moduleStyle)
			typesPath := filepath.Join(nsDir, "types.ts")
			if err := p.writeTransformedFile(typesPath, []byte(typesCode)); err != nil {
				return err
			}
			PrintFileCreated(typesPath, fs)

			// Generate server.ts for this namespace
			serverCode := generateServerTsForNamespace(nsTypes, nsStructMap, nsEnumMap, nsInterfaceMap, packagePrefix, true, namespaceMap, p.moduleStyle)
			serverPath := filepath.Join(nsDir, "server.ts")
			if err := p.writeTransformedFile(serverPath, []byte(serverCode)); err != nil {
				return err
			}
			PrintFileCreated(serverPath, fs)

			// Generate client.ts for this namespace
			clientCode := generateClientTsForNamespace(nsTypes, nsStructMap, nsEnumMap, packagePrefix, true, namespaceMap, p.moduleStyle)
			clientPath := filepath.Join(nsDir, "client.ts")
			if err := p.writeTransformedFile(clientPath, []byte(clientCode)); err != nil {
				return err
			}
			PrintFileCreated(clientPath, fs)

			// Generate index.ts for this namespace (re-exports from types, server, client)
			if err := p.generateNamespaceIndexTs(paths, ns); err != nil {
				return fmt.Errorf("failed to write %s/index.ts: %w", ns, err)
			}
			indexPath := filepath.Join(nsDir, "index.ts")
			PrintFileCreated(indexPath, fs)
		}
	} else {
		// Backwards-compatible flat output: generate single types.ts, server.ts, client.ts
		typesCode := generateTypesTs(structMap, enumMap)
		typesPath := filepath.Join(outputDir, "types.ts")
		if err := p.writeTransformedFile(typesPath, []byte(typesCode)); err != nil {
			return err
		}
		PrintFileCreated(typesPath, fs)

		serverCode := generateServerTs(idl, structMap, enumMap, interfaceMap, packagePrefix, namespaceMap, p.moduleStyle)
		serverPath := filepath.Join(outputDir, "server.ts")
		if err := p.writeTransformedFile(serverPath, []byte(serverCode)); err != nil {
			return err
		}
		PrintFileCreated(serverPath, fs)

		clientCode := generateClientTs(idl, structMap, enumMap, packagePrefix, namespaceMap, p.moduleStyle)
		clientPath := filepath.Join(outputDir, "client.ts")
		if err := p.writeTransformedFile(clientPath, []byte(clientCode)); err != nil {
			return err
		}
		PrintFileCreated(clientPath, fs)
	}

	// Steps 9 & 10: emit package.json / tsconfig.json when the
	// corresponding flags are set. Both files live at the root of
	// outputDir, independent of namespace layout. The flag checks are
	// no-ops when the flag isn't registered, matching the pattern used
	// by `silent` and `generate-test-files` above.
	if genPackageFlag := fs.Lookup("ts-gen-package-json"); genPackageFlag != nil && genPackageFlag.Value.String() == "true" {
		namespaces := sortedNamespaceNames(namespaceMap)
		if err := p.generatePackageJSON(outputDir, useNamespaceDirs, namespaces); err != nil {
			return fmt.Errorf("failed to generate package.json: %w", err)
		}
		PrintFileCreated(filepath.Join(outputDir, "package.json"), fs)
	}
	if genTSConfigFlag := fs.Lookup("ts-gen-tsconfig"); genTSConfigFlag != nil && genTSConfigFlag.Value.String() == "true" {
		if err := p.generateTSConfig(outputDir); err != nil {
			return fmt.Errorf("failed to generate tsconfig.json: %w", err)
		}
		PrintFileCreated(filepath.Join(outputDir, "tsconfig.json"), fs)
	}

	// Check if generate-test-files flag is set
	generateTestFilesFlag := fs.Lookup("generate-test-files")
	generateTestServer := generateTestFilesFlag != nil && generateTestFilesFlag.Value.String() == "true"

	// Generate test server and client if flag is set
	if generateTestServer {
		// Determine entry-point namespace directory for test files
		testDir := outputDir
		if entryPointNs != "" {
			testDir = paths.ResolveNamespaceDir(entryPointNs)
		}

		// Generate test_server.ts
		testServerCode := generateTestServerTs(idl, structMap, enumMap, interfaceMap, packagePrefix, namespaceMap, entryPointNs, p.moduleStyle)
		testServerPath := filepath.Join(testDir, "test_server.ts")
		if err := p.writeTransformedFile(testServerPath, []byte(testServerCode)); err != nil {
			return err
		}
		PrintFileCreated(testServerPath, fs)

		// Generate test_client.ts
		testClientCode := generateTestClientTs(idl, structMap, enumMap, interfaceMap, packagePrefix, namespaceMap, entryPointNs, p.moduleStyle)
		testClientPath := filepath.Join(testDir, "test_client.ts")
		if err := p.writeTransformedFile(testClientPath, []byte(testClientCode)); err != nil {
			return err
		}
		PrintFileCreated(testClientPath, fs)
	}

	return nil
}

// copyRuntimeFiles copies the TypeScript runtime library files to the output directory.
// The runtime tree is selected from p.moduleStyle: "esm-node" and "esm-bundler"
// share the ts-node tree (and the bundler transform in step 7 will rewrite
// imports in the written files); "cjs" pulls from the ts-cjs tree.
//
// Each file's bytes pass through transformFileForStyle before being
// written. For esm-node the transform is a strict no-op (byte-equal
// output), for esm-bundler it strips the `.js` suffix from every relative
// import, and for cjs it converts ESM imports/exports to require/module.exports.
func (p *TSClientServer) copyRuntimeFiles(paths TSNamespacePaths, silent bool) error {
	runtimeDir := paths.ResolveRuntimeDir()
	if err := os.MkdirAll(runtimeDir, 0755); err != nil {
		return fmt.Errorf("failed to create runtime directory: %w", err)
	}

	style := p.moduleStyle
	if style == "" {
		style = "esm-node"
	}
	files, err := runtime.GetRuntimeFilesForStyle("ts", style)
	if err != nil {
		return err
	}

	for filename, data := range files {
		dstPath := filepath.Join(runtimeDir, filename)
		// For esm-node the transform is a no-op; for esm-bundler it strips
		// .js suffixes from relative imports. For cjs the embedded runtime
		// tree is already authored as CJS (require/module.exports), so
		// running the line-based CJS transform on it would be wasted work
		// and could mangle content (e.g., trailing-newline drift).
		transformed := data
		if style != "cjs" {
			transformed = p.transformFileForStyle(data, p.moduleStyle)
		}
		if err := os.WriteFile(dstPath, transformed, 0644); err != nil {
			return fmt.Errorf("failed to write runtime file %s: %w", dstPath, err)
		}
		if !silent {
			fmt.Println(dstPath)
		}
	}

	return nil
}

// writeIDLJSONTs writes the IDL metadata as JSON to idl.json
//
// IDL Placement:
// - Writes to the specified output directory (e.g., {dir}/{namespace}/idl.json)
// - Only the entry-point namespace directory gets idl.json
func writeIDLJSONTs(idl *parser.IDL, outputDir string, fs *flag.FlagSet) error {
	checksum, err := parser.ComputeChecksum(idl)
	if err != nil {
		return fmt.Errorf("failed to compute checksum: %w", err)
	}
	idl.Checksum = checksum

	idlJSON, err := json.MarshalIndent(idl, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal IDL to JSON: %w", err)
	}

	idlPath := filepath.Join(outputDir, "idl.json")
	if err := os.WriteFile(idlPath, idlJSON, 0644); err != nil {
		return fmt.Errorf("failed to write idl.json: %w", err)
	}
	PrintFileCreated(idlPath, fs)
	return nil
}

// collectCrossNamespaceImports identifies which external namespaces are referenced
// by types in the given namespace's structs and enums. Returns a map of namespace
// names that need to be imported.
func collectCrossNamespaceImports(nsTypes *NamespaceTypes, currentNs string, allNamespaceMap map[string]*NamespaceTypes) map[string]string {
	imports := make(map[string]string)

	localTypes := make(map[string]bool)
	for _, s := range nsTypes.Structs {
		localTypes[GetBaseName(s.Name)] = true
	}
	for _, e := range nsTypes.Enums {
		localTypes[GetBaseName(e.Name)] = true
	}

	for _, s := range nsTypes.Structs {
		// Check extends clause
		if s.Extends != "" {
			typeName := GetBaseName(s.Extends)
			if !localTypes[typeName] {
				for ns, nsTypes := range allNamespaceMap {
					if ns == currentNs {
						continue
					}
					for _, otherStruct := range nsTypes.Structs {
						if GetBaseName(otherStruct.Name) == typeName {
							imports[ns] = ns + "Types"
							break
						}
					}
				}
			}
		}

		// Check struct fields
		for _, field := range s.Fields {
			if field.Type != nil && field.Type.IsUserDefined() {
				typeName := GetBaseName(field.Type.UserDefined)
				if !localTypes[typeName] {
					for ns, nsTypes := range allNamespaceMap {
						if ns == currentNs {
							continue
						}
						for _, otherStruct := range nsTypes.Structs {
							if GetBaseName(otherStruct.Name) == typeName {
								imports[ns] = ns + "Types"
								break
							}
						}
						if _, ok := imports[ns]; !ok {
							for _, otherEnum := range nsTypes.Enums {
								if GetBaseName(otherEnum.Name) == typeName {
									imports[ns] = ns + "Types"
									break
								}
							}
						}
					}
				}
			}
		}
	}

	return imports
}

// getExtendsTypeForNamespace returns the proper TypeScript type reference for an extends clause.
// When the parent type is from another namespace, it returns "namespace.TypeName" format.
func getExtendsTypeForNamespace(extendsType string, _ map[string]*parser.Struct, _ map[string]*parser.Enum, currentNs string, _ map[string]*NamespaceTypes) string {
	baseName := GetBaseName(extendsType)

	// Check if it's a local type (no namespace or same namespace)
	if !strings.Contains(extendsType, ".") {
		return baseName
	}

	// Check if the extends type belongs to this namespace
	nsPrefix := GetNamespaceFromType(extendsType, "")
	if nsPrefix == currentNs {
		return baseName
	}

	// It's from another namespace - use namespace.TypeName format
	if strings.Contains(extendsType, ".") {
		parts := strings.Split(extendsType, ".")
		if len(parts) >= 2 {
			return parts[len(parts)-2] + "." + baseName
		}
	}

	return baseName
}

// getTypeScriptTypeForNamespace converts a parser.Type to a TypeScript type string,
// with support for cross-namespace type references in multi-namespace mode.
// If useTypesPrefix is true, user-defined types are prefixed with "types." (for use in server.ts).
// When inNamespaceSubdir is true and the type belongs to another namespace, it prefixes with that namespace.
func getTypeScriptTypeForNamespace(t *parser.Type, structMap map[string]*parser.Struct, enumMap map[string]*parser.Enum, useTypesPrefix bool, inNamespaceSubdir bool, allNamespaceMap map[string]*NamespaceTypes) string {
	if t == nil {
		return "void"
	}
	if t.IsBuiltIn() {
		switch t.BuiltIn {
		case "string":
			return "string"
		case "int", "float":
			return "number"
		case "bool":
			return "boolean"
		default:
			return "any"
		}
	}
	if t.IsArray() {
		return getTypeScriptTypeForNamespace(t.Array, structMap, enumMap, useTypesPrefix, inNamespaceSubdir, allNamespaceMap) + "[]"
	}
	if t.IsMap() {
		return "Record<string, " + getTypeScriptTypeForNamespace(t.MapValue, structMap, enumMap, useTypesPrefix, inNamespaceSubdir, allNamespaceMap) + ">"
	}
	if t.IsUserDefined() {
		typeName := t.UserDefined
		// Check if it's a struct
		if structMap[typeName] != nil {
			if useTypesPrefix {
				return "types." + GetBaseName(typeName)
			}
			return typeName
		}
		// Check if it's an enum
		if enumMap[typeName] != nil {
			if useTypesPrefix {
				return "types." + GetBaseName(typeName)
			}
			return typeName
		}
		// Try to find by base name (for namespaced types like "inc.Response")
		for key := range structMap {
			if strings.HasSuffix(key, "."+typeName) {
				if useTypesPrefix {
					return "types." + key
				}
				return key
			}
		}
		for key := range enumMap {
			if strings.HasSuffix(key, "."+typeName) {
				if useTypesPrefix {
					return "types." + GetBaseName(key)
				}
				return GetBaseName(key)
			}
		}

		// In multi-namespace mode, check if this type belongs to another namespace
		if inNamespaceSubdir && allNamespaceMap != nil {
			baseTypeName := GetBaseName(typeName)
			for ns, nsTypes := range allNamespaceMap {
				for _, s := range nsTypes.Structs {
					if GetBaseName(s.Name) == baseTypeName {
						return ns + "." + baseTypeName
					}
				}
				for _, e := range nsTypes.Enums {
					if GetBaseName(e.Name) == baseTypeName {
						return ns + "." + baseTypeName
					}
				}
			}
		}

		if useTypesPrefix {
			return "types." + GetBaseName(typeName)
		}
		return GetBaseName(typeName)
	}
	return "any"
}

// Step 10 of the multi-namespace implementation spec:
// The -package flag no longer affects class names. It now only serves as the base module
// path for generated imports (e.g., @myapp/lib/rpc). The previous class-name-prefix behavior
// has been removed as no existing test or quickstart used it with a non-empty value.

// generateServerTs generates the server.ts file with abstract interface classes only
func generateServerTs(idl *parser.IDL, structMap map[string]*parser.Struct, enumMap map[string]*parser.Enum, _ map[string]*parser.Interface, _ string, _ map[string]*NamespaceTypes, moduleStyle string) string {
	var sb strings.Builder

	sb.WriteString("// Generated by pulserpc - do not edit\n\n")
	sb.WriteString("// Abstract service classes\n")
	sb.WriteString("// Implement these classes to create your service\n\n")
	fmt.Fprintf(&sb, "import { RPCError } from '%s';\n", tsImportPath(moduleStyle, "./pulserpc/rpc"))
	fmt.Fprintf(&sb, "import * as types from '%s';\n\n", tsImportPath(moduleStyle, "./types"))

	// Generate interface stub abstract classes
	ifaceNames := make([]string, 0, len(idl.Interfaces))
	for _, iface := range idl.Interfaces {
		ifaceNames = append(ifaceNames, iface.Name)
	}
	sort.Strings(ifaceNames)
	for _, ifaceName := range ifaceNames {
		var iface *parser.Interface
		for _, i := range idl.Interfaces {
			if i.Name == ifaceName {
				iface = i
				break
			}
		}
		if iface != nil {
			writeInterfaceStubTs(&sb, iface, structMap, enumMap)
		}
	}

	return sb.String()
}

// writeInterfaceStubTs generates an abstract class for an interface
func writeInterfaceStubTs(sb *strings.Builder, iface *parser.Interface, structMap map[string]*parser.Struct, enumMap map[string]*parser.Enum) {
	if iface.Comment != "" {
		lines := strings.Split(strings.TrimSpace(iface.Comment), "\n")
		for _, line := range lines {
			fmt.Fprintf(sb, "// %s\n", line)
		}
	}
	fmt.Fprintf(sb, "export abstract class %s {\n", iface.Name)

	methodNames := make([]string, 0, len(iface.Methods))
	for _, method := range iface.Methods {
		methodNames = append(methodNames, method.Name)
	}
	sort.Strings(methodNames)
	for _, methodName := range methodNames {
		var method *parser.Method
		for _, m := range iface.Methods {
			if m.Name == methodName {
				method = m
				break
			}
		}
		if method == nil {
			continue
		}
		fmt.Fprintf(sb, "  abstract %s(", method.Name)
		fmt.Fprintf(sb, "ctx: Record<string, any>")
		hasParams := len(method.Parameters) > 0
		if hasParams {
			sb.WriteString(", ")
		}
		for i, param := range method.Parameters {
			if i > 0 {
				sb.WriteString(", ")
			}
			tsType := getTypeScriptType(param.Type, structMap, enumMap, true)
			fmt.Fprintf(sb, "%s: %s", param.Name, tsType)
		}
		returnType := getTypeScriptType(method.ReturnType, structMap, enumMap, true)
		if method.ReturnOptional {
			returnType = returnType + " | null"
		}
		if returnType != "void" {
			returnType = "Promise<" + returnType + ">"
		}
		sb.WriteString("): " + returnType + ";\n")
	}
	sb.WriteString("}\n\n")
}

// writeInterfaceStubTsForNamespace generates an abstract class for an interface
// with support for cross-namespace type references.
func writeInterfaceStubTsForNamespace(sb *strings.Builder, iface *parser.Interface, structMap map[string]*parser.Struct, enumMap map[string]*parser.Enum, _ string, allNamespaceMap map[string]*NamespaceTypes) {
	if iface.Comment != "" {
		lines := strings.Split(strings.TrimSpace(iface.Comment), "\n")
		for _, line := range lines {
			fmt.Fprintf(sb, "// %s\n", line)
		}
	}
	fmt.Fprintf(sb, "export abstract class %s {\n", iface.Name)

	methodNames := make([]string, 0, len(iface.Methods))
	for _, method := range iface.Methods {
		methodNames = append(methodNames, method.Name)
	}
	sort.Strings(methodNames)
	for _, methodName := range methodNames {
		var method *parser.Method
		for _, m := range iface.Methods {
			if m.Name == methodName {
				method = m
				break
			}
		}
		if method == nil {
			continue
		}
		fmt.Fprintf(sb, "  abstract %s(", method.Name)
		fmt.Fprintf(sb, "ctx: Record<string, any>")
		hasParams := len(method.Parameters) > 0
		if hasParams {
			sb.WriteString(", ")
		}
		for i, param := range method.Parameters {
			if i > 0 {
				sb.WriteString(", ")
			}
			tsType := getTypeScriptTypeForNamespace(param.Type, structMap, enumMap, true, true, allNamespaceMap)
			fmt.Fprintf(sb, "%s: %s", param.Name, tsType)
		}
		returnType := getTypeScriptTypeForNamespace(method.ReturnType, structMap, enumMap, true, true, allNamespaceMap)
		if method.ReturnOptional {
			returnType = returnType + " | null"
		}
		if returnType != "void" {
			returnType = "Promise<" + returnType + ">"
		}
		sb.WriteString("): " + returnType + ";\n")
	}
	sb.WriteString("}\n\n")
}

// getTypeScriptType converts a parser.Type to a TypeScript type string
// If useTypesPrefix is true, user-defined types are prefixed with "types." (for use in server.ts)
func getTypeScriptType(t *parser.Type, structMap map[string]*parser.Struct, enumMap map[string]*parser.Enum, useTypesPrefix bool) string {
	if t == nil {
		return "void"
	}
	if t.IsBuiltIn() {
		switch t.BuiltIn {
		case "string":
			return "string"
		case "int", "float":
			return "number"
		case "bool":
			return "boolean"
		default:
			return "any"
		}
	}
	if t.IsArray() {
		return getTypeScriptType(t.Array, structMap, enumMap, useTypesPrefix) + "[]"
	}
	if t.IsMap() {
		return "Record<string, " + getTypeScriptType(t.MapValue, structMap, enumMap, useTypesPrefix) + ">"
	}
	if t.IsUserDefined() {
		typeName := t.UserDefined
		// Check if it's a struct
		if structMap[typeName] != nil {
			if useTypesPrefix {
				return "types." + GetBaseName(typeName)
			}
			return typeName
		}
		// Check if it's an enum
		if enumMap[typeName] != nil {
			if useTypesPrefix {
				return "types." + GetBaseName(typeName)
			}
			return typeName
		}
		// Try to find by base name (for namespaced types like "inc.Response")
		for key := range structMap {
			if strings.HasSuffix(key, "."+typeName) {
				if useTypesPrefix {
					return "types." + key
				}
				return key
			}
		}
		for key := range enumMap {
			if strings.HasSuffix(key, "."+typeName) {
				if useTypesPrefix {
					return "types." + GetBaseName(key)
				}
				return GetBaseName(key)
			}
		}
		if useTypesPrefix {
			return "types." + GetBaseName(typeName)
		}
		return typeName
	}
	return "any"
}

// generateTypesTs generates a types.ts file with TypeScript interfaces for structs and enums
func generateTypesTs(structMap map[string]*parser.Struct, enumMap map[string]*parser.Enum) string {
	var sb strings.Builder

	sb.WriteString("// Generated by pulserpc - do not edit\n\n")
	sb.WriteString("// TypeScript interfaces and enums for all IDL types\n\n")

	// Generate enums first (they may be used by structs)
	enumNames := make([]string, 0, len(enumMap))
	for name := range enumMap {
		enumNames = append(enumNames, name)
	}
	sort.Strings(enumNames)
	for _, enumName := range enumNames {
		enum := enumMap[enumName]
		comment := strings.TrimSpace(enum.Comment)
		if comment != "" {
			lines := strings.Split(comment, "\n")
			for _, line := range lines {
				fmt.Fprintf(&sb, "// %s\n", line)
			}
		}
		fmt.Fprintf(&sb, "export enum %s {\n", GetBaseName(enum.Name))
		for i, val := range enum.Values {
			valComment := strings.TrimSpace(val.Comment)
			if valComment != "" {
				lines := strings.Split(valComment, "\n")
				for _, line := range lines {
					fmt.Fprintf(&sb, "  // %s\n", line)
				}
			}
			fmt.Fprintf(&sb, "  %s = \"%s\"", val.Name, val.Name)
			if i < len(enum.Values)-1 {
				sb.WriteString(",\n")
			} else {
				sb.WriteString("\n")
			}
		}
		sb.WriteString("}\n\n")
	}

	// Generate structs (with extends support)
	structNames := make([]string, 0, len(structMap))
	for name := range structMap {
		structNames = append(structNames, name)
	}
	sort.Strings(structNames)
	for _, structName := range structNames {
		structDef := structMap[structName]
		comment := strings.TrimSpace(structDef.Comment)
		if comment != "" {
			lines := strings.Split(comment, "\n")
			for _, line := range lines {
				fmt.Fprintf(&sb, "// %s\n", line)
			}
		}
		fmt.Fprintf(&sb, "export interface %s", structDef.Name)
		if structDef.Extends != "" {
			// Handle extends - use just the base name if it's namespaced
			baseName := structDef.Extends
			if strings.Contains(baseName, ".") {
				parts := strings.Split(baseName, ".")
				baseName = parts[len(parts)-1]
			}
			sb.WriteString(" extends " + baseName)
		}
		sb.WriteString(" {\n")
		for _, field := range structDef.Fields {
			fieldComment := strings.TrimSpace(field.Comment)
			if fieldComment != "" {
				lines := strings.Split(fieldComment, "\n")
				for _, line := range lines {
					fmt.Fprintf(&sb, "  // %s\n", line)
				}
			}
			tsType := getTypeScriptType(field.Type, structMap, enumMap, false)
			optionalMarker := ""
			if field.Optional {
				optionalMarker = "?"
			}
			fmt.Fprintf(&sb, "  %s%s: %s;\n", field.Name, optionalMarker, tsType)
		}
		sb.WriteString("}\n\n")
	}

	return sb.String()
}

// generateTypesTsForNamespace generates a types.ts file with TypeScript interfaces for structs and enums
// belonging to a single namespace. Used in multi-namespace mode.
// When inNamespaceSubdir is true, cross-namespace type references use '../{namespace}' imports.
func generateTypesTsForNamespace(nsTypes *NamespaceTypes, currentNs string, structMap map[string]*parser.Struct, enumMap map[string]*parser.Enum, inNamespaceSubdir bool, allNamespaceMap map[string]*NamespaceTypes, moduleStyle string) string {
	var sb strings.Builder

	sb.WriteString("// Generated by pulserpc - do not edit\n\n")
	sb.WriteString("// TypeScript interfaces and enums for all IDL types\n\n")

	// Collect cross-namespace imports needed by types in this namespace
	crossNsImports := collectCrossNamespaceImports(nsTypes, currentNs, allNamespaceMap)

	// Write cross-namespace imports if any
	if inNamespaceSubdir && len(crossNsImports) > 0 {
		importedNs := make([]string, 0, len(crossNsImports))
		for ns := range crossNsImports {
			importedNs = append(importedNs, ns)
		}
		// Sort for deterministic output
		sort.Strings(importedNs)
		for _, ns := range importedNs {
			importPath := tsImportPath(moduleStyle, tsCrossNamespaceImportPath("", ns))
			fmt.Fprintf(&sb, "import * as %s from '%s';\n", ns, importPath)
		}
		sb.WriteString("\n")
	}

	// Generate enums first (they may be used by structs)
	enumNames := make([]string, 0, len(enumMap))
	for name := range enumMap {
		enumNames = append(enumNames, name)
	}
	sort.Strings(enumNames)
	for _, enumName := range enumNames {
		enum := enumMap[enumName]
		comment := strings.TrimSpace(enum.Comment)
		if comment != "" {
			lines := strings.Split(comment, "\n")
			for _, line := range lines {
				fmt.Fprintf(&sb, "// %s\n", line)
			}
		}
		fmt.Fprintf(&sb, "export enum %s {\n", GetBaseName(enum.Name))
		for i, val := range enum.Values {
			valComment := strings.TrimSpace(val.Comment)
			if valComment != "" {
				lines := strings.Split(valComment, "\n")
				for _, line := range lines {
					fmt.Fprintf(&sb, "  // %s\n", line)
				}
			}
			fmt.Fprintf(&sb, "  %s = \"%s\"", val.Name, val.Name)
			if i < len(enum.Values)-1 {
				sb.WriteString(",\n")
			} else {
				sb.WriteString("\n")
			}
		}
		sb.WriteString("}\n\n")
	}

	// Generate structs (with extends support)
	structNames := make([]string, 0, len(structMap))
	for name := range structMap {
		structNames = append(structNames, name)
	}
	sort.Strings(structNames)
	for _, structName := range structNames {
		structDef := structMap[structName]
		comment := strings.TrimSpace(structDef.Comment)
		if comment != "" {
			lines := strings.Split(comment, "\n")
			for _, line := range lines {
				fmt.Fprintf(&sb, "// %s\n", line)
			}
		}
		baseName := GetBaseName(structDef.Name)
		fmt.Fprintf(&sb, "export interface %s", baseName)
		if structDef.Extends != "" {
			extendsRef := getExtendsTypeForNamespace(structDef.Extends, structMap, enumMap, currentNs, allNamespaceMap)
			sb.WriteString(" extends " + extendsRef)
		}
		sb.WriteString(" {\n")
		for _, field := range structDef.Fields {
			fieldComment := strings.TrimSpace(field.Comment)
			if fieldComment != "" {
				lines := strings.Split(fieldComment, "\n")
				for _, line := range lines {
					fmt.Fprintf(&sb, "  // %s\n", line)
				}
			}
			tsType := getTypeScriptTypeForNamespace(field.Type, structMap, enumMap, false, inNamespaceSubdir, allNamespaceMap)
			optionalMarker := ""
			if field.Optional {
				optionalMarker = "?"
			}
			fmt.Fprintf(&sb, "  %s%s: %s;\n", field.Name, optionalMarker, tsType)
		}
		sb.WriteString("}\n\n")
	}

	return sb.String()
}

// generateServerTsForNamespace generates the server.ts file with abstract interface classes
// for a single namespace. Used in multi-namespace mode.
func generateServerTsForNamespace(nsTypes *NamespaceTypes, structMap map[string]*parser.Struct, enumMap map[string]*parser.Enum, _ map[string]*parser.Interface, _ string, inNamespaceSubdir bool, allNamespaceMap map[string]*NamespaceTypes, moduleStyle string) string {
	var sb strings.Builder

	sb.WriteString("// Generated by pulserpc - do not edit\n\n")
	sb.WriteString("// Abstract service classes\n")
	sb.WriteString("// Implement these classes to create your service\n\n")
	runtimeImport := tsRuntimeImportPath(inNamespaceSubdir)
	fmt.Fprintf(&sb, "import { RPCError } from '%s';\n", tsImportPath(moduleStyle, runtimeImport+"/rpc"))
	fmt.Fprintf(&sb, "import * as types from '%s';\n\n", tsImportPath(moduleStyle, "./types"))

	currentNs := ""
	for ns := range allNamespaceMap {
		if allNamespaceMap[ns] == nsTypes {
			currentNs = ns
			break
		}
	}

	crossNsImports := collectCrossNamespaceImports(nsTypes, currentNs, allNamespaceMap)
	for crossNs := range crossNsImports {
		fmt.Fprintf(&sb, "import * as %s from '%s';\n", crossNs, tsImportPath(moduleStyle, "../"+crossNs+"/types"))
	}
	if len(crossNsImports) > 0 {
		sb.WriteString("\n")
	}

	ifaceNames := make([]string, 0, len(nsTypes.Interfaces))
	for _, iface := range nsTypes.Interfaces {
		ifaceNames = append(ifaceNames, iface.Name)
	}
	sort.Strings(ifaceNames)
	for _, ifaceName := range ifaceNames {
		var iface *parser.Interface
		for _, i := range nsTypes.Interfaces {
			if i.Name == ifaceName {
				iface = i
				break
			}
		}
		if iface != nil {
			writeInterfaceStubTsForNamespace(&sb, iface, structMap, enumMap, currentNs, allNamespaceMap)
		}
	}

	return sb.String()
}

// generateClientTsForNamespace generates the client.ts file with static typed client classes
// for a single namespace. Used in multi-namespace mode.
func generateClientTsForNamespace(nsTypes *NamespaceTypes, structMap map[string]*parser.Struct, enumMap map[string]*parser.Enum, _ string, inNamespaceSubdir bool, allNamespaceMap map[string]*NamespaceTypes, moduleStyle string) string {
	var sb strings.Builder

	sb.WriteString("// Generated by pulserpc - do not edit\n\n")
	sb.WriteString("// Static typed client stubs for each interface\n")
	sb.WriteString("// Use these for compile-time type safety\n\n")

	runtimeImport := tsRuntimeImportPath(inNamespaceSubdir)
	fmt.Fprintf(&sb, "import { Transport, HttpTransport } from '%s';\n", tsImportPath(moduleStyle, runtimeImport+"/transport"))
	fmt.Fprintf(&sb, "import { RPCError } from '%s';\n", tsImportPath(moduleStyle, runtimeImport+"/rpc"))
	fmt.Fprintf(&sb, "import * as types from '%s';\n\n", tsImportPath(moduleStyle, "./types"))

	currentNs := ""
	for ns := range allNamespaceMap {
		if allNamespaceMap[ns] == nsTypes {
			currentNs = ns
			break
		}
	}

	crossNsImports := collectCrossNamespaceImports(nsTypes, currentNs, allNamespaceMap)
	for crossNs := range crossNsImports {
		fmt.Fprintf(&sb, "import * as %s from '%s';\n", crossNs, tsImportPath(moduleStyle, "../"+crossNs+"/types"))
	}
	if len(crossNsImports) > 0 {
		sb.WriteString("\n")
	}

	sb.WriteString("export { Transport, HttpTransport };\n\n")

	ifaceNames := make([]string, 0, len(nsTypes.Interfaces))
	for _, iface := range nsTypes.Interfaces {
		ifaceNames = append(ifaceNames, iface.Name)
	}
	sort.Strings(ifaceNames)
	for _, ifaceName := range ifaceNames {
		var iface *parser.Interface
		for _, i := range nsTypes.Interfaces {
			if i.Name == ifaceName {
				iface = i
				break
			}
		}
		if iface != nil {
			writeInterfaceClientTs(&sb, iface, structMap, enumMap, currentNs, allNamespaceMap)
		}
	}

	return sb.String()
}

// generateClientTs generates the client.ts file with static typed client classes
func generateClientTs(idl *parser.IDL, structMap map[string]*parser.Struct, enumMap map[string]*parser.Enum, _ string, _ map[string]*NamespaceTypes, moduleStyle string) string {
	var sb strings.Builder

	sb.WriteString("// Generated by pulserpc - do not edit\n\n")
	sb.WriteString("// Static typed client stubs for each interface\n")
	sb.WriteString("// Use these for compile-time type safety\n\n")

	fmt.Fprintf(&sb, "import { Transport, HttpTransport } from '%s';\n", tsImportPath(moduleStyle, "./pulserpc/transport"))
	fmt.Fprintf(&sb, "import { RPCError } from '%s';\n", tsImportPath(moduleStyle, "./pulserpc/rpc"))
	fmt.Fprintf(&sb, "import * as types from '%s';\n\n", tsImportPath(moduleStyle, "./types"))

	sb.WriteString("export { Transport, HttpTransport };\n\n")

	ifaceNames := make([]string, 0, len(idl.Interfaces))
	for _, iface := range idl.Interfaces {
		ifaceNames = append(ifaceNames, iface.Name)
	}
	sort.Strings(ifaceNames)
	for _, ifaceName := range ifaceNames {
		var iface *parser.Interface
		for _, i := range idl.Interfaces {
			if i.Name == ifaceName {
				iface = i
				break
			}
		}
		if iface != nil {
			writeInterfaceClientTs(&sb, iface, structMap, enumMap, "", nil)
		}
	}

	return sb.String()
}

// writeClientMethodTs generates a typed method for a TypeScript client class
func writeClientMethodTs(sb *strings.Builder, iface *parser.Interface, method *parser.Method, structMap map[string]*parser.Struct, enumMap map[string]*parser.Enum, ns string, allNamespaceMap map[string]*NamespaceTypes) {
	methodName := method.Name
	fmt.Fprintf(sb, "  async %s(", methodName)

	// Parameters
	for i, param := range method.Parameters {
		if i > 0 {
			sb.WriteString(", ")
		}
		tsType := getTypeScriptTypeForNamespace(param.Type, structMap, enumMap, true, ns != "", allNamespaceMap)
		fmt.Fprintf(sb, "%s: %s", param.Name, tsType)
	}
	sb.WriteString(")")

	// Return type
	if method.ReturnType != nil {
		returnType := getTypeScriptTypeForNamespace(method.ReturnType, structMap, enumMap, true, ns != "", allNamespaceMap)
		if method.ReturnOptional {
			fmt.Fprintf(sb, ": Promise<%s | null> {\n", returnType)
		} else {
			fmt.Fprintf(sb, ": Promise<%s> {\n", returnType)
		}
	} else {
		sb.WriteString(": Promise<void> {\n")
	}

	// Build request
	fmt.Fprintf(sb, "    const _req = {\n")
	sb.WriteString("      jsonrpc: \"2.0\" as const,\n")
	fmt.Fprintf(sb, "      method: \"%s.%s\",\n", iface.Name, method.Name)
	if len(method.Parameters) > 0 {
		sb.WriteString("      params: [")
		for i, param := range method.Parameters {
			if i > 0 {
				sb.WriteString(", ")
			}
			sb.WriteString(param.Name)
		}
		sb.WriteString("],\n")
	}
	sb.WriteString("    };\n")

	sb.WriteString("    const _resp = await this.transport.request(_req as any);\n")
	sb.WriteString("    if (_resp.error) {\n")
	sb.WriteString("      throw new RPCError(_resp.error.code, _resp.error.message, _resp.error.data);\n")
	sb.WriteString("    }\n")

	if method.ReturnType != nil {
		if method.ReturnOptional {
			sb.WriteString("    return _resp.result as ")
			returnType := getTypeScriptTypeForNamespace(method.ReturnType, structMap, enumMap, true, ns != "", allNamespaceMap)
			fmt.Fprintf(sb, "%s | null;\n", returnType)
		} else {
			sb.WriteString("    return _resp.result as ")
			returnType := getTypeScriptTypeForNamespace(method.ReturnType, structMap, enumMap, true, ns != "", allNamespaceMap)
			fmt.Fprintf(sb, "%s;\n", returnType)
		}
	} else {
		sb.WriteString("    return;\n")
	}

	sb.WriteString("  }\n\n")
}

// writeInterfaceClientTs generates a client class for a TypeScript interface
func writeInterfaceClientTs(sb *strings.Builder, iface *parser.Interface, structMap map[string]*parser.Struct, enumMap map[string]*parser.Enum, ns string, allNamespaceMap map[string]*NamespaceTypes) {
	if iface.Comment != "" {
		lines := strings.Split(strings.TrimSpace(iface.Comment), "\n")
		for _, line := range lines {
			fmt.Fprintf(sb, "// %s\n", line)
		}
	}

	clientName := iface.Name + "Client"
	fmt.Fprintf(sb, "export class %s {\n", clientName)
	sb.WriteString("  constructor(private transport: Transport) {}\n\n")

	// Generate methods
	for _, method := range iface.Methods {
		writeClientMethodTs(sb, iface, method, structMap, enumMap, ns, allNamespaceMap)
	}

	sb.WriteString("}\n\n")
}

// generateNamespaceIndexTs writes an index.ts file to the namespace subdirectory
// that re-exports from types.ts, server.ts, and client.ts. The import suffix
// is style-aware: ".js" for ESM variants, none for CJS.
func (p *TSClientServer) generateNamespaceIndexTs(paths TSNamespacePaths, namespace string) error {
	moduleStyle := p.moduleStyle
	nsDir := paths.ResolveNamespaceDir(namespace)
	indexContent := fmt.Sprintf(
		"export * from '%s';\nexport * from '%s';\nexport * from '%s';\n",
		tsImportPath(moduleStyle, "./types"),
		tsImportPath(moduleStyle, "./server"),
		tsImportPath(moduleStyle, "./client"),
	)
	indexPath := filepath.Join(nsDir, "index.ts")
	// Apply the active module-style transform so that, under esm-bundler,
	// the .js suffix is stripped from the re-exports; under cjs, the
	// `export * from` statements are rewritten to module.exports re-assign.
	transformed := p.transformFileForStyle([]byte(indexContent), moduleStyle)
	if err := os.WriteFile(indexPath, transformed, 0644); err != nil {
		return fmt.Errorf("failed to write %s/index.ts: %w", namespace, err)
	}
	return nil
}

// generateTestServerTs generates test_server.ts with concrete implementations of all interfaces
// When entryPointNs is provided, the file will be placed in that namespace subdirectory,
// so imports are adjusted accordingly.
func generateTestServerTs(idl *parser.IDL, structMap map[string]*parser.Struct, enumMap map[string]*parser.Enum, _ map[string]*parser.Interface, _ string, _ map[string]*NamespaceTypes, entryPointNs string, moduleStyle string) string {
	var sb strings.Builder

	// Determine if we're generating in a namespace subdirectory
	inNsSubdir := entryPointNs != ""
	// Runtime import path: '../pulserpc' from namespace subdir, './pulserpc' from root
	runtimeImport := "./pulserpc"
	if inNsSubdir {
		runtimeImport = "../pulserpc"
	}

	sb.WriteString("// Generated by pulserpc - do not edit\n")
	sb.WriteString("// Test server implementation for integration testing\n\n")
	sb.WriteString("import { readFileSync } from 'fs';\n")
	fmt.Fprintf(&sb, "import { Server, Contract } from '%s';\n", tsImportPath(moduleStyle, runtimeImport))

	for _, iface := range idl.Interfaces {
		ns := GetNamespaceFromType(iface.Name, iface.Namespace)
		if ns != "" {
			if inNsSubdir && ns == entryPointNs {
				// Interface is in the same namespace as where we're placing the test file
				fmt.Fprintf(&sb, "import { %s } from '%s';\n", iface.Name, tsImportPath(moduleStyle, "./server"))
			} else if inNsSubdir {
				// Interface is in a different namespace - go up and into that namespace
				fmt.Fprintf(&sb, "import { %s } from '%s';\n", iface.Name, tsImportPath(moduleStyle, "../"+ns+"/server"))
			} else {
				// File is at root - standard path
				fmt.Fprintf(&sb, "import { %s } from '%s';\n", iface.Name, tsImportPath(moduleStyle, "./"+ns+"/server"))
			}
		} else {
			fmt.Fprintf(&sb, "import { %s } from '%s';\n", iface.Name, tsImportPath(moduleStyle, "./server"))
		}
	}
	sb.WriteString("\n")

	// Generate implementation classes for each interface
	for _, iface := range idl.Interfaces {
		writeTestInterfaceImplTs(&sb, iface, structMap, enumMap)
	}

	// Generate main entry point
	sb.WriteString("// Load IDL and create Contract\n")
	sb.WriteString("const idlData = JSON.parse(readFileSync('idl.json', 'utf-8'));\n")
	sb.WriteString("const contract = new Contract(idlData);\n\n")
	sb.WriteString("// Create Server instance\n")
	sb.WriteString("const rpcServer = new Server({ contract, validateRequests: true, validateResponses: true });\n")
	for _, iface := range idl.Interfaces {
		fmt.Fprintf(&sb, "rpcServer.addHandler(\"%s\", new %sImpl());\n", iface.Name, iface.Name)
	}

	// Generate HTTP server handler
	sb.WriteString("\n")
	sb.WriteString("// HTTP server\n")
	sb.WriteString("import * as http from 'http';\n\n")
	sb.WriteString("class TestRPCHandler {\n")
	sb.WriteString("  private rpcServer: Server;\n\n")
	sb.WriteString("  constructor(rpcServer: Server) {\n")
	sb.WriteString("    this.rpcServer = rpcServer;\n")
	sb.WriteString("  }\n\n")
	sb.WriteString("  handle(req: http.IncomingMessage, res: http.ServerResponse): void {\n")
	sb.WriteString("    let body = '';\n")
	sb.WriteString("    req.on('data', (chunk) => { body += chunk.toString(); });\n")
	sb.WriteString("    req.on('end', async () => {\n")
	sb.WriteString("      try {\n")
	sb.WriteString("        const data = JSON.parse(body);\n")
	sb.WriteString("        const response = await this.rpcServer.call(data);\n")
	sb.WriteString("        if (response === null || response === undefined) {\n")
	sb.WriteString("          res.writeHead(204);\n")
	sb.WriteString("          res.end();\n")
	sb.WriteString("        } else {\n")
	sb.WriteString("          res.writeHead(200, { 'Content-Type': 'application/json' });\n")
	sb.WriteString("          res.end(JSON.stringify(response));\n")
	sb.WriteString("        }\n")
	sb.WriteString("      } catch (err: any) {\n")
	sb.WriteString("        const errorResponse = {\n")
	sb.WriteString("          jsonrpc: '2.0',\n")
	sb.WriteString("          error: { code: -32700, message: `Parse error: ${err.message}` },\n")
	sb.WriteString("          id: null,\n")
	sb.WriteString("        };\n")
	sb.WriteString("        res.writeHead(200, { 'Content-Type': 'application/json' });\n")
	sb.WriteString("        res.end(JSON.stringify(errorResponse));\n")
	sb.WriteString("      }\n")
	sb.WriteString("    });\n")
	sb.WriteString("  }\n")
	sb.WriteString("}\n\n")
	sb.WriteString("const handler = new TestRPCHandler(rpcServer);\n")
	sb.WriteString("const httpServer = http.createServer((req, res) => {\n")
	sb.WriteString("  if (req.method === 'POST') {\n")
	sb.WriteString("    handler.handle(req, res);\n")
	sb.WriteString("  } else {\n")
	sb.WriteString("    res.writeHead(405, { 'Content-Type': 'application/json' });\n")
	sb.WriteString("    res.end(JSON.stringify({ error: 'Method Not Allowed' }));\n")
	sb.WriteString("  }\n")
	sb.WriteString("});\n\n")
	sb.WriteString("httpServer.listen(8080, '0.0.0.0', () => {\n")
	sb.WriteString("  console.log('PulseRPC test server listening on http://0.0.0.0:8080');\n")
	sb.WriteString("});\n")

	return sb.String()
}

// writeTestInterfaceImplTs generates a test implementation class for an interface
func writeTestInterfaceImplTs(sb *strings.Builder, iface *parser.Interface, structMap map[string]*parser.Struct, enumMap map[string]*parser.Enum) {
	fmt.Fprintf(sb, "class %sImpl extends %s {\n", iface.Name, iface.Name)
	fmt.Fprintf(sb, "  // Test implementation of %s interface\n\n", iface.Name)

	// Generate method implementations
	for _, method := range iface.Methods {
		writeTestMethodImplTs(sb, iface, method, structMap, enumMap)
	}
	sb.WriteString("}\n\n")
}

// writeTestMethodImplTs generates a test implementation for a method
func writeTestMethodImplTs(sb *strings.Builder, iface *parser.Interface, method *parser.Method, structMap map[string]*parser.Struct, enumMap map[string]*parser.Enum) {
	// Method signature
	fmt.Fprintf(sb, "  %s(", method.Name)
	fmt.Fprintf(sb, "ctx: any")
	hasParams := len(method.Parameters) > 0
	if hasParams {
		sb.WriteString(", ")
	}
	for i, param := range method.Parameters {
		if i > 0 {
			sb.WriteString(", ")
		}
		fmt.Fprintf(sb, "%s: any", param.Name)
	}
	sb.WriteString("): any {\n")

	// Special handling for known test cases
	if iface.Name == "B" && method.Name == "echo" {
		sb.WriteString("    // Handle optional return: return null if s === 'return-null'\n")
		sb.WriteString("    if (s === 'return-null') {\n")
		sb.WriteString("      return null;\n")
		sb.WriteString("    }\n")
		sb.WriteString("    return s;\n")
		sb.WriteString("  }\n\n")
		return
	}

	// Generate based on method name patterns
	methodNameLower := strings.ToLower(method.Name)
	switch methodNameLower {
	case "add":
		sb.WriteString("    // returns a+b\n")
		sb.WriteString("    return a + b;\n")
	case "sqrt":
		sb.WriteString("    // returns the square root of a\n")
		sb.WriteString("    return globalThis.Math.sqrt(a);\n")
	case "calc":
		sb.WriteString("    // performs the given operation against all the values in nums and returns the result\n")
		sb.WriteString("    if (!nums || nums.length === 0) {\n")
		sb.WriteString("      return 0.0;\n")
		sb.WriteString("    }\n")
		sb.WriteString("    if (operation === 'add') {\n")
		sb.WriteString("      return nums.reduce((sum, num) => sum + num, 0.0);\n")
		sb.WriteString("    } else if (operation === 'multiply') {\n")
		sb.WriteString("      return nums.reduce((prod, num) => prod * num, 1.0);\n")
		sb.WriteString("    } else {\n")
		sb.WriteString("      return 0.0;\n")
		sb.WriteString("    }\n")
	case "repeat":
		sb.WriteString("    // Echos the req1.to_repeat string as a list, optionally forcing to_repeat to upper case\n")
		sb.WriteString("    // RepeatResponse.items should be a list of strings whose length is equal to req1.count\n")
		sb.WriteString("    const text = req1.to_repeat || '';\n")
		sb.WriteString("    const count = req1.count || 0;\n")
		sb.WriteString("    const forceUppercase = req1.force_uppercase || false;\n")
		sb.WriteString("    const finalText = forceUppercase ? text.toUpperCase() : text;\n")
		sb.WriteString("    const items: string[] = [];\n")
		sb.WriteString("    for (let i = 0; i < count; i++) {\n")
		sb.WriteString("      items.push(finalText);\n")
		sb.WriteString("    }\n")
		sb.WriteString("    return {\n")
		sb.WriteString("      status: 'ok',\n")
		sb.WriteString("      count: count,\n")
		sb.WriteString("      items: items,\n")
		sb.WriteString("    };\n")
	case "say_hi":
		sb.WriteString("    // returns a result with: hi='hi' and status='ok'\n")
		sb.WriteString("    return {\n")
		sb.WriteString("      hi: 'hi',\n")
		sb.WriteString("    };\n")
	case "repeat_num":
		sb.WriteString("    // returns num as an array repeated 'count' number of times\n")
		sb.WriteString("    const result: number[] = [];\n")
		sb.WriteString("    for (let i = 0; i < count; i++) {\n")
		sb.WriteString("      result.push(num);\n")
		sb.WriteString("    }\n")
		sb.WriteString("    return result;\n")
	case "putperson":
		sb.WriteString("    // simply returns p.personId\n")
		sb.WriteString("    // we use this to test the '[optional]' enforcement, as we invoke it with a null email\n")
		sb.WriteString("    return p.personId;\n")
	default:
		// Default implementation: return appropriate type based on return type
		writeDefaultTestReturnTs(sb, method.ReturnType, structMap, enumMap)
	}
	sb.WriteString("  }\n\n")
}

// writeDefaultTestReturnTs generates a default return value for a type
func writeDefaultTestReturnTs(sb *strings.Builder, returnType *parser.Type, structMap map[string]*parser.Struct, enumMap map[string]*parser.Enum) {
	if returnType.IsBuiltIn() {
		switch returnType.BuiltIn {
		case "string":
			sb.WriteString("    return '';\n")
		case "int":
			sb.WriteString("    return 0;\n")
		case "float":
			sb.WriteString("    return 0.0;\n")
		case "bool":
			sb.WriteString("    return false;\n")
		default:
			sb.WriteString("    return null;\n")
		}
	} else if returnType.IsArray() {
		sb.WriteString("    return [];\n")
	} else if returnType.IsMap() {
		sb.WriteString("    return {};\n")
	} else if returnType.IsUserDefined() {
		// Check if it's a struct
		if structMap[returnType.UserDefined] != nil {
			s := structMap[returnType.UserDefined]
			sb.WriteString("    return {\n")
			// Handle inheritance - get all fields including parent
			for _, field := range s.Fields {
				if field.Optional {
					continue // Skip optional fields in default return
				}
				fmt.Fprintf(sb, "      %s: ", field.Name)
				writeDefaultTestValueTs(sb, field.Type, structMap, enumMap)
				sb.WriteString(",\n")
			}
			// If extends, add parent fields
			if s.Extends != "" {
				baseName := s.Extends
				// First try looking up with full name (including namespace)
				baseStruct := structMap[baseName]
				// If not found and has a namespace prefix, try with just the base name
				if baseStruct == nil && strings.Contains(baseName, ".") {
					parts := strings.Split(baseName, ".")
					baseName = parts[len(parts)-1]
					baseStruct = structMap[baseName]
				}
				// If we found the parent struct, add its fields
				if baseStruct != nil {
					for _, field := range baseStruct.Fields {
						if field.Optional {
							continue
						}
						fmt.Fprintf(sb, "      %s: ", field.Name)
						writeDefaultTestValueTs(sb, field.Type, structMap, enumMap)
						sb.WriteString(",\n")
					}
				}
			}
			sb.WriteString("    };\n")
		} else if enumMap[returnType.UserDefined] != nil {
			// Return first enum value
			e := enumMap[returnType.UserDefined]
			if len(e.Values) > 0 {
				fmt.Fprintf(sb, "    return '%s';\n", e.Values[0].Name)
			} else {
				sb.WriteString("    return null;\n")
			}
		} else {
			sb.WriteString("    return null;\n")
		}
	} else {
		sb.WriteString("    return null;\n")
	}
}

// writeDefaultTestValueTs generates a default value for a type (used in structs)
func writeDefaultTestValueTs(sb *strings.Builder, t *parser.Type, structMap map[string]*parser.Struct, enumMap map[string]*parser.Enum) {
	if t.IsBuiltIn() {
		switch t.BuiltIn {
		case "string":
			sb.WriteString("''")
		case "int":
			sb.WriteString("0")
		case "float":
			sb.WriteString("0.0")
		case "bool":
			sb.WriteString("false")
		default:
			sb.WriteString("null")
		}
	} else if t.IsArray() {
		sb.WriteString("[]")
	} else if t.IsMap() {
		sb.WriteString("{}")
	} else if t.IsUserDefined() {
		if structMap[t.UserDefined] != nil {
			sb.WriteString("{}")
		} else {
			// Try to find enum
			e := enumMap[t.UserDefined]
			// If not found with exact name, try to find by base name
			// (e.g., 'Status' might be registered as 'inc.Status')
			if e == nil {
				for enumKey, enumVal := range enumMap {
					if strings.HasSuffix(enumKey, "."+t.UserDefined) || enumKey == t.UserDefined {
						e = enumVal
						break
					}
				}
			}
			if e != nil {
				if len(e.Values) > 0 {
					fmt.Fprintf(sb, "'%s'", e.Values[0].Name)
				} else {
					sb.WriteString("null")
				}
			} else {
				sb.WriteString("null")
			}
		}
	} else {
		sb.WriteString("null")
	}
}

// generateTestClientTs generates test_client.ts that exercises all client methods
// When entryPointNs is provided, the file will be placed in that namespace subdirectory,
// so imports are adjusted accordingly.
func generateTestClientTs(idl *parser.IDL, structMap map[string]*parser.Struct, enumMap map[string]*parser.Enum, _ map[string]*parser.Interface, _ string, _ map[string]*NamespaceTypes, entryPointNs string, moduleStyle string) string {
	var sb strings.Builder

	// Determine if we're generating in a namespace subdirectory
	inNsSubdir := entryPointNs != ""
	// Runtime import path: '../pulserpc' from namespace subdir, './pulserpc' from root
	runtimeImport := "./pulserpc"
	if inNsSubdir {
		runtimeImport = "../pulserpc"
	}

	sb.WriteString("// Generated by pulserpc - do not edit\n")
	sb.WriteString("// Test client for integration testing\n\n")
	fmt.Fprintf(&sb, "import { HttpTransport, Client } from '%s';\n\n", tsImportPath(moduleStyle, runtimeImport))

	// Generate wait for server function
	sb.WriteString("async function waitForServer(url: string, timeout: number = 10000): Promise<boolean> {\n")
	sb.WriteString("  const startTime = Date.now();\n")
	sb.WriteString("  let retryDelay = 200;\n\n")
	sb.WriteString("  while (Date.now() - startTime < timeout) {\n")
	sb.WriteString("    try {\n")
	sb.WriteString("      const controller = new AbortController();\n")
	sb.WriteString("      const timeoutId = setTimeout(() => controller.abort(), 2000);\n")
	sb.WriteString("      const response = await fetch(url, {\n")
	sb.WriteString("        method: 'POST',\n")
	sb.WriteString("        headers: { 'Content-Type': 'application/json' },\n")
	sb.WriteString("        body: '{\"jsonrpc\":\"2.0\",\"method\":\"pulserpc-idl\",\"id\":1}',\n")
	sb.WriteString("        signal: controller.signal,\n")
	sb.WriteString("      });\n")
	sb.WriteString("      clearTimeout(timeoutId);\n")
	sb.WriteString("      if (response.ok) {\n")
	sb.WriteString("        return true;\n")
	sb.WriteString("      }\n")
	sb.WriteString("    } catch (err: any) {\n")
	sb.WriteString("      // Connection error - server not ready yet\n")
	sb.WriteString("    }\n")
	sb.WriteString("    await new Promise(resolve => setTimeout(resolve, retryDelay));\n")
	sb.WriteString("    retryDelay = Math.min(retryDelay * 1.5, 1000);\n")
	sb.WriteString("  }\n")
	sb.WriteString("  return false;\n")
	sb.WriteString("}\n\n")

	// Generate main test function
	sb.WriteString("async function main() {\n")
	sb.WriteString("  const serverUrl = 'http://localhost:8080';\n\n")
	sb.WriteString("  // Wait for server to be ready\n")
	sb.WriteString("  if (!(await waitForServer(serverUrl, 10000))) {\n")
	sb.WriteString("    console.error('ERROR: Server did not become ready in time');\n")
	sb.WriteString("    process.exit(1);\n")
	sb.WriteString("  }\n\n")
	sb.WriteString("  console.log('Server is ready. Running tests...');\n")
	sb.WriteString("  console.log();\n\n")

	sb.WriteString("  // Create client - interfaces are auto-discovered\n")
	sb.WriteString("  const transport = new HttpTransport(serverUrl);\n")
	sb.WriteString("  const client = new Client(transport);\n")
	sb.WriteString("  await client.ready();\n\n")
	sb.WriteString("  const errors: string[] = [];\n\n")

	// Generate test cases for each method using dynamic proxy pattern
	for _, iface := range idl.Interfaces {
		for _, method := range iface.Methods {
			writeTestClientCallTs(&sb, iface, method, "client", structMap, enumMap)
		}
	}

	sb.WriteString("  // Report results\n")
	sb.WriteString("  console.log();\n")
	sb.WriteString("  if (errors.length > 0) {\n")
	sb.WriteString("    console.error(`FAILED: ${errors.length} test(s) failed:`);\n")
	sb.WriteString("    for (const error of errors) {\n")
	sb.WriteString("      console.error(`  - ${error}`);\n")
	sb.WriteString("    }\n")
	sb.WriteString("    process.exit(1);\n")
	sb.WriteString("  } else {\n")
	sb.WriteString("    console.log('SUCCESS: All tests passed!');\n")
	sb.WriteString("    process.exit(0);\n")
	sb.WriteString("  }\n")
	sb.WriteString("}\n\n")

	sb.WriteString("main().catch((err) => {\n")
	sb.WriteString("  console.error('Fatal error:', err);\n")
	sb.WriteString("  process.exit(1);\n")
	sb.WriteString("});\n")

	return sb.String()
}

// writeTestClientCallTs generates a test call for a method
func writeTestClientCallTs(sb *strings.Builder, iface *parser.Interface, method *parser.Method, clientVar string, structMap map[string]*parser.Struct, enumMap map[string]*parser.Enum) {
	testName := fmt.Sprintf("%s.%s", iface.Name, method.Name)
	fmt.Fprintf(sb, "  // Test %s\n", testName)
	sb.WriteString("  try {\n")

	// Generate test parameters based on method signature
	params := make([]string, 0)
	for _, param := range method.Parameters {
		paramValue := generateTestParamValueTs(param.Type, param.Name, structMap, enumMap)
		params = append(params, paramValue)
	}

	// Generate method call using dynamic proxy pattern: client.InterfaceName.methodName()
	if len(params) > 0 {
		fmt.Fprintf(sb, "    const result = await %s.%s.%s(%s);\n", clientVar, iface.Name, method.Name, strings.Join(params, ", "))
	} else {
		fmt.Fprintf(sb, "    const result = await %s.%s.%s();\n", clientVar, iface.Name, method.Name)
	}

	// Generate assertions based on method
	methodNameLower := strings.ToLower(method.Name)
	if iface.Name == "B" && method.Name == "echo" {
		sb.WriteString("    // Test normal return\n")
		sb.WriteString("    if (result !== 'test') {\n")
		sb.WriteString("      throw new Error(`Expected 'test', got ${result}`);\n")
		sb.WriteString("    }\n")
		sb.WriteString("    // Test null return\n")
		fmt.Fprintf(sb, "    const resultNull = await %s.%s.echo('return-null');\n", clientVar, iface.Name)
		sb.WriteString("    if (resultNull !== null) {\n")
		sb.WriteString("      throw new Error(`Expected null, got ${resultNull}`);\n")
		sb.WriteString("    }\n")
	} else if methodNameLower == "add" {
		sb.WriteString("    if (result !== 5) {\n")
		sb.WriteString("      throw new Error(`Expected 5, got ${result}`);\n")
		sb.WriteString("    }\n")
	} else if methodNameLower == "sqrt" {
		sb.WriteString("    if (globalThis.Math.abs(result - 2.0) >= 0.001) {\n")
		sb.WriteString("      throw new Error(`Expected ~2.0, got ${result}`);\n")
		sb.WriteString("    }\n")
	} else if methodNameLower == "calc" {
		sb.WriteString("    if (typeof result !== 'number') {\n")
		sb.WriteString("      throw new Error(`Expected number, got ${typeof result}`);\n")
		sb.WriteString("    }\n")
	} else if methodNameLower == "repeat" {
		sb.WriteString("    if (typeof result !== 'object' || !result) {\n")
		sb.WriteString("      throw new Error(`Expected object, got ${typeof result}`);\n")
		sb.WriteString("    }\n")
		sb.WriteString("    if (!('items' in result)) {\n")
		sb.WriteString("      throw new Error(\"Result missing 'items' field\");\n")
		sb.WriteString("    }\n")
		sb.WriteString("    if (result.items.length !== 3) {\n")
		sb.WriteString("      throw new Error(`Expected 3 items, got ${result.items.length}`);\n")
		sb.WriteString("    }\n")
	} else if methodNameLower == "say_hi" {
		sb.WriteString("    if (typeof result !== 'object' || !result) {\n")
		sb.WriteString("      throw new Error(`Expected object, got ${typeof result}`);\n")
		sb.WriteString("    }\n")
		sb.WriteString("    if (result.hi !== 'hi') {\n")
		sb.WriteString("      throw new Error(`Expected hi='hi', got ${JSON.stringify(result)}`);\n")
		sb.WriteString("    }\n")
	} else if methodNameLower == "repeat_num" {
		sb.WriteString("    if (!Array.isArray(result)) {\n")
		sb.WriteString("      throw new Error(`Expected array, got ${typeof result}`);\n")
		sb.WriteString("    }\n")
		sb.WriteString("    if (result.length !== 2) {\n")
		sb.WriteString("      throw new Error(`Expected 2 items, got ${result.length}`);\n")
		sb.WriteString("    }\n")
	} else if methodNameLower == "putperson" {
		sb.WriteString("    if (typeof result !== 'string') {\n")
		sb.WriteString("      throw new Error(`Expected string, got ${typeof result}`);\n")
		sb.WriteString("    }\n")
		sb.WriteString("    if (result !== 'person123') {\n")
		sb.WriteString("      throw new Error(`Expected 'person123', got ${result}`);\n")
		sb.WriteString("    }\n")
	} else {
		// Generic assertion - just check that we got a result
		sb.WriteString("    if (result === null || result === undefined) {\n")
		sb.WriteString("      throw new Error('Expected non-null result');\n")
		sb.WriteString("    }\n")
	}

	fmt.Fprintf(sb, "    console.log('✓ %s passed');\n", testName)
	sb.WriteString("  } catch (err: any) {\n")
	fmt.Fprintf(sb, "    const errorMsg = `%s failed: ${err.message || err}`;\n", testName)
	sb.WriteString("    errors.push(errorMsg);\n")
	fmt.Fprintf(sb, "    console.error(`✗ ${errorMsg}`);\n")
	sb.WriteString("  }\n")
	sb.WriteString("\n")
}

// generateTestParamValueTs generates a test parameter value for a type
func generateTestParamValueTs(t *parser.Type, paramName string, structMap map[string]*parser.Struct, enumMap map[string]*parser.Enum) string {
	if t.IsBuiltIn() {
		switch t.BuiltIn {
		case "string":
			if paramName == "s" {
				return "'test'"
			}
			return "'test'"
		case "int":
			switch paramName {
			case "a", "num":
				return "2"
			case "b":
				return "3"
			case "count":
				return "2"
			default:
				return "1"
			}
		case "float":
			if paramName == "a" {
				return "4.0"
			}
			return "1.0"
		case "bool":
			return "true"
		default:
			return "null"
		}
	} else if t.IsArray() {
		if t.Array.IsBuiltIn() && t.Array.BuiltIn == "float" {
			return "[1.0, 2.0, 3.0]"
		}
		return "[]"
	} else if t.IsMap() {
		return "{}"
	} else if t.IsUserDefined() {
		// Check if it's a struct
		if structMap[t.UserDefined] != nil {
			s := structMap[t.UserDefined]
			// Build struct object
			fields := []string{}
			for _, field := range s.Fields {
				if field.Optional && field.Name == "email" {
					// Special case: set email to null for putPerson test
					fields = append(fields, fmt.Sprintf("%s: null", field.Name))
				} else if !field.Optional {
					fieldValue := generateTestParamValueTs(field.Type, field.Name, structMap, enumMap)
					fields = append(fields, fmt.Sprintf("%s: %s", field.Name, fieldValue))
				}
			}
			// Handle inheritance
			if s.Extends != "" {
				baseName := s.Extends
				// First try looking up with full name (including namespace)
				baseStruct := structMap[baseName]
				// If not found and has a namespace prefix, try with just the base name
				if baseStruct == nil && strings.Contains(baseName, ".") {
					parts := strings.Split(baseName, ".")
					baseName = parts[len(parts)-1]
					baseStruct = structMap[baseName]
				}
				// If we found the parent struct, add its fields
				if baseStruct != nil {
					for _, field := range baseStruct.Fields {
						if !field.Optional {
							fieldValue := generateTestParamValueTs(field.Type, field.Name, structMap, enumMap)
							fields = append(fields, fmt.Sprintf("%s: %s", field.Name, fieldValue))
						}
					}
				}
			}
			// Special handling for RepeatRequest
			if t.UserDefined == "RepeatRequest" {
				return "{ to_repeat: 'hello', count: 3, force_uppercase: false }"
			}
			// Special handling for Person
			if t.UserDefined == "Person" {
				return "{ personId: 'person123', firstName: 'John', lastName: 'Doe', email: null }"
			}
			return "{ " + strings.Join(fields, ", ") + " }"
		} else if enumMap[t.UserDefined] != nil {
			e := enumMap[t.UserDefined]
			if len(e.Values) > 0 {
				// Special case for MathOp
				if t.UserDefined == "inc.MathOp" || strings.HasSuffix(t.UserDefined, "MathOp") {
					return "'add'"
				}
				return fmt.Sprintf("'%s'", e.Values[0].Name)
			}
			return "null"
		}
		return "null"
	}
	return "null"
}
