package convert

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"

	"github.com/tabilet/uws/uws1"
)

// TestRoundTripSampleFile tests JSON-HCL-JSON round-trip with the sample testdata file.
func TestRoundTripSampleFile(t *testing.T) {
	jsonData, err := os.ReadFile("../testdata/sample.uws.json")
	if err != nil {
		t.Fatalf("Failed to read sample file: %v", err)
	}

	var doc1 uws1.Document
	if err := json.Unmarshal(jsonData, &doc1); err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}
	doc1.Extensions = nil

	// Re-marshal a core-only copy because UWS HCL conversion rejects x-* extensions.
	jsonData1, err := json.Marshal(&doc1)
	if err != nil {
		t.Fatalf("Failed to re-marshal to JSON: %v", err)
	}

	// JSON -> HCL
	hclData, err := JSONToHCL(jsonData1)
	if err != nil {
		t.Fatalf("Failed to convert JSON to HCL: %v", err)
	}

	t.Logf("HCL output:\n%s", string(hclData))

	// HCL -> JSON
	jsonData2, err := HCLToJSON(hclData)
	if err != nil {
		t.Fatalf("Failed to convert HCL to JSON: %v", err)
	}

	var doc2 uws1.Document
	if err := json.Unmarshal(jsonData2, &doc2); err != nil {
		t.Fatalf("Failed to unmarshal round-trip JSON: %v", err)
	}

	compareUWSDocs(t, &doc1, &doc2)
}

// stripExtensions clears every Extensions map in the document tree. Used by
// the HCL extension-drop roundtrip test so the expected value matches what HCL
// conversion actually preserves (core fields only).
func stripExtensions(doc *uws1.Document) {
	if doc == nil {
		return
	}
	doc.Extensions = nil
	if doc.Info != nil {
		doc.Info.Extensions = nil
	}
	for _, sd := range doc.SourceDescriptions {
		if sd != nil {
			sd.Extensions = nil
		}
	}
	for _, op := range doc.Operations {
		if op == nil {
			continue
		}
		op.Extensions = nil
		for _, c := range op.SuccessCriteria {
			if c != nil {
				c.Extensions = nil
			}
		}
		for _, a := range op.OnFailure {
			if a == nil {
				continue
			}
			a.Extensions = nil
			for _, c := range a.Criteria {
				if c != nil {
					c.Extensions = nil
				}
			}
		}
		for _, a := range op.OnSuccess {
			if a == nil {
				continue
			}
			a.Extensions = nil
			for _, c := range a.Criteria {
				if c != nil {
					c.Extensions = nil
				}
			}
		}
	}
	for _, wf := range doc.Workflows {
		if wf == nil {
			continue
		}
		wf.Extensions = nil
		stripStepsExtensions(wf.Steps)
		stripCasesExtensions(wf.Cases)
		stripStepsExtensions(wf.Default)
	}
	for _, tr := range doc.Triggers {
		if tr == nil {
			continue
		}
		tr.Extensions = nil
		for _, r := range tr.Routes {
			if r != nil {
				r.Extensions = nil
			}
		}
	}
	for _, r := range doc.Results {
		if r != nil {
			r.Extensions = nil
		}
	}
	if doc.Components != nil {
		doc.Components.Extensions = nil
	}
}

func stripStepsExtensions(steps []*uws1.Step) {
	for _, s := range steps {
		if s == nil {
			continue
		}
		s.Extensions = nil
		stripStepsExtensions(s.Steps)
		stripCasesExtensions(s.Cases)
		stripStepsExtensions(s.Default)
	}
}

func stripCasesExtensions(cases []*uws1.Case) {
	for _, c := range cases {
		if c == nil {
			continue
		}
		c.Extensions = nil
		stripStepsExtensions(c.Steps)
	}
}

// TestHCLRoundTripDropsExtensions locks the contract stated in AGENTS.md: HCL
// conversion intentionally drops x-* extensions. After JSON -> strip -> HCL ->
// JSON, the resulting document must deep-equal the stripped original.
func TestHCLRoundTripDropsExtensions(t *testing.T) {
	withExtensions := &uws1.Document{
		UWS: "1.0.0",
		Info: &uws1.Info{
			Title:      "Ext",
			Version:    "1.0.0",
			Extensions: map[string]any{"x-info": "kept-in-json"},
		},
		SourceDescriptions: []*uws1.SourceDescription{
			{Name: "api", URL: "./openapi.yaml", Type: uws1.SourceDescriptionTypeOpenAPI, Extensions: map[string]any{"x-sd": "kept-in-json"}},
		},
		Operations: []*uws1.Operation{
			{
				OperationID:        "op1",
				SourceDescription:  "api",
				OpenAPIOperationID: "getOp",
				Extensions:         map[string]any{"x-op": "kept-in-json"},
				SuccessCriteria: []*uws1.Criterion{
					{Condition: "true", Extensions: map[string]any{"x-crit": "kept-in-json"}},
				},
			},
		},
		Extensions: map[string]any{"x-root": "kept-in-json"},
	}

	// JSON -> struct -> strip -> JSON -> HCL -> JSON -> struct
	jsonWith, err := json.Marshal(withExtensions)
	if err != nil {
		t.Fatalf("marshal with extensions: %v", err)
	}
	var stripped uws1.Document
	if err := json.Unmarshal(jsonWith, &stripped); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	stripExtensions(&stripped)

	jsonStripped, err := json.Marshal(&stripped)
	if err != nil {
		t.Fatalf("marshal stripped: %v", err)
	}
	hclData, err := JSONToHCL(jsonStripped)
	if err != nil {
		t.Fatalf("JSONToHCL after strip: %v", err)
	}
	backJSON, err := HCLToJSON(hclData)
	if err != nil {
		t.Fatalf("HCLToJSON: %v", err)
	}
	var back uws1.Document
	if err := json.Unmarshal(backJSON, &back); err != nil {
		t.Fatalf("unmarshal roundtrip: %v", err)
	}

	compareUWSDocs(t, &stripped, &back)

	// And the inverse guarantee: MarshalHCL refuses to run if extensions remain.
	if _, err := MarshalHCL(withExtensions); err == nil {
		t.Fatal("MarshalHCL should reject documents with x-* extensions")
	}
}

