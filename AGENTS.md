# Inheritance Pattern in Go (True Inheritance)

This project adopts the "True Inheritance" pattern to allow for a specialized execution model where the base specification (`uws1`) provides abstract orchestration logic, and concrete engines (`udon`) provide specialized runtime implementations.

## The Core Technique: Bound Runtime on the Base Document

The current execution model keeps orchestration on the base UWS structs and binds a specialized runtime at execution time.

### 1. The Runtime Reference Lives on the Base Document
The base struct (`Document`) includes a `Runtime` interface field. The runtime provides only leaf execution and expression/item evaluation.

```go
type Document struct {
    Runtime Runtime
}
```

### 2. Structural Execution Stays in UWS Core
`Document.Execute()` constructs an `Orchestrator` and the orchestrator walks workflows, steps, and dependencies. Leaf operations are delegated to the bound runtime.

```go
func (d *Document) Execute() error {
    orch := NewOrchestrator(d, d.Runtime)
    return orch.Execute(context.Background())
}
```

### 3. Binding Happens at Execution Time
The specialized engine binds its runtime implementation to the document before execution.

```go
func ExecuteWithRuntime(ctx context.Context, doc *uws1.Document, rt Runtime) error {
    doc.SetRuntime(rt)
    return doc.Execute(ctx)
}
```

## Benefits for UWS and Udon
- **Single Source of Truth:** Structural orchestration logic (loops, switches, parallel, merge, await) is defined once in `uws1`.
- **Decoupling:** `uws1` has zero knowledge of any concrete engine's specific runtimes (SSH, HTTP, etc.).
- **Consistency:** All UWS-compliant executors share the same core orchestration behavior and bind their own runtime implementations at execution time.

## References
- [Peter Bi, "Achieving Full Object Inheritance in Go", Medium, 2024](https://medium.com/@peterbi_91340/implement-true-inheritance-in-go-ff6243bfd7a8)
