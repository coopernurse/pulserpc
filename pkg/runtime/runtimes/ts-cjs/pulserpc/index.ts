/**
 * PulseRPC TypeScript Runtime Library (CommonJS variant).
 *
 * This library provides validation and RPC functionality for PulseRPC-generated code.
 */

export { RPCError } from "./rpc";
export {
  Contract,
  NoOpAuditor,
  LoggingAuditor,
  FailFastAuditor,
  InterfaceImpl,
  ContractAuditor,
} from "./contract";
export { Client, ClientOptions } from "./client";
export { Server, ServerOptions, HandlerCtx, Handler } from "./server";
export { Transport, HttpTransport, InProcTransport } from "./transport";
export { validateType } from "./validation";
export { diffIDL, classifySeverity } from "./diff";
export {
  EntityType,
  ChangeType,
  Direction,
  Severity,
  ContractDelta,
  VerificationResult,
  JsonRpcRequest,
  JsonRpcResponse,
  JsonRpcError,
  FieldDef,
  StructDef,
  EnumDef,
  StructMap,
  EnumMap,
  findStruct,
  findEnum,
  getStructFields,
  extractChecksum,
  ValidationError,
  ValidationResult,
} from "./types";
