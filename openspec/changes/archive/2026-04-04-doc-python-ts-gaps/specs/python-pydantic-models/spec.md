## ADDED Requirements

### Requirement: Pydantic model generation flag

The CLI SHALL support a `--use-pydantic` flag when using the `python-client-server` plugin that generates a `models.py` file containing Pydantic `BaseModel` classes for all struct and enum types defined in the IDL.

#### Scenario: Flag generates models.py
- **WHEN** user runs `pulserpc -plugin python-client-server -dir ./output --use-pydantic service.pulse`
- **THEN** a `models.py` file is created in the output directory
- **AND** the file contains Pydantic models for all structs and enums

#### Scenario: Flag without pydantic generates no models.py
- **WHEN** user runs `pulserpc -plugin python-client-server -dir ./output service.pulse` without `--use-pydantic`
- **THEN** no `models.py` file is created

### Requirement: Pydantic model structure

Generated Pydantic models SHALL use the `BaseModel` class from Pydantic and include field definitions matching the IDL struct/enum definitions.

#### Scenario: Struct generates Pydantic model with fields
- **WHEN** IDL defines `struct Product { name string, price float, stock int }`
- **THEN** generated `models.py` contains `class Product(BaseModel)` with `name: str`, `price: float`, `stock: int` fields

#### Scenario: Optional field in Pydantic model
- **WHEN** IDL defines `struct Product { name string, description string [optional] }`
- **THEN** generated model has `description: Optional[str] = None`

#### Scenario: Enum generates Pydantic model
- **WHEN** IDL defines `enum OrderStatus { pending, paid, shipped, delivered }`
- **THEN** generated `models.py` contains a Pydantic model with string field and JSON schema examples for each value

### Requirement: Pydantic model cross-namespace support

Generated Pydantic models SHALL correctly import types from other namespaces when used in field definitions.

#### Scenario: Cross-namespace type reference
- **WHEN** a struct in namespace `order` references a struct in namespace `common`
- **THEN** generated model uses `Optional['CommonType']` annotation with appropriate import
