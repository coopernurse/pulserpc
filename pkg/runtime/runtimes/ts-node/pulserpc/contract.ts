/**
 * Contract class for IDL validation and interface metadata
 *
 * This module provides the Contract class which parses IDL metadata
 * and provides validation for requests and responses.
 */

import * as fs from "fs";
import { TypeDef, StructMap, EnumMap, VerificationResult, ContractDelta, Severity, ValidationError, ValidationResult } from "./types.js";
import { validateType } from "./validation.js";
import { RPCError } from "./rpc.js";

export type { VerificationResult, ContractDelta };

/**
 * Keys that must never be copied out of untrusted IDL metadata. `key` comes
 * from a server's `pulserpc-idl` response, so assigning it with `obj[key] = v`
 * would invoke Object.prototype's inherited `__proto__` setter and poison the
 * prototype of the metadata object.
 */
const UNSAFE_META_KEYS = new Set(["__proto__", "constructor", "prototype"]);

export interface ContractAuditor {
  audit(result: VerificationResult): void;
  name(): string;
}

export class NoOpAuditor implements ContractAuditor {
  audit(_result: VerificationResult): void {}
  name(): string {
    return "NoOp";
  }
}

export class LoggingAuditor implements ContractAuditor {
  audit(result: VerificationResult): void {
    if (!result.compatible) {
      console.error(`Contract incompatibility detected: ${result.deltas.length} deltas found`);
    }
    for (const delta of result.deltas) {
      if (delta.severity === Severity.Error) {
        console.error(`${delta.entityType}: ${delta.description}`);
      } else if (delta.severity === Severity.Warning) {
        console.warn(`${delta.entityType}: ${delta.description}`);
      } else if (delta.severity === Severity.Info) {
        console.info(`${delta.entityType}: ${delta.description}`);
      }
    }
    if (result.compatible && result.deltas.length === 0) {
      console.info("Contract compatibility verified: client and server IDLs are identical");
    }
  }
  name(): string {
    return "Logging";
  }
}

export class FailFastAuditor implements ContractAuditor {
  audit(result: VerificationResult): void {
    if (!result.compatible) {
      const errorDeltas = result.deltas.filter((d) => d.severity === Severity.Error);
      const messages = errorDeltas.map((d) => `${d.entityType}: ${d.description}`).join("; ");
      throw new Error(`Contract compatibility verification failed: ${messages}`);
    }
  }
  name(): string {
    return "FailFast";
  }
}

export interface FunctionDef {
  name: string;
  parameters: Array<{
    name: string;
    type: TypeDef;
    optional?: boolean;
  }>;
  returnType?: TypeDef;
  returnOptional?: boolean;
}

export interface Interface {
  name: string;
  functions: Map<string, FunctionDef>;
  getFunction(funcName: string): FunctionDef | undefined;
}

export interface JsonRpcIdlFormat {
  interfaces: Array<{
    name: string;
    methods: FunctionDef[];
  }>;
  structs: Array<{
    name: string;
    extends?: string;
    fields: Array<{
      name: string;
      type: TypeDef;
      optional?: boolean;
    }>;
  }>;
  enums: Array<{
    name: string;
    values: Array<{ name: string }>;
  }>;
}

/**
 * Represents an interface from the IDL
 */
class InterfaceImpl implements Interface {
  name: string;
  functions: Map<string, FunctionDef>;

  constructor(ifaceData: { name: string; methods: FunctionDef[] }) {
    this.name = ifaceData.name;
    this.functions = new Map();
    for (const func of ifaceData.methods || []) {
      this.functions.set(func.name, func);
    }
  }

  getFunction(funcName: string): FunctionDef | undefined {
    return this.functions.get(funcName);
  }
}

