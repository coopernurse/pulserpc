/**
 * Server class for handling JSON-RPC 2.0 requests.
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

import { RPCError } from "./rpc";
import { Contract } from "./contract";
import type { JsonRpcRequest, JsonRpcResponse } from "./types";

export interface ServerOptions {
  contract: Contract;
  validateRequests?: boolean;
  validateResponses?: boolean;
}

export type HandlerCtx = Record<string, any>;
export type Handler = Record<string, (ctx: HandlerCtx, ...args: any[]) => any>;

const INTERNAL_ERROR = "Internal error";

export class Server {
  handlers: Map<string, Handler>;
  contract: Contract;
  validateRequests: boolean;
  validateResponses: boolean;

  constructor(options: ServerOptions) {
    this.handlers = new Map();
    this.contract = options.contract;
    this.validateRequests = options.validateRequests ?? true;
    this.validateResponses = options.validateResponses ?? true;
  }

  addHandler(ifaceName: string, handler: Handler): void {
    this.handlers.set(ifaceName, handler);
  }

  /**
   * Process a JSON-RPC request. Notifications (requests without an `id`, or
   * with `id: null`) are executed but produce no response at all.
   */
  async call(req: JsonRpcRequest, ctx: HandlerCtx = {}): Promise<JsonRpcResponse | null> {
    const isNotification =
      typeof req === "object" &&
      req !== null &&
      !Array.isArray(req) &&
      ((req as any).id === undefined || (req as any).id === null);

    const response = await this.callImpl(req, ctx);
    return isNotification ? null : response;
  }

  private async callImpl(req: any, ctx: HandlerCtx): Promise<JsonRpcResponse> {
    if (typeof req !== "object" || req === null || Array.isArray(req)) {
      return this.errorResponse(
        null,
        -32600,
        "Invalid Request",
        "Request must be an object"
      );
    }

    // R24: id must be a string or number (null means "id unknown").
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

    if (req.jsonrpc !== "2.0") {
      return this.errorResponse(
        null,
        -32600,
        "Invalid Request",
        "jsonrpc version must be '2.0'"
      );
    }

    const method = req.method;
    if (!method || typeof method !== "string") {
      // Invalid method => invalid envelope: reply with a null id. Notifications
      // are suppressed by `call` above.
      return this.errorResponse(
        null,
        -32600,
        "Invalid Request",
        "Method must be a string"
      );
    }

    if (method === "pulserpc-idl") {
      return {
        jsonrpc: "2.0",
        result: this.contract.idlParsed,
        id: reqId ?? null,
      };
    }

    const dotIndex = method.lastIndexOf(".");
    if (dotIndex === -1) {
      return this.errorResponse(reqId, -32601, "Method not found", `Invalid method name format: ${method}`);
    }

    const ifaceName = method.substring(0, dotIndex);
    const funcName = method.substring(dotIndex + 1);

    if (!this.handlers.has(ifaceName)) {
      return this.errorResponse(reqId, -32601, "Method not found", `Unknown interface: ${ifaceName}`);
    }

    const handler = this.handlers.get(ifaceName)!;

    if (typeof handler[funcName] !== "function") {
      return this.errorResponse(reqId, -32601, "Method not found", `Unknown method: ${method}`);
    }

    const func = handler[funcName].bind(handler);
    const paramDefs = this.contract.getInterface(ifaceName)?.getFunction(funcName)?.parameters;

    let positional: any[];
    const rawParams = req.params;

    if (Array.isArray(rawParams)) {
      positional = rawParams;
    } else if (rawParams === undefined || rawParams === null) {
      positional = [];
    } else if (typeof rawParams === "object") {
      // M20: object-form params are accepted, mapped by IDL parameter name in
      // signature order. Unknown keys are rejected rather than silently
      // dropped or used to shift argument positions.
      if (!paramDefs) {
        positional = Object.values(rawParams);
      } else {
        const allowed = new Set(paramDefs.map((p: any) => p.name));
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
        positional = paramDefs.map((p: any) => (rawParams as Record<string, any>)[p.name]);
        while (positional.length > 0 && positional[positional.length - 1] === undefined) {
          positional.pop();
        }
      }
    } else {
      return this.errorResponse(reqId, -32602, "Invalid params", "Parameters must be an object or array");
    }

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

    try {
      const result = await func(ctx, ...positional);

      if (this.validateResponses) {
        try {
          this.contract.validateResponse(ifaceName, funcName, result);
        } catch (e: any) {
          // R15/D4: response-validation detail is a server-side concern.
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
      if (e instanceof RPCError) {
        return this.errorResponse(reqId, e.code, e.message, e.data);
      }
      // R15/D4: log the exception, return the fixed internal-error message.
      console.error(`Handler exception for ${method}:`, e);
      return this.errorResponse(reqId, -32603, INTERNAL_ERROR);
    }
  }

  errorResponse(
    reqId: string | number | null | undefined,
    code: number,
    message: string,
    data?: any
  ): JsonRpcResponse {
    const error: { code: number; message: string; data?: any } = {
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
