package pulserpc;

import java.lang.reflect.Method;
import java.util.HashMap;
import java.util.List;
import java.util.Map;

/**
 * JSON-RPC 2.0 server with handler registration and optional validation
 */
public class Server {
    private final Contract contract;
    private final Map<String, Object> handlers = new HashMap<>();
    private final boolean validateRequests;
    private final boolean validateResponses;

    /**
     * Creates a new Server instance
     */
    public Server(Contract contract, boolean validateRequests, boolean validateResponses) {
        this.contract = contract;
        this.validateRequests = validateRequests;
        this.validateResponses = validateResponses;
    }

    /**
     * Registers a handler instance for an interface
     */
    public void addHandler(String ifaceName, Object handler) {
        handlers.put(ifaceName, handler);
    }

    /**
     * Processes a single JSON-RPC request and returns a JSON-RPC response.
     * Returns null for notifications (requests without 'id').
     */
    @SuppressWarnings("unchecked")
    public Map<String, Object> call(Map<String, Object> request) {
        if (request == null) {
            return errorResponse(null, -32600, "Invalid Request", "Request must be an object");
        }

        // Check JSON-RPC version
        Object jsonrpc = request.get("jsonrpc");
        if (!"2.0".equals(jsonrpc)) {
            return errorResponse(request.get("id"), -32600, "Invalid Request", "jsonrpc version must be '2.0'");
        }

        // Check for method
        Object methodObj = request.get("method");
        if (!(methodObj instanceof String) || ((String) methodObj).isEmpty()) {
            return errorResponse(request.get("id"), -32600, "Invalid Request", "Method must be a string");
        }
        String method = (String) methodObj;

        // Handle pulserpc-idl request
        if ("pulserpc-idl".equals(method)) {
            return successResponse(request.get("id"), contract.idlParsed);
        }

        // Check for notification (no 'id' means no response expected)
        Object reqId = request.get("id");
        boolean isNotification = reqId == null;

        // Parse method name (e.g., "UserService.getUser")
        String[] parts = parseMethodName(method);
        if (parts == null) {
            return errorResponse(reqId, -32601, "Method not found", "Invalid method name format: " + method);
        }
        String ifaceName = parts[0];
        String funcName = parts[1];

        // Look up handler
        Object handler = handlers.get(ifaceName);
        if (handler == null) {
            return errorResponse(reqId, -32601, "Method not found", "Unknown interface: " + ifaceName);
        }

        // Get function from handler using reflection
        Method methodInfo = findMethod(handler.getClass(), funcName);
        if (methodInfo == null) {
            return errorResponse(reqId, -32601, "Method not found", "Unknown method: " + method);
        }

        // Get params
        Object paramsObj = request.get("params");

        // Convert params to list if it's a list
        List<Object> positionalParams = new java.util.ArrayList<>();
        if (paramsObj instanceof List) {
            positionalParams.addAll((List<Object>) paramsObj);
        } else if (paramsObj == null) {
            positionalParams = new java.util.ArrayList<>();
        }

        // Validate request using positional params
        if (validateRequests && !positionalParams.isEmpty()) {
            try {
                contract.validateRequest(ifaceName, funcName, positionalParams);
            } catch (IllegalArgumentException ex) {
                return errorResponse(reqId, -32602, "Invalid params", ex.getMessage());
            }
        }

        // Convert positional params to named params using IDL signature
        Map<String, Object> namedParams = positionalToNamedParams(ifaceName, funcName, positionalParams);

        // Invoke handler method
        Object result;
        try {
            result = invokeHandler(handler, methodInfo, namedParams);
        } catch (IllegalArgumentException ex) {
            return errorResponse(reqId, -32602, "Invalid params", ex.getMessage());
        } catch (RPCError rpcErr) {
            return errorResponse(reqId, rpcErr.getCode(), rpcErr.getMessage(), rpcErr.getData());
        } catch (Exception ex) {
            if (ex.getCause() instanceof RPCError) {
                RPCError rpcErr = (RPCError) ex.getCause();
                return errorResponse(reqId, rpcErr.getCode(), rpcErr.getMessage(), rpcErr.getData());
            }
            return errorResponse(reqId, -32603, "Internal error", ex.getMessage());
        }

        // Validate response if validation is enabled
        if (validateResponses && result != null) {
            try {
                contract.validateResponse(ifaceName, funcName, result);
            } catch (IllegalArgumentException ex) {
                return errorResponse(reqId, -32603, "Internal error",
                    "Response validation failed: " + ex.getMessage());
            }
        }

        // Don't respond to notifications
        if (isNotification) {
            return null;
        }

        return successResponse(reqId, result);
    }

