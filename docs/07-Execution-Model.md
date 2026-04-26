# Feature 7: Execution Model

← [Success Criteria and Actions](06-Success-Criteria-and-Actions.md) | [Next: Extension Profiles →](08-Extension-Profiles.md)

---

UWS defines a real orchestrator/runtime split. This is what distinguishes UWS from a document-only format: the specification defines portable execution semantics, not just document shape.

## The Split

**UWS core (the Orchestrator) owns:**

- Document validation before execution
- Indexing operations, workflows, steps, trigger targets, and `parallelGroup` members
- Dependency execution across operations, workflows, and steps
- Evaluation and enforcement of `dependsOn`, `when`, `forEach`, `items`, `batchSize`, `wait`
- Execution of all six structural constructs
- Processing of success and failure actions (`retry`, `goto`, `end`)
- Output resolution and structural result shaping
- Trigger route resolution and routed execution

**The bound runtime owns:**

- Leaf execution of a single Operation (one HTTP call, or one extension-owned action)
- Expression evaluation against the current execution context
- Resolving the item list for `forEach` and `loop`

This line is what allows UWS to define portable orchestration semantics without standardizing HTTP clients, secret handling, storage engines, or product-specific runtime hooks.

## The Runtime Interface

Three methods are all the orchestrator needs from a concrete runtime:

```go
type Runtime interface {
    ExecuteLeaf(ctx context.Context, op *Operation) error
    EvaluateExpression(ctx context.Context, expr string) (any, error)
    ResolveItems(ctx context.Context, itemsExpr string) ([]any, error)
}
```

Everything above this line — dependency resolution, parallel scheduling, retry counting, switch evaluation, loop batching — is owned by UWS core.

## Example 1: Minimal Runtime Implementation

A mock runtime that records calls, useful for testing:

```go
type MockRuntime struct {
    Calls  []string
    Values map[string]any
}

func (r *MockRuntime) ExecuteLeaf(ctx context.Context, op *uws1.Operation) error {
    r.Calls = append(r.Calls, op.OperationID)
    return nil
}

func (r *MockRuntime) EvaluateExpression(ctx context.Context, expr string) (any, error) {
    if v, ok := r.Values[expr]; ok {
        return v, nil
    }
    return nil, nil
}

func (r *MockRuntime) ResolveItems(ctx context.Context, itemsExpr string) ([]any, error) {
    v, err := r.EvaluateExpression(ctx, itemsExpr)
    if v == nil {
        return nil, err
    }
    items, ok := v.([]any)
    if !ok {
        return nil, fmt.Errorf("items expression did not resolve to array")
    }
    return items, nil
}
```

## Example 2: Binding and Executing a Document

```go
package main

import (
    "context"
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

    rt := &MyHTTPRuntime{ /* ... */ }
    doc.SetRuntime(rt)

    ctx := context.Background()
    if err := doc.Execute(ctx); err != nil {
        log.Fatalf("execution failed: %v", err)
    }

    // Inspect what ran
    for key, record := range doc.ExecutionRecords() {
        log.Printf("%s: status=%s", key, record.Status)
    }
}
```

`doc.Execute(ctx)` runs three checks automatically before handing off to the orchestrator: `Validate()`, `ValidateExecutable()`, and `ValidateExecutionEntrypoint()`.

## Example 3: Trigger Dispatch

Accepting an inbound event and routing it through UWS core:

```go
// Receive a webhook payload (e.g. from an HTTP handler)
func handleWebhook(w http.ResponseWriter, r *http.Request) {
    var payload map[string]any
    json.NewDecoder(r.Body).Decode(&payload)

    // Determine which output fired based on payload content
    outputIndex := 0  // "created"
    if payload["event"] == "updated" {
        outputIndex = 1
    }

    err := doc.DispatchTrigger(r.Context(), "order_events", outputIndex, payload)
    if err != nil {
        http.Error(w, err.Error(), 500)
        return
    }
    w.WriteHeader(204)
}
```

UWS core resolves the route, validates the targets, and executes them through the standard orchestrator. The payload becomes `$trigger` inside the routed workflows.

## Example 4: Inspecting Execution Records

After execution completes, records identify every step that ran:

```go
doc.SetRuntime(rt)
if err := doc.Execute(ctx); err != nil {
    log.Printf("execution error: %v", err)
}

records := doc.ExecutionRecords()
for key, record := range records {
    fmt.Printf("%-40s status=%-10s kind=%s\n",
        key, record.Status, record.Kind)
}
```

Example output:
```
operations/list_pets           status=success    kind=operation
operations/send_report         status=success    kind=operation
workflows/main                 status=success    kind=workflow
```

The keying scheme is implementation-defined. UWS 1.0 does not standardize a serialized record store.

## Execution Context Available to the Runtime

During each `ExecuteLeaf` call, the runtime can inspect:

| Context | What it contains |
|---------|-----------------|
| Trigger context | Active trigger ID, emitted output label, and payload |
| Iteration context | Current item value, index, batch number, position in batch |
| Current-execution context | The operation or step being run right now |
| Execution records snapshot | All records accumulated so far |

These are passed through `context.Context` and are never serialized into the UWS wire format.

## Validation Before Execution

The three pre-execution checks run automatically inside `doc.Execute(ctx)`:

```go
if err := d.Validate(); err != nil {
    return err   // UWS document validity
}
if err := d.ValidateExecutable(); err != nil {
    return err   // entry workflow, runtime binding
}
if err := d.ValidateExecutionEntrypoint(); err != nil {
    return err   // entrypoint requirements
}
```

## The AI Agent Pipeline

For AI agent use cases, UWS acts as the contract between intent extraction and execution:

```
natural language  →  structured intent  →  UWS document  →  Validate()  →  Execute()
```

```go
// Agent proposes a UWS document
proposedYAML := agent.ProduceWorkflow(userIntent)

var doc uws1.Document
convert.UnmarshalYAML([]byte(proposedYAML), &doc)

// Catch errors before execution
result := doc.ValidateResult()
if !result.Valid() {
    // Feed structured errors back to the model for correction
    correction := agent.CorrectWorkflow(proposedYAML, result.Errors)
    // ... retry
    return
}

// Only execute a validated document
doc.SetRuntime(rt)
doc.Execute(ctx)
```

`ValidateResult()` returns path-tagged errors that drop straight into a corrective prompt. Two fields, two exact paths — enough for the model to fix in one pass rather than guessing from prose.

---

← [Success Criteria and Actions](06-Success-Criteria-and-Actions.md) | [Next: Extension Profiles →](08-Extension-Profiles.md)
