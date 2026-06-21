/**
 * Client class for making JSON-RPC 2.0 requests
 *
 * Provides automatic interface discovery via pulserpc-idl and
 * dynamic interface proxies for convenient RPC calls.
 */

import { fileURLToPath } from "url";
import { dirname, join } from "path";
import { existsSync, readFileSync } from "fs";
import { RPCError } from "./rpc.js";
import { Contract, Interface, ContractAuditor } from "./contract.js";
import { Transport, JsonRpcRequest } from "./transport.js";
import { diffIDL, extractChecksum } from "./diff.js";
import { ContractDelta, VerificationResult } from "./types.js";

export interface ClientOptions {
  auditor?: ContractAuditor;
  verifyOnBootstrap?: boolean;
}

/**
 * Proxy for an interface that provides callable methods
 *
 * Created dynamically by Client for each interface in the IDL.
 */
class InterfaceClientProxy {
  private client: Client;
  private iface: Interface;
  private ifaceName: string;

  constructor(client: Client, iface: Interface) {
    this.client = client;
    this.iface = iface;
    this.ifaceName = iface.name;

    // Create callable methods for each function in the interface
    for (const funcName of iface.functions.keys()) {
      (this as any)[funcName] = this.createMethodCaller(funcName);
    }
  }

  private createMethodCaller(funcName: string): (...args: any[]) => Promise<any> {
    return (...args: any[]) => {
      // Send as positional params - this ensures struct parameters are properly
      // wrapped as array elements rather than flat named params
      // For example: repeat(req1: RepeatRequest) expects [{to_repeat: 'hello'}]
      // not {to_repeat: 'hello'} (which would incorrectly flatten the struct)
      return this.client.call(
        `${this.ifaceName}.${funcName}`,
        args.length > 0 ? args : undefined
      );
    };
  }
}

/**
 * JSON-RPC 2.0 client with automatic interface discovery
 *
 * The Client class sends JSON-RPC requests via a Transport implementation.
 * On initialization, it fetches the IDL from the server and dynamically
 * creates interface proxies.
 *
 * Example:
 *   const transport = new HttpTransport("http://localhost:8080");
 *   const client = new Client(transport);
 *   const result = await client.UserService.getUser({ userId: "123" });
 */
export class Client {
  transport: Transport;
  validateRequest: boolean;
  validateResponse: boolean;
  contract: Contract | null = null;

  // Index signature to allow dynamic interface proxies
  // TypeScript doesn't know about these at compile time since they're
  // added dynamically via bootstrap(), but they exist at runtime
  [key: string]: any;

  private requestId: number = 0;
  private initialized: boolean = false;
  private initPromise: Promise<void> | null = null;
  private _auditor: ContractAuditor | undefined;
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

    // Bootstrap: fetch IDL from server asynchronously
    this._findIDLJson();
    this.initPromise = this.bootstrapWithVerification();
  }

  /**
   * Returns a promise that resolves when the client is fully initialized.
   * Call this after creating a Client and before making RPC calls.
   */
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

  /**
   * Fetch IDL from server and create interface proxies
   *
   * Makes a 'pulserpc-idl' request to get the IDL JSON,
   * then creates Contract and interface proxies.
   */
  private async bootstrap(): Promise<void> {
    const req: JsonRpcRequest = {
      jsonrpc: "2.0",
      method: "pulserpc-idl",
      id: "bootstrap",
    };

    const resp = await this.transport.request(req);

    if (resp.error) {
      throw new Error(
        `Failed to fetch IDL from server: ${resp.error.message}`
      );
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

  private _findIDLJson(): void {
    try {
      const currentFile = fileURLToPath(import.meta.url);
      let currentDir = dirname(currentFile);

      for (let i = 0; i < 10; i++) {
        const idlPath = join(currentDir, 'idl.json');
        try {
          if (existsSync(idlPath)) {
            const content = readFileSync(idlPath, 'utf-8');
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
      // import.meta.url not available or other error - local IDL must be set explicitly
    }
  }

  setLocalIDL(idlJson: string): void {
    this._localIDL = JSON.parse(idlJson);
  }

  async verifyCompatibility(): Promise<VerificationResult> {
    if (!this.contract || !this.contract.idlParsed) {
      throw new Error("No server IDL available - client not bootstrapped");
    }

    const serverIDL = this.contract.idlParsed;
    const clientIDL = this._localIDL;

    if (!clientIDL) {
      throw new Error("No local IDL available - call setLocalIDL() first");
    }

    const deltas = diffIDL(clientIDL, serverIDL);
    const hasError = deltas.some(d => d.severity === "Error");
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

  /**
   * Make a JSON-RPC call
   *
   * @param method Method name (e.g., "UserService.getUser")
   * @param params Parameters (dict for named params, list for positional)
   * @param expectResponse If false, send as notification (no response expected)
   * @returns Result value from the server
   */
  async call(
    method: string,
    params?: any,
    expectResponse: boolean = true
  ): Promise<any> {
    // Parse method name
    const dotIndex = method.lastIndexOf(".");
    if (dotIndex === -1) {
      throw new Error(`Invalid method name format: ${method}`);
    }

    const ifaceName = method.substring(0, dotIndex);
    const funcName = method.substring(dotIndex + 1);

    // Validate request if enabled
    if (this.validateRequest && this.contract) {
      // Convert params to list for validation
      if (typeof params === "object" && params !== null && !Array.isArray(params)) {
        // Named params - need to convert to positional
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

    // Generate request ID
    this.requestId++;
    const reqId = expectResponse ? this.requestId : null;

    // Build request
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

    // Send request via transport
    const response = await this.transport.request(req);

    // Handle notification (no response expected)
    if (!expectResponse) {
      return null;
    }

    // Check for error response
    if (response.error) {
      const error = response.error;
      throw new RPCError(
        error.code || -32603,
        error.message || "Unknown error",
        error.data
      );
    }

    // Get result
    const result = response.result;

    // Validate response if enabled
    if (this.validateResponse && this.contract && result !== undefined && result !== null) {
      try {
        this.contract.validateResponse(ifaceName, funcName, result);
      } catch (e: any) {
        throw new Error(`Response validation failed: ${e.message}`);
      }
    }

    return result;
  }

  /**
   * Send a JSON-RPC notification (no response expected)
   */
  notify(method: string, params?: any): Promise<void> {
    return this.call(method, params, false) as any;
  }

  /**
   * Convert named parameters to positional parameters using IDL signature
   */
  private namedToPositional(
    ifaceName: string,
    funcName: string,
    namedParams: Record<string, any>
  ): any[] | null {
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
