import com.bitmechanic.pulserpc.*;
import org.junit.Test;
import org.junit.Assert;
import java.util.*;

public class TypesTest {

    @Test
    public void testFindStruct() {
        Map<String, Map<String, Object>> allStructs = new HashMap<>();

        Map<String, Object> personStruct = new HashMap<>();
        personStruct.put("fields", Arrays.asList(
            Map.of("name", "name", "type", Map.of("builtIn", "string")),
            Map.of("name", "age", "type", Map.of("builtIn", "int"))
        ));
        allStructs.put("Person", personStruct);

        // Found
        Map<String, Object> result = Types.findStruct("Person", allStructs);
        Assert.assertNotNull(result);
        Assert.assertEquals(personStruct, result);

        // Not found
        Map<String, Object> notFound = Types.findStruct("NonExistent", allStructs);
        Assert.assertNull(notFound);
    }

    @Test
    public void testFindEnum() {
        Map<String, Map<String, Object>> allEnums = new HashMap<>();

        Map<String, Object> colorEnum = new HashMap<>();
        colorEnum.put("values", Arrays.asList(
            Map.of("name", "RED"),
            Map.of("name", "GREEN"),
            Map.of("name", "BLUE")
        ));
        allEnums.put("Color", colorEnum);

        // Found
        Map<String, Object> result = Types.findEnum("Color", allEnums);
        Assert.assertNotNull(result);
        Assert.assertEquals(colorEnum, result);

        // Not found
        Map<String, Object> notFound = Types.findEnum("NonExistent", allEnums);
        Assert.assertNull(notFound);
    }

    @Test
    public void testGetStructFields() {
        Map<String, Map<String, Object>> allStructs = new HashMap<>();

        // Base struct
        Map<String, Object> baseStruct = new HashMap<>();
        baseStruct.put("fields", Arrays.asList(
            Map.of("name", "id", "type", Map.of("builtIn", "int"))
        ));
        allStructs.put("Base", baseStruct);

        // Derived struct
        Map<String, Object> derivedStruct = new HashMap<>();
        derivedStruct.put("extends", "Base");
        derivedStruct.put("fields", Arrays.asList(
            Map.of("name", "name", "type", Map.of("builtIn", "string"))
        ));
        allStructs.put("Person", derivedStruct);

        List<Map<String, Object>> fields = Types.getStructFields("Person", allStructs);

        // Should have fields from both base and derived class
        Assert.assertEquals(2, fields.size());

        // Check field names (order may vary due to inheritance resolution)
        Set<String> fieldNames = new HashSet<>();
        for (Map<String, Object> field : fields) {
            fieldNames.add((String) field.get("name"));
        }
        Assert.assertTrue(fieldNames.contains("id"));
        Assert.assertTrue(fieldNames.contains("name"));
    }

    @Test
    public void testGetStructFields_ChildOverridesParent() {
        // java-06: a child redeclaration must replace the parent's field.
        Map<String, Map<String, Object>> allStructs = new HashMap<>();
        Map<String, Object> base = new HashMap<>();
        base.put("fields", Arrays.asList(
            Map.of("name", "id", "type", Map.of("builtIn", "string"), "optional", false)
        ));
        allStructs.put("Base", base);

        Map<String, Object> child = new HashMap<>();
        child.put("extends", "Base");
        child.put("fields", Arrays.asList(
            Map.of("name", "id", "type", Map.of("builtIn", "int"), "optional", true)
        ));
        allStructs.put("Child", child);

        List<Map<String, Object>> fields = Types.getStructFields("Child", allStructs);
        Assert.assertEquals("exactly one field for the overridden name", 1, fields.size());
        @SuppressWarnings("unchecked")
        Map<String, Object> type = (Map<String, Object>) fields.get(0).get("type");
        Assert.assertEquals("child's type wins", "int", type.get("builtIn"));
        Assert.assertEquals(Boolean.TRUE, fields.get(0).get("optional"));
    }

    @Test
    public void testFindStruct_QualifiedNameResolution() {
        // java-07: unqualified request resolves against a qualified map key and
        // a qualified request resolves against an unqualified key.
        Map<String, Map<String, Object>> structs = new HashMap<>();
        Map<String, Object> response = Map.of("name", "inc.Response");
        structs.put("inc.Response", response);

        Assert.assertEquals(response, Types.findStruct("Response", structs));
        Assert.assertEquals(response, Types.findStruct("other.Response", structs));
        Assert.assertEquals(response, Types.findStruct("inc.Response", structs));
        Assert.assertNull(Types.findStruct("Missing", structs));

        Map<String, Map<String, Object>> unqualified = new HashMap<>();
        unqualified.put("Status", Map.of("name", "Status"));
        Assert.assertNotNull(Types.findStruct("inc.Status", unqualified));
    }

    @Test
    public void testFindEnum_QualifiedNameResolution() {
        Map<String, Map<String, Object>> enums = new HashMap<>();
        Map<String, Object> color = Map.of("name", "inc.Color");
        enums.put("inc.Color", color);
        Assert.assertEquals(color, Types.findEnum("Color", enums));
        Assert.assertNull(Types.findEnum("Nope", enums));
    }

    @Test
    public void testGetStructFields_QualifiedExtends() {
        // java-07: an extends value qualified with a namespace must still be
        // resolved when collecting inherited fields.
        Map<String, Map<String, Object>> allStructs = new HashMap<>();
        allStructs.put("inc.Base", Map.of("fields", Arrays.asList(
            Map.of("name", "id", "type", Map.of("builtIn", "int"))
        )));
        allStructs.put("inc.Child", Map.of(
            "extends", "inc.Base",
            "fields", Arrays.asList(Map.of("name", "name", "type", Map.of("builtIn", "string")))
        ));

        List<Map<String, Object>> fields = Types.getStructFields("Child", allStructs);
        Assert.assertEquals(2, fields.size());
    }

    @Test
    public void testResolutionIsDeterministicAcrossLookups() {
        // Cached sorted keys must preserve the old deterministic tie-breaking.
        Map<String, Map<String, Object>> structs = new LinkedHashMap<>();
        Map<String, Object> alpha = Map.of("name", "a.Other");
        Map<String, Object> zeta = Map.of("name", "z.Other");
        structs.put("z.Other", zeta);   // insertion order differs from sorted order
        structs.put("a.Other", alpha);
        for (int i = 0; i < 10; i++) {
            Assert.assertEquals("qualified request tie-break", alpha, Types.findStruct("req.Other", structs));
            Assert.assertEquals("suffix request tie-break", alpha, Types.findStruct("Other", structs));
        }

        // Interface resolution uses the same cached, deterministic ordering.
        Map<String, Object> idl = Map.of("interfaces", List.of(
            Map.of("name", "z.Svc", "methods", List.of()),
            Map.of("name", "a.Svc", "methods", List.of())
        ));
        Contract contract = new Contract(idl);
        for (int i = 0; i < 10; i++) {
            Assert.assertEquals("interface tie-break", "a.Svc", contract.findInterfaceName("req.Svc"));
        }

        // A definitions map that grows after the cache was warmed must not
        // return a stale sorted key list.
        Map<String, Object> zero = Map.of("name", "0.Other");
        structs.put("0.Other", zero);
        Assert.assertEquals(zero, Types.findStruct("req.Other", structs));
    }
}