function buildValidationResult(errors: ValidationError[]): ValidationResult {
  if (errors.length === 0) {
    return { valid: true };
  }
  const result: ValidationResult = {
    valid: false,
    error: errors.map(e => e.path ? `${e.path}: ${e.message}` : e.message).join("; "),
  };
  const paths = errors.map(e => e.path).filter(Boolean);
  if (paths.length > 0) {
    result.invalid_fields = paths;
  }
  return result;
}

/**
 * Represents a parsed IDL contract
 *
 * The Contract class parses IDL JSON and provides validation
 * for requests and responses based on the interface definitions.
 */
export class Contract {
  idlParsed: any;
  interfaces: Map<string, Interface> = new Map();
  structs: StructMap = {};
  enums: EnumMap = {};
  /** Metadata declared by `meta` items, keyed by item field name (R12). */
  meta: Record<string, any> = {};

  constructor(idlParsed: any) {
    this.idlParsed = idlParsed;

    if (typeof idlParsed === 'object' && !Array.isArray(idlParsed)) {
      // PulseRPC format - dict with interfaces, structs, enums keys
      // Parse interfaces
      for (const ifaceData of idlParsed.interfaces || []) {
        this.interfaces.set(ifaceData.name, new InterfaceImpl(ifaceData));
      }

      // Parse structs
      for (const structData of idlParsed.structs || []) {
        this.structs[structData.name] = structData;
      }

      // Parse enums
      for (const enumData of idlParsed.enums || []) {
        this.enums[enumData.name] = enumData;
      }

      // Parse metadata (R12): the dict form may carry `meta` directly or as
      // a list of `{type: "meta", ...}` items, like the Barrister form.
      const rawMeta = idlParsed.meta;
      if (Array.isArray(rawMeta)) {
        for (const item of rawMeta) {
          this.addMetaItem(item);
        }
      } else if (rawMeta && typeof rawMeta === 'object') {
        // Do not use Object.assign here: it copies remote-controlled keys
        // straight onto `this.meta`, including an inherited `__proto__` setter.
        for (const [key, value] of Object.entries(rawMeta)) {
          this.setMetaItem(key, value);
        }
      }
    } else if (Array.isArray(idlParsed)) {
      // Barrister format - list of items with type field
      for (const item of idlParsed) {
        const itemType = item.type;
        if (itemType === 'struct') {
          this.structs[item.name] = item;
        } else if (itemType === 'enum') {
          this.enums[item.name] = item;
        } else if (itemType === 'interface') {
          this.interfaces.set(item.name, new InterfaceImpl(item));
        } else if (itemType === 'meta') {
          this.addMetaItem(item);
        }
      }
    }
  }

  private addMetaItem(item: any): void {
    if (!item || typeof item !== 'object') {
      return;
    }
    for (const [key, value] of Object.entries(item)) {
      if (key !== 'type') {
        this.setMetaItem(key, value);
      }
    }
  }

  /**
   * Store one metadata entry, rejecting prototype-polluting keys. Both the
   * Barrister item form and the dict form funnel through here so neither can
   * assign `__proto__`, `constructor` or `prototype` onto `this.meta`.
   */
  private setMetaItem(key: string, value: any): void {
    if (UNSAFE_META_KEYS.has(key)) {
      return;
    }
    this.meta[key] = value;
  }

  /**
   * Load a Contract from a JSON file path
   */
  static fromFile(path: string): Contract {
    const content = fs.readFileSync(path, "utf-8");
    return new Contract(JSON.parse(content));
  }

  hasInterface(ifaceName: string): boolean {
    return this.interfaces.has(ifaceName);
  }

  getInterface(ifaceName: string): Interface | undefined {
    return this.interfaces.get(ifaceName);
  }

  /**
   * Validate a value against a named type (struct or enum) from the IDL
   *
   * Returns a ValidationResult with valid=true on success, or valid=false
   * with error details and invalid field selectors on failure.
   *
   * Example:
   *   const result = contract.validate("Person", { username: "alice" });
   *   // { valid: true }
   *
   *   const result = contract.validate("Person", {});
   *   // { valid: false, error: ".username: Missing required field...", invalid_fields: [".username"] }
   */
  validate(typeName: string, value: any): ValidationResult {
    const typeDef: TypeDef = { userDefined: typeName };
    const errors = validateType(value, typeDef, this.structs, this.enums);
    return buildValidationResult(errors);
  }

