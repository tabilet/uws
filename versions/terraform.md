# Terraform, OpenAPI, and UWS: Rethinking API Workflows

Terraform solved a real problem: cloud systems are too large and too dynamic to manage by hand. It gave infrastructure teams a familiar cycle:

```text
write configuration
plan the change
approve the plan
apply it
track state
repeat
```

That model is still one of the best engineering patterns for infrastructure. The question is whether the same shape should be copied directly for API workflows.

UWS takes a related but different path. Terraform starts from provider plugins. UWS starts from OpenAPI or an equivalent machine-readable API description, then adds workflow metadata on top.

That difference matters.

## Terraform in Practice

Terraform is infrastructure as code. A user writes HCL configuration that declares the desired state of cloud or SaaS resources. Terraform then calculates a plan and applies changes in dependency order.

The common operational loop is:

```bash
terraform init
terraform plan -out=tfplan
terraform apply tfplan
terraform show -json tfplan > tfplan.json
```

Under the hood, this is not simply "run an HTTP request." Terraform is built around long-lived managed resources and state. A resource like `aws_instance.web` is not just a call to an EC2 API. It is a typed object with create, read, update, delete, import, diff, lifecycle, dependency, and drift behavior.

That is why Terraform providers exist.

## Terraform's Technical Stack

A simplified Terraform stack looks like this:

```text
Terraform HCL
  -> Terraform Core
  -> provider plugin
  -> backend API
  -> Terraform state
```

Terraform Core is the CLI and graph engine. It reads configuration, evaluates expressions, manages state, builds a dependency graph, creates a plan, and applies changes.

Provider plugins are separate executables, usually written in Go. Terraform Core communicates with them over RPC. Each provider implements a service-specific model: resources, data sources, functions, authentication, and API calls.

The provider boundary has real engineering advantages:

- Providers can model high-level resources rather than raw REST operations.
- Providers can hide backend API quirks behind a stable Terraform schema.
- Providers can encode state migration, retry behavior, diff suppression, import behavior, and lifecycle semantics.
- Providers let Terraform manage many backends through a common CLI and state model.

It also has costs:

- Every backend needs a maintained provider implementation.
- Providers are released separately from Terraform and have their own version cadence.
- A provider can lag behind the underlying service API when the API changes.
- Provider schemas may expose a simpler resource model and omit detailed operation-level API parameters.
- Enterprise teams must govern provider binaries: source, checksum, mirror/cache, version lock, review, and rollout.
- Local Terraform CLI workflows do not automatically create a complete historical deployment ledger unless teams preserve plans, logs, state snapshots, and approvals in CI/CD, HCP Terraform, Terraform Enterprise, or another audit system.

Terraform state is essential, but it is not the same thing as a complete run history. State is the current source of truth Terraform uses to decide what changes are needed. A plan can be exported to JSON, and CI/CD can retain artifacts, but that is an operational pattern around Terraform rather than a universal local run ledger.

## The Provider Problem for API Workflows

Provider plugins make sense for infrastructure resources. They are less obviously necessary for many API workflows.

Consider a normal API workflow:

1. Call a weather endpoint.
2. Transform the result.
3. Call an email endpoint.
4. Save an execution record.

If the weather and email APIs already have OpenAPI documents, the raw operation surface is already described:

- operation IDs
- HTTP methods
- paths
- request parameters
- request bodies
- response schemas
- security schemes
- server URLs

For this class of work, writing a Terraform-style provider first can be unnecessary. It introduces another schema and another release cadence between the workflow and the service API.

This is especially relevant because many major services already publish machine-readable API descriptions. Google APIs expose Discovery documents that describe REST API surfaces, schemas, OAuth scopes, and methods. Google Cloud also supports OpenAPI in API management products. AWS publicly publishes Smithy API models for AWS service APIs, and those service models are used for SDK and CLI generation. Many SaaS providers publish OpenAPI directly.

The important point is not that every service uses OpenAPI exactly. The point is that API descriptions already exist in the ecosystem. UWS is designed to use OpenAPI directly, and to accept equivalent descriptions after conversion.

## How UWS Works

UWS starts from this pipeline:

