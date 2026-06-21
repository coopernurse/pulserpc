/**
 * Contract class for IDL validation and interface metadata.
 *
 * This module provides the Contract class which parses IDL metadata
 * and provides validation for requests and responses.
 */

import type { StructMap, EnumMap, FieldDef } from "./types";
import { validateType } from "./validation";

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

  hasInterface(ifaceName: string): boolean {
    return this.interfaces.has(ifaceName);
  }

  getInterface(ifaceName: string): InterfaceImpl | undefined {
    return this.interfaces.get(ifaceName);
  }

  validateRequest(ifaceName: string, funcName: string, params: any[]): void {
    const iface = this.getInterface(ifaceName);
    if (!iface) {
      throw new Error(`Unknown interface: '${ifaceName}'`);
    }

    const func = iface.getFunction(funcName);
    if (!func) {
      throw new Error(`${ifaceName}: Unknown function: '${funcName}'`);
    }

    const paramDefs: FieldDef[] = func.parameters || [];

    if (params.length !== paramDefs.length) {
      throw new Error(
        `Function '${ifaceName}.${funcName}' expects ${paramDefs.length} param(s). ${params.length} given.`
      );
    }

    for (let i = 0; i < params.length; i++) {
      const paramValue = params[i];
      const paramDef = paramDefs[i];
      const paramName = paramDef.name;
      const paramType = paramDef.type;
      const isOptional = paramDef.optional || false;

      try {
        validateType(paramValue, paramType, this.structs, this.enums, isOptional);
      } catch (e: any) {
        throw new Error(
          `Function '${ifaceName}.${funcName}' invalid param '${paramName}'. ${e.message}`
        );
      }
    }
  }

  validateResponse(ifaceName: string, funcName: string, result: any): void {
    const iface = this.getInterface(ifaceName);
    if (!iface) {
      throw new Error(`Unknown interface: '${ifaceName}'`);
    }

    const func = iface.getFunction(funcName);
    if (!func) {
      throw new Error(`${ifaceName}: Unknown function: '${funcName}'`);
    }

    const returnType = func.returnType;
    if (!returnType) {
      if (result !== null && result !== undefined) {
        throw new Error(
          `Function '${ifaceName}.${funcName}' invalid response: '${JSON.stringify(result)}'. Expected null/undefined`
        );
      }
      return;
    }

    const isOptional = func.returnOptional || false;
    try {
      validateType(result, returnType, this.structs, this.enums, isOptional);
    } catch (e: any) {
      throw new Error(
        `Function '${ifaceName}.${funcName}' invalid response: '${JSON.stringify(result)}'. ${e.message}`
      );
    }
  }
}
