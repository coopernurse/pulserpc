import com.bitmechanic.pulserpc.*;
import org.junit.Test;
import org.junit.Assert;
import java.util.*;

public class ServerTest {

    public static class Handler {
        public String oneArg(String x) {
            return "got:" + x;
        }

        public String requiredReturn() {
            return null;
        }

        public String optionalReturn() {
            return null;
        }

        public String takeUser(Map<String, Object> user) {
            return "ok";
        }

        public String explode() {
            throw new IllegalStateException("secret-token=abc123 at com.example.Handler.explode");
        }
    }

    public static class LeakyHandler {
        public String leaky() {
            return "not a struct";
        }
    }

    private Contract contract() {
        Map<String, Object> idl = new HashMap<>();
        idl.put("interfaces", Arrays.asList(Map.of(
            "name", "Svc",
            "methods", Arrays.asList(
                Map.of("name", "oneArg",
                       "parameters", Arrays.asList(Map.of("name", "x", "type", Map.of("builtIn", "string"))),
                       "returnType", Map.of("builtIn", "string")),
                Map.of("name", "requiredReturn", "returnType", Map.of("builtIn", "string")),
                Map.of("name", "optionalReturn", "returnType", Map.of("builtIn", "string"), "returnOptional", true),
                Map.of("name", "takeUser",
                       "parameters", Arrays.asList(Map.of("name", "user", "type", Map.of("userDefined", "inc.User"))),
                       "returnType", Map.of("builtIn", "string")),
                Map.of("name", "explode", "returnType", Map.of("builtIn", "string"))
            )
        )));
        idl.put("structs", Arrays.asList(Map.of(
            "name", "inc.User",
            "fields", Arrays.asList(Map.of("name", "email", "type", Map.of("builtIn", "string")))
        )));
        return new Contract(idl);
    }

    private Map<String, Object> request(String method, Object params) {
        Map<String, Object> req = new HashMap<>();
        req.put("jsonrpc", "2.0");
        req.put("method", method);
        req.put("id", 1);
        if (params != null) {
            req.put("params", params);
        }
        return req;
    }

    private int errorCode(Map<String, Object> response) {
        Assert.assertNotNull(response);
        Assert.assertTrue("expected an error response: " + response, response.containsKey("error"));
        @SuppressWarnings("unchecked")
        Map<String, Object> error = (Map<String, Object>) response.get("error");
        return ((Number) error.get("code")).intValue();
    }

    @Test
    public void testZeroParamsStillValidatesCount() {
        // java-09: a method declared with one parameter cannot be called with none.
        Server server = new Server(contract(), true, false);
        server.addHandler("Svc", new Handler());
        Assert.assertEquals(-32602, errorCode(server.call(request("Svc.oneArg", null))));
        Assert.assertEquals(-32602, errorCode(server.call(request("Svc.oneArg", new ArrayList<>()))));
    }

    @Test
    public void testObjectFormParamsRejected() {
        // java-09: object-form params must be rejected, not silently dropped.
        Server server = new Server(contract(), true, false);
        server.addHandler("Svc", new Handler());
        Assert.assertEquals(-32602, errorCode(server.call(request("Svc.oneArg", Map.of("x", "hi")))));
    }

    @Test
    public void testValidParamsStillWork() {
        Server server = new Server(contract(), true, false);
        server.addHandler("Svc", new Handler());
        Map<String, Object> response = server.call(request("Svc.oneArg", Arrays.asList("hi")));
        Assert.assertNotNull(response);
        Assert.assertEquals("got:hi", response.get("result"));
    }

    @Test
    public void testMissingRequiredStructFieldRejected() {
        // java-04: a struct param missing a required field is rejected with -32602.
        Server server = new Server(contract(), true, false);
        server.addHandler("Svc", new Handler());
        Assert.assertEquals(-32602,
            errorCode(server.call(request("Svc.takeUser", Arrays.asList(new HashMap<>())))));
        Map<String, Object> response = server.call(
            request("Svc.takeUser", Arrays.asList(Map.of("email", "a@example.com"))));
        Assert.assertEquals("ok", response.get("result"));
    }

    @Test
    public void testNullResultForRequiredReturnIsRejected() {
        // java-10: null for a non-optional return must reach validateResponse.
        Server server = new Server(contract(), true, true);
        server.addHandler("Svc", new Handler());
        Assert.assertEquals(-32603, errorCode(server.call(request("Svc.requiredReturn", null))));
    }

    @Test
    public void testNullResultForOptionalReturnSucceeds() {
        Server server = new Server(contract(), true, true);
        server.addHandler("Svc", new Handler());
        Map<String, Object> response = server.call(request("Svc.optionalReturn", null));
        Assert.assertNotNull(response);
        Assert.assertTrue(response.containsKey("result"));
        Assert.assertNull(response.get("result"));
    }

