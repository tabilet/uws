# UWS

<p align="center">
  <img src="docs/assets/uws2.png" alt="UWS logo" width="520">
</p>

UWS is the Udon Workflow Specification Go package. It defines the UWS 1.x document model, JSON Schema, validation helpers, and JSON/YAML/HCL conversion helpers.

UWS is a workflow **overlay** over source documents — OpenAPI, AsyncAPI, GraphQL, OpenRPC, Protocol Buffers, OData, and the like. The source document owns the operations: methods, paths, channels, messages, schemas, servers, and security are all defined server-side and authoritative. UWS adds only what the source cannot express: operation binding, workflow structure, request values, outputs, triggers, and control flow.

This is what distinguishes UWS from full client-side workflow tools such as Arazzo and IaC engines such as OpenTofu and Terraform. Arazzo describes full client-side action sequences and treats each step as a bespoke client action. OpenTofu and Terraform act as full client-side workflow engines for infrastructure: each resource and provider call is described in the client configuration and resolved against a provider plugin at apply time. Neither approach assumes that the underlying operations are already defined by a server contract. UWS takes the opposite position: server actions are pre-defined by the source document, and UWS workflows reference those operations by ID rather than re-describing them. The result is a much smaller overlay: UWS does not duplicate request/response shapes, does not redeclare endpoints, and does not encode anything the source document already specifies.

UWS 1.9.1 is the latest release. It keeps OpenAPI compatibility, supports nine source description types, and adds reviewable content-provenance declarations plus deterministic advisory analysis. The `ansible-module` source type added in 1.6 was removed in 1.7, and UWS 1.9 defines no replacement Ansible operation profile. Missing `sourceDescription.type` still defaults to `openapi`; legacy `openapiOperationId` and `openapiOperationRef` remain valid for OpenAPI sources.

### Version highlights

| Version | Adds |
|---|---|
| **1.0** | Initial spec: OpenAPI-bound operations, workflow structure, request binding, structural control flow (sequence/parallel/switch/loop/merge/await), triggers, results, success criteria, success/failure actions, runtime expressions, and the `x-uws-` extension prefix with `x-uws-operation-profile`. |
| **1.1** | Portable `timeout` on operations/workflows/steps; workflow-level `idempotency` metadata for run de-duplication. |
| **1.2** | First-class `sourceDescription.type` for `openapi`, `google-discovery`, `aws-smithy`; canonical `sourceOperationId` / `sourceOperationRef` selectors. Legacy `openapiOperationId` / `openapiOperationRef` kept for OpenAPI sources. |
| **1.3** | First-class `asyncapi` source type; AsyncAPI operation selector rules including `#/operations/...`, `#/channels/...`, and `#/channels/.../messages/...` ref forms. |
| **1.4** | First-class `graphql`, `openrpc`, `grpc-protobuf`, and `odata` source types; generic selectors required for those families. |
| **1.5** | First-class `browser-profile` source type; capability profile sub-spec published separately as `versions/browser.1.5.{json,md}`. |
| **1.6** | First-class `ansible-module` source type (FQCN as `sourceOperationId`, `#/modules/<fqcn>` refs); argspec sub-spec published separately as `versions/ansible.1.0.{json,md}`. |
| **1.7** | Removed `ansible-module`: the managed host does not expose the collection module as a pre-existing operation, and UWS 1.7 does not standardize Ansible module calls. |
| **1.8** | Kept UWS core stable; added browser 1.6 and browser-authentication/call 1.1 profiles for bounded popup/frame contexts while retaining all older schemas. |
| **1.9.0** | Kept UWS core stable; added browser 1.7 locale-free scalar conversion for accessibility-text outputs while retaining browser authentication 1.1 and all older schemas. |
| **1.9.1** | Added optional `contentTrust` declarations and deterministic advisory provenance/capability analysis without changing existing output shapes or execution behavior. |

See [`versions/CHANGELOG.md`](versions/CHANGELOG.md) for the full changelog.

Non-source runtimes such as command execution, function calls, file I/O, SSH, SQL, browser automation, or LLM calls are extension-profile concerns represented with `x-*` fields, not UWS core service types. Operations without a source binding are extension-owned and require `x-uws-operation-profile` to name the implementation profile that can execute them. The optional `uws.runtime.1.0` supplement standardizes a small `x-uws-runtime` selector payload for those extension-owned operations.


