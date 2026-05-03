# Quick Start: Author Your First UWS Workflow

This guide is for authors who need to hand-write a UWS document without reading the full specification first. It focuses on the common path: bind existing OpenAPI operations, put them in a workflow, pass values between steps, and leave concrete credentials to the runtime.

UWS is a workflow overlay. OpenAPI owns HTTP methods, paths, schemas, servers, and security. UWS owns operation binding, workflow structure, request values, outputs, triggers, and control flow.

## Start With Three Files

Keep the API contract, workflow, and runtime configuration separate:

```text
openapi/
  support.yaml
workflow.uws.yaml
runtime-config.json       # runtime-owned, not UWS core
```

The UWS document points to OpenAPI. It does not copy endpoint URLs, schemas, or credentials out of the OpenAPI document.

## Minimal Workflow

```yaml
uws: "1.1.0"
info:
  title: Support Ticket Workflow
  version: "1.0.0"

sourceDescriptions:
  - name: support_api
    url: ./openapi/support.yaml
    type: openapi

operations:
  - operationId: create_ticket
    sourceDescription: support_api
    openapiOperationId: createTicket
    request:
      body:
        subject: "Example ticket"
        priority: normal
    outputs:
      ticket_id: "$response.body.id"

  - operationId: get_ticket
    sourceDescription: support_api
    openapiOperationId: getTicket
    dependsOn: [create_ticket]
    request:
      path:
        ticketId: "$steps.create.outputs.ticket_id"
    outputs:
      status: "$response.body.status"

workflows:
  - workflowId: main
    type: sequence
    steps:
      - stepId: create
        operationRef: create_ticket
      - stepId: verify
        operationRef: get_ticket
```

This document declares:

- One OpenAPI source named `support_api`.
- Two UWS-local operations, `create_ticket` and `get_ticket`.
- One workflow named `main`.
- Two sequence steps, where `verify` can refer to the output of step `create`.

## Step 1: Declare OpenAPI Sources

Use `sourceDescriptions` for every OpenAPI document the workflow references:

```yaml
sourceDescriptions:
  - name: support_api
    url: ./openapi/support.yaml
    type: openapi
```

Rules of thumb:

- `name` is the stable local handle used by operations.
- `url` can be a local path or reviewed remote location, depending on the runtime.
- `type: openapi` keeps the source explicit.

## Step 2: Bind Operations

The most common operation binding uses `sourceDescription` plus `openapiOperationId`:

```yaml
operations:
  - operationId: create_ticket
    sourceDescription: support_api
    openapiOperationId: createTicket
```

`operationId` is local to the UWS file. `openapiOperationId` is the operation ID from the referenced OpenAPI document.

Do not add HTTP method, path, server URL, or security configuration here. Those belong to OpenAPI and the bound runtime.

## Step 3: Add Request Values

Request values are grouped by OpenAPI location:

```yaml
request:
  path:
    ticketId: "$steps.create.outputs.ticket_id"
  query:
    includeHistory: "true"
  header:
    X-Request-Source: review
  body:
    priority: normal
```

Use runtime expressions when a value comes from a previous step, workflow input, trigger, variable, or response.

## Step 4: Name Outputs

Outputs turn response values into stable handles:

```yaml
outputs:
  ticket_id: "$response.body.id"
  status: "$response.body.status"
```

Common expression roots:

- `$response.body`: response body from the current operation.
- `$response.header`: response headers from the current operation.
- `$steps.<stepId>.outputs.<name>`: output from a previous step in the same workflow.
- `$inputs.<name>`: workflow input.
- `$variables.<name>`: reusable document variable.
- `$trigger`: trigger payload during trigger dispatch.

Keep expressions simple unless your runtime documents extensions beyond UWS core.

## Step 5: Compose Workflows

A sequence workflow runs steps in order:

```yaml
workflows:
  - workflowId: main
    type: sequence
    steps:
      - stepId: create
        operationRef: create_ticket
      - stepId: verify
        operationRef: get_ticket
```

Use the six structural types when needed:

- `sequence`: run steps in declaration order.
- `parallel`: run independent steps concurrently.
- `switch`: choose a branch.
- `loop`: repeat over items.
- `merge`: join control-flow branches.
- `await`: wait until a runtime expression becomes true.

Start with `sequence`. Add other constructs only when the workflow needs them.

## Add A Branch

```yaml
workflows:
  - workflowId: classify_ticket
    type: switch
    cases:
      - name: closed
        when: "$steps.verify.outputs.status == 'closed'"
        workflow: archive_ticket
      - name: open
        when: "$steps.verify.outputs.status == 'open'"
        workflow: notify_owner
    default:
      workflow: notify_owner
```

Switch cases route to workflows. Keep branch conditions in the core expression grammar when portability matters.

## Add A Trigger

```yaml
triggers:
  - triggerId: new_ticket
    type: webhook
    outputs:
      subject: "$trigger.body.subject"
    routes:
      - when: "$outputs.subject != ''"
        to: [main]
```

Trigger ingress, authentication, hosting, and persistence are runtime-owned. UWS only describes output extraction and route dispatch after a trigger event has been accepted.

## Extension-Owned Operations

If a step is not an OpenAPI operation, make that explicit with an extension profile:

```yaml
operations:
  - operationId: render_message
    x-uws-operation-profile: "example.message_renderer.v1"
    request:
      body:
        template: "Ticket {{ticket_id}} is ready"
```

UWS validates the shape and references. The bound runtime decides whether it supports the named extension profile.

## Validation Checklist

Before handing a workflow to a runtime:

- Every `sourceDescription` name is unique.
- Every OpenAPI-bound operation names an existing source.
- Every local `operationId`, `workflowId`, and step `stepId` is unique where required.
- Every `operationRef`, `workflow`, trigger route, and dependency target resolves.
- Request values do not contain secrets.
- Credentials and endpoint policy are supplied by the runtime, not embedded in UWS.
- Extension-owned operations declare an explicit `x-uws-operation-profile`.

## Where To Go Next

- [OpenAPI Operation Binding](01-OpenAPI-Operation-Binding.md)
- [Six Structural Constructs](02-Six-Structural-Constructs.md)
- [Runtime Expression Grammar](03-Runtime-Expression-Grammar.md)
- [Triggers and Route Dispatch](04-Triggers-and-Route-Dispatch.md)
- [Validation](09-Validation.md)
