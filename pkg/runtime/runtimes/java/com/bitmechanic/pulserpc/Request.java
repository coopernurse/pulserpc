package com.bitmechanic.pulserpc;

import java.util.List;
import java.util.Map;

/**
 * JSON-RPC 2.0 request message
 */
public class Request {
    private String jsonrpc;
    private String method;
    private Object params;
    private Object id;

    public Request() {
        this.jsonrpc = "2.0";
    }

    public Request(String method, Object params, Object id) {
        this.jsonrpc = "2.0";
        this.method = method;
        this.params = params;
        this.id = id;
    }

    public String getJsonrpc() {
        return jsonrpc;
    }

    public void setJsonrpc(String jsonrpc) {
        this.jsonrpc = jsonrpc;
    }

    public String getMethod() {
        return method;
    }

    public void setMethod(String method) {
        this.method = method;
    }

    public Object getParams() {
        return params;
    }

    public void setParams(Object params) {
        this.params = params;
    }

    public Object getId() {
        return id;
    }

    public void setId(Object id) {
        this.id = id;
    }
}

