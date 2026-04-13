package pulserpc

import (
	"fmt"
)

func DiffIDL(clientIDL, serverIDL interface{}) []ContractDelta {
	var deltas []ContractDelta

	clientInterfaces := extractInterfaces(clientIDL)
	serverInterfaces := extractInterfaces(serverIDL)

	deltas = append(deltas, diffInterfaces(clientInterfaces, serverInterfaces)...)

	clientStructs := extractStructs(clientIDL)
	serverStructs := extractStructs(serverIDL)
	deltas = append(deltas, diffStructs(clientStructs, serverStructs)...)

	clientEnums := extractEnums(clientIDL)
	serverEnums := extractEnums(serverIDL)
	deltas = append(deltas, diffEnums(clientEnums, serverEnums)...)

	clientErrors := extractErrors(clientIDL)
	serverErrors := extractErrors(serverIDL)
	deltas = append(deltas, diffErrors(clientErrors, serverErrors)...)

	return deltas
}

type idlInterfaces map[string]interface{}
type idlStructs map[string]interface{}
type idlEnums map[string]interface{}
type idlErrors map[string]interface{}

func extractInterfaces(idl interface{}) idlInterfaces {
	result := make(idlInterfaces)
	if dict, ok := idl.(map[string]interface{}); ok {
		if interfaces, ok := dict["interfaces"].([]interface{}); ok {
			for _, ifaceData := range interfaces {
				if ifaceDict, ok := ifaceData.(map[string]interface{}); ok {
					if name, ok := ifaceDict["name"].(string); ok {
						result[name] = ifaceDict
					}
				}
			}
		}
	}
	return result
}

func extractStructs(idl interface{}) idlStructs {
	result := make(idlStructs)
	if dict, ok := idl.(map[string]interface{}); ok {
		if structs, ok := dict["structs"].([]interface{}); ok {
			for _, structData := range structs {
				if structDict, ok := structData.(map[string]interface{}); ok {
					if name, ok := structDict["name"].(string); ok {
						result[name] = structDict
					}
				}
			}
		}
	}
	return result
}

func extractEnums(idl interface{}) idlEnums {
	result := make(idlEnums)
	if dict, ok := idl.(map[string]interface{}); ok {
		if enums, ok := dict["enums"].([]interface{}); ok {
			for _, enumData := range enums {
				if enumDict, ok := enumData.(map[string]interface{}); ok {
					if name, ok := enumDict["name"].(string); ok {
						result[name] = enumDict
					}
				}
			}
		}
	}
	return result
}

func extractErrors(idl interface{}) idlErrors {
	result := make(idlErrors)
	if dict, ok := idl.(map[string]interface{}); ok {
		if errors, ok := dict["errors"].([]interface{}); ok {
			for _, errData := range errors {
				if errDict, ok := errData.(map[string]interface{}); ok {
					if name, ok := errDict["name"].(string); ok {
						result[name] = errDict
					}
				}
			}
		}
	}
	return result
}

func diffInterfaces(client, server idlInterfaces) []ContractDelta {
	var deltas []ContractDelta

	for name, clientIface := range client {
		if serverIface, ok := server[name]; ok {
			deltas = append(deltas, diffInterfaceMethods(name, clientIface, serverIface)...)
		} else {
			deltas = append(deltas, ContractDelta{
				EntityType:  EntityInterface,
				EntityName:  name,
				ChangeType:  ChangeRemoved,
				Direction:   DirectionClientHasMore,
				Severity:    ClassifySeverity(EntityInterface, ChangeRemoved, DirectionClientHasMore),
				Description: fmt.Sprintf("Interface '%s' exists in client but not in server", name),
			})
		}
	}

	for name := range server {
		if _, ok := client[name]; !ok {
			deltas = append(deltas, ContractDelta{
				EntityType:  EntityInterface,
				EntityName:  name,
				ChangeType:  ChangeAdded,
				Direction:   DirectionClientHasLess,
				Severity:    ClassifySeverity(EntityInterface, ChangeAdded, DirectionClientHasLess),
				Description: fmt.Sprintf("Interface '%s' exists in server but not in client", name),
			})
		}
	}

	return deltas
}

