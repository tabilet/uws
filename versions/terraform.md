# Beyond Terraform: A Client-Side Execution Contract for API Workflows

Terraform solved infrastructure-as-code. Write, plan, apply, track state — that loop is still one of the best engineering patterns we have for managing long-lived resources. Providers wrap service APIs into typed resources with lifecycle semantics, and the state file lets the engine reason about drift.

A lot of modern work, though, is not desired-state infrastructure. It is multi-step API work against services that already publish OpenAPI, Discovery, or Smithy descriptions. For that class of work, a client-side execution contract bound directly to those descriptions is a lighter path. That is what UWS and its reference runtime `github.com/tabilet/udon` explore.

## The duplication problem

A Terraform provider re-describes the API it wraps: types, fields, request shapes, authentication, retries. When the underlying service already ships an OpenAPI document, the provider schema duplicates what is already machine-readable. The result is a second release cadence between the workflow and the service, and a simplified resource surface that can hide the exact request parameters a workflow wants to use.

UWS removes the duplication by binding directly to the existing API description. The provider plugin disappears; the OpenAPI document takes its place.

## Feature spotlight 1: OpenAPI is the provider

A UWS operation names a `sourceDescription` and exactly one OpenAPI binding:

```yaml
operationId: list_reports
sourceDescription: storage_api
openapiOperationId: listObjects
request:
  query:
    bucket: daily-reports
```

The runtime loads the OpenAPI document, resolves method, path, and server URL, applies the request binding, and executes an HTTP call. `udon` implements exactly this: local OpenAPI files are parsed, operations are resolved by ID or JSON Pointer, and responses are captured into the run record.

No per-service binary. Adding support for a new API means adding its OpenAPI document to `sourceDescriptions` — not shipping a new plugin on its own release channel.

## Feature spotlight 2: Extension profiles for non-HTTP work

Not every step is an OpenAPI call. Some steps format a message, invoke an in-process function, or dispatch a sub-workflow. UWS keeps those out of the core and routes them through extension profiles:

```yaml
- operationId: build_email
  x-uws-operation-profile: udon
  x-udon-runtime:
    type: fnct
    function: mail_raw
    args:
      from: bot@example.com
      to: ops@example.com
      subject: Latest daily report
      body: $outputs.list_reports.summary
```

The `x-uws-operation-profile` marker tells the validator the operation is intentionally runtime-owned, not a missing OpenAPI binding. `udon` today runs the `fnct` profile end-to-end — dispatching into a registered function or a named sub-workflow and feeding the result back into the pipeline.

The profile boundary is where non-OpenAPI work lives. Core UWS stays small and does not try to enumerate every runtime concern; a runtime that understands a profile executes it, and documents that do not depend on the profile stay portable.

## Feature spotlight 3: Structural control flow without a DSL

Terraform's graph is a dependency DAG. That covers "run A before B" but not the control flow an API workflow actually needs. UWS names six structural constructs:

- `sequence` — steps run in declaration order.
- `parallel` — steps run concurrently, subject to `dependsOn`.
- `switch` — the first case whose `when` is truthy runs; otherwise `default`.
- `loop` — iterates over a JSON array named by `items`, with an optional `batchSize`.
- `merge` — combines outputs of upstream constructs named by `dependsOn`.
- `await` — blocks until a `wait` expression is truthy or a timeout elapses.

`await` is the interesting one. `udon` persists checkpoint state to `.udon/awaits/<token>` so a long-running workflow can suspend across process restarts and resume when the wait condition becomes truthy. That is a first-class primitive Terraform does not have — `for_each` and `count` cover fan-out, but there is no supported "pause this run for a day and resume where we left off."

The six types are finite and validated. A second implementor does not need to ask what `merge` means — it has one paragraph of normative text and one set of validator rules.

## Feature spotlight 4: Webhook triggers as real entry points

UWS triggers are not documentation. A trigger declares a path, methods, authentication, and an ordered list of named outputs; each output routes to one or more operations:

```yaml
triggers:
  - triggerId: report_webhook
    path: /webhooks/reports
    methods: [POST]
    authentication: bearer
    outputs: [received]
    routes:
      - output: received
        to: [list_reports, send_latest_report]
```

`udon`'s trigger subsystem accepts HTTP POSTs, matches them against declared routes, dispatches to the target operations, and persists trigger state under `.udon/triggers/`. One UWS document describes both the workflow and the ways in; one runtime serves both. Terraform has no equivalent — external triggers live in a separate orchestration layer.

## Feature spotlight 5: A compiled runtime plan and a run record

