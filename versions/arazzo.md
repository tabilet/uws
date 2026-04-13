# UWS and Arazzo: A Detailed Comparison

This document describes the relationship and differences between the **Udon Workflow Specification (UWS) 1.0** and the **OpenAPI Initiative Arazzo Specification 1.0**.

Both specifications define machine-readable formats for describing sequences of operations. UWS draws inspiration from Arazzo's document structure and extension mechanism, but the two specifications serve different domains and make different design trade-offs.

## Overview

| | Arazzo 1.0 | UWS 1.0 |
|---|---|---|
| **Purpose** | API call sequencing | Multi-service workflow orchestration |
| **Protocol scope** | HTTP only (via OpenAPI) | HTTP, SSH, Cmd, Fnct, and 15+ other service types |
| **Operation definition** | Referenced from external OpenAPI specs | Defined inline, self-contained |
| **Control flow** | Sequential steps | Sequence, parallel, switch, merge, loop, await |
| **Error handling** | First-class success/failure actions with retry | Operation-level success criteria and success/failure actions |
| **Source documents** | Required (at least one) | Optional (provenance tracking) |

---

## Document Root

### Arazzo Object

```json
{
    "arazzo": "1.0.0",
    "info": { ... },
    "sourceDescriptions": [ ... ],
    "workflows": [ ... ],
    "components": { ... }
}
```

| Field | Required | Description |
|-------|----------|-------------|
| arazzo | Yes | Specification version |
| info | Yes | Metadata |
| sourceDescriptions | Yes | At least one OpenAPI or Arazzo source |
| workflows | Yes | At least one workflow |
| components | No | Reusable inputs, parameters, actions |

### UWS Object

```json
{
    "uws": "1.0.0",
    "info": { ... },
    "sourceDescriptions": [ ... ],
    "provider": { ... },
    "variables": { ... },
    "operations": [ ... ],
    "workflows": [ ... ],
    "triggers": [ ... ],
    "security": [ ... ],
    "results": [ ... ],
    "components": { ... }
}
```

| Field | Required | Description |
|-------|----------|-------------|
| uws | Yes | Specification version |
| info | Yes | Metadata |
| sourceDescriptions | No | Source OpenAPI or Arazzo documents |
| provider | No | Default service endpoint |
| variables | No | Global variables |
| operations | Yes | At least one operation |
| workflows | No | Workflow control-flow constructs |
| triggers | No | Entry points, such as webhooks |
| security | No | Global security requirements |
| results | No | Structural result declarations |
| components | No | Reusable operations, security schemes, variables |

**Key differences:**

- Arazzo requires `sourceDescriptions` and `workflows` at the root. UWS requires `operations`.
- UWS adds `provider`, `variables`, `triggers`, `security`, and `results` at the root level. These concepts do not exist in Arazzo.
- In Arazzo, `operations` live in the referenced OpenAPI documents. In UWS, operations are first-class, inline objects.

---

## Source Descriptions

Both specifications use a `sourceDescriptions` array to reference external documents (typically OpenAPI specifications). The objects are structurally identical:

| Field | Arazzo | UWS |
|-------|--------|-----|
| name | Required, pattern `^[A-Za-z0-9_\-]+$` | Required, pattern `^[A-Za-z0-9_\-]+$` |
| url | Required, URI reference | Required, URI reference |
| type | `openapi` or `arazzo` | `openapi` or `arazzo` |

The critical difference is that Arazzo **requires** at least one source description — every Arazzo workflow must reference an external API specification. UWS makes source descriptions **optional** — they serve as provenance metadata linking operations back to the specs they were generated from, but operations are fully self-contained and executable without them.

---

## Operations vs. Steps

This is the most fundamental architectural difference between the two specifications.

### Arazzo: Operations Are External References

Arazzo does not define operations. Instead, it references operations defined in external OpenAPI documents via one of three mechanisms:

