# Mastering OpenAPI-Bound Workflows in Go: A Deep Dive into UWS

Simplify, validate, and automate API workflows with the `github.com/tabilet/uws` package.

## Introduction: OpenAPI Describes the API, Not the Job

If you have built REST systems for more than a few months, you already know the value of OpenAPI. It tells clients what endpoints exist, what parameters are accepted, what payloads are legal, and what responses can come back.

But real products are not usually a single endpoint.

A useful task often looks more like this:

1. Read the user's intent.
2. Call a weather endpoint.
3. Transform the response into a concise message.
4. Send that message through a mail endpoint.
5. Capture the result for auditing or a follow-up step.

OpenAPI is still the right source of truth for the HTTP details. It should own the method, path, request schema, response schema, server URL, and security model. What it does not try to own is the workflow overlay: which operation comes first, how values flow through the task, which trigger starts the work, and which runtime expressions are meaningful to the workflow runner.

This matters because OpenAPI, or a close machine-readable variant, is already the common artifact in the wild. Major cloud and web platforms publish API descriptions in forms such as OpenAPI, Google Discovery documents, or service-specific API catalogs. AWS, Google Cloud, and many large SaaS providers have machine-readable API descriptions long before they have Arazzo workflow documents. The installed base is API description first, workflow description second.

That is the space for UWS, the Udon Workflow Specification.

UWS is a compact workflow document for OpenAPI-backed HTTP operations. It is similar in role to Arazzo, but it is not the same thing. Arazzo is an OpenAPI Initiative workflow description standard. UWS is a smaller execution-oriented interchange contract designed around `udon`: top-level operations, structural workflow steps, triggers, results, runtime expressions, and extension-owned profiles for behavior that is not part of OpenAPI.

The practical equation is:

```text
OpenAPI + UWS = OpenAPI-backed workflow contract
Arazzo = standard API workflow description
```

## The Missing Layer: Binding Operations into Workflows

Imagine an AI assistant receives this request:

> Send me a daily weather report for Los Angeles at 5 PM.

An OpenAPI document can describe the weather API and the mail API. It can tell us that one operation accepts a city query and another operation accepts a message payload.

But the workflow still needs answers:

- Which OpenAPI file owns the weather operation?
- Which operation should run first?
- Which request values are fixed by the user intent?
- Which values should come from the previous operation?
- Which runtime is allowed to execute non-HTTP behavior such as formatting an email body?

UWS separates those concerns cleanly. It binds local operation IDs to OpenAPI operations, adds request values, and keeps ordering/control-flow metadata outside the OpenAPI source document.

Here is a minimal UWS JSON document:

```json
{
  "uws": "1.0.0",
  "info": {
    "title": "Weather Report",
    "version": "1.0.0"
  },
  "sourceDescriptions": [
    {
      "name": "weather_api",
      "url": "./weather.openapi.yaml",
      "type": "openapi"
    },
    {
      "name": "gmail_api",
      "url": "./gmail.openapi.yaml",
      "type": "openapi"
    }
  ],
  "operations": [
    {
      "operationId": "current_weather",
      "sourceDescription": "weather_api",
      "openapiOperationId": "getCurrentWeather",
      "request": {
        "query": {
          "q": "Los Angeles",
          "units": "imperial"
        }
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
          "raw": "$outputs.weather_email_raw"
        }
      }
    }
  ]
}
```

Notice what is not here: no HTTP method, no path, no server URL, no response schema, no security scheme. Those stay in OpenAPI. UWS points to the operation and describes how the workflow uses it.

## Meet `github.com/tabilet/uws`

The `uws` package gives Go programs a small toolkit for working with UWS documents:

- `uws1` defines the UWS 1.x Go model.
- `(*uws1.Document).Validate()` checks structural and semantic rules.
- `ValidateResult()` returns all path-tagged validation errors.
- `convert` moves documents between JSON, YAML, and canonical HCL.
- `uws.json` provides the JSON Schema for UWS 1.0 documents.
- `versions/1.0.0.md` is the human-readable specification.

Install it like any normal Go module:

```bash
go get github.com/tabilet/uws
```

