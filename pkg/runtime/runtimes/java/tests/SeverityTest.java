import com.bitmechanic.pulserpc.ContractDelta;
import com.bitmechanic.pulserpc.DiffEngine;
import org.junit.Test;
import org.junit.Assert;

public class SeverityTest {

    @Test
    public void testClassifySeverity_StructRemoved_ReturnsError() {
        ContractDelta.Severity result = DiffEngine.classifySeverity(
            ContractDelta.EntityType.Struct,
            ContractDelta.ChangeType.Removed,
            ContractDelta.Direction.ClientHasMore
        );
        Assert.assertEquals(ContractDelta.Severity.Error, result);
    }

    @Test
    public void testClassifySeverity_StructAdded_ReturnsInfo() {
        ContractDelta.Severity result = DiffEngine.classifySeverity(
            ContractDelta.EntityType.Struct,
            ContractDelta.ChangeType.Added,
            ContractDelta.Direction.ClientHasLess
        );
        Assert.assertEquals(ContractDelta.Severity.Info, result);
    }

    @Test
    public void testClassifySeverity_FieldRemoved_ReturnsInfo() {
        ContractDelta.Severity result = DiffEngine.classifySeverity(
            ContractDelta.EntityType.Field,
            ContractDelta.ChangeType.Removed,
            ContractDelta.Direction.ClientHasMore
        );
        Assert.assertEquals(ContractDelta.Severity.Info, result);
    }

    @Test
    public void testClassifySeverity_FieldTypeChanged_ReturnsError() {
        ContractDelta.Severity result = DiffEngine.classifySeverity(
            ContractDelta.EntityType.Field,
            ContractDelta.ChangeType.Modified,
            ContractDelta.Direction.Mismatch
        );
        Assert.assertEquals(ContractDelta.Severity.Error, result);
    }

    @Test
    public void testClassifySeverity_FieldAddedRequired_ReturnsError() {
        ContractDelta.Severity result = DiffEngine.classifySeverity(
            ContractDelta.EntityType.Field,
            ContractDelta.ChangeType.Added,
            ContractDelta.Direction.ClientHasLess,
            "required"
        );
        Assert.assertEquals(ContractDelta.Severity.Error, result);
    }

    @Test
    public void testClassifySeverity_FieldAddedOptional_ReturnsInfo() {
        ContractDelta.Severity result = DiffEngine.classifySeverity(
            ContractDelta.EntityType.Field,
            ContractDelta.ChangeType.Added,
            ContractDelta.Direction.ClientHasLess,
            "optional"
        );
        Assert.assertEquals(ContractDelta.Severity.Info, result);
    }

    @Test
    public void testClassifySeverity_FieldMadeOptional_ReturnsInfo() {
        ContractDelta.Severity result = DiffEngine.classifySeverity(
            ContractDelta.EntityType.Field,
            ContractDelta.ChangeType.Modified,
            ContractDelta.Direction.ClientHasLess,
            "made_optional"
        );
        Assert.assertEquals(ContractDelta.Severity.Info, result);
    }

    @Test
    public void testClassifySeverity_FieldMadeRequired_ReturnsWarning() {
        ContractDelta.Severity result = DiffEngine.classifySeverity(
            ContractDelta.EntityType.Field,
            ContractDelta.ChangeType.Modified,
            ContractDelta.Direction.ClientHasLess,
            "made_required"
        );
        Assert.assertEquals(ContractDelta.Severity.Warning, result);
    }

    @Test
    public void testClassifySeverity_MethodRemoved_ReturnsError() {
        ContractDelta.Severity result = DiffEngine.classifySeverity(
            ContractDelta.EntityType.Method,
            ContractDelta.ChangeType.Removed,
            ContractDelta.Direction.ClientHasMore
        );
        Assert.assertEquals(ContractDelta.Severity.Error, result);
    }

    @Test
    public void testClassifySeverity_MethodAdded_ReturnsWarning() {
        ContractDelta.Severity result = DiffEngine.classifySeverity(
            ContractDelta.EntityType.Method,
            ContractDelta.ChangeType.Added,
            ContractDelta.Direction.ClientHasLess
        );
        Assert.assertEquals(ContractDelta.Severity.Warning, result);
    }

    @Test
    public void testClassifySeverity_MethodModified_ReturnsError() {
        ContractDelta.Severity result = DiffEngine.classifySeverity(
            ContractDelta.EntityType.Method,
            ContractDelta.ChangeType.Modified,
            ContractDelta.Direction.Mismatch
        );
        Assert.assertEquals(ContractDelta.Severity.Error, result);
    }

    @Test
    public void testClassifySeverity_EnumRemoved_ReturnsWarning() {
        ContractDelta.Severity result = DiffEngine.classifySeverity(
            ContractDelta.EntityType.Enum,
            ContractDelta.ChangeType.Removed,
            ContractDelta.Direction.ClientHasMore
        );
        Assert.assertEquals(ContractDelta.Severity.Warning, result);
    }

    @Test
    public void testClassifySeverity_EnumAdded_ReturnsWarning() {
        ContractDelta.Severity result = DiffEngine.classifySeverity(
            ContractDelta.EntityType.Enum,
            ContractDelta.ChangeType.Added,
            ContractDelta.Direction.ClientHasLess
        );
        Assert.assertEquals(ContractDelta.Severity.Warning, result);
    }

    @Test
    public void testClassifySeverity_ErrorRemoved_ReturnsInfo() {
        ContractDelta.Severity result = DiffEngine.classifySeverity(
            ContractDelta.EntityType.Error,
            ContractDelta.ChangeType.Removed,
            ContractDelta.Direction.ClientHasMore
        );
        Assert.assertEquals(ContractDelta.Severity.Info, result);
    }

    @Test
    public void testClassifySeverity_ErrorAdded_ReturnsInfo() {
        ContractDelta.Severity result = DiffEngine.classifySeverity(
            ContractDelta.EntityType.Error,
            ContractDelta.ChangeType.Added,
            ContractDelta.Direction.ClientHasLess
        );
        Assert.assertEquals(ContractDelta.Severity.Info, result);
    }

    @Test
    public void testClassifySeverity_InterfaceRemoved_ReturnsError() {
        ContractDelta.Severity result = DiffEngine.classifySeverity(
            ContractDelta.EntityType.Interface,
            ContractDelta.ChangeType.Removed,
            ContractDelta.Direction.ClientHasMore
        );
        Assert.assertEquals(ContractDelta.Severity.Error, result);
    }

    @Test
    public void testClassifySeverity_InterfaceAdded_ReturnsInfo() {
        ContractDelta.Severity result = DiffEngine.classifySeverity(
            ContractDelta.EntityType.Interface,
            ContractDelta.ChangeType.Added,
            ContractDelta.Direction.ClientHasLess
        );
        Assert.assertEquals(ContractDelta.Severity.Info, result);
    }

    @Test
    public void testClassifySeverity_UnknownCombination_ReturnsInfo() {
        ContractDelta.Severity result = DiffEngine.classifySeverity(
            ContractDelta.EntityType.Interface,
            ContractDelta.ChangeType.Modified,
            ContractDelta.Direction.Mismatch
        );
        Assert.assertEquals(ContractDelta.Severity.Info, result);
    }
}