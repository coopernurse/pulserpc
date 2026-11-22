import com.bitmechanic.pulserpc.*;
import com.sun.net.httpserver.HttpServer;
import org.junit.After;
import org.junit.Test;
import org.junit.Assert;

import java.io.OutputStream;
import java.net.InetSocketAddress;
import java.nio.charset.StandardCharsets;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.concurrent.atomic.AtomicReference;

/**
 * Invariants R29/R30/D13: the runtime transport applies custom headers and
 * normalizes a trailing slash on the base URL.
 */
public class HTTPTransportTest {

    private HttpServer server;
    private final AtomicReference<String> seenHeader = new AtomicReference<>();
    private final AtomicReference<String> seenPath = new AtomicReference<>();
    private final AtomicReference<String> seenMethod = new AtomicReference<>();

    private String startServer() throws Exception {
        JsonParser parser = new JacksonJsonParser();
        server = HttpServer.create(new InetSocketAddress("127.0.0.1", 0), 0);
        server.createContext("/rpc", exchange -> {
            seenHeader.set(exchange.getRequestHeaders().getFirst("X-Custom-Token"));
            seenPath.set(exchange.getRequestURI().getPath());
            seenMethod.set(exchange.getRequestMethod());

            @SuppressWarnings("unchecked")
            Map<String, Object> request = parser.fromJson(
                new String(exchange.getRequestBody().readAllBytes(), StandardCharsets.UTF_8), Map.class);
            Map<String, Object> response = new LinkedHashMap<>();
            response.put("jsonrpc", "2.0");
            response.put("result", "pong");
            response.put("id", request.get("id"));
            byte[] body = parser.toJson(response).getBytes(StandardCharsets.UTF_8);
            exchange.getResponseHeaders().set("Content-Type", "application/json");
            exchange.sendResponseHeaders(200, body.length);
            try (OutputStream os = exchange.getResponseBody()) {
                os.write(body);
            }
        });
        server.start();
        return "http://127.0.0.1:" + server.getAddress().getPort() + "/rpc/";
    }

    @After
    public void stopServer() {
        if (server != null) {
            server.stop(0);
        }
    }

    @Test
    public void testCustomHeadersAndTrailingSlash() throws Exception {
        String baseUrl = startServer();
        Map<String, String> headers = new LinkedHashMap<>();
        headers.put("X-Custom-Token", "secret-token");

        HTTPTransport transport = new HTTPTransport(baseUrl, new JacksonJsonParser(), headers);
        Request request = new Request("Svc.ping", List.of(), "req-1");
        Response response = transport.call(request);

        Assert.assertEquals("pong", response.getResult());
        Assert.assertEquals("POST", seenMethod.get());
        Assert.assertEquals("secret-token", seenHeader.get());
        Assert.assertEquals("a trailing slash must not produce a double slash",
            "/rpc", seenPath.get());
    }

    @Test
    public void testTwoArgConstructorStillWorks() throws Exception {
        String baseUrl = startServer();
        HTTPTransport transport = new HTTPTransport(baseUrl, new JacksonJsonParser());
        Response response = transport.call(new Request("Svc.ping", List.of(), 7));
        Assert.assertEquals("pong", response.getResult());
        Assert.assertNull(seenHeader.get());
        Assert.assertEquals("/rpc", seenPath.get());
    }
}
