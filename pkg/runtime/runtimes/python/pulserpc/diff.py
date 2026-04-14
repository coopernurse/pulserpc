"""IDL diff functionality for contract verification"""

from typing import Any, Dict, List, Tuple
from .types import (
    ContractDelta, EntityType, ChangeType, Direction, Severity
)


def diff_idl(client_idl: Dict[str, Any], server_idl: Dict[str, Any]) -> List[ContractDelta]:
    """Compute a directional structural diff between client and server IDL"""
    deltas: List[ContractDelta] = []

    client_interfaces = _extract_interfaces(client_idl)
    server_interfaces = _extract_interfaces(server_idl)
    deltas.extend(_diff_interfaces(client_interfaces, server_interfaces))

    client_structs = _extract_structs(client_idl)
    server_structs = _extract_structs(server_idl)
    deltas.extend(_diff_structs(client_structs, server_structs))

    client_enums = _extract_enums(client_idl)
    server_enums = _extract_enums(server_idl)
    deltas.extend(_diff_enums(client_enums, server_enums))

    client_errors = _extract_errors(client_idl)
    server_errors = _extract_errors(server_idl)
    deltas.extend(_diff_errors(client_errors, server_errors))

    return deltas


def _extract_interfaces(idl: Dict[str, Any]) -> Dict[str, Any]:
    result = {}
    for iface_data in idl.get('interfaces', []):
        name = iface_data.get('name')
        if name:
            result[name] = iface_data
    return result


def _extract_structs(idl: Dict[str, Any]) -> Dict[str, Any]:
    result = {}
    for struct_data in idl.get('structs', []):
        name = struct_data.get('name')
        if name:
            result[name] = struct_data
    return result


def _extract_enums(idl: Dict[str, Any]) -> Dict[str, Any]:
    result = {}
    for enum_data in idl.get('enums', []):
        name = enum_data.get('name')
        if name:
            result[name] = enum_data
    return result


def _extract_errors(idl: Dict[str, Any]) -> Dict[str, Any]:
    result = {}
    for err_data in idl.get('errors', []):
        name = err_data.get('name')
        if name:
            result[name] = err_data
    return result


def _diff_interfaces(client: Dict[str, Any], server: Dict[str, Any]) -> List[ContractDelta]:
    deltas: List[ContractDelta] = []

    for name, client_iface in client.items():
        if name in server:
            deltas.extend(_diff_interface_methods(name, client_iface, server[name]))
        else:
            deltas.append(ContractDelta(
                entity_type=EntityType.INTERFACE,
                entity_name=name,
                member_name='',
                change_type=ChangeType.REMOVED,
                direction=Direction.CLIENT_HAS_MORE,
                severity=Severity.ERROR,
                description=f"Interface '{name}' exists in client but not in server"
            ))

    for name in server:
        if name not in client:
            deltas.append(ContractDelta(
                entity_type=EntityType.INTERFACE,
                entity_name=name,
                member_name='',
                change_type=ChangeType.ADDED,
                direction=Direction.CLIENT_HAS_LESS,
                severity=Severity.INFO,
                description=f"Interface '{name}' exists in server but not in client"
            ))

    return deltas


def _diff_interface_methods(iface_name: str, client_iface: Dict[str, Any], server_iface: Dict[str, Any]) -> List[ContractDelta]:
    deltas: List[ContractDelta] = []
    client_methods = _extract_methods(client_iface)
    server_methods = _extract_methods(server_iface)

    for name, client_method in client_methods.items():
        if name in server_methods:
            if not _methods_equal(client_method, server_methods[name]):
                deltas.append(ContractDelta(
                    entity_type=EntityType.METHOD,
                    entity_name=iface_name,
                    member_name=name,
                    change_type=ChangeType.MODIFIED,
                    direction=Direction.MISMATCH,
                    severity=Severity.ERROR,
                    description=f"Method '{name}' in interface '{iface_name}' has mismatched signatures"
                ))
        else:
            deltas.append(ContractDelta(
                entity_type=EntityType.METHOD,
                entity_name=iface_name,
                member_name=name,
                change_type=ChangeType.REMOVED,
                direction=Direction.CLIENT_HAS_MORE,
                severity=Severity.ERROR,
                description=f"Method '{name}' in interface '{iface_name}' exists in client but not in server"
            ))

    for name in server_methods:
        if name not in client_methods:
            deltas.append(ContractDelta(
                entity_type=EntityType.METHOD,
                entity_name=iface_name,
                member_name=name,
                change_type=ChangeType.ADDED,
                direction=Direction.CLIENT_HAS_LESS,
                severity=Severity.WARNING,
                description=f"Method '{name}' in interface '{iface_name}' exists in server but not in client"
            ))

    return deltas


