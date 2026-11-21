package pulserpc

import "strings"

// baseDefinition records the definition for a base name and whether that base
// name is shared by more than one key. An ambiguous base name must not resolve
// to an arbitrary definition; exact key lookups still win.
type baseDefinition[T any] struct {
	def       T
	ambiguous bool
}

// definitionIndex resolves possibly-qualified type names in O(1) using a
// base-name index built once when a Contract is constructed. It preserves the
// original resolution rules: an exact key wins, then an exact base-name key,
// then a key whose base name matches (nil when that base name is ambiguous).
type definitionIndex[T any] struct {
	defs   map[string]T
	byBase map[string]baseDefinition[T]
}

func newDefinitionIndex[T any](defs map[string]T) *definitionIndex[T] {
	idx := &definitionIndex[T]{
		defs:   defs,
		byBase: make(map[string]baseDefinition[T], len(defs)),
	}
	for key, def := range defs {
		base := typeBaseName(key)
		entry, exists := idx.byBase[base]
		if !exists {
			idx.byBase[base] = baseDefinition[T]{def: def}
			continue
		}
		// More than one key shares this base name. Keep the ambiguity flag;
		// callers must fail closed instead of picking an arbitrary entry.
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

// resolve returns the definition for name using only O(1) map lookups.
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
	if !ok || entry.ambiguous {
		return zero
	}
	return entry.def
}