| Reference style | Example | Description |
|-----------------|---------|-------------|
| `operationId` | `"listPets"` | Named reference to an OpenAPI operation |
| `operationPath` | `"$sourceDescriptions.petstore#/paths/~1pets/get"` | JSON Pointer into a source document |
| `workflowId` | `"$sourceDescriptions.other#/workflows/login"` | Reference to another workflow |

Each Arazzo step uses exactly one of these three. The operation's method, path, parameters, and schemas are all inherited from the source OpenAPI document.

```json
{
    "stepId": "get_pets",
    "operationId": "listPets",
    "parameters": [
        { "name": "limit", "in": "query", "value": 10 }
    ],
    "successCriteria": [
        { "condition": "$statusCode == 200" }
    ]
}
```

### UWS: Operations Are Self-Contained

UWS defines operations inline with all details explicit:

```json
{
    "operationId": "list_pets",
    "serviceType": "http",
    "sourceDescription": "petstore_api",
    "method": "GET",
    "path": "/pets",
    "queryPars": {
        "type": "object",
        "properties": {
            "limit": { "type": "integer" }
        }
    },
    "request": { "limit": 10 },
    "responseStatusCode": 200
}
```

UWS operations carry their own method, path, parameter schemas, response schemas, and security requirements. The `sourceDescription` field is optional metadata that traces provenance but does not affect execution.

**Trade-off:** Arazzo avoids duplication by referencing external specs, but requires those specs to be available. UWS operations are portable and self-describing, but may duplicate information already present in an OpenAPI document.

---

## Service Type Support

### Arazzo

Arazzo implicitly supports only HTTP operations. All steps ultimately reference OpenAPI operations, which describe HTTP endpoints.

### UWS

UWS supports multiple service types, each with its own required fields:

| Service Type | Required Fields | Description |
|--------------|-----------------|-------------|
| `http` | `method` | HTTP/HTTPS API call |
| `ssh` | `command` | Remote command over SSH |
| `cmd` | `command` | Local process execution |
| `fnct` | `function` | In-process function invocation |

Additional types are defined for forward compatibility: `fileio`, `ftp`, `sftp`, `scp`, `smtp`, `dns`, `sql`, `s3`, `docker`, `ldap`, `llm`, `mcp`, `ioreadcloser`, `ioreadwritecloser`, `iowriter`.

A single UWS workflow can mix service types:

```json
{
    "operations": [
        { "operationId": "fetch_data",   "serviceType": "http", "method": "GET", "path": "/data" },
        { "operationId": "process_data", "serviceType": "fnct", "function": "transform.Apply" },
        { "operationId": "upload_file",  "serviceType": "cmd",  "command": "aws", "arguments": ["s3", "cp", "out.csv", "s3://bucket/"] }
    ]
}
```

This multi-protocol capability has no equivalent in Arazzo.

---

## Control Flow

### Arazzo: Sequential Steps

Arazzo workflows execute steps in order. The only flow control is:

- **`dependsOn`** on the Workflow object (to wait for other workflows)
- **`onSuccess`/`onFailure`** actions with `type: "goto"` (to jump to a step or workflow)
- **Nested workflows** via `workflowId` on a step

There are no native parallel, conditional, loop, or branching constructs. Complex control flow must be modeled through goto actions and workflow composition.

### UWS: Rich Structural Constructs

UWS provides six workflow types as first-class constructs:

| Type | Description |
|------|-------------|
| `sequence` | Execute steps in order |
| `parallel` | Execute steps concurrently |
| `switch` | Branch on conditions via `cases` and `default` |
| `merge` | Combine results from parallel branches |
| `loop` | Iterate over a collection with optional `batchSize` |
| `await` | Wait for asynchronous steps to complete |

Additionally, UWS operations support:

- **`when`** — conditional execution (skip if expression is false)
- **`forEach`** — iterate the operation over a collection
- **`wait`** — delay before execution
- **`parallelGroup`** — assign operations to concurrent groups
- **`dependsOn`** — operation-level dependency ordering

