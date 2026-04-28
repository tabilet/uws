# XRD-003: Portable Timeout And Workflow Idempotency

Status: accepted and implemented in UWS 1.1.0

Target version: UWS 1.1.0

Owner: UWS specification and Go model maintainers

Origin: Ramen XRD-003 handoff

## Problem

Ramen needs to describe two workflow policies without inventing private public-semantics fields:

- A portable serialized timeout that can be validated and preserved across UWS tooling.
- Workflow-level idempotency metadata that can prevent duplicate side effects for a logical
  workflow run.

UWS 1.0.0 deliberately does not provide either contract. `await` timeout is executor-owned and
configured out of band, and idempotency keys can only be ordinary OpenAPI request values when an API
declares an idempotency header or body field.

UWS 1.1.0 now defines these semantics. Ramen still needs its own prompt/schema/eval follow-up
before generated artifacts emit the new fields by default.

## Non-Goals

- Do not change UWS 1.0.0 behavior.
- Do not define provider credentials, secret storage, retry policy, or replay storage
  implementation.
- Do not inject HTTP idempotency headers automatically. API-specific idempotency headers remain
  explicit OpenAPI request bindings.
- Do not define product-specific policy such as who may approve side-effectful execution.

## Proposal Summary

Add two UWS 1.1.0 fields:

| Field | Object | Type | Summary |
| --- | --- | --- | --- |
| `timeout` | Operation, Workflow, Step | `number` | Maximum execution duration in seconds for the runnable after it is eligible to start. |
| `idempotency` | Workflow | Idempotency Object | Logical workflow-run de-duplication metadata. |

The fields are core UWS fields, not `x-*` extensions, because their behavior must be portable across
conforming tooling.

## Timeout Semantics

`timeout` is an optional positive number of seconds.

Validation:

- `timeout` MUST be greater than `0` when present.
- `timeout` MAY be fractional.
- `timeout` MAY appear on Operation, Workflow, and Step objects.

Execution:

- The timeout clock starts when the runnable becomes eligible to start, after its `dependsOn`
  dependencies have completed and after its `when` condition allows execution.
- For an Operation, `timeout` bounds one operation attempt delegated to the bound runtime.
- For a retrying Operation, each retry attempt receives a fresh timeout budget.
- For a Workflow or structural Step, `timeout` bounds the structural construct and all descendant
  work started by that construct.
- For an `await` Workflow or Step, `timeout` bounds the wait after the await construct starts.
- When a timeout expires, the runnable fails with a timeout failure. If the timed-out runnable is an
  Operation with matching `onFailure` actions, those actions are evaluated using the same failure
  handling rules as other operation failures. Otherwise execution fails for the enclosing construct.
- A parent context cancellation MAY terminate execution before `timeout` expires.

Serialization:

```yaml
operations:
  - operationId: charge_payment
    sourceDescription: payments_api
    openapiOperationId: createCharge
    timeout: 10

workflows:
  - workflowId: main
    type: sequence
    timeout: 60
    steps:
      - stepId: charge
        operationRef: charge_payment
```

## Idempotency Semantics

`idempotency` is an optional Workflow object field. It applies to one logical execution of that
workflow. It does not apply to every operation automatically and does not replace explicit
API-level idempotency request bindings.

### Idempotency Object

| Field | Type | Required | Summary |
| --- | --- | --- | --- |
| `key` | `string` | Yes | Runtime expression or literal string that resolves to the logical workflow-run key. |
| `onConflict` | `string` | No | One of `reject` or `returnPrevious`. Defaults to `reject`. |
| `ttl` | `number` | No | Optional retention window in seconds. MUST be greater than `0` when present. |

Validation:

- `key` is REQUIRED and MUST contain at least one non-whitespace character.
- `onConflict`, when present, MUST be `reject` or `returnPrevious`.
- `ttl`, when present, MUST be greater than `0`.

Execution:

- The executor resolves `key` when the workflow is eligible to start.
- The idempotency identity is `(document identity, workflowId, resolved key)`.
- The executor MUST reserve the idempotency identity before starting side-effectful work in the
  workflow.
- If no prior record exists, execution proceeds and the executor records the terminal outcome.
- If a prior record exists and `onConflict` is `reject`, the executor MUST fail before starting new
  side-effectful work.
- If a prior successful record exists and `onConflict` is `returnPrevious`, the executor MUST return
  the prior workflow outputs without starting new side-effectful work.
- If `onConflict` is `returnPrevious` but the prior record is not successful or retained outputs are
  unavailable, the executor MUST fail before starting new side-effectful work.
- `ttl` allows an executor to expire the idempotency record after the declared number of seconds.
  UWS does not mandate a storage backend.

Serialization:

```yaml
workflows:
  - workflowId: create_customer_case
    type: sequence
    idempotency:
      key: $variables.caseRequestId
      onConflict: returnPrevious
      ttl: 86400
    steps:
      - stepId: create_case
        operationRef: create_case
```

## Schema And Model Changes

The UWS implementation adds a UWS 1.1.0 contract instead of mutating the published 1.0.0 contract:

1. Add `versions/1.1.0.json`.
2. Add `versions/1.1.0.md`.
3. Add Go model fields:
   - `Timeout float64` on `OperationExecutionFields`, `WorkflowExecutionFields`, and
     `StepExecutionFields`.
   - `Idempotency *Idempotency` on `Workflow`.
   - `Idempotency` model with `Key string`, `OnConflict string`, and `TTL float64`.
4. Update known-field lists for Operation, Workflow, and Step.
5. Add validation for positive timeout, idempotency required fields, `onConflict` enum values, and
   positive `ttl`.
6. Add schema conformance and round-trip tests.
7. Update execution tests after the orchestrator/runtime contract grows concrete timeout and
   idempotency enforcement support.

## Ramen Handoff

Ramen may enable prompt/schema support in a separate follow-up now that UWS has accepted and
implemented this public contract. Until that Ramen follow-up lands:

- Ramen may document desired timeout and idempotency policy in project review evidence.
- Ramen may bind API-specific idempotency keys only through explicit OpenAPI request fields.
- Ramen generation behavior should remain unchanged.

## Open Questions

- Should UWS define a canonical document identity source, or should each executor supply document
  identity as part of its runtime configuration?
- Should `returnPrevious` return only declared workflow `outputs`, or also execution records?
- Should timeout failures expose a standard runtime expression source such as `$error.type` for
  `onFailure.criteria`, or should timeout matching remain executor-specific for UWS 1.1.0?
