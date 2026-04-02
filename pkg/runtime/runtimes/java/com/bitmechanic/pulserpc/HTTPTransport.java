package com.bitmechanic.pulserpc;

import java.io.IOException;
import java.net.URI;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.time.Duration;
import java.util.Map;

/**
 * HTTP implementation of Transport that makes HTTP POST requests
 */
public class HTTPTransport implements Transport {
    private final HttpClient httpClient;
    private final String baseUrl;
    private final JsonParser jsonParser;

    public HTTPTransport(String baseUrl, JsonParser jsonParser) {
        this.baseUrl = baseUrl.endsWith("/") ? baseUrl.substring(0, baseUrl.length() - 1) : baseUrl;
        this.jsonParser = jsonParser;
        this.httpClient = HttpClient.newBuilder()
            .connectTimeout(Duration.ofSeconds(10))
            .build();
    }

    @Override
    public Response call(Request request) throws Exception {
        String requestJson = jsonParser.toJson(request);

        HttpRequest httpRequest = HttpRequest.newBuilder()
            .uri(URI.create(baseUrl))
            .header("Content-Type", "application/json")
            .POST(HttpRequest.BodyPublishers.ofString(requestJson))
            .timeout(Duration.ofSeconds(30))
            .build();

        HttpResponse<String> httpResponse = httpClient.send(httpRequest, HttpResponse.BodyHandlers.ofString());

        if (httpResponse.statusCode() != 200) {
            throw new IOException("HTTP error: " + httpResponse.statusCode() + " - " + httpResponse.body());
        }

        Response response = jsonParser.fromJson(httpResponse.body(), Response.class);

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

