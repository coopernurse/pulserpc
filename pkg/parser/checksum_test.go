package parser

import (
	"strings"
	"testing"
)

func TestComputeChecksum(t *testing.T) {
	// Test basic checksum computation
	idl := &IDL{
		RootNamespace: "test",
		Interfaces: []*Interface{
			{
				Name:      "TestService",
				Namespace: "test",
				Methods: []*Method{
					{
						Name: "testMethod",
						Parameters: []*Parameter{
							{Name: "param1", Type: &Type{BuiltIn: "string"}},
						},
						ReturnType: &Type{BuiltIn: "int"},
					},
				},
			},
		},
	}

	checksum, err := ComputeChecksum(idl)
	if err != nil {
		t.Fatalf("ComputeChecksum failed: %v", err)
	}

	// Should be 43 characters (base64url encoded SHA-256, no padding)
	// 32 bytes * 8 bits = 256 bits. 256 / 6 = 42.67 chars, rounded up = 43 chars without padding
	if len(checksum) != 43 {
		t.Errorf("Expected checksum length 43, got %d", len(checksum))
	}

	// Checksum should only contain base64url characters
	for _, c := range checksum {
		if !strings.Contains("ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_", string(c)) {
			t.Errorf("Invalid base64url character: %c", c)
		}
	}

	// Same IDL should produce same checksum
	checksum2, err := ComputeChecksum(idl)
	if err != nil {
		t.Fatalf("ComputeChecksum failed: %v", err)
	}
	if checksum != checksum2 {
		t.Errorf("Same IDL should produce same checksum")
	}
}

func TestChecksumOrderInvariance(t *testing.T) {
	// Two IDLs with same content but different declaration order should produce same checksum
	idl1 := &IDL{
		RootNamespace: "test",
		Interfaces: []*Interface{
			{Name: "ServiceA", Namespace: "test"},
			{Name: "ServiceB", Namespace: "test"},
		},
		Structs: []*Struct{
			{Name: "StructA", Namespace: "test", Fields: []*Field{}},
			{Name: "StructB", Namespace: "test", Fields: []*Field{}},
		},
		Enums: []*Enum{
			{Name: "EnumA", Namespace: "test", Values: []*EnumValue{{Name: "a"}, {Name: "b"}}},
			{Name: "EnumB", Namespace: "test", Values: []*EnumValue{{Name: "c"}, {Name: "d"}}},
		},
	}

	idl2 := &IDL{
		RootNamespace: "test",
		Structs: []*Struct{
			{Name: "StructB", Namespace: "test", Fields: []*Field{}},
			{Name: "StructA", Namespace: "test", Fields: []*Field{}},
		},
		Enums: []*Enum{
			{Name: "EnumB", Namespace: "test", Values: []*EnumValue{{Name: "d"}, {Name: "c"}}},
			{Name: "EnumA", Namespace: "test", Values: []*EnumValue{{Name: "b"}, {Name: "a"}}},
		},
		Interfaces: []*Interface{
			{Name: "ServiceB", Namespace: "test"},
			{Name: "ServiceA", Namespace: "test"},
		},
	}

	cs1, err := ComputeChecksum(idl1)
	if err != nil {
		t.Fatalf("ComputeChecksum failed: %v", err)
	}
	cs2, err := ComputeChecksum(idl2)
	if err != nil {
		t.Fatalf("ComputeChecksum failed: %v", err)
	}

	if cs1 != cs2 {
		t.Errorf("Different order should produce same checksum.\nIDL1 checksum: %s\nIDL2 checksum: %s", cs1, cs2)
	}
}

