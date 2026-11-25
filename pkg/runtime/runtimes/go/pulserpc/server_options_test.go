package pulserpc

import (
	"context"
	"encoding/json"
	"math"
	"reflect"
	"strings"
	"testing"
)

// optionsTestIDL declares the methods and nested structs used by the server
// policy tests.
const optionsTestIDL = `{
	"interfaces": [{
		"name": "Opt",
		"methods": [
			{"name":"ping","parameters":[],"returnType":{"builtIn":"string"}},
			{"name":"echo","parameters":[{"name":"value","type":{"builtIn":"string"}}],"returnType":{"builtIn":"string"}},
			{"name":"add","parameters":[
				{"name":"a","type":{"builtIn":"int"}},
				{"name":"b","type":{"builtIn":"int"}}
			],"returnType":{"builtIn":"int"}},
			{"name":"scale","parameters":[{"name":"x","type":{"builtIn":"float"}}],"returnType":{"builtIn":"float"}},
			{"name":"meta","parameters":[{"name":"item","type":{"userDefined":"OptItem"}}],"returnType":{"builtIn":"string"}}
		]
	}],
	"structs": [
		{"name":"OptBase","fields":[{"name":"label","type":{"builtIn":"string"},"optional":true}]},
		{"name":"OptMeta","extends":"OptBase","fields":[{"name":"note","type":{"builtIn":"string"}}]},
		{"name":"OptItem","namespace":"admin.example","fields":[
			{"name":"name","type":{"builtIn":"string"}},
			{"name":"count","type":{"builtIn":"int"}},
			{"name":"meta","type":{"userDefined":"OptMeta"}},
			{"name":"child","type":{"userDefined":"OptMeta"},"optional":true},
			{"name":"children","type":{"array":{"userDefined":"OptMeta"}}},
			{"name":"labels","type":{"mapValue":{"userDefined":"OptMeta"}}}
		]}
	],
	"enums": []
}`

type OptMeta struct {
	Note  string  `json:"note"`
	Label *string `json:"label,omitempty"`
}

type OptItem struct {
	Name     string             `json:"name"`
	Count    int64              `json:"count"`
	Meta     OptMeta            `json:"meta"`
	Child    *OptMeta           `json:"child"`
	Children []OptMeta          `json:"children"`
	Labels   map[string]OptMeta `json:"labels"`
}

type optionsHandler struct {
	pings int
	last  OptItem
}

func (h *optionsHandler) Ping(ctx context.Context) (string, error) {
	h.pings++
	return "pong", nil
}

func (h *optionsHandler) Echo(ctx context.Context, value string) (string, error) {
	return value, nil
}

func (h *optionsHandler) Add(ctx context.Context, a, b int64) (int64, error) {
	return a + b, nil
}

func (h *optionsHandler) Scale(ctx context.Context, x float64) (float64, error) {
	return x * 2, nil
}

func (h *optionsHandler) Meta(ctx context.Context, item OptItem) (string, error) {
	h.last = item
	return item.Name, nil
}

func newOptionsServer(t *testing.T, config ServerConfig) (*Server, *optionsHandler) {
	t.Helper()
	var idl interface{}
	if err := json.Unmarshal([]byte(optionsTestIDL), &idl); err != nil {
		t.Fatalf("unmarshal IDL: %v", err)
	}
	handler := &optionsHandler{}
	server := NewServerWithConfig(NewContract(idl), true, true, config)
	if err := server.AddHandler("Opt", handler); err != nil {
		t.Fatalf("AddHandler: %v", err)
	}
	return server, handler
}

func errorData(t *testing.T, response map[string]interface{}) (interface{}, bool) {
	t.Helper()
	errObj, ok := response["error"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected an error response, got %#v", response)
	}
	data, exists := errObj["data"]
	return data, exists
}

