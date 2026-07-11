/**
 * RPC error handling for JSON-RPC 2.0
 */

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
