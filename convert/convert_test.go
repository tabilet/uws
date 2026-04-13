package convert

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/tabilet/uws/uws1"
)

func TestJSONToHCL(t *testing.T) {
	jsonData := []byte(`{
		"uws": "1.0.0",
		"info": {
			"title": "Pet Store Workflow",
			"version": "1.0.0"
		},
		"operations": [
			{
				"operationId": "list_pets",
				"serviceType": "http",
				"method": "GET",
				"path": "/pets"
			}
		]
	}`)

	hclData, err := JSONToHCL(jsonData)
	if err != nil {
		t.Fatalf("JSONToHCL failed: %v", err)
	}

	hclStr := string(hclData)

	if !strings.Contains(hclStr, "uws") {
		t.Error("HCL output missing 'uws'")
	}
	if !strings.Contains(hclStr, "info") {
		t.Error("HCL output missing 'info' block")
	}
	if !strings.Contains(hclStr, "operation") {
		t.Error("HCL output missing 'operation' block")
	}
}

func TestHCLToJSON(t *testing.T) {
	hclData := []byte(`
uws = "1.0.0"

info {
  title   = "Pet Store Workflow"
  version = "1.0.0"
}

operation "list_pets" {
  serviceType = "http"
  method      = "GET"
  path        = "/pets"
}
`)

	jsonData, err := HCLToJSON(hclData)
	if err != nil {
		t.Fatalf("HCLToJSON failed: %v", err)
	}

	var doc uws1.Document
	if err := json.Unmarshal(jsonData, &doc); err != nil {
		t.Fatalf("Failed to parse JSON output: %v", err)
	}

	if doc.UWS != "1.0.0" {
		t.Errorf("Expected uws '1.0.0', got '%s'", doc.UWS)
	}
	if doc.Info == nil || doc.Info.Title != "Pet Store Workflow" {
		t.Error("Info not properly converted")
	}
	if len(doc.Operations) != 1 || doc.Operations[0].OperationID != "list_pets" {
		t.Error("Operations not properly converted")
	}
}

func TestMarshalUnmarshalHCL(t *testing.T) {
	doc := &uws1.Document{
		UWS: "1.0.0",
		Info: &uws1.Info{
			Title:       "Test API",
			Summary:     "A test workflow",
			Description: "This is a test",
			Version:     "1.0.0",
		},
		Operations: []*uws1.Operation{
			{
				OperationID: "get_user",
				ServiceType: "http",
				Method:      "GET",
				Path:        "/users/1",
				SuccessCriteria: []*uws1.Criterion{
					{
						Condition: "$statusCode == 200",
						Type:      uws1.CriterionSimple,
					},
				},
			},
		},
		Workflows: []*uws1.Workflow{
			{
				WorkflowID:  "test-workflow",
				Type:        "parallel",
				Description: "A workflow for testing",
				Steps: []*uws1.Step{
					{
						StepID:       "step1",
						Type:         "http",
						OperationRef: "get_user",
					},
				},
				Outputs: map[string]string{
					"userId": "$steps.step1.outputs.id",
				},
			},
		},
	}

	hclData, err := MarshalHCL(doc)
	if err != nil {
		t.Fatalf("MarshalHCL failed: %v", err)
	}

	var doc2 uws1.Document
	if err := UnmarshalHCL(hclData, &doc2); err != nil {
		t.Fatalf("UnmarshalHCL failed: %v", err)
	}

	if doc2.UWS != doc.UWS {
		t.Errorf("UWS version mismatch: got %s, want %s", doc2.UWS, doc.UWS)
	}
	if doc2.Info.Title != doc.Info.Title {
		t.Errorf("Title mismatch: got %s, want %s", doc2.Info.Title, doc.Info.Title)
	}
	if len(doc2.Operations) != len(doc.Operations) {
		t.Errorf("Operation count mismatch: got %d, want %d", len(doc2.Operations), len(doc.Operations))
	}
	if len(doc2.Workflows) != len(doc.Workflows) {
		t.Errorf("Workflow count mismatch: got %d, want %d", len(doc2.Workflows), len(doc.Workflows))
	}
}

