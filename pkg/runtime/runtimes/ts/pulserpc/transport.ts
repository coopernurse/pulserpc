/**
 * Transport abstraction for JSON-RPC requests
 *
 * Provides abstract Transport base class and concrete implementations:
 * - HttpTransport: HTTP-based transport using fetch
 * - InProcTransport: In-process transport for testing
 */

export interface JsonRpcRequest {
  jsonrpc: "2.0";
  method: string;
  params?: any;
  id?: string | number | null;
}

export interface JsonRpcResponse {
  jsonrpc: "2.0";
  result?: any;
  error?: {
    code: number;
    message: string;
    data?: any;
  };
  id?: string | number | null;
}

/**
 * Abstract transport base class
 */
export abstract class Transport {
  /**
   * Send a JSON-RPC request and return the response
   */
  abstract request(req: JsonRpcRequest): Promise<JsonRpcResponse>;

  /**
   * Close the transport and release any resources
   */
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

  constructor(
    baseUrl: string,
    headers: Record<string, string> = {},
    fetchFn: typeof fetch = fetch
  ) {
    super();
    this.baseUrl = baseUrl.endsWith("/") ? baseUrl.slice(0, -1) : baseUrl;
    this.headers = {
      "Content-Type": "application/json",
      ...headers,
    };
    this.fetchFn = fetchFn;
  }

  async request(req: JsonRpcRequest): Promise<JsonRpcResponse> {
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
      return JSON.parse(text) as JsonRpcResponse;
    } catch (e: any) {
      throw new Error(`Invalid JSON response: ${e.message}`);
    }
  }
}

/**
 * In-process transport for testing
 *
 * This transport calls the server directly without HTTP.
 */
export class InProcTransport extends Transport {
  private serverCallFn: (req: JsonRpcRequest) => JsonRpcResponse | null;

  constructor(serverCallFn: (req: JsonRpcRequest) => JsonRpcResponse | null) {
    super();
    this.serverCallFn = serverCallFn;
  }

  async request(req: JsonRpcRequest): Promise<JsonRpcResponse> {
    const response = this.serverCallFn(req);
    if (!response) {
      // Notification - should not happen in normal usage
      return {
        jsonrpc: "2.0",
        id: req.id || null,
      };
    }
    return response;
  }
}
