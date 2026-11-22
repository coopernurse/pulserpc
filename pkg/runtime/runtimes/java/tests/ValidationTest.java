import com.bitmechanic.pulserpc.*;
import org.junit.Test;
import org.junit.Assert;
import java.util.*;

public class ValidationTest {

    @Test
    public void testValidateString() {
        // Valid string
        Validation.validateString("hello");

        // Invalid - null
        try {
            Validation.validateString(null);
            Assert.fail("Expected IllegalArgumentException");
        } catch (IllegalArgumentException e) {
            Assert.assertTrue(e.getMessage().contains("Expected string"));
        }

        // Invalid - integer
        try {
            Validation.validateString(123);
            Assert.fail("Expected IllegalArgumentException");
        } catch (IllegalArgumentException e) {
            Assert.assertTrue(e.getMessage().contains("Expected string"));
        }
    }

    @Test
    public void testValidateInt() {
        // Valid int
        Validation.validateInt(42);

        // Invalid - string
        try {
            Validation.validateInt("not an int");
            Assert.fail("Expected IllegalArgumentException");
        } catch (IllegalArgumentException e) {
            Assert.assertTrue(e.getMessage().contains("Expected int"));
        }
    }

    @Test
    public void testValidateIntSigned32BitRange() {
        // Invariant R25/D1: IDL ints are signed 32-bit.
        Validation.validateInt(Integer.MAX_VALUE);
        Validation.validateInt(Integer.MIN_VALUE);
        Validation.validateInt(2147483647L);
        Validation.validateInt(-2147483648L);
        Validation.validateInt(2147483647.0);
        Validation.validateInt(-2147483648.0);

        Object[] invalid = {
            2147483648L, -2147483649L,
            2147483648.0, -2147483649.0,
            Double.POSITIVE_INFINITY, Double.NaN,
            1.5, Double.valueOf(1.25)
        };
        for (Object value : invalid) {
            try {
                Validation.validateInt(value);
                Assert.fail("Expected IllegalArgumentException for " + value);
            } catch (IllegalArgumentException e) {
                Assert.assertTrue("message should name int: " + e.getMessage(),
                    e.getMessage().contains("Expected int"));
            }
        }
    }

    @Test
    public void testValidateFloat() {
        // java-08: integer values of every Number type are valid floats.
        Validation.validateFloat(5);
        Validation.validateFloat(5L);
        Validation.validateFloat(5.0f);
        Validation.validateFloat(5.0d);

        try {
            Validation.validateFloat("5");
            Assert.fail("Expected IllegalArgumentException");
        } catch (IllegalArgumentException e) {
            Assert.assertTrue(e.getMessage().contains("Expected float"));
        }
        try {
            Validation.validateFloat(true);
            Assert.fail("Expected IllegalArgumentException");
        } catch (IllegalArgumentException e) {
            Assert.assertTrue(e.getMessage().contains("Expected float"));
        }
    }

    @Test
    public void testValidateBool() {
        // Valid boolean
        Validation.validateBool(true);
        Validation.validateBool(false);

        // Invalid - string
        try {
            Validation.validateBool("true");
            Assert.fail("Expected IllegalArgumentException");
        } catch (IllegalArgumentException e) {
            Assert.assertTrue(e.getMessage().contains("Expected bool"));
        }
    }

    @Test
    public void testValidateArray() {
        List<String> stringArray = Arrays.asList("a", "b", "c");

        // Valid array of strings
        Validation.validateArray(stringArray, Validation::validateString);

        // Invalid - element validation fails
        List<Object> mixedArray = Arrays.asList("a", 123, "c");
        try {
            Validation.validateArray(mixedArray, Validation::validateString);
            Assert.fail("Expected IllegalArgumentException");
        } catch (IllegalArgumentException e) {
            Assert.assertTrue(e.getMessage().contains("Expected string"));
        }
    }

    @Test
    public void testValidateMap() {
        Map<String, String> stringMap = new HashMap<>();
        stringMap.put("key1", "value1");
        stringMap.put("key2", "value2");

        // Valid map of strings
        Validation.validateMap(stringMap, Validation::validateString);

        // Invalid - value validation fails
        Map<String, Object> mixedMap = new HashMap<>();
        mixedMap.put("key1", "value1");
        mixedMap.put("key2", 123);
        try {
            Validation.validateMap(mixedMap, Validation::validateString);
            Assert.fail("Expected IllegalArgumentException");
        } catch (IllegalArgumentException e) {
            Assert.assertTrue(e.getMessage().contains("Expected string"));
        }
    }

    @Test
    public void testValidateEnum() {
        List<String> allowedValues = Arrays.asList("RED", "GREEN", "BLUE");

        // Valid enum value
        Validation.validateEnum("RED", "Color", allowedValues);

        // Invalid - not in allowed values
        try {
            Validation.validateEnum("YELLOW", "Color", allowedValues);
            Assert.fail("Expected IllegalArgumentException");
        } catch (IllegalArgumentException e) {
            Assert.assertTrue(e.getMessage().contains("Invalid value"));
        }

        // Invalid - not a string
        try {
            Validation.validateEnum(123, "Color", allowedValues);
            Assert.fail("Expected IllegalArgumentException");
        } catch (IllegalArgumentException e) {
            Assert.assertTrue(e.getMessage().contains("Expected string"));
        }
    }

