package pulserpc

import (
	"context"
	"encoding/json"
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
		{"name":"OptMeta","fields":[{"name":"note","type":{"builtIn":"string"}}]},
		{"name":"OptItem","fields":[
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
	Note string `json:"note"`
}

type OptItem struct {
	Name     string             `json:"name"`
	Count    int                `json:"count"`
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

func (h *optionsHandler) Add(ctx context.Context, a, b int) (int, error) {
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
	if named["result"] != 5 {
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

	want := []ErrorPhase{ErrorPhaseDecodeParams, ErrorPhaseDecodeParams, ErrorPhaseValidateParams}
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
	if response["result"] != 5 {
		t.Fatalf("json.Number int params result = %#v, want 5", response["result"])
	}

	response = server.Call(map[string]interface{}{
		"jsonrpc": "2.0", "method": "Opt.scale",
		"params": []interface{}{json.Number("1.5")}, "id": 1,
	})
	if response["result"] != 3.0 {
		t.Fatalf("json.Number float params result = %#v, want 3", response["result"])
	}

	for name, number := range map[string]string{
		"fractional":   "1.5",
		"out of range": "4294967296",
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
