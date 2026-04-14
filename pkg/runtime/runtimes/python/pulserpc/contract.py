"""Contract class for IDL validation and interface metadata

This module provides the Contract class which parses IDL metadata
and provides validation for requests and responses.
"""

import logging
from abc import ABC, abstractmethod
from datetime import datetime
from typing import Any, Dict, List, Optional
from .validation import validate_type
from .types import VerificationResult, ContractDelta, Severity


class ContractAuditor(ABC):
    """Abstract base class for contract auditors"""

    @abstractmethod
    def audit(self, result: VerificationResult) -> None:
        """Audit a verification result
        
        Args:
            result: The verification result to audit
        """
        pass

    @abstractmethod
    def name(self) -> str:
        """Return the auditor's name
        
        Returns:
            String name of the auditor
        """
        pass


class NoOpAuditor(ContractAuditor):
    """Auditor that performs no action"""

    def audit(self, result: VerificationResult) -> None:
        pass

    def name(self) -> str:
        return "NoOp"


class LoggingAuditor(ContractAuditor):
    """Auditor that logs verification results"""

    def audit(self, result: VerificationResult) -> None:
        if not result.compatible:
            logging.error(f"Contract incompatibility detected: {len(result.deltas)} deltas found")
        for delta in result.deltas:
            if delta.severity == Severity.ERROR:
                logging.error(f"{delta.entity_type}: {delta.description}")
            elif delta.severity == Severity.WARNING:
                logging.warning(f"{delta.entity_type}: {delta.description}")
            elif delta.severity == Severity.INFO:
                logging.info(f"{delta.entity_type}: {delta.description}")
        if result.compatible and not result.deltas:
            logging.info("Contract compatibility verified: client and server IDLs are identical")

    def name(self) -> str:
        return "Logging"


class FailFastAuditor(ContractAuditor):
    """Auditor that raises an exception on incompatible contracts"""

    def audit(self, result: VerificationResult) -> None:
        if not result.compatible:
            error_deltas = [d for d in result.deltas if d.severity == Severity.ERROR]
            messages = "; ".join(f"{d.entity_type}: {d.description}" for d in error_deltas)
            raise RuntimeError(f"Contract compatibility verification failed: {messages}")

    def name(self) -> str:
        return "FailFast"


class Interface:
    """Represents an interface from the IDL"""

    def __init__(self, iface_data: Dict[str, Any]):
        """Initialize Interface from IDL data

        Args:
            iface_data: Interface dict from parsed IDL
        """
        self.name = iface_data['name']
        self.functions: Dict[str, Dict[str, Any]] = {}
        for func in iface_data.get('methods', []):
            self.functions[func['name']] = func

    def get_function(self, func_name: str) -> Optional[Dict[str, Any]]:
        """Get function metadata by name

        Args:
            func_name: Name of the function

        Returns:
            Function dict or None if not found
        """
        return self.functions.get(func_name)