func diffInterfaceMethods(ifaceName string, clientIface, serverIface interface{}) []ContractDelta {
	var deltas []ContractDelta

	clientMethods := extractMethods(clientIface)
	serverMethods := extractMethods(serverIface)

	for name, clientMethod := range clientMethods {
		if serverMethod, ok := serverMethods[name]; ok {
			if !methodsEqual(clientMethod, serverMethod) {
				deltas = append(deltas, ContractDelta{
					EntityType:  EntityMethod,
					EntityName:  ifaceName,
					MemberName:  name,
					ChangeType:  ChangeModified,
					Direction:   DirectionMismatch,
					Severity:    ClassifySeverity(EntityMethod, ChangeModified, DirectionMismatch),
					Description: fmt.Sprintf("Method '%s' in interface '%s' has mismatched signatures", name, ifaceName),
				})
			}
		} else {
			deltas = append(deltas, ContractDelta{
				EntityType:  EntityMethod,
				EntityName:  ifaceName,
				MemberName:  name,
				ChangeType:  ChangeRemoved,
				Direction:   DirectionClientHasMore,
				Severity:    ClassifySeverity(EntityMethod, ChangeRemoved, DirectionClientHasMore),
				Description: fmt.Sprintf("Method '%s' in interface '%s' exists in client but not in server", name, ifaceName),
			})
		}
	}

	for name := range serverMethods {
		if _, ok := clientMethods[name]; !ok {
			deltas = append(deltas, ContractDelta{
				EntityType:  EntityMethod,
				EntityName:  ifaceName,
				MemberName:  name,
				ChangeType:  ChangeAdded,
				Direction:   DirectionClientHasLess,
				Severity:    ClassifySeverity(EntityMethod, ChangeAdded, DirectionClientHasLess),
				Description: fmt.Sprintf("Method '%s' in interface '%s' exists in server but not in client", name, ifaceName),
			})
		}
	}

	return deltas
}

func extractMethods(iface interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	if dict, ok := iface.(map[string]interface{}); ok {
		if methods, ok := dict["methods"].([]interface{}); ok {
			for _, method := range methods {
				if methodDict, ok := method.(map[string]interface{}); ok {
					if name, ok := methodDict["name"].(string); ok {
						result[name] = methodDict
					}
				}
			}
		}
	}
	return result
}

func methodsEqual(a, b interface{}) bool {
	aMap, aOk := a.(map[string]interface{})
	bMap, bOk := b.(map[string]interface{})
	if !aOk || !bOk {
		return a == b
	}

	aParams := aMap["parameters"]
	bParams := bMap["parameters"]
	if !mapsEqual(aParams, bParams) {
		return false
	}

	aReturn := aMap["returnType"]
	bReturn := bMap["returnType"]
	if !mapsEqual(aReturn, bReturn) {
		return false
	}

	return true
}

func mapsEqual(a, b interface{}) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	aMap, aOk := a.(map[string]interface{})
	bMap, bOk := b.(map[string]interface{})
	if !aOk || !bOk {
		return interfaceEqual(a, b)
	}
	if len(aMap) != len(bMap) {
		return false
	}
	for k, v := range aMap {
		if bv, ok := bMap[k]; !ok || !mapsEqual(v, bv) {
			return false
		}
	}
	return true
}

func interfaceEqual(a, b interface{}) bool {
	switch a.(type) {
	case []interface{}:
		bSlice, ok := b.([]interface{})
		if !ok {
			return false
		}
		aSlice := a.([]interface{})
		if len(aSlice) != len(bSlice) {
			return false
		}
		for i := range aSlice {
			if !interfaceEqual(aSlice[i], bSlice[i]) {
				return false
			}
		}
		return true
	case map[string]interface{}:
		return mapsEqual(a, b)
	case string, float64, int, bool:
		return a == b
	default:
		return a == b
	}
}

func diffStructs(client, server idlStructs) []ContractDelta {
	var deltas []ContractDelta

	for name, clientStruct := range client {
		if serverStruct, ok := server[name]; ok {
			deltas = append(deltas, diffStructFields(name, clientStruct, serverStruct)...)
		} else {
			deltas = append(deltas, ContractDelta{
				EntityType:  EntityStruct,
				EntityName:  name,
				ChangeType:  ChangeRemoved,
				Direction:   DirectionClientHasMore,
				Severity:    ClassifySeverity(EntityStruct, ChangeRemoved, DirectionClientHasMore),
				Description: fmt.Sprintf("Struct '%s' exists in client but not in server", name),
			})
		}
	}

	for name := range server {
		if _, ok := client[name]; !ok {
			deltas = append(deltas, ContractDelta{
				EntityType:  EntityStruct,
				EntityName:  name,
				ChangeType:  ChangeAdded,
				Direction:   DirectionClientHasLess,
				Severity:    ClassifySeverity(EntityStruct, ChangeAdded, DirectionClientHasLess),
				Description: fmt.Sprintf("Struct '%s' exists in server but not in client", name),
			})
		}
	}

	return deltas
}

