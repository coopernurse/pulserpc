"""Helper functions for working with type definitions"""

from collections import OrderedDict
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


class NameResolutionIndex:
    """Precomputed lookup tables for one fixed definitions dict.

    ``resolve_qualified_name`` used to call ``sorted(definitions)`` on every
    lookup (twice for a qualified miss), even though struct/enum tables are
    immutable for a Contract's lifetime.  Building this index once makes every
    subsequent lookup O(1) while preserving the original resolution rules:

    * exact keys always win;
    * a qualified suffix match (``key.endswith('.' + name)``) is tried next;
    * a last-segment / short-name match is tried last.

    Both maps are filled in sorted-key order and use "first key wins", which is
    exactly what the old linear scans over ``sorted(definitions)`` returned.
    ``ambiguous`` records names that had more than one candidate key; it is
    informational only and does not alter the deterministic first-sorted pick.
    """

    __slots__ = ('size', 'suffix', 'short', 'ambiguous')

    def __init__(self, definitions: Dict[str, Any]):
        self.size = len(definitions)
        self.suffix: Dict[str, str] = {}
        self.short: Dict[str, str] = {}
        self.ambiguous: set = set()

        for key in sorted(definitions):
            parts = key.rsplit('.')
            self._record(self.short, parts[-1], key)
            # A suffix match requires a '.' immediately before the name, so a
            # key only contributes suffixes starting at segment index 1.
            for i in range(1, len(parts)):
                self._record(self.suffix, '.'.join(parts[i:]), key)

    def _record(self, mapping: Dict[str, str], name: str, key: str) -> None:
        existing = mapping.get(name)
        if existing is None:
            mapping[name] = key
        elif existing != key:
            self.ambiguous.add(name)


# Index cache: id(definitions) -> (definitions, index).  The definitions object
# is held strongly so its id() cannot be recycled while it is cached.  The
# cache is bounded so long-running processes do not accumulate every transient
# dict ever passed to a helper.
_INDEX_CACHE_MAX = 256
_index_cache: "OrderedDict[int, tuple]" = OrderedDict()


def _get_resolution_index(definitions: Dict[str, Any]) -> NameResolutionIndex:
    """Return the cached index for ``definitions``, rebuilding on mutation."""
    cache_key = id(definitions)
    cached = _index_cache.get(cache_key)
    if cached is not None:
        cached_defs, index = cached
        if cached_defs is definitions and index.size == len(definitions):
            _index_cache.move_to_end(cache_key)
            return index

    index = NameResolutionIndex(definitions)
    _index_cache[cache_key] = (definitions, index)
    _index_cache.move_to_end(cache_key)
    while len(_index_cache) > _INDEX_CACHE_MAX:
        _index_cache.popitem(last=False)
    return index


def _resolve_with_index(name: str, definitions: Dict[str, Any],
                        index: NameResolutionIndex) -> Optional[str]:
    """Perform an O(1) resolution using a prebuilt index (review finding 2)."""
    # Exact-key-wins is checked against the live dict, not the index.
    if name in definitions:
        return name

    # e.g. reference "Response" -> key "inc.Response", or reference
    # "sub.inc.Response" -> key "inc.Response".
    key = index.suffix.get(name)
    if key is not None:
        return key

    # e.g. reference "inc.Response" -> key "Response".
    short_name = name.rsplit('.', 1)[-1]
    return index.short.get(short_name)


def resolve_qualified_name(name: str, definitions: Dict[str, Any],
                           _index: Optional[NameResolutionIndex] = None
                           ) -> Optional[str]:
    """Resolve a possibly namespace-qualified type name to a definition key.

    Generated IDLs key structs and enums by their plain name (e.g. ``Response``)
    while references may carry a namespace qualifier (e.g. ``inc.Response``).
    Exact matches win; otherwise a qualified suffix match and finally a
    last-segment match are used.  Validation and inheritance resolution both
    go through this helper so they cannot disagree (audit python-01).

    The lookup tables are built once per definitions dict and cached; callers
    may pass a prebuilt ``_index`` to avoid repeated cache lookups in hot loops.
    """
    if _index is None:
        _index = _get_resolution_index(definitions)
    return _resolve_with_index(name, definitions, _index)


def find_struct(struct_name: str, all_structs: Dict[str, Any],
                _index: Optional[NameResolutionIndex] = None
                ) -> Optional[Dict[str, Any]]:
    """Find a struct definition by name (qualified or unqualified)"""
    if _index is None:
        _index = _get_resolution_index(all_structs)
    key = _resolve_with_index(struct_name, all_structs, _index)
    return all_structs[key] if key is not None else None


def find_enum(enum_name: str, all_enums: Dict[str, Any],
              _index: Optional[NameResolutionIndex] = None
              ) -> Optional[Dict[str, Any]]:
    """Find an enum definition by name (qualified or unqualified)"""
    if _index is None:
        _index = _get_resolution_index(all_enums)
    key = _resolve_with_index(enum_name, all_enums, _index)
    return all_enums[key] if key is not None else None


def get_struct_fields(struct_name: str, all_structs: Dict[str, Any],
                      _visited: Optional[set] = None,
                      _index: Optional[NameResolutionIndex] = None
                      ) -> List[Dict[str, Any]]:
    """Recursively resolve struct extends to return all fields (parent + child)"""
    if _index is None:
        _index = _get_resolution_index(all_structs)
    key = _resolve_with_index(struct_name, all_structs, _index)
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
        parent_fields = get_struct_fields(
            struct_def['extends'], all_structs, _visited, _index
        )
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

