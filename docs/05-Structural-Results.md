# Feature 5: Structural Results

← [Triggers and Route Dispatch](04-Triggers-and-Route-Dispatch.md) | [Next: Success Criteria and Actions →](06-Success-Criteria-and-Actions.md)

---

A structural result gives a named, addressable identity to the output of a structural construct — a workflow or step whose `type` is `switch`, `merge`, or `loop`. Without this declaration, a construct's output is anonymous control flow. With it, the output becomes a named artifact that downstream code can reference.

## Structural Result Object Fields

| Field | Required | Description |
|-------|----------|-------------|
| `name` | REQUIRED | Unique result name within `results[]`. MUST match `^[a-zA-Z0-9._-]+$` |
| `kind` | REQUIRED | One of `switch`, `merge`, `loop`. MUST equal the `type` of the referenced construct |
| `from` | REQUIRED | `<workflowId>` or `<workflowId>.<stepId>` of the emitting construct |
| `value` | optional | Runtime expression selecting the value to expose; implementation-defined when omitted |

## The `from` Field

`from` identifies the emitting construct in one of two forms:

- **`<workflowId>`** — a top-level workflow whose `type` is `switch`, `merge`, or `loop`.
- **`<workflowId>.<stepId>`** — a step within a named workflow whose `type` is `switch`, `merge`, or `loop`.

The validator resolves `from` to a real workflow or step, then checks that the referenced `type` matches `kind`. A mismatch produces a structured error.

## Why Only Three Kinds?

`switch`, `merge`, and `loop` each produce a meaningful aggregate result:

- **`switch`** — the output of whichever branch ran (or nothing if no branch matched).
- **`merge`** — the combined outputs of multiple upstream constructs.
- **`loop`** — the accumulated results of iterating over an array.

`sequence`, `parallel`, and `await` do not produce a single named aggregate output — their outputs flow through step-level `outputs` maps instead.

## Example 1: `merge` Result — Combining Parallel Checks

Two validation steps run in parallel; a merge step collects their results; a named result exposes the combined output.

```yaml
workflows:
  - workflowId: validate_order
    type: parallel
    steps:
      - stepId: check_inventory
        operationRef: validate_stock
        parallelGroup: validators
        outputs:
          ok:      $response.body.available
          message: $response.body.reason

      - stepId: check_credit
        operationRef: validate_payment
        parallelGroup: validators
        outputs:
          ok:      $response.body.approved
          limit:   $response.body.creditLimit

      - stepId: combine_checks
        type: merge
        dependsOn: [validators]
        outputs:
          inventory_ok: $steps.check_inventory.outputs.ok
          credit_ok:    $steps.check_credit.outputs.ok
          credit_limit: $steps.check_credit.outputs.limit

results:
  - name: order_validation
    kind: merge
    from: validate_order.combine_checks
    value: $steps.combine_checks.outputs
```

`order_validation` is now a named, addressable result. Any downstream logic that needs to know "did the validation pass?" references this result by name.

## Example 2: `loop` Result — Accumulated Iteration Output

A loop processes each item in an array and collects the results into a named output.

```yaml
workflows:
  - workflowId: import_records
    type: loop
    items: $outputs.pending_records
    steps:
      - stepId: upsert_record
        operationRef: create_or_update
        outputs:
          record_id: $response.body.id
          created:   $response.body.created

results:
  - name: import_summary
    kind: loop
    from: import_records
    value: $steps.upsert_record.outputs
```

`import_summary` names the loop output. The runtime defines what "accumulated loop results" means in practice (e.g. an array of per-iteration outputs), but the named result is the UWS-level handle.

## Example 3: `switch` Result — Named Branch Decision

A switch construct selects one processing path; the result names which branch ran and what it produced.

```yaml
workflows:
  - workflowId: classify_event
    type: switch
    cases:
      - name: high_value
        when: $trigger.amount >= 1000
        steps:
          - stepId: premium_process
            operationRef: handle_premium_order
            outputs:
              tier:     "premium"
              discount: $response.body.appliedDiscount

      - name: standard
        when: $trigger.amount < 1000
        steps:
          - stepId: normal_process
            operationRef: handle_standard_order
            outputs:
              tier:     "standard"
              discount: "0"

results:
  - name: classification_result
    kind: switch
    from: classify_event
    value: $steps.premium_process.outputs
```

## Example 4: `from` Pointing at a Top-Level Workflow

When the emitting construct is itself a top-level workflow (not a nested step):

```yaml
workflows:
  - workflowId: batch_process
    type: loop
    items: $outputs.job_queue
    steps:
      - stepId: run_job
        operationRef: execute_job

results:
  - name: batch_results
    kind: loop
    from: batch_process       # top-level workflow, no ".stepId"
```

## Name Uniqueness

Result names MUST be unique within `results[]`. The validator catches duplicates:

```yaml
results:
  - name: my_result
    kind: merge
    from: wf1.step1
  - name: my_result     # ← duplicate
    kind: loop
    from: wf2
# error: results[1].name: duplicate result name "my_result"
```

## Validator: Kind/Type Mismatch

```yaml
workflows:
  - workflowId: my_loop
    type: loop
    items: $outputs.items
    steps:
      - stepId: process
        operationRef: do_work

results:
  - name: bad_result
    kind: merge          # ← wrong: loop construct, but kind is merge
    from: my_loop
# error: results[0].kind: kind "merge" does not match "my_loop" type "loop"
```

---

← [Triggers and Route Dispatch](04-Triggers-and-Route-Dispatch.md) | [Next: Success Criteria and Actions →](06-Success-Criteria-and-Actions.md)