// The default policy keeps the v0.4.0 behavior: batches, named params, and
// notifications are all accepted, and Config returns the applied values.
func TestDefaultServerConfigIsLenient(t *testing.T) {
	server, _ := newOptionsServer(t, DefaultServerConfig())
	config := server.Config()
	if !config.AllowBatch || config.RequireNonNullID ||
		config.ParamsMode != ParamsModeAny || config.MaxRequestBodyBytes != 0 ||
		config.RejectInvalidUTF8 || config.RejectUnknownRequestFields || config.MapError != nil {
		t.Fatalf("DefaultServerConfig() = %#v", config)
	}

	// Named params are accepted.
	named := server.Call(map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "Opt.add",
		"params":  map[string]interface{}{"a": 2, "b": 3},
		"id":      1,
	})
	if named["result"] != int64(5) {
		t.Fatalf("named params result = %#v, want 5", named["result"])
	}

	// Notifications stay silent even when validation fails.
	if response := server.Call(map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "Opt.add",
		"params":  []interface{}{"bad", 2},
	}); response != nil {
		t.Fatalf("notification received a response: %#v", response)
	}
}

// RequireNonNullID turns absent and null ids into -32600 errors instead of
// notifications, and the handler must not run.
func TestRequireNonNullIDRejectsMissingAndNullIDs(t *testing.T) {
	config := DefaultServerConfig()
	config.RequireNonNullID = true
	server, handler := newOptionsServer(t, config)

	for name, request := range map[string]map[string]interface{}{
		"absent": {"jsonrpc": "2.0", "method": "Opt.ping", "params": []interface{}{}},
		"null":   {"jsonrpc": "2.0", "method": "Opt.ping", "params": []interface{}{}, "id": nil},
	} {
		response := server.Call(request)
		if response == nil {
			t.Fatalf("%s id: expected an error response, got nil", name)
		}
		if code := errorCode(t, response); code != -32600 {
			t.Errorf("%s id: code = %d, want -32600", name, code)
		}
		if responseID, exists := response["id"]; !exists || responseID != nil {
			t.Errorf("%s id: response id = %#v (exists=%t), want explicit null", name, responseID, exists)
		}
	}
	if handler.pings != 0 {
		t.Errorf("handler ran %d times for requests without an id", handler.pings)
	}
}

// PositionalOnly rejects object params with -32600 while absent/null params
// still mean zero arguments.
func TestParamsModePositionalOnly(t *testing.T) {
	config := DefaultServerConfig()
	config.ParamsMode = ParamsModePositionalOnly
	server, handler := newOptionsServer(t, config)

	response := server.Call(map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "Opt.add",
		"params":  map[string]interface{}{"a": 1, "b": 2},
		"id":      1,
	})
	if code := errorCode(t, response); code != -32600 {
		t.Errorf("named params code = %d, want -32600", code)
	}

	// Absent and explicit null params both mean zero arguments.
	for _, request := range []map[string]interface{}{
		{"jsonrpc": "2.0", "method": "Opt.ping", "id": 2},
		{"jsonrpc": "2.0", "method": "Opt.ping", "params": nil, "id": 3},
	} {
		result := server.Call(request)
		if result["result"] != "pong" {
			t.Errorf("zero-arg request %#v: result = %#v, want pong", request, result["result"])
		}
	}
	if handler.pings != 2 {
		t.Errorf("zero-arg handler ran %d times, want 2", handler.pings)
	}
}

func fullItemMap() map[string]interface{} {
	return map[string]interface{}{
		"name":     "widget",
		"count":    3,
		"meta":     map[string]interface{}{"note": "n"},
		"child":    map[string]interface{}{"note": "p"},
		"children": []interface{}{map[string]interface{}{"note": "c"}},
		"labels":   map[string]interface{}{"k": map[string]interface{}{"note": "l"}},
	}
}

func itemRequest(item map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "Opt.meta",
		"params":  []interface{}{item},
		"id":      1,
	}
}

