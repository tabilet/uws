package convert

import (
	"encoding/json"
	"os"
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

func compareUWSDocs(t *testing.T, doc1, doc2 *uws1.Document) {
	t.Helper()

	if doc1.UWS != doc2.UWS {
		t.Errorf("UWS version mismatch: got %q, want %q", doc2.UWS, doc1.UWS)
	}

	if doc1.Info != nil && doc2.Info != nil {
		if doc1.Info.Title != doc2.Info.Title {
			t.Errorf("Info.Title mismatch: got %q, want %q", doc2.Info.Title, doc1.Info.Title)
		}
		if doc1.Info.Version != doc2.Info.Version {
			t.Errorf("Info.Version mismatch: got %q, want %q", doc2.Info.Version, doc1.Info.Version)
		}
	} else if (doc1.Info == nil) != (doc2.Info == nil) {
		t.Error("Info presence mismatch")
	}

	if len(doc1.SourceDescriptions) != len(doc2.SourceDescriptions) {
		t.Errorf("SourceDescriptions count mismatch: got %d, want %d",
			len(doc2.SourceDescriptions), len(doc1.SourceDescriptions))
	} else {
		for i := range doc1.SourceDescriptions {
			if doc1.SourceDescriptions[i].Name != doc2.SourceDescriptions[i].Name {
				t.Errorf("SourceDescriptions[%d].Name mismatch: got %q, want %q",
					i, doc2.SourceDescriptions[i].Name, doc1.SourceDescriptions[i].Name)
			}
			if doc1.SourceDescriptions[i].URL != doc2.SourceDescriptions[i].URL {
				t.Errorf("SourceDescriptions[%d].URL mismatch: got %q, want %q",
					i, doc2.SourceDescriptions[i].URL, doc1.SourceDescriptions[i].URL)
			}
		}
	}

	if len(doc1.Operations) != len(doc2.Operations) {
		t.Errorf("Operations count mismatch: got %d, want %d",
			len(doc2.Operations), len(doc1.Operations))
	} else {
		for i := range doc1.Operations {
			o1, o2 := doc1.Operations[i], doc2.Operations[i]
			if o1.OperationID != o2.OperationID {
				t.Errorf("Operations[%d].OperationID mismatch: got %q, want %q",
					i, o2.OperationID, o1.OperationID)
			}
			if o1.SourceDescription != o2.SourceDescription {
				t.Errorf("Operations[%d].SourceDescription mismatch: got %q, want %q",
					i, o2.SourceDescription, o1.SourceDescription)
			}
			if o1.OpenAPIOperationID != o2.OpenAPIOperationID {
				t.Errorf("Operations[%d].OpenAPIOperationID mismatch: got %q, want %q",
					i, o2.OpenAPIOperationID, o1.OpenAPIOperationID)
			}
			if o1.OpenAPIOperationRef != o2.OpenAPIOperationRef {
				t.Errorf("Operations[%d].OpenAPIOperationRef mismatch: got %q, want %q",
					i, o2.OpenAPIOperationRef, o1.OpenAPIOperationRef)
			}
			if len(o1.SuccessCriteria) != len(o2.SuccessCriteria) {
				t.Errorf("Operations[%d].SuccessCriteria count mismatch: got %d, want %d",
					i, len(o2.SuccessCriteria), len(o1.SuccessCriteria))
			}
			if len(o1.OnFailure) != len(o2.OnFailure) {
				t.Errorf("Operations[%d].OnFailure count mismatch: got %d, want %d",
					i, len(o2.OnFailure), len(o1.OnFailure))
			}
			if len(o1.OnSuccess) != len(o2.OnSuccess) {
				t.Errorf("Operations[%d].OnSuccess count mismatch: got %d, want %d",
					i, len(o2.OnSuccess), len(o1.OnSuccess))
			}
		}
	}

	if len(doc1.Workflows) != len(doc2.Workflows) {
		t.Errorf("Workflows count mismatch: got %d, want %d",
			len(doc2.Workflows), len(doc1.Workflows))
	} else {
		for i := range doc1.Workflows {
			w1, w2 := doc1.Workflows[i], doc2.Workflows[i]
			if w1.WorkflowID != w2.WorkflowID {
				t.Errorf("Workflows[%d].WorkflowID mismatch: got %q, want %q",
					i, w2.WorkflowID, w1.WorkflowID)
			}
			if w1.Type != w2.Type {
				t.Errorf("Workflows[%d].Type mismatch: got %q, want %q",
					i, w2.Type, w1.Type)
			}
			if len(w1.Steps) != len(w2.Steps) {
				t.Errorf("Workflows[%d].Steps count mismatch: got %d, want %d",
					i, len(w2.Steps), len(w1.Steps))
			} else {
				for j := range w1.Steps {
					s1, s2 := w1.Steps[j], w2.Steps[j]
					if s1.StepID != s2.StepID {
						t.Errorf("Workflows[%d].Steps[%d].StepID mismatch: got %q, want %q",
							i, j, s2.StepID, s1.StepID)
					}
				}
			}
		}
	}

	if len(doc1.Triggers) != len(doc2.Triggers) {
		t.Errorf("Triggers count mismatch: got %d, want %d",
			len(doc2.Triggers), len(doc1.Triggers))
	} else {
		for i := range doc1.Triggers {
			if doc1.Triggers[i].TriggerID != doc2.Triggers[i].TriggerID {
				t.Errorf("Triggers[%d].TriggerID mismatch: got %q, want %q",
					i, doc2.Triggers[i].TriggerID, doc1.Triggers[i].TriggerID)
			}
		}
	}
}
