/**
 * RPC error handling for JSON-RPC 2.0
 */

export class RPCError extends Error {
  code: number;
  data?: any;

  constructor(code: number, message: string, data?: any) {
    super(message);
    this.name = `RPCError ${code}`;
    this.code = code;
    this.data = data;
    if (typeof (Error as any).captureStackTrace === "function") {
      (Error as any).captureStackTrace(this, RPCError);
    }
  }
}