// RejectUnknownRequestFields rejects unknown keys at the struct top level and
// recursively through nested structs, pointers, slices, and maps; it is off by
// default.
func TestRejectUnknownRequestFields(t *testing.T) {
	config := DefaultServerConfig()
	config.RejectUnknownRequestFields = true
	server, _ := newOptionsServer(t, config)

	cases := map[string]func(map[string]interface{}){
		"top level": func(item map[string]interface{}) { item["bogus"] = true },
		"nested struct": func(item map[string]interface{}) {
			item["meta"] = map[string]interface{}{"note": "n", "bogus": 1}
		},
		"pointer": func(item map[string]interface{}) {
			item["child"] = map[string]interface{}{"note": "p", "bogus": 1}
		},
		"slice element": func(item map[string]interface{}) {
			item["children"] = []interface{}{map[string]interface{}{"note": "c", "bogus": 1}}
		},
		"map value": func(item map[string]interface{}) {
			item["labels"] = map[string]interface{}{"k": map[string]interface{}{"note": "l", "bogus": 1}}
		},
	}
	for name, mutate := range cases {
		item := fullItemMap()
		mutate(item)
		response := server.Call(itemRequest(item))
		if code := errorCode(t, response); code != -32602 {
			t.Errorf("%s: code = %d, want -32602", name, code)
		}
		data, _ := errorData(t, response)
		message, _ := data.(string)
		if !strings.Contains(message, "unknown field \"bogus\"") {
			t.Errorf("%s: data = %#v, want an unknown field error", name, data)
		}
	}

	// Known fields are accepted.
	response := server.Call(itemRequest(fullItemMap()))
	if response["result"] != "widget" {
		t.Fatalf("known fields result = %#v, want widget", response["result"])
	}

	// Off by default: the same unknown key is ignored.
	lenient, _ := newOptionsServer(t, DefaultServerConfig())
	item := fullItemMap()
	item["bogus"] = true
	response = lenient.Call(itemRequest(item))
	if response["result"] != "widget" {
		t.Fatalf("default config rejected an unknown field: %#v", response)
	}
}

// MapError receives the phase, returns the configured error without validation
// details, and nil falls back to the standard -32602 response.
func TestMapErrorPhasesAndFallback(t *testing.T) {
	var phases []ErrorPhase
	config := DefaultServerConfig()
	config.RejectUnknownRequestFields = true
	config.MapError = func(phase ErrorPhase, err error) *RPCError {
		phases = append(phases, phase)
		return NewRPCError(4101, "Invalid input")
	}
	server, _ := newOptionsServer(t, config)

	// Decode failure: an unknown named parameter cannot be converted.
	response := server.Call(map[string]interface{}{
		"jsonrpc": "2.0", "method": "Opt.add",
		"params": map[string]interface{}{"a": 1, "b": 2, "c": 3}, "id": 1,
	})
	if code := errorCode(t, response); code != 4101 {
		t.Fatalf("decode code = %d, want 4101", code)
	}
	if _, exists := errorData(t, response); exists {
		t.Errorf("mapped error must not carry data: %#v", response)
	}

	// Decode failure: unknown field in a struct param.
	item := fullItemMap()
	item["bogus"] = true
	response = server.Call(itemRequest(item))
	if code := errorCode(t, response); code != 4101 {
		t.Fatalf("unknown field code = %d, want 4101", code)
	}

	// Decode failure: parameter count mismatch.
	response = server.Call(map[string]interface{}{
		"jsonrpc": "2.0", "method": "Opt.add", "params": []interface{}{1}, "id": 1,
	})
	if code := errorCode(t, response); code != 4101 {
		t.Fatalf("count mismatch code = %d, want 4101", code)
	}

	// Validation failure.
	response = server.Call(map[string]interface{}{
		"jsonrpc": "2.0", "method": "Opt.add", "params": []interface{}{"bad", 2}, "id": 1,
	})
	if code := errorCode(t, response); code != 4101 {
		t.Fatalf("validation code = %d, want 4101", code)
	}
	errObj := response["error"].(map[string]interface{})
	if errObj["message"] != "Invalid input" {
		t.Errorf("mapped message = %#v, want Invalid input", errObj["message"])
	}

	want := []ErrorPhase{ErrorPhaseDecodeParams, ErrorPhaseDecodeParams, ErrorPhaseDecodeParams, ErrorPhaseValidateParams}
	if len(phases) != len(want) {
		t.Fatalf("phases = %#v, want %#v", phases, want)
	}
	for i := range want {
		if phases[i] != want[i] {
			t.Errorf("phase[%d] = %q, want %q", i, phases[i], want[i])
		}
	}

	// Envelope protocol errors are not mapped.
	response = server.Call(map[string]interface{}{
		"jsonrpc": "2.0", "method": "Opt.add", "params": map[string]interface{}{"a": 1, "b": 2}, "id": true,
	})
	if code := errorCode(t, response); code != -32600 {
		t.Errorf("invalid id code = %d, want -32600", code)
	}

	// nil falls back to the standard response.
	fallbackConfig := DefaultServerConfig()
	fallbackConfig.MapError = func(phase ErrorPhase, err error) *RPCError { return nil }
	fallback, _ := newOptionsServer(t, fallbackConfig)
	response = fallback.Call(map[string]interface{}{
		"jsonrpc": "2.0", "method": "Opt.add", "params": []interface{}{1}, "id": 1,
	})
	if code := errorCode(t, response); code != -32602 {
		t.Errorf("fallback code = %d, want -32602", code)
	}
	if data, ok := errorData(t, response); !ok || data == nil {
		t.Errorf("fallback response lost its standard data: %#v", response)
	}
}