```text
UWS or HCL workflow
  -> OpenAPI / Discovery / converted service model
  -> validation
  -> runtime plan
  -> execution
  -> persisted run record
```

In UWS core, an operation binds to OpenAPI:

```yaml
operationId: current_weather
sourceDescription: weather_api
openapiOperationId: getCurrentWeather
request:
  query:
    q: Los Angeles
    units: imperial
```

OpenAPI owns method, path, schemas, server, and security. UWS owns the workflow overlay: local operation IDs, request values, dependencies, outputs, triggers, and structural control flow.

A full UWS document can then wire operations together:

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

  - operationId: send_report
    sourceDescription: gmail_api
    openapiOperationId: sendMessage
    dependsOn:
      - current_weather
    request:
      body:
        userId: me
        raw: $outputs.build_email.raw
```

No Terraform provider is required for the OpenAPI-bound operations. The runtime can resolve the operation from the OpenAPI source and execute it as HTTP.

## Extension Profiles: Where UWS Becomes Provider-Like

Some behavior is not OpenAPI.

Examples:

- building a MIME email body
- writing a local artifact
- running a shell command
- reading a file
- calling an in-process function
- querying a local database
- invoking an LLM

UWS does not push those into core service types. It uses extension profiles.

An implementation such as `udon` can define a profile:

```yaml
operationId: render_email
x-uws-operation-profile: udon
x-udon-runtime:
  type: fnct
  function: mail_raw
  args:
    from: bot@example.com
    to: user@example.com
    subject: Daily weather report
    body: $outputs.current_weather.summary
dependsOn:
  - current_weather
```

The same profile can specify file I/O:

```yaml
operationId: write_report
x-uws-operation-profile: udon
x-udon-runtime:
  type: fileio
  action: write
  path: ./out/weather-report.txt
  body: $outputs.render_email.text
dependsOn:
  - render_email
```

Or command execution:

```yaml
operationId: publish_artifact
x-uws-operation-profile: udon
x-udon-runtime:
  type: cmd
  command: ./publish-report.sh
  arguments:
    - ./out/weather-report.txt
dependsOn:
  - write_report
```

This is intentionally provider-like, but narrower. OpenAPI-bound HTTP calls do not need a custom provider. Only non-OpenAPI behavior needs a profile specification.

## Terraform vs UWS

The strongest way to compare them is by runtime contract.

```text
Terraform:
  HCL -> Terraform Core -> provider plugin -> API -> state

UWS / udon:
  HCL or UWS -> OpenAPI -> runtime plan -> API execution -> run history
```

Terraform optimizes for desired state. UWS optimizes for workflow execution.

Terraform asks:

```text
What should the infrastructure look like after apply?
```

UWS asks:

```text
What workflow should run, in what order, using which API operations?
```

That difference changes the implementation.

Terraform needs provider plugins because resources have lifecycle behavior. UWS can use OpenAPI directly because many workflow steps are ordinary API operations.

Terraform hides backend detail behind resource schemas. UWS can expose the operation-level request/response shape from the API description.

Terraform state tracks managed infrastructure. UWS execution history can track workflow runs: input, UWS document, OpenAPI source references, validated runtime plan, per-step request/response metadata, outputs, errors, approvals, and artifacts.

## Where UWS Can Improve the Terraform Pattern

### 1. Start from the API Description

Most cloud services and web APIs already have machine-readable API descriptions: OpenAPI, Google Discovery documents, Smithy models, or vendor-specific catalogs.

UWS can use those descriptions as the provider surface. That avoids rewriting the API schema by hand into a provider plugin before a workflow can call it.

### 2. Reduce Provider Lag

Terraform providers need to be updated when the service API changes and when the provider schema needs to expose new features.

If a new OpenAPI operation or parameter appears, a UWS/OpenAPI runtime can often use it as soon as the OpenAPI document is updated and the workflow binds to it. That does not remove the need for testing, approvals, or compatibility checks, but it shortens the path between API evolution and workflow availability.

### 3. Preserve Detailed API Parameters

Terraform provider resources intentionally simplify APIs into resource arguments. That is good for infrastructure lifecycle management, but it can hide detailed operation-level parameters.

UWS keeps the request binding close to the OpenAPI operation:

```yaml
request:
  query:
    maxResults: 50
    pageToken: $outputs.previous_page.nextPageToken
  header:
    X-Trace-ID: $variables.trace_id
  body:
    dryRun: true
