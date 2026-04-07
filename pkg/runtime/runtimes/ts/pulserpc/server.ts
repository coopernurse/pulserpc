/**
 * Server class for handling JSON-RPC 2.0 requests
 *
 * Transport-independent server that processes JSON-RPC requests
 * and dispatches to registered handlers.
 */

import { RPCError } from "./rpc.js";
import { Contract } from "./contract.js";
import { JsonRpcRequest, JsonRpcResponse } from "./transport.js";

interface ServerOptions {
  contract: Contract;
  validateRequests?: boolean;
  validateResponses?: boolean;
}

interface HandlerMethod {
  (...args: any[]): any;
}

/**
 * JSON-RPC 2.0 server with handler registration and optional validation
 *
 * The Server class provides transport-independent request processing.
 * It can be used with any HTTP server (Node http, Express, FastAPI, etc.)
 * by feeding it parsed JSON-RPC requests.
 */
export class Server {
  private handlers: Map<string, HandlerMethod> = new Map();
  contract: Contract;
  validateRequests: boolean;
  validateResponses: boolean;

  constructor(options: ServerOptions) {
    this.contract = options.contract;
    this.validateRequests = options.validateRequests ?? true;
    this.validateResponses = options.validateResponses ?? true;
  }

  /**
   * Register a handler instance for an interface
   */
  addHandler(ifaceName: string, handler: any): void {
    this.handlers.set(ifaceName, handler);
  }

  /**
   * Process a single JSON-RPC request
   *
   * @param req JSON-RPC request dict with 'jsonrpc', 'method', 'params', 'id'
   * @returns JSON-RPC response dict, or null for notification (requests without 'id')
   */
  call(req: JsonRpcRequest): JsonRpcResponse | null {
    // Validate request format
    if (typeof req !== "object" || req === null) {
      return this.errorResponse(
        (req as any)?.id ?? null,
        -32600,
        "Invalid Request",
        "Request must be an object"
      );
    }

    // Check JSON-RPC version
    if ((req as any).jsonrpc !== "2.0") {
      return this.errorResponse(
        (req as any).id ?? null,
        -32600,
        "Invalid Request",
        "jsonrpc version must be '2.0'"
      );
    }

    // Check for method
    const method = (req as any).method;
    if (!method || typeof method !== "string") {
      return this.errorResponse(
        (req as any).id ?? null,
        -32600,
        "Invalid Request",
        "Method must be a string"
      );
    }

    // Handle pulserpc-idl request
    if (method === "pulserpc-idl") {
      return {
        jsonrpc: "2.0",
        result: this.contract.idlParsed,
        id: (req as any).id ?? null,
      };
    }

    // Check for notification (no 'id' means no response expected)
    const reqId = (req as any).id;
    const isNotification = reqId === undefined || reqId === null;

    // Parse method name (e.g., "UserService.getUser")
    const dotIndex = method.lastIndexOf(".");
    if (dotIndex === -1) {
      return this.errorResponse(reqId, -32601, "Method not found", `Invalid method name format: ${method}`);
    }

    const ifaceName = method.substring(0, dotIndex);
    const funcName = method.substring(dotIndex + 1);

    // Look up handler
    if (!this.handlers.has(ifaceName)) {
      return this.errorResponse(reqId, -32601, "Method not found", `Unknown interface: ${ifaceName}`);
    }

    const handler = this.handlers.get(ifaceName) as any;

    // Get function from handler
    if (typeof handler[funcName] !== "function") {
      return this.errorResponse(reqId, -32601, "Method not found", `Unknown method: ${method}`);
    }

    const func = handler[funcName].bind(handler);

    // Get params
    let params = (req as any).params;

    // Normalize params to dict if it's a list
    if (Array.isArray(params)) {
      // Validate request using positional params
      if (this.validateRequests) {
        try {
          this.contract.validateRequest(ifaceName, funcName, params);
        } catch (e: any) {
          return this.errorResponse(reqId, -32602, "Invalid params", e.message);
        }
      }

      // Convert positional params to named params using IDL signature
      try {
        params = this.positionalToNamedParams(ifaceName, funcName, params);
      } catch (e: any) {
        return this.errorResponse(reqId, -32602, "Invalid params", e.message);
      }
    } else if (params === undefined || params === null) {
      params = {};
    }

    if (typeof params !== "object" || Array.isArray(params)) {
      return this.errorResponse(reqId, -32602, "Invalid params", "Parameters must be an object or array");
    }

    // Validate request if using named params (dict)
    if (this.validateRequests && !Array.isArray((req as any).params)) {
      // Convert dict to list for validation
      const paramList = this.namedToPositionalParams(ifaceName, funcName, params);
      if (paramList !== null) {
        try {
          this.contract.validateRequest(ifaceName, funcName, paramList);
        } catch (e: any) {
          return this.errorResponse(reqId, -32602, "Invalid params", e.message);
        }
      }
    }

    // Invoke handler method
    try {
      // Call handler function with positional params (spread the object values)
      const args = Object.values(params);
      const result = func(...args);
      return {
        jsonrpc: "2.0",
        result,
        id: reqId ?? null,
      };
    } catch (e: any) {
      // Convert application errors to RPC errors
      if (e instanceof RPCError) {
        return this.errorResponse(reqId, e.code, e.message, e.data);
      } else {
        // Log unexpected errors and return internal error
        console.error(`Handler exception for ${method}:`, e);
        return this.errorResponse(reqId, -32603, "Internal error", `Handler exception: ${e.message || String(e)}`);
      }
    }
  }

