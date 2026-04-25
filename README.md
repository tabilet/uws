# UWS

UWS is the Udon Workflow Specification Go package. It defines the UWS 1.x document model, JSON Schema, validation helpers, and JSON/YAML/HCL conversion helpers.

UWS is similar in role to Arazzo, but it is a workflow overlay for OpenAPI-backed HTTP operations. OpenAPI owns methods, paths, schemas, servers, and security. UWS owns operation binding, workflow structure, request values, outputs, triggers, and control flow.

Non-OpenAPI runtimes such as command execution, function calls, file I/O, SSH, SQL, or LLM calls are extension-profile concerns represented with `x-*` fields, not UWS core service types. Operations without an OpenAPI binding are extension-owned and require `x-uws-operation-profile` to name the implementation profile that can execute them.

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

## Conversion

The `convert` package exposes JSON, YAML, and HCL helpers:

```go
hclData, err := convert.JSONToHCL(jsonData)
jsonData, err := convert.HCLToJSON(hclData)
yamlData, err := convert.MarshalYAML(doc)
```

`MarshalHCL` works on a deep copy and does not mutate the caller-owned document. HCL conversion preserves dynamic map keys such as `$ref` through reversible key rewriting. JSON and YAML helpers preserve `x-*` extensions through the JSON extension model; HCL conversion rejects documents with `x-*` extensions because extension maps are intentionally excluded from the HCL struct tags and would otherwise be lossy.

## Development

```bash
go test ./...
go vet ./...
```

## License

Apache License 2.0. See [LICENSE](LICENSE).
