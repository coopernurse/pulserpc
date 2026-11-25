## Context

`generateStructTypesGo` writes JSON tags from the IDL field name. Optional
fields currently become ``json:"name,omitempty"``; required fields become
``json:"name"``. The generator has no option to change that policy, and struct
tags are compiled into the generated package, so a runtime `ServerConfig` field
cannot change them after generation.

## Goals / Non-Goals

**Goals:**

- A generator-level opt-in that adds `omitempty` to every generated struct
  field.
- Unchanged default output.
- Deterministic, test-covered tag generation.

**Non-Goals:**

- Changing runtime serialization or `ServerConfig`.
- Changing other language generators.
- Omission of embedded parent fields (Go does not apply JSON tags to struct
  embedding in the same way).

## Decisions

### Generator flag, not runtime config

Add `-omit-empty` to `RegisterFlags` and thread its value to
`generateNamespaceGo`/`generateStructTypesGo`. Tags are part of generated source,
so a runtime option would have to replace the JSON encoder and replicate struct
tag behavior.

`OmitEmpty=true` means "add omitempty to all fields". Optional fields already
receive it, so the only visible change is on required fields. The default
`false` preserves the existing optional-only behavior.

**Alternative considered:** a `ServerConfig.OmitEmpty` field. Rejected: it
cannot change compiled struct tags and would create inconsistent request and
response behavior.

### Zero values are deliberately omitted

With the flag enabled, a required `int`, `float`, `bool`, or `string` field with
its zero value is absent from JSON. That is the requested behavior, but it can
conflict with IDL required-field validation on a request. The reference must
call this out; users who need explicit zeros should leave the flag off.

## Risks / Trade-offs

- **[Required fields disappear]** Enabling the flag can make a required field
  absent when its value is zero. Mitigation: document the caveat and keep the
  default off.
- **[Generated API churn]** Regenerating with the flag changes struct tags.
  Mitigation: opt-in flag only.
