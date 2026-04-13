import com.bitmechanic.pulserpc.*;
import com.fasterxml.jackson.databind.ObjectMapper;
import org.junit.Test;
import org.junit.Assert;
import java.util.*;

public class DiffEngineTest {

    private ObjectMapper mapper = new ObjectMapper();

    @Test
    public void testDiffIDL_IdenticalIDLs_ReturnsNoDeltas() throws Exception {
        String idl = "{\"interfaces\":[{\"name\":\"TestService\",\"methods\":[{\"name\":\"testMethod\",\"parameters\":[{\"name\":\"arg1\",\"type\":\"string\"}],\"returnType\":{\"type\":\"string\"}}]}],\"structs\":[{\"name\":\"TestStruct\",\"fields\":[{\"name\":\"field1\",\"type\":\"string\"}]}]}";

        Object clientIDL = mapper.readValue(idl, Object.class);
        Object serverIDL = mapper.readValue(idl, Object.class);

        List<ContractDelta> deltas = DiffEngine.diffIDL(clientIDL, serverIDL);

        Assert.assertEquals(0, deltas.size());
    }

    @Test
    public void testDiffIDL_AddedOptionalField_ReturnsInfoSeverity() throws Exception {
        Object clientIDL = mapper.readValue("{\"interfaces\":[],\"structs\":[{\"name\":\"TestStruct\",\"fields\":[{\"name\":\"existingField\",\"type\":\"string\",\"optional\":false}]}]}", Object.class);
        Object serverIDL = mapper.readValue("{\"interfaces\":[],\"structs\":[{\"name\":\"TestStruct\",\"fields\":[{\"name\":\"existingField\",\"type\":\"string\",\"optional\":false},{\"name\":\"newField\",\"type\":\"int\",\"optional\":true}]}]}", Object.class);

        List<ContractDelta> deltas = DiffEngine.diffIDL(clientIDL, serverIDL);

        Assert.assertEquals(1, deltas.size());
        Assert.assertEquals(ContractDelta.Severity.Info, deltas.get(0).getSeverity());
        Assert.assertEquals(ContractDelta.ChangeType.Added, deltas.get(0).getChangeType());
        Assert.assertEquals(ContractDelta.Direction.ClientHasLess, deltas.get(0).getDirection());
    }

    @Test
    public void testDiffIDL_AddedRequiredField_ReturnsErrorSeverity() throws Exception {
        Object clientIDL = mapper.readValue("{\"interfaces\":[],\"structs\":[{\"name\":\"TestStruct\",\"fields\":[{\"name\":\"existingField\",\"type\":\"string\",\"optional\":false}]}]}", Object.class);
        Object serverIDL = mapper.readValue("{\"interfaces\":[],\"structs\":[{\"name\":\"TestStruct\",\"fields\":[{\"name\":\"existingField\",\"type\":\"string\",\"optional\":false},{\"name\":\"newField\",\"type\":\"int\",\"optional\":false}]}]}", Object.class);

        List<ContractDelta> deltas = DiffEngine.diffIDL(clientIDL, serverIDL);

        Assert.assertEquals(1, deltas.size());
        Assert.assertEquals(ContractDelta.Severity.Error, deltas.get(0).getSeverity());
    }

    @Test
    public void testDiffIDL_RemovedField_ReturnsInfoSeverity() throws Exception {
        Object clientIDL = mapper.readValue("{\"interfaces\":[],\"structs\":[{\"name\":\"TestStruct\",\"fields\":[{\"name\":\"existingField\",\"type\":\"string\",\"optional\":false},{\"name\":\"oldField\",\"type\":\"int\",\"optional\":false}]}]}", Object.class);
        Object serverIDL = mapper.readValue("{\"interfaces\":[],\"structs\":[{\"name\":\"TestStruct\",\"fields\":[{\"name\":\"existingField\",\"type\":\"string\",\"optional\":false}]}]}", Object.class);

        List<ContractDelta> deltas = DiffEngine.diffIDL(clientIDL, serverIDL);

        Assert.assertEquals(1, deltas.size());
        Assert.assertEquals(ContractDelta.Severity.Info, deltas.get(0).getSeverity());
        Assert.assertEquals(ContractDelta.ChangeType.Removed, deltas.get(0).getChangeType());
        Assert.assertEquals(ContractDelta.Direction.ClientHasMore, deltas.get(0).getDirection());
    }

