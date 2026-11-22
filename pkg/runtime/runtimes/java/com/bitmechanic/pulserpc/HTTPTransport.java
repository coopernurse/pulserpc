package com.bitmechanic.pulserpc;

import java.io.IOException;
import java.io.InputStream;
import java.net.URI;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.util.Collections;
import java.util.LinkedHashMap;
import java.util.Map;

/**
 * HTTP implementation of Transport that makes HTTP POST requests
 */
public class HTTPTransport implements Transport {
    private final HttpClient httpClient;
    private final String baseUrl;
    private final JsonParser jsonParser;
    private final Map<String, String> headers;

    /**
     * Creates a transport with no custom headers.
     */
    public HTTPTransport(String baseUrl, JsonParser jsonParser) {
        this(baseUrl, jsonParser, Collections.emptyMap());
    }

    /**
     * Creates a transport that sends {@code headers} with every request
     * (invariant R29/D13). A trailing slash on {@code baseUrl} is normalized
     * away (invariant R30/D13).
     */
    public HTTPTransport(String baseUrl, JsonParser jsonParser, Map<String, String> headers) {
        String normalized = baseUrl;
        while (normalized.endsWith("/")) {
            normalized = normalized.substring(0, normalized.length() - 1);
        }
        this.baseUrl = normalized;
        this.jsonParser = jsonParser;
        this.headers = headers == null
            ? Collections.emptyMap()
            : Collections.unmodifiableMap(new LinkedHashMap<>(headers));
        this.httpClient = HttpClient.newBuilder()
            .connectTimeout(HttpLimits.CONNECT_TIMEOUT)
            .build();
    }

    @Override
    public Response call(Request request) throws Exception {
        String requestJson = jsonParser.toJson(request);

        HttpRequest.Builder builder = HttpRequest.newBuilder()
            .uri(URI.create(baseUrl))
            .setHeader("Content-Type", "application/json")
            .POST(HttpRequest.BodyPublishers.ofString(requestJson))
            .timeout(HttpLimits.REQUEST_TIMEOUT);
        for (Map.Entry<String, String> header : headers.entrySet()) {
            builder.setHeader(header.getKey(), header.getValue());
        }

        HttpResponse<InputStream> httpResponse =
            httpClient.send(builder.build(), HttpResponse.BodyHandlers.ofInputStream());

        if (httpResponse.statusCode() != 200) {
            throw new IOException("HTTP error: " + httpResponse.statusCode());
        }

        // Invariant R17/D5: bound the response body before reading it.
        long declaredLength = httpResponse.headers().firstValueAsLong("Content-Length").orElse(-1L);
        if (declaredLength > HttpLimits.MAX_RESPONSE_BODY) {
            throw new IOException("Response body exceeds maximum size of "
                + HttpLimits.MAX_RESPONSE_BODY + " bytes");
        }
        String body;
        try (InputStream in = httpResponse.body()) {
            body = new String(HttpLimits.readBounded(in, HttpLimits.MAX_RESPONSE_BODY, "Response body"));
        }

        // java-14: validate the raw envelope so the check is parser-agnostic
        // (Gson bypasses setters, so a parsed Response cannot be trusted here).
        // Parse the body only once; validate that Map and build the Response
        // from the same parsed data rather than deserializing the body twice.
        Map<String, Object> envelope = jsonParser.fromJson(body, Map.class);
        Response.validateMap(envelope, request.getId());
        Response response = Response.fromMap(envelope);

        if (response.hasError()) {
            Map<String, Object> error = response.getError();
            int code = error.containsKey("code") ? ((Number) error.get("code")).intValue() : -32603;
            String message = error.containsKey("message") ? (String) error.get("message") : "Unknown error";
            Object data = error.get("data");
            throw new RPCError(code, message, data);
        }

        return response;
    }
}
