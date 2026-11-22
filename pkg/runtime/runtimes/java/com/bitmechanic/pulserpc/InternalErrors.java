package com.bitmechanic.pulserpc;

/**
 * Shared server-side logging for internal errors (invariant R15/D4).
 *
 * <p>Details are written to stderr only; the wire response never carries the
 * exception class, message, stack trace or any argument data. Both the runtime
 * {@link Server} and the generated Java server use this single implementation
 * so redaction cannot drift between them.</p>
 */
public final class InternalErrors {

    private InternalErrors() {
    }

    /**
     * Logs {@code error} against {@code where} on stderr.
     *
     * @param where a short description of the failing operation, e.g.
     *              {@code "method Svc.ping"}
     * @param error the throwable to log; {@code null} is tolerated
     */
    public static void log(String where, Throwable error) {
        System.err.println("Internal error in " + where + ": "
            + (error == null ? "unknown error"
                : error.getClass().getName() + ": " + error.getMessage()));
        if (error != null) {
            error.printStackTrace(System.err);
        }
    }
}