    @Test
    public void testDiffIDL_FieldMadeOptional_ReturnsInfoSeverity() throws Exception {
        Object clientIDL = mapper.readValue("{\"interfaces\":[],\"structs\":[{\"name\":\"TestStruct\",\"fields\":[{\"name\":\"status\",\"type\":\"string\",\"optional\":false}]}]}", Object.class);
        Object serverIDL = mapper.readValue("{\"interfaces\":[],\"structs\":[{\"name\":\"TestStruct\",\"fields\":[{\"name\":\"status\",\"type\":\"string\",\"optional\":true}]}]}", Object.class);

        List<ContractDelta> deltas = DiffEngine.diffIDL(clientIDL, serverIDL);

        Assert.assertEquals(1, deltas.size());
        Assert.assertEquals(ContractDelta.Severity.Info, deltas.get(0).getSeverity());
        Assert.assertEquals(ContractDelta.ChangeType.Modified, deltas.get(0).getChangeType());
    }

    @Test
    public void testDiffIDL_FieldMadeRequired_ReturnsWarningSeverity() throws Exception {
        Object clientIDL = mapper.readValue("{\"interfaces\":[],\"structs\":[{\"name\":\"TestStruct\",\"fields\":[{\"name\":\"status\",\"type\":\"string\",\"optional\":true}]}]}", Object.class);
        Object serverIDL = mapper.readValue("{\"interfaces\":[],\"structs\":[{\"name\":\"TestStruct\",\"fields\":[{\"name\":\"status\",\"type\":\"string\",\"optional\":false}]}]}", Object.class);

        List<ContractDelta> deltas = DiffEngine.diffIDL(clientIDL, serverIDL);

        Assert.assertEquals(1, deltas.size());
        Assert.assertEquals(ContractDelta.Severity.Warning, deltas.get(0).getSeverity());
        Assert.assertEquals(ContractDelta.ChangeType.Modified, deltas.get(0).getChangeType());
    }

    @Test
    public void testComputeChecksum_SameIDL_ReturnsSameChecksum() throws Exception {
        Object idl = mapper.readValue("{\"test\":\"data\"}", Object.class);

        String checksum1 = DiffEngine.computeChecksum(idl);
        String checksum2 = DiffEngine.computeChecksum(idl);

        Assert.assertEquals(checksum1, checksum2);
    }

    @Test
    public void testComputeChecksum_DifferentIDL_ReturnsDifferentChecksum() throws Exception {
        Object idl1 = mapper.readValue("{\"test\":\"data1\"}", Object.class);
        Object idl2 = mapper.readValue("{\"test\":\"data2\"}", Object.class);

        String checksum1 = DiffEngine.computeChecksum(idl1);
        String checksum2 = DiffEngine.computeChecksum(idl2);

        Assert.assertNotEquals(checksum1, checksum2);
    }

    @Test
    public void testDiffIDL_StructRemovedFromServer_ReturnsErrorSeverity() throws Exception {
        Object clientIDL = mapper.readValue("{\"structs\":[{\"name\":\"TestStruct\",\"fields\":[{\"name\":\"field1\",\"type\":\"string\"}]}]}", Object.class);
        Object serverIDL = mapper.readValue("{\"structs\":[]}", Object.class);

        List<ContractDelta> deltas = DiffEngine.diffIDL(clientIDL, serverIDL);

        Assert.assertEquals(1, deltas.size());
        Assert.assertEquals(ContractDelta.Severity.Error, deltas.get(0).getSeverity());
        Assert.assertEquals(ContractDelta.EntityType.Struct, deltas.get(0).getEntityType());
    }

    @Test
    public void testDiffIDL_InterfaceAddedToServer_ReturnsInfoSeverity() throws Exception {
        Object clientIDL = mapper.readValue("{\"interfaces\":[]}", Object.class);
        Object serverIDL = mapper.readValue("{\"interfaces\":[{\"name\":\"NewService\",\"methods\":[]}]}", Object.class);

        List<ContractDelta> deltas = DiffEngine.diffIDL(clientIDL, serverIDL);

        Assert.assertEquals(1, deltas.size());
        Assert.assertEquals(ContractDelta.Severity.Info, deltas.get(0).getSeverity());
        Assert.assertEquals(ContractDelta.EntityType.Interface, deltas.get(0).getEntityType());
    }
}