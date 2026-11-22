import com.bitmechanic.pulserpc.*;
import org.junit.Test;
import org.junit.Assert;

import java.io.ByteArrayInputStream;
import java.io.IOException;
import java.io.InputStream;
import java.time.Duration;

/**
 * Invariants R16-R19/D5/D6: canonical resource limits enforced by generated
 * adapters and the runtime transport.
 */
public class HttpLimitsTest {

    @Test
    public void testCanonicalLimitConstants() {
        Assert.assertEquals(1024 * 1024, HttpLimits.MAX_REQUEST_BODY);
        Assert.assertEquals(10 * 1024 * 1024, HttpLimits.MAX_RESPONSE_BODY);
        Assert.assertEquals(100, HttpLimits.MAX_BATCH_SIZE);
        Assert.assertEquals(Duration.ofSeconds(30), HttpLimits.REQUEST_TIMEOUT);
        Assert.assertEquals(Duration.ofSeconds(10), HttpLimits.CONNECT_TIMEOUT);
    }

    @Test
    public void testReadBoundedAcceptsLimit() throws IOException {
        byte[] body = new byte[1024];
        byte[] read = HttpLimits.readBounded(new ByteArrayInputStream(body), body.length, "Request body");
        Assert.assertEquals(body.length, read.length);
    }

    @Test
    public void testReadBoundedRejectsOversizeStream() {
        // One byte over the declared maximum must fail even when the stream can
        // produce more data (a missing or lying Content-Length).
        byte[] body = new byte[1025];
        try {
            HttpLimits.readBounded(new ByteArrayInputStream(body), 1024, "Request body");
            Assert.fail("Expected BodyTooLargeException for an oversized stream");
        } catch (HttpLimits.BodyTooLargeException e) {
            Assert.assertTrue(e.getMessage().contains("exceeds maximum size"));
        } catch (IOException e) {
            Assert.fail("size violations must use BodyTooLargeException, got " + e);
        }
    }

    @Test
    public void testCheckDeclaredLength() throws IOException {
        HttpLimits.checkDeclaredLength(null, 1024, "Request body");
        HttpLimits.checkDeclaredLength("garbage", 1024, "Request body");
        HttpLimits.checkDeclaredLength("1024", 1024, "Request body");

        try {
            HttpLimits.checkDeclaredLength("1025", 1024, "Request body");
            Assert.fail("Expected BodyTooLargeException for an oversized Content-Length");
        } catch (HttpLimits.BodyTooLargeException e) {
            Assert.assertTrue(e.getMessage().contains("exceeds maximum size"));
        } catch (IOException e) {
            Assert.fail("size violations must use BodyTooLargeException, got " + e);
        }
    }

    @Test
    public void testTransportReadErrorIsNotMislabelledAsOversize() {
        // A client disconnect or corrupt chunked encoding is a transport error,
        // not a size violation: generated servers must answer -32603 for it.
        InputStream failing = new InputStream() {
            @Override
            public int read() throws IOException {
                throw new IOException("connection reset by peer");
            }
        };
        try {
            HttpLimits.readBounded(failing, 1024, "Request body");
            Assert.fail("Expected the transport IOException to propagate");
        } catch (HttpLimits.BodyTooLargeException e) {
            Assert.fail("transport read errors must not be labelled as oversized: " + e);
        } catch (IOException expected) {
            Assert.assertEquals("connection reset by peer", expected.getMessage());
        }
    }
}
