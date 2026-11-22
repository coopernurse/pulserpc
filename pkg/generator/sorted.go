package generator

import "sort"

// sortedMapKeys returns the keys of m in ascending order. Generated output is
// derived from maps in several places; sorting the keys keeps generation
// byte-deterministic (invariant C1/D12).
func sortedMapKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
