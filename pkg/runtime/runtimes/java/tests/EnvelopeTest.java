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
}
