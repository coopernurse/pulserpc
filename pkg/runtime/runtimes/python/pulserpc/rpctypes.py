"""Helper functions for working with type definitions"""

from dataclasses import dataclass
from datetime import datetime
from enum import Enum
from typing import Any, Dict, List, Optional


class EntityType(str, Enum):
    INTERFACE = "Interface"
    METHOD = "Method"
    STRUCT = "Struct"
    FIELD = "Field"
    ENUM = "Enum"
    ERROR = "Error"


class ChangeType(str, Enum):
    ADDED = "Added"
    REMOVED = "Removed"
    MODIFIED = "Modified"


class Direction(str, Enum):
    CLIENT_HAS_MORE = "ClientHasMore"
    CLIENT_HAS_LESS = "ClientHasLess"
    MISMATCH = "Mismatch"


class Severity(str, Enum):
    ERROR = "Error"
    WARNING = "Warning"
    INFO = "Info"


@dataclass
class ContractDelta:
    entity_type: EntityType
    entity_name: str
    member_name: str
    change_type: ChangeType
    direction: Direction
    severity: Severity
    description: str


@dataclass
class VerificationResult:
    compatible: bool
    server_checksum: str
    client_checksum: str
    deltas: List[ContractDelta]
    timestamp: datetime


def resolve_qualified_name(name: str, definitions: Dict[str, Any]) -> Optional[str]:
    """Resolve a possibly namespace-qualified type name to a definition key.

    Generated IDLs key structs and enums by their plain name (e.g. ``Response``)
    while references may carry a namespace qualifier (e.g. ``inc.Response``).
    Exact matches win; otherwise a qualified suffix match and finally a
    last-segment match are used.  Validation and inheritance resolution both
    go through this helper so they cannot disagree (audit python-01).
    """
    if name in definitions:
        return name

    # e.g. reference "Response" -> key "inc.Response", or reference
    # "sub.inc.Response" -> key "inc.Response".
    for key in sorted(definitions):
        if key.endswith('.' + name):
            return key

    # e.g. reference "inc.Response" -> key "Response".
    short_name = name.rsplit('.', 1)[-1]
    for key in sorted(definitions):
        if key.rsplit('.', 1)[-1] == short_name:
            return key

    return None


def find_struct(struct_name: str, all_structs: Dict[str, Any]) -> Optional[Dict[str, Any]]:
    """Find a struct definition by name (qualified or unqualified)"""
    key = resolve_qualified_name(struct_name, all_structs)
    return all_structs[key] if key is not None else None


def find_enum(enum_name: str, all_enums: Dict[str, Any]) -> Optional[Dict[str, Any]]:
    """Find an enum definition by name (qualified or unqualified)"""
    key = resolve_qualified_name(enum_name, all_enums)
    return all_enums[key] if key is not None else None


def get_struct_fields(struct_name: str, all_structs: Dict[str, Any],
                      _visited: Optional[set] = None) -> List[Dict[str, Any]]:
    """Recursively resolve struct extends to return all fields (parent + child)"""
    key = resolve_qualified_name(struct_name, all_structs)
    if key is None:
        return []

    # Guard against cyclic extends clauses (defensive: the parser also rejects them).
    if _visited is None:
        _visited = set()
    if key in _visited:
        return []
    _visited = _visited | {key}

    struct_def = all_structs[key]
    fields = []
    
    # Get parent fields first
    if struct_def.get('extends'):
        parent_fields = get_struct_fields(struct_def['extends'], all_structs, _visited)
        fields.extend(parent_fields)
    
    # Add child fields (override parent if name conflict)
    field_names = {f['name'] for f in fields}
    for field in struct_def.get('fields', []):
        if field['name'] not in field_names:
            fields.append(field)
            field_names.add(field['name'])
        else:
            # Override parent field
            for i, f in enumerate(fields):
                if f['name'] == field['name']:
                    fields[i] = field
                    break
    
    return fields


@dataclass
class ValidationError:
    path: str
    message: str


@dataclass
class ValidationResult:
    valid: bool
    error: Optional[str] = None
    invalid_fields: Optional[List[str]] = None


def extract_checksum(idl: Any) -> str:
    """Extract checksum from IDL data.
    
    Reads the checksum field from IDL data that was computed at code generation time.
    This avoids the need to compute SHA-256 at runtime.
    
    Args:
        idl: Parsed IDL data (dict with 'checksum' field)
        
    Returns:
        Checksum string, or empty string if not found
    """
    if isinstance(idl, dict):
        checksum = idl.get('checksum')
        if checksum is not None:
            return str(checksum)
    return ''

