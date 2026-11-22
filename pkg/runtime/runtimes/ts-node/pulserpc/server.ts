/**
 * Server class for handling JSON-RPC 2.0 requests
 *
 * Transport-independent server that processes JSON-RPC requests
 * and dispatches to registered handlers.
 *
 * Wire invariants implemented here (see docs/audit/invariants-additional.md):
 * - R15/D4: internal errors never leak handler/validation detail. The response
 *   is always `-32603 "Internal error"` with no `data`; the underlying
 *   exception is logged server-side only.
 * - R24/D2: `id` must be a string, number, or null. Booleans, objects and
 *   arrays are rejected with `-32600`.
 * - M20: object-form (named) params are mapped to the IDL signature order and
 *   unknown keys are rejected explicitly with `-32602`.
 * - M21: a request with `id: null` is treated as a notification.
 * - M22: `error.data` is omitted when there is no data.
 */

import { RPCError } from "./rpc.js";
import { Contract } from "./contract.js";
import { JsonRpcRequest, JsonRpcResponse } from "./transport.js";

interface ServerOptions {
  contract: Contract;
  validateRequests?: boolean;
  validateResponses?: boolean;
}

// Service implementations are usually class instances. TypeScript does not
// give classes a string index signature, so a strict Record<...> here rejects
// the documented `addHandler(name, new ServiceImpl())` form even though it is
// valid at runtime.
type HandlerMethod = { [methodName: string]: any };

const INTERNAL_ERROR = "Internal error";

export class Server {
  handlers: Map<string, HandlerMethod> = new Map();
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
  addHandler(ifaceName: string, handler: HandlerMethod): void {
    this.handlers.set(ifaceName, handler);
  }

  /**
   * Process a single JSON-RPC request
   *
   * A request with no `id` member, or with `id: null`, is a notification: the
   * handler still executes, but no response is produced (R11 / M4 / M21).
   */
  async call(req: JsonRpcRequest, ctx: any = {}): Promise<JsonRpcResponse | null> {
    const isNotification =
      typeof req === "object" &&
      req !== null &&
      !Array.isArray(req) &&
      ((req as any).id === undefined || (req as any).id === null);

    const response = await this.callImpl(req, ctx);
    return isNotification ? null : response;
  }

