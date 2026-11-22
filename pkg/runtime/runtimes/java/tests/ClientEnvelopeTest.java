import com.bitmechanic.pulserpc.*;
import org.junit.Test;
import org.junit.Assert;

import java.util.ArrayList;
import java.util.HashMap;
import java.util.List;
import java.util.Map;
import java.util.function.Function;

/**
 * Canonical client-envelope conformance cases (M5/M14/M24/M27/D2) driven
 * through the runtime Client, matching the Go integration conformance test.
 */
public class ClientEnvelopeTest {

    /** IDL with a required-return and an optional-return method. */
    private static Map<String, Object> idl() {
        Map<String, Object> required = new HashMap<>();
        required.put("name", "req");
        required.put("parameters", new ArrayList<>());
        required.put("returnType", Map.of("builtIn", "string"));

        Map<String, Object> optional = new HashMap<>();
        optional.put("name", "opt");
        optional.put("parameters", new ArrayList<>());
        optional.put("returnType", Map.of("builtIn", "string"));
        optional.put("returnOptional", true);

        Map<String, Object> iface = new HashMap<>();
        iface.put("name", "Svc");
        iface.put("methods", List.of(required, optional));

        Map<String, Object> idl = new HashMap<>();
        idl.put("interfaces", List.of(iface));
        idl.put("structs", new ArrayList<>());
        idl.put("enums", new ArrayList<>());
        return idl;
    }

    /** Serves the IDL on bootstrap, then delegates to a per-case responder. */
    private static final class ScriptedTransport implements Transport {
        private final Map<String, Object> idl;
        private final Function<Request, Response> responder;

        ScriptedTransport(Map<String, Object> idl, Function<Request, Response> responder) {
            this.idl = idl;
            this.responder = responder;
        }

        @Override
        public Response call(Request request) throws Exception {
            if ("pulserpc-idl".equals(request.getMethod())) {
                return resultResponse(request.getId(), idl);
            }
            return responder.apply(request);
        }
    }

    private static Response resultResponse(Object id, Object result) {
        Response response = new Response();
        response.setJsonrpc("2.0");
        response.setId(id);
        response.setResult(result);
        return response;
    }

    private static void expectEnvelopeError(Client client) {
        try {
            client.call("Svc.req", new ArrayList<>());
            Assert.fail("expected an RPCError for a malformed envelope");
        } catch (RPCError e) {
            Assert.assertEquals(-32600, e.getCode());
        }
    }

    @Test
    public void testWrongJsonRpcVersionRejected() {
        ScriptedTransport transport = new ScriptedTransport(idl(), req -> {
            Response r = resultResponse(req.getId(), "ok");
            r.setJsonrpc("1.0");
            return r;
        });
        expectEnvelopeError(new Client(transport, false, false));
    }

    @Test
    public void testIdMismatchRejected() {
        ScriptedTransport transport = new ScriptedTransport(idl(), req ->
            resultResponse(((Number) req.getId()).intValue() + 100, "ok"));
        expectEnvelopeError(new Client(transport, false, false));
    }

    @Test
    public void testBothResultAndErrorRejected() {
        ScriptedTransport transport = new ScriptedTransport(idl(), req -> {
            Response r = resultResponse(req.getId(), "ok");
            Map<String, Object> error = new HashMap<>();
            error.put("code", -32603);
            error.put("message", "boom");
            r.setError(error);
            return r;
        });
        expectEnvelopeError(new Client(transport, false, false));
    }

    @Test
    public void testNeitherResultNorErrorRejected() {
        ScriptedTransport transport = new ScriptedTransport(idl(), req -> {
            Response r = new Response();
            r.setJsonrpc("2.0");
            r.setId(req.getId());
            return r;
        });
        expectEnvelopeError(new Client(transport, false, false));
    }

    @Test
    public void testMalformedErrorCodeAndMessageRejected() {
        // M27: a non-numeric code and a non-string message must fall back to a
        // safe internal-error response rather than being trusted verbatim.
        ScriptedTransport transport = new ScriptedTransport(idl(), req -> {
            Response r = new Response();
            r.setJsonrpc("2.0");
            r.setId(req.getId());
            Map<String, Object> error = new HashMap<>();
            error.put("code", "not-a-number");
            error.put("message", 42);
            r.setError(error);
            return r;
        });
        Client client = new Client(transport, false, false);
        try {
            client.call("Svc.req", new ArrayList<>());
            Assert.fail("expected an RPCError for a malformed error object");
        } catch (RPCError e) {
            Assert.assertEquals(-32603, e.getCode());
            Assert.assertEquals("42", e.getMessage());
        }
    }

    @Test
    public void testNumericIdComparedByValue() {
        // M24: an int request id and a numerically equal double echoed id match.
        ScriptedTransport transport = new ScriptedTransport(idl(), req ->
            resultResponse(((Number) req.getId()).doubleValue() + 0.0d, "ok"));
        Client client = new Client(transport, false, false);
        Assert.assertEquals("ok", client.call("Svc.req", new ArrayList<>()));
    }

    @Test
    public void testNullResultRequiredRejectedAndOptionalAccepted() {
        Client required = new Client(
            new ScriptedTransport(idl(), req -> resultResponse(req.getId(), null)), false, true);
        try {
            required.call("Svc.req", new ArrayList<>());
            Assert.fail("expected a null result for a required return to be rejected");
        } catch (RuntimeException e) {
            Assert.assertTrue(e.getMessage().contains("invalid response"));
        }

        Client optional = new Client(
            new ScriptedTransport(idl(), req -> resultResponse(req.getId(), null)), false, true);
        Assert.assertNull(optional.call("Svc.opt", new ArrayList<>()));
    }
}
