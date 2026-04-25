package uws1

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDocument_RoundTrip(t *testing.T) {
	doc := &Document{
		UWS: "1.0.0",
		Info: &Info{
			Title:   "Test Workflow",
			Version: "0.1.0",
		},
		SourceDescriptions: []*SourceDescription{
			{Name: "api", URL: "./openapi.yaml", Type: SourceDescriptionTypeOpenAPI},
		},
		Variables: map[string]any{
			"env":     "dev",
			"timeout": float64(30),
		},
		Operations: []*Operation{
			{
				OperationID:        "get_users",
				SourceDescription:  "api",
				OpenAPIOperationID: "listUsers",
				Request: map[string]any{
					"query": map[string]any{"limit": float64(10)},
				},
				OperationExecutionFields: OperationExecutionFields{
					DependsOn: []string{},
				},
				Outputs: map[string]string{
					"userList": "$response.body.items",
				},
			},
			{
				OperationID:         "create_user",
				SourceDescription:   "api",
				OpenAPIOperationRef: "#/paths/~1users/post",
				OperationExecutionFields: OperationExecutionFields{
					DependsOn: []string{"get_users"},
				},
			},
		},
		Triggers: []*Trigger{
			{
				TriggerID:     "webhook_1",
				TriggerFields: TriggerFields{Path: "/hooks/test", Methods: []string{"POST"}},
				Outputs:       []string{"primary"},
				Routes: []*TriggerRoute{
					{TriggerRouteFields: TriggerRouteFields{Output: "primary", To: []string{"parallel_block"}}},
				},
			},
		},
		Workflows: []*Workflow{
			{
				WorkflowID: "parallel_block",
				Type:       "parallel",
				Steps: []*Step{
					{StepID: "step_a", OperationRef: "get_users"},
					{StepID: "step_b", Type: "sequence", Steps: []*Step{{StepID: "nested", OperationRef: "create_user"}}},
					{StepID: "merge_users", Type: "merge", StepExecutionFields: StepExecutionFields{DependsOn: []string{"step_a", "step_b"}}},
				},
			},
		},
		Results: []*StructuralResult{
			{Name: "merge_out", Kind: "merge", From: "parallel_block.merge_users", Value: "$steps.merge_users.outputs"},
		},
		Extensions: map[string]any{
			"x-source": "test",
		},
	}

	data, err := json.MarshalIndent(doc, "", "  ")
	require.NoError(t, err)

	var decoded Document
	require.NoError(t, json.Unmarshal(data, &decoded))

	assert.Equal(t, doc.UWS, decoded.UWS)
	assert.Equal(t, doc.Info.Title, decoded.Info.Title)
	assert.Equal(t, doc.Info.Version, decoded.Info.Version)
	assert.Equal(t, doc.Variables["env"], decoded.Variables["env"])
	assert.Len(t, decoded.SourceDescriptions, 1)
	assert.Equal(t, "api", decoded.SourceDescriptions[0].Name)
	assert.Len(t, decoded.Operations, 2)
	assert.Equal(t, "get_users", decoded.Operations[0].OperationID)
	assert.Equal(t, "api", decoded.Operations[0].SourceDescription)
	assert.Equal(t, "listUsers", decoded.Operations[0].OpenAPIOperationID)
	assert.Equal(t, "#/paths/~1users/post", decoded.Operations[1].OpenAPIOperationRef)
	assert.Len(t, decoded.Triggers, 1)
	assert.Equal(t, "webhook_1", decoded.Triggers[0].TriggerID)
	assert.Equal(t, []string{"primary"}, decoded.Triggers[0].Outputs)
	assert.Len(t, decoded.Workflows, 1)
	assert.Equal(t, "parallel_block", decoded.Workflows[0].WorkflowID)
	assert.Len(t, decoded.Workflows[0].Steps, 3)
	assert.Len(t, decoded.Results, 1)
	assert.Equal(t, "merge_out", decoded.Results[0].Name)
	assert.Equal(t, "parallel_block.merge_users", decoded.Results[0].From)
	assert.Equal(t, "$steps.merge_users.outputs", decoded.Results[0].Value)
	assert.Equal(t, "test", decoded.Extensions["x-source"])
	require.NoError(t, decoded.Validate())
}

