import com.bitmechanic.pulserpc.*;
import org.junit.Test;
import org.junit.Assert;
import java.util.*;

public class EnvelopeTest {

    @Test
    public void testValidEnvelope() {
        Map<String, Object> response = new HashMap<>();
        response.put("jsonrpc", "2.0");
        response.put("result", "ok");
        response.put("id", 1);
        Response.validateMap(response, 1);

        Response typed = new Response();
        typed.setJsonrpc("2.0");
        typed.setId(1);
        typed.setResult("ok");
        typed.validate(1);
    }

    @Test
    public void testIdMismatchRejected() {
        Map<String, Object> response = new HashMap<>();
        response.put("jsonrpc", "2.0");
        response.put("result", "ok");
        response.put("id", 2);
        try {
            Response.validateMap(response, 1);
            Assert.fail("Expected RPCError for id mismatch");
        } catch (RPCError e) {
            Assert.assertEquals(-32600, e.getCode());
        }

        Response typed = new Response();
        typed.setJsonrpc("2.0");
        typed.setId(2);
        typed.setResult("ok");
        try {
            typed.validate(1);
            Assert.fail("Expected RPCError for id mismatch");
        } catch (RPCError e) {
            Assert.assertEquals(-32600, e.getCode());
        }
    }

    @Test
    public void testBothResultAndErrorRejected() {
        Map<String, Object> response = new HashMap<>();
        response.put("jsonrpc", "2.0");
        response.put("result", "ok");
        response.put("error", Map.of("code", -32603, "message", "boom"));
        response.put("id", 1);
        try {
            Response.validateMap(response, 1);
            Assert.fail("Expected RPCError when both result and error present");
        } catch (RPCError e) {
            Assert.assertEquals(-32600, e.getCode());
        }
    }

    @Test
    public void testNeitherResultNorErrorRejected() {
        Map<String, Object> response = new HashMap<>();
        response.put("jsonrpc", "2.0");
        response.put("id", 1);
        try {
            Response.validateMap(response, 1);
            Assert.fail("Expected RPCError when neither result nor error present");
        } catch (RPCError e) {
            Assert.assertEquals(-32600, e.getCode());
        }
    }

    @Test
    public void testWrongVersionRejected() {
        Map<String, Object> response = new HashMap<>();
        response.put("jsonrpc", "1.0");
        response.put("result", "ok");
        response.put("id", 1);
        try {
            Response.validateMap(response, 1);
            Assert.fail("Expected RPCError for wrong jsonrpc version");
        } catch (RPCError e) {
            Assert.assertEquals(-32600, e.getCode());
        }
    }

    @Test
    public void testNullResultSuccessEnvelopeAccepted() {
        Map<String, Object> response = new HashMap<>();
        response.put("jsonrpc", "2.0");
        response.put("result", null);
        response.put("id", 1);
        Response.validateMap(response, 1);
    }

    @Test
    public void testMissingIdRejectedWhenIdExpected() {
        // A response with no "id" key must be rejected when the caller expected
        // one, matching the typed validate() behaviour at Response.java:71.
        Map<String, Object> response = new HashMap<>();
        response.put("jsonrpc", "2.0");
        response.put("result", "ok");
        // deliberately no "id" key
        try {
            Response.validateMap(response, 1);
            Assert.fail("Expected RPCError when expected id is absent from response");
        } catch (RPCError e) {
            Assert.assertEquals(-32600, e.getCode());
        }

        // No expected id means an absent response id is still valid.
        Response.validateMap(response, null);
    }

    @Test
    public void testEnvelopeValidationWithGsonAndJackson() {
        // HTTPTransport now parses the body once into a Map, validates that
        // Map, and builds the Response from the same parsed data. Both parsers
        // must produce Maps that validate and convert identically.
        // Use a string id: Gson and Jackson disagree on the Java type of JSON
        // numbers (Double vs Integer), which is orthogonal to envelope shape.
        JsonParser[] parsers = { new GsonJsonParser(), new JacksonJsonParser() };
        String[] bodies = {
            "{\"jsonrpc\":\"2.0\",\"result\":{\"ok\":true},\"id\":\"req-7\"}",
            "{\"jsonrpc\":\"2.0\",\"error\":{\"code\":-32602,\"message\":\"bad params\"},\"id\":\"req-7\"}"
        };

        for (JsonParser parser : parsers) {
            for (String body : bodies) {
                Map<String, Object> envelope = parser.fromJson(body, Map.class);
                Response.validateMap(envelope, "req-7");
                Response response = Response.fromMap(envelope);
                response.validate("req-7");
                Assert.assertEquals("2.0", response.getJsonrpc());
                Assert.assertEquals("req-7", response.getId());
                if (envelope.containsKey("error")) {
                    Assert.assertTrue(response.hasError());
                    Assert.assertEquals(-32602,
                        ((Number) response.getError().get("code")).intValue());
                    Assert.assertEquals("bad params", response.getError().get("message"));
                } else {
                    Assert.assertFalse(response.hasError());
                    Assert.assertTrue(response.hasResult());
                }
            }
        }
    }
}