func TestChecksumParameterSortInvariance(t *testing.T) {
	idl1 := &IDL{
		RootNamespace: "test",
		Interfaces: []*Interface{
			{
				Name:      "Service",
				Namespace: "test",
				Methods: []*Method{
					{
						Name: "method",
						Parameters: []*Parameter{
							{Name: "a", Type: &Type{BuiltIn: "string"}},
							{Name: "b", Type: &Type{BuiltIn: "int"}},
							{Name: "c", Type: &Type{BuiltIn: "bool"}},
						},
						ReturnType: &Type{BuiltIn: "void"},
					},
				},
			},
		},
	}

	idl2 := &IDL{
		RootNamespace: "test",
		Interfaces: []*Interface{
			{
				Name:      "Service",
				Namespace: "test",
				Methods: []*Method{
					{
						Name: "method",
						Parameters: []*Parameter{
							{Name: "c", Type: &Type{BuiltIn: "bool"}},
							{Name: "a", Type: &Type{BuiltIn: "string"}},
							{Name: "b", Type: &Type{BuiltIn: "int"}},
						},
						ReturnType: &Type{BuiltIn: "void"},
					},
				},
			},
		},
	}

	cs1, _ := ComputeChecksum(idl1)
	cs2, _ := ComputeChecksum(idl2)

	if cs1 != cs2 {
		t.Errorf("Different parameter order should produce same checksum")
	}
}

func TestChecksumFieldSortInvariance(t *testing.T) {
	idl1 := &IDL{
		RootNamespace: "test",
		Structs: []*Struct{
			{
				Name:      "Struct",
				Namespace: "test",
				Fields: []*Field{
					{Name: "fieldA", Type: &Type{BuiltIn: "string"}},
					{Name: "fieldB", Type: &Type{BuiltIn: "int"}},
					{Name: "fieldC", Type: &Type{BuiltIn: "bool"}},
				},
			},
		},
	}

	idl2 := &IDL{
		RootNamespace: "test",
		Structs: []*Struct{
			{
				Name:      "Struct",
				Namespace: "test",
				Fields: []*Field{
					{Name: "fieldC", Type: &Type{BuiltIn: "bool"}},
					{Name: "fieldA", Type: &Type{BuiltIn: "string"}},
					{Name: "fieldB", Type: &Type{BuiltIn: "int"}},
				},
			},
		},
	}

	cs1, _ := ComputeChecksum(idl1)
	cs2, _ := ComputeChecksum(idl2)

	if cs1 != cs2 {
		t.Errorf("Different field order should produce same checksum")
	}
}

func TestChecksumDifferentContent(t *testing.T) {
	idl1 := &IDL{
		RootNamespace: "test",
		Interfaces: []*Interface{
			{Name: "ServiceA", Namespace: "test"},
		},
	}

	idl2 := &IDL{
		RootNamespace: "test",
		Interfaces: []*Interface{
			{Name: "ServiceB", Namespace: "test"},
		},
	}

	cs1, _ := ComputeChecksum(idl1)
	cs2, _ := ComputeChecksum(idl2)

	if cs1 == cs2 {
		t.Errorf("Different content should produce different checksum")
	}
}

func TestChecksumErrorCodeIncluded(t *testing.T) {
	idl1 := &IDL{
		RootNamespace: "test",
		Errors: []*ErrorDef{
			{Name: "test.ErrorA", Namespace: "test", Code: 1001},
		},
	}

	idl2 := &IDL{
		RootNamespace: "test",
		Errors: []*ErrorDef{
			{Name: "test.ErrorA", Namespace: "test", Code: 1002},
		},
	}

	cs1, _ := ComputeChecksum(idl1)
	cs2, _ := ComputeChecksum(idl2)

	if cs1 == cs2 {
		t.Errorf("Different error codes should produce different checksum")
	}
}

func TestChecksumErrorMessageExcluded(t *testing.T) {
	idl1 := &IDL{
		RootNamespace: "test",
		Errors: []*ErrorDef{
			{Name: "test.ErrorA", Namespace: "test", Code: 1001, Message: "Message A"},
		},
	}

	idl2 := &IDL{
		RootNamespace: "test",
		Errors: []*ErrorDef{
			{Name: "test.ErrorA", Namespace: "test", Code: 1001, Message: "Message B is completely different"},
		},
	}

	cs1, _ := ComputeChecksum(idl1)
	cs2, _ := ComputeChecksum(idl2)

	if cs1 != cs2 {
		t.Errorf("Different error messages should produce same checksum (message excluded)")
	}
}

