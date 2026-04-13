# UWS

UWS is the Udon Workflow Specification Go package. It defines the UWS 1.x document model, JSON Schema, validation helpers, and JSON/YAML/HCL conversion helpers.

UWS is similar in role to Arazzo, but it describes multi-service workflows rather than only OpenAPI operation sequences. Operations are defined inline and can represent HTTP, SSH, command, function, and other generic service types.

## Packages

- `uws1` contains the UWS 1.x Go model and structural validation.
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

Validation checks required root fields, duplicate identifiers, known service and structural types, selected reference integrity, action/criterion rules, trigger routes, security scheme shape, and selected schema-alignment constraints.

## Conversion

The `convert` package exposes JSON, YAML, and HCL helpers:

```go
hclData, err := convert.JSONToHCL(jsonData)
jsonData, err := convert.HCLToJSON(hclData)
yamlData, err := convert.MarshalYAML(doc)
```

`MarshalHCL` works on a deep copy and does not mutate the caller-owned document. HCL conversion preserves dynamic map keys such as `$ref` through reversible key rewriting. JSON and YAML helpers preserve `x-*` extensions through the JSON extension model; HCL struct conversion does not currently emit arbitrary `x-*` extension attributes because extension maps are intentionally excluded from the HCL struct tags.

## Development

```bash
go test ./...
go vet ./...
```

## License

Apache License 2.0. See [LICENSE](LICENSE).
