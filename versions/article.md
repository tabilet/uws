# From Arazzo to UWS: A Client-Side Execution Contract for OpenAPI

A deep dive into `github.com/tabilet/uws` — a compact, execution-oriented workflow specification that sits directly on top of the OpenAPI documents you already have.

## Where we left off

In an [earlier article](https://medium.com/@peterbi_91340/mastering-api-workflows-in-go-a-deep-dive-into-arazzo-40f1e37e1307) I walked through Arazzo and the `genelet/arazzo` package. Arazzo is an excellent description standard, but in practice a lot of what an Arazzo workflow carries — operation shapes, parameters, response structures — is already described by the underlying OpenAPI document. That duplication is useful for a portable recipe, less useful when you simply want to execute against an OpenAPI service that already exists.

And OpenAPI is what already exists. Every major cloud, SaaS, and developer platform ships OpenAPI or a close variant. A client-side execution contract that binds directly to those documents — without re-describing them — is a lighter, more pragmatic way to run multi-step API tasks.

That is what UWS is. This article focuses on what it can do.

## UWS in one paragraph

UWS (Udon Workflow Specification) is a compact document format for OpenAPI-backed workflows. A UWS document lists `operations` that bind by reference to OpenAPI operations, optional `workflows` that compose those operations using six structural control-flow constructs, optional `triggers` that serve as entry points, and optional `results` that name the outputs of structural constructs. Values flow between steps through a small normative expression grammar. Documents are valid as JSON, YAML, or HCL, and the Go reference library — `github.com/tabilet/uws` — validates them before execution.

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

What is absent is the point. No HTTP method, no path, no server URL, no response schema, no security scheme. Those all live in the OpenAPI documents named by `sourceDescriptions`. UWS points at the operation and describes how the workflow uses it.

## Feature spotlight 1: Strict OpenAPI binding

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

The validator rejects anything else. That single rule is what keeps the UWS document and its OpenAPI source from drifting apart silently, and it is what lets UWS stay small: if the OpenAPI document describes the request shape, UWS does not need to.

## Feature spotlight 2: Six structural constructs

Operations are the leaves. Workflows and steps compose them, and each workflow picks one of six structural types:

- **`sequence`** — steps run in declaration order.
- **`parallel`** — steps run concurrently, subject to `dependsOn`.
- **`switch`** — the first `case` whose `when` is truthy runs; otherwise `default`.
- **`loop`** — iterates over a JSON array named by `items`, with an optional `batchSize`.
- **`merge`** — combines outputs of upstream constructs named by `dependsOn`.
- **`await`** — blocks until its `wait` expression is truthy or a timeout elapses.

Each construct has concrete validation rules: a `loop` must have `items`, a `merge` must have at least one `dependsOn`, a `switch` must not have `items`, an `await` must have `wait`. The validator enforces these before the runtime sees the document.

Structural types are intentionally finite. A second implementor does not need to ask what `merge` means — it has one paragraph of normative text and one set of validator rules.

## Feature spotlight 3: A normative expression grammar

Workflows pass values between steps through expressions. UWS defines a closed set of sources:

- `$response.statusCode`, `$response.body`, `$response.body#/json/pointer`, `$response.headers.<name>`
- `$outputs.<name>` — a same-scope output
- `$steps.<stepId>.outputs.<name>` — a sibling step's output
- `$variables.<name>` — a document-scope variable
- `$trigger`, `$trigger.<path>` — the trigger payload

Consumers may dot-walk into structured values, so `$steps.get_weather.outputs.summary` reaches into the named output produced by the prior step. Comparison operators (`==`, `!=`, `<`, `<=`, `>`, `>=`) are defined for `when`, `Criterion.condition`, and similar fields. Richer grammar — functions, boolean operators, templating — is explicitly implementation-defined and must not be required for core conformance.

The payoff is portability. An expression written against the UWS core means the same thing in any compliant runtime.

## Feature spotlight 4: Triggers as typed entry points

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

## Feature spotlight 5: Structural results, linked back to their source

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

## Feature spotlight 6: Extension profiles for non-HTTP work

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

## Feature spotlight 7: Validation that fails fast

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

`ValidateResult` returns every error with a dotted path. That matters when a language model produces the workflow: you want to tell it exactly which field was wrong, not just that something was.

## Feature spotlight 8: JSON, YAML, and HCL — pick the format that fits the moment

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

Here is a complete UWS document that exercises almost every spotlight: OpenAPI-backed operations, a profile-owned runtime step, explicit dependencies, structured outputs, and a sequential workflow.

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

## Where UWS fits for AI agents

Language models are good at reading intent and filling templates. They are less reliable at planning a multi-step API sequence from a large OpenAPI catalog.

UWS narrows the surface. An agent extracts intent, materializes a UWS document (from a template or freshly generated), validates it with the Go library, and hands the validated document to a deterministic runtime:

```text
natural language  →  structured intent  →  UWS document  →  Validate()  →  execution
```

The model proposes; the runtime executes. Invalid workflows are caught at the contract boundary, not inside a live API session where recovery is expensive.

For an agent, this is the difference between "the model guessed a tool call" and "the model produced a workflow contract the runtime can validate and execute."

## TL;DR

- UWS is a compact, client-side execution contract for OpenAPI-backed workflows.
- Operations bind strictly to OpenAPI — no duplication of methods, paths, or schemas.
- Six structural constructs (`sequence`, `parallel`, `switch`, `loop`, `merge`, `await`) cover real control flow; each has concrete validation rules.
- A small normative expression grammar keeps workflows portable across runtimes.
- Triggers, structural results, and extension profiles complete the model.
- `github.com/tabilet/uws` ships the Go model, the JSON Schema, JSON/YAML/HCL conversion, and a semantic validator that fails fast.
- For AI agents, UWS is the contract between intent extraction and deterministic execution.