func TestChecksumNamespaceResolution(t *testing.T) {
	idl1 := &IDL{
		RootNamespace: "test",
		Structs: []*Struct{
			{
				Name:      "Response",
				Namespace: "test",
				Fields: []*Field{
					{Name: "base", Type: &Type{UserDefined: "BaseResponse"}},
				},
			},
			{
				Name:      "BaseResponse",
				Namespace: "test",
				Fields:    []*Field{},
			},
		},
	}

	idl2 := &IDL{
		RootNamespace: "test",
		Structs: []*Struct{
			{
				Name:      "Response",
				Namespace: "test",
				Fields: []*Field{
					{Name: "base", Type: &Type{UserDefined: "test.BaseResponse"}},
				},
			},
			{
				Name:      "BaseResponse",
				Namespace: "test",
				Fields:    []*Field{},
			},
		},
	}

	cs1, _ := ComputeChecksum(idl1)
	cs2, _ := ComputeChecksum(idl2)

	if cs1 != cs2 {
		t.Errorf("Same type with different qualification should produce same checksum")
	}
}

func TestChecksumBookPulse(t *testing.T) {
	// Read book.pulse and compute checksum
	idl, err := ParseIDL("book.pulse", "")
	if err != nil {
		t.Fatalf("Failed to parse book.pulse: %v", err)
	}

	checksum, err := ComputeChecksum(idl)
	if err != nil {
		t.Fatalf("ComputeChecksum failed: %v", err)
	}

	// Verify checksum format
	if len(checksum) != 43 {
		t.Errorf("Expected checksum length 43, got %d", len(checksum))
	}

	// This is a regression test - the checksum should be deterministic
	// Expected checksum for book.pulse (we'll capture this after first run)
	t.Logf("book.pulse checksum: %s", checksum)
}

func TestChecksumEnumValueSort(t *testing.T) {
	idl1 := &IDL{
		RootNamespace: "test",
		Enums: []*Enum{
			{
				Name:      "Status",
				Namespace: "test",
				Values: []*EnumValue{
					{Name: "zebra"},
					{Name: "apple"},
					{Name: "mango"},
				},
			},
		},
	}

	idl2 := &IDL{
		RootNamespace: "test",
		Enums: []*Enum{
			{
				Name:      "Status",
				Namespace: "test",
				Values: []*EnumValue{
					{Name: "mango"},
					{Name: "zebra"},
					{Name: "apple"},
				},
			},
		},
	}

	cs1, _ := ComputeChecksum(idl1)
	cs2, _ := ComputeChecksum(idl2)

	if cs1 != cs2 {
		t.Errorf("Different enum value order should produce same checksum")
	}
}

func TestChecksumMethodSort(t *testing.T) {
	idl1 := &IDL{
		RootNamespace: "test",
		Interfaces: []*Interface{
			{
				Name:      "Service",
				Namespace: "test",
				Methods: []*Method{
					{Name: "alpha"},
					{Name: "beta"},
					{Name: "gamma"},
				},
			},
		},
	}

	idl2 := &IDL{
		RootNamespace: "test",
		Interfaces: []*Interface{
			{
				Name:      "Service",
				Namespace: "test",
				Methods: []*Method{
					{Name: "gamma"},
					{Name: "alpha"},
					{Name: "beta"},
				},
			},
		},
	}

	cs1, _ := ComputeChecksum(idl1)
	cs2, _ := ComputeChecksum(idl2)

	if cs1 != cs2 {
		t.Errorf("Different method order should produce same checksum")
	}
}

