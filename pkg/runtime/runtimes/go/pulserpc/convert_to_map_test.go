package pulserpc

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

// helper: the reflection converter must produce exactly the shape that the
// original JSON round trip produced for struct values.
func requireJSONShape(t *testing.T, value interface{}) interface{} {
	t.Helper()
	got, err := convertToMap(value)
	if err != nil {
		t.Fatalf("convertToMap(%T): %v", value, err)
	}
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("json.Marshal(%T): %v", value, err)
	}
	var want interface{}
	if err := json.Unmarshal(data, &want); err != nil {
		t.Fatalf("json.Unmarshal(%T): %v", value, err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("convertToMap(%T) shape changed\n got: %#v\nwant: %#v", value, got, want)
	}
	return got
}

type ctmInner struct {
	Label  string `json:"label"`
	Count  int    `json:"count,omitempty"`
	Secret string `json:"-"`
	Bytes  []byte `json:"bytes,omitempty"`
}

type ctmEnum string

type ctmOuter struct {
	Name   string         `json:"user_name"`
	Empty  string         `json:"empty,omitempty"`
	Nested ctmInner       `json:"nested"`
	Ptr    *ctmInner      `json:"ptr,omitempty"`
	NilPtr *ctmInner      `json:"nil_ptr"`
	Items  []ctmInner     `json:"items"`
	Op     ctmEnum        `json:"op"`
	Tags   map[string]int `json:"tags"`
	Array  [2]ctmInner    `json:"array"`
}

func TestConvertToMapJSONTagsPointersSlicesAndEnums(t *testing.T) {
	value := ctmOuter{
		Name:   "alice",
		Nested: ctmInner{Label: "nested", Count: 2, Secret: "hidden", Bytes: []byte("abc")},
		Ptr:    &ctmInner{Label: "ptr", Count: 3},
		Items:  []ctmInner{{Label: "first"}, {Label: "second", Count: 4}},
		Op:     ctmEnum("add"),
		Tags:   map[string]int{"one": 1},
		Array:  [2]ctmInner{{Label: "a"}, {Label: "b"}},
	}

	got := requireJSONShape(t, value).(map[string]interface{})

	// Spot-check the specific tag behaviors requested by the review.
	if _, ok := got["user_name"]; !ok {
		t.Fatalf("json rename lost: %#v", got)
	}
	if _, ok := got["Name"]; ok {
		t.Fatalf("Go field name leaked: %#v", got)
	}
	if _, ok := got["empty"]; ok {
		t.Fatalf("omitempty zero value emitted: %#v", got)
	}
	nested := got["nested"].(map[string]interface{})
	if _, ok := nested["secret"]; ok {
		t.Fatalf("json:\"-\" field emitted: %#v", nested)
	}
	if _, ok := nested["count"]; !ok || nested["count"] != float64(2) {
		t.Fatalf("nested omitempty/count wrong: %#v", nested)
	}
	if nested["bytes"] != "YWJj" {
		t.Fatalf("byte slice must keep base64 JSON shape: %#v", nested)
	}
	if got["nil_ptr"] != nil {
		t.Fatalf("nil pointer must serialize as null: %#v", got["nil_ptr"])
	}
	if got["op"] != "add" {
		t.Fatalf("named enum type not normalized: %#v", got["op"])
	}
	if _, ok := got["items"].([]interface{}); !ok {
		t.Fatalf("slice not converted: %#v", got["items"])
	}
}

type CTMBase struct {
	ID   string `json:"id"`
	Note string `json:"note,omitempty"`
}

type ctmWithEmbedded struct {
	CTMBase
	Extra string `json:"extra"`
}

type ctmWithTaggedEmbedded struct {
	Base  CTMBase `json:"base"`
	Extra string  `json:"extra"`
}

type ctmUnexportedBase struct {
	ID string `json:"id"`
}

type ctmWithUnexportedEmbedded struct {
	ctmUnexportedBase
	Extra string `json:"extra"`
}

type CTMConflictA struct {
	X string `json:"x"`
}

type CTMConflictB struct {
	X int `json:"x"`
}

type ctmWithConflict struct {
	CTMConflictA
	CTMConflictB
}

