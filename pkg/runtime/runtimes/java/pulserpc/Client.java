package com.bitmechanic.pulserpc;

import java.util.HashMap;
import java.util.List;
import java.util.Map;
import java.util.function.Function;

/**
 * JSON-RPC 2.0 client with automatic interface discovery
 */
public class Client {
    private final Transport transport;
    private final boolean validateRequest;
    private final boolean validateResponse;
    private int requestID = 0;
    private final Map<String, InterfaceClientProxy> ifaces = new HashMap<>();

    public Contract contract;

    /**
     * Creates a new Client and bootstraps by fetching IDL from server
     */
    public Client(Transport transport, boolean validateRequest, boolean validateResponse) {
        this.transport = transport;
        this.validateRequest = validateRequest;
        this.validateResponse = validateResponse;

        // Bootstrap: fetch IDL from server
        bootstrap();
    }

    /**
     * Gets an interface proxy by name
     */
    public InterfaceClientProxy getInterface(String ifaceName) {
        return ifaces.get(ifaceName);
    }

    /**
     * Makes a JSON-RPC call
     */
    public Object call(String method, Object params) {
        // Validate request if enabled
        if (validateRequest && contract != null) {
            String[] parts = parseMethodName(method);
            if (parts != null) {
                List<Object> paramList = convertParamsToList(parts[0], parts[1], params);
                if (paramList != null) {
                    try {
                        contract.validateRequest(parts[0], parts[1], paramList);
                    } catch (IllegalArgumentException ex) {
                        throw new RuntimeException("Request validation failed: " + ex.getMessage(), ex);
                    }
                }
            }
        }

        // Generate request ID
        requestID++;
        Object reqId = requestID;

        // Build request
        Request request = new Request();
        request.setJsonrpc("2.0");
        request.setMethod(method);
        request.setId(reqId);
        request.setParams(params);

        // Send request via transport
        Response response;
        try {
            response = transport.call(request);
        } catch (Exception e) {
            throw new RuntimeException("Transport request failed: " + e.getMessage(), e);
        }

        // Check for error response
        if (response.hasError()) {
            Map<String, Object> errObj = response.getError();
            int code = errObj.containsKey("code") ? ((Number) errObj.get("code")).intValue() : -32603;
            String message = errObj.containsKey("message") ? (String) errObj.get("message") : "Unknown error";
            Object data = errObj.get("data");
            throw new RPCError(code, message, data);
        }

        // Get result
        Object result = response.getResult();

        // Validate response if enabled
        if (validateResponse && contract != null && result != null) {
            String[] parts = parseMethodName(method);
            if (parts != null) {
                try {
                    contract.validateResponse(parts[0], parts[1], result);
                } catch (IllegalArgumentException ex) {
                    throw new RuntimeException("Response validation failed: " + ex.getMessage(), ex);
                }
            }
        }

        return result;
    }

    /**
     * Sends a notification (no response expected)
     */
    public void notify(String method, Object params) {
        try {
            call(method, params);
        } catch (Exception e) {
            // Ignore errors for notifications
        }
    }

    /**
     * Bootstrap: fetch IDL from server and create interface proxies
     */
    private void bootstrap() {
        // Make request to get IDL
        Request req = new Request("pulserpc-idl", null, "bootstrap");

        Response resp;
        try {
            resp = transport.call(req);
        } catch (Exception e) {
            throw new RuntimeException("Failed to fetch IDL from server: " + e.getMessage(), e);
        }

        if (resp.hasError()) {
            throw new RuntimeException("Failed to fetch IDL from server: " + resp.getError().get("message"));
        }

        Object idlJson = resp.getResult();
        if (idlJson == null) {
            throw new RuntimeException("Server returned empty IDL");
        }

        // Create contract
        contract = new Contract(idlJson);

        // Create interface proxies
        for (Map.Entry<String, Interface> entry : contract.interfaces.entrySet()) {
            InterfaceClientProxy proxy = new InterfaceClientProxy(this, entry.getValue());
            ifaces.put(entry.getKey(), proxy);
        }
    }

    /**
     * Parses "Interface.method" format
     */
    private String[] parseMethodName(String method) {
        int lastDot = method.lastIndexOf('.');
        if (lastDot < 0) {
            return null;
        }
        return new String[]{method.substring(0, lastDot), method.substring(lastDot + 1)};
    }

    /**
     * Converts params to positional list for validation
     */
    @SuppressWarnings("unchecked")
    private List<Object> convertParamsToList(String ifaceName, String funcName, Object params) {
        if (contract == null) return null;

        Interface iface = contract.getInterface(ifaceName);
        if (iface == null) return null;

        FunctionDef fn = iface.getFunction(funcName);
        if (fn == null) return null;

        List<Object> paramDefs = fn.containsKey("parameters") && fn.get("parameters") instanceof List
            ? (List<Object>) fn.get("parameters")
            : new java.util.ArrayList<>();

        List<Object> result = new java.util.ArrayList<>();

        if (params instanceof List) {
            result.addAll((List<Object>) params);
        } else if (params instanceof Map) {
            Map<String, Object> namedParams = (Map<String, Object>) params;
            for (Object paramDef : paramDefs) {
                if (paramDef instanceof Map) {
                    Map<String, Object> pd = (Map<String, Object>) paramDef;
                    String name = pd.containsKey("name") ? (String) pd.get("name") : "";
                    result.add(namedParams.get(name));
                } else {
                    result.add(null);
                }
            }
        } else if (params != null) {
            result.add(params);
        }

        return result;
    }
}