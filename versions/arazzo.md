# UWS And Arazzo

This note compares Udon Workflow Specification (UWS) v1.0 with the OpenAPI Initiative Arazzo Specification v1.0.

## Shared Direction

Both UWS and Arazzo describe workflows over API operations defined outside the workflow document. In both specifications, OpenAPI remains the source of truth for HTTP methods, paths, schemas, servers, and security schemes.

UWS is not a replacement for OpenAPI and should not duplicate OpenAPI operation metadata.

## Core Difference

Arazzo is an API-call sequencing specification. UWS is an OpenAPI-backed workflow overlay intended for workflow authoring and execution tooling such as `udon`.

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

## Source Descriptions

UWS core source descriptions are OpenAPI documents:

```json
{
  "name": "petstore_api",
  "url": "./petstore.yaml",
  "type": "openapi"
}
```

Arazzo-style source documents are not part of the UWS core profile. They can be handled by an implementation-specific extension profile if needed.

## Operation Metadata

UWS core does not define these fields on operations:

- `serviceType`
- `method`
- `path`
- `host`
- request/response schema fields
- provider or credential fields
- security requirements
- command/function runtime fields

Those details are resolved from the referenced OpenAPI document or handled by `x-*` implementation extensions.

## Structural Workflows

UWS keeps structural workflow constructs as core concepts:

- `sequence`
- `parallel`
- `switch`
- `merge`
- `loop`
- `await`

Workflow steps call operations with `operationRef`. Step `type` is reserved for structural steps, not service types.

## Extensions

UWS uses `x-*` extensions for non-core behavior. Examples include local commands, in-process functions, file I/O, SSH, SQL, LLM calls, provider credentials, or product-specific runtime metadata.

When an operation omits `sourceDescription` and OpenAPI operation binding fields, it is extension-owned and MUST include `x-uws-operation-profile` so implementations can identify which profile owns execution.

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

## Choosing Between Them

Use Arazzo when you want an OpenAPI Initiative workflow document focused on API-call sequencing.

Use UWS when you want a compact workflow interchange document for OpenAPI-backed workflow execution, structural control flow, triggers, and extension-friendly runtime profiles.