```

The workflow author can use the precise request surface described by the API document.

### 4. Simplify Enterprise Binary Governance

Terraform provider plugins are standalone binaries. That is a powerful extension point, but it creates governance work in enterprise environments:

- pinning provider versions
- reviewing provider provenance
- scanning binaries
- maintaining mirrors or caches
- controlling plugin upgrades
- approving private providers

UWS still needs trusted execution, but OpenAPI-bound operations do not require a new binary per service. The runtime can enforce one execution engine, one audit model, and one approval path for many API descriptions.

### 5. Make Run History First-Class

Terraform's state file is not a complete historical deployment record. Teams often preserve history through CI/CD logs, saved plan files, HCP Terraform, Terraform Enterprise, or external audit systems.

UWS can make execution history a first-class runtime concern. A UWS executor can persist full run data into SQLite:

- submitted intent
- generated or approved UWS document
- OpenAPI source versions or hashes
- runtime plan
- approvals
- step start/end timestamps
- request and response metadata
- outputs
- errors
- produced artifacts

That is useful for AI workflows because the execution trace is part of the safety story. The model may propose a workflow, but the runtime can record exactly what was validated and executed.

## Example: From API Catalog to Workflow

Suppose a cloud provider publishes a machine-readable description for a storage API and an email API.

Terraform would typically need provider resources:

```hcl
resource "example_storage_bucket" "reports" {
  name   = "daily-reports"
  region = "us-west"
}
```

That is perfect when the goal is to manage a bucket as infrastructure.

UWS is better when the goal is to run a task:

```yaml
operations:
  - operationId: list_reports
    sourceDescription: storage_api
    openapiOperationId: listObjects
    request:
      query:
        bucket: daily-reports

  - operationId: send_latest_report
    sourceDescription: email_api
    openapiOperationId: sendMessage
    dependsOn:
      - list_reports
    request:
      body:
        to: ops@example.com
        subject: Latest daily report
        body: $outputs.list_reports.latest
```

The resource-management case and the workflow-execution case are different problems. Terraform should stay strong where it is strong. UWS should handle API workflow execution without forcing every API through a provider plugin first.

## What UWS Should Not Claim

UWS should not claim to replace Terraform.

Terraform is a mature IaC system with resource lifecycle semantics, large provider ecosystems, state backends, modules, policy integrations, and team workflows.

UWS should claim something narrower:

```text
For API workflows, if the backend contract is already machine-readable, use that contract directly.
```

That is the architectural improvement.

## TL;DR

- Terraform is excellent for desired-state infrastructure management.
- Terraform providers are powerful but require standalone binaries, versioning, governance, and provider-specific maintenance.
- Many major cloud and web services already publish OpenAPI or equivalent machine-readable API descriptions such as Google Discovery documents or AWS Smithy models.
- UWS uses those API descriptions directly for OpenAPI-bound HTTP workflow operations.
- UWS extension profiles such as `x-udon-runtime` cover non-OpenAPI services like `fileio`, `cmd`, `fnct`, `ssh`, `sql`, and `llm` with their own specification.
- Terraform state is not the same as a complete historical run ledger; UWS/udon can make full workflow execution history, including SQLite persistence, part of the runtime design.

## Sources

- HashiCorp Terraform introduction: https://developer.hashicorp.com/terraform/intro
- Terraform providers: https://developer.hashicorp.com/terraform/language/providers
- Terraform plugin architecture: https://developer.hashicorp.com/terraform/plugin/how-terraform-works
- Terraform state purpose: https://developer.hashicorp.com/terraform/language/state/purpose
- Terraform JSON output format: https://developer.hashicorp.com/terraform/internals/json-format
- Google API Discovery Service: https://cloud.google.com/docs/discovery/apis
- AWS API models announcement: https://aws.amazon.com/about-aws/whats-new/2025/06/open-source-aws-api-models