Then parse and validate a document:

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
			fmt.Printf("validation error: %s: %s\n", issue.Path, issue.Message)
		}
		return
	}

	fmt.Println("UWS document is valid and ready for execution planning.")
}
```

This validation layer is important. For an AI-generated workflow, you do not want to discover at runtime that an operation references a missing source description or that an extension-owned operation forgot to name its runtime profile.

## Feature Spotlight 1: OpenAPI Stays Canonical

The narrowed UWS core is intentionally OpenAPI-first.

A UWS operation can bind to OpenAPI in two ways:

```json
{
  "operationId": "list_pets",
  "sourceDescription": "petstore_api",
  "openapiOperationId": "listPets"
}
```

or:

```json
{
  "operationId": "list_pets",
  "sourceDescription": "petstore_api",
  "openapiOperationRef": "#/paths/~1pets/get"
}
```

Use `openapiOperationId` when the OpenAPI operation has a stable operation ID. Use `openapiOperationRef` when the OpenAPI document lacks a useful ID or when you want a direct JSON Pointer fragment.

The rule is simple: OpenAPI-bound UWS operations must include a `sourceDescription` and exactly one OpenAPI binding field.

## Feature Spotlight 2: HCL for Human Authoring

JSON is a good interchange format. YAML is readable. HCL is often better for authoring workflows that humans need to edit repeatedly.

The `convert` package supports canonical HCL for UWS core fields:

```hcl
uws = "1.0.0"

info {
  title   = "Pet Store Workflow"
  version = "1.0.0"
}

sourceDescription "petstore_api" {
  url  = "./petstore.yaml"
  type = "openapi"
}

operation "list_pets" {
  sourceDescription  = "petstore_api"
  openapiOperationId = "listPets"

  request = {
    query = {
      limit = 10
    }
  }
}
```

Convert it to JSON:

```go
jsonData, err := convert.HCLToJSON(hclData)
```

Or convert JSON back to HCL:

```go
hclData, err := convert.JSONToHCL(jsonData)
```

One important design choice: JSON and YAML preserve `x-*` extensions, but HCL conversion rejects documents with extensions. That prevents a subtle bug where implementation-specific runtime data would disappear during conversion.

## Feature Spotlight 3: Extension-Owned Runtime Profiles

UWS core does not define local commands, function calls, file I/O, SSH, SQL, object storage, LLM calls, or browser automation.

Those behaviors belong in implementation profiles.

For example, an implementation like `udon` can define a profile-owned function operation:

```json
{
  "operationId": "build_email",
  "x-uws-operation-profile": "udon",
  "x-udon-runtime": {
    "type": "fnct",
    "function": "mail_raw",
    "args": {
      "from": "bot@example.com",
      "to": "user@example.com",
      "subject": "Daily weather report",
      "body": "$outputs.current_weather.summary"
    }
  }
}
```

This keeps the UWS core clean. If a behavior can be explained as an OpenAPI-backed HTTP operation, it belongs in the core model. If it needs local execution semantics, it belongs behind an extension profile.

The extension-owned operation must include `x-uws-operation-profile`, and the value must contain at least one non-whitespace character. That small rule matters because it lets validators distinguish “intentionally owned by a runtime profile” from “missing OpenAPI binding by mistake.”

## Demo: Weather Report Workflow

Let’s build the common agentic example: ask for a weather report, fetch weather, construct an email, and send it.

The OpenAPI documents own the HTTP facts:

```text
weather.openapi.yaml -> getCurrentWeather
gmail.openapi.yaml   -> sendMessage
```

The UWS document owns the task structure:

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

  - operationId: build_email
    x-uws-operation-profile: udon
    x-udon-runtime:
      type: fnct
      function: mail_raw
      args:
        from: bot@example.com
        to: user@example.com
        subject: Daily weather report for Los Angeles
        body: $outputs.current_weather.summary
    dependsOn:
      - current_weather

  - operationId: send_report
    sourceDescription: gmail_api
    openapiOperationId: sendMessage
    dependsOn:
      - build_email
    request:
      body:
        userId: me
        raw: $outputs.build_email.raw

workflows:
  - workflowId: main
    type: sequence
    steps:
      - stepId: get_weather
        operationRef: current_weather
      - stepId: render_email
        operationRef: build_email
        dependsOn:
          - current_weather
      - stepId: send_email
        operationRef: send_report
        dependsOn:
          - build_email
```

This document is intentionally split:

- Weather and Gmail remain OpenAPI-bound.
- Email formatting is runtime-owned because MIME formatting is not an OpenAPI operation by itself.
- The sequence is explicit and auditable.
- A runner can validate the document before executing anything.

## Practical Use Cases

### 1. Contract Testing

