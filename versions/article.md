# From Arazzo to UWS: A Client-Side Execution Contract for OpenAPI

A deep dive into `github.com/tabilet/uws` — a compact, execution-oriented workflow specification that sits directly on top of the OpenAPI documents many providers already publish.

## Why UWS exists

Most API ecosystems already have the hard part: an operation catalog.

Major cloud providers, SaaS platforms, and developer tools increasingly publish OpenAPI directly. Even when a platform starts from a discovery-style API surface, the practical execution target still tends to be an OpenAPI-shaped operation catalog with stable methods, paths, parameters, schemas, and security definitions.

That creates an obvious design question:

> If the provider already publishes the operation contract, what should the workflow document still need to say?

UWS answers that by narrowing scope aggressively.

OpenAPI already owns:

- HTTP methods
- paths
- request and response schemas
- servers
- security schemes

UWS owns the workflow overlay:

- which OpenAPI operations participate in the workflow
- how they depend on each other
- how values flow between them
- which structural constructs shape control flow
- which triggers enter the workflow
- which execution semantics are portable across runtimes

That is the core reason to have UWS at all. It is not trying to replace OpenAPI, and it is not trying to become a second API-description language. It exists because "execute these existing API operations in this order, with this control flow and these data bindings" is a different problem from "describe an HTTP API."

This is also why UWS lands differently from Arazzo.

Arazzo and UWS overlap because both are workflow overlays over API operations described elsewhere. But Arazzo carries more workflow-local object model around concepts that already overlap heavily with OpenAPI, while stopping short of defining a compact, normative execution model in the way UWS now does. UWS chooses the smaller surface: bind directly to the OpenAPI you already have, keep the workflow document narrow, and define the orchestrator/runtime split explicitly.

The detailed comparison belongs in `versions/arazzo.md`. This article focuses on what UWS gives you when you take the OpenAPI-first path, and it walks the full major feature set of UWS 1.0 rather than a hand-picked sample.

## Where we left off