```json
{
    "workflowId": "route_by_type",
    "type": "switch",
    "items": "$operations.list_pets.response.body",
    "cases": [
        {
            "name": "dog",
            "when": "item.type == 'dog'",
            "steps": [
                { "stepId": "process_dog", "type": "http", "operationRef": "create_pet" }
            ]
        }
    ],
    "default": [
        { "stepId": "log_unknown", "type": "cmd", "body": { "command": "echo" } }
    ]
}
```

---

## Error Handling and Success Criteria

### Arazzo: First-Class Error Handling

Arazzo provides explicit objects for validating outcomes and handling errors:

**Criterion Object** — validates step success:

```json
{
    "successCriteria": [
        { "condition": "$statusCode == 200" },
        { "condition": "$response.body#/status", "type": "regex", "context": "^(active|pending)$" }
    ]
}
```

Criteria support four expression types: `simple`, `regex`, `jsonpath`, `xpath`. Expression types can specify versions (e.g., `draft-goessner-dispatch-jsonpath-00`).

**SuccessAction / FailureAction Objects** — define what happens after success or failure:

```json
{
    "onFailure": [
        {
            "name": "retry_step",
            "type": "retry",
            "retryAfter": 5,
            "retryLimit": 3
        }
    ],
    "onSuccess": [
        {
            "name": "next_workflow",
            "type": "goto",
            "workflowId": "process_results"
        }
    ]
}
```

Actions can be conditional (with their own `criteria`) and support three types: `end`, `goto`, `retry`. Retry actions support `retryAfter` (seconds) and `retryLimit`. Actions can be defined at the workflow level (as defaults) and overridden per step.

### UWS: Operation-Level Error Handling

UWS defines success criteria, success actions, and failure actions on operations. These provide portable operation-level checks and routing while keeping workflow and step objects focused on control-flow structure.

- **`successCriteria`** — explicit checks evaluated after an operation runs
- **`onFailure`** — operation-level `end`, `goto`, or `retry` actions
- **`onSuccess`** — operation-level `end` or `goto` actions
- **`responseStatusCode`** — the expected HTTP status code for HTTP operations

**Trade-off:** Arazzo provides step-, workflow-, and component-scoped action objects. UWS currently keeps actions operation-local, which is simpler for multi-protocol execution but less reusable than Arazzo's handler model.

---

## Parameters and Request Bodies

### Arazzo: Parameter Objects with Location

Arazzo uses explicit Parameter objects with an `in` field:

```json
{
    "parameters": [
        { "name": "petId", "in": "path", "value": "$steps.create.outputs.id" },
        { "name": "limit", "in": "query", "value": 10 },
        { "name": "Authorization", "in": "header", "value": "Bearer $inputs.token" }
    ]
}
```

Request bodies are separate objects with support for targeted modifications:

```json
{
    "requestBody": {
        "contentType": "application/json",
        "payload": { "name": "Buddy", "tag": "dog" },
        "replacements": [
            { "target": "/name", "value": "$steps.get_name.outputs.name" }
        ]
    }
}
```

The `replacements` field allows surgical modification of a payload using JSON Pointers, without restating the entire body.

### UWS: Schema Objects with Flat Request Map

UWS uses separate schema objects for each parameter location:

```json
{
    "queryPars": {
        "type": "object",
        "properties": {
            "limit": { "type": "integer" }
        }
    },
    "headerPars": {
        "type": "object",
        "properties": {
            "Authorization": { "type": "string" }
        }
    },
    "request": {
        "limit": 10,
        "Authorization": "Bearer token123"
    }
}
```

UWS separates parameter _schemas_ (`queryPars`, `pathPars`, `headerPars`, `cookiePars`, `payloadPars`) from parameter _values_ (`request`). There is no equivalent to Arazzo's `replacements` — the entire request body is specified or omitted.

---

## Security

### Arazzo

Arazzo does not define security. Authentication and authorization are inherited from the referenced OpenAPI documents' security schemes.

### UWS

UWS defines security as a first-class concept with three object types:

**Security Requirement** — names a scheme and its scopes:

```json
{
    "name": "bearer_auth",
    "scheme": {
        "type": "http",
        "scheme": "bearer"
    }
}
```