def _extract_methods(iface: Dict[str, Any]) -> Dict[str, Any]:
    result = {}
    for method in iface.get('methods', []):
        name = method.get('name')
        if name:
            result[name] = method
    return result


def _methods_equal(a: Dict[str, Any], b: Dict[str, Any]) -> bool:
    if not _maps_equal(a.get('parameters'), b.get('parameters')):
        return False
    if not _maps_equal(a.get('returnType'), b.get('returnType')):
        return False
    return True


def _maps_equal(a: Any, b: Any) -> bool:
    if a is None and b is None:
        return True
    if a is None or b is None:
        return False
    if isinstance(a, dict) and isinstance(b, dict):
        if len(a) != len(b):
            return False
        for k, v in a.items():
            if k not in b or not _maps_equal(v, b[k]):
                return False
        return True
    if isinstance(a, list) and isinstance(b, list):
        if len(a) != len(b):
            return False
        for i in range(len(a)):
            if not _maps_equal(a[i], b[i]):
                return False
        return True
    return a == b


def _diff_structs(client: Dict[str, Any], server: Dict[str, Any]) -> List[ContractDelta]:
    deltas: List[ContractDelta] = []

    for name, client_struct in client.items():
        if name in server:
            deltas.extend(_diff_struct_fields(name, client_struct, server[name]))
        else:
            deltas.append(ContractDelta(
                entity_type=EntityType.STRUCT,
                entity_name=name,
                member_name='',
                change_type=ChangeType.REMOVED,
                direction=Direction.CLIENT_HAS_MORE,
                severity=Severity.ERROR,
                description=f"Struct '{name}' exists in client but not in server"
            ))

    for name in server:
        if name not in client:
            deltas.append(ContractDelta(
                entity_type=EntityType.STRUCT,
                entity_name=name,
                member_name='',
                change_type=ChangeType.ADDED,
                direction=Direction.CLIENT_HAS_LESS,
                severity=Severity.INFO,
                description=f"Struct '{name}' exists in server but not in client"
            ))

    return deltas