[![GoDoc](https://godoc.org/github.com/OpenUdon/uws?status.svg)](https://godoc.org/github.com/OpenUdon/uws)


## Documentation

- **Docs site**: [openudon.github.io/uws](https://openudon.github.io/uws/)
- Human-readable specification: [versions/1.9.1.md](versions/1.9.1.md)
- Content trust guide: [docs/content-trust.md](docs/content-trust.md)
- Runtime supplement: [versions/runtime.1.0.md](versions/runtime.1.0.md)
- Runtime supplement schema: [versions/runtime.1.0.json](versions/runtime.1.0.json)
- Browser profile supplement: [versions/browser.1.7.md](versions/browser.1.7.md) / [versions/browser.1.7.json](versions/browser.1.7.json)
- Browser authentication profile: [versions/browser-authentication.1.1.md](versions/browser-authentication.1.1.md) / [versions/browser-authentication.1.1.json](versions/browser-authentication.1.1.json)
- Browser authentication call supplement: [versions/browser-authentication-call.1.1.md](versions/browser-authentication-call.1.1.md) / [versions/browser-authentication-call.1.1.json](versions/browser-authentication-call.1.1.json)
- Browser registration profile: [versions/browser-registration.1.1.md](versions/browser-registration.1.1.md) / [versions/browser-registration.1.1.json](versions/browser-registration.1.1.json)
- Browser registration call supplement: [versions/browser-registration-call.1.1.md](versions/browser-registration-call.1.1.md) / [versions/browser-registration-call.1.1.json](versions/browser-registration-call.1.1.json)
- Private registration input envelope: [versions/browser-registration-input.1.0.md](versions/browser-registration-input.1.0.md) / [versions/browser-registration-input.1.0.json](versions/browser-registration-input.1.0.json)
- Browser capability distribution milestone: [docs/browser-capability-goal.md](docs/browser-capability-goal.md)
- UWS 1.6 Ansible argspec (historical): [versions/ansible.1.0.md](versions/ansible.1.0.md) / [versions/ansible.1.0.json](versions/ansible.1.0.json)
- UWS 1.6 Ansible design note (historical): [docs/uws_1_6_ansible.md](docs/uws_1_6_ansible.md)
- JSON Schema: [versions/1.9.1.json](versions/1.9.1.json)

## Packages

- `uws1` contains the UWS 1.x Go model, structural vocabulary, and structural validation.
- `convert` converts UWS documents between JSON, YAML, and the HCL authoring form.
- `schemas` locates version documents and validates the separately versioned browser profiles. Go schema APIs formerly under `versions` moved here so `versions/` contains documents only.
- `validation` loads JSON, YAML, or HCL artifacts and applies versioned JSON Schema plus semantic validation.
- `contenttrust` performs explicit, deterministic advisory analysis using source- and extension-profile resolvers.
- `runtimes` contains the public `uws.runtime.1.0` supplement constants, wire structs, and extension helpers.
- `browserauthentication` contains the additive secret-free sign-in profile and named-session operation extension types.
- `browserregistration` contains the separate additive secret-free account-registration profile and explicitly approved mutation extension types.
- `versions/1.9.1.md` is the latest human-readable UWS 1.9 specification.
- `versions/1.9.1.json` is the latest JSON Schema for UWS 1.9 documents; 1.9.0 remains immutable and accepted.
- `versions/browser.1.7.*` publishes portable scalar accessibility-text conversion on top of browser 1.6 contexts; immutable browser 1.5/1.6 documents remain accepted.
- `versions/browser-authentication.1.1.*` and `versions/browser-authentication-call.1.1.*` publish context-capable sign-in recipes and explicit named-session establishment; immutable 1.0 documents remain accepted.
- `versions/browser-registration.1.0.*` and `versions/browser-registration-call.1.0.*` publish account-creation recipes with symbolic credentials, an explicit submit approval, fail-on-duplicate behavior, no ambiguous retry, and a preselected cleanup disposition.
- `versions/ansible.1.0.md` / `versions/ansible.1.0.json` are retained only with the historical UWS 1.6 contract.

The UWS-owned Ansible module-call supplement, its `ansiblemodulecall` Go package,
and its schema accessors were removed when 1.7 support was retired. UWS 1.6
documents still validate. Consumers of those historical UWS-named Go contracts
must pin an older revision; the last published pre-removal tree is commit
`a68a209`. The Ramen repository instead keeps its static conversion-only
implementation in its own `internal/ansibleconvert` package with Ramen-owned identifiers. It is not a
compatibility copy and does not accept the retired UWS Ansible identifiers.
This recovers the historical files into the current directory:

```bash
git archive a68a209 ansiblemodulecall versions/ansible.1.0.json versions/ansible.1.0.md versions/ansible-module-call.1.0.json versions/ansible-module-call.1.0.md | tar -x
```

The browser-authentication documents are separate profiles rather than part of
the UWS 1.8+ core schema. `browser-authentication.1.1` describes a reviewed,
secret-free sign-in recipe, while `browser-authentication-call.1.1` validates
the operation-level `x-uws-browser-authentication` envelope. A UWS 1.8+ document
selects that envelope through `x-uws-operation-profile`; tooling that implements
the profile then validates it with `schemas`. This separation lets the browser
profile evolve independently and keeps authentication semantics out of core
UWS parsing.

The browser-registration documents are likewise separate from both UWS core
and browser authentication. They describe an explicitly approved account-
creation mutation and its fixed duplicate, ambiguity, and cleanup controls.
They do not establish a session, carry credential values, automate human
verification, retry an ambiguous outcome, or perform cleanup.

Registration 1.1 adds typed private fields and explicit input checkpoints. Its
optional discovery metadata remains accepted for compatibility. Discovery
inventory, coverage limitations and owner review belong in authoring tools
such as OpenUdon's iCoT; new portable recipes should omit that metadata. See
[registration authoring boundaries](docs/registration-authoring.md).
A local form or editable JSON draft can collect all
inputs for one registration type, including optional and conditional fields.
The filled input envelope remains owner-private outside the package. Use
`schemas.BrowserRegistrationInputTemplate` and
`schemas.ValidateBrowserRegistrationInputUpdate` for browser-free preparation
and validation. Profile/call 1.0, the unversioned call validator and schema
lookup defaults retain their original meanings; select 1.1 explicitly.
The extension defines runtime obligations but does not itself discover pages,
provide a form UI or add execution support to a downstream browser driver.

## Validation

Use `(*uws1.Document).Validate()` when an `error` is enough, or `ValidateResult()` when callers need all path-tagged validation errors.

```go
result := doc.ValidateResult()
if !result.Valid() {
    return result
}
```

Validation checks required root fields, source operation bindings, extension-owned operation profiles, duplicate identifiers, standard request-binding keys, known structural types, selected reference integrity, action/criterion rules, and trigger routes.

`versions/1.9.1.json` provides structural JSON Schema validation. Use the Go validator for semantic checks such as duplicate identifiers, reference integrity, and malformed `contentTrust` declarations. Go callers resolve it with `schemas.PathForVersion`.

The separate `versions/runtime.1.0.json` schema validates the public runtime supplement payload. It requires `x-uws-runtime.type`, accepts only the non-HTTP runtime identifiers defined by the supplement, and rejects HTTP/API/event source metadata because HTTP and event calls are represented by core source operation binding fields.

Content-trust analysis is explicit and advisory:

```go
report, err := contenttrust.Analyze(ctx, doc, resolvers...)
```

The report contains stable edges and findings without runtime values or content excerpts. Findings do not enter `ValidationResult`, prevent execution, or alter executor results. Provenance (`trusted`, `untrusted`, `unknown`) remains separate from capability (`free_text`, `constrained_scalar`, `composite`, `unknown`).

## Execution

UWS 1.x defines a bound-runtime execution model. UWS core owns orchestration and structural execution semantics; the bound runtime owns leaf execution plus the evaluation services needed for expressions and iterative constructs.

At a high level:

- `Document.Execute(ctx)` executes the document through the orchestrator
- `Document.DispatchTrigger(ctx, triggerID, output, payload)` dispatches a trigger event into the same execution model
- `Document.ExecutionRecords()` exposes the accumulated execution snapshot
- `Runtime` is responsible for leaf execution, expression evaluation, and item resolution

Execution requires a bound runtime and a document that passes validation for execution. Trigger dispatch resolves outputs by label or decimal index and routes only to declared workflows or top-level entry-workflow steps.

## Interchange

The `convert` package provides JSON, YAML, and HCL helpers such as `JSONToHCL`, `HCLToJSON`, and `MarshalYAML`. `MarshalHCL` works on a deep copy and does not mutate the caller-owned document.

HCL conversion preserves dynamic map keys such as `$ref` through reversible key rewriting. JSON and YAML preserve `x-*` extensions through the JSON extension model; HCL represents object-level extensions with `extensions { ... }` blocks and flattens them back to `x-*` fields when converting to JSON or YAML.

Large round-trip fixtures under `testdata/big/` exercise the HCL/JSON converter with runtime supplement metadata and multi-file source references.

## Development

```bash
go test ./...
go vet ./...
```

## License

Apache License 2.0. See [LICENSE](LICENSE).