`udon` compiles a validated UWS document into an intermediate representation (`runtimeir.Config`) and executes it through a pipe of stages. Every run writes the executed configuration — captured status codes, response bodies, headers, and per-step timings — back through a FileIO backend as HCL/JSON files.

That is the audit substrate: a run ledger on disk, one file per execution, diffable and greppable. It is intentionally narrower than Terraform state. Terraform state answers "what do I manage and is it drifting?" A UWS run record answers "what ran, in what order, against which API descriptions, and what did the service return?" They are different questions, and the run record is the one API workflows actually need.

## Putting it together

A complete, executable UWS document for a two-API workflow with one profile-owned formatting step:

```yaml
uws: 1.0.0
info:
  title: Daily Report Dispatch
  version: 1.0.0

sourceDescriptions:
  - name: storage_api
    url: ./storage.openapi.yaml
    type: openapi
  - name: email_api
    url: ./email.openapi.yaml
    type: openapi

operations:
  - operationId: list_reports
    sourceDescription: storage_api
    openapiOperationId: listObjects
    request:
      query:
        bucket: daily-reports
    outputs:
      latest: $response.body.items[0]

  - operationId: build_email
    x-uws-operation-profile: udon
    x-udon-runtime:
      type: fnct
      function: mail_raw
      args:
        from: bot@example.com
        to: ops@example.com
        subject: Latest daily report
        body: $steps.fetch.outputs.latest
    dependsOn:
      - list_reports
    outputs:
      raw: $response.body.raw

  - operationId: send_latest_report
    sourceDescription: email_api
    openapiOperationId: sendMessage
    dependsOn:
      - build_email
    request:
      body:
        userId: me
        raw: $steps.render.outputs.raw

workflows:
  - workflowId: main
    type: sequence
    steps:
      - stepId: fetch
        operationRef: list_reports
      - stepId: render
        operationRef: build_email
      - stepId: send
        operationRef: send_latest_report
```

Storage and email stay OpenAPI-bound. The formatting step is runtime-owned. The sequence is explicit. Every identifier, reference, and expression is validated before a single HTTP call leaves the machine.

## Where UWS can improve the Terraform pattern

**Bind to the API description, don't rewrite it.** Most major services now publish OpenAPI, Google Discovery, or AWS Smithy models. UWS uses those descriptions directly as the operation surface. Terraform providers wrap them — a duplicate schema on a separate release cycle.

**Keep the operation-level request surface visible.** Provider resources abstract arguments like `maxResults`, `pageToken`, or per-request headers into simpler fields. A UWS request binding exposes exactly the OpenAPI request shape — useful when the workflow needs the precise knobs and expressions like `$outputs.previous_page.nextPageToken`.

**No per-service binary governance.** Terraform providers are standalone binaries that need pinning, provenance review, mirroring, and upgrade control in enterprise environments. OpenAPI documents can be pinned, hashed, and reviewed like any other data asset. One runtime serves many descriptions.

**Workflow primitives Terraform lacks.** `loop` with `items`, `switch` with `when`, `await` with resumable checkpoints, typed trigger entry points. Terraform has `for_each`, `count`, and `dynamic` blocks, but no first-class suspend-and-resume and no inbound webhook dispatch.

**Run history as a runtime concern, not a bolt-on.** Each execution writes the compiled plan plus the observed responses back through the pipe. That is a workflow audit trail produced by the runtime itself — distinct from Terraform's state, which is a desired-vs-actual snapshot rather than a run log.

## What this is not

UWS is not an IaC replacement. Desired-state management, drift detection, resource lifecycle, state locking, and the mature provider ecosystem around long-lived infrastructure remain Terraform's strengths. The UWS pitch is narrower: for multi-step API work against services that already publish a machine-readable description, binding directly to that description is the lighter path. Terraform should stay where it is strong; UWS takes the workflow-execution slice.

## TL;DR

- UWS binds workflows directly to OpenAPI — no provider plugin, no duplicate schema, no separate binary release cadence.
- Six structural constructs (`sequence`, `parallel`, `switch`, `loop`, `merge`, `await`) cover real API control flow; `await` supports resumable checkpoints Terraform does not offer.
- Triggers are typed entry points the runtime dispatches, not documentation.
- Extension profiles like `x-uws-operation-profile: udon` keep non-HTTP work (functions, sub-workflows) out of the core.
- `udon` compiles validated UWS into a runtime IR and writes a full run record — compiled plan plus observed responses — back to disk.
- UWS does not replace Terraform; it addresses a different class of work that today is often forced through a provider plugin it does not need.
