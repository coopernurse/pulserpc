/**
 * Client class for making JSON-RPC 2.0 requests
 *
 * Provides automatic interface discovery via pulserpc-idl and
 * dynamic interface proxies for convenient RPC calls.
 */

import { dirname, join } from "path";
import { existsSync, readFileSync } from "fs";
import { RPCError } from "./rpc";
import { Contract } from "./contract";
import { Transport } from "./transport";
import { diffIDL, extractChecksum } from "./diff";
import type { JsonRpcRequest, JsonRpcResponse, VerificationResult } from "./types";

/**
 * Proxy for an interface that provides callable methods.
 *
 * Created dynamically by Client for each interface in the IDL.
 */
class InterfaceClientProxy {
  client: Client;
  iface: any;
  ifaceName: string;

  constructor(client: Client, iface: any) {
    this.client = client;
    this.iface = iface;
    this.ifaceName = iface.name;

    for (const funcName of iface.functions.keys()) {
      (this as any)[funcName] = this.createMethodCaller(funcName);
    }
  }

  createMethodCaller(funcName: string): (...args: any[]) => Promise<any> {
    return (...args: any[]) => {
      return this.client.call(
        `${this.ifaceName}.${funcName}`,
        args.length > 0 ? args : undefined
      );
    };
  }
}

export interface ClientOptions {
  auditor?: any;
  verifyOnBootstrap?: boolean;
}

/**
 * JSON-RPC 2.0 client with automatic interface discovery.
 *
 * The Client class sends JSON-RPC requests via a Transport implementation.
 * On initialization, it fetches the IDL from the server and dynamically
 * creates interface proxies.
 */
export class Client {
  transport: Transport;
  validateRequest: boolean;
  validateResponse: boolean;
  contract: Contract | null = null;

  // Index signature to allow dynamic interface proxies
  [key: string]: any;

  private requestId: number = 0;
  private initialized: boolean = false;
  private initPromise: Promise<void> | null = null;
  private _auditor: any;
  private _verifyOnBootstrap: boolean = false;
  private _localIDL: Record<string, any> | null = null;

  constructor(
    transport: Transport,
    validateRequest: boolean = false,
    validateResponse: boolean = false,
    options?: ClientOptions
  ) {
    this.transport = transport;
    this.validateRequest = validateRequest;
    this.validateResponse = validateResponse;

    if (options) {
      this._auditor = options.auditor;
      this._verifyOnBootstrap = options.verifyOnBootstrap || false;
    }

    this._findIDLJson();

    this.initPromise = this.bootstrapWithVerification();
  }

  async ready(): Promise<void> {
    if (this.initialized) {
      return;
    }
    if (this.initPromise) {
      await this.initPromise;
      this.initialized = true;
    }
  }

  private async bootstrapWithVerification(): Promise<void> {
    await this.bootstrap();
    if (this._verifyOnBootstrap) {
      await this.verifyCompatibility();
    }
  }

  private async bootstrap(): Promise<void> {
    const req: JsonRpcRequest = {
      jsonrpc: "2.0",
      method: "pulserpc-idl",
      id: "bootstrap",
    };

    const resp = await this.transport.request(req);

    if (resp.error) {
      throw new Error(`Failed to fetch IDL from server: ${resp.error.message}`);
    }

    const idlJson = resp.result;
    if (!idlJson) {
      throw new Error("Server returned empty IDL");
    }

    this.contract = new Contract(idlJson);

    if (this.contract.interfaces) {
      for (const [ifaceName, iface] of this.contract.interfaces.entries()) {
        (this as any)[ifaceName] = new InterfaceClientProxy(this, iface);
      }
    }

    this.initialized = true;
  }

  _findIDLJson(): void {
    try {
      // __dirname is the CJS equivalent of import.meta.url
      let currentDir = __dirname;

      for (let i = 0; i < 10; i++) {
        const idlPath = join(currentDir, "idl.json");
        try {
          if (existsSync(idlPath)) {
            const content = readFileSync(idlPath, "utf-8");
            this._localIDL = JSON.parse(content);
            return;
          }
        } catch (e) {
          // File doesn't exist or can't be read, try parent directory
        }
        const parentDir = dirname(currentDir);
        if (parentDir === currentDir) break;
        currentDir = parentDir;
      }
    } catch (e) {
      // __dirname not available or other error - local IDL must be set explicitly
    }
  }

  setLocalIDL(idlJson: string): void {
    this._localIDL = JSON.parse(idlJson);
  }

  async verifyCompatibility(): Promise<VerificationResult> {
    if (!this.contract || !(this.contract as any).idlParsed) {
      throw new Error("No server IDL available - client not bootstrapped");
    }

    const serverIDL = (this.contract as any).idlParsed;
    const clientIDL = this._localIDL;

    if (!clientIDL) {
      throw new Error("No local IDL available - call setLocalIDL() first");
    }

    const deltas = diffIDL(clientIDL, serverIDL);
    const hasError = deltas.some((d: any) => d.severity === "Error");
    const compatible = !hasError;

    const serverChecksum = extractChecksum(serverIDL);
    const clientChecksum = extractChecksum(clientIDL);

    const result: VerificationResult = {
      compatible,
      serverChecksum,
      clientChecksum,
      deltas,
      timestamp: new Date(),
    };

    if (this._auditor) {
      this._auditor.audit(result);
    }

    return result;
  }

  async call(method: string, params?: any, expectResponse: boolean = true): Promise<any> {
    const dotIndex = method.lastIndexOf(".");
    if (dotIndex === -1) {
      throw new Error(`Invalid method name format: ${method}`);
    }

    const ifaceName = method.substring(0, dotIndex);
    const funcName = method.substring(dotIndex + 1);

    if (this.validateRequest && this.contract) {
      if (typeof params === "object" && params !== null && !Array.isArray(params)) {
        const paramList = this.namedToPositional(ifaceName, funcName, params);
        if (paramList !== null) {
          try {
            this.contract.validateRequest(ifaceName, funcName, paramList);
          } catch (e: any) {
            throw new Error(`Request validation failed: ${e.message}`);
          }
        }
      } else if (Array.isArray(params)) {
        try {
          this.contract.validateRequest(ifaceName, funcName, params);
        } catch (e: any) {
          throw new Error(`Request validation failed: ${e.message}`);
        }
      }
    }

    this.requestId++;
    const reqId = expectResponse ? this.requestId : null;

    const req: JsonRpcRequest = {
      jsonrpc: "2.0",
      method,
    };
    if (params !== undefined) {
      req.params = params;
    }
    if (reqId !== null) {
      req.id = reqId;
    }

    const response = await this.transport.request(req);

    if (!expectResponse) {
      return null;
    }

    if (response.error) {
      const error = response.error;
      throw new RPCError(
        error.code || -32603,
        error.message || "Unknown error",
        error.data
      );
    }

    const result = response.result;

    if (this.validateResponse && this.contract && result !== undefined && result !== null) {
      try {
        this.contract.validateResponse(ifaceName, funcName, result);
      } catch (e: any) {
        throw new Error(`Response validation failed: ${e.message}`);
      }
    }

    return result;
  }

  notify(method: string, params?: any): Promise<void> {
    return this.call(method, params, false) as any;
  }

  private namedToPositional(ifaceName: string, funcName: string, namedParams: Record<string, any>): any[] | null {
    if (!this.contract) {
      return null;
    }

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
