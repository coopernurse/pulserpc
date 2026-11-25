import com.bitmechanic.pulserpc.*;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.google.gson.Gson;
import org.junit.Test;
import org.junit.Assert;
import java.math.BigInteger;
import java.util.*;

public class JsonParserTest {

    @Test
    public void testJacksonJsonParser() {
        JsonParser parser = new JacksonJsonParser();

        // Test serialization
        Map<String, Object> data = Map.of("name", "John", "age", 30);
        String json = parser.toJson(data);
        Assert.assertTrue(json.contains("\"name\":\"John\""));
        Assert.assertTrue(json.contains("\"age\":30"));

        // Test deserialization
        Map<String, Object> result = parser.fromJson(json, Map.class);
        Assert.assertEquals("John", result.get("name"));
        // JSON numbers may be Integer or Double depending on parser
        Object age = result.get("age");
        Assert.assertTrue(age instanceof Number);
        Assert.assertEquals(30, ((Number) age).intValue());
    }

    @Test
    public void testGsonJsonParser() {
        JsonParser parser = new GsonJsonParser();

        // Test serialization
        Map<String, Object> data = Map.of("name", "Jane", "age", 25);
        String json = parser.toJson(data);
        Assert.assertTrue(json.contains("\"name\":\"Jane\""));
        Assert.assertTrue(json.contains("\"age\":25"));

        // Test deserialization
        Map<String, Object> result = parser.fromJson(json, Map.class);
        Assert.assertEquals("Jane", result.get("name"));
        // JSON numbers may be Integer or Double depending on parser
        Object age = result.get("age");
        Assert.assertTrue(age instanceof Number);
        Assert.assertEquals(25, ((Number) age).intValue());
    }

    @Test
    public void testCustomObjectSerialization() {
        // Create a simple test object
        Map<String, Object> person = Map.of(
            "name", "Alice",
            "age", 28,
            "active", true
        );

        // Test both parsers
        JsonParser jacksonParser = new JacksonJsonParser();
        JsonParser gsonParser = new GsonJsonParser();

        // Serialize with Jackson, deserialize with GSON
        String json = jacksonParser.toJson(person);
        Map<String, Object> result = gsonParser.fromJson(json, Map.class);
        Assert.assertEquals("Alice", result.get("name"));
        Object age1 = result.get("age");
        Assert.assertTrue(age1 instanceof Number);
        Assert.assertEquals(28, ((Number) age1).intValue());
        Assert.assertEquals(true, result.get("active"));

        // Serialize with GSON, deserialize with Jackson
        json = gsonParser.toJson(person);
        result = jacksonParser.fromJson(json, Map.class);
        Assert.assertEquals("Alice", result.get("name"));
        Object age2 = result.get("age");
        Assert.assertTrue(age2 instanceof Number);
        Assert.assertEquals(28, ((Number) age2).intValue());
        Assert.assertEquals(true, result.get("active"));
    }

    @Test
    public void testJacksonCustomObjectMapper() {
        ObjectMapper customMapper = new ObjectMapper();
        // Configure custom mapper if needed
        JsonParser parser = new JacksonJsonParser(customMapper);

        Map<String, Object> data = Map.of("test", "value");
        String json = parser.toJson(data);
        Assert.assertTrue(json.contains("\"test\":\"value\""));
    }

    @Test
    public void testBothParsersPreserveInt64ObjectNumbers() {
        for (JsonParser parser : new JsonParser[]{new JacksonJsonParser(), new GsonJsonParser()}) {
            Map<String, Object> result = parser.fromJson(
                "{\"max\":9223372036854775807,\"min\":-9223372036854775808,\"small\":42,\"ratio\":1.5}",
                Map.class);

            Assert.assertEquals(Long.valueOf(Long.MAX_VALUE), result.get("max"));
            Assert.assertEquals(Long.valueOf(Long.MIN_VALUE), result.get("min"));
            Assert.assertEquals(42, ((Number) result.get("small")).intValue());
            Assert.assertEquals(1.5, ((Number) result.get("ratio")).doubleValue(), 0.0);

            // Integer-form text outside int64 must stay exact and be rejected by
            // validation rather than being rounded into range.
            Object tooBig = parser.fromJson("{\"n\":9223372036854775808}", Map.class).get("n");
            Assert.assertTrue(tooBig instanceof BigInteger);
            Assert.assertFalse(Validation.validateInt(tooBig, "").isEmpty());
        }
    }

    @Test
    public void testGsonCustomInstance() {
        Gson customGson = new Gson();
        // Configure custom GSON if needed
        JsonParser parser = new GsonJsonParser(customGson);

        Map<String, Object> data = Map.of("test", "value");
        String json = parser.toJson(data);
        Assert.assertTrue(json.contains("\"test\":\"value\""));
    }
}
