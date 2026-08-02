import { RPCError } from "./rpc.js";

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

export abstract class Transport {
  abstract request(_req: JsonRpcRequest): Promise<JsonRpcResponse>;

  close(): void {
  }
}

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
      throw new RPCError(-32000, `HTTP ${response.status}: ${response.statusText}`, { status: response.status });
    }

    const text = await response.text();
    if (!text) {
      throw new RPCError(-32000, "Empty response body");
    }

    try {
      return JSON.parse(text) as JsonRpcResponse;
    } catch (e: any) {
      throw new RPCError(-32000, `Invalid JSON response: ${e.message}`);
    }
  }
}

export class InProcTransport extends Transport {
  private serverCallFn: (req: JsonRpcRequest) => Promise<JsonRpcResponse | null> | JsonRpcResponse | null;

  constructor(serverCallFn: (req: JsonRpcRequest) => Promise<JsonRpcResponse | null> | JsonRpcResponse | null) {
    super();
    this.serverCallFn = serverCallFn;
  }

  async request(req: JsonRpcRequest): Promise<JsonRpcResponse> {
    const response = await this.serverCallFn(req);
    if (!response) {
      return {
        jsonrpc: "2.0",
        id: req.id || null,
      };
    }
    return response;
  }
}
