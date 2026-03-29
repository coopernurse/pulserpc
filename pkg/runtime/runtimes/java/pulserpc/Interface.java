package com.bitmechanic.pulserpc;

import java.util.Map;

/**
 * Represents an interface from the IDL
 */
public class Interface {
    public String name;
    public Map<String, FunctionDef> functions = new java.util.HashMap<>();

    public Interface() {}

    public FunctionDef getFunction(String funcName) {
        return functions.get(funcName);
    }
}