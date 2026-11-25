package pulserpc

import (
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"math/rand"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func TestExactJSONInt64(t *testing.T) {
	for _, tc := range []struct {
		token string
		want  int64
	}{
		{"1e0", 1},
		{"0.000e999999999999999999999999", 0},
		{"-0e-999999999999999999999999", 0},
		{"0.001e+3", 1},
		{"100e-2", 1},
		{"1.2300e2", 123},
		{"9007199254740993.0", 9007199254740993},
		{"922337203685477580.7e1", math.MaxInt64},
		{"-92233720368547758080e-1", math.MinInt64},
	} {
		if got, ok := exactJSONInt64(json.Number(tc.token)); !ok || got != tc.want {
			t.Errorf("exactJSONInt64(%q) = %d, %t, want %d", tc.token, got, ok, tc.want)
		}
	}
	for _, tc := range []string{
		"9223372036854775808.0", "-9223372036854775809e0",
		"9007199254740992.5", "1.000000000000000000001",
		"1e-999999999999999999999999", "1e999999999999999999999999",
		"0e+", "1e", "1.00.1",
	} {
		if got, ok := exactJSONInt64(json.Number(tc)); ok {
			t.Errorf("exactJSONInt64(%q) = %d, want rejection", tc, got)
		}
	}
}

func TestNormalizeJSONNumbersRetainsPrecisionForIntegerValidation(t *testing.T) {
	for _, tc := range []struct {
		token string
		want  interface{}
	}{
		{"1e0", float64(1)},
		{"3.5", float64(3.5)},
		{"9007199254740993e0", json.Number("9007199254740993")},
		{"9223372036854775807.0", json.Number("9223372036854775807")},
		{"9007199254740992.5", json.Number("9007199254740992.5")},
		{"1.000000000000000000001", json.Number("1.000000000000000000001")},
	} {
		if got := normalizeJSONNumbers(json.Number(tc.token)); got != tc.want {
			t.Errorf("normalizeJSONNumbers(%q) = %T(%v), want %T(%v)", tc.token, got, got, tc.want, tc.want)
		}
	}

	for _, token := range []string{"-0.0", "-0e0"} {
		if got, ok := normalizeJSONNumbers(json.Number(token)).(float64); !ok || !math.Signbit(got) {
			t.Errorf("normalized token %s lost negative zero", token)
		}
	}

	data := []byte(`{"jsonrpc":"2.0","id":1,"result":{"key":9223372036854775807.0}}`)
	response, err := DecodeResponse(data)
	if err != nil {
		t.Fatal(err)
	}
	result := response["result"].(map[string]interface{})
	if result["key"] != json.Number("9223372036854775807") {
		t.Fatalf("lost exact key: %#v", result)
	}
}

func TestExactJSONInt64AgainstRationalArithmetic(t *testing.T) {
	random := rand.New(rand.NewSource(42))
	for range 1024 {
		digits := strconv.FormatInt(random.Int63n(math.MaxInt64-1)+1, 10)
		index := 1 + random.Intn(len(digits))
		token := digits
		if index < len(digits) {
			token = digits[:index] + "." + digits[index:]
		}
		if random.Intn(2) == 0 {
			token = "-" + token
		}
		token = fmt.Sprintf("%se%d", token, random.Intn(49)-24)
		rational, ok := new(big.Rat).SetString(token)
		if !ok {
			t.Fatalf("invalid generated number %q", token)
		}
		want := rational.IsInt() && rational.Num().IsInt64()
		if result, got := exactJSONInt64(json.Number(token)); got != want || got && result != rational.Num().Int64() {
			t.Fatalf("exactJSONInt64(%q) = %d, %t; rational %v", token, result, got, rational)
		}
	}
}

func TestConvertJSONNumberPreservesOptionalWideInteger(t *testing.T) {
	target := reflect.TypeOf((*int64)(nil))
	server := &Server{}
	for _, token := range []string{"9007199254740993", "9007199254740993e0", "9007199254740993.0"} {
		got := server.convertValueToType(reflect.ValueOf(json.Number(token)), target)
		ptr := got.Interface().(*int64)
		if ptr == nil || *ptr != 9007199254740993 {
			t.Errorf("optional int64 lost %s: %v", token, ptr)
		}
	}
	float := server.convertValueToType(reflect.ValueOf(json.Number("1.000000000000000000001")), reflect.TypeOf((*float64)(nil))).Interface().(*float64)
	if float == nil || *float != 1 {
		t.Errorf("optional float was not converted: %v", float)
	}
	if got := server.convertValueToType(reflect.ValueOf(json.Number("9007199254740992.5")), target); !got.IsNil() {
		t.Errorf("unexpected pointer for a fractional number: %v", got.Interface())
	}
}

func TestValidateIntRejectsFloatsThatOnlyLookIntegral(t *testing.T) {
	for _, token := range []string{"9007199254740992.5", "1.000000000000000000001"} {
		value := normalizeJSONNumbers(json.Number(token))
		if err := ValidateInt(value); err == nil || !strings.Contains(err.Error(), token) {
			t.Errorf("ValidateInt(%v) = %v", value, err)
		}
		if err := ValidateFloat(value); err != nil {
			t.Errorf("ValidateFloat(%v) = %v", value, err)
		}
	}
}

func TestNormalizeJSONNumbersWithCompensatingExponent(t *testing.T) {
	for _, token := range []string{
		"0." + strings.Repeat("0", 100000) + "1e100001",
		"1" + strings.Repeat("0", 100001) + "e-100001",
	} {
		if got := normalizeJSONNumbers(json.Number(token)); got != float64(1) {
			t.Fatalf("unexpected normalized %T result for %d-byte token", got, len(token))
		}
	}
}
