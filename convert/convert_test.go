package convert

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/tabilet/uws/uws1"
)

func testDocument() *uws1.Document {
	return &uws1.Document{
		UWS:  "1.0.0",
		Info: &uws1.Info{Title: "Test Workflow", Summary: "Line 1\nLine 2", Version: "1.0.0"},
		SourceDescriptions: []*uws1.SourceDescription{
			{Name: "api", URL: "./openapi.yaml", Type: uws1.SourceDescriptionTypeOpenAPI},
		},
		Variables: map[string]any{"$root": "kept"},
		Operations: []*uws1.Operation{
			{
				OperationID:        "op1",
				SourceDescription:  "api",
				OpenAPIOperationID: "getOp",
				Description:        "Line 1\nLine 2",
				Request:            map[string]any{"body": map[string]any{"$request": map[string]any{"nested": "value"}}},
			},
		},
		Workflows: []*uws1.Workflow{
			{
				WorkflowID: "wf",
				Type:       "parallel",
				Steps: []*uws1.Step{
					{StepID: "step", OperationRef: "op1", Body: map[string]any{"$body": "kept"}},
				},
			},
		},
	}
}

func TestJSONToHCL(t *testing.T) {
	jsonData := []byte(`{
		"uws": "1.0.0",
		"info": {"title": "Pet Store Workflow", "version": "1.0.0"},
		"sourceDescriptions": [{"name": "petstore_api", "url": "./petstore.yaml", "type": "openapi"}],
		"operations": [
			{
				"operationId": "list_pets",
				"sourceDescription": "petstore_api",
				"openapiOperationId": "listPets",
				"request": {"query": {"limit": 10}}
			}
		]
	}`)

	hclData, err := JSONToHCL(jsonData)
	if err != nil {
		t.Fatalf("JSONToHCL failed: %v", err)
	}

	hclStr := string(hclData)
	for _, want := range []string{"uws", "info", "sourceDescription", "operation", "openapiOperationId"} {
		if !strings.Contains(hclStr, want) {
			t.Fatalf("HCL output missing %q:\n%s", want, hclStr)
		}
	}
}

func TestMarshalHCLDoesNotMutateDocument(t *testing.T) {
	doc := testDocument()
	original, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("Failed to snapshot original document: %v", err)
	}

	if _, err := MarshalHCL(doc); err != nil {
		t.Fatalf("MarshalHCL failed: %v", err)
	}
	after, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("Failed to snapshot document after MarshalHCL: %v", err)
	}

	if !reflect.DeepEqual(original, after) {
		t.Fatalf("MarshalHCL mutated the source document:\noriginal=%s\nafter=%s", original, after)
	}
}

func TestHCLConversionTransformsNestedBodies(t *testing.T) {
	jsonData := []byte(`{
		"uws": "1.0.0",
		"info": {"title": "Nested Body Test", "version": "1.0.0"},
		"sourceDescriptions": [{"name": "api", "url": "./openapi.yaml", "type": "openapi"}],
		"operations": [{"operationId": "op1", "sourceDescription": "api", "openapiOperationId": "getOp"}],
		"workflows": [
			{
				"workflowId": "wf",
				"type": "switch",
				"steps": [
					{
						"stepId": "outer",
						"type": "switch",
						"cases": [
							{
								"name": "case_a",
								"body": {"$case": "value"},
								"steps": [
									{"stepId": "inner", "operationRef": "op1", "body": {"$inner": {"line": "a\nb"}}}
								]
							}
						],
						"default": [
							{"stepId": "fallback", "type": "sequence", "body": {"$default": "value"}}
						]
					}
				]
			}
		]
	}`)

	hclData, err := JSONToHCL(jsonData)
	if err != nil {
		t.Fatalf("JSONToHCL failed: %v", err)
	}
	hcl := string(hclData)
	for _, want := range []string{"__dollar__case", "__dollar__inner", "__dollar__default"} {
		if !strings.Contains(hcl, want) {
			t.Fatalf("HCL output missing nested transformed key %q:\n%s", want, hcl)
		}
	}

	jsonData2, err := HCLToJSON(hclData)
	if err != nil {
		t.Fatalf("HCLToJSON failed: %v", err)
	}
	var doc uws1.Document
	if err := json.Unmarshal(jsonData2, &doc); err != nil {
		t.Fatalf("Failed to parse JSON output: %v", err)
	}
	nestedCase := doc.Workflows[0].Steps[0].Cases[0]
	if _, ok := nestedCase.Body["$case"]; !ok {
		t.Fatal("Expected nested case $case key after round-trip")
	}
	if _, ok := nestedCase.Steps[0].Body["$inner"]; !ok {
		t.Fatal("Expected nested step $inner key after round-trip")
	}
	if _, ok := doc.Workflows[0].Steps[0].Default[0].Body["$default"]; !ok {
		t.Fatal("Expected nested default $default key after round-trip")
	}
}