def _diff_struct_fields(struct_name: str, client_struct: Dict[str, Any], server_struct: Dict[str, Any]) -> List[ContractDelta]:
    deltas: List[ContractDelta] = []
    client_fields = _extract_fields(client_struct)
    server_fields = _extract_fields(server_struct)

    for name, client_field in client_fields.items():
        if name in server_fields:
            type_changed, optionality_changed, was_required, is_required = _fields_equal_detailed(client_field, server_fields[name])
            if type_changed:
                deltas.append(ContractDelta(
                    entity_type=EntityType.FIELD,
                    entity_name=struct_name,
                    member_name=name,
                    change_type=ChangeType.MODIFIED,
                    direction=Direction.MISMATCH,
                    severity=Severity.ERROR,
                    description=f"Field '{name}' in struct '{struct_name}' has changed type"
                ))
            elif optionality_changed:
                if was_required and not is_required:
                    deltas.append(ContractDelta(
                        entity_type=EntityType.FIELD,
                        entity_name=struct_name,
                        member_name=name,
                        change_type=ChangeType.MODIFIED,
                        direction=Direction.CLIENT_HAS_LESS,
                        severity=Severity.INFO,
                        description=f"Field '{name}' in struct '{struct_name}' optionality changed from required to optional"
                    ))
                elif not was_required and is_required:
                    deltas.append(ContractDelta(
                        entity_type=EntityType.FIELD,
                        entity_name=struct_name,
                        member_name=name,
                        change_type=ChangeType.MODIFIED,
                        direction=Direction.CLIENT_HAS_LESS,
                        severity=Severity.WARNING,
                        description=f"Field '{name}' in struct '{struct_name}' optionality changed from optional to required"
                    ))
        else:
            deltas.append(ContractDelta(
                entity_type=EntityType.FIELD,
                entity_name=struct_name,
                member_name=name,
                change_type=ChangeType.REMOVED,
                direction=Direction.CLIENT_HAS_MORE,
                severity=Severity.INFO,
                description=f"Field '{name}' in struct '{struct_name}' exists in client but not in server"
            ))

    for name, server_field in server_fields.items():
        if name not in client_fields:
            is_required = _is_field_required(server_field)
            severity = _classify_severity(EntityType.FIELD, ChangeType.ADDED, Direction.CLIENT_HAS_LESS, "required" if is_required else "optional")
            deltas.append(ContractDelta(
                entity_type=EntityType.FIELD,
                entity_name=struct_name,
                member_name=name,
                change_type=ChangeType.ADDED,
                direction=Direction.CLIENT_HAS_LESS,
                severity=severity,
                description=f"Field '{name}' in struct '{struct_name}' exists in server but not in client"
            ))

    return deltas


def _extract_fields(struct_data: Dict[str, Any]) -> Dict[str, Any]:
    result = {}
    for field in struct_data.get('fields', []):
        name = field.get('name')
        if name:
            result[name] = field
    return result


def _fields_equal_detailed(a: Dict[str, Any], b: Dict[str, Any]) -> Tuple[bool, bool, bool, bool]:
    type_changed = not _maps_equal(a.get('type'), b.get('type'))
    a_optional = _get_field_optional(a)
    b_optional = _get_field_optional(b)
    was_required = not a_optional
    is_required = not b_optional
    optionality_changed = a_optional != b_optional
    return type_changed, optionality_changed, was_required, is_required


def _get_field_optional(field: Dict[str, Any]) -> bool:
    return bool(field.get('optional', False))


def _is_field_required(field: Dict[str, Any]) -> bool:
    return not field.get('optional', False)


def _diff_enums(client: Dict[str, Any], server: Dict[str, Any]) -> List[ContractDelta]:
    deltas: List[ContractDelta] = []

    for name, client_enum in client.items():
        if name in server:
            deltas.extend(_diff_enum_values(name, client_enum, server[name]))
        else:
            deltas.append(ContractDelta(
                entity_type=EntityType.ENUM,
                entity_name=name,
                member_name='',
                change_type=ChangeType.REMOVED,
                direction=Direction.CLIENT_HAS_MORE,
                severity=Severity.WARNING,
                description=f"Enum '{name}' exists in client but not in server"
            ))

    for name in server:
        if name not in client:
            deltas.append(ContractDelta(
                entity_type=EntityType.ENUM,
                entity_name=name,
                member_name='',
                change_type=ChangeType.ADDED,
                direction=Direction.CLIENT_HAS_LESS,
                severity=Severity.WARNING,
                description=f"Enum '{name}' exists in server but not in client"
            ))

    return deltas


def _diff_enum_values(enum_name: str, client_enum: Dict[str, Any], server_enum: Dict[str, Any]) -> List[ContractDelta]:
    deltas: List[ContractDelta] = []
    client_values = _extract_enum_values(client_enum)
    server_values = _extract_enum_values(server_enum)

    for name in client_values:
        if name not in server_values:
            deltas.append(ContractDelta(
                entity_type=EntityType.ENUM,
                entity_name=enum_name,
                member_name=name,
                change_type=ChangeType.REMOVED,
                direction=Direction.CLIENT_HAS_MORE,
                severity=Severity.WARNING,
                description=f"Enum value '{name}' in enum '{enum_name}' exists in client but not in server"
            ))

    for name in server_values:
        if name not in client_values:
            deltas.append(ContractDelta(
                entity_type=EntityType.ENUM,
                entity_name=enum_name,
                member_name=name,
                change_type=ChangeType.ADDED,
                direction=Direction.CLIENT_HAS_LESS,
                severity=Severity.WARNING,
                description=f"Enum value '{name}' in enum '{enum_name}' exists in server but not in client"
            ))

    return deltas


