package com.bitmechanic.pulserpc;

import java.util.HashMap;
import java.util.List;
import java.util.Map;
import java.util.function.Function;

/**
 * InterfaceClientProxy represents a dynamic proxy for an interface
 */
public class InterfaceClientProxy {
    private final Client client;
    private final Interface iface;
    private final String ifaceName;
    private final Map<String, Function<Object[], Object>> methods = new HashMap<>();

    public InterfaceClientProxy(Client client, Interface iface) {
        this.client = client;
        this.iface = iface;
        this.ifaceName = iface.name;

        // Create method callables for each function
        for (String funcName : iface.functions.keySet()) {
            String capturedFuncName = funcName;
            methods.put(funcName, (params) -> client.call(ifaceName + "." + capturedFuncName, params));
        }
    }

    /**
     * Calls a method on the interface
     */
    public Object call(String funcName, Object... args) {
        Function<Object[], Object> method = methods.get(funcName);
        if (method == null) {
            throw new IllegalArgumentException("Method not found: " + funcName);
        }
        return method.apply(args);
    }

    /**
     * Gets a method by name
     */
    public Function<Object[], Object> getMethod(String funcName) {
        return methods.get(funcName);
    }

    /**
     * Lists all available method names
     */
    public List<String> listMethods() {
        return new java.util.ArrayList<>(methods.keySet());
    }
}