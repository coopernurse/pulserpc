import com.bitmechanic.pulserpc.*;
import org.junit.Test;
import org.junit.Assert;

import java.util.ArrayList;
import java.util.HashMap;
import java.util.List;
import java.util.Map;
import java.util.Set;
import java.util.concurrent.ConcurrentHashMap;
import java.util.concurrent.CountDownLatch;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;
import java.util.concurrent.Future;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.atomic.AtomicBoolean;

/**
 * Invariant R20: concurrent client calls use unique request ids.
 */
public class ClientConcurrencyTest {

    private static final class RecordingTransport implements Transport {
        final Set<Integer> ids = ConcurrentHashMap.newKeySet();
        final AtomicBoolean duplicate = new AtomicBoolean(false);

        @Override
        public Response call(Request request) throws Exception {
            Object id = request.getId();
            if ("pulserpc-idl".equals(request.getMethod())) {
                Map<String, Object> idl = new HashMap<>();
                idl.put("interfaces", List.of());
                return response(id, idl);
            }
            int numericId = ((Number) id).intValue();
            if (!ids.add(numericId)) {
                duplicate.set(true);
            }
            return response(id, "ok");
        }

        private Response response(Object id, Object result) {
            Response response = new Response();
            response.setJsonrpc("2.0");
            response.setId(id);
            response.setResult(result);
            return response;
        }
    }

    @Test
    public void testConcurrentCallsUseUniqueIds() throws Exception {
        RecordingTransport transport = new RecordingTransport();
        Client client = new Client(transport, false, false);

        int threads = 16;
        int perThread = 1000;
        ExecutorService pool = Executors.newFixedThreadPool(threads);
        CountDownLatch start = new CountDownLatch(1);
        List<Future<?>> futures = new ArrayList<>();
        for (int t = 0; t < threads; t++) {
            futures.add(pool.submit(() -> {
                start.await();
                for (int i = 0; i < perThread; i++) {
                    client.call("Svc.ping", new ArrayList<>());
                }
                return null;
            }));
        }
        start.countDown();
        for (Future<?> future : futures) {
            future.get(30, TimeUnit.SECONDS);
        }
        pool.shutdown();

        Assert.assertFalse("duplicate request id detected", transport.duplicate.get());
        Assert.assertEquals(threads * perThread, transport.ids.size());
    }
}
