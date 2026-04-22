/**
 * RPC error handling for JSON-RPC 2.0
 */

export class RPCError extends Error {
  public code: number;
  public data?: any;

  constructor(code: number, message: string, data?: any) {
    super(`RPCError ${code}: ${message}`);
    this.code = code;
    // Don't override message - it's already set by super() with the formatted string
    this.data = data;
    // Maintains proper stack trace for where our error was thrown (only available on V8)
    if (Error.captureStackTrace) {
      Error.captureStackTrace(this, RPCError);
    }
  }
}
