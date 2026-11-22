import com.bitmechanic.pulserpc.*;
import org.junit.Test;
import org.junit.Assert;

import java.util.ArrayList;
import java.util.Arrays;
import java.util.HashMap;
import java.util.List;
import java.util.Map;

/**
 * Invariant R31/D15: verification diffs the local contract against the IDL
 * fetched from the server, and a divergent server IDL is incompatible.
 */
public class ContractVerifyTest {

    private static Map<String, Object> idlWithMethod(String methodName) {
        Map<String, Object> method = new HashMap<>();
        method.put("name", methodName);
        method.put("returnType", Map.of("builtIn", "string"));

        Map<String, Object> iface = new HashMap<>();
        iface.put("name", "Svc");
        iface.put("methods", Arrays.asList(method));

        Map<String, Object> idl = new HashMap<>();
        idl.put("rootNamespace", "test");
        idl.put("interfaces", Arrays.asList(iface));
        idl.put("structs", new ArrayList<>());
        idl.put("enums", new ArrayList<>());
        return idl;
    }

    @Test
    public void testDivergentServerIDLIsIncompatible() {
        // The runtime Client fetches pulserpc-idl during bootstrap, so the
        // server side of the diff is the fetched contract, not the local one.
        Contract serverContract = new Contract(idlWithMethod("b"));
        Server server = new Server(serverContract, false, false);
        Client client = new Client(new InProcTransport(server), false, false);

        JsonParser parser = new JacksonJsonParser();
        client.setLocalIDL(parser.toJson(idlWithMethod("a")), parser);

        VerificationResult result = client.verifyCompatibility();
        Assert.assertFalse(result.isCompatible());
        Assert.assertFalse("expected at least one delta", result.getDeltas().isEmpty());
    }

    @Test
    public void testIdenticalServerIDLIsCompatible() {
        Contract serverContract = new Contract(idlWithMethod("a"));
        Server server = new Server(serverContract, false, false);
        Client client = new Client(new InProcTransport(server), false, false);

        JsonParser parser = new JacksonJsonParser();
        client.setLocalIDL(parser.toJson(idlWithMethod("a")), parser);

        VerificationResult result = client.verifyCompatibility();
        Assert.assertTrue("identical IDLs must be compatible: " + result.getDeltas(),
            result.isCompatible());
    }
}