func TestComplexDocumentConversion(t *testing.T) {
	jsonData := []byte(`{
		"uws": "1.0.0",
		"info": {
			"title": "Complex Workflow",
			"version": "2.0.0",
			"summary": "A complex test workflow",
			"description": "Testing all features"
		},
		"sourceDescriptions": [
			{"name": "api1", "url": "./api1.json", "type": "openapi"},
			{"name": "api2", "url": "./api2.json", "type": "arazzo"}
		],
		"operations": [
			{
				"operationId": "create_resource",
				"serviceType": "http",
				"method": "POST",
				"path": "/resources",
				"successCriteria": [
					{"condition": "$statusCode == 201", "type": "simple"}
				],
				"onFailure": [
					{
						"name": "retry_create",
						"type": "retry",
						"retryAfter": 2,
						"retryLimit": 3
					}
				],
				"outputs": {
					"resourceId": "$response.body.id"
				}
			},
			{
				"operationId": "verify_resource",
				"serviceType": "http",
				"method": "GET",
				"path": "/resources/1",
				"dependsOn": ["create_resource"],
				"onSuccess": [
					{
						"name": "goto_workflow",
						"type": "goto",
						"workflowId": "parallel_checks"
					}
				]
			}
		],
		"workflows": [
			{
				"workflowId": "parallel_checks",
				"type": "parallel",
				"dependsOn": ["create_resource"],
				"steps": [
					{
						"stepId": "validate",
						"type": "http",
						"operationRef": "verify_resource",
						"body": {"path": "/validate"}
					},
					{
						"stepId": "log",
						"type": "cmd",
						"body": {"command": "echo", "arguments": ["done"]}
					}
				],
				"outputs": {
					"result": "$steps.validate.outputs.body"
				}
			}
		]
	}`)

	hclData, err := JSONToHCL(jsonData)
	if err != nil {
		t.Fatalf("JSONToHCL failed: %v", err)
	}

	t.Logf("Generated HCL:\n%s", string(hclData))

	jsonData2, err := HCLToJSON(hclData)
	if err != nil {
		t.Fatalf("HCLToJSON failed: %v", err)
	}

	var doc1, doc2 uws1.Document
	if err := json.Unmarshal(jsonData, &doc1); err != nil {
		t.Fatalf("Failed to parse original JSON: %v", err)
	}
	if err := json.Unmarshal(jsonData2, &doc2); err != nil {
		t.Fatalf("Failed to parse converted JSON: %v", err)
	}

	if doc1.UWS != doc2.UWS {
		t.Errorf("UWS version mismatch after round-trip")
	}
	if doc1.Info.Title != doc2.Info.Title {
		t.Errorf("Title mismatch after round-trip")
	}
	if len(doc1.SourceDescriptions) != len(doc2.SourceDescriptions) {
		t.Errorf("SourceDescriptions count mismatch after round-trip")
	}
	if len(doc1.Operations) != len(doc2.Operations) {
		t.Errorf("Operations count mismatch after round-trip")
	}
	if len(doc1.Workflows) != len(doc2.Workflows) {
		t.Errorf("Workflows count mismatch after round-trip")
	}
	if len(doc1.Workflows[0].Steps) != len(doc2.Workflows[0].Steps) {
		t.Errorf("Steps count mismatch after round-trip")
	}
}

func TestRoundTripPreservesCriteriaAndActions(t *testing.T) {
	doc := &uws1.Document{
		UWS: "1.0.0",
		Info: &uws1.Info{
			Title:   "Test",
			Version: "1.0.0",
		},
		Operations: []*uws1.Operation{
			{
				OperationID: "get_user",
				ServiceType: "http",
				Method:      "GET",
				Path:        "/users/1",
				SuccessCriteria: []*uws1.Criterion{
					{
						Condition: "$statusCode == 200",
						Type:      uws1.CriterionSimple,
					},
					{
						Condition: "^\\{",
						Type:      uws1.CriterionRegex,
						Context:   "$response.body",
					},
				},
				OnSuccess: []*uws1.SuccessAction{
					{
						Name:   "continue",
						Type:   "goto",
						StepID: "nextStep",
						Criteria: []*uws1.Criterion{
							{
								Condition: "$statusCode == 200",
								Type:      uws1.CriterionSimple,
							},
						},
					},
				},
				OnFailure: []*uws1.FailureAction{
					{
						Name:       "retry_once",
						Type:       "retry",
						RetryAfter: 1.5,
						RetryLimit: 3,
					},
				},
			},
		},
	}

	jsonData, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("Failed to marshal to JSON: %v", err)
	}

	hclData, err := JSONToHCL(jsonData)
	if err != nil {
		t.Fatalf("JSONToHCL failed: %v", err)
	}

	jsonData2, err := HCLToJSON(hclData)
	if err != nil {
		t.Fatalf("HCLToJSON failed: %v", err)
	}

	var doc2 uws1.Document
	if err := json.Unmarshal(jsonData2, &doc2); err != nil {
		t.Fatalf("Failed to parse JSON output: %v", err)
	}

	op := doc2.Operations[0]
	if len(op.SuccessCriteria) != 2 {
		t.Fatalf("Expected 2 success criteria, got %d", len(op.SuccessCriteria))
	}
	if op.SuccessCriteria[0].Type != uws1.CriterionSimple {
		t.Errorf("Expected criterion type 'simple', got %q", op.SuccessCriteria[0].Type)
	}
	if op.SuccessCriteria[1].Type != uws1.CriterionRegex {
		t.Errorf("Expected criterion type 'regex', got %q", op.SuccessCriteria[1].Type)
	}

	if len(op.OnSuccess) != 1 {
		t.Fatalf("Expected 1 onSuccess action, got %d", len(op.OnSuccess))
	}
	if op.OnSuccess[0].StepID != "nextStep" {
		t.Errorf("Expected onSuccess stepId 'nextStep', got %q", op.OnSuccess[0].StepID)
	}
	if len(op.OnSuccess[0].Criteria) != 1 {
		t.Fatalf("Expected 1 onSuccess criterion, got %d", len(op.OnSuccess[0].Criteria))
	}

	if len(op.OnFailure) != 1 {
		t.Fatalf("Expected 1 onFailure action, got %d", len(op.OnFailure))
	}
	if op.OnFailure[0].RetryLimit != 3 {
		t.Errorf("Expected retryLimit 3, got %d", op.OnFailure[0].RetryLimit)
	}
}