    /**
     * Creates a JSON-RPC error response
     */
    private Map<String, Object> errorResponse(Object reqId, int code, String message, Object data) {
        Map<String, Object> errObj = new HashMap<>();
        errObj.put("code", code);
        errObj.put("message", message);
        if (data != null) {
            errObj.put("data", data);
        }

        Map<String, Object> response = new HashMap<>();
        response.put("jsonrpc", "2.0");
        response.put("error", errObj);
        if (reqId != null) {
            response.put("id", reqId);
        }
        return response;
    }

    /**
     * Creates a JSON-RPC success response
     */
    private Map<String, Object> successResponse(Object reqId, Object result) {
        Map<String, Object> response = new HashMap<>();
        response.put("jsonrpc", "2.0");
        response.put("result", result);
        if (reqId != null) {
            response.put("id", reqId);
        }
        return response;
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
     * Finds a method by name (case-insensitive, with capital first letter for Java conventions)
     */
    private Method findMethod(Class<?> clazz, String name) {
        // Try exact match first
        for (Method m : clazz.getMethods()) {
            if (m.getName().equals(name)) {
                return m;
            }
        }

        // Try with first letter uppercased (Go/Java convention)
        if (name.length() > 0) {
            String capitalized = Character.toUpperCase(name.charAt(0)) + name.substring(1);
            for (Method m : clazz.getMethods()) {
                if (m.getName().equals(capitalized)) {
                    return m;
                }
            }
        }

        return null;
    }

    /**
     * Converts positional params to named params using IDL signature
     */
    @SuppressWarnings("unchecked")
    private Map<String, Object> positionalToNamedParams(String ifaceName, String funcName,
            List<Object> positionalParams) {
        Map<String, Object> result = new HashMap<>();

        if (contract == null) {
            for (int i = 0; i < positionalParams.size(); i++) {
                result.put("p" + i, positionalParams.get(i));
            }
            return result;
        }

        Interface iface = contract.getInterface(ifaceName);
        if (iface == null) {
            for (int i = 0; i < positionalParams.size(); i++) {
                result.put("p" + i, positionalParams.get(i));
            }
            return result;
        }

        FunctionDef fn = iface.getFunction(funcName);
        if (fn == null) {
            for (int i = 0; i < positionalParams.size(); i++) {
                result.put("p" + i, positionalParams.get(i));
            }
            return result;
        }

        // Get parameter definitions
        List<?> paramDefs = fn.containsKey("parameters") && fn.get("parameters") instanceof List
            ? (List<?>) fn.get("parameters")
            : new java.util.ArrayList<>();

        for (int i = 0; i < positionalParams.size(); i++) {
            if (i < paramDefs.size() && paramDefs.get(i) instanceof Map) {
                @SuppressWarnings("rawtypes")
                Map paramDef = (Map) paramDefs.get(i);
                String paramName = paramDef.containsKey("name") ? (String) paramDef.get("name") : "p" + i;
                result.put(paramName, positionalParams.get(i));
            } else {
                result.put("p" + i, positionalParams.get(i));
            }
        }

        return result;
    }

    /**
     * Invokes a handler method with the given named parameters
     */
    private Object invokeHandler(Object handler, Method methodInfo, Map<String, Object> namedParams) throws Exception {
        Class<?>[] paramTypes = methodInfo.getParameterTypes();
        Object[] args = new Object[paramTypes.length];

        for (int i = 0; i < paramTypes.length; i++) {
            String paramName = methodInfo.getParameters()[i].getName();
            if (paramName == null) paramName = "p" + i;

            // Try to find the parameter by various names
            Object arg = null;

            // Try exact name
            if (namedParams.containsKey(paramName)) {
                arg = namedParams.get(paramName);
            }

            // Try lowercase first letter (Go style)
            if (arg == null && paramName.length() > 0) {
                String lowerName = Character.toLowerCase(paramName.charAt(0)) + paramName.substring(1);
                if (namedParams.containsKey(lowerName)) {
                    arg = namedParams.get(lowerName);
                }
            }

            // Try all keys (for when params come as a map)
            if (arg == null) {
                for (Map.Entry<String, Object> entry : namedParams.entrySet()) {
                    if (entry.getValue() != null && paramTypes[i].isAssignableFrom(entry.getValue().getClass())) {
                        arg = entry.getValue();
                        break;
                    }
                }
            }

            // Use null if not found
            if (arg == null) {
                arg = null;
            }

            args[i] = arg;
        }

        return methodInfo.invoke(handler, args);
    }
}
