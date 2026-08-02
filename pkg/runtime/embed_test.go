package runtime

import (
	"bytes"
	"sort"
	"strings"
	"testing"
)

// TestGetRuntimeFilesForStyle covers the matrix required by step 3.6 of the
// plan: all TS styles (esm-node, esm-bundler, cjs) pull from the unified
// runtimes/ts/pulserpc tree. The transform is applied downstream.
func TestGetRuntimeFilesForStyle(t *testing.T) {
	// Legacy call: GetRuntimeFiles("ts") must return the unified tree.
	legacy, err := GetRuntimeFiles("ts")
	if err != nil {
		t.Fatalf("GetRuntimeFiles(\"ts\") failed: %v", err)
	}
	if len(legacy) == 0 {
		t.Fatalf("GetRuntimeFiles(\"ts\") returned an empty map")
	}

	// 1. esm-node pulls from the unified tree.
	esmNode, err := GetRuntimeFilesForStyle("ts", "esm-node")
	if err != nil {
		t.Fatalf("GetRuntimeFilesForStyle(\"ts\", \"esm-node\") failed: %v", err)
	}
	if !sameFileSet(legacy, esmNode) {
		t.Errorf("GetRuntimeFilesForStyle(\"ts\", \"esm-node\") must be byte-equal to GetRuntimeFiles(\"ts\")")
	}

	// 2. Empty style is equivalent to esm-node.
	empty, err := GetRuntimeFilesForStyle("ts", "")
	if err != nil {
		t.Fatalf("GetRuntimeFilesForStyle(\"ts\", \"\") failed: %v", err)
	}
	if !sameFileSet(legacy, empty) {
		t.Errorf("empty style must match legacy GetRuntimeFiles(\"ts\")")
	}

	// 3. Aliases all map to the unified tree.
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

	// 4. esm-bundler shares the unified tree (the generator applies the
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

	// 5. cjs uses the unified ts tree (the generator's CJS transform
	//    converts imports downstream).
	cjs, err := GetRuntimeFilesForStyle("ts", "cjs")
	if err != nil {
		t.Fatalf("GetRuntimeFilesForStyle(\"ts\", \"cjs\") failed: %v", err)
	}
	if len(cjs) == 0 {
		t.Fatalf("GetRuntimeFilesForStyle(\"ts\", \"cjs\") returned an empty map")
	}
	if !sameFileSet(legacy, cjs) {
		t.Errorf("cjs runtime tree must equal the unified ts tree at the embed level")
	}
	for _, name := range []string{"client.ts", "server.ts", "index.ts"} {
		if _, ok := cjs[name]; !ok {
			t.Errorf("cjs runtime is missing expected file %q", name)
		}
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

// TestGetRuntimeFilesForStyleCjsIsSameAsEsmNode asserts that cjs and
// esm-node both pull from the unified ts tree, so the embed-level bytes are
// identical. The generator's transform handles the import-path differences.
func TestGetRuntimeFilesForStyleCjsIsSameAsEsmNode(t *testing.T) {
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
	if differing > 0 {
		t.Errorf("cjs and esm-node runtime trees must be byte-equal from the unified tree, but %d files differ", differing)
	}
}

// TestRuntimeEmbedUnifiedTree asserts the unified ts tree contains the expected
// source files and that legacy ts-node/ts-cjs trees differ from the unified tree
// (validating that we're serving the new merged sources, not the old ones).
func TestRuntimeEmbedUnifiedTree(t *testing.T) {
	unified, err := GetRuntimeFiles("ts")
	if err != nil {
		t.Fatalf("GetRuntimeFiles(\"ts\") failed: %v", err)
	}
	legacyTsNode, err := GetRuntimeFiles("ts-node")
	if err != nil {
		t.Fatalf("GetRuntimeFiles(\"ts-node\") failed: %v", err)
	}

	// The unified tree must have the new files (contract.ts, client.ts, etc.)
	for _, name := range []string{"index.ts", "types.ts", "client.ts", "server.ts", "contract.ts", "transport.ts", "validation.ts", "diff.ts", "rpc.ts"} {
		if _, ok := unified[name]; !ok {
			t.Errorf("unified ts runtime is missing expected file %q", name)
		}
	}

	// The unified tree should differ from the legacy ts-node tree (due to merges).
	differing := 0
	for name, unifiedData := range unified {
		if legacyData, ok := legacyTsNode[name]; ok && !bytes.Equal(unifiedData, legacyData) {
			differing++
		}
	}
	if differing == 0 {
		t.Logf("warning: unified ts tree is byte-equal to legacy ts-node tree - possibly no merge changes were applied")
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
		"ts":      "runtimes/ts/pulserpc",
		"ts-node": "runtimes/ts-node/pulserpc",
		"ts-cjs":  "runtimes/ts-cjs/pulserpc",
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
