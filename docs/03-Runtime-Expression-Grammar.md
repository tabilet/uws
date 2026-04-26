# Feature 3: Runtime Expression Grammar

← [Six Structural Constructs](02-Six-Structural-Constructs.md) | [Next: Triggers and Route Dispatch →](04-Triggers-and-Route-Dispatch.md)

---

UWS uses runtime expression strings in control-flow fields (`when`, `forEach`, `wait`, `items`, `batchSize`), in criterion conditions, and in the string values of `outputs` maps. The expression language is deliberately small and normative — a runtime that implements it verbatim is portable by construction.

## Expression Sources

Every expression references data through a leading `$` sigil. A dotted suffix `.<segment>` walks into the resolved value.

| Source | Meaning |
|--------|---------|
| `$response.statusCode` | HTTP response status code of the current operation |
| `$response.body` | HTTP response body of the current operation |
| `$response.body#/json/pointer` | RFC 6901 JSON Pointer into the response body |
| `$response.headers.<name>` | HTTP response header value by name |
| `$outputs.<name>` | Same-scope output declared by the enclosing operation, workflow, or step |
| `$outputs.<name>.<path>` | Dot-walk into a structured output value |
| `$steps.<stepId>.outputs.<name>` | Output of a named sibling step |
| `$steps.<stepId>.outputs.<name>.<path>` | Dot-walk into a sibling step output |
| `$variables.<name>` | Document-scope variable from `variables` or `components.variables` |
| `$variables.<name>.<path>` | Dot-walk into a structured variable value |
| `$trigger` | The payload delivered by the enclosing trigger |
| `$trigger.<path>` | Dot-walk into the trigger payload |

A resolved value may be any JSON type: string, number, boolean, null, object, or array.

## Accessing Response Data

The most common expression source — reading data out of an HTTP response:

```yaml
operationId: search_products
sourceDescription: catalog_api
openapiOperationId: searchProducts
outputs:
  # Whole body
  raw_response: $response.body

  # Top-level field
  total_count: $response.body.total

  # JSON Pointer — first item's id field
  first_id: $response.body#/items/0/id

  # Nested field via dot-walk
  first_name: $response.body.items.0.name

  # HTTP status code
  status: $response.statusCode

  # Response header
  next_page: $response.headers.X-Next-Page
```

**Using a response value to conditionally skip the next step:**

```yaml
- stepId: fetch_user
  operationRef: get_user
  outputs:
    active: $response.body.active

- stepId: send_email
  operationRef: send_welcome
  when: $steps.fetch_user.outputs.active == true
```

## Cross-Step References with `$steps`

`$steps.<stepId>.outputs.<name>` reads the named output of a sibling step within the same workflow:

```yaml
workflowId: enrich_order
type: sequence
steps:
  - stepId: load_order
    operationRef: get_order
    outputs:
      order_id:   $response.body.id
      user_id:    $response.body.userId
      total:      $response.body.total

  - stepId: load_user
    operationRef: get_user
    request:
      path:
        userId: $steps.load_order.outputs.user_id

  - stepId: apply_discount
    operationRef: compute_discount
    request:
      body:
        orderId:     $steps.load_order.outputs.order_id
        userTier:    $steps.load_user.outputs.tier
        baseTotal:   $steps.load_order.outputs.total
```

**Dot-walking into a structured output:**

```yaml
outputs:
  street: $steps.load_user.outputs.address.street
  city:   $steps.load_user.outputs.address.city
```

Dot-walk segments MUST match `[A-Za-z0-9_-]+`. A property name containing `.` cannot be accessed this way — use a JSON Pointer on `$response.body` or a dedicated output instead.

## Document-Scope Variables with `$variables`

Variables declared at the top level or in `components.variables` are available everywhere:

```yaml
variables:
  api_env: production
  page_size: 20
  config:
    region: us-west-2
    timeout: 30

operations:
  - operationId: list_items
    sourceDescription: catalog_api
    openapiOperationId: listItems
    request:
      query:
        env:      $variables.api_env
        pageSize: $variables.page_size
        region:   $variables.config.region
```