Use UWS as a test contract for multi-step API behavior. The OpenAPI file describes the endpoint. The UWS file describes a known task. A Go test runner can load both, validate the UWS document, and execute the workflow against staging.

If the workflow breaks, either the API contract changed or the workflow logic is no longer valid.

### 2. Documentation

Endpoint documentation is often too low-level for product users. A UWS document can describe a capability: “create a customer,” “submit an order,” “send a weather report.”

That capability can be rendered as a tutorial, a test fixture, or an executable recipe.

### 3. AI Agents

This is where UWS becomes especially useful.

LLMs are good at reading user intent, but weak at reliably planning multi-step API sequences from a giant OpenAPI catalog. If you expose every endpoint as a separate tool, the model must choose the right order, guess intermediate data flow, and remember edge cases.

This is also why starting from OpenAPI is pragmatic. Most real integrations already have OpenAPI or a similar API description format available. Arazzo is useful when a provider or team publishes workflow recipes, but it is not the common starting point for the majority of web APIs today.

UWS reduces that burden. The AI does not need to think through every HTTP call. It can select or generate a workflow contract, validate it, and hand it to a deterministic Go runtime.

## Using UWS as an AI Workflow

An AI workflow can be split into three layers.

### Phase 1: Intent Extraction

The user starts with natural language:

```text
Send me a weather report for Los Angeles every day at 5 PM.
```

The AI should not immediately call tools. It should first collect the missing slots:

```json
{
  "intent": "daily_weather_report",
  "city": "Los Angeles",
  "schedule": "17:00",
  "recipient": "user@example.com",
  "status": "ready_for_workflow"
}
```

This is the reasoning layer. Interactive refinement is useful here: if the user says “send me the report,” the agent should ask for a recipient before execution.

### Phase 2: Workflow Materialization

The intent becomes a UWS document.

For a known capability, the system can fill a template:

```yaml
operationId: current_weather
sourceDescription: weather_api
openapiOperationId: getCurrentWeather
request:
  query:
    q: Los Angeles
    units: imperial
```

Then it can run validation:

```go
if err := doc.Validate(); err != nil {
	return fmt.Errorf("AI produced an invalid workflow: %w", err)
}
```

This is where UWS is valuable. The model can propose structure, but Go validation enforces the contract.

### Phase 3: Deterministic Execution

Once the UWS document is valid, the runtime can lower it into an execution plan. In the `udon` ecosystem, UWS is the standard workflow contract, while `udon` is the compiler and executor.

The flow looks like this:

```text
Natural language
  -> structured intent
  -> UWS document
  -> ValidateForExecution
  -> runtime plan
  -> OpenAPI-backed execution
```

The AI remains useful, but it is no longer improvising every API call at runtime. It prepares the intent and workflow. The Go runtime executes the plan.

## API Engine vs. Browser Operator

For agentic systems, there are two broad ways to act:

1. API-first execution: use OpenAPI plus UWS, validate the workflow, then call APIs directly.
2. Browser-first operation: inspect the DOM or page image, click, type, scroll, and recover from UI changes.

The API-first approach is better when the API exists and the task must be reliable, fast, auditable, and cheap to repeat. The browser approach is better when no useful API exists or when the user truly needs a one-off assistant to navigate a website.

UWS belongs on the API-first side. It gives the agent a typed, inspectable bridge between intent and execution.

## Why This Matters

The future of API automation is not just larger prompts. It is better contracts between probabilistic reasoning and deterministic execution.

OpenAPI tells us what calls are possible. UWS tells us how a task binds those calls into a workflow. Go gives us validation, concurrency, and predictable runtime behavior.

For AI systems, that separation is the difference between “the model guessed a tool call” and “the model produced a workflow contract that the runtime can validate and execute.”

## TL;DR

- OpenAPI remains the source of truth for HTTP methods, paths, schemas, servers, and security.
- Major cloud and web services commonly publish OpenAPI or similar API description formats, not Arazzo workflow documents.
- UWS describes the workflow overlay: operation bindings, request values, dependencies, structural control flow, triggers, and runtime expression fields.
- `github.com/tabilet/uws` provides Go models, validation, JSON Schema, and JSON/YAML/HCL conversion helpers.
- UWS and Arazzo overlap, but they are not equivalent.
- Non-OpenAPI behavior belongs in `x-*` extension profiles.
- For AI agents, UWS is the contract between intent extraction and deterministic OpenAPI-backed execution.
