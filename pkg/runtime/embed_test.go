package runtime

import (
	"bytes"
	"sort"
	"strings"
	"testing"
)

// TestGetRuntimeFilesForStyle covers the matrix required by step 3.6 of the
// plan: legacy behaviour, CJS, and unknown-style errors.
func TestGetRuntimeFilesForStyle(t *testing.T) {
	// Legacy call: GetRuntimeFiles("ts") must remain the canonical source of
	// the ESM-Node tree.
	legacy, err := GetRuntimeFiles("ts")
	if err != nil {
		t.Fatalf("GetRuntimeFiles(\"ts\") failed: %v", err)
	}

	// 1. esm-node is the canonical form of the legacy default.
	esmNode, err := GetRuntimeFilesForStyle("ts", "esm-node")
	if err != nil {
		t.Fatalf("GetRuntimeFilesForStyle(\"ts\", \"esm-node\") failed: %v", err)
	}
	if !sameFileSet(legacy, esmNode) {
		t.Errorf("GetRuntimeFilesForStyle(\"ts\", \"esm-node\") must be byte-equal to GetRuntimeFiles(\"ts\")")
	}
	for name, data := range legacy {
		if !bytes.Equal(esmNode[name], data) {
			t.Errorf("esm-node file %q differs from legacy GetRuntimeFiles(\"ts\")", name)
		}
	}

	// 2. Empty style is equivalent to esm-node.
	empty, err := GetRuntimeFilesForStyle("ts", "")
	if err != nil {
		t.Fatalf("GetRuntimeFilesForStyle(\"ts\", \"\") failed: %v", err)
	}
	if !sameFileSet(legacy, empty) {
		t.Errorf("empty style must match legacy GetRuntimeFiles(\"ts\")")
	}

	// 3. Aliases all map to esm-node.
	for _, alias := range []string{"esm", "node"} {
		got, err := GetRuntimeFilesForStyle("ts", alias)
		if err != nil {
			t.Errorf("GetRuntimeFilesForStyle(\"ts\", %q) returned error: %v", alias, err)
			continue
		}
		if !sameFileSet(legacy, got) {
			t.Errorf("alias %q must match legacy GetRuntimeFiles(\"ts\")", alias)
		}
	}

	// 4. esm-bundler shares the ts-node tree (the generator applies the
	//    bundler transform at a later step).
	bundler, err := GetRuntimeFilesForStyle("ts", "esm-bundler")
	if err != nil {
		t.Fatalf("GetRuntimeFilesForStyle(\"ts\", \"esm-bundler\") failed: %v", err)
	}
	if !sameFileSet(legacy, bundler) {
		t.Errorf("esm-bundler runtime tree must equal esm-node at the embed level")
	}
	bundlerAlias, err := GetRuntimeFilesForStyle("ts", "bundler")
	if err != nil {
		t.Errorf("GetRuntimeFilesForStyle(\"ts\", \"bundler\") returned error: %v", err)
	}
	if !sameFileSet(legacy, bundlerAlias) {
		t.Errorf("bundler alias must match legacy GetRuntimeFiles(\"ts\")")
	}

	// 5. cjs uses the ts-cjs tree.
	cjs, err := GetRuntimeFilesForStyle("ts", "cjs")
	if err != nil {
		t.Fatalf("GetRuntimeFilesForStyle(\"ts\", \"cjs\") failed: %v", err)
	}
	if len(cjs) == 0 {
		t.Fatalf("GetRuntimeFilesForStyle(\"ts\", \"cjs\") returned an empty map")
	}
	for _, name := range []string{"client.ts", "server.ts", "index.ts"} {
		if _, ok := cjs[name]; !ok {
			t.Errorf("cjs runtime is missing expected file %q", name)
		}
	}
	// The CJS variant must use a CJS-compatible idiom for IDL discovery —
	// either __dirname (current value) or __filename — and must not use
	// ESM "import" statements at the top level.
	clientData := cjs["client.ts"]
	if !bytes.Contains(clientData, []byte("require(")) {
		t.Errorf("cjs client.ts must use require() for imports")
	}
	if hasTopLevelImport(clientData) {
		t.Errorf("cjs client.ts must not contain top-level ESM import statements")
	}
	cjsAlias, err := GetRuntimeFilesForStyle("ts", "commonjs")
	if err != nil {
		t.Errorf("GetRuntimeFilesForStyle(\"ts\", \"commonjs\") returned error: %v", err)
	}
	if !sameFileSet(cjs, cjsAlias) {
		t.Errorf("commonjs alias must match cjs runtime")
	}

	// 6. Unknown style returns an error mentioning the valid values.
	if _, err := GetRuntimeFilesForStyle("ts", "bogus"); err == nil {
		t.Errorf("GetRuntimeFilesForStyle(\"ts\", \"bogus\") must return an error")
	} else {
		for _, want := range []string{"esm-node", "esm-bundler", "cjs"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("error message %q must list valid value %q", err.Error(), want)
			}
		}
	}

	// 7. Non-TS languages reject non-empty styles.
	if _, err := GetRuntimeFilesForStyle("python", "esm-bundler"); err == nil {
		t.Errorf("GetRuntimeFilesForStyle(\"python\", \"esm-bundler\") must return an error")
	}
	if _, err := GetRuntimeFilesForStyle("python", ""); err != nil {
		t.Errorf("GetRuntimeFilesForStyle(\"python\", \"\") must succeed: %v", err)
	}
}

