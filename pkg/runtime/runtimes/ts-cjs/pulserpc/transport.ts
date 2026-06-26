/**
 * Transport abstraction for JSON-RPC requests.
 *
 * Provides abstract Transport base class and concrete implementations:
 * - HttpTransport: HTTP-based transport using fetch
 * - InProcTransport: In-process transport for testing
 */

import type { JsonRpcRequest, JsonRpcResponse } from "./types";

/**
 * Abstract transport base class
 */
export class Transport {
  async request(_req: JsonRpcRequest): Promise<JsonRpcResponse> {
    throw new Error("Transport.request must be implemented by subclass");
  }

  close(): void {
    // Default implementation does nothing
  }
}

/**
 * HTTP transport implementation using fetch
 */
export class HttpTransport extends Transport {
  baseUrl: string;
  headers: Record<string, string>;
  fetchFn: typeof fetch;

  constructor(baseUrl: string, headers: Record<string, string> = {}, fetchFn: typeof fetch = fetch) {
    super();
    // The endpoint URL is used verbatim; do not strip or append anything so
    // callers can target a specific path (e.g. a reverse-proxied /api/rpc).
    this.baseUrl = baseUrl;
    this.headers = {
      "Content-Type": "application/json",
      ...headers,
    };
    this.fetchFn = fetchFn;
  }

  async request(req: JsonRpcRequest): Promise<JsonRpcResponse> {
    const url = this.baseUrl;

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
    } catch (e: any) {
      throw new Error(`Invalid JSON response: ${e.message}`);
    }
  }
}

/**
 * In-process transport for testing.
 *
 * This transport calls the server directly without HTTP.
 */
export class InProcTransport extends Transport {
  serverCallFn: (req: JsonRpcRequest) => JsonRpcResponse | undefined;

  constructor(serverCallFn: (req: JsonRpcRequest) => JsonRpcResponse | undefined) {
    super();
    this.serverCallFn = serverCallFn;
  }

  async request(req: JsonRpcRequest): Promise<JsonRpcResponse> {
    const response = this.serverCallFn(req);
    if (!response) {
      return {
        jsonrpc: "2.0",
        id: req.id ?? null,
      };
    }
    return response;
  }
}
