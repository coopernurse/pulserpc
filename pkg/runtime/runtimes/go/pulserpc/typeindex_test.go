package pulserpc

import (
	"fmt"
	"testing"
)

// contractWithStructNames builds a contract whose structs are all uniquely
// named, for resolution benchmarks at different map sizes.
func contractWithStructNames(names []string) *Contract {
	structs := make([]interface{}, 0, len(names))
	for _, name := range names {
		structs = append(structs, map[string]interface{}{
			"name":   name,
			"fields": []interface{}{},
		})
	}
	return NewContract(map[string]interface{}{
		"structs": structs,
		"enums":   []interface{}{},
	})
}

// audit-go-2: the contract-owned index must preserve exact-match-wins and
// ambiguity-to-nil semantics for both structs and enums.
func TestContractFindUsesIndexSemantics(t *testing.T) {
	c := NewContract(map[string]interface{}{
		"structs": []interface{}{
			map[string]interface{}{"name": "a.Status"},
			map[string]interface{}{"name": "b.Status"},
			map[string]interface{}{"name": "Exact"},
		},
		"enums": []interface{}{
			map[string]interface{}{"name": "a.Mode", "values": []interface{}{map[string]interface{}{"name": "X"}}},
			map[string]interface{}{"name": "b.Mode", "values": []interface{}{map[string]interface{}{"name": "Y"}}},
			map[string]interface{}{"name": "Unique", "values": []interface{}{map[string]interface{}{"name": "Z"}}},
		},
	})

	if def := c.FindStruct("a.Status"); def == nil {
		t.Error("FindStruct should resolve an exact qualified key")
	}
	if def := c.FindStruct("x.Exact"); def == nil {
		t.Error("FindStruct should resolve a qualified name by exact base-name key")
	}
	if def := c.FindStruct("x.Status"); def != nil {
		t.Errorf("FindStruct resolved an ambiguous base name: %#v", def)
	}

	if def := c.FindEnum("a.Mode"); def == nil {
		t.Error("FindEnum should resolve an exact qualified key")
	}
	if def := c.FindEnum("x.Unique"); def == nil {
		t.Error("FindEnum should resolve a qualified name by base name")
	}
	if def := c.FindEnum("x.Mode"); def != nil {
		t.Errorf("FindEnum resolved an ambiguous base name: %#v", def)
	}
}

// benchmarkContractFindStructMiss measures an O(1) miss through the
// contract-owned index. The two sizes make an O(N) regression obvious: a full
// map scan would make the 10,000-entry case ~1000x slower than the 10-entry
// case, while the indexed implementation stays flat.
func benchmarkContractFindStructMiss(b *testing.B, size int) {
	names := make([]string, size)
	for i := range names {
		names[i] = fmt.Sprintf("ns%d.Type%d", i, i)
	}
	c := contractWithStructNames(names)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = c.FindStruct("ns.Type.DoesNotExist")
	}
}

func BenchmarkContractFindStructMiss10(b *testing.B) {
	benchmarkContractFindStructMiss(b, 10)
}

func BenchmarkContractFindStructMiss10000(b *testing.B) {
	benchmarkContractFindStructMiss(b, 10000)
}
