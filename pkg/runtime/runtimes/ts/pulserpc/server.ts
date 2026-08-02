import { RPCError } from "./rpc.js";
import { Contract } from "./contract.js";
import type { JsonRpcResponse } from "./transport.js";

export interface ServerOptions {
  contract: Contract;
  validateRequests?: boolean;
  validateResponses?: boolean;
}

export type HandlerCtx = Record<string, any>;
export type Handler = Record<string, (ctx: HandlerCtx, ...args: any[]) => any>;

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

  async call(req: any, ctx: HandlerCtx = {}): Promise<JsonRpcResponse | null> {
    if (typeof req !== "object" || req === null) {
      return this.errorResponse(
        req?.id ?? null,
        -32600,
        "Invalid Request",
        "Request must be an object"
      );
    }

    if (req.jsonrpc !== "2.0") {
      return this.errorResponse(
        req.id ?? null,
        -32600,
        "Invalid Request",
        "jsonrpc version must be '2.0'"
      );
    }

    const method = req.method;
    if (!method || typeof method !== "string") {
      return this.errorResponse(
        req.id ?? null,
        -32600,
        "Invalid Request",
        "Method must be a string"
      );
    }

    if (method === "pulserpc-idl") {
      return {
        jsonrpc: "2.0",
        result: this.contract.idlParsed,
        id: req.id ?? null,
      };
    }

    const reqId = req.id;
    const isNotification = !("id" in req);

    const dotIndex = method.lastIndexOf(".");
    if (dotIndex === -1) {
      if (isNotification) return null;
      return this.errorResponse(reqId, -32601, "Method not found", `Invalid method name format: ${method}`);
    }

    const ifaceName = method.substring(0, dotIndex);
    const funcName = method.substring(dotIndex + 1);

    if (!this.handlers.has(ifaceName)) {
      if (isNotification) return null;
      return this.errorResponse(reqId, -32601, "Method not found", `Unknown interface: ${ifaceName}`);
    }

    const handler = this.handlers.get(ifaceName)!;

    if (typeof handler[funcName] !== "function") {
      if (isNotification) return null;
      return this.errorResponse(reqId, -32601, "Method not found", `Unknown method: ${method}`);
    }

    const func = handler[funcName].bind(handler);

    let params = req.params;

    if (Array.isArray(params)) {
      if (this.validateRequests) {
        try {
          this.contract.validateRequest(ifaceName, funcName, params);
        } catch (e: any) {
          if (isNotification) return null;
          const code = e instanceof RPCError ? e.code : -32602;
          return this.errorResponse(reqId, code, e.message || "Invalid params", e.data);
        }
      }

      try {
        params = this.positionalToNamedParams(ifaceName, funcName, params);
      } catch (e: any) {
        if (isNotification) return null;
        return this.errorResponse(reqId, -32602, "Invalid params", e.message);
      }
    } else if (params === undefined || params === null) {
      params = {};
    } else if (this.validateRequests && typeof params === "object" && params !== null) {
      const positional = this.namedToPositionalParams(ifaceName, funcName, params);
      if (positional !== null) {
        try {
          this.contract.validateRequest(ifaceName, funcName, positional);
        } catch (e: any) {
          if (isNotification) return null;
          const code = e instanceof RPCError ? e.code : -32602;
          return this.errorResponse(reqId, code, e.message || "Invalid params", e.data);
        }
      }
    }

    try {
      const args = Object.values(params);
      const result = await func(ctx, ...args);
      if (isNotification) return null;
      return {
        jsonrpc: "2.0",
        result,
        id: reqId ?? null,
      };
    } catch (e: any) {
      if (isNotification) return null;
      if (e instanceof RPCError) {
        return this.errorResponse(reqId, e.code, e.message, e.data);
      } else {
        console.error(`Handler exception for ${method}:`, e);
        return this.errorResponse(reqId, -32603, "Internal error", `Handler exception: ${e?.message || String(e)}`);
      }
    }
  }

  errorResponse(reqId: any, code: number, message: string, data?: any): JsonRpcResponse {
    const error: { code: number; message: string; data?: any } = {
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

  positionalToNamedParams(ifaceName: string, funcName: string, positionalParams: any[]): Record<string, any> {
    const iface = this.contract.getInterface(ifaceName);
    if (!iface) {
      return positionalParams.reduce<Record<string, any>>((acc, v, i) => {
        acc[String(i)] = v;
        return acc;
      }, {});
    }

    const func = iface.getFunction(funcName);
    if (!func) {
      return positionalParams.reduce<Record<string, any>>((acc, v, i) => {
        acc[String(i)] = v;
        return acc;
      }, {});
    }

    const paramDefs = func.parameters || [];

    if (positionalParams.length !== paramDefs.length) {
      const requiredCount = paramDefs.filter((p: any) => !p.optional).length;
      if (
        positionalParams.length < requiredCount ||
        positionalParams.length > paramDefs.length
      ) {
        throw new Error(
          `Parameter count mismatch: expected ${paramDefs.length}, got ${positionalParams.length}`
        );
      }
    }

    const namedParams: Record<string, any> = {};
    for (let i = 0; i < positionalParams.length; i++) {
      if (i < paramDefs.length) {
        const paramName = paramDefs[i].name;
        namedParams[paramName] = positionalParams[i];
      } else {
        namedParams[String(i)] = positionalParams[i];
      }
    }

    return namedParams;
  }

  namedToPositionalParams(ifaceName: string, funcName: string, namedParams: Record<string, any>): any[] | null {
    const iface = this.contract.getInterface(ifaceName);
    if (!iface) {
      return null;
    }

    const func = iface.getFunction(funcName);
    if (!func) {
      return null;
    }

    const paramDefs = func.parameters || [];

    const positionalParams: any[] = [];
    for (const paramDef of paramDefs) {
      positionalParams.push(namedParams[paramDef.name]);
    }

    return positionalParams;
  }
}
