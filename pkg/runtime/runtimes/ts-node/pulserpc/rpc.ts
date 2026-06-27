/**
 * RPC error handling for JSON-RPC 2.0
 */

export class RPCError extends Error {
  public code: number;
  public data?: any;

  constructor(code: number, message: string, data?: any) {
    super(message);
    this.code = code;
    this.message = message;
    this.data = data;
    // Maintains proper stack trace for where our error was thrown (only available on V8)
    if (Error.captureStackTrace) {
      Error.captureStackTrace(this, RPCError);
    }
  }
}
