# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
go test ./...                              # full test suite
go test ./uws1 -run TestSchemaConformance  # run a single test by name
go test ./convert -run TestRoundtrip       # run tests in one package
go vet ./...
```

Module path: `github.com/tabilet/uws` (Go 1.25.4).

## Architecture

UWS is a workflow overlay for OpenAPI-backed HTTP operations — similar in role to Arazzo. OpenAPI owns methods, paths, schemas, servers, and security; UWS owns operation binding, workflow structure, request values, outputs, triggers, and control flow. Non-HTTP runtimes (command exec, SSH, SQL, LLM, etc.) are **extension-profile** concerns expressed via `x-*` fields and `x-uws-operation-profile`, not built-in service types.

Three coordinated artifacts must stay in sync:

1. **`uws.json`** — the canonical JSON Schema for UWS 1.0 documents.
2. **`uws1/`** — the Go model and semantic validator. Struct tags cover `json`, `yaml`, and `hcl` simultaneously; every object that accepts `x-*` fields has an `Extensions map[string]any` with custom `UnmarshalJSON`/`MarshalJSON` that round-trips extensions and rejects unknown non-`x-*` keys (see `uws1/extensions.go`, pattern repeated in `document.go`, `operation.go`, etc.).
3. **`versions/1.0.0.md`** — the human-readable spec. `versions/arazzo.md`, `versions/article.md`, and `ideas/terraform.md` are comparison/background docs.

`uws1/schema_conformance_test.go` is the bridge: it reads `uws.json` and asserts that the schema's required fields, enums, and `$defs` match the Go validator's rules (e.g. `validWorkflowTypes`, `standardRequestKeys`). **When you change validation rules, update both `uws.json` and `uws1/validation.go`, and this test will catch drift.**

### Validation layering

- `uws.json` (via `santhosh-tekuri/jsonschema`) covers structural/shape checks.
- `(*Document).Validate()` / `ValidateResult()` in `uws1/validation.go` cover semantic checks the schema cannot: duplicate identifiers, OpenAPI operation binding vs. `x-uws-operation-profile` mutual exclusivity, reference integrity across operations/workflows/steps/triggers/parallel groups/sourceDescriptions, action/criterion rules, trigger routes, standard request-binding keys (`path`/`query`/`header`/`cookie`/`body`).
- Use `Validate()` for a single `error`; use `ValidateResult()` when callers need path-tagged errors.

### Conversion (`convert/`)

JSON ↔ YAML ↔ HCL helpers. Key invariants:

- **JSON and YAML** preserve `x-*` extensions through the `Extensions` map pattern.
- **HCL** intentionally drops extensions — `MarshalHCL` rejects documents containing `x-*` fields rather than silently losing them. `hcl` struct tags exclude extension maps.
- **HCL key rewriting**: `$`-prefixed map keys (e.g. `$ref`) are rewritten to HCL-safe forms on the way in and restored on the way out. Legacy JSON Schema keys (`$ref`, `$id`, `$schema`, `$defs`, …) use the `_`-prefix form; other `$foo` keys use `__dollar__foo`. This lives in `convert.go`'s `toHCLKey`/`fromHCLKey`.
- `MarshalHCL` works on a deep copy — it does not mutate the caller's document.

### Extension pattern (important when adding fields)

Every type that may carry `x-*` fields follows the same four-part recipe:

1. `Extensions map[string]any` field with `json:"-" yaml:"-" hcl:"-"`.
2. A `var xxxKnownFields = []string{...}` list of the schema-owned keys.
3. `UnmarshalJSON` using a type alias, then `rejectUnknownFields` + `extractExtensions`.
4. `MarshalJSON` calling `marshalWithExtensions`.

When adding a new field to an object, add it to the struct **and** the `knownFields` list, or the unmarshaller will reject valid documents.