// A json.Number id must round-trip through the response exactly.
func TestNumericIDRoundTripIsExact(t *testing.T) {
	server, _ := newOptionsServer(t, DefaultServerConfig())
	request := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "Opt.ping",
		"params":  []interface{}{},
		"id":      json.Number("9007199254740993"),
	}
	response := server.Call(request)
	raw, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("marshal response: %v", err)
	}
	if !strings.Contains(string(raw), `"id":9007199254740993`) {
		t.Fatalf("response id lost precision: %s", raw)
	}
}

// Validation and conversion accept json.Number for int and float params while
// still rejecting fractional and out-of-range ints.
func TestJSONNumberParams(t *testing.T) {
	server, _ := newOptionsServer(t, DefaultServerConfig())

	response := server.Call(map[string]interface{}{
		"jsonrpc": "2.0", "method": "Opt.add",
		"params": []interface{}{json.Number("2"), json.Number("3")}, "id": 1,
	})
	if response["result"] != int64(5) {
		t.Fatalf("json.Number int params result = %#v, want 5", response["result"])
	}

	// The full int64 range converts exactly.
	response = server.Call(map[string]interface{}{
		"jsonrpc": "2.0", "method": "Opt.add",
		"params": []interface{}{json.Number("9223372036854775807"), json.Number("0")}, "id": 1,
	})
	if response["result"] != int64(math.MaxInt64) {
		t.Fatalf("max int64 json.Number result = %#v, want %d", response["result"], int64(math.MaxInt64))
	}

	response = server.Call(map[string]interface{}{
		"jsonrpc": "2.0", "method": "Opt.scale",
		"params": []interface{}{json.Number("1.5")}, "id": 1,
	})
	if response["result"] != 3.0 {
		t.Fatalf("json.Number float params result = %#v, want 3", response["result"])
	}

	for name, number := range map[string]string{
		"fractional":        "1.5",
		"above int64 range": "9223372036854775808",
		"below int64 range": "-9223372036854775809",
	} {
		response = server.Call(map[string]interface{}{
			"jsonrpc": "2.0", "method": "Opt.add",
			"params": []interface{}{json.Number(number), 1}, "id": 1,
		})
		if code := errorCode(t, response); code != -32602 {
			t.Errorf("%s json.Number code = %d, want -32602", name, code)
		}
	}
}

// A large int64 result decoded by the client stays json.Number; response
// validation must accept it as an int rather than treating it as a string.
func TestValidateResponseAcceptsLargeJSONNumberInt(t *testing.T) {
	server, _ := newOptionsServer(t, DefaultServerConfig())
	if err := server.contract.ValidateResponse("Opt", "add", json.Number("9223372036854775807")); err != nil {
		t.Fatalf("ValidateResponse rejected a large json.Number int: %v", err)
	}
	if err := server.contract.ValidateResponse("Opt", "add", json.Number("9223372036854775808")); err == nil {
		t.Fatal("ValidateResponse accepted an out-of-range json.Number int")
	}
}