func TestDocument_Extensions(t *testing.T) {
	input := `{
		"uws": "1.0.0",
		"info": {"title": "T", "version": "1.0.0", "x-custom": true},
		"sourceDescriptions": [{"name": "api", "url": "./openapi.yaml", "type": "openapi"}],
		"operations": [{"operationId": "op1", "sourceDescription": "api", "openapiOperationId": "getOp", "x-tier": 1}],
		"x-generated": "claude"
	}`

	var doc Document
	require.NoError(t, json.Unmarshal([]byte(input), &doc))

	assert.Equal(t, "claude", doc.Extensions["x-generated"])
	assert.Equal(t, true, doc.Info.Extensions["x-custom"])
	assert.Equal(t, float64(1), doc.Operations[0].Extensions["x-tier"])
}

func TestDocument_RejectsRemovedCoreFields(t *testing.T) {
	input := `{
		"uws": "1.0.0",
		"info": {"title": "T", "version": "1.0.0"},
		"provider": {"name": "legacy"},
		"sourceDescriptions": [{"name": "api", "url": "./openapi.yaml", "type": "openapi"}],
		"operations": [{"operationId": "op1", "sourceDescription": "api", "openapiOperationId": "getOp"}]
	}`

	var doc Document
	require.ErrorContains(t, json.Unmarshal([]byte(input), &doc), `document field "provider" is not defined`)

	input = `{
		"uws": "1.0.0",
		"info": {"title": "T", "version": "1.0.0"},
		"sourceDescriptions": [{"name": "api", "url": "./openapi.yaml", "type": "openapi"}],
		"operations": [{"operationId": "op1", "sourceDescription": "api", "openapiOperationId": "getOp", "serviceType": "http"}]
	}`
	require.ErrorContains(t, json.Unmarshal([]byte(input), &doc), `operation field "serviceType" is not defined`)
}

func TestDocument_MarshalRejectsInvalidExtensionKey(t *testing.T) {
	doc := validDocument()
	doc.Extensions = map[string]any{"not-x": "bad"}

	_, err := json.Marshal(doc)
	require.ErrorContains(t, err, "must start with x-")
}

func TestDocument_MarshalRejectsFixedFieldExtensionOverride(t *testing.T) {
	doc := validDocument()
	doc.Operations[0].Extensions = map[string]any{"operationId": "hijack"}

	_, err := json.Marshal(doc)
	require.ErrorContains(t, err, "must start with x-")
}

func TestDocument_SampleFile(t *testing.T) {
	data, err := os.ReadFile("../testdata/sample.uws.json")
	require.NoError(t, err)

	var doc Document
	require.NoError(t, json.Unmarshal(data, &doc))

	assert.Equal(t, "1.0.0", doc.UWS)
	assert.Equal(t, "Sample UWS Workflow", doc.Info.Title)
	assert.Len(t, doc.SourceDescriptions, 1)
	assert.Equal(t, "petstore_api", doc.SourceDescriptions[0].Name)
	assert.Equal(t, "./petstore.yaml", doc.SourceDescriptions[0].URL)
	assert.Equal(t, SourceDescriptionTypeOpenAPI, doc.SourceDescriptions[0].Type)
	assert.Len(t, doc.Operations, 2)
	assert.Equal(t, "list_pets", doc.Operations[0].OperationID)
	assert.Equal(t, "petstore_api", doc.Operations[0].SourceDescription)
	assert.Equal(t, "listPets", doc.Operations[0].OpenAPIOperationID)
	assert.Equal(t, "create_pet", doc.Operations[1].OperationID)
	assert.Equal(t, "#/paths/~1pets/post", doc.Operations[1].OpenAPIOperationRef)
	assert.Len(t, doc.Workflows, 1)
	assert.Len(t, doc.Triggers, 1)
	assert.Equal(t, "udon-cli", doc.Extensions["x-generator"])

	require.NoError(t, doc.Validate())

	encoded, err := json.Marshal(&doc)
	require.NoError(t, err)

	var decoded Document
	require.NoError(t, json.Unmarshal(encoded, &decoded))
	assert.Equal(t, doc.UWS, decoded.UWS)
	assert.Len(t, decoded.Operations, 2)
}

func TestSourceDescription_RoundTrip(t *testing.T) {
	sd := &SourceDescription{
		Name:       "petstore",
		URL:        "./petstore.yaml",
		Type:       "openapi",
		Extensions: map[string]any{"x-version": "3.1"},
	}

	data, err := json.Marshal(sd)
	require.NoError(t, err)

	var decoded SourceDescription
	require.NoError(t, json.Unmarshal(data, &decoded))

	assert.Equal(t, "petstore", decoded.Name)
	assert.Equal(t, "./petstore.yaml", decoded.URL)
	assert.Equal(t, SourceDescriptionTypeOpenAPI, decoded.Type)
	assert.Equal(t, "3.1", decoded.Extensions["x-version"])
}

