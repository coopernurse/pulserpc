/**
 * Contract class for IDL validation and interface metadata.
 *
 * This module provides the Contract class which parses IDL metadata
 * and provides validation for requests and responses.
 */

import * as fs from "fs";
import type { StructMap, EnumMap, FieldDef, ValidationError, ValidationResult } from "./types";
import { validateType } from "./validation";
import { RPCError } from "./rpc";

export interface ContractAuditor {
  audit(result: { compatible: boolean; deltas: any[] }): void;
  name(): string;
}

export class NoOpAuditor implements ContractAuditor {
  audit(_result: any): void {}
  name(): string {
    return "NoOp";
  }
}

export class LoggingAuditor implements ContractAuditor {
  audit(result: any): void {
    if (!result.compatible) {
      console.error(
        `Contract incompatibility detected: ${result.deltas.length} deltas found`
      );
    }
    for (const delta of result.deltas) {
      if (delta.severity === "Error") {
        console.error(`${delta.entityType}: ${delta.description}`);
      } else if (delta.severity === "Warning") {
        console.warn(`${delta.entityType}: ${delta.description}`);
      } else if (delta.severity === "Info") {
        console.info(`${delta.entityType}: ${delta.description}`);
      }
    }
    if (result.compatible && result.deltas.length === 0) {
      console.info(
        "Contract compatibility verified: client and server IDLs are identical"
      );
    }
  }
  name(): string {
    return "Logging";
  }
}

export class FailFastAuditor implements ContractAuditor {
  audit(result: any): void {
    if (!result.compatible) {
      const errorDeltas = result.deltas.filter((d: any) => d.severity === "Error");
      const messages = errorDeltas
        .map((d: any) => `${d.entityType}: ${d.description}`)
        .join("; ");
      throw new Error(`Contract compatibility verification failed: ${messages}`);
    }
  }
  name(): string {
    return "FailFast";
  }
}

/**
 * Represents an interface from the IDL
 */
export class InterfaceImpl {
  name: string;
  functions: Map<string, any>;

  constructor(ifaceData: any) {
    this.name = ifaceData.name;
    this.functions = new Map();
    for (const func of ifaceData.methods || []) {
      this.functions.set(func.name, func);
    }
  }

  getFunction(funcName: string): any {
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
    result.invalidFields = paths;
  }
  return result;
}

/**
 * Represents a parsed IDL contract.
 *
 * The Contract class parses IDL JSON and provides validation
 * for requests and responses based on the interface definitions.
 */
export class Contract {
  idlParsed: any;
  interfaces: Map<string, InterfaceImpl>;
  structs: StructMap;
  enums: EnumMap;

  constructor(idlParsed: any) {
    this.idlParsed = idlParsed;
    this.interfaces = new Map();
    this.structs = {};
    this.enums = {};

    if (typeof idlParsed === "object" && !Array.isArray(idlParsed)) {
      for (const ifaceData of idlParsed.interfaces || []) {
        this.interfaces.set(ifaceData.name, new InterfaceImpl(ifaceData));
      }

      for (const structData of idlParsed.structs || []) {
        this.structs[structData.name] = structData;
      }

      for (const enumData of idlParsed.enums || []) {
        this.enums[enumData.name] = enumData;
      }
    } else if (Array.isArray(idlParsed)) {
      for (const item of idlParsed) {
        const itemType = item.type;
        if (itemType === "struct") {
          this.structs[item.name] = item;
        } else if (itemType === "enum") {
          this.enums[item.name] = item;
        } else if (itemType === "interface") {
          this.interfaces.set(item.name, new InterfaceImpl(item));
        }
      }
    }
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

  getInterface(ifaceName: string): InterfaceImpl | undefined {
    return this.interfaces.get(ifaceName);
  }

  /**
   * Validate a value against a named type (struct or enum) from the IDL
   */
  validate(typeName: string, value: any): ValidationResult {
    const typeDef = { userDefined: typeName };
    const errors = validateType(value, typeDef, this.structs, this.enums);
    return buildValidationResult(errors);
  }

  validateRequest(ifaceName: string, funcName: string, params: any[]): void {
    const iface = this.getInterface(ifaceName);
    if (!iface) {
      throw new RPCError(-32602, `Unknown interface: '${ifaceName}'`);
    }

    const func = iface.getFunction(funcName);
    if (!func) {
      throw new RPCError(-32602, `${ifaceName}: Unknown function: '${funcName}'`);
    }

    const paramDefs: FieldDef[] = func.parameters || [];

    if (params.length !== paramDefs.length) {
      throw new RPCError(
        -32602,
        `Function '${ifaceName}.${funcName}' expects ${paramDefs.length} param(s). ${params.length} given.`
      );
    }

    const allErrors: ValidationError[] = [];
    for (let i = 0; i < params.length; i++) {
      const paramValue = params[i];
      const paramDef = paramDefs[i];
      const paramType = paramDef.type;
      const isOptional = paramDef.optional || false;

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

  validateResponse(ifaceName: string, funcName: string, result: any): void {
    const iface = this.getInterface(ifaceName);
    if (!iface) {
      throw new RPCError(-32603, `Unknown interface: '${ifaceName}'`);
    }

    const func = iface.getFunction(funcName);
    if (!func) {
      throw new RPCError(-32603, `${ifaceName}: Unknown function: '${funcName}'`);
    }

    const returnType = func.returnType;
    if (!returnType) {
      if (result !== null && result !== undefined) {
        throw new RPCError(
          -32603,
          `Function '${ifaceName}.${funcName}' invalid response: '${JSON.stringify(result)}'. Expected null/undefined`
        );
      }
      return;
    }

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
