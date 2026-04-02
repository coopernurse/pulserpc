package com.bitmechanic.pulserpc;

/**
 * Exception class for JSON-RPC 2.0 errors
 */
public class RPCError extends RuntimeException {

    private final int code;
    private final String message;
    private final Object data;

    /**
     * Creates a new RPCError instance
     * @param code JSON-RPC error code
     * @param message Error message
     * @param data Optional error data
     */
    public RPCError(int code, String message, Object data) {
        super("RPCError " + code + ": " + message);
        this.code = code;
        this.message = message;
        this.data = data;
    }

    /**
     * Creates a new RPCError instance without data
     * @param code JSON-RPC error code
     * @param message Error message
     */
    public RPCError(int code, String message) {
        this(code, message, null);
    }

    /**
     * JSON-RPC error code
     */
    public int getCode() {
        return code;
    }

    /**
     * Error message
     */
    @Override
    public String getMessage() {
        return message;
    }

    /**
     * Optional error data
     */
    public Object getData() {
        return data;
    }
}