func TestInfo_RoundTrip(t *testing.T) {
	info := &Info{
		Title:       "My Workflow",
		Summary:     "A test",
		Description: "Longer description",
		Version:     "2.0.0",
		Extensions:  map[string]any{"x-owner": "team-a"},
	}

	data, err := json.Marshal(info)
	require.NoError(t, err)

	var decoded Info
	require.NoError(t, json.Unmarshal(data, &decoded))

	assert.Equal(t, info.Title, decoded.Title)
	assert.Equal(t, info.Summary, decoded.Summary)
	assert.Equal(t, info.Description, decoded.Description)
	assert.Equal(t, info.Version, decoded.Version)
	assert.Equal(t, "team-a", decoded.Extensions["x-owner"])
}

func TestCriterionAndActions_RoundTrip(t *testing.T) {
	op := &Operation{
		OperationID:        "create_resource",
		SourceDescription:  "api",
		OpenAPIOperationID: "createResource",
		SuccessCriteria: []*Criterion{
			{Condition: "$response.statusCode == 201", Type: CriterionSimple},
			{Condition: "$response.body#/id", Type: CriterionJSONPath, Context: "$response.body"},
		},
		OnFailure: []*FailureAction{
			{Name: "retry_once", Type: "retry", RetryAfter: 1.5, RetryLimit: 3},
		},
		OnSuccess: []*SuccessAction{
			{Name: "continue", Type: "goto", StepID: "nextStep"},
		},
	}

	data, err := json.Marshal(op)
	require.NoError(t, err)

	var decoded Operation
	require.NoError(t, json.Unmarshal(data, &decoded))

	assert.Equal(t, "create_resource", decoded.OperationID)
	assert.Len(t, decoded.SuccessCriteria, 2)
	assert.Equal(t, CriterionJSONPath, decoded.SuccessCriteria[1].Type)
	assert.Len(t, decoded.OnFailure, 1)
	assert.Equal(t, 3, decoded.OnFailure[0].RetryLimit)
	assert.Len(t, decoded.OnSuccess, 1)
	assert.Equal(t, "nextStep", decoded.OnSuccess[0].StepID)
}

func TestWorkflow_RoundTrip(t *testing.T) {
	wf := &Workflow{
		WorkflowID: "main",
		Type:       "switch",
		Cases: []*Case{
			{
				CaseFields: CaseFields{
					Name: "dog",
					When: "$response.body#/type == \"dog\"",
				},
				Steps: []*Step{{StepID: "process_dog", OperationRef: "create_pet"}},
			},
		},
		Default: []*Step{
			{StepID: "log_unknown", Type: "sequence", Body: map[string]any{"message": "unknown"}},
		},
		Outputs: map[string]string{"kind": "$merge.kind"},
	}

	data, err := json.Marshal(wf)
	require.NoError(t, err)

	var decoded Workflow
	require.NoError(t, json.Unmarshal(data, &decoded))

	assert.Equal(t, "main", decoded.WorkflowID)
	assert.Equal(t, "switch", decoded.Type)
	assert.Len(t, decoded.Cases, 1)
	assert.Equal(t, "process_dog", decoded.Cases[0].Steps[0].StepID)
	assert.Len(t, decoded.Default, 1)
	assert.Equal(t, "sequence", decoded.Default[0].Type)
}

func TestDocumentValidateExecutionEntrypoint(t *testing.T) {
	doc := &Document{
		UWS:  "1.0.0",
		Info: &Info{Title: "test", Version: "1.0.0"},
		Operations: []*Operation{
			{OperationID: "fetch", Extensions: map[string]any{ExtensionOperationProfile: "udon"}},
		},
	}

	require.ErrorContains(t, doc.ValidateExecutionEntrypoint(), "entry workflow")

	doc.Workflows = []*Workflow{{WorkflowID: "secondary", Type: WorkflowTypeSequence}}
	require.NoError(t, doc.ValidateExecutionEntrypoint())

	doc.Workflows = []*Workflow{
		{WorkflowID: "secondary", Type: WorkflowTypeSequence},
		{WorkflowID: "tertiary", Type: WorkflowTypeSequence},
	}
	require.ErrorContains(t, doc.ValidateExecutionEntrypoint(), "main")

	doc.Workflows = []*Workflow{
		{WorkflowID: "main", Type: WorkflowTypeSequence},
		{WorkflowID: "secondary", Type: WorkflowTypeSequence},
	}
	require.NoError(t, doc.ValidateExecutionEntrypoint())
}
