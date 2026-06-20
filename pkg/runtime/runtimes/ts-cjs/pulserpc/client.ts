/**
 * Client class for making JSON-RPC 2.0 requests
 *
 * Provides automatic interface discovery via pulserpc-idl and
 * dynamic interface proxies for convenient RPC calls.
 */

const { dirname, join } = require("path");
const { existsSync, readFileSync } = require("fs");
const { RPCError } = require("./rpc");
const { Contract } = require("./contract");
const { Transport } = require("./transport");
const { diffIDL, extractChecksum } = require("./diff");

/**
 * Proxy for an interface that provides callable methods
 *
 * Created dynamically by Client for each interface in the IDL.
 */
class InterfaceClientProxy {
  constructor(client, iface) {
    this.client = client;
    this.iface = iface;
    this.ifaceName = iface.name;

    for (const funcName of iface.functions.keys()) {
      this[funcName] = this.createMethodCaller(funcName);
    }
  }

  createMethodCaller(funcName) {
    return (...args) => {
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
class Client {
  constructor(
    transport,
    validateRequest = false,
    validateResponse = false,
    options
  ) {
    this.transport = transport;
    this.validateRequest = validateRequest;
    this.validateResponse = validateResponse;

    if (options) {
      this._auditor = options.auditor;
      this._verifyOnBootstrap = options.verifyOnBootstrap || false;
    }

    this.contract = null;
    this.requestId = 0;
    this.initialized = false;
    this.initPromise = this.bootstrapWithVerification();
  }

  async ready() {
    if (this.initialized) {
      return;
    }
    if (this.initPromise) {
      await this.initPromise;
      this.initialized = true;
    }
  }

  async bootstrapWithVerification() {
    await this.bootstrap();
    if (this._verifyOnBootstrap) {
      await this.verifyCompatibility();
    }
  }

  async bootstrap() {
    const req = {
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
        this[ifaceName] = new InterfaceClientProxy(this, iface);
      }
    }

    this.initialized = true;
  }

  _findIDLJson() {
    try {
      // In CommonJS, __dirname is the directory of this file (equivalent to
      // dirname(fileURLToPath(import.meta.url)) in ESM).
      let currentDir = __dirname;

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
      // __dirname not available or other error - local IDL must be set explicitly
    }
  }

  setLocalIDL(idlJson) {
    this._localIDL = JSON.parse(idlJson);
  }

  async verifyCompatibility() {
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

    const result = {
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

  async call(method, params, expectResponse = true) {
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
          } catch (e) {
            throw new Error(`Request validation failed: ${e.message}`);
          }
        }
      } else if (Array.isArray(params)) {
        try {
          this.contract.validateRequest(ifaceName, funcName, params);
        } catch (e) {
          throw new Error(`Request validation failed: ${e.message}`);
        }
      }
    }

    this.requestId++;
    const reqId = expectResponse ? this.requestId : null;

    const req = {
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
      } catch (e) {
        throw new Error(`Response validation failed: ${e.message}`);
      }
    }

    return result;
  }

  notify(method, params) {
    return this.call(method, params, false);
  }

  namedToPositional(ifaceName, funcName, namedParams) {
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

    const positionalParams = [];
    for (const paramDef of paramDefs) {
      positionalParams.push(namedParams[paramDef.name]);
    }

    return positionalParams;
  }
}

module.exports = {
  Client,
};
