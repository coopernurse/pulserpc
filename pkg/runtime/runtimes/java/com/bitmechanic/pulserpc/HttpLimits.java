package com.bitmechanic.pulserpc;

import java.io.IOException;
import java.io.InputStream;
import java.net.http.HttpResponse;
import java.nio.charset.StandardCharsets;
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
     * Raised only when a body violates a size limit. Callers map this to
     * {@code -32600 "Request body too large"} (server) or a size error
     * (client); every other {@link IOException} is a transport failure and must
     * not be reported as an oversized body.
     */
    public static final class BodyTooLargeException extends IOException {
        private static final long serialVersionUID = 1L;

        public BodyTooLargeException(String message) {
            super(message);
        }
    }

    /**
     * Read at most {@code max} bytes from {@code in}. Throws a
     * {@link BodyTooLargeException} when the stream is longer than {@code max}
     * so a caller never buffers an unbounded body. Transport read failures pass
     * through unchanged as a plain {@link IOException}.
     */
    public static byte[] readBounded(InputStream in, int max, String what) throws IOException {
        byte[] data = in.readNBytes(max + 1);
        if (data.length > max) {
            throw new BodyTooLargeException(oversized(what, max));
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
            throw new BodyTooLargeException(oversized(what, max));
        }
    }

    /**
     * Read a bounded HTTP response body, closing the response stream on every
     * path. The stream returned by
     * {@code HttpResponse.BodyHandlers.ofInputStream()} must be closed even
     * when the status or size checks reject the response, otherwise the
     * underlying connection is leaked (invariant R17/D5).
     *
     * @throws IOException on a non-200 status or a transport read failure
     * @throws BodyTooLargeException when the body exceeds {@code max} bytes
     */
    public static String readResponseBody(HttpResponse<InputStream> response, int max, String what)
            throws IOException {
        try (InputStream body = response.body()) {
            int status = response.statusCode();
            if (status != 200) {
                throw new IOException("HTTP error: " + status);
            }
            long declaredLength = response.headers().firstValueAsLong("Content-Length").orElse(-1L);
            if (declaredLength > max) {
                throw new BodyTooLargeException(oversized(what, max));
            }
            return new String(readBounded(body, max, what), StandardCharsets.UTF_8);
        }
    }

    private static String oversized(String what, int max) {
        return what + " exceeds maximum size of " + max + " bytes";
    }
}
