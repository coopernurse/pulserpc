package main

import (
	"testing"

	"github.com/coopernurse/pulserpc/pkg/runtime/runtimes/go/pulserpc"
)

func TestFindStruct(t *testing.T) {
	allStructs := pulserpc.StructMap{
		"TestStruct": pulserpc.StructDef{
			"fields": []interface{}{
				map[string]interface{}{
					"name": "field1",
					"type": map[string]interface{}{"builtIn": "string"},
				},
			},
		},
	}

	structDef := pulserpc.FindStruct("TestStruct", allStructs)
	if structDef == nil {
		t.Error("Expected to find TestStruct")
	}

	notFound := pulserpc.FindStruct("NotFound", allStructs)
	if notFound != nil {
		t.Error("Expected NotFound to return nil")
	}
}

func TestFindEnum(t *testing.T) {
	allEnums := pulserpc.EnumMap{
		"TestEnum": pulserpc.EnumDef{
			"values": []interface{}{
				map[string]interface{}{"name": "VALUE1"},
				map[string]interface{}{"name": "VALUE2"},
			},
		},
	}

	enumDef := pulserpc.FindEnum("TestEnum", allEnums)
	if enumDef == nil {
		t.Error("Expected to find TestEnum")
	}

	// Test qualified name
	enumDef2 := pulserpc.FindEnum("ns.TestEnum", allEnums)
	if enumDef2 == nil {
		t.Error("Expected to find TestEnum via qualified name")
	}

	notFound := pulserpc.FindEnum("NotFound", allEnums)
	if notFound != nil {
		t.Error("Expected NotFound to return nil")
	}
}

// go-07: qualified names must resolve against qualified keys, and a base name
// shared by two namespaces must not resolve nondeterministically.
func TestQualifiedTypeResolution(t *testing.T) {
	structs := pulserpc.StructMap{
		"a.Status": pulserpc.StructDef{"fields": []interface{}{}},
		"b.Status": pulserpc.StructDef{"fields": []interface{}{}},
		"Exact":    pulserpc.StructDef{"fields": []interface{}{}},
	}
	if def := pulserpc.FindStruct("a.Status", structs); def == nil {
		t.Error("FindStruct should resolve an exact qualified key")
	}
	if def := pulserpc.FindStruct("x.Exact", structs); def == nil {
		t.Error("FindStruct should resolve a qualified name by base name")
	}
	for i := 0; i < 100; i++ {
		if def := pulserpc.FindStruct("x.Status", structs); def != nil {
			t.Fatalf("FindStruct resolved an ambiguous base name on iteration %d", i)
		}
	}

	enums := pulserpc.EnumMap{
		"a.Status": pulserpc.EnumDef{"values": []interface{}{map[string]interface{}{"name": "OK"}}},
		"b.Status": pulserpc.EnumDef{"values": []interface{}{map[string]interface{}{"name": "FAIL"}}},
	}
	for i := 0; i < 100; i++ {
		if def := pulserpc.FindEnum("x.Status", enums); def != nil {
			t.Fatalf("FindEnum resolved an ambiguous base name on iteration %d", i)
		}
	}
	if def := pulserpc.FindEnum("a.Status", enums); def == nil {
		t.Error("FindEnum should resolve an exact qualified key")
	}
}

func TestGetStructFields(t *testing.T) {
	allStructs := pulserpc.StructMap{
		"Parent": pulserpc.StructDef{
			"fields": []interface{}{
				map[string]interface{}{
					"name": "parentField",
					"type": map[string]interface{}{"builtIn": "string"},
				},
			},
		},
		"Child": pulserpc.StructDef{
			"extends": "Parent",
			"fields": []interface{}{
				map[string]interface{}{
					"name": "childField",
					"type": map[string]interface{}{"builtIn": "int"},
				},
			},
		},
	}

	fields := pulserpc.GetStructFields("Child", allStructs)
	if len(fields) != 2 {
		t.Errorf("Expected 2 fields (parent + child), got %d", len(fields))
	}

	// Check parent field
	if fields[0]["name"] != "parentField" && fields[1]["name"] != "parentField" {
		t.Error("Expected to find parentField in fields")
	}

	// Check child field
	if fields[0]["name"] != "childField" && fields[1]["name"] != "childField" {
		t.Error("Expected to find childField in fields")
	}
}
