package com.bitmechanic.pulserpc;

import java.util.Map;

/**
 * Represents a function/method definition from the IDL
 */
@SuppressWarnings("unchecked")
public class FunctionDef extends java.util.HashMap<String, Object> {
    public FunctionDef() {}
    public FunctionDef(Map<String, Object> map) {
        super(map);
    }
}