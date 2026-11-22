import com.bitmechanic.pulserpc.*;
import org.junit.After;
import org.junit.Test;
import org.junit.Assert;

import java.io.ByteArrayOutputStream;
import java.io.PrintStream;
import java.nio.charset.StandardCharsets;

/**
 * Invariant R15/D4: internal error details are logged server-side only by the
 * single shared helper used by both the runtime Server and the generated Java
 * server.
 */
public class InternalErrorsTest {

    private final PrintStream originalErr = System.err;
    private final ByteArrayOutputStream captured = new ByteArrayOutputStream();

    private void captureStderr() {
        System.setErr(new PrintStream(captured, true, StandardCharsets.UTF_8));
    }

    private String stderr() {
        return new String(captured.toByteArray(), StandardCharsets.UTF_8);
    }

    @After
    public void restoreStderr() {
        System.setErr(originalErr);
    }

    @Test
    public void testLogIncludesContextExceptionAndMessageOnStderr() {
        captureStderr();
        InternalErrors.log("method Svc.boom", new IllegalStateException("secret-token"));

        String out = stderr();
        Assert.assertTrue(out, out.contains("Internal error in method Svc.boom"));
        Assert.assertTrue(out, out.contains("java.lang.IllegalStateException"));
        Assert.assertTrue(out, out.contains("secret-token"));
    }

    @Test
    public void testLogToleratesNullThrowable() {
        captureStderr();
        InternalErrors.log("request dispatch", null);

        String out = stderr();
        Assert.assertTrue(out, out.contains("Internal error in request dispatch"));
        Assert.assertTrue(out, out.contains("unknown error"));
    }
}
