# AGENTS.md

## Commands

```bash
go test ./...                              # full test suite
go test ./uws1 -run TestSchemaConformance  # run a single test by name
go test ./convert -run TestRoundtrip       # run tests in one package
go vet ./...
```

Module path: `github.com/tabilet/uws` (Go 1.25.4).

## Architecture

UWS is a workflow overlay for OpenAPI-backed HTTP operations. OpenAPI owns methods, paths, schemas, servers, and security; UWS owns operation binding, workflow structure, request values, outputs, triggers, and control flow. Non-HTTP runtimes (command exec, SSH, SQL, LLM, etc.) are extension-profile concerns expressed via `x-*` fields and `x-uws-operation-profile`, not built-in service types.

Three coordinated artifacts must stay in sync:

1. `uws.json` — the canonical JSON Schema for UWS 1.0 documents.
2. `uws1/` — the Go model and semantic validator.
3. `versions/1.0.0.md` — the human-readable spec. `versions/arazzo.md`, `versions/article.md`, and `ideas/terraform.md` are comparison/background docs.

## Execution Model

This repo uses a bound-runtime execution model. Structural orchestration lives in `uws1`; concrete engines bind a runtime implementation at execution time.

### Runtime lives on the base document

The base `Document` carries a `Runtime` interface reference. The runtime provides only leaf execution plus expression/item evaluation.

```go
type Document struct {
    Runtime Runtime
}
```

### Structural execution stays in UWS core

`Document.Execute()` constructs an `Orchestrator`, and the orchestrator walks workflows, steps, and dependencies. Leaf operations are delegated to the bound runtime.

```go
func (d *Document) Execute() error {
    orch := NewOrchestrator(d, d.Runtime)
    return orch.Execute(context.Background())
}
```

### Binding happens at execution time

The specialized engine binds its runtime implementation to the document before execution.

```go
func ExecuteWithRuntime(ctx context.Context, doc *uws1.Document, rt Runtime) error {
    doc.SetRuntime(rt)
    return doc.Execute(ctx)
}
```

Benefits of this model:

- Structural orchestration logic (`loop`, `switch`, `parallel`, `merge`, `await`) is defined once in `uws1`.
- `uws1` has zero knowledge of any concrete engine's specific runtimes.
- All UWS-compliant executors share the same orchestration behavior and bind their own runtime implementations.

## Schema / Spec / Code Sync

`uws1/schema_conformance_test.go` is the bridge between the schema and the Go validator. It reads `uws.json` and asserts that the schema's required fields, enums, patterns, and related rule coverage match the Go-side validation rules.

When changing validation rules:

1. update `uws.json`
2. update `uws1/validation.go`
3. update `versions/1.0.0.md` when the public contract changed
4. make the schema conformance and parity tests pass

## Validation Layering

- `uws.json` covers structural and shape checks.
- `(*Document).Validate()` / `ValidateResult()` in `uws1/validation.go` cover semantic checks the schema cannot: duplicate identifiers, OpenAPI binding rules, reference integrity across operations/workflows/steps/triggers/parallel groups/sourceDescriptions, action and criterion rules, trigger routes, and standard request-binding keys.
- Use `Validate()` when a single `error` is enough.
- Use `ValidateResult()` when callers need path-tagged errors.

## Conversion

`convert/` provides JSON, YAML, and HCL helpers.

Key invariants:

- JSON and YAML preserve `x-*` extensions through the `Extensions` map pattern.
- HCL intentionally drops extensions. `MarshalHCL` rejects documents containing `x-*` fields rather than silently losing them.
- HCL key rewriting preserves `$`-prefixed keys on round-trip. Legacy JSON Schema keys (`$ref`, `$id`, `$schema`, `$defs`, etc.) use the `_`-prefix form in HCL; other `$foo` keys use `__dollar__foo`.
- `MarshalHCL` works on a deep copy and does not mutate the caller's document.

## Extension Pattern

Every type that accepts `x-*` fields follows the same recipe:

1. an `Extensions map[string]any` field with `json:"-" yaml:"-" hcl:"-"`
2. a `knownFields` list of schema-owned keys
3. `UnmarshalJSON` using a type alias plus `rejectUnknownFields` and `extractExtensions`
4. `MarshalJSON` calling `marshalWithExtensions`

When adding a field to such a type, update both the struct and its `knownFields` list or the unmarshaller will reject valid documents.

## References

- [Peter Bi, "Achieving Full Object Inheritance in Go", Medium, 2024](https://medium.com/@peterbi_91340/implement-true-inheritance-in-go-ff6243bfd7a8)
