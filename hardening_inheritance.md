# Implementation Plan: Hardening Inheritance Model & Refactoring

## Objective
Harden the "True Inheritance" pattern by fully implementing structural orchestration in `uws1` and improving the `Runtime` interface. This allows `udon` to focus exclusively on leaf execution while `uws` handles the complex workflow logic.

## Key Changes

### 1. UWS Core (`uws` repository)

#### `uws1/execution.go`
- **Fully Implement Orchestrator**: Add normative logic for `parallel`, `switch`, and `loop` constructs.
- **Enhance `Runtime` Interface**: Add `ResolveItems(ctx, expr) ([]any, error)` to handle loop/switch data resolution.
- **Context Propagation**: Ensure `context.Context` is correctly passed through all orchestration methods.

#### `uws1/workflow.go` & `uws1/operation.go`
- **Self-Binding Implementation**: Ensure that `Execute()` methods on `Workflow`, `Step`, and `Operation` are the primary entry points and always delegate to the `Runtime` bound at the `Document` level.

### 2. Udon Specialization (`udon` repository)

#### `spider/execute.go`
- **Bridge to Existing Engine**: Implement the new `Runtime` methods (`ResolveItems`) and connect `ExecuteLeaf` to the existing `spider.RunLeaf` logic.
- **Remove Redundant Logic**: Delete the structural execution logic in `spider` that is now handled by the `uws1.Orchestrator`.

#### `pkg/workflow/workflow.go`
- **Clean Up Shadowed Fields**: Refine the relationship between `Program` and `Document` to minimize field duplication. Ensure that parsing directly populates `uws1.Document` where possible.

## Verification & Testing

### 1. Orchestration Tests in `uws1`
- Add comprehensive test cases for parallel execution, nested loops, and switch branching using mock runtimes.

### 2. Integration Tests in `udon`
- Run the full suite of `udon` integration tests to ensure that the new `uws1` orchestration logic correctly executes `udon`'s leaf runtimes (SSH, HTTP).

## Migration strategy
- This refactor will be applied incrementally, starting with the `uws1` Orchestrator hardening, followed by the `udon` engine integration.
