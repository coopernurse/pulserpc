## Context

The quickstart guides demonstrate PulseRPC's e-commerce checkout example but only document errors in comments within the IDL file, without showing how to use PulseRPC's formal `errors` keyword and `raises()` clause syntax. The errors guide at `docs/idl-guide/errors.md` explains this feature, but users don't see it in the quickstart context.

## Goals / Non-Goals

**Goals:**
- Update checkout.idl to use formal `errors` keyword and `raises()` clauses
- Add a "Defining Errors in IDL" section to each language quickstart showing the syntax
- Keep changes minimal - this is a documentation-only update

**Non-Goals:**
- No code changes to generators or runtime
- No changes to existing API behavior
- Not a comprehensive tutorial on error handling

## Decisions

1. **Update checkout.idl to use formal error syntax**
   - Move error definitions from comments to `errors {}` block
   - Add `raises()` clauses to `createOrder` method on OrderService
   - Rationale: Shows users the proper IDL syntax for declaring errors

2. **Add section to each quickstart showing the errors keyword**
   - Add "Defining Service Errors" subsection after the IDL section
   - Shows the `errors` block and `raises()` clause syntax
   - Links to full errors documentation
   - Rationale: Users see the pattern in context before implementing servers

3. **Keep existing "Error Codes" section unchanged**
   - Shows how to raise errors in server implementations
   - Rationale: Maintains existing content while adding the new declarative syntax

## Risks / Trade-offs

- Minimal risk - purely documentation changes
- No impact on generated code or runtime behavior
