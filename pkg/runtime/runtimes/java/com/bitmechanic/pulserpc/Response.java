package com.bitmechanic.pulserpc;

import java.util.Map;

/**
 * JSON-RPC 2.0 response message
 */
public class Response {
    private String jsonrpc;
    private Object result;
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
}

