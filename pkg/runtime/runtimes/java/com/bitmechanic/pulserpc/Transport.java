package com.bitmechanic.pulserpc;

/**
 * Transport abstraction for making RPC calls
 */
public interface Transport {
    /**
     * Make an RPC call
     * @param request The JSON-RPC request
     * @return The JSON-RPC response
     * @throws Exception if the call fails
     */
    Response call(Request request) throws Exception;
}

