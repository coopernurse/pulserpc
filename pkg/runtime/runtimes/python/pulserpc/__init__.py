"""
PulseRPC Python Runtime Library

This library provides validation and RPC functionality for PulseRPC-generated code.
"""

from .rpc import (
    RPCError,
    PARSE_ERROR,
    INVALID_REQUEST,
    METHOD_NOT_FOUND,
    INVALID_PARAMS,
    INTERNAL_ERROR,
)
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
from .rpctypes import (
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
    ValidationError,
    ValidationResult,
)
from .diff import diff_idl
from .contract import (
    ContractAuditor,
    NoOpAuditor,
    LoggingAuditor,
    FailFastAuditor,
)
from .server import Server, DEFAULT_MAX_BATCH_SIZE
from .client import Client
from .contract import Contract
from .transport import (
    Transport,
    HttpTransport,
    InProcTransport,
    DEFAULT_TIMEOUT,
    MAX_RESPONSE_BODY_SIZE,
)

__all__ = [
    "RPCError",
    "PARSE_ERROR",
    "INVALID_REQUEST",
    "METHOD_NOT_FOUND",
    "INVALID_PARAMS",
    "INTERNAL_ERROR",
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
    "ValidationError",
    "ValidationResult",
    "diff_idl",
    "ContractAuditor",
    "NoOpAuditor",
    "LoggingAuditor",
    "FailFastAuditor",
    "Server",
    "DEFAULT_MAX_BATCH_SIZE",
    "Client",
    "Contract",
    "Transport",
    "HttpTransport",
    "InProcTransport",
    "DEFAULT_TIMEOUT",
    "MAX_RESPONSE_BODY_SIZE",
]