func TestConvertToMapEmbeddedFields(t *testing.T) {
	requireJSONShape(t, ctmWithEmbedded{CTMBase: CTMBase{ID: "1", Note: "n"}, Extra: "e"})
	requireJSONShape(t, ctmWithTaggedEmbedded{Base: CTMBase{ID: "2"}, Extra: "e"})
	requireJSONShape(t, ctmWithUnexportedEmbedded{ctmUnexportedBase: ctmUnexportedBase{ID: "3"}, Extra: "e"})
	requireJSONShape(t, ctmWithConflict{CTMConflictA: CTMConflictA{X: "a"}, CTMConflictB: CTMConflictB{X: 1}})
}

func TestConvertToMapKeepsShortCircuits(t *testing.T) {
	if v, err := convertToMap(nil); err != nil || v != nil {
		t.Fatalf("nil: %v, %v", v, err)
	}
	for _, v := range []interface{}{"x", 1, 1.5, true, []interface{}{1, "a"}} {
		got, err := convertToMap(v)
		if err != nil || !reflect.DeepEqual(got, v) {
			t.Fatalf("short-circuit %T changed: %#v, %v", v, got, err)
		}
	}
	if v, err := convertToMap(namedEnum("add")); err != nil || v != "add" {
		t.Fatalf("named primitive: %#v, %v", v, err)
	}
	if v, err := convertToMap([]namedEnum{"add", "multiply"}); err != nil || !reflect.DeepEqual(v, []interface{}{"add", "multiply"}) {
		t.Fatalf("named slice: %#v, %v", v, err)
	}
	var nilPtr *ctmInner
	if v, err := convertToMap(nilPtr); err != nil || v != nil {
		t.Fatalf("nil pointer: %#v, %v", v, err)
	}
}

type ctmBadMarshaler struct{}

func (ctmBadMarshaler) MarshalJSON() ([]byte, error) { return nil, fmt.Errorf("boom") }

func TestConvertToMapMalformedValueErrorUnchanged(t *testing.T) {
	values := []interface{}{
		struct {
			Bad chan int `json:"bad"`
		}{},
		struct {
			Bad func() `json:"bad"`
		}{},
		ctmBadMarshaler{},
	}
	for _, value := range values {
		_, marshalErr := json.Marshal(value)
		if marshalErr == nil {
			t.Fatalf("json.Marshal(%T) unexpectedly succeeded", value)
		}
		_, err := convertToMap(value)
		want := "failed to marshal struct to JSON: " + marshalErr.Error()
		if err == nil || err.Error() != want {
			t.Fatalf("convertToMap(%T) error changed:\n got: %v\nwant: %s", value, err, want)
		}
	}
}

func requireIDL(t *testing.T) *Contract {
	t.Helper()
	raw := `{
		"interfaces": [
			{"name": "Svc", "methods": [
				{"name": "Add", "returnType": {"userDefined": "MathOp"}},
				{"name": "Get", "returnType": {"userDefined": "User"}}
			]}
		],
		"structs": [{"name": "User", "fields": [
			{"name": "user_name", "type": {"builtIn": "string"}},
			{"name": "age", "type": {"builtIn": "int"}}
		]}],
		"enums": [{"name": "MathOp", "values": [{"name": "add"}, {"name": "multiply"}]}]
	}`
	var parsed interface{}
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		t.Fatalf("unmarshal IDL: %v", err)
	}
	return NewContract(parsed)
}

type ctmUser struct {
	UserName string `json:"user_name"`
	Age      int    `json:"age,omitempty"`
}

func TestValidateResponseWithReflectionConvertedStructAndEnum(t *testing.T) {
	contract := requireIDL(t)

	if err := contract.ValidateResponse("Svc", "Add", namedEnum("add")); err != nil {
		t.Fatalf("named enum response rejected: %v", err)
	}
	err := contract.ValidateResponse("Svc", "Add", namedEnum("subtract"))
	if err == nil || !strings.Contains(err.Error(), "Allowed values: [add multiply]") {
		t.Fatalf("invalid enum error changed: %v", err)
	}

	if err := contract.ValidateResponse("Svc", "Get", ctmUser{UserName: "alice", Age: 7}); err != nil {
		t.Fatalf("tagged struct response rejected: %v", err)
	}
	// omitempty drops the zero age, which must become a missing-field error.
	err = contract.ValidateResponse("Svc", "Get", ctmUser{UserName: "alice"})
	if err == nil || !strings.Contains(err.Error(), "missing required field 'age'") {
		t.Fatalf("omitted field error changed: %v", err)
	}
}