// TestGetRuntimeFilesForStyleCjsIsDistinctFromEsmNode asserts that at least
// one file in the cjs runtime tree differs in content from the esm-node
// tree, so cjs is a real alternate runtime, not a no-op alias.
func TestGetRuntimeFilesForStyleCjsIsDistinctFromEsmNode(t *testing.T) {
	esm, err := GetRuntimeFilesForStyle("ts", "esm-node")
	if err != nil {
		t.Fatalf("GetRuntimeFilesForStyle(\"ts\", \"esm-node\") failed: %v", err)
	}
	cjs, err := GetRuntimeFilesForStyle("ts", "cjs")
	if err != nil {
		t.Fatalf("GetRuntimeFilesForStyle(\"ts\", \"cjs\") failed: %v", err)
	}
	differing := 0
	for name, esmData := range esm {
		if cjsData, ok := cjs[name]; ok && !bytes.Equal(esmData, cjsData) {
			differing++
		}
	}
	if differing == 0 {
		t.Errorf("cjs runtime tree must differ from esm-node in at least one file's bytes")
	}
}

// TestRuntimeEmbedRename asserts the ts/ts-node aliases return the same set of
// files (covers the rename in step 1).
func TestRuntimeEmbedRename(t *testing.T) {
	tsFiles, err := GetRuntimeFiles("ts")
	if err != nil {
		t.Fatalf("GetRuntimeFiles(\"ts\") failed: %v", err)
	}
	tsNodeFiles, err := GetRuntimeFiles("ts-node")
	if err != nil {
		t.Fatalf("GetRuntimeFiles(\"ts-node\") failed: %v", err)
	}
	if !sameFileSet(tsFiles, tsNodeFiles) {
		t.Errorf("ts and ts-node runtime trees must be byte-equal")
	}
	for name, data := range tsFiles {
		if !bytes.Equal(tsNodeFiles[name], data) {
			t.Errorf("file %q differs between ts and ts-node runtimes", name)
		}
	}
}

// TestGetRuntimePackageName asserts the runtime package-name accessor returns
// the expected values for the TS family (covers the rename in step 1).
func TestGetRuntimePackageName(t *testing.T) {
	cases := map[string]string{
		"ts":      "pulserpc",
		"ts-node": "pulserpc",
		"ts-cjs":  "pulserpc",
		"go":      "pulserpc",
		"python":  "pulserpc",
		"java":    "com/bitmechanic/pulserpc",
		"csharp":  "PulseRPC",
	}
	for lang, want := range cases {
		if got := getRuntimePackageName(lang); got != want {
			t.Errorf("getRuntimePackageName(%q) = %q, want %q", lang, got, want)
		}
	}
}

// TestGetRuntimeEmbedPath asserts the embed path accessor covers the TS
// variants and the standard languages.
func TestGetRuntimeEmbedPath(t *testing.T) {
	cases := map[string]string{
		"ts-node": "runtimes/ts-node/pulserpc",
		"ts-cjs":  "runtimes/ts-cjs/pulserpc",
		"ts":      "runtimes/ts-node/pulserpc",
		"python":  "runtimes/python/pulserpc",
		"go":      "runtimes/go/pulserpc",
		"java":    "runtimes/java/com/bitmechanic/pulserpc",
		"csharp":  "runtimes/csharp/PulseRPC",
	}
	for lang, want := range cases {
		if got := getRuntimeEmbedPath(lang); got != want {
			t.Errorf("getRuntimeEmbedPath(%q) = %q, want %q", lang, got, want)
		}
	}
}

// TestRuntimeFilesHaveNoExtensionFiltering asserts no .ts file is filtered
// out of the ts-node or ts-cjs runtime trees by the extension rule.
func TestRuntimeFilesHaveNoExtensionFiltering(t *testing.T) {
	for _, style := range []string{"esm-node", "cjs"} {
		files, err := GetRuntimeFilesForStyle("ts", style)
		if err != nil {
			t.Fatalf("GetRuntimeFilesForStyle(\"ts\", %q) failed: %v", style, err)
		}
		if len(files) == 0 {
			t.Errorf("ts %q runtime tree is empty", style)
		}
		for name := range files {
			if !strings.HasSuffix(name, ".ts") {
				t.Errorf("ts %q runtime tree contains non-.ts file %q", style, name)
			}
		}
	}
}

// hasTopLevelImport returns true if the file contains an ESM import
// statement (not just a string inside a comment or a require() call).
func hasTopLevelImport(content []byte) bool {
	// Skip over block comments to avoid false positives.
	stripped := stripBlockComments(content)
	for _, line := range strings.Split(string(stripped), "\n") {
		trimmed := strings.TrimSpace(line)
		// Strip leading // comments on the same line.
		if idx := strings.Index(trimmed, "//"); idx >= 0 {
			trimmed = strings.TrimSpace(trimmed[:idx])
		}
		if strings.HasPrefix(trimmed, "import ") {
			return true
		}
	}
	return false
}

// stripBlockComments removes /* ... */ comments from the input.
func stripBlockComments(content []byte) []byte {
	var out bytes.Buffer
	inBlock := false
	for i := 0; i < len(content); i++ {
		if inBlock {
			if i+1 < len(content) && content[i] == '*' && content[i+1] == '/' {
				inBlock = false
				i++
			}
			continue
		}
		if i+1 < len(content) && content[i] == '/' && content[i+1] == '*' {
			inBlock = true
			i++
			continue
		}
		out.WriteByte(content[i])
	}
	return out.Bytes()
}

// sameFileSet returns true when both maps hold the same filenames and the
// same number of files.
func sameFileSet(a, b map[string][]byte) bool {
	if len(a) != len(b) {
		return false
	}
	namesA := make([]string, 0, len(a))
	for n := range a {
		namesA = append(namesA, n)
	}
	namesB := make([]string, 0, len(b))
	for n := range b {
		namesB = append(namesB, n)
	}
	sort.Strings(namesA)
	sort.Strings(namesB)
	for i := range namesA {
		if namesA[i] != namesB[i] {
			return false
		}
	}
	return true
}