  /**
   * Single owner of the positional parameter-count rule (M9): trailing
   * optional parameters may be omitted, so any length between `requiredCount`
   * and the full signature length is accepted.
   *
   * Exposed so `Server` can keep the rule when request validation is disabled.
   */
  checkParamCount(
    ifaceName: string,
    funcName: string,
    paramDefs: FunctionDef["parameters"],
    givenCount: number
  ): void {
    const requiredCount = paramDefs.filter((p) => !p.optional).length;
    if (givenCount < requiredCount || givenCount > paramDefs.length) {
      throw new RPCError(
        -32602,
        `Function '${ifaceName}.${funcName}' expects ${requiredCount}-${paramDefs.length} param(s). ${givenCount} given.`
      );
    }
  }

  /**
   * Validate request parameters against IDL
   */
  validateRequest(ifaceName: string, funcName: string, params: any[]): void {
    const iface = this.getInterface(ifaceName);
    if (!iface) {
      throw new RPCError(-32602, `Unknown interface: '${ifaceName}'`);
    }

    const func = iface.getFunction(funcName);
    if (!func) {
      throw new RPCError(-32602, `${ifaceName}: Unknown function: '${funcName}'`);
    }

    const paramDefs = func.parameters || [];

    // Trailing optional parameters may be omitted: accept any length between
    // the number of required parameters and the full signature (M9).
    this.checkParamCount(ifaceName, funcName, paramDefs, params.length);

    // Validate each parameter and collect errors
    const allErrors: ValidationError[] = [];
    for (let i = 0; i < params.length; i++) {
      const paramValue = params[i];
      const paramDef = paramDefs[i];
      const paramType = paramDef.type;
      const isOptional = paramDef.optional || false;

      // An absent value at an optional position (e.g. a named-param request
      // that omits a trailing optional field) is not a validation error.
      if (paramValue === undefined && isOptional) {
        continue;
      }

      const errors = validateType(paramValue, paramType, this.structs, this.enums, isOptional);
      for (const err of errors) {
        allErrors.push({
          path: err.path ? `param[${i}]${err.path}` : `param[${i}]`,
          message: err.message,
        });
      }
    }

    if (allErrors.length > 0) {
      throw new RPCError(
        -32602,
        `Function '${ifaceName}.${funcName}' invalid params`,
        buildValidationResult(allErrors)
      );
    }
  }

  /**
   * Validate response result against IDL
   */
  validateResponse(ifaceName: string, funcName: string, result: any): void {
    const iface = this.getInterface(ifaceName);
    if (!iface) {
      throw new RPCError(-32603, `Unknown interface: '${ifaceName}'`);
    }

    const func = iface.getFunction(funcName);
    if (!func) {
      throw new RPCError(-32603, `${ifaceName}: Unknown function: '${funcName}'`);
    }

    // Check if function has a return type
    const returnType = func.returnType;
    if (!returnType) {
      // Function returns void/None. Null is the canonical JSON-RPC value
      // across runtimes; `undefined` is not a valid wire value (R14).
      if (result !== null) {
        throw new RPCError(
          -32603,
          `Function '${ifaceName}.${funcName}' invalid response: '${JSON.stringify(result)}'. Expected null`
        );
      }
      return;
    }

    // Validate return type
    const isOptional = func.returnOptional || false;
    const errors = validateType(result, returnType, this.structs, this.enums, isOptional);
    if (errors.length > 0) {
      throw new RPCError(
        -32603,
        `Function '${ifaceName}.${funcName}' invalid response`,
        buildValidationResult(errors)
      );
    }
  }
}
