import com.bitmechanic.pulserpc.*;
import org.junit.Test;
import org.junit.Assert;

import javax.net.ssl.SSLSession;
import java.io.ByteArrayInputStream;
import java.io.IOException;
import java.io.InputStream;
import java.net.URI;
import java.net.http.HttpClient;
import java.net.http.HttpHeaders;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.nio.charset.StandardCharsets;
import java.util.List;
import java.util.Map;
import java.util.Optional;

/**
 * Invariant R17/D5: reading a response body must close the
 * {@code BodyHandlers.ofInputStream()} stream on every path, including a
 * non-200 status and an oversized body, and must label only size violations as
 * {@link HttpLimits.BodyTooLargeException}.
 */
public class HTTPTransportBodyTest {

    /** Records whether the body stream was closed. */
    static final class TrackingStream extends ByteArrayInputStream {
        boolean closed;

        TrackingStream(String body) {
            super(body.getBytes(StandardCharsets.UTF_8));
        }

        @Override
        public void close() throws IOException {
            closed = true;
            super.close();
        }
    }

    static final class FakeResponse implements HttpResponse<InputStream> {
        private final int status;
        private final InputStream body;
        private final HttpHeaders headers;

        FakeResponse(int status, InputStream body, Map<String, List<String>> headerMap) {
            this.status = status;
            this.body = body;
            this.headers = HttpHeaders.of(headerMap, (name, value) -> true);
        }

        @Override
        public int statusCode() {
            return status;
        }

        @Override
        public HttpRequest request() {
            return null;
        }

        @Override
        public Optional<HttpResponse<InputStream>> previousResponse() {
            return Optional.empty();
        }

        @Override
        public HttpHeaders headers() {
            return headers;
        }

        @Override
        public InputStream body() {
            return body;
        }

        @Override
        public Optional<SSLSession> sslSession() {
            return Optional.empty();
        }

        @Override
        public URI uri() {
            return URI.create("http://127.0.0.1/rpc");
        }

        @Override
        public HttpClient.Version version() {
            return HttpClient.Version.HTTP_1_1;
        }
    }

    @Test
    public void testSuccessReadsBodyAndClosesStream() throws Exception {
        TrackingStream body = new TrackingStream("pong");
        String read = HttpLimits.readResponseBody(new FakeResponse(200, body, Map.of()), 1024, "Response body");
        Assert.assertEquals("pong", read);
        Assert.assertTrue("a successful read must close the response stream", body.closed);
    }

    @Test
    public void testNon200ClosesStreamBeforeThrowing() {
        TrackingStream body = new TrackingStream("server exploded");
        try {
            HttpLimits.readResponseBody(new FakeResponse(500, body, Map.of()), 1024, "Response body");
            Assert.fail("Expected IOException for a non-200 status");
        } catch (IOException e) {
            Assert.assertEquals("HTTP error: 500", e.getMessage());
        }
        Assert.assertTrue("a non-200 response must still close the body stream", body.closed);
    }

    @Test
    public void testOversizedDeclaredLengthClosesStreamBeforeThrowing() {
        TrackingStream body = new TrackingStream("short");
        try {
            HttpLimits.readResponseBody(
                new FakeResponse(200, body, Map.of("Content-Length", List.of("20000000"))),
                1024, "Response body");
            Assert.fail("Expected BodyTooLargeException for an oversized declared length");
        } catch (HttpLimits.BodyTooLargeException e) {
            Assert.assertTrue(e.getMessage().contains("exceeds maximum size"));
        } catch (IOException e) {
            Assert.fail("size violations must use BodyTooLargeException, got " + e);
        }
        Assert.assertTrue("an oversized declared length must still close the body stream", body.closed);
    }

    @Test
    public void testOversizedStreamClosesBeforeThrowing() {
        StringBuilder big = new StringBuilder();
        for (int i = 0; i < 32; i++) {
            big.append("0123456789");
        }
        TrackingStream body = new TrackingStream(big.toString());
        try {
            HttpLimits.readResponseBody(new FakeResponse(200, body, Map.of()), 16, "Response body");
            Assert.fail("Expected BodyTooLargeException for an oversized stream");
        } catch (HttpLimits.BodyTooLargeException e) {
            Assert.assertTrue(e.getMessage().contains("exceeds maximum size"));
        } catch (IOException e) {
            Assert.fail("size violations must use BodyTooLargeException, got " + e);
        }
        Assert.assertTrue("an oversized stream must still close the body", body.closed);
    }

    @Test
    public void testTransportReadErrorIsNotOversize() {
        InputStream failing = new InputStream() {
            @Override
            public int read() throws IOException {
                throw new IOException("connection reset by peer");
            }
        };
        try {
            HttpLimits.readResponseBody(new FakeResponse(200, failing, Map.of()), 1024, "Response body");
            Assert.fail("Expected the transport IOException to propagate");
        } catch (HttpLimits.BodyTooLargeException e) {
            Assert.fail("transport read errors must not be labelled as oversized: " + e);
        } catch (IOException expected) {
            Assert.assertEquals("connection reset by peer", expected.getMessage());
        }
    }
}