class Contract:
    """Represents a parsed IDL contract

    The Contract class parses IDL JSON and provides validation
    for requests and responses based on the interface definitions.
    """

    def __init__(self, idl_parsed):
        """Initialize Contract from parsed IDL

        Args:
            idl_parsed: Parsed IDL (can be a list like barrister or a dict with top-level keys)
        """
        # Handle both barrister format (list) and PulseRPC format (dict)
        if isinstance(idl_parsed, dict):
            # PulseRPC format - dict with interfaces, structs, enums keys
            self.idl_parsed = idl_parsed
            self.interfaces: Dict[str, Interface] = {}
            self.structs: Dict[str, Dict[str, Any]] = {}
            self.enums: Dict[str, Dict[str, Any]] = {}
            self.meta: Dict[str, Any] = {}

            # Parse interfaces
            for iface_data in idl_parsed.get('interfaces', []):
                self.interfaces[iface_data['name']] = Interface(iface_data)

            # Parse structs
            for struct_data in idl_parsed.get('structs', []):
                self.structs[struct_data['name']] = struct_data

            # Parse enums
            for enum_data in idl_parsed.get('enums', []):
                self.enums[enum_data['name']] = enum_data
        else:
            # Barrister format - list of items with type field
            self.idl_parsed = idl_parsed
            self.interfaces: Dict[str, Interface] = {}
            self.structs: Dict[str, Dict[str, Any]] = {}
            self.enums: Dict[str, Dict[str, Any]] = {}
            self.meta: Dict[str, Any] = {}

            for item in idl_parsed:
                item_type = item.get('type')
                if item_type == 'struct':
                    self.structs[item['name']] = item
                elif item_type == 'enum':
                    self.enums[item['name']] = item
                elif item_type == 'interface':
                    self.interfaces[item['name']] = Interface(item)
                elif item_type == 'meta':
                    # Copy metadata
                    for key, value in item.items():
                        if key != 'type':
                            self.meta[key] = value

    def has_interface(self, iface_name: str) -> bool:
        """Check if interface exists

        Args:
            iface_name: Name of the interface

        Returns:
            True if interface exists
        """
        return iface_name in self.interfaces

    def get_interface(self, iface_name: str) -> Optional[Interface]:
        """Get interface by name

        Args:
            iface_name: Name of the interface

        Returns:
            Interface instance or None if not found
        """
        return self.interfaces.get(iface_name)

    def validate_request(self, iface_name: str, func_name: str,
                        params: List[Any]) -> None:
        """Validate request parameters against IDL

        Args:
            iface_name: Interface name
            func_name: Function name
            params: List of parameter values

        Raises:
            ValueError: If validation fails
            TypeError: If parameter types don't match
        """
        interface = self.get_interface(iface_name)
        if not interface:
            raise ValueError(f"Unknown interface: '{iface_name}'")

        func = interface.get_function(func_name)
        if not func:
            raise ValueError(f"{iface_name}: Unknown function: '{func_name}'")

        param_defs = func.get('parameters', [])

        # Check parameter count
        if len(params) != len(param_defs):
            raise ValueError(
                f"Function '{iface_name}.{func_name}' expects "
                f"{len(param_defs)} param(s). {len(params)} given."
            )

        # Validate each parameter
        for i, param_value in enumerate(params):
            param_def = param_defs[i]
            param_name = param_def['name']
            param_type = param_def['type']
            is_optional = param_def.get('optional', False)

            try:
                validate_type(param_value, param_type, self.structs,
                            self.enums, is_optional)
            except (TypeError, ValueError) as e:
                raise ValueError(
                    f"Function '{iface_name}.{func_name}' invalid param "
                    f"'{param_name}'. {e}"
                ) from e

    def validate_response(self, iface_name: str, func_name: str,
                         result: Any) -> None:
        """Validate response result against IDL

        Args:
            iface_name: Interface name
            func_name: Function name
            result: Result value from the function

        Raises:
            ValueError: If validation fails
            TypeError: If result type doesn't match
        """
        interface = self.get_interface(iface_name)
        if not interface:
            raise ValueError(f"Unknown interface: '{iface_name}'")

        func = interface.get_function(func_name)
        if not func:
            raise ValueError(f"{iface_name}: Unknown function: '{func_name}'")

        # Check if function has a return type
        return_type = func.get('returnType')
        if not return_type:
            # Function returns void/None
            if result is not None:
                raise ValueError(
                    f"Function '{iface_name}.{func_name}' invalid response: "
                    f"'{result}'. Expected None"
                )
            return

        # Validate return type
        is_optional = func.get('returnOptional', False)
        try:
            validate_type(result, return_type, self.structs,
                        self.enums, is_optional)
        except (TypeError, ValueError) as e:
            raise ValueError(
                f"Function '{iface_name}.{func_name}' invalid response: "
                f"'{result}'. {e}"
            ) from e
