/**
 * Transport abstraction for JSON-RPC requests
 *
 * Provides abstract Transport base class and concrete implementations:
 * - HttpTransport: HTTP-based transport using fetch
 * - InProcTransport: In-process transport for testing
 */

/**
 * Abstract transport base class
 */
class Transport {
  async request(_req) {
    throw new Error("Transport.request must be implemented by subclass");
  }

  close() {
    // Default implementation does nothing
  }
}

/**
 * HTTP transport implementation using fetch
 */
class HttpTransport extends Transport {
  constructor(baseUrl, headers = {}, fetchFn = fetch) {
    super();
    this.baseUrl = baseUrl.endsWith("/") ? baseUrl.slice(0, -1) : baseUrl;
    this.headers = {
      "Content-Type": "application/json",
      ...headers,
    };
    this.fetchFn = fetchFn;
  }

  async request(req) {
    const url = `${this.baseUrl}/`;

    const response = await this.fetchFn(url, {
      method: "POST",
      headers: this.headers,
      body: JSON.stringify(req),
    });

    if (!response.ok) {
      throw new Error(`HTTP error: ${response.status} ${response.statusText}`);
    }

    const text = await response.text();
    if (!text) {
      throw new Error("Empty response body");
    }

    try {
      return JSON.parse(text);
    } catch (e) {
      throw new Error(`Invalid JSON response: ${e.message}`);
    }
  }
}

/**
 * In-process transport for testing
 *
 * This transport calls the server directly without HTTP.
 */
class InProcTransport extends Transport {
  constructor(serverCallFn) {
    super();
    this.serverCallFn = serverCallFn;
  }

  async request(req) {
    const response = this.serverCallFn(req);
    if (!response) {
      return {
        jsonrpc: "2.0",
        id: req.id || null,
      };
    }
    return response;
  }
}

module.exports = {
  Transport,
  HttpTransport,
  InProcTransport,
};
