# Feature 8: Extension Profiles

← [Execution Model](07-Execution-Model.md) | [Next: Validation →](09-Validation.md)

---

Not every step in a workflow is an HTTP call. Formatting a message, invoking a function, running a local command, calling a language model, reading a file, or executing a SQL query are common needs. UWS keeps these out of the core and uses extension profiles instead.

## Extension-Owned Operations

An operation without an OpenAPI binding is extension-owned. It MUST include `x-uws-operation-profile` naming the profile that can execute it:

- `x-uws-operation-profile` MUST contain at least one non-whitespace character.
- `sourceDescription`, `openapiOperationId`, and `openapiOperationRef` MUST NOT be set.
- Additional `x-*` fields carry profile-specific configuration and are not interpreted by UWS core.

The validator accepts extension-owned operations as intentionally runtime-owned — it does not flag the absent OpenAPI binding as an error.

## Example 1: Function Call

Invoke a local or serverless function within the workflow:

```yaml
operationId: format_report
x-uws-operation-profile: udon
x-udon-runtime:
  type: fnct
  function: render_markdown
  args:
    template: daily_report
    data:
      summary:    $steps.get_weather.outputs.summary
      date:       $variables.report_date
      recipient:  $steps.get_user.outputs.email
dependsOn: [get_weather, get_user]
outputs:
  html: $response.body.rendered
```

The `udon` runtime resolves `render_markdown` locally. UWS core sees only `dependsOn`, `outputs`, and that the profile is `udon`.

## Example 2: Language Model Call

Run a prompt through an LLM as part of the workflow:

```yaml
operationId: summarize_feedback
x-uws-operation-profile: udon
x-udon-runtime:
  type: llm
  model: gpt-4o
  prompt: |
    Summarize the following customer feedback in one sentence:
    {{ $steps.fetch_feedback.outputs.text }}
  temperature: 0.3
dependsOn: [fetch_feedback]
outputs:
  summary: $response.body.content
```

The LLM call is runtime-owned. The rest of the workflow — how `fetch_feedback` runs, what `summary` feeds into downstream — is orchestrated by UWS core.

## Example 3: SQL Query

Execute a database query as a workflow step:

```yaml
operationId: load_pending_orders
x-uws-operation-profile: udon
x-udon-runtime:
  type: sql
  query: |
    SELECT id, total, customer_id
    FROM orders
    WHERE status = 'pending'
      AND created_at > $variables.cutoff_date
  database: orders_db
outputs:
  rows:  $response.body.rows
  count: $response.body.count
```

## Example 4: SSH / Shell Command

Run a remote command over SSH:

```yaml
operationId: deploy_artifact
x-uws-operation-profile: udon
x-udon-runtime:
  type: ssh
  host:    $variables.deploy_host
  command: |
    cd /opt/app && \
    ./deploy.sh {{ $steps.build.outputs.artifact_path }}
  timeout: 120
dependsOn: [build]
outputs:
  exit_code: $response.body.exitCode
  logs:      $response.body.stdout
```

## Example 5: Mixing Core and Extension Operations

A single document with three operation kinds — OpenAPI-bound, function call, and OpenAPI-bound:

```yaml
sourceDescriptions:
  - name: weather_api
    url: ./weather.openapi.yaml
    type: openapi
  - name: gmail_api
    url: ./gmail.openapi.yaml
    type: openapi

operations:
  # Shape 1: OpenAPI-bound
  - operationId: get_weather
    sourceDescription: weather_api
    openapiOperationId: getCurrentWeather
    request:
      query:
        q: Los Angeles
    outputs:
      summary: $response.body.summary
      temp_f:  $response.body.main.temp

  # Shape 3: Extension-owned (function call)
  - operationId: build_email
    x-uws-operation-profile: udon
    x-udon-runtime:
      type: fnct
      function: mail_raw
      args:
        subject: "Weather: {{ $steps.fetch.outputs.temp_f }}°F in Los Angeles"
        body:    $steps.fetch.outputs.summary
    dependsOn: [get_weather]
    outputs:
      raw: $response.body.raw

  # Shape 1: OpenAPI-bound
  - operationId: send_report
    sourceDescription: gmail_api
    openapiOperationId: sendMessage
    dependsOn: [build_email]
    request:
      body:
        userId: me
        raw:    $steps.compose.outputs.raw
```

Weather and Gmail stay fully OpenAPI-bound. Email formatting is runtime-owned. The document validates before any execution begins.

## Specification Extensions on Non-Operation Objects

`x-*` fields are not limited to extension-owned operations. Any UWS object can carry them as metadata:

```json
{
  "workflowId": "main",
  "type": "sequence",
  "x-owner": "payments-team",
  "x-sla-ms": 5000,
  "steps": [...]
}
```

Conforming tooling MUST preserve these fields on round-trip and MUST NOT interpret or modify them.

## Reserved Prefix

| Prefix | Usage |
|--------|-------|
| `x-uws-` | **Reserved** — UWS core only. `x-uws-operation-profile` is the sole 1.0 occupant. |
| `x-udon-` | `udon` runtime implementation |
| `x-<vendor>-` | Vendor-specific |
| `x-<product>-` | Product-specific |

Third-party tooling MUST NOT introduce fields under `x-uws-`. All other `x-*` prefixes are governed entirely by the implementation that defines them.

## What Happens When a Profile Is Unknown

The UWS validator accepts extension-owned operations regardless of whether the named profile is supported by the current runtime. Profile resolution is a runtime concern, not a schema concern:

```yaml
operationId: do_something
x-uws-operation-profile: my_custom_profile
x-my-custom-profile:
  action: whatever
```

- **Validator**: accepts the document — `my_custom_profile` is a valid non-empty string. ✓
- **Runtime**: if the runtime does not implement `my_custom_profile`, it returns an error at execution time.
- **HCL conversion**: rejects the document — extension fields would be lost in HCL. Use JSON or YAML for documents with extension profiles.

---

← [Execution Model](07-Execution-Model.md) | [Next: Validation →](09-Validation.md)
