/**
 * PulseRPC TypeScript Runtime Library (CommonJS variant)
 *
 * This library provides validation and RPC functionality for PulseRPC-generated code.
 */

const { RPCError } = require("./rpc");
const { Contract, NoOpAuditor, LoggingAuditor, FailFastAuditor } = require("./contract");
const { Client } = require("./client");
const { Server } = require("./server");
const { Transport, HttpTransport, InProcTransport } = require("./transport");
const types = require("./types");
const validation = require("./validation");
const { diffIDL, classifySeverity, extractChecksum } = require("./diff");

module.exports = {
  RPCError,
  Contract,
  NoOpAuditor,
  LoggingAuditor,
  FailFastAuditor,
  Client,
  Server,
  Transport,
  HttpTransport,
  InProcTransport,
  diffIDL,
  classifySeverity,
  extractChecksum,
  ...types,
  ...validation,
};