// The adapter keeps the top-level id as json.Number and normalizes params (and
// nested maps, including a param literally named "id") to float64 where the
// conversion is exact. Integers above 2^53 stay json.Number so they are not
// corrupted.
func TestNormalizeRequestNumbersPreservesOnlyRequestID(t *testing.T) {
	request := map[string]interface{}{
		"id":     json.Number("9007199254740993"),
		"params": []interface{}{json.Number("1"), map[string]interface{}{"id": json.Number("2")}},
	}
	NormalizeRequestNumbers(request)

	if _, ok := request["id"].(json.Number); !ok {
		t.Fatalf("request id = %T, want json.Number", request["id"])
	}
	params := request["params"].([]interface{})
	if _, ok := params[0].(float64); !ok {
		t.Fatalf("param[0] = %T, want float64", params[0])
	}
	nested := params[1].(map[string]interface{})
	if _, ok := nested["id"].(float64); !ok {
		t.Fatalf("nested id = %T, want float64", nested["id"])
	}

	for _, text := range []string{"9007199254740993", "9223372036854775807", "-9223372036854775808", "9223372036854775808"} {
		large := map[string]interface{}{"params": []interface{}{json.Number(text)}}
		NormalizeRequestNumbers(large)
		if got := large["params"].([]interface{})[0]; got != json.Number(text) {
			t.Fatalf("large param %s = %T(%v), want json.Number", text, got, got)
		}
	}

	batch := []interface{}{map[string]interface{}{"id": json.Number("3"), "params": []interface{}{json.Number("4")}}}
	NormalizeRequestNumbers(batch)
	member := batch[0].(map[string]interface{})
	if _, ok := member["id"].(json.Number); !ok {
		t.Fatalf("batch member id = %T, want json.Number", member["id"])
	}
	if _, ok := member["params"].([]interface{})[0].(float64); !ok {
		t.Fatalf("batch param = %T, want float64", member["params"].([]interface{})[0])
	}
}

// Client responses keep an exact id while result/error numbers stay float64
// where the conversion is exact. Large integers stay json.Number.
func TestDecodeResponsePreservesIDAndNormalizesResult(t *testing.T) {
	response, err := DecodeResponse([]byte(`{"jsonrpc":"2.0","id":9007199254740993,"result":{"n":9007199254740993}}`))
	if err != nil {
		t.Fatalf("DecodeResponse: %v", err)
	}
	if _, ok := response["id"].(json.Number); !ok {
		t.Fatalf("id = %T, want json.Number", response["id"])
	}
	n := response["result"].(map[string]interface{})["n"]
	if _, ok := n.(json.Number); !ok {
		t.Fatalf("large result number = %T, want json.Number", n)
	}

	small, err := DecodeResponse([]byte(`{"jsonrpc":"2.0","id":1,"result":{"n":42}}`))
	if err != nil {
		t.Fatalf("DecodeResponse: %v", err)
	}
	if _, ok := small["result"].(map[string]interface{})["n"].(float64); !ok {
		t.Fatalf("small result number = %T, want float64", small["result"].(map[string]interface{})["n"])
	}

	if _, err := DecodeResponse([]byte("{\"jsonrpc\":\"2.0\",\"id\":1,\"result\":null}\n")); err != nil {
		t.Fatalf("DecodeResponse rejected trailing whitespace: %v", err)
	}
	if _, err := DecodeResponse([]byte(`{"jsonrpc":"2.0","id":1} {"jsonrpc":"2.0","id":2}`)); err == nil {
		t.Fatal("DecodeResponse accepted trailing data")
	}
}

// json.Number has string kind; conversion must not silently turn a JSON number
// into a string.
func TestJSONNumberDoesNotConvertToString(t *testing.T) {
	server := &Server{}
	converted := server.convertValueToType(reflect.ValueOf(json.Number("123")), reflect.TypeOf(""))
	if converted.Kind() != reflect.String || converted.String() != "" {
		t.Fatalf("json.Number -> string = %#v, want empty string", converted.Interface())
	}
}

// json.Number conversion covers the full int64 range and refuses integer text
// outside it instead of rounding through float64.
func TestConvertJSONNumberInt64Bounds(t *testing.T) {
	server := &Server{}
	target := reflect.TypeOf(int64(0))

	for _, tc := range []struct {
		text string
		want int64
	}{
		{"9223372036854775807", math.MaxInt64},
		{"-9223372036854775808", math.MinInt64},
		{"9007199254740993", 9007199254740993},
		{"5.0", 5},
		{"9007199254740993e0", 9007199254740993},
		{"9223372036854775807.0", math.MaxInt64},
		{"-92233720368547758080e-1", math.MinInt64},
	} {
		got := server.convertValueToType(reflect.ValueOf(json.Number(tc.text)), target)
		if got.Kind() != reflect.Int64 || got.Int() != tc.want {
			t.Errorf("json.Number(%q) -> %v, want int64 %d", tc.text, got.Interface(), tc.want)
		}
	}

	for _, text := range []string{"9223372036854775808", "-9223372036854775809", "9223372036854775808.0", "9007199254740992.5"} {
		got := server.convertValueToType(reflect.ValueOf(json.Number(text)), target)
		if got.Int() != 0 {
			t.Errorf("json.Number(%q) -> %v, want the int64 zero value", text, got.Interface())
		}
	}
}

