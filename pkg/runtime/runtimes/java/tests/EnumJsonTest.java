import com.bitmechanic.pulserpc.*;
import org.junit.Test;
import org.junit.Assert;

/**
 * Invariant C11/R27: enums are JSON strings (their declared names), never
 * ordinals, under both supported JSON libraries.
 */
public class EnumJsonTest {

    public enum Colour {
        red,
        green
    }

    @Test
    public void testEnumsSerializeAsNamesWithBothParsers() {
        JsonParser[] parsers = { new JacksonJsonParser(), new GsonJsonParser() };
        for (JsonParser parser : parsers) {
            String name = parser.getClass().getSimpleName();
            Assert.assertEquals(name, "\"red\"", parser.toJson(Colour.red));
            Assert.assertEquals(name, Colour.green, parser.fromJson("\"green\"", Colour.class));
        }
    }
}
