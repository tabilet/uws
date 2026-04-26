# Feature 9: Validation

← [Extension Profiles](08-Extension-Profiles.md) | [Next: Interchange Formats →](10-Interchange-Formats.md)

---

`github.com/tabilet/uws` layers two kinds of validation. Running both catches errors at the contract boundary — before any HTTP call leaves the machine.

## Two Layers

### Layer 1: Structural (JSON Schema)

The published JSON Schema (`uws.json`) validates document shape:

- Required fields (`uws`, `info`, `operations`)
- Type and format constraints
- Enum values (workflow types, action types, criterion types)
- Pattern constraints (ID formats, output label formats)
- Conditional rules (e.g. `sourceDescriptions` required when any operation declares `sourceDescription`)
- The operation three-shape `oneOf`
- `unevaluatedProperties: false` on every object type

### Layer 2: Semantic (Go validator)

`(*uws1.Document).Validate()` and `ValidateResult()` catch rules the schema cannot express:

- Duplicate identifiers (operationId, workflowId, stepId, triggerId, result names)
- Reference integrity across operations, source descriptions, workflows, steps, trigger routes
- Operation binding mutual exclusivity
- Structural-type field constraints (`loop` requires `items`, `await` requires `wait`, etc.)
- `StructuralResult.from` linkage and kind/type match
- `dependsOn` cycle detection
- Component variable key patterns

## The Go API

**`Validate()`** — returns the first error as a single `error`, suitable for a quick pass:

```go
if err := doc.Validate(); err != nil {
    log.Fatal(err)
}
```

**`ValidateResult()`** — returns every error with a dotted path:

```go
result := doc.ValidateResult()
if !result.Valid() {
    for _, issue := range result.Errors {
        fmt.Printf("%s: %s\n", issue.Path, issue.Message)
    }
}
```

## Example 1: Common Errors and Their Path-Tagged Output

A document with several mistakes at once:

```yaml
uws: "1.0.0"
info:
  title: Broken Workflow
  version: 1.0.0
operations:
  - operationId: fetch
    sourceDescription: missing_api    # sourceDescription not declared
    openapiOperationId: getData
    onFailure:
      - name: bad_retry
        type: retry
        # retryLimit is missing
  - operationId: fetch                # duplicate operationId
    x-uws-operation-profile: udon
```

`ValidateResult()` output:

```
sourceDescriptions: is required when any operation declares sourceDescription; operations[0].sourceDescription is "missing_api"
operations[0].sourceDescription: references unknown sourceDescription "missing_api"
operations[0].onFailure[0]: retry requires retryLimit > 0
operations[1].operationId: duplicate operationId "fetch"
```

Each error names the exact field and what is wrong — directly actionable.

## Example 2: Reference Integrity Errors

```yaml
workflows:
  - workflowId: main
    type: sequence
    steps:
      - stepId: s1
        operationRef: nonexistent_op    # operation not declared
      - stepId: s1                      # duplicate stepId
        workflow: ghost_workflow        # workflow not declared
```

```
workflows[0].steps[0].operationRef: references unknown operationId "nonexistent_op"
workflows[0].steps[1].stepId: duplicate stepId "s1"
workflows[0].steps[1].workflow: references unknown workflowId "ghost_workflow"
```

## Example 3: Dependency Cycle Detection

```yaml
operations:
  - operationId: a
    sourceDescription: api
    openapiOperationId: opA
    dependsOn: [b]

  - operationId: b
    sourceDescription: api
    openapiOperationId: opB
    dependsOn: [a]    # a depends on b, b depends on a → cycle
```

```
dependsOn: cycle detected: a -> b -> a
```

## Complete Parse-and-Validate Example

```go
package main

import (
    "fmt"
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
        log.Fatalf("parse error: %v", err)
    }

    result := doc.ValidateResult()
    if !result.Valid() {
        fmt.Println("Validation failed:")
        for _, issue := range result.Errors {
            fmt.Printf("  %s: %s\n", issue.Path, issue.Message)
        }
        os.Exit(1)
    }

    fmt.Println("Document is valid.")
}
```

## Example 4: AI Agent Corrective Loop

Path-tagged errors are structured enough to feed directly back to a language model:

```go
for attempt := 0; attempt < 3; attempt++ {
    proposedYAML := agent.ProduceWorkflow(userIntent)

    var doc uws1.Document
    if err := convert.UnmarshalYAML([]byte(proposedYAML), &doc); err != nil {
        userIntent = fmt.Sprintf("Previous attempt had a parse error: %v\n\nOriginal request: %s", err, userIntent)
        continue
    }

    result := doc.ValidateResult()
    if result.Valid() {
        doc.SetRuntime(rt)
        return doc.Execute(ctx)
    }

    // Build corrective prompt from path-tagged errors
    var errLines []string
    for _, e := range result.Errors {
        errLines = append(errLines, fmt.Sprintf("%s: %s", e.Path, e.Message))
    }
    userIntent = fmt.Sprintf(
        "The workflow had validation errors. Fix them and try again:\n%s\n\nOriginal request: %s",
        strings.Join(errLines, "\n"), userIntent,
    )
}
```

Errors like `operations[0].onFailure[0]: retry requires retryLimit > 0` give the model exactly what to fix without prose interpretation.

## Schema/Code/Spec Sync

The three artifacts that define UWS are kept in sync by a reflection-driven test suite:

- **`TestSchemaParity_StructTagsMatchKnownFields`** — for every Go struct with an `Extensions` field, verifies that struct JSON tags exactly match its `knownFields` list. A mismatch means the unmarshaller would reject valid documents or silently accept invalid ones.
- **`TestSchemaParity_KnownFieldsMatchSchema`** — compares each type's `knownFields` against the corresponding `$def` in `uws.json`. Drift in either direction fails the build.
- **`TestSchemaParity_DefCoverageIsExhaustive`** — fails when `uws.json` grows a `$def` that no parity entry tracks. Tripwire for adding a new type without wiring it through the extension machinery.
- **`TestSchemaConformance_*`** — reads `uws.json` and asserts that every `required`, `enum`, and `pattern` rule the schema declares is also covered by the Go validator.

Adding a property to one artifact without updating the others fails the build immediately.

---

← [Extension Profiles](08-Extension-Profiles.md) | [Next: Interchange Formats →](10-Interchange-Formats.md)