func TestHCLConversionPreservesDollarKeys(t *testing.T) {
	jsonData := []byte(`{
		"uws": "1.0.0",
		"info": {
			"title": "Dollar Key Test",
			"version": "1.0.0"
		},
		"variables": {
			"$custom": "value",
			"regular": "normal"
		},
		"operations": [
			{
				"operationId": "op1",
				"serviceType": "http",
				"method": "GET",
				"path": "/test",
				"request": {
					"$meta": {"id": 1},
					"name": "test"
				}
			}
		]
	}`)

	hclData, err := JSONToHCL(jsonData)
	if err != nil {
		t.Fatalf("JSONToHCL failed: %v", err)
	}

	hclStr := string(hclData)
	if !strings.Contains(hclStr, "__dollar__custom") {
		t.Error("HCL output missing transformed $custom key")
	}
	if !strings.Contains(hclStr, "__dollar__meta") {
		t.Error("HCL output missing transformed $meta key")
	}

	jsonData2, err := HCLToJSON(hclData)
	if err != nil {
		t.Fatalf("HCLToJSON failed: %v", err)
	}

	var doc uws1.Document
	if err := json.Unmarshal(jsonData2, &doc); err != nil {
		t.Fatalf("Failed to parse JSON output: %v", err)
	}

	if doc.Variables == nil {
		t.Fatal("Variables should not be nil")
	}
	if _, ok := doc.Variables["$custom"]; !ok {
		t.Error("Expected $custom key in variables after round-trip")
	}

	if doc.Operations[0].Request == nil {
		t.Fatal("Request should not be nil")
	}
	if _, ok := doc.Operations[0].Request["$meta"]; !ok {
		t.Error("Expected $meta key in request after round-trip")
	}
}

func TestHCLToJSONIndent(t *testing.T) {
	hclData := []byte(`
uws = "1.0.0"

info {
  title   = "Test"
  version = "1.0.0"
}

operation "op1" {
  serviceType = "http"
  method      = "GET"
  path        = "/test"
}
`)

	jsonData, err := HCLToJSONIndent(hclData, "", "  ")
	if err != nil {
		t.Fatalf("HCLToJSONIndent failed: %v", err)
	}

	if !strings.Contains(string(jsonData), "\n") {
		t.Error("JSON output is not indented")
	}

	var doc uws1.Document
	if err := json.Unmarshal(jsonData, &doc); err != nil {
		t.Fatalf("Output is not valid JSON: %v", err)
	}
}

