"""
PulseRPC Python Runtime Library

This library provides validation and RPC functionality for PulseRPC-generated code.
"""

from .rpc import RPCError
from .validation import (
    validate_type,
    validate_string,
    validate_int,
    validate_float,
    validate_bool,
    validate_array,
    validate_map,
    validate_enum,
    validate_struct,
)
from .types import (
    find_struct,
    find_enum,
    get_struct_fields,
    extract_checksum,
    EntityType,
    ChangeType,
    Direction,
    Severity,
    ContractDelta,
    VerificationResult,
)
from .diff import diff_idl
from .contract import (
    ContractAuditor,
    NoOpAuditor,
    LoggingAuditor,
    FailFastAuditor,
)
from .server import Server
from .client import Client
from .contract import Contract
from .transport import Transport, HttpTransport, InProcTransport

__all__ = [
    "RPCError",
    "validate_type",
    "validate_string",
    "validate_int",
    "validate_float",
    "validate_bool",
    "validate_array",
    "validate_map",
    "validate_enum",
    "validate_struct",
    "find_struct",
    "find_enum",
    "get_struct_fields",
    "extract_checksum",
    "EntityType",
    "ChangeType",
    "Direction",
    "Severity",
    "ContractDelta",
    "VerificationResult",
    "diff_idl",
    "ContractAuditor",
    "NoOpAuditor",
    "LoggingAuditor",
    "FailFastAuditor",
    "Server",
    "Client",
    "Contract",
    "Transport",
    "HttpTransport",
    "InProcTransport",
]

