import com.bitmechanic.pulserpc.ContractAuditor;
import com.bitmechanic.pulserpc.ContractDelta;
import com.bitmechanic.pulserpc.VerificationResult;
import org.junit.Test;
import org.junit.Assert;
import java.util.*;

public class AuditorTest {

    @Test
    public void testNoOpAuditor_Name_ReturnsNoOp() {
        ContractAuditor auditor = ContractAuditor.noOp();
        Assert.assertEquals("NoOp", auditor.name());
    }

    @Test
    public void testNoOpAuditor_AuditCompatible_DoesNotThrow() {
        ContractAuditor auditor = ContractAuditor.noOp();
        List<ContractDelta> deltas = new ArrayList<>();
        VerificationResult result = new VerificationResult(
            true, "abc123", "def456", deltas, System.currentTimeMillis()
        );

        try {
            auditor.audit(result);
        } catch (Exception e) {
            Assert.fail("Should not throw: " + e.getMessage());
        }
    }

    @Test
    public void testNoOpAuditor_AuditIncompatible_DoesNotThrow() {
        ContractAuditor auditor = ContractAuditor.noOp();
        List<ContractDelta> deltas = new ArrayList<>();
        deltas.add(new ContractDelta(
            ContractDelta.EntityType.Struct,
            "TestStruct",
            "",
            ContractDelta.ChangeType.Removed,
            ContractDelta.Direction.ClientHasMore,
            ContractDelta.Severity.Error,
            "Struct 'TestStruct' removed"
        ));
        VerificationResult result = new VerificationResult(
            false, "abc123", "def456", deltas, System.currentTimeMillis()
        );

        try {
            auditor.audit(result);
        } catch (Exception e) {
            Assert.fail("Should not throw: " + e.getMessage());
        }
    }

    @Test
    public void testLoggingAuditor_Name_ReturnsLogging() {
        ContractAuditor auditor = ContractAuditor.logging();
        Assert.assertEquals("Logging", auditor.name());
    }

    @Test
    public void testLoggingAuditor_AuditCompatible_DoesNotThrow() {
        ContractAuditor auditor = ContractAuditor.logging();
        List<ContractDelta> deltas = new ArrayList<>();
        VerificationResult result = new VerificationResult(
            true, "abc123", "def456", deltas, System.currentTimeMillis()
        );

        try {
            auditor.audit(result);
        } catch (Exception e) {
            Assert.fail("Should not throw: " + e.getMessage());
        }
    }

    @Test
    public void testFailFastAuditor_Name_ReturnsFailFast() {
        ContractAuditor auditor = ContractAuditor.failFast();
        Assert.assertEquals("FailFast", auditor.name());
    }

    @Test
    public void testFailFastAuditor_AuditCompatible_DoesNotThrow() {
        ContractAuditor auditor = ContractAuditor.failFast();
        List<ContractDelta> deltas = new ArrayList<>();
        VerificationResult result = new VerificationResult(
            true, "abc123", "def456", deltas, System.currentTimeMillis()
        );

        try {
            auditor.audit(result);
        } catch (Exception e) {
            Assert.fail("Should not throw: " + e.getMessage());
        }
    }

    @Test
    public void testFailFastAuditor_AuditIncompatibleWithErrorDelta_ThrowsException() {
        ContractAuditor auditor = ContractAuditor.failFast();
        List<ContractDelta> deltas = new ArrayList<>();
        deltas.add(new ContractDelta(
            ContractDelta.EntityType.Struct,
            "TestStruct",
            "",
            ContractDelta.ChangeType.Removed,
            ContractDelta.Direction.ClientHasMore,
            ContractDelta.Severity.Error,
            "Struct 'TestStruct' removed"
        ));
        VerificationResult result = new VerificationResult(
            false, "abc123", "def456", deltas, System.currentTimeMillis()
        );

        boolean threw = false;
        try {
            auditor.audit(result);
        } catch (RuntimeException e) {
            threw = true;
        }
        Assert.assertTrue("Should throw RuntimeException for incompatible contract", threw);
    }

    @Test
    public void testFailFastAuditor_AuditWithWarningOnly_DoesNotThrow() {
        ContractAuditor auditor = ContractAuditor.failFast();
        List<ContractDelta> deltas = new ArrayList<>();
        deltas.add(new ContractDelta(
            ContractDelta.EntityType.Enum,
            "TestEnum",
            "",
            ContractDelta.ChangeType.Added,
            ContractDelta.Direction.ClientHasLess,
            ContractDelta.Severity.Warning,
            "Enum value added"
        ));
        VerificationResult result = new VerificationResult(
            true, "abc123", "def456", deltas, System.currentTimeMillis()
        );

        try {
            auditor.audit(result);
        } catch (Exception e) {
            Assert.fail("Should not throw for warning-level deltas: " + e.getMessage());
        }
    }

    @Test
    public void testFailFastAuditor_AuditWithInfoOnly_DoesNotThrow() {
        ContractAuditor auditor = ContractAuditor.failFast();
        List<ContractDelta> deltas = new ArrayList<>();
        deltas.add(new ContractDelta(
            ContractDelta.EntityType.Field,
            "TestStruct",
            "newField",
            ContractDelta.ChangeType.Added,
            ContractDelta.Direction.ClientHasLess,
            ContractDelta.Severity.Info,
            "Field 'newField' added"
        ));
        VerificationResult result = new VerificationResult(
            true, "abc123", "def456", deltas, System.currentTimeMillis()
        );

        try {
            auditor.audit(result);
        } catch (Exception e) {
            Assert.fail("Should not throw for info-level deltas: " + e.getMessage());
        }
    }
}