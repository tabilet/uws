# UWS

UWS is the Udon Workflow Specification Go package. It defines the UWS 1.x document model, JSON Schema, validation helpers, and JSON/YAML/HCL conversion helpers.

UWS is similar in role to Arazzo, but it is a workflow overlay for OpenAPI-backed HTTP operations. OpenAPI owns methods, paths, schemas, servers, and security. UWS owns operation binding, workflow structure, request values, outputs, triggers, and control flow.

Non-OpenAPI runtimes such as command execution, function calls, file I/O, SSH, SQL, or LLM calls are extension-profile concerns represented with `x-*` fields, not UWS core service types. Operations without an OpenAPI binding are extension-owned and require `x-uws-operation-profile` to name the implementation profile that can execute them.

## Protocol

- Human-readable specification: [versions/1.0.0.md](versions/1.0.0.md)
- JSON Schema: [uws.json](uws.json)

## Packages

- `uws1` contains the UWS 1.x Go model, structural vocabulary, and structural validation.
- `convert` converts UWS documents between JSON, YAML, and the HCL authoring form.
- `versions/1.0.0.md` is the human-readable UWS 1.0 specification.
- `uws.json` is the JSON Schema for UWS 1.0 documents.

## Validation

Use `(*uws1.Document).Validate()` when an `error` is enough, or `ValidateResult()` when callers need all path-tagged validation errors.

```go
result := doc.ValidateResult()
if !result.Valid() {
    return result
}
```

Validation checks required root fields, OpenAPI operation bindings, extension-owned operation profiles, duplicate identifiers, standard request-binding keys, known structural types, selected reference integrity, action/criterion rules, and trigger routes.

`uws.json` provides structural JSON Schema validation. Use the Go validator for semantic checks such as duplicate identifiers and reference integrity.

## Execution

UWS 1.0 defines a bound-runtime execution model. UWS core owns orchestration and structural execution semantics; the bound runtime owns leaf execution plus the evaluation services needed for expressions and iterative constructs.

At a high level:

- `Document.Execute(ctx)` executes the document through the orchestrator
- `Document.DispatchTrigger(ctx, triggerID, output, payload)` dispatches a trigger event into the same execution model
- `Document.ExecutionRecords()` exposes the accumulated execution snapshot
- `Runtime` is responsible for leaf execution, expression evaluation, and item resolution

Execution requires a bound runtime and a document that passes validation for execution. Trigger dispatch resolves outputs by label or decimal index and routes only to declared workflows or top-level entry-workflow steps.

## Interchange

The `convert` package provides JSON, YAML, and HCL helpers such as `JSONToHCL`, `HCLToJSON`, and `MarshalYAML`. `MarshalHCL` works on a deep copy and does not mutate the caller-owned document.

HCL conversion preserves dynamic map keys such as `$ref` through reversible key rewriting. JSON and YAML preserve `x-*` extensions through the JSON extension model; HCL conversion rejects documents with `x-*` extensions because those fields would otherwise round-trip lossy.

## Development

```bash
go test ./...
go vet ./...
```

## License

Apache License 2.0. See [LICENSE](LICENSE).