In an [earlier article](https://medium.com/@peterbi_91340/mastering-api-workflows-in-go-a-deep-dive-into-arazzo-40f1e37e1307) I walked through Arazzo and the `genelet/arazzo` package. Arazzo is a serious workflow description effort, and it is useful to study because it makes the tradeoffs visible.

But for client-side execution against providers that already publish OpenAPI, duplicating operation metadata inside the workflow document is often more weight than leverage. Once the OpenAPI document already tells you what the operation is, what its parameters are, and how it is secured, the workflow layer should stay focused on workflow concerns.

That is the design center of UWS.

The UWS 1.0 spec is explicit about this narrowing. Its §2.1 lists what UWS deliberately does not carry — no reusable-component registry for criteria or actions, no separate Parameter object, no payload-replacement model, no non-OpenAPI source type. Those omissions are not accidental. They are the mechanism that keeps the workflow contract compact.

## UWS in one paragraph

UWS (Udon Workflow Specification) is a compact document format for OpenAPI-backed workflows. A UWS document lists `operations` that bind by reference to OpenAPI operations, optional `workflows` that compose those operations using six structural control-flow constructs, optional `triggers` that serve as entry points, and optional `results` that name the outputs of structural constructs. Values flow between steps through a small normative expression grammar. At execution time, UWS core owns orchestration while a bound runtime owns leaf execution and expression/item evaluation. Documents are valid as JSON, YAML, or HCL, and the Go reference library — `github.com/tabilet/uws` — validates them before execution.

## A minimal document

```json
{
  "uws": "1.0.0",
  "info": { "title": "Weather Report", "version": "1.0.0" },
  "sourceDescriptions": [
    { "name": "weather_api", "url": "./weather.openapi.yaml", "type": "openapi" },
    { "name": "gmail_api",   "url": "./gmail.openapi.yaml",   "type": "openapi" }
  ],
  "operations": [
    {
      "operationId": "current_weather",
      "sourceDescription": "weather_api",
      "openapiOperationId": "getCurrentWeather",
      "request": {
        "query": { "q": "Los Angeles", "units": "imperial" }
      },
      "outputs": {
        "summary": "$response.body.summary"
      }
    },
    {
      "operationId": "send_report",
      "sourceDescription": "gmail_api",
      "openapiOperationId": "sendMessage",
      "dependsOn": ["current_weather"],
      "request": {
        "body": {
          "userId": "me",
          "text": "$steps.get_weather.outputs.summary"
        }
      }
    }
  ],
  "workflows": [
    {
      "workflowId": "main",
      "type": "sequence",
      "steps": [
        { "stepId": "get_weather", "operationRef": "current_weather" },
        { "stepId": "send_email", "operationRef": "send_report" }
      ]
    }
  ]
}
```

What is absent is the point. No HTTP method, no path, no server URL, no response schema, no security scheme. Those live in the OpenAPI documents named by `sourceDescriptions`. UWS points at the operation and describes how the workflow uses it.

## The major features of UWS 1.0

The rest of this article is deterministic: it covers the full major feature set that defines UWS 1.0 as a workflow contract.

1. strict OpenAPI operation binding
2. six structural workflow constructs
3. a normative runtime-expression grammar
4. trigger entry points and route-based dispatch
5. structural results
6. success criteria and control actions
7. a normative execution model
8. extension profiles for non-core execution
9. validation and schema/parity discipline
10. JSON, YAML, and HCL interchange

Those ten areas are the core of the format. What follows walks them in that order.

## Major feature 1 of 10: Strict OpenAPI binding

Every OpenAPI-bound UWS operation names a `sourceDescription` and exactly one binding field:

```json
{
  "operationId": "list_pets",
  "sourceDescription": "petstore_api",
  "openapiOperationId": "listPets"
}
```

or, when the OpenAPI document does not assign a stable ID, a JSON Pointer fragment resolved against the named source:

```json
{
  "operationId": "list_pets",
  "sourceDescription": "petstore_api",
  "openapiOperationRef": "#/paths/~1pets/get"
}
```

The validator rejects anything else. That single rule is what keeps the UWS document and its OpenAPI source from drifting apart silently, and it is what lets UWS stay small: if the OpenAPI document already describes the request shape, UWS does not need to.

The spec formalizes this as a three-shape table: OpenAPI-bound by `operationId`, OpenAPI-bound by JSON Pointer, or extension-owned with `x-uws-operation-profile`. Every valid operation matches exactly one shape. The schema encodes it as a `oneOf`; the Go validator re-checks it; the spec prose tabulates it. Three independent places enforcing the same invariant is not accidental — it is how UWS keeps the binding contract unambiguous for both human and machine authors.

## Major feature 2 of 10: Six structural constructs

Operations are the leaves. Workflows and steps compose them, and each workflow picks one of six structural types:

- **`sequence`** — steps run in declaration order.
- **`parallel`** — steps run concurrently, subject to `dependsOn`.
- **`switch`** — the first `case` whose `when` is truthy runs; otherwise `default`.
- **`loop`** — iterates over a JSON array named by `items`, with an optional `batchSize`.
- **`merge`** — combines outputs of upstream constructs named by `dependsOn`.
- **`await`** — blocks until its `wait` expression is truthy or a timeout elapses.

Each construct has concrete validation rules: a `loop` must have `items`, a `merge` must have at least one `dependsOn`, a `switch` must not have `items`, an `await` must have `wait`. The validator enforces these before the runtime sees the document.

Structural types are intentionally finite. A second implementor does not need to ask what `merge` means — it has one paragraph of normative text and one set of validator rules.

## Major feature 3 of 10: A normative expression grammar

Workflows pass values between steps through expressions. UWS defines a closed set of sources:

- `$response.statusCode`, `$response.body`, `$response.body#/json/pointer`, `$response.headers.<name>`
- `$outputs.<name>` — a same-scope output
- `$steps.<stepId>.outputs.<name>` — a sibling step's output
- `$variables.<name>` — a document-scope variable
- `$trigger`, `$trigger.<path>` — the trigger payload

Consumers may dot-walk into structured values, so `$steps.get_weather.outputs.summary` reaches into the named output produced by the prior step. Comparison operators (`==`, `!=`, `<`, `<=`, `>`, `>=`) are defined for `when`, `Criterion.condition`, and similar fields. Richer grammar — functions, boolean operators, templating — is explicitly implementation-defined and must not be required for core conformance.

The 1.0 spec consolidates this in §5.6 as a normative ABNF (RFC 5234) grammar. Six short interpretation notes pin down the corners that informal prose tends to leave open — operator longest-match for `==` versus `=`, the single-space whitespace rule around comparison operators, the JSON Pointer escape semantics, and the boundary between path segments (strict `[A-Za-z0-9_-]+`) and the looser character class used for output names. A runtime that implements the grammar verbatim is portable by construction.

The payoff is portability. An expression written against the UWS core means the same thing in any compliant runtime.

## Major feature 4 of 10: Triggers as typed entry points

Every workflow needs a way in. A trigger declares its path, methods, authentication, and an ordered list of named outputs. Each output routes to one or more operations:

```json
{
  "triggerId": "pet_webhook",
  "path": "/webhooks/pets",
  "methods": ["POST"],
  "outputs": ["received"],
  "routes": [
    { "output": "received", "to": ["list_pets", "create_pet"] }
  ]
}
```

A route may address its output by name or by decimal index. Routes that reference an undeclared output fail validation. Each invocation emits exactly one output. That is a small surface area, and it is enough to model webhooks, scheduled jobs, and manual invocations without extra ceremony.

## Major feature 5 of 10: Structural results, linked back to their source

The outputs of a structural construct are named explicitly in `results[]`:

```yaml
results:
  - name: validation_merge
    kind: merge
    from: parallel_checks.merge_validation
    value: $steps.merge_validation.outputs
```

Each result carries a `name`, a `kind` (one of `switch`, `merge`, `loop`), a `from` that names the emitting workflow or workflow-plus-step, and a `value` expression. The validator resolves `from` to a real workflow or step and checks that the referenced type matches `kind`. Names are unique across `results[]`.

That linkage is what turns a structural construct from "some control flow" into "a named, addressable output that downstream code can depend on."

## Major feature 6 of 10: Success criteria, retry, and goto

Real workflows need to decide whether a step actually succeeded and what to do when it did not. UWS covers both with a small vocabulary: a `Criterion` describes a condition; `successCriteria` on an operation lists the conditions that define success; `onSuccess` and `onFailure` list the actions that fire when those criteria match.

```yaml
operationId: list_pets
sourceDescription: petstore_api
openapiOperationId: listPets
successCriteria:
  - condition: $response.statusCode == 200
onFailure:
  - name: retry_on_5xx
    type: retry
    retryAfter: 1
    retryLimit: 3
    criteria:
      - condition: $response.statusCode >= 500
  - name: give_up
    type: end
```

`Criterion.type` selects how the condition is evaluated — `simple` for the normative comparison grammar, or `regex`, `jsonpath`, `xpath` for richer matching (the latter three require a `context` value telling the runtime what to apply the pattern against). `FailureAction.type` is `end`, `retry`, or `goto`; `SuccessAction.type` is `end` or `goto`. A `goto` action names exactly one of `workflowId` or `stepId` to branch to; a `retry` action must set `retryLimit` and may set `retryAfter`.

The validator enforces every one of those rules before the document sees a runtime. A retry with no `retryLimit`, a goto that names both `workflowId` and `stepId`, a regex criterion with no context — each gets a structured error pointing at the exact offending path.

Criteria and actions are declared inline. UWS intentionally does not provide a reusable-component registry for them, unlike Arazzo. Reuse is expressed through extension profiles or by duplicating the declaration. The tradeoff is deliberate: the document stays self-describing, and two sibling operations can evolve independently.

## Major feature 7 of 10: A real execution model

This is where UWS stops being just a document format.

The current `uws1` package does not merely validate workflows; it defines a real orchestrator/runtime split. UWS core owns structural orchestration. A bound runtime owns leaf execution and expression/item resolution.

At the API level, the shape is small:

```go
type Runtime interface {
    ExecuteLeaf(ctx context.Context, op *Operation) error
    EvaluateExpression(ctx context.Context, expr string) (any, error)
    ResolveItems(ctx context.Context, itemsExpr string) ([]any, error)
}
```

And execution looks like this:

```go
doc.SetRuntime(rt)
if err := doc.Execute(ctx); err != nil {
    return err
}
records := doc.ExecutionRecords()
```

That split is the practical center of UWS 1.0.

The orchestrator owns:

- document validation before execution
- indexing operations, workflows, steps, top-level trigger targets, and `parallelGroup` members
- dependency execution across operations, workflows, steps, and parallel barriers
- evaluation of `when`, `forEach`, `items`, `batchSize`, and `wait`
- execution of the six structural constructs
- application of success/failure actions such as `retry`, `goto`, and `end`
- output resolution and structural result shaping
- trigger route resolution once a trigger event has been accepted

The runtime owns:

- actual leaf execution for one operation
- expression evaluation against the current execution context
- resolving iterative item lists for `forEach` and `loop`

That line matters. It is what lets UWS define portable orchestration semantics without forcing the specification to standardize HTTP clients, storage engines, secret handling, process management, or product-specific runtime hooks.

The current execution model also carries a useful execution context into runtime hooks. At runtime, the executor can observe:

- the current trigger payload
- the current iteration item, index, batch, and batch index
- the current construct being evaluated
- the snapshot of execution records accumulated so far

So the runtime does not execute blindly. It gets a normalized view of where the orchestrator is, what has already happened, and what values are in scope.

Trigger dispatch now fits into the same model. A runtime accepts ingress however it likes, but once a trigger event has been accepted, UWS core resolves the trigger output, validates the routed targets, exposes `$trigger`, and executes the targets through the same orchestrator rules used for normal workflow execution.

That is the point where UWS diverges most clearly from "a workflow-shaped document." The document is only half the contract. The execution model is the other half.

## Major feature 8 of 10: Extension profiles for non-HTTP work

Not every step is an HTTP call. Formatting a message, invoking a function, running a local command, or calling a language model are common needs. UWS keeps those out of the core and uses extension profiles instead:

```yaml
- operationId: build_email
  x-uws-operation-profile: udon
  x-udon-runtime:
    type: fnct
    function: mail_raw
    args:
      from: bot@example.com
      to: user@example.com
      subject: Daily weather report
      body: $steps.get_weather.outputs.summary
```

The `x-uws-operation-profile` marker tells the validator that this operation is intentionally runtime-owned, not a missing OpenAPI binding. Any runtime that understands the profile can execute it; UWS core stays small and declines to specify what `type: fnct` means.

The effect is that a single UWS document can describe a complete task — an OpenAPI call, a runtime-owned formatting step, another OpenAPI call — without pretending anything is something it is not.

Spec 1.0 reserves the `x-uws-` prefix for future core evolution — `x-uws-operation-profile` is the only 1.0 occupant, but anything else under that prefix belongs to UWS core. Third parties use their own prefix (`x-udon-`, `x-<vendor>-`) and their extension is governed entirely by the profile that defines it. Conforming tooling preserves unknown `x-*` fields on round-trip and must not silently drop or rename them.

## Major feature 9 of 10: Validation that fails fast

`github.com/tabilet/uws` layers two kinds of validation:

- **Structural**, via the published JSON Schema (`uws.json`). This catches shape errors, type errors, and required-field violations.
- **Semantic**, via `(*uws1.Document).Validate()`. This catches duplicate identifiers, unknown references (an operation pointing at a missing `sourceDescription`, a step referencing an undeclared workflow), binding-rule violations, unresolved trigger-route outputs, structural-type mistakes, and `results[]` linkage errors.

Parsing and validating a document is a few lines:

```go
package main

import (
    "fmt"
    "log"
    "os"

    "github.com/tabilet/uws/convert"
    "github.com/tabilet/uws/uws1"
)

func main() {
    data, err := os.ReadFile("workflow.uws.yaml")
    if err != nil {
        log.Fatal(err)
    }

    var doc uws1.Document
    if err := convert.UnmarshalYAML(data, &doc); err != nil {
        log.Fatal(err)
    }

    if result := doc.ValidateResult(); !result.Valid() {
        for _, issue := range result.Errors {
            fmt.Printf("%s: %s\n", issue.Path, issue.Message)
        }
        return
    }

    fmt.Println("document is valid")
}
```

`ValidateResult` returns every error with a dotted path. That matters when a language model produces the workflow: you want to tell it exactly which field was wrong, not just that something was. Each `ValidationError` is a `{Path, Message}` pair — structured enough to show an end user, machine-readable enough to hand back to the model that produced the invalid document, keyed precisely enough that a diff tool can suggest a single-line fix.

The three artifacts that define UWS — the JSON Schema (`uws.json`), the Go types (`uws1/`), and the spec prose (`versions/1.0.0.md`) — are kept in sync by a reflection-driven test suite. A parity test walks every Go struct with an `Extensions` field, checks its JSON tags against the `knownFields` list its unmarshaller uses, and compares both against the `$def` in the schema. A driven-conformance test reads `uws.json` and asserts that every `required`, `enum`, and `pattern` rule the schema declares is also declared in a Go-side coverage table. Adding a property to one place and forgetting another fails the build.

That is unusual for a spec of this size, and it is the mechanical reason a third-party implementer can trust that the schema and the prose will not quietly disagree.

## Major feature 10 of 10: JSON, YAML, and HCL — pick the format that fits the moment

The `convert` package moves documents between three formats: JSON for interchange, YAML for readability, HCL for authoring. Converters are symmetric and round-trippable, with one intentional asymmetry: HCL conversion rejects documents that carry `x-*` extensions, because HCL has no canonical place to put them. JSON and YAML preserve extensions through the round-trip.

Authoring the same operation in HCL:

```hcl
operation "list_pets" {
  sourceDescription  = "petstore_api"
  openapiOperationId = "listPets"

  request = {
    query = { limit = 10 }
  }
}
```

Rejecting lossy conversion rather than silently dropping data is a small choice that prevents a large class of bugs.

## Putting it together

Here is a complete UWS document that exercises most of the major feature set above: OpenAPI-backed operations, a profile-owned runtime step, explicit dependencies, structured outputs, and a sequential workflow.

```yaml
uws: 1.0.0
info:
  title: Daily Weather Report
  version: 1.0.0

sourceDescriptions:
  - name: weather_api
    url: ./weather.openapi.yaml
    type: openapi
  - name: gmail_api
    url: ./gmail.openapi.yaml
    type: openapi

operations:
  - operationId: current_weather
    sourceDescription: weather_api
    openapiOperationId: getCurrentWeather
    request:
      query:
        q: Los Angeles
        units: imperial
    outputs:
      summary: $response.body.summary

  - operationId: build_email
    x-uws-operation-profile: udon
    x-udon-runtime:
      type: fnct
      function: mail_raw
      args:
        from: bot@example.com
        to: user@example.com
        subject: Daily weather report for Los Angeles
        body: $steps.get_weather.outputs.summary
    dependsOn:
      - current_weather
    outputs:
      raw: $response.body.raw

  - operationId: send_report
    sourceDescription: gmail_api
    openapiOperationId: sendMessage
    dependsOn:
      - build_email
    request:
      body:
        userId: me
        raw: $steps.render_email.outputs.raw

workflows:
  - workflowId: main
    type: sequence
    steps:
      - stepId: get_weather
        operationRef: current_weather
      - stepId: render_email
        operationRef: build_email
      - stepId: send_email
        operationRef: send_report
```

Weather and Gmail stay OpenAPI-bound. Email formatting is runtime-owned. The sequence is explicit. The document is fully validatable before any HTTP call leaves the machine.

## What UWS hands off to the runtime

UWS is explicit about where its contract ends and the runtime's job begins. A conforming validator decides whether a document is well-formed and internally consistent; it does not decide whether the document will succeed in practice. The orchestrator owns portable workflow semantics, but concrete execution still belongs to the runtime.

In particular, the following remain runtime concerns:

- **HTTP transport** — URL construction, serialization, connection pooling, TLS, redirect handling, and concrete retry timing beyond the declarative `retryAfter`.
- **Provider-specific authentication** — OpenAPI says what the operation requires; the runtime decides how credentials are stored, injected, rotated, and refreshed.
- **Expression engine implementation** — UWS fixes the meaning of the core expression grammar; the runtime still implements the evaluator.
- **Worker sizing and resource scheduling** — UWS core owns `dependsOn`, `parallel`, `parallelGroup`, `loop`, and trigger-routing semantics, but thread pools, queues, and backpressure are executor choices.
- **Persistence and resumption policy** — execution records and trigger state can be exposed and persisted by a runtime, but storage layout and resume mechanics are not part of the UWS wire contract.
- **Extension profile semantics** — `x-uws-operation-profile: udon` tells the validator the operation is intentionally runtime-owned; the profile itself decides what `x-udon-runtime` means.

The division is the whole design. The document is portable because everything portable is in the document and the executor-specific machinery stays out of it.

## Where UWS fits for AI agents

Language models are good at reading intent and filling templates. They are less reliable at planning a multi-step API sequence from a large OpenAPI catalog and then executing it safely.

UWS narrows the surface. An agent extracts intent, materializes a UWS document (from a template or freshly generated), validates it with the Go library, and hands the validated document to a deterministic runtime:

```text
natural language  ->  structured intent  ->  UWS document  ->  Validate()  ->  execution
```

The model proposes; the runtime executes. Invalid workflows are caught at the contract boundary, not inside a live API session where recovery is expensive.

The path-tagged error shape is what closes the loop. When the model produces a workflow with, say, a retry action missing `retryLimit` and a `sourceDescription` pointing at a source the agent never included, `ValidateResult` returns something like:

```text
operations[0].onFailure[0]: retry requires retryLimit > 0
operations[1].sourceDescription: references unknown sourceDescription "gmail_api"
```

That output drops straight into the model's next prompt. Two fields, two exact paths, two sentences — enough for a corrective pass that converges in one or two turns instead of walking the model through prose.

For an agent, this is the difference between "the model guessed a tool call" and "the model produced a workflow contract the runtime can validate and execute."

## TL;DR

- UWS exists because many providers already publish OpenAPI, and the workflow layer should not re-describe what OpenAPI already owns.
- UWS is a compact, client-side execution contract for OpenAPI-backed workflows.
- Operations bind strictly to OpenAPI — no duplication of methods, paths, or schemas.
- Every valid operation matches exactly one of three shapes: OpenAPI-by-id, OpenAPI-by-ref, or extension-owned with `x-uws-operation-profile`.
- Six structural constructs (`sequence`, `parallel`, `switch`, `loop`, `merge`, `await`) cover real control flow; each has concrete validation rules.
- Success criteria and failure handling (`retry`, `goto`, `end`) are first-class and inline — no separate action registry.
- A small normative expression grammar, formalized in ABNF, keeps workflows portable across runtimes.
- UWS defines a real orchestrator/runtime split rather than stopping at document shape alone.
- Triggers, structural results, and extension profiles complete the model; `x-uws-` is reserved for core evolution.
- `github.com/tabilet/uws` ships the Go model, the JSON Schema, JSON/YAML/HCL conversion, and a semantic validator that returns structured, path-tagged errors.
- For AI agents, UWS is the contract between intent extraction and deterministic execution.
