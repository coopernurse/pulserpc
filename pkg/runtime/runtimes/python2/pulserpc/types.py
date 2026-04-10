"""Helper functions for working with type definitions"""

try:
    unicode
except NameError:
    unicode = str


def find_struct(struct_name, all_structs):
    """Find a struct definition by name"""
    return all_structs.get(struct_name)


def find_enum(enum_name, all_enums):
    """Find an enum definition by name"""
    return all_enums.get(enum_name)


def get_struct_fields(struct_name, all_structs):
    """Recursively resolve struct extends to return all fields (parent + child)"""
    struct_def = find_struct(struct_name, all_structs)
    if not struct_def:
        return []
    
    fields = []
    
    if struct_def.get('extends'):
        parent_fields = get_struct_fields(struct_def['extends'], all_structs)
        fields.extend(parent_fields)
    
    field_names = set()
    for f in fields:
        field_names.add(f['name'])
    for field in struct_def.get('fields', []):
        if field['name'] not in field_names:
            fields.append(field)
            field_names.add(field['name'])
        else:
            for i, f in enumerate(fields):
                if f['name'] == field['name']:
                    fields[i] = field
                    break
    
    return fields