## Trigger Payload Access with `$trigger`

Inside workflows started by a trigger dispatch, `$trigger` holds the inbound payload:

```yaml
triggers:
  - triggerId: order_webhook
    outputs: [created, updated]
    routes:
      - output: created
        to: [handle_new_order]

workflows:
  - workflowId: handle_new_order
    type: sequence
    steps:
      - stepId: save_order
        operationRef: create_order
        request:
          body:
            externalId: $trigger.orderId
            amount:     $trigger.total
            currency:   $trigger.currency

      - stepId: notify
        operationRef: send_confirmation
        when: $trigger.notify == true
        request:
          body:
            email: $trigger.customer.email
```

## Comparison Operators

Inside `when`, `Criterion.condition` (when `type` is `simple`), and other boolean expression fields, UWS defines six comparison operators:

```
== != < <= > >=
```

Rules:
- Operands MUST be of the same JSON type — no implicit coercion.
- `==` and `!=` apply to strings, numbers, booleans, and null.
- `<`, `<=`, `>`, `>=` apply to strings (lexicographic) and numbers only.
- A single space surrounds the operator; `==` / `!=` / `<=` / `>=` are always two-character tokens.

```yaml
# Number comparison
when: $response.body.count > 0
when: $response.statusCode == 200
when: $variables.retries <= 3

# String comparison
when: $response.body.status == "active"
when: $trigger.event != "ping"

# Null check
when: $steps.load.outputs.result != null
```

## JSON Pointer Fragments

A `#`-prefixed RFC 6901 JSON Pointer can be appended to `$response.body` to address deeply nested values:

```yaml
outputs:
  # First item in an array
  first_item: $response.body#/items/0

  # Nested object field
  street: $response.body#/address/street

  # Key with a slash in its name (slash escaped as ~1)
  value: $response.body#/metrics~1rate
```

Inside a `Criterion` with `type: jsonpath`, the `context` field uses the same pointer form to select the value to test.

## Output Dot-Walk and `null` Semantics

If a segment in a dot-walk path does not resolve — the property is absent, the index is out of bounds, or the segment is applied to a scalar — the expression evaluates to `null`:

```yaml
# If body.items is absent or empty, first_id resolves to null
outputs:
  first_id: $response.body.items.0.id

# Guard against null with a criterion
successCriteria:
  - condition: $outputs.first_id != null
```

## Normative ABNF Grammar (§5.6)

The full grammar is defined in §5.6 of the spec. Key productions:

```abnf
expression     = condition / source-expr
condition      = source-expr SP op SP operand
op             = "==" / "!=" / "<=" / ">=" / "<" / ">"
operand        = source-expr / literal

source-expr    = response-expr / outputs-expr / steps-expr / variables-expr / trigger-expr
response-expr  = "$response.statusCode"
               / "$response.body" [ json-pointer ]
               / "$response.headers." header-name
outputs-expr   = "$outputs." name [ "." path ]
steps-expr     = "$steps." identifier ".outputs." name [ "." path ]
variables-expr = "$variables." name [ "." path ]
trigger-expr   = "$trigger" [ "." path ]

path           = segment *( "." segment )
segment        = 1*id-char
id-char        = ALPHA / DIGIT / "_" / "-"
literal        = json-string / json-number / json-bool / json-null
```

## Implementation Extensions

Richer features — boolean connectives (`&&`, `||`, `!`), arithmetic, function calls — are implementation-defined. A conforming UWS core MUST NOT require them for documents using only the normative grammar. Documents that depend on extended syntax SHOULD declare the dependency via `x-uws-operation-profile`.

```yaml
# This is UWS-core expression (normative — all runtimes support it):
when: $response.statusCode == 200

# This requires an implementation extension (non-portable):
when: $response.statusCode == 200 && $response.body.count > 0
```

---

← [Six Structural Constructs](02-Six-Structural-Constructs.md) | [Next: Triggers and Route Dispatch →](04-Triggers-and-Route-Dispatch.md)
