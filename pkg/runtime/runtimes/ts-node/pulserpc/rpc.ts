/**
 * RPC error handling for JSON-RPC 2.0
 */

/**
 * Standard JSON-RPC 2.0 error codes (RUNTIME_IMPLEMENTATION_GUIDE.md:815).
 * Transport/parse failures must use one of these, never a private -32000 code.
 */
export const RPCErrorCode = {
  ParseError: -32700,
  InvalidRequest: -32600,
  MethodNotFound: -32601,
  InvalidParams: -32602,
  InternalError: -32603,
} as const;

export class RPCError extends Error {
  public code: number;
  public data?: any;

  constructor(code: number, message: string, data?: any) {
    super(message);
    this.name = `RPCError ${code}`;
    this.code = code;
    this.data = data;
    if (Error.captureStackTrace) {
      Error.captureStackTrace(this, RPCError);
    }
  }
}