func TestYAMLHelpersPreserveExtensions(t *testing.T) {
	yamlData := []byte(`
uws: 1.0.0
info:
  title: YAML Test
  version: 1.0.0
  x-info: true
sourceDescriptions:
  - name: api
    url: ./openapi.yaml
    type: openapi
operations:
  - operationId: op1
    sourceDescription: api
    openapiOperationId: getOp
    x-tier: 2
triggers:
  - triggerId: webhook
    routes:
      - output: "0"
        to: [op1]
        x-route: yes
results:
  - name: out
    kind: merge
    x-result: kept
x-root: yaml
`)

	var doc uws1.Document
	if err := UnmarshalYAML(yamlData, &doc); err != nil {
		t.Fatalf("UnmarshalYAML failed: %v", err)
	}
	if doc.Extensions["x-root"] != "yaml" {
		t.Fatalf("Expected root extension, got %#v", doc.Extensions)
	}
	if doc.Info.Extensions["x-info"] != true {
		t.Fatalf("Expected info extension, got %#v", doc.Info.Extensions)
	}
	if doc.Operations[0].Extensions["x-tier"] != float64(2) {
		t.Fatalf("Expected operation extension, got %#v", doc.Operations[0].Extensions)
	}
	if doc.Triggers[0].Routes[0].Extensions["x-route"] != "yes" {
		t.Fatalf("Expected route extension, got %#v", doc.Triggers[0].Routes[0].Extensions)
	}
	if doc.Results[0].Extensions["x-result"] != "kept" {
		t.Fatalf("Expected result extension, got %#v", doc.Results[0].Extensions)
	}

	encoded, err := MarshalYAML(&doc)
	if err != nil {
		t.Fatalf("MarshalYAML failed: %v", err)
	}
	if !strings.Contains(string(encoded), "x-root") || !strings.Contains(string(encoded), "x-result") {
		t.Fatalf("YAML output did not preserve extensions:\n%s", encoded)
	}
}

func TestHCLConversionRejectsExtensions(t *testing.T) {
	jsonData := []byte(`{
		"uws": "1.0.0",
		"info": {"title": "Extension", "version": "1.0.0"},
		"operations": [
			{
				"operationId": "build_email",
				"x-uws-operation-profile": "udon",
				"x-udon-runtime": {"type": "fnct", "function": "mail_raw"}
			}
		]
	}`)

	_, err := JSONToHCL(jsonData)
	if err == nil || !strings.Contains(err.Error(), "cannot preserve extension profiles") {
		t.Fatalf("Expected extension-preservation error, got %v", err)
	}

	var doc uws1.Document
	if err := json.Unmarshal(jsonData, &doc); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}
	_, err = MarshalHCL(&doc)
	if err == nil || !strings.Contains(err.Error(), "cannot preserve extension profiles") {
		t.Fatalf("Expected extension-preservation error, got %v", err)
	}
}

