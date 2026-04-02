package com.bitmechanic.pulserpc;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.JsonNode;
import java.lang.reflect.Type;
import java.io.IOException;

/**
 * Jackson-based implementation of JsonParser
 */
public class JacksonJsonParser implements JsonParser {

    private final ObjectMapper objectMapper;

    /**
     * Create a new JacksonJsonParser with default ObjectMapper
     */
    public JacksonJsonParser() {
        this.objectMapper = new ObjectMapper();
    }

    /**
     * Create a new JacksonJsonParser with custom ObjectMapper
     * @param objectMapper The ObjectMapper to use
     */
    public JacksonJsonParser(ObjectMapper objectMapper) {
        this.objectMapper = objectMapper;
    }

    @Override
    public String toJson(Object obj) {
        try {
            return objectMapper.writeValueAsString(obj);
        } catch (Exception e) {
            throw new RuntimeException("Failed to serialize object to JSON", e);
        }
    }

    @Override
    public <T> T fromJson(String json, Class<T> clazz) {
        try {
            return objectMapper.readValue(json, clazz);
        } catch (Exception e) {
            throw new RuntimeException("Failed to deserialize JSON to " + clazz.getSimpleName(), e);
        }
    }

    @Override
    public <T> T fromJsonElement(Object jsonElement, Class<T> clazz) {
        if (!(jsonElement instanceof JsonNode)) {
            throw new IllegalArgumentException("Expected JsonNode, got " + jsonElement.getClass().getSimpleName());
        }
        try {
            return objectMapper.treeToValue((JsonNode) jsonElement, clazz);
        } catch (Exception e) {
            throw new RuntimeException("Failed to deserialize JsonNode to " + clazz.getSimpleName(), e);
        }
    }

    @Override
    public <T> T fromJson(String json, Type type) {
        try {
            return objectMapper.readValue(json, objectMapper.getTypeFactory().constructType(type));
        } catch (Exception e) {
            throw new RuntimeException("Failed to deserialize JSON", e);
        }
    }

    /**
     * Get the underlying ObjectMapper for advanced usage
     * @return The ObjectMapper instance
     */
    public ObjectMapper getObjectMapper() {
        return objectMapper;
    }
}
