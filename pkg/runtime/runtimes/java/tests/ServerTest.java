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
                       "returnType", Map.of("builtIn", "string"))
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
}