func TestHCLToJSON(t *testing.T) {
	hclData := []byte(`
uws = "1.0.0"

info {
  title   = "Pet Store Workflow"
  version = "1.0.0"
}

sourceDescription "petstore_api" {
  url  = "./petstore.yaml"
  type = "openapi"
}

operation "list_pets" {
  sourceDescription  = "petstore_api"
  openapiOperationId = "listPets"
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
	if len(doc.SourceDescriptions) != 1 || doc.SourceDescriptions[0].Name != "petstore_api" {
		t.Error("SourceDescriptions not properly converted")
	}
	if len(doc.Operations) != 1 || doc.Operations[0].OperationID != "list_pets" {
		t.Error("Operations not properly converted")
	}
}

func TestMarshalUnmarshalHCL(t *testing.T) {
	doc := testDocument()

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
	if len(doc2.SourceDescriptions) != len(doc.SourceDescriptions) {
		t.Errorf("SourceDescription count mismatch: got %d, want %d", len(doc2.SourceDescriptions), len(doc.SourceDescriptions))
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
			{"name": "api2", "url": "./api2.json", "type": "openapi"}
		],
		"operations": [
			{
				"operationId": "create_resource",
				"sourceDescription": "api1",
				"openapiOperationId": "createResource",
				"successCriteria": [
					{"condition": "$response.statusCode == 201", "type": "simple"}
				],
				"onFailure": [
					{"name": "retry_create", "type": "retry", "retryAfter": 2, "retryLimit": 3}
				],
				"outputs": {"resourceId": "$response.body.id"}
			},
			{
				"operationId": "verify_resource",
				"sourceDescription": "api2",
				"openapiOperationRef": "#/paths/~1resources~11/get",
				"dependsOn": ["create_resource"],
				"onSuccess": [
					{"name": "goto_workflow", "type": "goto", "workflowId": "parallel_checks"}
				]
			}
		],
		"workflows": [
			{
				"workflowId": "parallel_checks",
				"type": "parallel",
				"dependsOn": ["create_resource"],
				"steps": [
					{"stepId": "validate", "operationRef": "verify_resource", "body": {"$validate": true}},
					{"stepId": "after_validate", "type": "sequence"}
				],
				"outputs": {"result": "$steps.validate.outputs.body"}
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
}

func TestRoundTripPreservesCriteriaAndActions(t *testing.T) {
	doc := testDocument()
	doc.Operations[0].SuccessCriteria = []*uws1.Criterion{
		{Condition: "$response.statusCode == 200", Type: uws1.CriterionSimple},
		{Condition: "^\\{", Type: uws1.CriterionRegex, Context: "$response.body"},
	}
	doc.Operations[0].OnSuccess = []*uws1.SuccessAction{
		{Name: "continue", Type: "goto", StepID: "nextStep", Criteria: []*uws1.Criterion{{Condition: "$response.statusCode == 200"}}},
	}
	doc.Operations[0].OnFailure = []*uws1.FailureAction{
		{Name: "retry_once", Type: "retry", RetryAfter: 1.5, RetryLimit: 3},
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
	if op.SuccessCriteria[1].Type != uws1.CriterionRegex {
		t.Errorf("Expected criterion type 'regex', got %q", op.SuccessCriteria[1].Type)
	}
	if len(op.OnSuccess) != 1 || op.OnSuccess[0].StepID != "nextStep" {
		t.Fatalf("Expected onSuccess stepId 'nextStep', got %#v", op.OnSuccess)
	}
	if len(op.OnFailure) != 1 || op.OnFailure[0].RetryLimit != 3 {
		t.Fatalf("Expected retryLimit 3, got %#v", op.OnFailure)
	}
}

func TestHCLConversionPreservesDollarKeys(t *testing.T) {
	doc := testDocument()
	jsonData, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("Failed to marshal JSON: %v", err)
	}

	hclData, err := JSONToHCL(jsonData)
	if err != nil {
		t.Fatalf("JSONToHCL failed: %v", err)
	}

	hclStr := string(hclData)
	if !strings.Contains(hclStr, "__dollar__root") {
		t.Error("HCL output missing transformed $root key")
	}
	if !strings.Contains(hclStr, "__dollar__request") {
		t.Error("HCL output missing transformed $request key")
	}

	jsonData2, err := HCLToJSON(hclData)
	if err != nil {
		t.Fatalf("HCLToJSON failed: %v", err)
	}

	var roundTripped uws1.Document
	if err := json.Unmarshal(jsonData2, &roundTripped); err != nil {
		t.Fatalf("Failed to parse JSON output: %v", err)
	}

	if _, ok := roundTripped.Variables["$root"]; !ok {
		t.Error("Expected $root key in variables after round-trip")
	}
	body, ok := roundTripped.Operations[0].Request["body"].(map[string]any)
	if !ok {
		t.Fatalf("Expected body map in request after round-trip, got %#v", roundTripped.Operations[0].Request["body"])
	}
	if _, ok := body["$request"]; !ok {
		t.Error("Expected $request key in request body after round-trip")
	}
}

func TestHCLToJSONIndent(t *testing.T) {
	doc := testDocument()
	hclData, err := MarshalHCL(doc)
	if err != nil {
		t.Fatalf("MarshalHCL failed: %v", err)
	}

	jsonData, err := HCLToJSONIndent(hclData, "", "  ")
	if err != nil {
		t.Fatalf("HCLToJSONIndent failed: %v", err)
	}

	if !strings.Contains(string(jsonData), "\n") {
		t.Error("JSON output is not indented")
	}

	var decoded uws1.Document
	if err := json.Unmarshal(jsonData, &decoded); err != nil {
		t.Fatalf("Output is not valid JSON: %v", err)
	}
}

func TestWorkflowWithTriggers(t *testing.T) {
	jsonData := []byte(`{
		"uws": "1.0.0",
		"info": {"title": "Full Feature Test", "version": "1.0.0"},
		"sourceDescriptions": [{"name": "api", "url": "./openapi.yaml", "type": "openapi"}],
		"operations": [{"operationId": "op1", "sourceDescription": "api", "openapiOperationId": "getOp"}],
		"triggers": [
			{
				"triggerId": "webhook",
				"path": "/webhooks/test",
				"methods": ["POST"],
				"authentication": "bearer",
				"options": {"timeout": 30},
				"routes": [{"output": "0", "to": ["op1"]}]
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
}

func TestNewlineEscaping(t *testing.T) {
	doc := testDocument()
	doc.Info.Description = "First line\nSecond line\nThird line"
	doc.Operations[0].Description = "Op description\nwith newlines"

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
