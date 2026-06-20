/**
 * PulseRPC TypeScript Runtime Library
 *
 * This library provides validation and RPC functionality for PulseRPC-generated code.
 */

export { RPCError } from "./rpc.js";
export { Contract } from "./contract.js";
export type { Interface, FunctionDef, VerificationResult, ContractAuditor } from "./contract.js";
export { Client } from "./client.js";
export type { ClientOptions } from "./client.js";
export { Server } from "./server.js";
export { Transport, HttpTransport, InProcTransport } from "./transport.js";
export type { JsonRpcRequest, JsonRpcResponse } from "./transport.js";
export * from "./types.js";
export * from "./validation.js";
export { diffIDL, classifySeverity, extractChecksum } from "./diff.js";
export { NoOpAuditor, LoggingAuditor, FailFastAuditor } from "./contract.js";
