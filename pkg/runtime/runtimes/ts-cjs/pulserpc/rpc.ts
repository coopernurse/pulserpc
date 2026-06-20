/**
 * RPC error handling for JSON-RPC 2.0
 */

class RPCError extends Error {
  constructor(code, message, data) {
    super(`RPCError ${code}: ${message}`);
    this.code = code;
    this.data = data;
    if (Error.captureStackTrace) {
      Error.captureStackTrace(this, RPCError);
    }
  }
}

module.exports.RPCError = RPCError;
