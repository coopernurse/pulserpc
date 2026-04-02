package com.bitmechanic.pulserpc;

import java.lang.reflect.Type;

/**
 * Abstraction layer for JSON parsing
 */
public interface JsonParser {

    /**
     * Convert an object to its JSON string representation
     * @param obj The object to serialize
     * @return JSON string
     */
    String toJson(Object obj);

    /**
     * Parse a JSON string into an object of the specified class
     * @param json The JSON string
     * @param clazz The target class
     * @param <T> The type of the target object
     * @return The deserialized object
     */
    <T> T fromJson(String json, Class<T> clazz);

    /**
     * Parse a JSON element (from another parser) into an object of the specified class
     * @param jsonElement The JSON element (implementation-specific)
     * @param clazz The target class
     * @param <T> The type of the target object
     * @return The deserialized object
     */
    <T> T fromJsonElement(Object jsonElement, Class<T> clazz);

    /**
     * Parse a JSON string into an object of the specified type
     * @param json The JSON string
     * @param type The target type
     * @param <T> The type of the target object
     * @return The deserialized object
     */
    <T> T fromJson(String json, Type type);
}
