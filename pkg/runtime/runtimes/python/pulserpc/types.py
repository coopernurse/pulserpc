"""Helper functions for working with type definitions"""

from typing import Any, Dict, List, Optional


def find_struct(struct_name: str, all_structs: Dict[str, Any]) -> Optional[Dict[str, Any]]:
    """Find a struct definition by name"""
    return all_structs.get(struct_name)


def find_enum(enum_name: str, all_enums: Dict[str, Any]) -> Optional[Dict[str, Any]]:
    """Find an enum definition by name"""
    return all_enums.get(enum_name)


def get_struct_fields(struct_name: str, all_structs: Dict[str, Any]) -> List[Dict[str, Any]]:
    """Recursively resolve struct extends to return all fields (parent + child)"""
    struct_def = find_struct(struct_name, all_structs)
    if not struct_def:
        return []
    
    fields = []
    
    # Get parent fields first
    if struct_def.get('extends'):
        parent_fields = get_struct_fields(struct_def['extends'], all_structs)
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

