package pulserpc

import (
	"sort"
	"strings"
)

// baseDefinition records the definition for a base name and whether that base
// name is shared by more than one key. Resolution is deterministic: when a
// base name is ambiguous the first key in sorted order wins, matching the
// Python, Java and TypeScript runtimes (invariant R6). The ambiguous flag is
// retained for diagnostics.
type baseDefinition[T any] struct {
	def       T
	ambiguous bool
}

// definitionIndex resolves possibly-qualified type names in O(1) using a
// base-name index built once when a Contract is constructed. Resolution order:
// an exact key wins, then an exact base-name key, then the first sorted key
// whose base name matches.
type definitionIndex[T any] struct {
	defs   map[string]T
	byBase map[string]baseDefinition[T]
}

func newDefinitionIndex[T any](defs map[string]T) *definitionIndex[T] {
	idx := &definitionIndex[T]{
		defs:   defs,
		byBase: make(map[string]baseDefinition[T], len(defs)),
	}
	keys := make([]string, 0, len(defs))
	for key := range defs {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		base := typeBaseName(key)
		entry, exists := idx.byBase[base]
		if !exists {
			idx.byBase[base] = baseDefinition[T]{def: defs[key]}
			continue
		}
		// More than one key shares this base name. Keep the first (sorted)
		// definition for deterministic resolution and flag the ambiguity.
		entry.ambiguous = true
		idx.byBase[base] = entry
	}
	return idx
}

func typeBaseName(name string) string {
	if i := strings.LastIndex(name, "."); i >= 0 {
		return name[i+1:]
	}
	return name
}

// resolve returns the definition for name using only O(1) map lookups after the
// index is built.
func (idx *definitionIndex[T]) resolve(name string) T {
	var zero T
	if idx == nil {
		return zero
	}
	if def, ok := idx.defs[name]; ok {
		return def
	}
	base := typeBaseName(name)
	if def, ok := idx.defs[base]; ok {
		return def
	}
	entry, ok := idx.byBase[base]
	if !ok {
		return zero
	}
	return entry.def
}