func TestChecksumExtendsResolved(t *testing.T) {
	idl1 := &IDL{
		RootNamespace: "test",
		Structs: []*Struct{
			{
				Name:      "Child",
				Namespace: "test",
				Extends:   "Parent",
				Fields:    []*Field{},
			},
			{
				Name:      "Parent",
				Namespace: "test",
				Fields:    []*Field{},
			},
		},
	}

	idl2 := &IDL{
		RootNamespace: "test",
		Structs: []*Struct{
			{
				Name:      "Child",
				Namespace: "test",
				Extends:   "test.Parent",
				Fields:    []*Field{},
			},
			{
				Name:      "Parent",
				Namespace: "test",
				Fields:    []*Field{},
			},
		},
	}

	cs1, _ := ComputeChecksum(idl1)
	cs2, _ := ComputeChecksum(idl2)

	if cs1 != cs2 {
		t.Errorf("Same extends with different qualification should produce same checksum")
	}
}

func TestChecksumReturnOptional(t *testing.T) {
	idl1 := &IDL{
		RootNamespace: "test",
		Interfaces: []*Interface{
			{
				Name:      "Service",
				Namespace: "test",
				Methods: []*Method{
					{
						Name:           "method",
						ReturnType:     &Type{UserDefined: "Response"},
						ReturnOptional: false,
					},
				},
			},
		},
	}

	idl2 := &IDL{
		RootNamespace: "test",
		Interfaces: []*Interface{
			{
				Name:      "Service",
				Namespace: "test",
				Methods: []*Method{
					{
						Name:           "method",
						ReturnType:     &Type{UserDefined: "Response"},
						ReturnOptional: true,
					},
				},
			},
		},
	}

	cs1, _ := ComputeChecksum(idl1)
	cs2, _ := ComputeChecksum(idl2)

	if cs1 == cs2 {
		t.Errorf("Different returnOptional should produce different checksum")
	}
}

func TestChecksumRaisesSorted(t *testing.T) {
	idl1 := &IDL{
		RootNamespace: "test",
		Errors: []*ErrorDef{
			{Name: "ErrorA", Namespace: "test", Code: 1},
			{Name: "ErrorB", Namespace: "test", Code: 2},
			{Name: "ErrorC", Namespace: "test", Code: 3},
		},
		Interfaces: []*Interface{
			{
				Name:      "Service",
				Namespace: "test",
				Methods: []*Method{
					{
						Name:   "method",
						Raises: []string{"ErrorC", "ErrorA", "ErrorB"},
					},
				},
			},
		},
	}

	idl2 := &IDL{
		RootNamespace: "test",
		Errors: []*ErrorDef{
			{Name: "ErrorA", Namespace: "test", Code: 1},
			{Name: "ErrorB", Namespace: "test", Code: 2},
			{Name: "ErrorC", Namespace: "test", Code: 3},
		},
		Interfaces: []*Interface{
			{
				Name:      "Service",
				Namespace: "test",
				Methods: []*Method{
					{
						Name:   "method",
						Raises: []string{"ErrorA", "ErrorB", "ErrorC"},
					},
				},
			},
		},
	}

	cs1, _ := ComputeChecksum(idl1)
	cs2, _ := ComputeChecksum(idl2)

	if cs1 != cs2 {
		t.Errorf("Same raises in different order should produce same checksum")
	}
}

func TestChecksumFieldOptional(t *testing.T) {
	idl1 := &IDL{
		RootNamespace: "test",
		Structs: []*Struct{
			{
				Name:      "Struct",
				Namespace: "test",
				Fields: []*Field{
					{Name: "field", Type: &Type{BuiltIn: "string"}, Optional: false},
				},
			},
		},
	}

	idl2 := &IDL{
		RootNamespace: "test",
		Structs: []*Struct{
			{
				Name:      "Struct",
				Namespace: "test",
				Fields: []*Field{
					{Name: "field", Type: &Type{BuiltIn: "string"}, Optional: true},
				},
			},
		},
	}

	cs1, _ := ComputeChecksum(idl1)
	cs2, _ := ComputeChecksum(idl2)

	if cs1 == cs2 {
		t.Errorf("Different optional flag should produce different checksum")
	}
}
