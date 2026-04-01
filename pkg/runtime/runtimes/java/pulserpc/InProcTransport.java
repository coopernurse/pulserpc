package pulserpc;

/**
 * In-process transport for testing (directly calls Server)
 */
public class InProcTransport implements Transport {
    private final Server server;

    public InProcTransport(Server server) {
        this.server = server;
    }

    @Override
    public Response call(Request request) throws Exception {
        // Convert Request to Map for Server.call()
        java.util.Map<String, Object> requestMap = new java.util.HashMap<>();
        requestMap.put("jsonrpc", request.getJsonrpc());
        requestMap.put("method", request.getMethod());
        requestMap.put("id", request.getId());
        if (request.getParams() != null) {
            requestMap.put("params", request.getParams());
        }

        // Call server
        java.util.Map<String, Object> responseMap = server.call(requestMap);

        // Convert response Map back to Response object
        Response response = new Response();
        if (responseMap != null) {
            response.setJsonrpc((String) responseMap.get("jsonrpc"));
            response.setId(responseMap.get("id"));
            if (responseMap.containsKey("result")) {
                response.setResult(responseMap.get("result"));
            }
            if (responseMap.containsKey("error")) {
                response.setError((java.util.Map<String, Object>) responseMap.get("error"));
            }
        }

        return response;
    }
}
