package com.bitmechanic.pulserpc;

import java.io.IOException;
import java.io.InputStream;
import java.time.Duration;

/**
 * Resource limits shared by generated Java adapters and {@link HTTPTransport}
 * (invariants R16-R19 / D5-D6).
 *
 * <p>The values are canonical across every PulseRPC runtime:</p>
 * <ul>
 *   <li>request body: 1 MiB</li>
 *   <li>response body: 10 MiB</li>
 *   <li>batch entries: 100</li>
 *   <li>request timeout: 30 seconds (connect: 10 seconds)</li>
 * </ul>
 */
public final class HttpLimits {

    /** Maximum JSON-RPC request body accepted by a generated server. */
    public static final int MAX_REQUEST_BODY = 1024 * 1024;

    /** Maximum JSON-RPC response body accepted by a generated client. */
    public static final int MAX_RESPONSE_BODY = 10 * 1024 * 1024;

    /** Maximum number of entries accepted in a JSON-RPC batch. */
    public static final int MAX_BATCH_SIZE = 100;

    /** Request (read) timeout for generated HTTP clients. */
    public static final Duration REQUEST_TIMEOUT = Duration.ofSeconds(30);

    /** Connect timeout for generated HTTP clients and the runtime transport. */
    public static final Duration CONNECT_TIMEOUT = Duration.ofSeconds(10);

    private HttpLimits() {
    }

    /**
     * Read at most {@code max} bytes from {@code in}. Throws when the stream is
     * longer than {@code max} so a caller never buffers an unbounded body.
     */
    public static byte[] readBounded(InputStream in, int max, String what) throws IOException {
        byte[] data = in.readNBytes(max + 1);
        if (data.length > max) {
            throw new IOException(what + " exceeds maximum size of " + max + " bytes");
        }
        return data;
    }

    /**
     * Reject a declared {@code Content-Length} before reading the body. A
     * missing or malformed header is left to {@link #readBounded}.
     */
    public static void checkDeclaredLength(String contentLength, int max, String what) throws IOException {
        if (contentLength == null || contentLength.isEmpty()) {
            return;
        }
        long declared;
        try {
            declared = Long.parseLong(contentLength.trim());
        } catch (NumberFormatException e) {
            return;
        }
        if (declared > max) {
            throw new IOException(what + " exceeds maximum size of " + max + " bytes");
        }
    }
}