func TestValidateResponseMalformedStructErrorUnchanged(t *testing.T) {
	contract := requireIDL(t)
	bad := struct {
		UserName string   `json:"user_name"`
		Age      int      `json:"age"`
		Broken   chan int `json:"broken"`
	}{UserName: "a", Age: 1}
	_, marshalErr := json.Marshal(bad)
	if marshalErr == nil {
		t.Fatal("expected marshal error")
	}
	err := contract.ValidateResponse("Svc", "Get", bad)
	want := "failed to marshal struct to JSON: " + marshalErr.Error()
	if err == nil || err.Error() != "function 'Svc.Get' invalid response: "+want {
		t.Fatalf("response error changed:\n got: %v\nwant: %s", err, "function 'Svc.Get' invalid response: "+want)
	}
}

func fieldNames(fields []map[string]interface{}) []string {
	names := make([]string, 0, len(fields))
	for _, f := range fields {
		if name, ok := f["name"].(string); ok {
			names = append(names, name)
		}
	}
	return names
}

func TestTypeResolverCachesStructFieldsAndEnums(t *testing.T) {
	structs := StructMap{
		"Base": {
			"name": "Base",
			"fields": []interface{}{
				map[string]interface{}{"name": "id", "type": map[string]interface{}{"builtIn": "string"}},
				map[string]interface{}{"name": "label", "type": map[string]interface{}{"builtIn": "string"}},
			},
		},
		"Child": {
			"name": "Child", "extends": "Base",
			"fields": []interface{}{
				map[string]interface{}{"name": "label", "type": map[string]interface{}{"builtIn": "int"}},
				map[string]interface{}{"name": "extra", "type": map[string]interface{}{"builtIn": "bool"}},
			},
		},
	}
	resolver := newTypeResolver(structs, nil)
	first := resolver.getStructFields("Child")
	second := resolver.getStructFields("Child")
	if &first[0] != &second[0] {
		t.Fatal("struct field cache was not reused")
	}
	if got := strings.Join(fieldNames(first), ","); got != "id,label,extra" {
		t.Fatalf("resolved field order: %s", got)
	}
	if typ, _ := first[1]["type"].(map[string]interface{}); typ["builtIn"] != "int" {
		t.Fatalf("child field did not override parent: %#v", first[1])
	}
	if got := strings.Join(fieldNames(GetStructFields("Child", structs)), ","); got != "id,label,extra" {
		t.Fatalf("public helper changed: %s", got)
	}

	// Enum membership is cached by exact and base name, matching resolution.
	enums := EnumMap{
		"a.MathOp": {"name": "a.MathOp", "values": []interface{}{map[string]interface{}{"name": "add"}}},
		"b.MathOp": {"name": "b.MathOp", "values": []interface{}{map[string]interface{}{"name": "multiply"}}},
	}
	er := newTypeResolver(nil, enums)
	if errs := validateEnumEWithSet(namedEnum("add"), "MathOp", "", er.enumSetFor("MathOp", nil)); len(errs) != 0 {
		t.Fatalf("base-name enum cache rejected add: %+v", errs)
	}
	if errs := validateEnumEWithSet(namedEnum("multiply"), "MathOp", "", er.enumSetFor("MathOp", nil)); len(errs) == 0 {
		t.Fatal("base-name enum cache accepted multiply")
	}
	if errs := validateEnumEWithSet(namedEnum("multiply"), "b.MathOp", "", er.enumSetFor("b.MathOp", nil)); len(errs) != 0 {
		t.Fatalf("exact-name enum cache rejected multiply: %+v", errs)
	}
}

func TestInvalidEnumDefinitionErrorsUnchanged(t *testing.T) {
	structs := StructMap{}
	cases := []struct {
		name string
		def  EnumDef
		want string
	}{
		{"A", EnumDef{"name": "A"}, "invalid enum definition for A: missing values"},
		{"B", EnumDef{"name": "B", "values": "add"}, "invalid enum definition for B: values is not a list"},
	}
	for _, tc := range cases {
		err := ValidateType("add", map[string]interface{}{"userDefined": tc.name}, structs, EnumMap{tc.name: tc.def}, false)
		if err == nil || err.Error() != tc.want {
			t.Fatalf("enum definition error changed:\n got: %v\nwant: %s", err, tc.want)
		}
	}
}
