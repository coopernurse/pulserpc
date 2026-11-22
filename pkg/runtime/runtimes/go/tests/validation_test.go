package main

import (
	"math"
	"testing"

	"github.com/coopernurse/pulserpc/pkg/runtime/runtimes/go/pulserpc"
)

func TestValidateString(t *testing.T) {
	if err := pulserpc.ValidateString("test"); err != nil {
		t.Errorf("Expected nil error for string, got %v", err)
	}

	if err := pulserpc.ValidateString(123); err == nil {
		t.Error("Expected error for non-string value")
	}
}

func TestValidateInt(t *testing.T) {
	if err := pulserpc.ValidateInt(123); err != nil {
		t.Errorf("Expected nil error for int, got %v", err)
	}

	// JSON numbers are decoded as float64, so we accept them
	if err := pulserpc.ValidateInt(123.0); err != nil {
		t.Errorf("Expected nil error for float64 representing int, got %v", err)
	}

	if err := pulserpc.ValidateInt("123"); err == nil {
		t.Error("Expected error for non-int value")
	}
}

// go-06: fractional floats (including near-integers) are rejected
// symmetrically for positive and negative values.
func TestValidateIntRejectsFractionalFloats(t *testing.T) {
	for _, value := range []float64{1.5, -1.5, 1.0000000001, -1.0000000001, 0.0000000001, -0.0000000001} {
		if err := pulserpc.ValidateInt(value); err == nil {
			t.Errorf("ValidateInt(%v) should reject a fractional part", value)
		}
	}
	for _, value := range []float64{123.0, -123.0, 0.0} {
		if err := pulserpc.ValidateInt(value); err != nil {
			t.Errorf("ValidateInt(%v) should accept a whole number: %v", value, err)
		}
	}
}

func TestValidateFloat(t *testing.T) {
	if err := pulserpc.ValidateFloat(123.45); err != nil {
		t.Errorf("Expected nil error for float64, got %v", err)
	}

	if err := pulserpc.ValidateFloat(123); err != nil {
		t.Errorf("Expected nil error for int, got %v", err)
	}

	if err := pulserpc.ValidateFloat("123.45"); err == nil {
		t.Error("Expected error for non-float value")
	}
}

func TestValidateBool(t *testing.T) {
	if err := pulserpc.ValidateBool(true); err != nil {
		t.Errorf("Expected nil error for bool, got %v", err)
	}

	if err := pulserpc.ValidateBool(false); err != nil {
		t.Errorf("Expected nil error for bool, got %v", err)
	}

	if err := pulserpc.ValidateBool("true"); err == nil {
		t.Error("Expected error for non-bool value")
	}
}

func TestValidateArray(t *testing.T) {
	typeDef := map[string]interface{}{
		"builtIn": "string",
	}
	allStructs := pulserpc.StructMap{}
	allEnums := pulserpc.EnumMap{}

	elementValidator := func(v interface{}) error {
		return pulserpc.ValidateType(v, typeDef, allStructs, allEnums, false)
	}

	arr := []interface{}{"a", "b", "c"}
	if err := pulserpc.ValidateArray(arr, elementValidator); err != nil {
		t.Errorf("Expected nil error for valid array, got %v", err)
	}

	invalidArr := []interface{}{"a", 123, "c"}
	if err := pulserpc.ValidateArray(invalidArr, elementValidator); err == nil {
		t.Error("Expected error for invalid array element")
	}

	if err := pulserpc.ValidateArray("not an array", elementValidator); err == nil {
		t.Error("Expected error for non-array value")
	}
}

func TestValidateMap(t *testing.T) {
	typeDef := map[string]interface{}{
		"builtIn": "int",
	}
	allStructs := pulserpc.StructMap{}
	allEnums := pulserpc.EnumMap{}

	valueValidator := func(v interface{}) error {
		return pulserpc.ValidateType(v, typeDef, allStructs, allEnums, false)
	}

	m := map[string]interface{}{
		"key1": 1,
		"key2": 2,
	}
	if err := pulserpc.ValidateMap(m, valueValidator); err != nil {
		t.Errorf("Expected nil error for valid map, got %v", err)
	}

	invalidMap := map[string]interface{}{
		"key1": "not an int",
	}
	if err := pulserpc.ValidateMap(invalidMap, valueValidator); err == nil {
		t.Error("Expected error for invalid map value")
	}

	if err := pulserpc.ValidateMap("not a map", valueValidator); err == nil {
		t.Error("Expected error for non-map value")
	}
}

func TestValidateEnum(t *testing.T) {
	allowedValues := []string{"VALUE1", "VALUE2", "VALUE3"}

	if err := pulserpc.ValidateEnum("VALUE1", "TestEnum", allowedValues); err != nil {
		t.Errorf("Expected nil error for valid enum value, got %v", err)
	}

	if err := pulserpc.ValidateEnum("INVALID", "TestEnum", allowedValues); err == nil {
		t.Error("Expected error for invalid enum value")
	}

	if err := pulserpc.ValidateEnum(123, "TestEnum", allowedValues); err == nil {
		t.Error("Expected error for non-string enum value")
	}
}

