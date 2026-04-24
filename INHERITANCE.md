# INHERITANCE: Implementation Plan

This document tracks the progress of the inheritance-based execution refactor for the UWS (`uws`) and Udon (`udon`) repositories. The current model keeps structural orchestration in `uws1` and binds a specialized runtime implementation at execution time.

## Objective
Refactor the repositories to adopt an inheritance-based execution model. `uws1` acts as the executable contract and orchestrator, while `udon` provides the bound runtime implementation for leaf execution and expression evaluation.

## Key Changes

### 1. UWS Core (`uws` repository)

#### `AGENTS.md`
- **Status:** [DONE]
- **Context:** Architectural guide documenting the "True Inheritance" pattern (Interface-Backed Embedding, Cross-Type Method Calls, and Late Binding).

#### `versions/1.0.0.md`
- **Status:** [DONE]
- **Context:** Formalized the Execution Model in Section 7. Added definitions for the Orchestrator and Runtime Interface.

#### `uws1/execution.go` (New)
- **Status:** [DONE]
- **Context:** Defined the `Runtime` interface and the `Orchestrator` base logic for structural constructs.

#### `uws1/document.go`
- **Status:** [DONE]
- **Context:** Added `Runtime` field to `Document` and implemented `Execute()` and `SetRuntime()`.

#### `uws1/workflow.go` & `uws1/operation.go`
- **Status:** [DONE]
- **Context:** Added `Execute()` methods to `Workflow`, `Step`, and `Operation` to support delegated execution.

#### `uws.json`
- **Status:** [DONE]
- **Context:** Reviewed schema; no changes needed as execution fields are shadowed or unexported in serialization.

### 2. Udon Specialization (`udon` repository)

#### `pkg/workflow/workflow.go`
- **Status:** [DONE]
- **Context:** `Program` embeds `uws1.Document` and keeps canonical HCL-only wrapper views (`Steps`/`Triggers`) derived from the embedded document.

#### `pkg/spider/execute.go`
- **Status:** [DONE]
- **Context:** `Spider` implements `uws1.Runtime`. `ExecuteRuntimePlan` now binds `Spider` to the document with `doc.SetRuntime(spider)` and executes the upstream orchestrator.

#### `pkg/uwsruntime/convert.go`
- **Status:** [IN PROGRESS]
- **Context:** Mapping logic is being reduced so canonical HCL wrappers and UWS documents share one semantic source of truth.

## Verification & Testing

### 1. Unit Tests in `uws1`
- **Status:** [DONE]
- **Context:** Verified `Orchestrator` handles structural logic correctly with mock runtimes.

### 2. Integration Tests in `udon`
- **Status:** [IN PROGRESS]
- **Context:** Runtime-plan execution now goes through `uws1.Document.Execute`, but legacy IR-only execution paths still exist for compatibility and resume flows.

### 3. Specification Compliance
- **Status:** [DONE]
- **Context:** `uws1` validation tests pass.

## Migration Strategy
- **Status:** [IN PROGRESS]
- **Context:** Structural execution is upstream. Remaining work is to keep the embedded `uws1.Document` authoritative across canonical HCL parse/import/export paths and to retire remaining legacy-only bridges.
