/**
 * Contract class for IDL validation and interface metadata
 *
 * This module provides the Contract class which parses IDL metadata
 * and provides validation for requests and responses.
 */

import { TypeDef, StructMap, EnumMap, findStruct, findEnum } from "./types.js";
import { validateType } from "./validation.js";

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
        }
      }
    }
  }

  hasInterface(ifaceName: string): boolean {
    return this.interfaces.has(ifaceName);
  }

  getInterface(ifaceName: string): Interface | undefined {
    return this.interfaces.get(ifaceName);
  }

  /**
   * Validate request parameters against IDL
   */
  validateRequest(ifaceName: string, funcName: string, params: any[]): void {
    const iface = this.getInterface(ifaceName);
    if (!iface) {
      throw new Error(`Unknown interface: '${ifaceName}'`);
    }

    const func = iface.getFunction(funcName);
    if (!func) {
      throw new Error(`${ifaceName}: Unknown function: '${funcName}'`);
    }

    const paramDefs = func.parameters || [];

    // Check parameter count
    if (params.length !== paramDefs.length) {
      throw new Error(
        `Function '${ifaceName}.${funcName}' expects ${paramDefs.length} param(s). ${params.length} given.`
      );
    }

    // Validate each parameter
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

  /**
   * Validate response result against IDL
   */
  validateResponse(ifaceName: string, funcName: string, result: any): void {
    const iface = this.getInterface(ifaceName);
    if (!iface) {
      throw new Error(`Unknown interface: '${ifaceName}'`);
    }

    const func = iface.getFunction(funcName);
    if (!func) {
      throw new Error(`${ifaceName}: Unknown function: '${funcName}'`);
    }

    // Check if function has a return type
    const returnType = func.returnType;
    if (!returnType) {
      // Function returns void/None
      if (result !== null && result !== undefined) {
        throw new Error(
          `Function '${ifaceName}.${funcName}' invalid response: '${JSON.stringify(result)}'. Expected null/undefined`
        );
      }
      return;
    }

    // Validate return type
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