func TestCmdAndFnctOperations(t *testing.T) {
	jsonData := []byte(`{
		"uws": "1.0.0",
		"info": {
			"title": "Multi-Service Test",
			"version": "1.0.0"
		},
		"operations": [
			{
				"operationId": "check_disk",
				"serviceType": "cmd",
				"command": "df",
				"arguments": ["-h", "/"],
				"workingDir": "/tmp"
			},
			{
				"operationId": "compute_hash",
				"serviceType": "fnct",
				"function": "crypto.SHA256",
				"arguments": ["input_data"]
			}
		]
	}`)

	hclData, err := JSONToHCL(jsonData)
	if err != nil {
		t.Fatalf("JSONToHCL failed: %v", err)
	}

	t.Logf("HCL:\n%s", string(hclData))

	jsonData2, err := HCLToJSON(hclData)
	if err != nil {
		t.Fatalf("HCLToJSON failed: %v", err)
	}

	var doc uws1.Document
	if err := json.Unmarshal(jsonData2, &doc); err != nil {
		t.Fatalf("Failed to parse JSON output: %v", err)
	}

	if len(doc.Operations) != 2 {
		t.Fatalf("Expected 2 operations, got %d", len(doc.Operations))
	}

	cmdOp := doc.Operations[0]
	if cmdOp.ServiceType != "cmd" {
		t.Errorf("Expected serviceType 'cmd', got %q", cmdOp.ServiceType)
	}
	if cmdOp.Command != "df" {
		t.Errorf("Expected command 'df', got %q", cmdOp.Command)
	}

	fnctOp := doc.Operations[1]
	if fnctOp.ServiceType != "fnct" {
		t.Errorf("Expected serviceType 'fnct', got %q", fnctOp.ServiceType)
	}
	if fnctOp.Function != "crypto.SHA256" {
		t.Errorf("Expected function 'crypto.SHA256', got %q", fnctOp.Function)
	}
}

func TestWorkflowWithTriggersAndSecurity(t *testing.T) {
	jsonData := []byte(`{
		"uws": "1.0.0",
		"info": {
			"title": "Full Feature Test",
			"version": "1.0.0"
		},
		"operations": [
			{
				"operationId": "op1",
				"serviceType": "http",
				"method": "GET",
				"path": "/test"
			}
		],
		"triggers": [
			{
				"triggerId": "webhook",
				"path": "/webhooks/test",
				"methods": ["POST"],
				"authentication": "bearer",
				"options": {"timeout": 30},
				"routes": [
					{"output": "0", "to": ["op1"]}
				]
			}
		],
		"security": [
			{
				"name": "bearer_auth",
				"scheme": {
					"type": "http",
					"scheme": "bearer"
				}
			}
		]
	}`)

	hclData, err := JSONToHCL(jsonData)
	if err != nil {
		t.Fatalf("JSONToHCL failed: %v", err)
	}

	jsonData2, err := HCLToJSON(hclData)
	if err != nil {
		t.Fatalf("HCLToJSON failed: %v", err)
	}

	var doc uws1.Document
	if err := json.Unmarshal(jsonData2, &doc); err != nil {
		t.Fatalf("Failed to parse JSON output: %v", err)
	}

	if len(doc.Triggers) != 1 {
		t.Fatalf("Expected 1 trigger, got %d", len(doc.Triggers))
	}
	if doc.Triggers[0].TriggerID != "webhook" {
		t.Errorf("Expected triggerId 'webhook', got %q", doc.Triggers[0].TriggerID)
	}
	if doc.Triggers[0].Path != "/webhooks/test" {
		t.Errorf("Expected path '/webhooks/test', got %q", doc.Triggers[0].Path)
	}
	if len(doc.Security) != 1 {
		t.Fatalf("Expected 1 security requirement, got %d", len(doc.Security))
	}
}

func TestNewlineEscaping(t *testing.T) {
	doc := &uws1.Document{
		UWS: "1.0.0",
		Info: &uws1.Info{
			Title:       "Newline Test",
			Summary:     "Line 1\nLine 2",
			Description: "First line\nSecond line\nThird line",
			Version:     "1.0.0",
		},
		Operations: []*uws1.Operation{
			{
				OperationID: "op1",
				ServiceType: "http",
				Method:      "GET",
				Path:        "/test",
				Description: "Op description\nwith newlines",
			},
		},
	}

	jsonData, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("Failed to marshal to JSON: %v", err)
	}

	hclData, err := JSONToHCL(jsonData)
	if err != nil {
		t.Fatalf("JSONToHCL failed: %v", err)
	}

	jsonData2, err := HCLToJSON(hclData)
	if err != nil {
		t.Fatalf("HCLToJSON failed: %v", err)
	}

	var doc2 uws1.Document
	if err := json.Unmarshal(jsonData2, &doc2); err != nil {
		t.Fatalf("Failed to parse JSON output: %v", err)
	}

	if doc2.Info.Summary != "Line 1\nLine 2" {
		t.Errorf("Summary newlines not preserved: got %q", doc2.Info.Summary)
	}
	if doc2.Info.Description != "First line\nSecond line\nThird line" {
		t.Errorf("Description newlines not preserved: got %q", doc2.Info.Description)
	}
	if doc2.Operations[0].Description != "Op description\nwith newlines" {
		t.Errorf("Operation description newlines not preserved: got %q", doc2.Operations[0].Description)
	}
}