func TestValidateStruct(t *testing.T) {
	allStructs := pulserpc.StructMap{
		"TestStruct": pulserpc.StructDef{
			"fields": []interface{}{
				map[string]interface{}{
					"name": "field1",
					"type": map[string]interface{}{"builtIn": "string"},
				},
				map[string]interface{}{
					"name":     "field2",
					"type":     map[string]interface{}{"builtIn": "int"},
					"optional": true,
				},
			},
		},
	}
	allEnums := pulserpc.EnumMap{}

	structDef := allStructs["TestStruct"]
	value := map[string]interface{}{
		"field1": "test",
		"field2": 123,
	}

	if err := pulserpc.ValidateStruct(value, "TestStruct", structDef, allStructs, allEnums); err != nil {
		t.Errorf("Expected nil error for valid struct, got %v", err)
	}

	// Missing required field
	invalidValue := map[string]interface{}{
		"field2": 123,
	}
	if err := pulserpc.ValidateStruct(invalidValue, "TestStruct", structDef, allStructs, allEnums); err == nil {
		t.Error("Expected error for missing required field")
	}

	// Invalid field type
	invalidTypeValue := map[string]interface{}{
		"field1": 123, // should be string
	}
	if err := pulserpc.ValidateStruct(invalidTypeValue, "TestStruct", structDef, allStructs, allEnums); err == nil {
		t.Error("Expected error for invalid field type")
	}
}

func TestValidateType(t *testing.T) {
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
	allEnums := pulserpc.EnumMap{
		"TestEnum": pulserpc.EnumDef{
			"values": []interface{}{
				map[string]interface{}{"name": "VALUE1"},
			},
		},
	}

	// Test built-in types
	stringType := map[string]interface{}{"builtIn": "string"}
	if err := pulserpc.ValidateType("test", stringType, allStructs, allEnums, false); err != nil {
		t.Errorf("Expected nil error for string, got %v", err)
	}

	// Test optional
	if err := pulserpc.ValidateType(nil, stringType, allStructs, allEnums, true); err != nil {
		t.Errorf("Expected nil error for optional nil, got %v", err)
	}

	// Test array
	arrayType := map[string]interface{}{
		"array": map[string]interface{}{"builtIn": "string"},
	}
	if err := pulserpc.ValidateType([]interface{}{"a", "b"}, arrayType, allStructs, allEnums, false); err != nil {
		t.Errorf("Expected nil error for array, got %v", err)
	}

	// Test map
	mapType := map[string]interface{}{
		"mapValue": map[string]interface{}{"builtIn": "int"},
	}
	if err := pulserpc.ValidateType(map[string]interface{}{"key": 1}, mapType, allStructs, allEnums, false); err != nil {
		t.Errorf("Expected nil error for map, got %v", err)
	}

	// Test struct
	structType := map[string]interface{}{
		"userDefined": "TestStruct",
	}
	structValue := map[string]interface{}{
		"field1": "test",
	}
	if err := pulserpc.ValidateType(structValue, structType, allStructs, allEnums, false); err != nil {
		t.Errorf("Expected nil error for struct, got %v", err)
	}

	// Test enum
	enumType := map[string]interface{}{
		"userDefined": "TestEnum",
	}
	if err := pulserpc.ValidateType("VALUE1", enumType, allStructs, allEnums, false); err != nil {
		t.Errorf("Expected nil error for enum, got %v", err)
	}
}

// R25/D1: IDL int is signed 32-bit. In-range whole numbers pass; out-of-range
// integral values, fractions, NaN/Inf, and bools fail.
func TestValidateIntRange(t *testing.T) {
	valid := []interface{}{
		int(math.MinInt32), int(math.MaxInt32),
		int64(math.MinInt32), int64(math.MaxInt32),
		float64(math.MinInt32), float64(math.MaxInt32),
	}
	for _, value := range valid {
		if err := pulserpc.ValidateInt(value); err != nil {
			t.Errorf("ValidateInt(%v) = %v, want nil", value, err)
		}
	}

	invalid := []interface{}{
		int64(math.MaxInt32) + 1, int64(math.MinInt32) - 1,
		float64(math.MaxInt32) + 1, float64(math.MinInt32) - 1,
		int64(math.MaxInt32) * 2,
		1.5, -1.5,
		math.NaN(), math.Inf(1), math.Inf(-1),
		true,
	}
	for _, value := range invalid {
		if err := pulserpc.ValidateInt(value); err == nil {
			t.Errorf("ValidateInt(%v) = nil, want an error", value)
		}
	}
}