func diffStructFields(structName string, clientStruct, serverStruct interface{}) []ContractDelta {
	var deltas []ContractDelta

	clientFields := extractFields(clientStruct)
	serverFields := extractFields(serverStruct)

	for name, clientField := range clientFields {
		if serverField, ok := serverFields[name]; ok {
			typeChanged, optionalityChanged, wasRequired, isRequired := fieldsEqualDetailed(clientField, serverField)
			if typeChanged {
				deltas = append(deltas, ContractDelta{
					EntityType:  EntityField,
					EntityName:  structName,
					MemberName:  name,
					ChangeType:  ChangeModified,
					Direction:   DirectionMismatch,
					Severity:    ClassifySeverity(EntityField, ChangeModified, DirectionMismatch),
					Description: fmt.Sprintf("Field '%s' in struct '%s' has changed type", name, structName),
				})
			} else if optionalityChanged {
				if wasRequired && !isRequired {
					deltas = append(deltas, ContractDelta{
						EntityType:  EntityField,
						EntityName:  structName,
						MemberName:  name,
						ChangeType:  ChangeModified,
						Direction:   DirectionClientHasLess,
						Severity:    ClassifySeverity(EntityField, ChangeModified, DirectionClientHasLess, "made_optional"),
						Description: fmt.Sprintf("Field '%s' in struct '%s' optionality changed from required to optional", name, structName),
					})
				} else if !wasRequired && isRequired {
					deltas = append(deltas, ContractDelta{
						EntityType:  EntityField,
						EntityName:  structName,
						MemberName:  name,
						ChangeType:  ChangeModified,
						Direction:   DirectionClientHasLess,
						Severity:    ClassifySeverity(EntityField, ChangeModified, DirectionClientHasLess, "made_required"),
						Description: fmt.Sprintf("Field '%s' in struct '%s' optionality changed from optional to required", name, structName),
					})
				}
			}
		} else {
			deltas = append(deltas, ContractDelta{
				EntityType:  EntityField,
				EntityName:  structName,
				MemberName:  name,
				ChangeType:  ChangeRemoved,
				Direction:   DirectionClientHasMore,
				Severity:    ClassifySeverity(EntityField, ChangeRemoved, DirectionClientHasMore),
				Description: fmt.Sprintf("Field '%s' in struct '%s' exists in client but not in server", name, structName),
			})
		}
	}

	for name, serverField := range serverFields {
		if _, ok := clientFields[name]; !ok {
			isRequired := false
			if opt, ok := serverField.(map[string]interface{}); ok {
				if required, ok := opt["optional"].(bool); ok && !required {
					isRequired = true
				}
			}
			severity := ClassifySeverity(EntityField, ChangeAdded, DirectionClientHasLess, func() string {
				if isRequired {
					return "required"
				}
				return "optional"
			}())
			deltas = append(deltas, ContractDelta{
				EntityType:  EntityField,
				EntityName:  structName,
				MemberName:  name,
				ChangeType:  ChangeAdded,
				Direction:   DirectionClientHasLess,
				Severity:    severity,
				Description: fmt.Sprintf("Field '%s' in struct '%s' exists in server but not in client", name, structName),
			})
		}
	}

	return deltas
}

func extractFields(structData interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	if dict, ok := structData.(map[string]interface{}); ok {
		if fields, ok := dict["fields"].([]interface{}); ok {
			for _, field := range fields {
				if fieldMap, ok := field.(map[string]interface{}); ok {
					if name, ok := fieldMap["name"].(string); ok {
						result[name] = fieldMap
					}
				}
			}
		}
	}
	return result
}

func fieldsEqual(a, b interface{}) bool {
	typeChanged, _, _, _ := fieldsEqualDetailed(a, b)
	return !typeChanged
}

func fieldsEqualDetailed(a, b interface{}) (typeChanged bool, optionalityChanged bool, wasRequired bool, isRequired bool) {
	if a == nil && b == nil {
		return false, false, false, false
	}
	aMap, aOk := a.(map[string]interface{})
	bMap, bOk := b.(map[string]interface{})
	if !aOk || !bOk {
		return a != b, false, false, false
	}
	aType := aMap["type"]
	bType := bMap["type"]
	typeChanged = !mapsEqual(aType, bType)

	aOptional := getFieldOptional(aMap)
	bOptional := getFieldOptional(bMap)
	wasRequired = !aOptional
	isRequired = !bOptional
	optionalityChanged = aOptional != bOptional

	return typeChanged, optionalityChanged, wasRequired, isRequired
}

