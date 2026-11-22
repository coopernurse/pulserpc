package com.bitmechanic.pulserpc;

import java.util.Map;

/**
 * JSON-RPC 2.0 response message
 */
public class Response {
    private String jsonrpc;
    private Object result;
    private boolean resultPresent;
    private Map<String, Object> error;
    private Object id;

    public Response() {
        this.jsonrpc = "2.0";
    }

    public String getJsonrpc() {
        return jsonrpc;
    }

    public void setJsonrpc(String jsonrpc) {
        this.jsonrpc = jsonrpc;
    }

    public Object getResult() {
        return result;
    }

    public void setResult(Object result) {
        this.result = result;
        this.resultPresent = true;
    }

    public boolean hasResult() {
        // Gson sets fields directly and bypasses setResult(), so also treat a
        // non-null result as present. In-process responses always set the flag.
        return resultPresent || result != null;
    }

    public Map<String, Object> getError() {
        return error;
    }

    public void setError(Map<String, Object> error) {
        this.error = error;
    }

    public Object getId() {
        return id;
    }

    public void setId(Object id) {
        this.id = id;
    }

    public boolean hasError() {
        return error != null;
    }

    /**
     * Compare a request id with the id echoed in a response. JSON parsers
     * disagree on the Java type of JSON numbers (Gson yields Double while
     * Jackson yields Integer), so numeric ids must be compared by value
     * (matching the Go runtime's idsEqual policy).
     */
    private static boolean idsMatch(Object expected, Object actual) {
        if (expected == null || actual == null) {
            return expected == null && actual == null;
        }
        if (expected instanceof Number && actual instanceof Number) {
            return ((Number) expected).doubleValue() == ((Number) actual).doubleValue();
        }
        return expected.equals(actual);
    }

    /**
     * Validate the JSON-RPC 2.0 response envelope (java-14): the version must be
     * "2.0", the id must match the request, and exactly one of result/error must
     * be present. Throws RPCError(-32600) on a malformed envelope.
     */
    public void validate(Object expectedId) {
        if (!"2.0".equals(jsonrpc)) {
            throw new RPCError(-32600, "Invalid response: jsonrpc must be '2.0'");
        }
        if (expectedId != null && !idsMatch(expectedId, id)) {
            throw new RPCError(-32600, "Invalid response: id mismatch (expected " + expectedId + ", got " + id + ")");
        }
        if (hasResult() == hasError()) {
            throw new RPCError(-32600, "Invalid response: exactly one of result or error is required");
        }
    }

    /**
     * Map-based variant used by generated clients that parse the raw envelope.
     */
    public static void validateMap(Map<String, Object> envelope, Object expectedId) {
        if (envelope == null || !"2.0".equals(envelope.get("jsonrpc"))) {
            throw new RPCError(-32600, "Invalid response: jsonrpc must be '2.0'");
        }
        Object responseId = envelope.get("id");
        // Mirror typed validate(): when an id is expected, a missing id is a
        // mismatch too. The old responseId != null guard let an envelope with
        // no "id" key slip through validation (java-audit low finding).
        if (expectedId != null && !idsMatch(expectedId, responseId)) {
            throw new RPCError(-32600, "Invalid response: id mismatch (expected " + expectedId + ", got " + responseId + ")");
        }
        boolean hasResult = envelope.containsKey("result");
        boolean hasError = envelope.containsKey("error");
        if (hasResult == hasError) {
            throw new RPCError(-32600, "Invalid response: exactly one of result or error is required");
        }
    }

    /**
     * Build a Response from an envelope that has already been parsed into a
     * plain Map by a JsonParser. This lets callers parse the raw body once:
     * validate that same Map (parser-agnostic) and then construct the Response
     * from it, instead of deserializing the body a second time.
     */
    @SuppressWarnings("unchecked")
    public static Response fromMap(Map<String, Object> envelope) {
        Response response = new Response();
        response.setJsonrpc((String) envelope.get("jsonrpc"));
        response.setId(envelope.get("id"));
        if (envelope.containsKey("result")) {
            // setResult records presence even when the wire value is null.
            response.setResult(envelope.get("result"));
        }
        if (envelope.containsKey("error")) {
            response.setError((Map<String, Object>) envelope.get("error"));
        }
        return response;
    }
}

