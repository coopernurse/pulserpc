package com.bitmechanic.pulserpc;

import com.google.gson.Gson;
import com.google.gson.GsonBuilder;
import com.google.gson.JsonArray;
import com.google.gson.JsonElement;
import com.google.gson.JsonObject;
import com.google.gson.JsonPrimitive;
import com.google.gson.stream.JsonReader;
import com.google.gson.stream.JsonToken;
import java.io.IOException;
import java.io.StringReader;
import java.lang.reflect.Type;
import java.math.BigInteger;
import java.util.ArrayList;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;

/**
 * GSON-based implementation of JsonParser.
 *
 * <p>Gson's default {@code Object} adapter turns every JSON number into a
 * {@code Double}, which loses precision for int64 values. The raw
 * {@code Object}/{@code Map}/{@code List} paths therefore use a small tree
 * converter that keeps integral numbers as {@code Long} (or {@code BigInteger}
 * when they do not fit) and only falls back to {@code Double} for
 * decimal/exponent forms.</p>
 */
public class GsonJsonParser implements JsonParser {

    private final Gson gson;

    /**
     * Create a new GsonJsonParser with default Gson instance.
     *
     * Nulls are serialized so JSON-RPC envelopes keep an explicit
     * `"result": null` (and struct optional fields keep their keys). This
     * matches Jackson and the other runtimes; Gson's default drops null map
     * values, which produces an invalid response envelope for a null result.
     */
    public GsonJsonParser() {
        this.gson = new GsonBuilder().serializeNulls().create();
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
            if (wantsObjectTree(clazz)) {
                return clazz.cast(parseObjectTree(json));
            }
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
            if (wantsObjectTree(clazz)) {
                return clazz.cast(toJavaValue((JsonElement) jsonElement));
            }
            return gson.fromJson((JsonElement) jsonElement, clazz);
        } catch (Exception e) {
            throw new RuntimeException("Failed to deserialize JsonElement to " + clazz.getSimpleName(), e);
        }
    }

    @Override
    @SuppressWarnings("unchecked")
    public <T> T fromJson(String json, Type type) {
        try {
            if (wantsObjectTree(type)) {
                return (T) parseObjectTree(json);
            }
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

    private static boolean wantsObjectTree(Type type) {
        return type == Object.class || type == Map.class || type == List.class;
    }

    /**
     * Parse a raw Object/Map/List target in one pass, keeping integral numbers
     * as Long (or BigInteger when they do not fit) instead of Gson's default
     * Double. Uses the configured Gson instance's reader factory.
     */
    private Object parseObjectTree(String json) throws IOException {
        JsonReader reader = gson.newJsonReader(new StringReader(json));
        // Gson.fromJson sets its reader lenient for the duration of a parse;
        // match that behavior for the raw-tree path.
        reader.setLenient(true);
        Object value = readValue(reader);
        if (reader.peek() != JsonToken.END_DOCUMENT) {
            throw new IOException("trailing data after JSON value");
        }
        return value;
    }

    private static Object readValue(JsonReader reader) throws IOException {
        JsonToken token = reader.peek();
        switch (token) {
            case BEGIN_OBJECT:
                Map<String, Object> map = new LinkedHashMap<>();
                reader.beginObject();
                while (reader.hasNext()) {
                    String name = reader.nextName();
                    map.put(name, readValue(reader));
                }
                reader.endObject();
                return map;
            case BEGIN_ARRAY:
                List<Object> list = new ArrayList<>();
                reader.beginArray();
                while (reader.hasNext()) {
                    list.add(readValue(reader));
                }
                reader.endArray();
                return list;
            case STRING:
                return reader.nextString();
            case NUMBER:
                String text = reader.nextString();
                if (text.indexOf('.') < 0 && text.indexOf('e') < 0 && text.indexOf('E') < 0) {
                    try {
                        return Long.parseLong(text);
                    } catch (NumberFormatException e) {
                        // Integer-form text outside long: keep it exact so
                        // validation rejects it instead of a rounded double.
                        return new BigInteger(text);
                    }
                }
                return Double.parseDouble(text);
            case BOOLEAN:
                return reader.nextBoolean();
            case NULL:
                reader.nextNull();
                return null;
            default:
                throw new IOException("Unexpected JSON token: " + token);
        }
    }

    private static Object toJavaValue(JsonElement element) {
        if (element == null || element.isJsonNull()) {
            return null;
        }
        if (element.isJsonObject()) {
            JsonObject object = element.getAsJsonObject();
            Map<String, Object> map = new LinkedHashMap<>();
            for (Map.Entry<String, JsonElement> entry : object.entrySet()) {
                map.put(entry.getKey(), toJavaValue(entry.getValue()));
            }
            return map;
        }
        if (element.isJsonArray()) {
            JsonArray array = element.getAsJsonArray();
            List<Object> list = new ArrayList<>(array.size());
            for (JsonElement item : array) {
                list.add(toJavaValue(item));
            }
            return list;
        }

        JsonPrimitive primitive = element.getAsJsonPrimitive();
        if (primitive.isBoolean()) {
            return primitive.getAsBoolean();
        }
        if (primitive.isString()) {
            return primitive.getAsString();
        }
        if (primitive.isNumber()) {
            String text = primitive.getAsString();
            if (text.indexOf('.') < 0 && text.indexOf('e') < 0 && text.indexOf('E') < 0) {
                try {
                    return Long.parseLong(text);
                } catch (NumberFormatException e) {
                    // Integer-form text outside long: keep it exact so
                    // validation rejects it instead of a rounded double.
                    return new BigInteger(text);
                }
            }
            return primitive.getAsDouble();
        }
        return null;
    }
}