**Security Scheme** — defines the mechanism (supports `http`, `apiKey`, `oauth2`):

```json
{
    "type": "apiKey",
    "name": "X-API-Key",
    "in": "header"
}
```

**OAuth Flows** — configures OAuth2 flows (password, implicit, authorizationCode, clientCredentials):

```json
{
    "type": "oauth2",
    "flows": {
        "authorizationCode": {
            "authorizationUrl": "https://auth.example.com/authorize",
            "tokenUrl": "https://auth.example.com/token",
            "scopes": { "read": "Read access" }
        }
    }
}
```

Security can be specified globally (applies to all HTTP operations) or per operation (overrides global).

---

## Triggers

UWS defines trigger objects for workflow entry points. Arazzo has no equivalent concept.

```json
{
    "triggerId": "pet_webhook",
    "path": "/webhooks/pets",
    "methods": ["POST"],
    "authentication": "bearer",
    "routes": [
        { "output": "0", "to": ["list_pets", "create_pet"] }
    ]
}
```

Triggers define how external events, such as webhooks, initiate workflow execution and route their outputs to specific operations.

---

## Provider

UWS defines a Provider object for default service configuration. Arazzo has no equivalent — service endpoints are part of the referenced OpenAPI documents.

```json
{
    "name": "petstore",
    "serverUrl": "https://api.petstore.example.com",
    "appendices": {
        "region": "us-east-1"
    }
}
```

Providers can be set at the document level (default for all operations) or per operation (override).

---

## Components

Both specifications support reusable components, but with different scopes:

| Component Type | Arazzo | UWS |
|----------------|--------|-----|
| inputs (JSON Schema) | Yes | No |
| parameters | Yes | No |
| successActions | Yes | No |
| failureActions | Yes | No |
| operations | No | Yes |
| securitySchemes | No | Yes |
| variables | No | Yes |

Arazzo components are referenced via runtime expressions (`$components.parameters.myParam`) and support value overrides at the reference site. In the UWS beta, component map keys are storage keys: `operationRef` resolves by the referenced operation's `operationId`, and `SecurityRequirement.name` does not automatically resolve `components.securitySchemes`.

---

## Runtime Expressions

### Arazzo

Arazzo defines a formal expression syntax:

| Expression | Description |
|------------|-------------|
| `$method` | HTTP method of the current step |
| `$url` | Full URL of the current step |
| `$statusCode` | HTTP status code of the response |
| `$request.header.{name}` | Request header value |
| `$request.query.{name}` | Request query parameter |
| `$request.body#/json/pointer` | Value from request body via JSON Pointer |
| `$response.header.{name}` | Response header value |
| `$response.body#/json/pointer` | Value from response body via JSON Pointer |
| `$inputs.{name}` | Workflow input value |
| `$outputs.{name}` | Current workflow output |
| `$steps.{stepId}.outputs.{name}` | Output from a previous step |
| `$workflows.{id}.outputs.{name}` | Output from a dependent workflow |
| `$sourceDescriptions.{name}` | Source description reference |
| `$components.{type}.{name}` | Component reference |

### UWS

UWS uses runtime expressions in `when`, `forEach`, `wait`, `workflow`, `outputs`, and `successCriteria` fields. It extends Arazzo-style runtime expressions for multi-protocol operations:

| Expression | Description |
|------------|-------------|
| `$response.body#/json/pointer` | Value from response body via JSON Pointer |
| `$request.body#/json/pointer` | Value from request body via JSON Pointer |
| `$inputs.{name}` | Workflow input value |
| `$steps.{stepId}.outputs.{name}` | Output from a previous step |
| `$operations.{operationId}.outputs.{name}` | Output from a named operation |
| `length(...)` | Built-in functions |
| Comparisons | `<`, `>`, `==`, etc. |

UWS uses JSON Pointers for request and response bodies. UWS documents generated from HCL may also contain HCL-compatible expression strings in runtime expression fields; such expressions require a runtime that supports those functions and evaluation rules.

---

