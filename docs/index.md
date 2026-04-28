# UWS — Udon Workflow Specification

UWS is a compact, execution-oriented workflow specification that sits directly on top of OpenAPI. A UWS document describes *how* to orchestrate HTTP API operations that OpenAPI already describes — without duplicating methods, paths, schemas, servers, or security schemes.

> OpenAPI owns the API contract. UWS owns the workflow overlay.

## Why UWS?

Many API providers already publish OpenAPI. Once those documents exist, a workflow layer should stay focused on workflow concerns:

- which operations participate
- how they depend on each other
- how values flow between them
- which structural constructs shape control flow
- which triggers start execution
- which semantics are portable across runtimes

UWS answers exactly those questions and nothing else.

Beyond document shape, UWS also defines a normative **execution model**: a clean split between an orchestrator that owns structural workflow execution (dependency resolution, control flow, retry, output propagation) and a bound runtime that owns leaf execution (HTTP calls, expression evaluation, item resolution). This split is what makes UWS portable — any compliant runtime shares the same orchestration semantics while bringing its own transport, credentials, and extension-profile logic.

## A Minimal Document

```json
{
  "uws": "1.1.0",
  "info": { "title": "Weather Report", "version": "1.1.0" },
  "sourceDescriptions": [
    { "name": "weather_api", "url": "./weather.openapi.yaml", "type": "openapi" },
    { "name": "gmail_api",   "url": "./gmail.openapi.yaml",   "type": "openapi" }
  ],
  "operations": [
    {
      "operationId": "current_weather",
      "sourceDescription": "weather_api",
      "openapiOperationId": "getCurrentWeather",
      "request": { "query": { "q": "Los Angeles", "units": "imperial" } },
      "outputs": { "summary": "$response.body.summary" }
    },
    {
      "operationId": "send_report",
      "sourceDescription": "gmail_api",
      "openapiOperationId": "sendMessage",
      "dependsOn": ["current_weather"],
      "request": { "body": { "userId": "me", "text": "$steps.get_weather.outputs.summary" } }
    }
  ],
  "workflows": [
    {
      "workflowId": "main",
      "type": "sequence",
      "steps": [
        { "stepId": "get_weather", "operationRef": "current_weather" },
        { "stepId": "send_email",  "operationRef": "send_report" }
      ]
    }
  ]
}
```

No HTTP method, no path, no server URL, no response schema — those live in the referenced OpenAPI documents. UWS points at the operations and describes how the workflow uses them.

## Executing a Document

A UWS document is not just a description — it is executable. Bind a runtime, call `Execute`, and UWS core orchestrates the entire workflow:

```go
doc.SetRuntime(rt)          // bind your HTTP/extension runtime
if err := doc.Execute(ctx); err != nil {
    log.Fatal(err)
}
records := doc.ExecutionRecords()  // inspect what ran
```

The orchestrator owns all structural concerns: dependency resolution, parallel scheduling, conditional branching, loop iteration, retry counting, and trigger routing. The bound runtime owns only leaf work: making the actual HTTP call, evaluating expressions, and resolving iteration items. This split means the same UWS document runs identically on any compliant runtime.

## Reference

- **Specification**: [`versions/1.1.0.md`](https://github.com/tabilet/uws/blob/main/versions/1.1.0.md)
- **JSON Schema**: [`versions/1.1.0.json`](https://github.com/tabilet/uws/blob/main/versions/1.1.0.json)
- **Go package**: `github.com/tabilet/uws`
- **License**: Apache 2.0

## The 10 Major Features

| # | Feature | Summary |
|---|---------|---------|
| 1 | [OpenAPI Operation Binding](01-OpenAPI-Operation-Binding.md) | Strict three-shape binding to OpenAPI operations |
| 2 | [Six Structural Constructs](02-Six-Structural-Constructs.md) | sequence · parallel · switch · loop · merge · await |
| 3 | [Runtime Expression Grammar](03-Runtime-Expression-Grammar.md) | Normative ABNF expression language for value flow |
| 4 | [Triggers and Route Dispatch](04-Triggers-and-Route-Dispatch.md) | Typed entry points with output-based routing |
| 5 | [Structural Results](05-Structural-Results.md) | Named outputs linked back to their source construct |
| 6 | [Success Criteria and Actions](06-Success-Criteria-and-Actions.md) | Inline retry, goto, end with per-criterion scoping |
| 7 | [Execution Model](07-Execution-Model.md) | Orchestrator/runtime split for portable semantics |
| 8 | [Extension Profiles](08-Extension-Profiles.md) | Non-HTTP operations via `x-uws-operation-profile` |
| 9 | [Validation](09-Validation.md) | Two-layer structural + semantic validation |
| 10 | [Interchange Formats](10-Interchange-Formats.md) | JSON, YAML, and HCL with round-trip guarantees |