// TestRoundTripRichDocument covers fields the sample fixture does not: Criterion.Type
// and Context, trigger Options, Components.Variables, loop/switch workflows, and
// nested Cases with bodies. Deep-equals the round-tripped document against the
// original so a regression in any field would fail loudly.
func TestRoundTripRichDocument(t *testing.T) {
	doc := &uws1.Document{
		UWS: "1.0.0",
		Info: &uws1.Info{
			Title:       "Rich",
			Summary:     "line1\nline2",
			Description: "first\nsecond",
			Version:     "1.2.3",
		},
		SourceDescriptions: []*uws1.SourceDescription{
			{Name: "api", URL: "./openapi.yaml", Type: uws1.SourceDescriptionTypeOpenAPI},
		},
		Variables: map[string]any{
			"env":  "prod",
			"nums": []any{float64(1), float64(2), float64(3)},
			"deep": map[string]any{"k": "v", "n": float64(42)},
		},
		Operations: []*uws1.Operation{
			{
				OperationID:        "op1",
				SourceDescription:  "api",
				OpenAPIOperationID: "getOp",
				Request: map[string]any{
					"query":  map[string]any{"limit": float64(10)},
					"header": map[string]any{"x-trace": "abc"},
					"body":   map[string]any{"name": "Buddy"},
				},
				SuccessCriteria: []*uws1.Criterion{
					{Condition: "$response.body.ok", Type: uws1.CriterionJSONPath, Context: "$response.body"},
				},
				OnFailure: []*uws1.FailureAction{
					{Name: "r", Type: "retry", RetryAfter: 1.5, RetryLimit: 4, Criteria: []*uws1.Criterion{
						{Condition: "$response.statusCode >= 500"},
					}},
				},
				Outputs: map[string]string{"id": "$response.body.id"},
			},
		},
		Workflows: []*uws1.Workflow{
			{
				WorkflowID: "pick",
				Type:       "switch",
				Cases: []*uws1.Case{
					{
						CaseFields: uws1.CaseFields{
							Name: "c1",
							When: "$outputs.op1.id == 1",
						},
						Body: map[string]any{"note": "first"},
						Steps: []*uws1.Step{
							{StepID: "s1", OperationRef: "op1"},
						},
					},
				},
				Default: []*uws1.Step{
					{StepID: "d1", OperationRef: "op1"},
				},
			},
			{
				WorkflowID: "iter",
				Type:       "loop",
				StructuralFields: uws1.StructuralFields{
					Items: "$outputs.op1.ids",
				},
				Steps: []*uws1.Step{
					{StepID: "body", OperationRef: "op1"},
				},
			},
		},
		Triggers: []*uws1.Trigger{
			{
				TriggerID: "hook",
				TriggerFields: uws1.TriggerFields{
					Path:           "/hook",
					Methods:        []string{"POST"},
					Authentication: "bearer",
				},
				Options: map[string]any{"timeout": float64(30), "nested": map[string]any{"k": "v"}},
				Outputs: []string{"received"},
				Routes: []*uws1.TriggerRoute{
					{TriggerRouteFields: uws1.TriggerRouteFields{Output: "received", To: []string{"op1"}}},
				},
			},
		},
		Components: &uws1.Components{
			Variables: map[string]any{"shared": "yes"},
		},
	}

	jsonData, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	hclData, err := JSONToHCL(jsonData)
	if err != nil {
		t.Fatalf("JSONToHCL: %v", err)
	}
	backJSON, err := HCLToJSON(hclData)
	if err != nil {
		t.Fatalf("HCLToJSON: %v", err)
	}
	var back uws1.Document
	if err := json.Unmarshal(backJSON, &back); err != nil {
		t.Fatalf("unmarshal roundtrip: %v", err)
	}

	compareUWSDocs(t, doc, &back)
}

// compareUWSDocs asserts the two documents are deeply equal. On mismatch it
// prints indented JSON of both sides so the diff is readable.
func compareUWSDocs(t *testing.T, doc1, doc2 *uws1.Document) {
	t.Helper()
	if reflect.DeepEqual(doc1, doc2) {
		return
	}
	a, _ := json.MarshalIndent(doc1, "", "  ")
	b, _ := json.MarshalIndent(doc2, "", "  ")
	t.Fatalf("roundtrip mismatch:\nwant:\n%s\ngot:\n%s", a, b)
}
