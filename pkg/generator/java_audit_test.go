package generator

import (
	"strings"
	"testing"
)

// Generated Java adapters must build JSON-RPC envelopes without Map.of so null
// ids/data never throw, and must implement notifications, batches, IDL
// validation and response-envelope validation. These are the generated-code
// counterparts of java-01, java-02, java-03, java-04, java-05, java-14, java-15.
func TestGeneratedJavaServerAndClientAuditFixes(t *testing.T) {
	idl := parseTestIDL(t, "namespace test\ninterface Svc { Ping() string }\n")
	server := generateServerJava(idl, nil, map[string]*NamespaceTypes{}, "com.example", "package com.example;", "{}")
	client := generateClientJava(idl, map[string]*NamespaceTypes{}, "com.example", "package com.example;", "{}")

	t.Run("java-01_null_safe_envelopes", func(t *testing.T) {
		for _, fragment := range []string{
			"private Map<String, Object> errorEnvelope(",
			"private Map<String, Object> resultEnvelope(",
			"Map<String, Object> error = errorEnvelope(code, message, null, null);",
			"if (id != null) {",
		} {
			if !strings.Contains(server, fragment) {
				t.Errorf("generated Java server missing null-safe envelope code: %q", fragment)
			}
		}
		if strings.Contains(server, "Map.of(") {
			t.Errorf("generated Java server still uses Map.of, which rejects null values")
		}
	})

	t.Run("java-02_notifications_no_response", func(t *testing.T) {
		for _, fragment := range []string{
			"boolean isNotification = id == null;",
			"if (isNotification) {\n                return null;\n            }",
			"exchange.sendResponseHeaders(200, -1);",
			// The wrapper suppresses error envelopes for notifications too.
			"private Map<String, Object> handleJsonRpcRequest(Map<String, Object> request) {",
			"private Map<String, Object> handleJsonRpcRequestCore(Map<String, Object> request) {",
			"if (isNotification && response != null && response.containsKey(\"error\")) {",
		} {
			if !strings.Contains(server, fragment) {
				t.Errorf("generated Java server missing notification handling: %q", fragment)
			}
		}
	})

	t.Run("java-02_error_envelope_has_id", func(t *testing.T) {
		// Parse errors must carry id:null, and all error envelopes include id.
		if !strings.Contains(server, "response.put(\"id\", id);") {
			t.Errorf("generated Java errorEnvelope does not always include an id")
		}
	})

	t.Run("java-14_unique_request_ids", func(t *testing.T) {
		for _, fragment := range []string{
			"private final java.util.concurrent.atomic.AtomicInteger nextRequestId",
			"int requestId = nextRequestId.incrementAndGet();",
			"\"id\", requestId",
			"Response.validateMap(jsonResponse, requestId);",
		} {
			if !strings.Contains(client, fragment) {
				t.Errorf("generated Java client missing unique request-id code: %q", fragment)
			}
		}
	})

	t.Run("java-03_parse_error_preserved", func(t *testing.T) {
		// Core already fixed this; guard against regressions.
		if !strings.Contains(server, `sendError(exchange, -32700, "Parse error")`) {
			t.Errorf("generated Java server lost -32700 parse-error handling")
		}
	})

	t.Run("java-04_idl_validation", func(t *testing.T) {
		for _, fragment := range []string{
			"new Contract(jsonParser.fromJson(IDL_JSON, Object.class))",
			"contract.validateRequest(idlIfaceName, methodName, paramList)",
			"contract.validateResponse(idlIfaceName, methodName, plainResult)",
		} {
			if !strings.Contains(server, fragment) {
				t.Errorf("generated Java server missing IDL validation: %q", fragment)
			}
		}
		if !strings.Contains(client, "new Contract(jsonParser.fromJson(IDL_JSON, Object.class))") ||
			!strings.Contains(client, "contract.validateResponse(iface, fn, result)") {
			t.Errorf("generated Java client missing IDL response validation")
		}
	})

	t.Run("java-05_server_idl_fetch_preserved", func(t *testing.T) {
		// Core already fixed this; guard against regressions.
		for _, fragment := range []string{
			"public Client withServerIDL(Object idl)",
			`call("pulserpc-idl", new HashMap<>())`,
		} {
			if !strings.Contains(client, fragment) {
				t.Errorf("generated Java client lost server-IDL fetch: %q", fragment)
			}
		}
	})

	t.Run("java-14_envelope_validation", func(t *testing.T) {
		if !strings.Contains(client, "Response.validateMap(jsonResponse, requestId);") {
			t.Errorf("generated Java client does not validate the response envelope")
		}
	})

	t.Run("java-15_batch_requests", func(t *testing.T) {
		for _, fragment := range []string{
			"private Object handleRequestObject(Object parsed)",
			"if (parsed instanceof List)",
			"Invalid Request - empty batch",
			"return responses.isEmpty() ? null : responses;",
		} {
			if !strings.Contains(server, fragment) {
				t.Errorf("generated Java server missing batch support: %q", fragment)
			}
		}
	})
}

// Result validation used to roundtrip every typed result through
// jsonParser.toJson/fromJson before validating it. The roundtrip is only needed
// when the IDL return type contains a user-defined struct/enum.
func TestGeneratedJavaServerConditionalResultConversion(t *testing.T) {
	idl := parseTestIDL(t, "namespace test\ninterface Svc { Ping() string }\n")
	server := generateServerJava(idl, nil, map[string]*NamespaceTypes{}, "com.example", "package com.example;", "{}")

	for _, fragment := range []string{
		"Object plainResult = result;",
		"if (responseNeedsPlainConversion(returnTypeDef)) {",
		`plainResult = jsonParser.fromJson(jsonParser.toJson(result), Object.class);`,
		"private static boolean responseNeedsPlainConversion(Map<String, Object> typeDef) {",
		`if (typeDef.containsKey("userDefined")) return true;`,
		`return responseNeedsPlainConversion((Map<String, Object>) typeDef.get("array"));`,
		`return responseNeedsPlainConversion((Map<String, Object>) typeDef.get("mapValue"));`,
		"contract.validateResponse(idlIfaceName, methodName, plainResult);",
	} {
		if !strings.Contains(server, fragment) {
			t.Errorf("generated Java server missing conditional result conversion: %q", fragment)
		}
	}

	// The conversion must be guarded, not the default assignment.
	if strings.Contains(server, `Object plainResult = jsonParser.fromJson(jsonParser.toJson(result), Object.class);`) {
		t.Error("generated Java server still performs an unconditional JSON roundtrip on every result")
	}
}
