# Feature 6: Success Criteria and Actions

← [Structural Results](05-Structural-Results.md) | [Next: Execution Model →](07-Execution-Model.md)

---

UWS defines a first-class vocabulary for deciding whether an operation succeeded, and for specifying what to do when it did or did not. Criteria and actions are declared inline on the operation — there is no shared registry.

## Criterion Object

A `Criterion` describes a condition to evaluate:

| Field | Required | Description |
|-------|----------|-------------|
| `condition` | REQUIRED | Runtime expression or literal condition |
| `type` | optional | `simple` (default), `regex`, `jsonpath`, or `xpath` |
| `context` | REQUIRED when type is `regex`, `jsonpath`, or `xpath` | Value to apply the condition against |

**`simple` type (default)** — uses the normative comparison grammar:

```yaml
successCriteria:
  - condition: $response.statusCode == 200
  - condition: $response.body.success == true
  - condition: $response.body.count > 0
```

**`regex` type** — tests a string value against a regular expression:

```yaml
successCriteria:
  - condition: "^(200|201|204)$"
    type: regex
    context: $response.statusCode
```

**`jsonpath` type** — evaluates a JSONPath expression against a context value:

```yaml
successCriteria:
  - condition: "$.items[?(@.status == 'active')]"
    type: jsonpath
    context: $response.body
```

**`xpath` type** — evaluates an XPath expression against an XML context:

```yaml
successCriteria:
  - condition: "//order[@status='confirmed']"
    type: xpath
    context: $response.body
```

## Failure Actions

`onFailure` fires when the operation's `successCriteria` are not satisfied:

| Field | Required | Description |
|-------|----------|-------------|
| `name` | REQUIRED | Unique action name |
| `type` | REQUIRED | `end`, `goto`, or `retry` |
| `workflowId` | REQUIRED for `goto` (exactly one of `workflowId`/`stepId`) | Target workflow |
| `stepId` | REQUIRED for `goto` (exactly one of `workflowId`/`stepId`) | Target step |
| `retryAfter` | optional | Seconds to wait before retrying. MUST be ≥ 0 |
| `retryLimit` | REQUIRED for `retry` | Maximum retry attempts. MUST be ≥ 1 |
| `criteria` | optional | Additional conditions that scope this action |

## Success Actions

`onSuccess` fires when the operation's `successCriteria` are all satisfied:

| Field | Required | Description |
|-------|----------|-------------|
| `name` | REQUIRED | Unique action name |
| `type` | REQUIRED | `end` or `goto` |
| `workflowId` | REQUIRED for `goto` (exactly one of `workflowId`/`stepId`) | Target workflow |
| `stepId` | REQUIRED for `goto` (exactly one of `workflowId`/`stepId`) | Target step |
| `criteria` | optional | Additional conditions that scope this action |

## Example 1: Retry with Fallback

Retry on server errors up to 3 times; give up and end on client errors; branch to an error handler on total failure.

```yaml
operationId: charge_payment
sourceDescription: stripe_api
openapiOperationId: createCharge
successCriteria:
  - condition: $response.statusCode == 200
  - condition: $response.body.status == "succeeded"
onFailure:
  - name: retry_on_5xx
    type: retry
    retryAfter: 2
    retryLimit: 3
    criteria:
      - condition: $response.statusCode >= 500

  - name: skip_on_4xx
    type: end
    criteria:
      - condition: $response.statusCode >= 400
      - condition: $response.statusCode < 500

  - name: escalate
    type: goto
    workflowId: payment_error_handler
```

`retry_on_5xx` only activates when status ≥ 500. `skip_on_4xx` activates for 4xx. `escalate` catches anything else (applies when no scoping criteria are set, acting as the default fallback).

## Example 2: `goto` to a Specific Step

On success, branch directly to a named step rather than a whole workflow:

```yaml
operationId: check_eligibility
sourceDescription: rules_api
openapiOperationId: evaluateRules
successCriteria:
  - condition: $response.body.eligible == true
onSuccess:
  - name: fast_track
    type: goto
    stepId: approve_immediately
    criteria:
      - condition: $response.body.score >= 90

  - name: standard
    type: goto
    stepId: queue_for_review
```

`fast_track` sends high-scoring applicants straight to approval. Everyone else goes to the review queue.

## Example 3: `goto` to a Workflow on Failure

Route persistent failures to a dedicated error-handling workflow:

```yaml
operationId: sync_inventory
sourceDescription: warehouse_api
openapiOperationId: syncStock
successCriteria:
  - condition: $response.statusCode == 200
onFailure:
  - name: retry_briefly
    type: retry
    retryAfter: 5
    retryLimit: 2
    criteria:
      - condition: $response.statusCode >= 500

  - name: raise_incident
    type: goto
    workflowId: ops_incident_workflow
```

After 2 failed retries, `raise_incident` fires and routes to the incident management workflow.

## Example 4: `end` on Partial Success

Stop early when there is nothing left to do, without calling it an error:

```yaml
operationId: list_pending_orders
sourceDescription: orders_api
openapiOperationId: getPendingOrders
successCriteria:
  - condition: $response.statusCode == 200
onSuccess:
  - name: nothing_to_process
    type: end
    criteria:
      - condition: $response.body.total == 0
```

If the list is empty, execution ends cleanly. If items exist, the default behavior continues to the next step.

## Example 5: Multi-Criterion `successCriteria`

All criteria in `successCriteria` must be satisfied for the operation to be considered successful:

```yaml
operationId: create_session
sourceDescription: auth_api
openapiOperationId: initiateSession
successCriteria:
  - condition: $response.statusCode == 201
  - condition: $response.body.token != null
  - condition: "^[A-Za-z0-9_-]{32,}$"
    type: regex
    context: $response.body.token
onFailure:
  - name: auth_failure
    type: goto
    workflowId: handle_auth_error
```

All three criteria must pass. If the token is missing or malformed, `auth_failure` fires.

## Inline Declaration

UWS does not provide a reusable component registry for criteria and actions. Each operation declares its own. This keeps the document self-describing and allows sibling operations to evolve independently without shared state.

---

← [Structural Results](05-Structural-Results.md) | [Next: Execution Model →](07-Execution-Model.md)