## Workflow Object Comparison

The word "Workflow" means different things in each specification:

### Arazzo Workflow

An Arazzo Workflow is an ordered sequence of steps with optional inputs, outputs, and default error handlers:

```json
{
    "workflowId": "create_and_verify",
    "summary": "Create a pet and verify it exists",
    "inputs": {
        "type": "object",
        "properties": {
            "petName": { "type": "string" }
        }
    },
    "steps": [
        {
            "stepId": "create",
            "operationId": "createPet",
            "requestBody": {
                "payload": { "name": "$inputs.petName" }
            },
            "outputs": { "petId": "$response.body#/id" }
        },
        {
            "stepId": "verify",
            "operationId": "getPet",
            "parameters": [
                { "name": "petId", "in": "path", "value": "$steps.create.outputs.petId" }
            ],
            "successCriteria": [
                { "condition": "$statusCode == 200" }
            ]
        }
    ],
    "outputs": {
        "createdId": "$steps.create.outputs.petId"
    }
}
```

### UWS Workflow

A UWS Workflow is a structural control-flow construct — it describes _how_ steps execute rather than _what_ they do:

```json
{
    "workflowId": "parallel_checks",
    "type": "parallel",
    "dependsOn": ["create_pet"],
    "steps": [
        {
            "stepId": "validate",
            "type": "http",
            "operationRef": "create_pet"
        },
        {
            "stepId": "log",
            "type": "cmd",
            "body": { "command": "echo", "arguments": ["done"] }
        }
    ]
}
```

| Aspect | Arazzo Workflow | UWS Workflow |
|--------|----------------|--------------|
| Purpose | Sequence of API calls | Control-flow pattern |
| Required fields | `workflowId`, `steps` | `workflowId`, `type` |
| Type field | None (always sequential) | `sequence`, `parallel`, `switch`, `merge`, `loop`, `await` |
| Inputs | JSON Schema for workflow parameters | JSON Schema for workflow parameters |
| Success/failure handlers | Yes (workflow-level defaults) | No at workflow level; operation-level actions are supported |
| Outputs | Yes (map to expressions) | Yes (map to expressions) |
| Steps | At least one required | Optional (switch uses `cases`) |
| Branching | Via goto actions | Via `cases` and `default` |
| Looping | Via goto actions | Via `type: "loop"` with `items`, `batchSize` |

---

## Step Object Comparison

