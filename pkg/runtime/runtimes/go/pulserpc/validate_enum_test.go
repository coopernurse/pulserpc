package pulserpc

import "testing"

type namedEnum string

func TestValidateEnumAcceptsNamedStringType(t *testing.T) {
	if errs := ValidateEnumE(namedEnum("add"), "MathOp", []string{"add", "multiply"}, "result"); len(errs) != 0 {
		t.Fatalf("named enum string was rejected: %+v", errs)
	}
	if errs := ValidateEnumE(namedEnum("subtract"), "MathOp", []string{"add", "multiply"}, "result"); len(errs) != 1 {
		t.Fatalf("invalid named enum value was accepted: %+v", errs)
	}
	if errs := ValidateEnumE(42, "MathOp", []string{"add"}, "result"); len(errs) != 1 {
		t.Fatalf("non-string enum value was accepted: %+v", errs)
	}
}