// Large integer ids compare exactly across the runtime's uint64 request ids and
// json.Number echoes, while mixed float64 pairs keep their legacy behavior.
func TestExactNumericIDComparison(t *testing.T) {
	if !idsEqual(uint64(9007199254740993), json.Number("9007199254740993")) {
		t.Error("uint64/json.Number large ids did not compare equal")
	}
	if idsEqual(uint64(9007199254740993), json.Number("9007199254740992")) {
		t.Error("distinct large ids compared equal")
	}
	if !idsEqual(7, 7.0) || !idsEqual(json.Number("7.1"), 7.1) {
		t.Error("mixed numeric id comparisons no longer match v0.4.0 behavior")
	}
}

func TestMapMethodErrorSelectsMethodAndFallsThrough(t *testing.T) {
	config := DefaultServerConfig()
	config.MapMethodError = func(method string, phase ErrorPhase, err error) *RPCError {
		if err == nil {
			t.Fatal("expected parameter failure")
		}
		if method == "Opt.add" {
			if phase == ErrorPhaseDecodeParams {
				return NewRPCError(1001, "wrong argument shape")
			}
			return NewRPCError(1002, "wrong argument value")
		}
		if method != itemRequest(fullItemMap())["method"] {
			t.Fatalf("unexpected mapped method %q", method)
		}
		return nil
	}
	config.MapError = func(ErrorPhase, error) *RPCError {
		return NewRPCError(4101, "Invalid input")
	}
	server, _ := newOptionsServer(t, config)
	call := func(method any, params any, id any) map[string]interface{} {
		return server.Call(map[string]interface{}{
			"jsonrpc": "2.0", "method": method, "params": params, "id": id,
		})
	}
	for _, test := range []struct {
		method any
		params any
		id     any
		want   int
	}{
		{"Opt.add", []any{1}, 7, 1001},
		{"Opt.add", []any{1, "bad"}, 7, 1002},
		{itemRequest(fullItemMap())["method"], []any{1}, 7, 4101},
		{"Opt.notRegistered", []any{1}, 7, -32601},
		{"Opt.add", []any{1}, true, -32600},
	} {
		if got := errorCode(t, call(test.method, test.params, test.id)); got != test.want {
			t.Errorf("method %v with params %v and ID %v: got %d, want %d", test.method, test.params, test.id, got, test.want)
		}
	}

	config.MapMethodError = func(string, ErrorPhase, error) *RPCError { return nil }
	config.MapError = func(ErrorPhase, error) *RPCError { return nil }
	server, _ = newOptionsServer(t, config)
	if got := errorCode(t, call("Opt.add", []any{1}, 7)); got != -32602 {
		t.Errorf("both mappers declined: got %d, want -32602", got)
	}

}

func TestRejectNullRequestFieldDistinguishesOmittedFromNull(t *testing.T) {
	config := DefaultServerConfig()
	config.RejectNullRequestField = func(structName, fieldName string) bool {
		return structName == "admin.example.OptItem" && fieldName == "child"
	}
	server, _ := newOptionsServer(t, config)
	for _, test := range []struct {
		name  string
		child any
	}{
		{"object", map[string]any{"note": "valid"}},
		{"null", nil},
	} {
		t.Run(test.name, func(t *testing.T) {
			item := fullItemMap()
			item["child"] = test.child
			response := server.Call(itemRequest(item))
			if test.child == nil {
				if got := errorCode(t, response); got != -32602 {
					t.Fatalf("explicit null: got %d, want -32602", got)
				}
				if err := server.contract.ValidateRequest("Opt", "meta", []any{item}); err != nil {
					t.Fatalf("direct validation used server policy: %v", err)
				}
			} else if response["result"] != "widget" {
				t.Fatalf("valid object: got %#v", response)
			}
		})
	}
	item := fullItemMap()
	delete(item, "child")
	if response := server.Call(itemRequest(item)); response["result"] != "widget" {
		t.Fatalf("omitted optional field: got %#v", response)
	}

	config.RejectNullRequestField = func(string, string) bool { return false }
	other, _ := newOptionsServer(t, config)
	item["child"] = nil
	if response := other.Call(itemRequest(item)); response["result"] != "widget" {
		t.Fatalf("unselected optional field: got %#v", response)
	}
}