func getFieldOptional(fieldMap map[string]interface{}) bool {
	if opt, ok := fieldMap["optional"].(bool); ok {
		return opt
	}
	return false
}

func diffEnums(client, server idlEnums) []ContractDelta {
	var deltas []ContractDelta

	for name, clientEnum := range client {
		if serverEnum, ok := server[name]; ok {
			deltas = append(deltas, diffEnumValues(name, clientEnum, serverEnum)...)
		} else {
			deltas = append(deltas, ContractDelta{
				EntityType:  EntityEnum,
				EntityName:  name,
				ChangeType:  ChangeRemoved,
				Direction:   DirectionClientHasMore,
				Severity:    ClassifySeverity(EntityEnum, ChangeRemoved, DirectionClientHasMore),
				Description: fmt.Sprintf("Enum '%s' exists in client but not in server", name),
			})
		}
	}

	for name := range server {
		if _, ok := client[name]; !ok {
			deltas = append(deltas, ContractDelta{
				EntityType:  EntityEnum,
				EntityName:  name,
				ChangeType:  ChangeAdded,
				Direction:   DirectionClientHasLess,
				Severity:    ClassifySeverity(EntityEnum, ChangeAdded, DirectionClientHasLess),
				Description: fmt.Sprintf("Enum '%s' exists in server but not in client", name),
			})
		}
	}

	return deltas
}

func diffEnumValues(enumName string, clientEnum, serverEnum interface{}) []ContractDelta {
	var deltas []ContractDelta

	clientValues := extractEnumValues(clientEnum)
	serverValues := extractEnumValues(serverEnum)

	for name := range clientValues {
		if _, ok := serverValues[name]; !ok {
			deltas = append(deltas, ContractDelta{
				EntityType:  EntityEnum,
				EntityName:  enumName,
				MemberName:  name,
				ChangeType:  ChangeRemoved,
				Direction:   DirectionClientHasMore,
				Severity:    ClassifySeverity(EntityEnum, ChangeRemoved, DirectionClientHasMore),
				Description: fmt.Sprintf("Enum value '%s' in enum '%s' exists in client but not in server", name, enumName),
			})
		}
	}

	for name := range serverValues {
		if _, ok := clientValues[name]; !ok {
			deltas = append(deltas, ContractDelta{
				EntityType:  EntityEnum,
				EntityName:  enumName,
				MemberName:  name,
				ChangeType:  ChangeAdded,
				Direction:   DirectionClientHasLess,
				Severity:    ClassifySeverity(EntityEnum, ChangeAdded, DirectionClientHasLess),
				Description: fmt.Sprintf("Enum value '%s' in enum '%s' exists in server but not in client", name, enumName),
			})
		}
	}

	return deltas
}

func extractEnumValues(enumData interface{}) map[string]bool {
	result := make(map[string]bool)
	if dict, ok := enumData.(map[string]interface{}); ok {
		if values, ok := dict["values"].([]interface{}); ok {
			for _, value := range values {
				if valueMap, ok := value.(map[string]interface{}); ok {
					if name, ok := valueMap["name"].(string); ok {
						result[name] = true
					}
				}
			}
		}
	}
	return result
}

func diffErrors(client, server idlErrors) []ContractDelta {
	var deltas []ContractDelta

	for name := range client {
		if _, ok := server[name]; !ok {
			deltas = append(deltas, ContractDelta{
				EntityType:  EntityError,
				EntityName:  name,
				ChangeType:  ChangeRemoved,
				Direction:   DirectionClientHasMore,
				Severity:    ClassifySeverity(EntityError, ChangeRemoved, DirectionClientHasMore),
				Description: fmt.Sprintf("Error '%s' exists in client but not in server", name),
			})
		}
	}

	for name := range server {
		if _, ok := client[name]; !ok {
			deltas = append(deltas, ContractDelta{
				EntityType:  EntityError,
				EntityName:  name,
				ChangeType:  ChangeAdded,
				Direction:   DirectionClientHasLess,
				Severity:    ClassifySeverity(EntityError, ChangeAdded, DirectionClientHasLess),
				Description: fmt.Sprintf("Error '%s' exists in server but not in client", name),
			})
		}
	}

	return deltas
}