  /**
   * Create a JSON-RPC error response
   */
  private errorResponse(
    reqId: string | number | null,
    code: number,
    message: string,
    data?: string
  ): JsonRpcResponse {
    const error: any = {
      code,
      message,
    };
    if (data !== undefined) {
      error.data = data;
    }

    const response: JsonRpcResponse = {
      jsonrpc: "2.0",
      error,
    };
    if (reqId !== null && reqId !== undefined) {
      response.id = reqId;
    }

    return response;
  }

  /**
   * Convert positional parameters to named parameters using IDL signature
   */
  private positionalToNamedParams(
    ifaceName: string,
    funcName: string,
    positionalParams: any[]
  ): Record<string, any> {
    const iface = this.contract.getInterface(ifaceName);
    if (!iface) {
      // Without contract, can't map positional to named
      return positionalParams.reduce((acc: Record<string, any>, v: any, i: number) => {
        acc[String(i)] = v;
        return acc;
      }, {});
    }

    const func = iface.getFunction(funcName);
    if (!func) {
      return positionalParams.reduce((acc: Record<string, any>, v: any, i: number) => {
        acc[String(i)] = v;
        return acc;
      }, {});
    }

    // Get parameter names from IDL
    const paramDefs = func.parameters || [];

    // Check parameter count
    if (positionalParams.length !== paramDefs.length) {
      // Allow fewer params if trailing ones are optional
      const requiredCount = paramDefs.filter((p: { optional?: boolean }) => !p.optional).length;
      if (
        positionalParams.length < requiredCount ||
        positionalParams.length > paramDefs.length
      ) {
        throw new Error(
          `Parameter count mismatch: expected ${paramDefs.length}, got ${positionalParams.length}`
        );
      }
    }

    // Map positional params to names
    const namedParams: Record<string, any> = {};
    for (let i = 0; i < positionalParams.length; i++) {
      if (i < paramDefs.length) {
        const paramName = paramDefs[i].name;
        namedParams[paramName] = positionalParams[i];
      } else {
        // Fallback for extra params
        namedParams[String(i)] = positionalParams[i];
      }
    }

    return namedParams;
  }

  /**
   * Convert named parameters to positional parameters using IDL signature
   */
  private namedToPositionalParams(
    ifaceName: string,
    funcName: string,
    namedParams: Record<string, any>
  ): any[] | null {
    const iface = this.contract.getInterface(ifaceName);
    if (!iface) {
      return null;
    }

    const func = iface.getFunction(funcName);
    if (!func) {
      return null;
    }

    // Get parameter names from IDL
    const paramDefs = func.parameters || [];

    // Build positional list in IDL order
    const positionalParams: any[] = [];
    for (const paramDef of paramDefs) {
      positionalParams.push(namedParams[paramDef.name]);
    }

    return positionalParams;
  }
}