def _extract_enum_values(enum_data: Dict[str, Any]) -> Dict[str, bool]:
    result = {}
    for value in enum_data.get('values', []):
        name = value.get('name')
        if name:
            result[name] = True
    return result


def _diff_errors(client: Dict[str, Any], server: Dict[str, Any]) -> List[ContractDelta]:
    deltas: List[ContractDelta] = []

    for name in client:
        if name not in server:
            deltas.append(ContractDelta(
                entity_type=EntityType.ERROR,
                entity_name=name,
                member_name='',
                change_type=ChangeType.REMOVED,
                direction=Direction.CLIENT_HAS_MORE,
                severity=Severity.INFO,
                description=f"Error '{name}' exists in client but not in server"
            ))

    for name in server:
        if name not in client:
            deltas.append(ContractDelta(
                entity_type=EntityType.ERROR,
                entity_name=name,
                member_name='',
                change_type=ChangeType.ADDED,
                direction=Direction.CLIENT_HAS_LESS,
                severity=Severity.INFO,
                description=f"Error '{name}' exists in server but not in client"
            ))

    return deltas


def _classify_severity(entity_type: EntityType, change_type: ChangeType, direction: Direction, extra: str = '') -> Severity:
    if entity_type == EntityType.STRUCT:
        if change_type == ChangeType.REMOVED and direction == Direction.CLIENT_HAS_MORE:
            return Severity.ERROR
        if change_type == ChangeType.ADDED and direction == Direction.CLIENT_HAS_LESS:
            return Severity.INFO

    elif entity_type == EntityType.FIELD:
        if change_type == ChangeType.MODIFIED and direction == Direction.MISMATCH:
            return Severity.ERROR
        if change_type == ChangeType.REMOVED and direction == Direction.CLIENT_HAS_MORE:
            return Severity.INFO
        if change_type == ChangeType.ADDED and direction == Direction.CLIENT_HAS_LESS:
            if extra == 'required':
                return Severity.ERROR
            return Severity.INFO
        if change_type == ChangeType.MODIFIED and direction == Direction.CLIENT_HAS_LESS:
            if extra == 'made_required':
                return Severity.WARNING
            if extra == 'made_optional':
                return Severity.INFO
            return Severity.INFO

    elif entity_type == EntityType.METHOD:
        if change_type == ChangeType.REMOVED and direction == Direction.CLIENT_HAS_MORE:
            return Severity.ERROR
        if change_type == ChangeType.ADDED and direction == Direction.CLIENT_HAS_LESS:
            return Severity.WARNING
        if change_type == ChangeType.MODIFIED and direction == Direction.MISMATCH:
            return Severity.ERROR

    elif entity_type == EntityType.ENUM:
        if change_type == ChangeType.REMOVED and direction == Direction.CLIENT_HAS_MORE:
            return Severity.WARNING
        if change_type == ChangeType.ADDED and direction == Direction.CLIENT_HAS_LESS:
            return Severity.WARNING

    elif entity_type == EntityType.ERROR:
        if change_type == ChangeType.REMOVED and direction == Direction.CLIENT_HAS_MORE:
            return Severity.INFO
        if change_type == ChangeType.ADDED and direction == Direction.CLIENT_HAS_LESS:
            return Severity.INFO

    elif entity_type == EntityType.INTERFACE:
        if change_type == ChangeType.REMOVED and direction == Direction.CLIENT_HAS_MORE:
            return Severity.ERROR
        if change_type == ChangeType.ADDED and direction == Direction.CLIENT_HAS_LESS:
            return Severity.INFO

    return Severity.INFO