func TestRequireIDLFieldNamesRejectsAliasesInNestedRequests(t *testing.T) {
	config := DefaultServerConfig()
	config.RequireIDLFieldNames = true
	strict, _ := newOptionsServer(t, config)
	legacyConfig := DefaultServerConfig()
	legacyConfig.RejectUnknownRequestFields = true
	legacy, _ := newOptionsServer(t, legacyConfig)

	item := fullItemMap()
	item["labels"].(map[string]any)["arbitrary map key"] = map[string]any{"note": "value", "label": "inherited"}
	if response := strict.Call(itemRequest(item)); response["result"] != "widget" {
		t.Fatalf("canonical field and arbitrary map key should pass: %#v", response)
	}

	for _, mutate := range []func(map[string]any){
		func(data map[string]any) { data["Name"] = "alias" },
		func(data map[string]any) { data["meta"].(map[string]any)["Label"] = "alias" },
		func(data map[string]any) { data["children"].([]any)[0].(map[string]any)["Note"] = "alias" },
		func(data map[string]any) { data["labels"].(map[string]any)["k"].(map[string]any)["Label"] = "alias" },
	} {
		item := fullItemMap()
		mutate(item)
		if code := errorCode(t, strict.Call(itemRequest(item))); code != -32602 {
			t.Fatalf("alias was not rejected: %d", code)
		}
		if response := legacy.Call(itemRequest(item)); response["result"] != "widget" {
			t.Fatalf("legacy alias behavior changed: %#v", response)
		}
	}
}

func TestQualifiedInterfaceRegistrationKeepsWireName(t *testing.T) {
	var idl any
	raw := strings.Replace(optionsTestIDL, `"name": "Opt",`, `"name": "Opt", "namespace": "example.api",`, 1)
	if err := json.Unmarshal([]byte(raw), &idl); err != nil {
		t.Fatal(err)
	}
	contract := NewContract(idl)
	if !contract.HasInterface("Opt") || !contract.HasInterface("example.api.Opt") ||
		contract.HasInterface("other.api.Opt") || len(contract.Interfaces) != 1 {
		t.Fatal("qualified lookup changed the declared contract")
	}
	server := NewServer(contract, true, true)
	if err := server.AddHandler("example.api.Opt", &optionsHandler{}); err != nil {
		t.Fatal(err)
	}
	call := func(name string, params []any) map[string]any {
		return server.Call(map[string]any{"jsonrpc": "2.0", "id": 17, "method": name, "params": params})
	}
	if response := call("example.api.Opt.ping", nil); response["result"] != "pong" || response["id"] != 17 {
		t.Fatalf("qualified result: %#v", response)
	}
	if code := errorCode(t, call("Opt.ping", nil)); code != -32601 {
		t.Fatalf("unregistered bare name: got %d, want -32601", code)
	}
	if code := errorCode(t, call("example.api.Opt.add", []any{"bad", 2})); code != -32602 {
		t.Fatalf("qualified parameter validation: got %d, want -32602", code)
	}
}

func TestQualifiedInterfaceLookupPrefersExactDeclaration(t *testing.T) {
	var idl any
	if err := json.Unmarshal([]byte(`{"interfaces":[
		{"name":"Example","namespace":"app","methods":[{"name":"root"}]},
		{"name":"app.Example","namespace":"app","methods":[{"name":"exact"}]}
	]}`), &idl); err != nil {
		t.Fatal(err)
	}
	contract := NewContract(idl)
	if iface := contract.GetInterface("app.Example"); iface == nil || iface.GetFunction("exact") == nil {
		t.Fatalf("expected exact declaration, got %#v", iface)
	}
}
