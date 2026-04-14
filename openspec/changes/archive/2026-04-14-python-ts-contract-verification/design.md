## Context

Contract compatibility verification was implemented for Go, Java, and C# in prior changes. This design specifies implementing the same capability for Python and TypeScript runtimes.

The Python and TypeScript runtimes differ from Go/Java/C# in one key aspect: they do not embed `IDL_JSON` as a string constant in generated client code. Instead, the generator writes `idl.json` as a separate file. This design leverages that existing pattern by having the runtime read `idl.json` at runtime rather than requiring code embedding.

## Goals / Non-Goals

**Goals:**
- Mirror the Go/Java/C# verification capability in Python and TypeScript
- Use idiomatic patterns for each language (options dict for Python, options object for TypeScript)
- Allow custom auditor implementations
- Read local IDL from `idl.json` at runtime (no code embedding needed)
- Ensure behavioral consistency with Go/Java/C# implementations via shared test data
- Ensure quickstart and generator tests pass

**Non-Goals:**
- Modifying the Go, Java, or C# implementations (already complete)
- Embedding `IDL_JSON` constant in generated Python/TypeScript client code
- Adding cancellation support to Python (not needed)
- Changing the severity classification matrix (already defined in spec)

## Decisions

### Decision: Python Client Options Dict

**Chosen:** `Client(transport, validate_request=False, validate_response=False, options=None)`

**Rationale:** Python dictionaries are the natural way to pass optional configuration. A dict with keys `auditor` and `verify_on_bootstrap` keeps the API simple and Pythonic.

**Alternatives Considered:**
- Fluent builder (method chaining): More verbose, less Pythonic
- Separate methods like `with_auditor()`: Works but dict is simpler for multiple options

### Decision: TypeScript Client Constructor Options

**Chosen:** `new Client(transport, validateRequest = false, validateResponse = false, options?: ClientOptions)`

**Rationale:** TypeScript benefits from a typed options object. The options interface includes `auditor?: IContractAuditor` and `verifyOnBootstrap?: boolean`. This follows the async-first nature of TypeScript (async bootstrap requires async ready pattern).

**Alternatives Considered:**
- Separate methods like `withAuditor()`: Works but typed options is more TypeScript-like
- No options (always async verification): Less flexible

### Decision: Python SetLocalIDL Uses JSON String

**Chosen:** `set_local_idl(idl_json: str)` takes a JSON string, not a parsed dict

**Rationale:** Aligns with Go/Java/C# patterns where `IDL_JSON` is a string constant in generated code. User may load IDL from file, pass to function.

**Alternatives Considered:**
- Accept parsed dict: More convenient for internal use, but less consistent with other runtimes

### Decision: TypeScript verifyCompatibility is Async

**Chosen:** `async verifyCompatibility(): Promise<VerificationResult>`

**Rationale:** The TypeScript client already has async bootstrap and `ready()` method. Making verification async follows the same pattern and allows for future extensibility (e.g., async auditor implementations).

**Alternatives Considered:**
- Sync version: Would be inconsistent with async-first TypeScript runtime

### Decision: IDL Loading via Directory Walk

**Chosen:** The runtime provides a helper function that walks up from `__file__`/`import.meta.url` to find `idl.json`

**Rationale:** Handles all directory structure variations (with/without `-package` flag, single/multi-namespace). The generated client code doesn't need to know its own depth.

**Alternatives Considered:**
- Fixed relative path from generated client: Must account for directory depth variations
- Require explicit path: Less convenient, defeats "auto-discovery" purpose

### Decision: Shared Test Data File for Diff Consistency

**Chosen:** Create a language-neutral JSON file with test cases, each runtime reads and executes

**Rationale:** Ensures all language implementations produce identical results for same inputs. The test cases are already standardized in Go integration tests - express them in a portable format.

**Alternatives Considered:**
- Copy test code to each runtime: Duplication, drift over time
- Cross-language test script: More complexity, less isolation

## Risks / Trade-offs

[Risk] **Behavioral drift between language implementations** → Mitigation: Shared test data file ensures consistency

[Risk] **Different JSON library behavior** → Mitigation: Use the same JSON library each runtime already uses (json for Python, native JSON for TypeScript)

[Risk] **Quickstart test path resolution with package flag** → Mitigation: Directory walk helper handles all depths

## Open Questions

None at this time. Key decisions have been made based on existing implementation precedent and language idioms.