    private Map<String, Map<String, Object>> sampleStructs() {
        Map<String, Map<String, Object>> structs = new HashMap<>();
        structs.put("Address", Map.of("fields", Arrays.asList(
            Map.of("name", "street", "type", Map.of("builtIn", "string"))
        )));
        Map<String, Object> user = new HashMap<>();
        user.put("fields", Arrays.asList(
            Map.of("name", "email", "type", Map.of("builtIn", "string")),
            Map.of("name", "age", "type", Map.of("builtIn", "int"), "optional", true),
            Map.of("name", "address", "type", Map.of("userDefined", "Address")),
            Map.of("name", "status", "type", Map.of("userDefined", "Status")),
            Map.of("name", "tags", "type", Map.of("array", Map.of("builtIn", "string"))),
            Map.of("name", "scores", "type", Map.of("mapValue", Map.of("builtIn", "int")))
        ));
        structs.put("User", user);
        return structs;
    }

    private Map<String, Map<String, Object>> sampleEnums() {
        Map<String, Map<String, Object>> enums = new HashMap<>();
        enums.put("Status", Map.of("values", Arrays.asList(
            Map.of("name", "ACTIVE"), Map.of("name", "INACTIVE")
        )));
        return enums;
    }

    private Map<String, Object> validUser() {
        Map<String, Object> user = new HashMap<>();
        user.put("email", "a@example.com");
        user.put("address", Map.of("street", "Main St"));
        user.put("status", "ACTIVE");
        user.put("tags", Arrays.asList("x", "y"));
        user.put("scores", Map.of("math", 10));
        return user;
    }

    @Test
    public void testValidateStruct_RequiredOptionalAndFieldPaths() {
        Map<String, Map<String, Object>> structs = sampleStructs();
        Map<String, Map<String, Object>> enums = sampleEnums();

        Validation.validateStruct(validUser(), "User", structs.get("User"), structs, enums);

        // Missing required field
        Map<String, Object> missing = validUser();
        missing.remove("email");
        try {
            Validation.validateStruct(missing, "User", structs.get("User"), structs, enums);
            Assert.fail("Expected IllegalArgumentException");
        } catch (IllegalArgumentException e) {
            Assert.assertTrue(e.getMessage().contains("email"));
        }

        // Null required field
        Map<String, Object> nullRequired = validUser();
        nullRequired.put("email", null);
        try {
            Validation.validateStruct(nullRequired, "User", structs.get("User"), structs, enums);
            Assert.fail("Expected IllegalArgumentException");
        } catch (IllegalArgumentException e) {
            Assert.assertTrue(e.getMessage().contains("email"));
        }

        // Missing optional field is fine
        Map<String, Object> noAge = validUser();
        noAge.remove("age");
        Validation.validateStruct(noAge, "User", structs.get("User"), structs, enums);

        // Nested struct violation reports the field path
        Map<String, Object> badAddress = validUser();
        badAddress.put("address", Map.of("street", 123));
        try {
            Validation.validateStruct(badAddress, "User", structs.get("User"), structs, enums);
            Assert.fail("Expected IllegalArgumentException");
        } catch (IllegalArgumentException e) {
            // Message chains the outer and inner field names (address.street).
            Assert.assertTrue(e.getMessage().contains("address"));
            Assert.assertTrue(e.getMessage().contains("street"));
        }

        // Enum field violation reports the field path
        Map<String, Object> badEnum = validUser();
        badEnum.put("status", "UNKNOWN");
        try {
            Validation.validateStruct(badEnum, "User", structs.get("User"), structs, enums);
            Assert.fail("Expected IllegalArgumentException");
        } catch (IllegalArgumentException e) {
            Assert.assertTrue(e.getMessage().contains("status"));
        }

        // Array element violation reports the field path
        Map<String, Object> badArray = validUser();
        badArray.put("tags", Arrays.asList("ok", 7));
        try {
            Validation.validateStruct(badArray, "User", structs.get("User"), structs, enums);
            Assert.fail("Expected IllegalArgumentException");
        } catch (IllegalArgumentException e) {
            Assert.assertTrue(e.getMessage().contains("tags"));
        }

        // Map value violation reports the field path
        Map<String, Object> badMap = validUser();
        badMap.put("scores", Map.of("math", "oops"));
        try {
            Validation.validateStruct(badMap, "User", structs.get("User"), structs, enums);
            Assert.fail("Expected IllegalArgumentException");
        } catch (IllegalArgumentException e) {
            Assert.assertTrue(e.getMessage().contains("scores"));
        }
    }

    @Test
    public void testValidateType_QualifiedUserDefinedAndOptional() {
        Map<String, Map<String, Object>> structs = new HashMap<>();
        structs.put("inc.Address", Map.of("fields", Arrays.asList(
            Map.of("name", "street", "type", Map.of("builtIn", "string"))
        )));
        Map<String, Map<String, Object>> enums = new HashMap<>();

        // java-07: an unqualified userDefined resolves against a qualified key.
        Validation.validateType(Map.of("street", "Main"), Map.of("userDefined", "Address"), structs, enums, false);
        // ... and a qualified userDefined resolves against an unqualified key.
        Validation.validateType(Map.of("street", "Main"), Map.of("userDefined", "other.Address"), structs, enums, false);

        Validation.validateType(null, Map.of("builtIn", "string"), structs, enums, true);
        try {
            Validation.validateType(null, Map.of("builtIn", "string"), structs, enums, false);
            Assert.fail("Expected IllegalArgumentException");
        } catch (IllegalArgumentException e) {
            Assert.assertTrue(e.getMessage().contains("null"));
        }
    }
}
