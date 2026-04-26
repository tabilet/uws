# Feature 2: Six Structural Constructs

← [OpenAPI Operation Binding](01-OpenAPI-Operation-Binding.md) | [Next: Runtime Expression Grammar →](03-Runtime-Expression-Grammar.md)

---

Operations are the leaves. Workflows and steps compose them using six structural control-flow constructs. Each workflow declares exactly one `type`; nested steps may also declare a structural `type` to form composite control flow.

## `sequence`

Steps execute in declaration order. Each step completes before the next begins. Use `sequence` for any pipeline where outputs flow from one step to the next.

```yaml
workflowId: checkout
type: sequence
steps:
  - stepId: validate_cart
    operationRef: validate_order
  - stepId: charge_card
    operationRef: charge_payment
    dependsOn: [validate_cart]
  - stepId: send_receipt
    operationRef: send_email
    dependsOn: [charge_card]
```

`dependsOn` within a sequence adds explicit cross-step dependencies beyond declaration order. `items` MUST NOT be set.

**Adding a cross-workflow dependency from another workflow:**

```yaml
workflowId: post_checkout
type: sequence
dependsOn: [checkout]        # waits for the entire checkout workflow first
steps:
  - stepId: update_inventory
    operationRef: decrement_stock
```

## `parallel`

Steps execute concurrently, subject only to `dependsOn` relationships. Use `parallel` when independent calls can overlap.

```yaml
workflowId: enrichment
type: parallel
steps:
  - stepId: fetch_weather
    operationRef: get_weather
  - stepId: fetch_stocks
    operationRef: get_stocks
  - stepId: fetch_news
    operationRef: get_headlines
```

All three operations fire simultaneously. The construct completes when every step terminates.

**`parallelGroup` as a dependency barrier:**

```yaml
workflowId: validate_and_merge
type: parallel
steps:
  - stepId: check_format
    operationRef: validate_format
    parallelGroup: validators     # member of the "validators" group

  - stepId: check_range
    operationRef: validate_range
    parallelGroup: validators     # also a member

  - stepId: aggregate
    operationRef: merge_results
    dependsOn: [validators]       # waits for ALL members of "validators"
```

`dependsOn: [validators]` waits for every step in the `validators` group. Membership in a group does not create additional ordering among members themselves — they still run concurrently.

## `switch`

Exactly one `case` whose `when` evaluates truthy runs its steps. If no case matches, `default` runs if present; otherwise the construct emits no result.

```yaml
workflowId: route_event
type: switch
cases:
  - name: new_user
    when: $trigger.body.event == "signup"
    steps:
      - stepId: welcome
        operationRef: send_welcome_email

  - name: returning_user
    when: $trigger.body.event == "login"
    steps:
      - stepId: log_access
        operationRef: record_login

default:
  - stepId: fallback_log
    operationRef: log_unknown_event
```

`items` MUST NOT be set on a `switch`.

**`switch` inside a `sequence` step:**

```yaml
workflowId: process_order
type: sequence
steps:
  - stepId: fetch_order
    operationRef: get_order
  - stepId: route
    type: switch
    cases:
      - name: express
        when: $steps.fetch_order.outputs.tier == "express"
        steps:
          - stepId: expedite
            operationRef: fast_ship
      - name: standard
        when: $steps.fetch_order.outputs.tier == "standard"
        steps:
          - stepId: queue
            operationRef: standard_ship
```

## `loop`

Iterates over a JSON array. `items` is REQUIRED and must resolve to a JSON array at runtime. Each element is bound into the iteration scope for use in nested steps.

```yaml
workflowId: notify_all
type: loop
items: $outputs.subscribers
steps:
  - stepId: send_one
    operationRef: send_notification
```

**`loop` with `batchSize` — processing in fixed groups:**

```yaml
workflowId: bulk_import
type: loop
items: $outputs.records           # e.g. array of 1000 records
batchSize: "50"                   # process 50 at a time
steps:
  - stepId: import_batch
    operationRef: bulk_upsert
```

`batchSize` MUST resolve to a positive integer. Batches execute sequentially; within each batch the items are available as the iteration context. `cases` and `default` MUST NOT be set on a `loop`.

## `merge`

Combines the outputs of multiple upstream constructs named by `dependsOn` into a single structural result. Use `merge` after a `parallel` to collect independent results.

```yaml
workflowId: gather_data
type: parallel
steps:
  - stepId: prices
    operationRef: get_prices
    parallelGroup: fetchers
  - stepId: ratings
    operationRef: get_ratings
    parallelGroup: fetchers
  - stepId: combine
    type: merge
    dependsOn: [fetchers]
    outputs:
      prices:  $steps.prices.outputs.data
      ratings: $steps.ratings.outputs.data
```

The corresponding result declaration:

```yaml
results:
  - name: market_data
    kind: merge
    from: gather_data.combine
    value: $steps.combine.outputs
```

`dependsOn` is REQUIRED and MUST name at least one construct. `items` MUST NOT be set.

## `await`

Blocks execution until its `wait` expression evaluates truthy. Use `await` to poll for an async job to complete, or to wait for an external signal.

```yaml
workflowId: wait_for_job
type: await
wait: $outputs.job_status == "done"
```

**`await` after kicking off an async job:**

```yaml
workflowId: run_report
type: sequence
steps:
  - stepId: submit
    operationRef: start_report_job
    outputs:
      job_id: $response.body.jobId

  - stepId: poll
    type: await
    wait: $steps.check_status.outputs.status == "complete"

  - stepId: check_status
    operationRef: get_job_status
    request:
      path:
        jobId: $steps.submit.outputs.job_id

  - stepId: download
    operationRef: fetch_report_result
```

Any timeout applied to `await` is an executor option, not a serialized UWS field. `cases`, `default`, and `items` MUST NOT be set.

## Field Constraints Summary

| Type | `items` | `wait` | `cases`/`default` | `dependsOn` |
|------|---------|--------|-------------------|-------------|
| `sequence` | MUST NOT | optional | MUST NOT | optional |
| `parallel` | MUST NOT | optional | MUST NOT | optional |
| `switch` | MUST NOT | optional | allowed | optional |
| `loop` | REQUIRED | optional | MUST NOT | optional |
| `merge` | MUST NOT | optional | MUST NOT | REQUIRED (≥1) |
| `await` | MUST NOT | REQUIRED | MUST NOT | optional |

The validator enforces every constraint above before the runtime sees the document.

## Composing Types: Nested Steps

A top-level `workflow` always declares a `type`. A nested `step` may declare a `type` to become an inline structural construct, use `operationRef` to call an operation, or use `workflow` to invoke a top-level workflow by ID.

```yaml
# A sequence whose third step is itself a loop
workflowId: process_batch
type: sequence
steps:
  - stepId: fetch_all
    operationRef: list_items
    outputs:
      items: $response.body.items

  - stepId: validate_all
    operationRef: validate_batch

  - stepId: process_each
    type: loop
    items: $steps.fetch_all.outputs.items
    steps:
      - stepId: handle_item
        operationRef: process_single_item
```

---

← [OpenAPI Operation Binding](01-OpenAPI-Operation-Binding.md) | [Next: Runtime Expression Grammar →](03-Runtime-Expression-Grammar.md)