  private async callImpl(req: any, ctx: any): Promise<JsonRpcResponse> {
    // Validate request format
    if (typeof req !== "object" || req === null || Array.isArray(req)) {
      return this.errorResponse(
        null,
        -32600,
        "Invalid Request",
        "Request must be an object"
      );
    }

    // R24: id must be a string or number. The JSON-RPC envelope only permits
    // null when the id could not be determined, so a boolean/object/array id
    // is invalid. Response id is null because the client-supplied id is not a
    // legal id value.
    const reqId = req.id;
    if (
      reqId !== undefined &&
      reqId !== null &&
      typeof reqId !== "string" &&
      typeof reqId !== "number"
    ) {
      return this.errorResponse(
        null,
        -32600,
        "Invalid Request",
        "id must be a string, number, or null"
      );
    }

    // Check JSON-RPC version. An invalid/unknown version makes the request
    // envelope itself invalid, so the id cannot be trusted and must be null
    // (notifications remain silent via `call` above).
    if (req.jsonrpc !== "2.0") {
      return this.errorResponse(
        null,
        -32600,
        "Invalid Request",
        "jsonrpc version must be '2.0'"
      );
    }

    // Check for method. An invalid method makes the request envelope invalid,
    // so the id cannot be trusted and must be null (notifications are silent
    // via `call` above).
    const method = req.method;
    if (!method || typeof method !== "string") {
      return this.errorResponse(
        null,
        -32600,
        "Invalid Request",
        "Method must be a string"
      );
    }

    // Handle pulserpc-idl request. The result is the parsed IDL object (C5),
    // never a double-encoded JSON string.
    if (method === "pulserpc-idl") {
      return {
        jsonrpc: "2.0",
        result: this.contract.idlParsed,
        id: reqId ?? null,
      };
    }

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

    const handler = this.handlers.get(ifaceName) as HandlerMethod;

    // Get function from handler
    if (typeof handler[funcName] !== "function") {
      return this.errorResponse(reqId, -32601, "Method not found", `Unknown method: ${method}`);
    }

    const func = handler[funcName].bind(handler);
    const paramDefs = this.contract.getInterface(ifaceName)?.getFunction(funcName)?.parameters;

    // Normalize params to positional arguments in IDL order.
    let positional: any[];
    const rawParams = req.params;

    if (Array.isArray(rawParams)) {
      positional = rawParams;
    } else if (rawParams === undefined || rawParams === null) {
      positional = [];
    } else if (typeof rawParams === "object") {
      // M20: object-form params are accepted, but mapped explicitly by IDL
      // parameter name rather than by the object's key order. Unknown keys are
      // rejected instead of being silently dropped (and shifting arguments).
      if (!paramDefs) {
        // No contract metadata: fall back to insertion order.
        positional = Object.values(rawParams);
      } else {
        const allowed = new Set(paramDefs.map((p) => p.name));
        for (const key of Object.keys(rawParams)) {
          if (!allowed.has(key)) {
            return this.errorResponse(
              reqId,
              -32602,
              "Invalid params",
              `Unknown parameter '${key}' for ${method}`
            );
          }
        }
        positional = paramDefs.map((p) => (rawParams as Record<string, any>)[p.name]);
        // Trailing optional parameters omitted from the object become
        // `undefined`; drop them so the count rule accepts a short signature.
        while (positional.length > 0 && positional[positional.length - 1] === undefined) {
          positional.pop();
        }
      }
    } else {
      return this.errorResponse(reqId, -32602, "Invalid params", "Parameters must be an object or array");
    }

    // Validate the request. When validation is disabled the parameter-count
    // rule still runs through the Contract (the single owner of that rule), so
    // a bad count fails fast instead of reaching the handler mis-mapped (M9).
    if (this.validateRequests) {
      try {
        this.contract.validateRequest(ifaceName, funcName, positional);
      } catch (e: any) {
        const code = e instanceof RPCError ? e.code : -32602;
        return this.errorResponse(reqId, code, e.message || "Invalid params", e.data);
      }
    } else if (paramDefs) {
      try {
        this.contract.checkParamCount(ifaceName, funcName, paramDefs, positional.length);
      } catch (e: any) {
        return this.errorResponse(reqId, -32602, "Invalid params", e.message);
      }
    }

    // Invoke handler method
    try {
      const result = await func(ctx, ...positional);

      // Validate the handler result against the IDL when enabled (R10).
      // A failed response validation is a server-side defect: log it and
      // redact the detail from the client (R15 / D4).
      if (this.validateResponses) {
        try {
          this.contract.validateResponse(ifaceName, funcName, result);
        } catch (e: any) {
          console.error(`Response validation failed for ${method}:`, e);
          return this.errorResponse(reqId, -32603, INTERNAL_ERROR);
        }
      }

      return {
        jsonrpc: "2.0",
        result,
        id: reqId ?? null,
      };
    } catch (e: any) {
      // Convert declared application errors to RPC errors, preserving their
      // code/message/data. Anything else is an internal error whose detail is
      // logged server-side and never sent to the client (R15 / D4).
      if (e instanceof RPCError) {
        return this.errorResponse(reqId, e.code, e.message, e.data);
      }
      console.error(`Handler exception for ${method}:`, e);
      return this.errorResponse(reqId, -32603, INTERNAL_ERROR);
    }
  }

  /**
   * Create a JSON-RPC error response.
   *
   * `data` is included only when it is not `undefined` (M22), and `id` is
   * always present (null when the request id is unknown).
   */
  private errorResponse(
    reqId: string | number | null | undefined,
    code: number,
    message: string,
    data?: any
  ): JsonRpcResponse {
    const error: any = {
      code,
      message,
    };
    if (data !== undefined) {
      error.data = data;
    }

    return {
      jsonrpc: "2.0",
      error,
      id: reqId ?? null,
    };
  }
}
