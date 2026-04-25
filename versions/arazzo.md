# UWS And Arazzo

This note compares Udon Workflow Specification (UWS) v1.0 with the OpenAPI Initiative Arazzo Specification v1.0.

## Shared Ground

Both UWS and Arazzo describe workflows over API operations defined outside the workflow document. In both specifications, OpenAPI remains the source of truth for HTTP methods, paths, schemas, servers, and security schemes.

Neither format replaces OpenAPI. The overlap appears because both specifications try to answer a similar question: once an API contract already exists, what additional document shape is needed to describe workflow behavior over those operations?

That shared starting point does not make OpenAPI plus UWS equivalent to Arazzo. The combinations overlap, but they use different local object models and target different execution and tooling contracts.

## Why They Overlap

The strongest overlap is not accidental. Both specifications are workflow overlays over API operations described elsewhere, and in practice that usually means OpenAPI-described HTTP operations.

That overlap shows up in familiar concerns:

- selecting operations from an API description,
- binding workflow-local inputs to those operations,
- propagating outputs into later steps,
- sequencing or branching those calls, and
- validating that the workflow and API descriptions still agree.

The difference is how much of that object model each format carries locally. Arazzo carries more workflow-local structure around API invocation. UWS keeps more of the API contract in OpenAPI and keeps the workflow overlay narrower.

## Core Difference

Arazzo is an OpenAPI Initiative workflow description format for API-call sequencing. UWS is a smaller OpenAPI-backed workflow and execution contract intended for workflow authoring and execution tooling.

The narrowed UWS core is intentionally OpenAPI-first: executable core operations bind to OpenAPI, while non-OpenAPI behavior is expressed through implementation-owned `x-*` profiles.

That design has two immediate consequences:

- UWS avoids re-describing API operation metadata that OpenAPI already owns.
- UWS spends more of its core surface on structural workflow semantics and execution behavior.

## OpenAPI Reuse And Document Weight

UWS is designed on the assumption that many providers already publish the operation contract directly as OpenAPI, or can be normalized to an OpenAPI-shaped catalog before workflow authoring begins.

That leads to a smaller UWS operation shape. UWS expects OpenAPI to continue owning:

- HTTP methods,
- paths,
- request and response schemas,
- servers, and
- security schemes.

The UWS document adds the workflow overlay: selection of operations, request-value binding, control flow, triggers, and portable execution semantics.

The practical result is a lighter document. More of the API description stays where it already belongs, and the workflow layer stays focused on workflow concerns.

UWS keeps a simpler local operation model:

```json
{
  "operationId": "list_pets",
  "sourceDescription": "petstore_api",
  "openapiOperationId": "listPets",
  "request": {
    "query": {
      "limit": 10
    }
  }
}
```

The UWS `operationId` is local to the UWS document. The OpenAPI binding is carried by exactly one of:

- `openapiOperationId`
- `openapiOperationRef`

`openapiOperationRef` is a JSON Pointer fragment into the selected OpenAPI source document, for example `#/paths/~1pets/get`.

## Source Descriptions And Operation Binding

UWS core source descriptions are OpenAPI documents:

```json
{
  "name": "petstore_api",
  "url": "./petstore.yaml",
  "type": "openapi"
}
```

Arazzo-style source documents are not part of the UWS core profile. They can be handled by an implementation-specific extension profile if needed.

The UWS operation binding rule is intentionally narrow. A core operation names a `sourceDescription` and exactly one of:

- `openapiOperationId`
- `openapiOperationRef`

If an operation is not OpenAPI-bound, it becomes extension-owned and must declare `x-uws-operation-profile`.

## Request, Outputs, And Value Flow

UWS core does not define these fields on operations:

- `serviceType`
- `method`
- `path`
- `host`
- request/response schema fields
- provider or credential fields
- security requirements
- command/function runtime fields

For OpenAPI-bound operations, those details are resolved from the referenced OpenAPI document. For extension-owned operations, they belong in `x-*` implementation profiles.

This makes request binding more direct. UWS does not try to reproduce a broad API-call description object locally; instead it layers workflow-owned request values, outputs, criteria, and actions over the referenced API operation.

## Structural Workflows

UWS keeps structural workflow constructs as core concepts:

- `sequence`
- `parallel`
- `switch`
- `merge`
- `loop`
- `await`

Workflow steps call operations with `operationRef`. Step `type` is reserved for structural steps, not service types.

Structural control flow is therefore a first-class part of the UWS core model rather than an incidental property of operation sequencing.

## Execution Model

This is one of the clearest differences between the two specifications.

UWS 1.0 defines a normative execution model in `versions/1.0.0.md` §7. In that model:

- UWS core owns orchestration,
- the orchestrator owns dependency handling, structural execution, trigger routing, output propagation, and success/failure actions,
- a bound runtime owns leaf execution, expression evaluation, and iterative item resolution.

That execution split is part of the public UWS contract. It is not merely an implementation detail of one runtime.

Arazzo overlaps with UWS on workflow description, but it does not define the same compact orchestrator/runtime contract for execution. That difference matters if the workflow format is expected to serve not only as an interchange document, but also as a portable execution boundary.

## Triggers, Results, And Runtime Shape

UWS includes trigger routing and structural results as first-class workflow concepts.

Triggers provide typed entry points into the workflow graph. Results provide named outputs for structural constructs such as `switch`, `merge`, and `loop`. Together with the Section 7 execution model, that gives UWS a clearer runtime shape for hosted execution and event-driven dispatch.

Arazzo can model overlapping workflow behavior, but UWS makes these runtime-facing execution concepts more explicit in the core document.

## Extensions And Non-OpenAPI Execution

UWS uses `x-*` extensions for non-core behavior. Examples include local commands, in-process functions, file I/O, SSH, SQL, LLM calls, provider credentials, or product-specific runtime metadata.

When an operation omits `sourceDescription` and OpenAPI operation binding fields, it is extension-owned and MUST include `x-uws-operation-profile` with at least one non-whitespace character so implementations can identify which profile owns execution.

Example:

```json
{
  "operationId": "build_email",
  "x-uws-operation-profile": "udon",
  "x-udon-runtime": {
    "type": "fnct",
    "function": "mail_raw"
  }
}
```

The core schema preserves this extension but does not validate or execute it.

This keeps the core contract small while still allowing implementations to extend UWS into non-OpenAPI execution domains without changing the base document model.

## Conversion Boundary

UWS and Arazzo can be convertible for a shared subset: OpenAPI-backed operation calls, request parameter binding, simple sequencing, outputs, and success/failure criteria.

Conversion is not guaranteed for the full models. Arazzo workflow steps, reusable components, runtime expression conventions, and source document semantics do not map one-to-one to UWS top-level operations, structural workflows, triggers, results, and extension-owned operation profiles.

## Choosing Between Them

Use Arazzo when you want an OpenAPI Initiative workflow document focused on API-call sequencing and alignment with the OAI workflow format.

Use UWS when you want a smaller OpenAPI-first workflow contract with explicit execution semantics, structural control flow, triggers, results, and extension-friendly runtime profiles.
