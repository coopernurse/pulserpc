"""Helper functions for working with type definitions"""


def find_struct(struct_name, all_structs):
    """Find a struct definition by name"""
    return all_structs.get(struct_name)


def find_enum(enum_name, all_enums):
    """Find an enum definition by name"""
    return all_enums.get(enum_name)


def get_struct_fields(struct_name, all_structs):
    """Recursively resolve struct extends to return all fields (parent + child).

    Parent fields come first, in declaration order; a child field with the same
    name replaces the inherited one in place (preserving its position).  A
    name -> index map keeps overrides O(1) instead of rescanning the list.
    """
    struct_def = find_struct(struct_name, all_structs)
    if not struct_def:
        return []

    fields = []
    field_index = {}  # field name -> position in ``fields``

    if struct_def.get("extends"):
        for parent_field in get_struct_fields(struct_def["extends"], all_structs):
            field_index[parent_field["name"]] = len(fields)
            fields.append(parent_field)

    for field in struct_def.get("fields", []):
        name = field["name"]
        if name in field_index:
            fields[field_index[name]] = field
        else:
            field_index[name] = len(fields)
            fields.append(field)

    return fields
