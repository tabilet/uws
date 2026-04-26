# Feature 4: Triggers and Route Dispatch

← [Runtime Expression Grammar](03-Runtime-Expression-Grammar.md) | [Next: Structural Results →](05-Structural-Results.md)

---

Triggers are the entry points into a UWS document. They model webhook-style events, scheduled invocations, or any external signal that starts workflow execution. Once a trigger event has been accepted, UWS core owns route resolution and execution — the same orchestrator rules used for normal workflow execution apply.

## Trigger Object Fields

| Field | Required | Description |
|-------|----------|-------------|
| `triggerId` | REQUIRED | Unique trigger identifier |
| `path` | optional | HTTP path for webhook-style triggers |
| `methods` | optional | Accepted HTTP methods |
| `authentication` | optional | Authentication metadata |
| `options` | optional | Open-shape trigger-specific options |
| `outputs` | REQUIRED when `routes` is set | Ordered list of unique output labels |
| `routes` | optional | Output-to-target routing table |

## Basic Webhook Trigger

A trigger that accepts POST requests and routes to different workflows based on event type:

```yaml
triggers:
  - triggerId: order_events
    path: /webhooks/orders
    methods:
      - POST
    authentication: bearer
    outputs:
      - order.created
      - order.updated
      - order.cancelled
    routes:
      - output: order.created
        to: [handle_new_order]
      - output: order.updated
        to: [handle_order_update]
      - output: order.cancelled
        to: [handle_cancellation, send_refund_notification]
```

`order.cancelled` routes to two targets simultaneously — both `handle_cancellation` and `send_refund_notification` execute when that output fires.

## Outputs and Uniqueness

`outputs` is an ordered list of unique labels the trigger may emit. Each label MUST match `^[a-zA-Z0-9._-]+$`. Each invocation MUST emit exactly one label.

```yaml
outputs:
  - created        # index 0
  - updated        # index 1
  - deleted        # index 2
```

## Addressing Outputs by Decimal Index

A route's `output` may be the label itself or the zero-based decimal-string index into `outputs`:

```yaml
routes:
  - output: created        # by label
    to: [create_flow]

  - output: "1"            # index 1 = "updated"
    to: [update_flow]

  - output: "2"            # index 2 = "deleted"
    to: [delete_flow]
```

Both forms are equivalent. Label form is more readable; index form is useful when labels are dynamic.

## Multi-Target Routing

One output can fan out to multiple targets. All targets execute through the same orchestrator rules:

```yaml
routes:
  - output: user.registered
    to:
      - onboarding_workflow      # sends welcome email
      - analytics_workflow       # records signup event
      - crm_sync_workflow        # creates CRM contact
```

Each target receives the same trigger payload and runs independently.

## Trigger with Options

`options` is an open-shape map for trigger-specific configuration. Its shape is entirely implementation-defined:

```yaml
triggers:
  - triggerId: scheduled_report
    options:
      schedule: "0 9 * * 1-5"    # weekdays at 09:00
      timezone: America/New_York
      retryOnFailure: true
    outputs:
      - fire
    routes:
      - output: fire
        to: [generate_daily_report]
```

## Using Trigger Payload in Workflow Steps

During trigger-routed execution, `$trigger` holds the inbound payload. Steps within the routed workflow can read from it:

```yaml
triggers:
  - triggerId: pet_event
    path: /webhooks/pets
    methods: [POST]
    outputs: [created]
    routes:
      - output: created
        to: [handle_new_pet]

workflows:
  - workflowId: handle_new_pet
    type: sequence
    steps:
      - stepId: save_pet
        operationRef: create_pet
        request:
          body:
            name:    $trigger.pet.name
            species: $trigger.pet.species
            owner:   $trigger.owner.id
        when: $trigger.valid == true

      - stepId: notify_owner
        operationRef: send_notification
        request:
          body:
            userId:  $trigger.owner.id
            message: "Your pet has been registered"
```

## Validation: What Fails

```yaml
# Route references undeclared output — validation error:
routes:
  - output: cancelled        # not in outputs list
    to: [cancel_flow]
# error: "cancelled" is not a declared trigger output

# Route with empty to — validation error:
routes:
  - output: created
    to: []
# error: to must contain at least one top-level stepId or workflowId

# routes set but outputs absent — validation error:
routes:
  - output: created
    to: [main]
# error: outputs is required when routes is set
```

## Trigger Dispatch in Go

After accepting an event externally, dispatch it into the document:

```go
// output is the zero-based index into trigger.outputs
// payload is the trigger event data
err := doc.DispatchTrigger(ctx, "order_events", 0, map[string]any{
    "orderId": "ord-123",
    "total":   99.99,
    "notify":  true,
})
```

UWS core resolves the route, validates targets, and executes them through the orchestrator. The payload becomes `$trigger` inside the routed workflows.

---

← [Runtime Expression Grammar](03-Runtime-Expression-Grammar.md) | [Next: Structural Results →](05-Structural-Results.md)