| Field | Arazzo Step | UWS Step |
|-------|------------|----------|
| stepId | Required | Required |
| type | Not present | Required (service type or structural) |
| operationId | Exactly one of operationId, operationPath, workflowId | Not present |
| operationPath | JSON Pointer into source description | Not present |
| operationRef | Not present | Reference to operation by operationId |
| workflowId | Nested workflow reference | Not present |
| body | Not present | Inline step definition |
| description | CommonMark description | Not present |
| parameters | Explicit Parameter objects | Not present (uses operation's request) |
| requestBody | Explicit RequestBody with replacements | Not present |
| successCriteria | Criterion array | Not present on steps; present on operations |
| onSuccess | SuccessAction array | Not present on steps; present on operations |
| onFailure | FailureAction array | Not present on steps; present on operations |
| outputs | Map to runtime expressions | Map to runtime expressions |
| when | Not present | Conditional expression |
| forEach | Not present | Iteration expression |
| wait | Not present | Wait duration |
| workflow | Not present | Sub-workflow reference |
| parallelGroup | Not present | Parallel group assignment |
| steps | Not present | Nested sub-steps (recursive) |
| cases | Not present | Nested switch cases (recursive) |
| default | Not present | Nested default branch (recursive) |

Arazzo steps are lean references with rich error handling. UWS steps are recursive structural nodes with rich control flow.

---

## Summary of Design Philosophies

### Arazzo

Arazzo is designed for **API-first workflow orchestration**. It assumes:

- All operations are HTTP API calls defined in external OpenAPI documents.
- The specification references external definitions rather than restating them.
- Error handling and validation are first-class concerns — every step can declare success criteria and have explicit success/failure actions with retry support.
- Control flow is simple (sequential) — complex patterns are achieved through workflow composition and goto actions.
- The specification is prescriptive about runtime behavior: conforming engines must implement criteria evaluation, action handling, and expression resolution.

### UWS

UWS is designed for **multi-protocol workflow orchestration**. It assumes:

- Operations span multiple protocols (HTTP, SSH, commands, functions, and more).
- Operations are self-contained and carry their own schemas and configuration.
- Control flow is a first-class concern — parallel execution, branching, looping, and merging are native structural constructs.
- Operation-level error handling is specified with success criteria and success/failure actions.
- Security, triggers, and providers are part of the workflow definition rather than external concerns.
- The specification is descriptive rather than prescriptive about full runtime behavior — it defines what to execute while leaving engine-specific edge-case policies to implementations.

### Expression Language and HCL

A critical difference not visible in the JSON/YAML serialization is that UWS workflows are **authored in HCL**, not JSON.

Both Arazzo and UWS use runtime expressions — strings like `$statusCode == 200` or `length(items) > 0` that are evaluated at execution time. But the two specifications differ fundamentally in where expressiveness lives:

**Arazzo** is a pure JSON/YAML specification. Its runtime expressions are a defined grammar within the spec itself (e.g., `$response.body#/path`, `$steps.id.outputs.name`). The expression language is simple and fully specified — any conforming runtime must evaluate these expressions identically.

**UWS** is a JSON/YAML **interchange format** for workflows whose canonical source is [HCL](https://github.com/hashicorp/hcl). HCL provides capabilities that JSON cannot express:

| Capability | HCL | JSON |
|---|---|---|
| Functions | `length(items)`, `upper(name)` | Not expressible |
| Variable references | `var.timeout`, `local.region` | Not expressible |
| String interpolation | `"api-${var.env}"` | Not expressible |
| Conditional expressions | `var.env == "prod" ? 443 : 8080` | Not expressible |
| `for` expressions | `[for x in items : x.id]` | Not expressible |
| Nested blocks with logic | HCL `Body` with embedded expressions | Flattened to literal map |

When a UWS document is generated from HCL:

- **Expression fields** (`when`, `forEach`, `wait`, `workflow`) are preserved as opaque strings — e.g., `"length(items) > 0"`. A runtime must use an HCL-compatible evaluator to execute them.
- **Body values** (`request`, `variables`, trigger `options`) are **statically evaluated** — HCL expressions, variable references, and function calls are resolved to literal JSON values. The dynamic semantics are lost.

This means a UWS JSON document is a **materialized snapshot**: it captures the fully-resolved structure but not the dynamic logic of the original HCL source. Arazzo documents, by contrast, are the canonical source — there is no "richer" authoring format behind them.

This distinction affects tooling:

| Concern | Arazzo | UWS |
|---|---|---|
| Can a document be round-tripped losslessly? | Yes (JSON is the source) | Partially (expression fields yes, body expressions no) |
| Can a document be authored directly in JSON? | Yes | Possible but limited — no functions, variables, or conditionals in body values |
| Is an HCL parser needed at runtime? | No | Yes, if expression fields use HCL syntax |
| Is the document self-sufficient? | Yes | For interchange and validation, yes; for full dynamic behavior, the HCL source is authoritative |

### When to Use Each

| Scenario | Recommendation |
|----------|----------------|
| Orchestrating OpenAPI-described REST APIs | Arazzo |
| Mixing HTTP calls with shell commands, functions, or file operations | UWS |
| Specifying step/workflow-level retry policies and reusable success criteria handlers | Arazzo |
| Specifying operation-level retry policies and success criteria in a multi-protocol document | UWS |
| Parallel execution, conditional branching, or loop patterns | UWS |
| Documenting API integration sequences for external consumers | Arazzo |
| Defining internal automation workflows with multiple service types | UWS |
| Workflows that must reference and stay in sync with OpenAPI specs | Arazzo |
| Self-contained, portable workflow definitions | UWS |
