package com.bitmechanic.pulserpc;

import com.google.gson.Gson;
import com.google.gson.JsonElement;
import java.lang.reflect.Type;

/**
 * GSON-based implementation of JsonParser
 */
public class GsonJsonParser implements JsonParser {

    private final Gson gson;

    /**
     * Create a new GsonJsonParser with default Gson instance
     */
    public GsonJsonParser() {
        this.gson = new Gson();
    }

    /**
     * Create a new GsonJsonParser with custom Gson instance
     * @param gson The Gson instance to use
     */
    public GsonJsonParser(Gson gson) {
        this.gson = gson;
    }

    @Override
    public String toJson(Object obj) {
        return gson.toJson(obj);
    }

    @Override
    public <T> T fromJson(String json, Class<T> clazz) {
        try {
            return gson.fromJson(json, clazz);
        } catch (Exception e) {
            throw new RuntimeException("Failed to deserialize JSON to " + clazz.getSimpleName(), e);
        }
    }

    @Override
    public <T> T fromJsonElement(Object jsonElement, Class<T> clazz) {
        if (!(jsonElement instanceof JsonElement)) {
            throw new IllegalArgumentException("Expected JsonElement, got " + jsonElement.getClass().getSimpleName());
        }
        try {
            return gson.fromJson((JsonElement) jsonElement, clazz);
        } catch (Exception e) {
            throw new RuntimeException("Failed to deserialize JsonElement to " + clazz.getSimpleName(), e);
        }
    }

    @Override
    public <T> T fromJson(String json, Type type) {
        try {
            return gson.fromJson(json, type);
        } catch (Exception e) {
            throw new RuntimeException("Failed to deserialize JSON", e);
        }
    }

    /**
     * Get the underlying Gson instance for advanced usage
     * @return The Gson instance
     */
    public Gson getGson() {
        return gson;
    }
}
