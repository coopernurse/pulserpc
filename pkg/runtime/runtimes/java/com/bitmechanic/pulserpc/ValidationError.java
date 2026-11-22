package com.bitmechanic.pulserpc;

import java.util.List;
import java.util.ArrayList;
import java.util.Map;
import java.util.LinkedHashMap;

/**
 * A single validation error carrying a field path and a human-readable message.
 * Mirrors the Python/TypeScript ValidationError.
 */
public class ValidationError {
    public final String path;
    public final String message;

    public ValidationError(String path, String message) {
        this.path = path == null ? "" : path;
        this.message = message;
    }
}