    @Test
    public void testInternalErrorIsRedacted() {
        // Invariant R15/D4: handler exceptions are logged server-side and the
        // wire only sees the fixed message with no data.
        Server server = new Server(contract(), true, false);
        server.addHandler("Svc", new Handler());

        Map<String, Object> response = server.call(request("Svc.explode", null));
        @SuppressWarnings("unchecked")
        Map<String, Object> error = (Map<String, Object>) response.get("error");
        Assert.assertEquals(-32603, ((Number) error.get("code")).intValue());
        Assert.assertEquals("Internal error", error.get("message"));
        Assert.assertFalse("internal errors must not carry data", error.containsKey("data"));

        String wire = new GsonJsonParser().toJson(response);
        Assert.assertFalse(wire, wire.contains("secret-token"));
        Assert.assertFalse(wire, wire.contains("IllegalStateException"));
        Assert.assertFalse(wire, wire.contains("com.example.Handler"));
    }

    @Test
    public void testResponseValidationFailureIsRedacted() {
        // R15: the response-validation path must not leak the validator message.
        Map<String, Object> idl = new HashMap<>();
        idl.put("interfaces", Arrays.asList(Map.of(
            "name", "Svc",
            "methods", Arrays.asList(Map.of(
                "name", "leaky",
                "returnType", Map.of("userDefined", "inc.User")))
        )));
        idl.put("structs", Arrays.asList(Map.of(
            "name", "inc.User",
            "fields", Arrays.asList(Map.of("name", "email", "type", Map.of("builtIn", "string")))
        )));
        Server server = new Server(new Contract(idl), true, true);
        server.addHandler("Svc", new LeakyHandler());

        Map<String, Object> response = server.call(request("Svc.leaky", null));
        @SuppressWarnings("unchecked")
        Map<String, Object> error = (Map<String, Object>) response.get("error");
        Assert.assertEquals(-32603, ((Number) error.get("code")).intValue());
        Assert.assertEquals("Internal error", error.get("message"));
        Assert.assertFalse(error.containsKey("data"));
    }

    @Test
    public void testIdTypeValidation() {
        // Invariant R24/D2: only string and number ids are accepted.
        Server server = new Server(contract(), true, false);
        server.addHandler("Svc", new Handler());

        Object[] invalidIds = { Boolean.TRUE, Map.of("nested", 1), Arrays.asList(1, 2) };
        for (Object badId : invalidIds) {
            Map<String, Object> req = request("Svc.oneArg", Arrays.asList("hi"));
            req.put("id", badId);
            Map<String, Object> response = server.call(req);
            Assert.assertEquals("id type " + badId.getClass().getSimpleName(),
                -32600, errorCode(response));
            // D3: error envelopes always carry id; an unusable id is reported
            // as null rather than echoing the bad value.
            Assert.assertTrue("error responses must include an id member", response.containsKey("id"));
            Assert.assertNull("invalid id responses must not echo the bad id", response.get("id"));
        }

        Map<String, Object> numeric = request("Svc.oneArg", Arrays.asList("hi"));
        numeric.put("id", 42);
        Assert.assertEquals(42, server.call(numeric).get("id"));

        Map<String, Object> stringId = request("Svc.oneArg", Arrays.asList("hi"));
        stringId.put("id", "req-7");
        Assert.assertEquals("req-7", server.call(stringId).get("id"));
    }

    @Test
    public void testNullIdIsNotification() {
        // Invariant M21: id:null is a notification and produces no response.
        Server server = new Server(contract(), true, false);
        server.addHandler("Svc", new Handler());
        Map<String, Object> req = request("Svc.oneArg", Arrays.asList("hi"));
        req.remove("id");
        Assert.assertNull(server.call(req));
        req.put("id", null);
        Assert.assertNull(server.call(req));
    }

    @Test
    public void testErrorEnvelopeShape() {
        // Invariant M22/M25: error.code is numeric, error.message is a string,
        // and error.data is omitted when nil.
        Server server = new Server(contract(), true, false);
        server.addHandler("Svc", new Handler());
        Map<String, Object> response = server.call(request("Svc.explode", null));
        Assert.assertEquals("2.0", response.get("jsonrpc"));
        Assert.assertTrue(response.containsKey("error"));
        @SuppressWarnings("unchecked")
        Map<String, Object> error = (Map<String, Object>) response.get("error");
        Assert.assertTrue(error.get("code") instanceof Number);
        Assert.assertTrue(error.get("message") instanceof String);
        Assert.assertFalse(error.containsKey("data"));
        Assert.assertEquals(1, response.get("id"));
    }

    @Test
    public void testUnknownIdErrorEnvelopeHasNullIdOnTheWire() {
        // D3/JSON-RPC: an error response whose request id is unknown must still
        // serialize an explicit "id": null, matching the generated adapters.
        Server server = new Server(contract(), true, false);
        Map<String, Object> response = server.call(null);
        Assert.assertEquals(-32600, errorCode(response));
        Assert.assertTrue("error envelope must carry an id member", response.containsKey("id"));
        Assert.assertNull(response.get("id"));

        for (JsonParser parser : new JsonParser[] { new GsonJsonParser(), new JacksonJsonParser() }) {
            String wire = parser.toJson(response);
            Map<String, Object> parsed = parser.fromJson(wire, Map.class);
            Assert.assertTrue(parser.getClass().getSimpleName() + " dropped the null id: " + wire,
                parsed.containsKey("id"));
            Assert.assertNull(parsed.get("id"));
        }
    }
}
