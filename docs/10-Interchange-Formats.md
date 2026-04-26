# Feature 10: Interchange Formats

← [Validation](09-Validation) | [Home →](Home)

---

UWS documents are valid JSON, YAML, or canonical HCL. The `convert` package in `github.com/tabilet/uws` moves documents between all three formats with round-trip guarantees.

## Three Formats, One Document

| Format | Best for | Extensions preserved |
|--------|----------|---------------------|
| JSON | Machine interchange, API responses, LLM output | ✓ |
| YAML | Human authoring, configuration files | ✓ |
| HCL | Canonical authoring for the `udon` runtime | ✗ (rejected if present) |

## Example 1: The Same Operation in All Three Formats

The following is semantically identical in JSON, YAML, and HCL:

**JSON**
```json
{
  "operationId": "list_pets",
  "sourceDescription": "petstore_api",
  "openapiOperationId": "listPets",
  "request": {
    "query": { "limit": 10, "status": "available" },
    "header": { "X-Trace-Id": "abc-123" }
  },
  "outputs": {
    "firstPet": "$response.body#/0",
    "total": "$response.body.total"
  }
}
```

**YAML**
```yaml
operationId: list_pets
sourceDescription: petstore_api
openapiOperationId: listPets
request:
  query:
    limit: 10
    status: available
  header:
    X-Trace-Id: abc-123
outputs:
  firstPet: $response.body#/0
  total: $response.body.total
```

**HCL**
```hcl
operation "list_pets" {
  sourceDescription  = "petstore_api"
  openapiOperationId = "listPets"

  request = {
    query  = { limit = 10, status = "available" }
    header = { "X-Trace-Id" = "abc-123" }
  }

  outputs = {
    firstPet = "$response.body#/0"
    total    = "$response.body.total"
  }
}
```

## Example 2: A Full Document in All Three Formats

A complete minimal document with a workflow:

**YAML** (most readable for authoring)
```yaml
uws: "1.0.0"
info:
  title: Pet Workflow
  version: 1.0.0
sourceDescriptions:
  - name: petstore_api
    url: ./petstore.yaml
    type: openapi
operations:
  - operationId: list_pets
    sourceDescription: petstore_api
    openapiOperationId: listPets
    request:
      query:
        limit: 5
    outputs:
      first: $response.body#/0/id
workflows:
  - workflowId: main
    type: sequence
    steps:
      - stepId: fetch
        operationRef: list_pets
```

**HCL** (canonical authoring form)
```hcl
uws  = "1.0.0"

info {
  title   = "Pet Workflow"
  version = "1.0.0"
}

sourceDescription "petstore_api" {
  url  = "./petstore.yaml"
  type = "openapi"
}

operation "list_pets" {
  sourceDescription  = "petstore_api"
  openapiOperationId = "listPets"
  request = { query = { limit = 5 } }
  outputs = { first = "$response.body#/0/id" }
}

workflow "main" {
  type = "sequence"
  step "fetch" {
    operationRef = "list_pets"
  }
}
```

## Example 3: Switch Case in HCL

Structural constructs translate naturally:

**YAML**
```yaml
workflows:
  - workflowId: route
    type: switch
    cases:
      - name: premium
        when: $outputs.tier == "premium"
        steps:
          - stepId: fast_track
            operationRef: express_process
    default:
      - stepId: standard
        operationRef: normal_process
```

**HCL**
```hcl
workflow "route" {
  type = "switch"

  case "premium" {
    when = "$outputs.tier == \"premium\""
    step "fast_track" {
      operationRef = "express_process"
    }
  }

  default {
    step "standard" {
      operationRef = "normal_process"
    }
  }
}
```

## The `convert` Package

All conversion helpers live in `github.com/tabilet/uws/convert`:

```go
// Between byte slices (raw format conversion)
jsonOut, _ := convert.YAMLToJSON(yamlData)
yamlOut, _ := convert.JSONToYAML(jsonData)
hclOut,  _ := convert.JSONToHCL(jsonData)   // errors if x-* extensions present
jsonOut, _ = convert.HCLToJSON(hclData)

// Marshal a Document struct to bytes
jsonBytes, _ := convert.MarshalJSON(doc)
yamlBytes, _ := convert.MarshalYAML(doc)
hclBytes,  _ := convert.MarshalHCL(doc)     // errors if x-* extensions present

// Unmarshal bytes into a Document struct
convert.UnmarshalJSON(jsonData, &doc)
convert.UnmarshalYAML(yamlData, &doc)
convert.UnmarshalHCL(hclData, &doc)
```

## Example 4: Format Conversion in a Go Program

Read a YAML workflow, validate it, and write it back as JSON for machine consumption:

```go
package main

import (
    "log"
    "os"

    "github.com/tabilet/uws/convert"
    "github.com/tabilet/uws/uws1"
)

func main() {
    yamlData, _ := os.ReadFile("workflow.uws.yaml")

    var doc uws1.Document
    if err := convert.UnmarshalYAML(yamlData, &doc); err != nil {
        log.Fatal(err)
    }
    if err := doc.Validate(); err != nil {
        log.Fatal(err)
    }

    jsonData, err := convert.MarshalJSONIndent(&doc, "", "  ")
    if err != nil {
        log.Fatal(err)
    }
    os.WriteFile("workflow.uws.json", jsonData, 0644)
}
```

## HCL Extension Rejection

HCL is a core-only format. `MarshalHCL` rejects documents that carry `x-*` extension fields rather than silently dropping them:

```go
// Document with an extension field
doc := &uws1.Document{
    Operations: []*uws1.Operation{
        {
            OperationID: "op1",
            Extensions:  map[string]any{"x-timeout": 30},  // extension present
        },
    },
}

_, err := convert.MarshalHCL(doc)
// err: "operations[0] contains x-* extensions; UWS HCL conversion is
//       core-only and cannot preserve extension profiles, use JSON or YAML"
```

This is the "reject rather than silently lose" principle. If you need to author a document with extensions, use YAML or JSON.

## `$`-Key Rewriting for HCL

JSON Schema keys like `$ref`, `$id`, `$defs` are not valid HCL identifiers. The package rewrites them symmetrically in both directions:

| JSON / YAML key | HCL key |
|-----------------|---------|
| `$ref` | `_ref` |
| `$id` | `_id` |
| `$schema` | `_schema` |
| `$defs` | `_defs` |
| `$customKey` | `__dollar__customKey` |

**A `ParamSchema` with `$ref` in YAML and HCL:**

```yaml
# YAML
inputs:
  _ref: "#/components/schemas/OrderInput"
```

```hcl
# HCL — $ref becomes _ref
inputs = { _ref = "#/components/schemas/OrderInput" }
```

Round-tripping through HCL → JSON restores `$ref` exactly.

## Round-Trip Guarantee

For any core-only (extension-free) UWS document:

```
JSON → HCL → JSON  produces a structurally identical document
YAML → HCL → YAML  produces a structurally identical document
```

`MarshalHCL` works on a deep copy — the caller's document is never mutated during conversion.

---

← [Validation](09-Validation) | [Home →](Home)
