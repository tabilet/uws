package uws1

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDocument_RoundTrip(t *testing.T) {
	statusCode := 200
	doc := &Document{
		UWS: "1.0.0",
		Info: &Info{
			Title:   "Test Workflow",
			Version: "0.1.0",
		},
		Provider: &Provider{
			Name:      "test",
			ServerURL: "https://api.example.com",
			Appendices: map[string]any{
				"region": "us-west-2",
			},
		},
		Variables: map[string]any{
			"env":     "dev",
			"timeout": float64(30),
		},
		Operations: []*Operation{
			{
				OperationID: "get_users",
				ServiceType: "http",
				Method:      "GET",
				Path:        "/users",
				IsJSON:      true,
				QueryPars: &ParamSchema{
					Type: "object",
					Properties: map[string]*ParamSchema{
						"limit": {Type: "integer"},
					},
				},
				Request: map[string]any{
					"limit": float64(10),
				},
				ResponseStatusCode: &statusCode,
				DependsOn:          []string{},
				Outputs: map[string]string{
					"userList": "$response.body.items",
				},
			},
			{
				OperationID: "run_report",
				ServiceType: "cmd",
				Command:     "/bin/generate-report",
				Arguments:   []any{"--format", "json"},
				WorkingDir:  "/tmp",
				DependsOn:   []string{"get_users"},
			},
			{
				OperationID: "hash_data",
				ServiceType: "fnct",
				Function:    "crypto.SHA256",
				Arguments:   []any{"input"},
			},
		},
		Security: []*SecurityRequirement{
			{
				Name: "bearer_auth",
				Scheme: &SecurityScheme{
					Type:   "http",
					Scheme: "bearer",
				},
			},
		},
		Triggers: []*Trigger{
			{
				TriggerID: "webhook_1",
				Path:      "/hooks/test",
				Methods:   []string{"POST"},
				Routes: []*TriggerRoute{
					{Output: "0", To: []string{"get_users"}},
				},
			},
		},
		Workflows: []*Workflow{
			{
				WorkflowID: "parallel_block",
				Type:       "parallel",
				Steps: []*Step{
					{StepID: "step_a", Type: "http", OperationRef: "get_users"},
					{StepID: "step_b", Type: "cmd", Body: map[string]any{"command": "echo"}},
				},
			},
		},
		Results: []*StructuralResult{
			{Name: "merge_out", Kind: "merge"},
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
	assert.Equal(t, doc.Provider.Name, decoded.Provider.Name)
	assert.Equal(t, doc.Provider.ServerURL, decoded.Provider.ServerURL)
	assert.Equal(t, doc.Variables["env"], decoded.Variables["env"])
	assert.Len(t, decoded.Operations, 3)
	assert.Equal(t, "get_users", decoded.Operations[0].OperationID)
	assert.Equal(t, "http", decoded.Operations[0].ServiceType)
	assert.Equal(t, "GET", decoded.Operations[0].Method)
	assert.Equal(t, 200, *decoded.Operations[0].ResponseStatusCode)
	assert.Equal(t, "run_report", decoded.Operations[1].OperationID)
	assert.Equal(t, "cmd", decoded.Operations[1].ServiceType)
	assert.Equal(t, "/bin/generate-report", decoded.Operations[1].Command)
	assert.Equal(t, "hash_data", decoded.Operations[2].OperationID)
	assert.Equal(t, "fnct", decoded.Operations[2].ServiceType)
	assert.Equal(t, "crypto.SHA256", decoded.Operations[2].Function)
	assert.Len(t, decoded.Security, 1)
	assert.Equal(t, "bearer_auth", decoded.Security[0].Name)
	assert.Len(t, decoded.Triggers, 1)
	assert.Equal(t, "webhook_1", decoded.Triggers[0].TriggerID)
	assert.Len(t, decoded.Workflows, 1)
	assert.Equal(t, "parallel_block", decoded.Workflows[0].WorkflowID)
	assert.Len(t, decoded.Workflows[0].Steps, 2)
	assert.Len(t, decoded.Results, 1)
	assert.Equal(t, "merge_out", decoded.Results[0].Name)
	assert.Equal(t, "test", decoded.Extensions["x-source"])
}

func TestDocument_Extensions(t *testing.T) {
	input := `{
		"uws": "1.0.0",
		"info": {"title": "T", "version": "1.0.0", "x-custom": true},
		"operations": [{"operationId": "op1", "serviceType": "http", "method": "GET", "x-tier": 1}],
		"x-generated": "claude"
	}`

	var doc Document
	require.NoError(t, json.Unmarshal([]byte(input), &doc))

	assert.Equal(t, "claude", doc.Extensions["x-generated"])
	assert.Equal(t, true, doc.Info.Extensions["x-custom"])
	assert.Equal(t, float64(1), doc.Operations[0].Extensions["x-tier"])
}

func TestDocument_SampleFile(t *testing.T) {
	data, err := os.ReadFile("../testdata/sample.uws.json")
	require.NoError(t, err)

	var doc Document
	require.NoError(t, json.Unmarshal(data, &doc))

	assert.Equal(t, "1.0.0", doc.UWS)
	assert.Equal(t, "Sample UWS Workflow", doc.Info.Title)
	assert.Equal(t, "petstore", doc.Provider.Name)
	assert.Len(t, doc.SourceDescriptions, 1)
	assert.Equal(t, "petstore_api", doc.SourceDescriptions[0].Name)
	assert.Equal(t, "./petstore.yaml", doc.SourceDescriptions[0].URL)
	assert.Equal(t, SourceDescriptionTypeOpenAPI, doc.SourceDescriptions[0].Type)
	assert.Len(t, doc.Operations, 4)
	assert.Equal(t, "list_pets", doc.Operations[0].OperationID)
	assert.Equal(t, "http", doc.Operations[0].ServiceType)
	assert.Equal(t, "petstore_api", doc.Operations[0].SourceDescription)
	assert.Equal(t, "check_disk", doc.Operations[2].OperationID)
	assert.Equal(t, "cmd", doc.Operations[2].ServiceType)
	assert.Equal(t, "compute_hash", doc.Operations[3].OperationID)
	assert.Equal(t, "fnct", doc.Operations[3].ServiceType)
	assert.Len(t, doc.Workflows, 1)
	assert.Len(t, doc.Triggers, 1)
	assert.Equal(t, "udon-cli", doc.Extensions["x-generator"])

	require.NoError(t, doc.Validate())

	// Round-trip
	encoded, err := json.Marshal(&doc)
	require.NoError(t, err)

	var decoded Document
	require.NoError(t, json.Unmarshal(encoded, &decoded))
	assert.Equal(t, doc.UWS, decoded.UWS)
	assert.Len(t, decoded.Operations, 4)
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

func TestDocument_WithSourceDescriptions(t *testing.T) {
	doc := &Document{
		UWS:  "1.0.0",
		Info: &Info{Title: "Test", Version: "1.0.0"},
		SourceDescriptions: []*SourceDescription{
			{Name: "api1", URL: "./api1.yaml", Type: "openapi"},
			{Name: "api2", URL: "https://example.com/api2.json", Type: "openapi"},
		},
		Operations: []*Operation{
			{
				OperationID:       "get_data",
				ServiceType:       "http",
				SourceDescription: "api1",
				Method:            "GET",
				Path:              "/data",
			},
			{
				OperationID:       "post_data",
				ServiceType:       "http",
				SourceDescription: "api2",
				Method:            "POST",
				Path:              "/data",
			},
		},
	}

	data, err := json.MarshalIndent(doc, "", "  ")
	require.NoError(t, err)

	var decoded Document
	require.NoError(t, json.Unmarshal(data, &decoded))

	assert.Len(t, decoded.SourceDescriptions, 2)
	assert.Equal(t, "api1", decoded.SourceDescriptions[0].Name)
	assert.Equal(t, "api2", decoded.SourceDescriptions[1].Name)
	assert.Equal(t, "api1", decoded.Operations[0].SourceDescription)
	assert.Equal(t, "api2", decoded.Operations[1].SourceDescription)

	require.NoError(t, decoded.Validate())
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
	assert.Equal(t, info.Version, decoded.Version)
	assert.Equal(t, "team-a", decoded.Extensions["x-owner"])
}

func TestParamSchema_Recursive(t *testing.T) {
	schema := &ParamSchema{
		Type: "object",
		Properties: map[string]*ParamSchema{
			"name": {Type: "string"},
			"address": {
				Type: "object",
				Properties: map[string]*ParamSchema{
					"city":  {Type: "string"},
					"state": {Type: "string"},
				},
				Required: []string{"city"},
			},
			"tags": {
				Type:  "array",
				Items: &ParamSchema{Type: "string"},
			},
		},
		Required: []string{"name"},
	}

	data, err := json.Marshal(schema)
	require.NoError(t, err)

	var decoded ParamSchema
	require.NoError(t, json.Unmarshal(data, &decoded))

	assert.Equal(t, "object", decoded.Type)
	assert.Len(t, decoded.Properties, 3)
	assert.Equal(t, "string", decoded.Properties["name"].Type)
	assert.Equal(t, "object", decoded.Properties["address"].Type)
	assert.Len(t, decoded.Properties["address"].Properties, 2)
	assert.Equal(t, "array", decoded.Properties["tags"].Type)
	assert.Equal(t, "string", decoded.Properties["tags"].Items.Type)
}

func TestSecurityScheme_OAuth2(t *testing.T) {
	sec := &SecurityRequirement{
		Name:   "oauth2_auth",
		Scopes: []string{"read", "write"},
		Scheme: &SecurityScheme{
			Type: "oauth2",
			Flows: &OAuthFlows{
				Password: &OAuthFlow{
					TokenURL: "https://auth.example.com/token",
					Scopes: map[string]string{
						"read":  "Read access",
						"write": "Write access",
					},
				},
			},
		},
	}

	data, err := json.Marshal(sec)
	require.NoError(t, err)

	var decoded SecurityRequirement
	require.NoError(t, json.Unmarshal(data, &decoded))

	assert.Equal(t, "oauth2_auth", decoded.Name)
	assert.Equal(t, "oauth2", decoded.Scheme.Type)
	assert.NotNil(t, decoded.Scheme.Flows.Password)
	assert.Equal(t, "https://auth.example.com/token", decoded.Scheme.Flows.Password.TokenURL)
	assert.Len(t, decoded.Scheme.Flows.Password.Scopes, 2)
}

func TestCriterion_RoundTrip(t *testing.T) {
	c := &Criterion{
		Condition:  "$statusCode == 200",
		Type:       CriterionSimple,
		Extensions: map[string]any{"x-note": "test"},
	}

	data, err := json.Marshal(c)
	require.NoError(t, err)

	var decoded Criterion
	require.NoError(t, json.Unmarshal(data, &decoded))

	assert.Equal(t, "$statusCode == 200", decoded.Condition)
	assert.Equal(t, CriterionSimple, decoded.Type)
	assert.Equal(t, "test", decoded.Extensions["x-note"])
}

func TestCriterion_RegexWithContext(t *testing.T) {
	c := &Criterion{
		Condition: "^/dev/",
		Type:      CriterionRegex,
		Context:   "$response.body",
	}

	data, err := json.Marshal(c)
	require.NoError(t, err)

	var decoded Criterion
	require.NoError(t, json.Unmarshal(data, &decoded))

	assert.Equal(t, "^/dev/", decoded.Condition)
	assert.Equal(t, CriterionRegex, decoded.Type)
	assert.Equal(t, "$response.body", decoded.Context)
}

func TestFailureAction_RoundTrip(t *testing.T) {
	fa := &FailureAction{
		Name:       "retry_on_timeout",
		Type:       "retry",
		RetryAfter: 5.0,
		RetryLimit: 3,
		Criteria: []*Criterion{
			{Condition: "$statusCode == 503"},
		},
		Extensions: map[string]any{"x-scope": "network"},
	}

	data, err := json.Marshal(fa)
	require.NoError(t, err)

	var decoded FailureAction
	require.NoError(t, json.Unmarshal(data, &decoded))

	assert.Equal(t, "retry_on_timeout", decoded.Name)
	assert.Equal(t, "retry", decoded.Type)
	assert.Equal(t, 5.0, decoded.RetryAfter)
	assert.Equal(t, 3, decoded.RetryLimit)
	assert.Len(t, decoded.Criteria, 1)
	assert.Equal(t, "$statusCode == 503", decoded.Criteria[0].Condition)
	assert.Equal(t, "network", decoded.Extensions["x-scope"])
}

func TestFailureAction_Goto(t *testing.T) {
	fa := &FailureAction{
		Name:       "fallback",
		Type:       "goto",
		WorkflowID: "error_handler",
	}

	data, err := json.Marshal(fa)
	require.NoError(t, err)

	var decoded FailureAction
	require.NoError(t, json.Unmarshal(data, &decoded))

	assert.Equal(t, "goto", decoded.Type)
	assert.Equal(t, "error_handler", decoded.WorkflowID)
}

func TestSuccessAction_RoundTrip(t *testing.T) {
	sa := &SuccessAction{
		Name:       "goto_validate",
		Type:       "goto",
		WorkflowID: "validation_workflow",
		Criteria: []*Criterion{
			{Condition: "$response.body#/id != null", Type: CriterionJSONPath},
		},
	}

	data, err := json.Marshal(sa)
	require.NoError(t, err)

	var decoded SuccessAction
	require.NoError(t, json.Unmarshal(data, &decoded))

	assert.Equal(t, "goto_validate", decoded.Name)
	assert.Equal(t, "goto", decoded.Type)
	assert.Equal(t, "validation_workflow", decoded.WorkflowID)
	assert.Len(t, decoded.Criteria, 1)
	assert.Equal(t, CriterionJSONPath, decoded.Criteria[0].Type)
}

func TestOperation_WithCriteriaAndActions(t *testing.T) {
	op := &Operation{
		OperationID: "call_api",
		ServiceType: "http",
		Method:      "POST",
		Path:        "/api/resource",
		SuccessCriteria: []*Criterion{
			{Condition: "$statusCode == 200"},
			{Condition: "^\\{", Type: CriterionRegex, Context: "$response.body"},
		},
		OnFailure: []*FailureAction{
			{Name: "retry", Type: "retry", RetryAfter: 2, RetryLimit: 3},
			{Name: "abort", Type: "end"},
		},
		OnSuccess: []*SuccessAction{
			{Name: "continue", Type: "goto", WorkflowID: "next_step"},
		},
	}

	data, err := json.Marshal(op)
	require.NoError(t, err)

	var decoded Operation
	require.NoError(t, json.Unmarshal(data, &decoded))

	assert.Len(t, decoded.SuccessCriteria, 2)
	assert.Equal(t, "$statusCode == 200", decoded.SuccessCriteria[0].Condition)
	assert.Equal(t, CriterionRegex, decoded.SuccessCriteria[1].Type)
	assert.Len(t, decoded.OnFailure, 2)
	assert.Equal(t, "retry", decoded.OnFailure[0].Type)
	assert.Equal(t, 3, decoded.OnFailure[0].RetryLimit)
	assert.Len(t, decoded.OnSuccess, 1)
	assert.Equal(t, "next_step", decoded.OnSuccess[0].WorkflowID)
}

func TestWorkflow_InputsAndOutputs(t *testing.T) {
	wf := &Workflow{
		WorkflowID:  "create_and_verify",
		Type:        "parallel",
		Description: "Create resource and verify it",
		Inputs: &ParamSchema{
			Type: "object",
			Properties: map[string]*ParamSchema{
				"petName": {Type: "string"},
				"ownerId": {Type: "integer"},
			},
			Required: []string{"petName"},
		},
		Steps: []*Step{
			{
				StepID:      "create",
				Type:        "http",
				Description: "Create the pet",
				OperationRef: "create_pet",
				Outputs: map[string]string{
					"petId": "$response.body#/id",
				},
			},
		},
		Outputs: map[string]string{
			"allResults": "$steps.create.outputs.petId",
		},
	}

	data, err := json.Marshal(wf)
	require.NoError(t, err)

	var decoded Workflow
	require.NoError(t, json.Unmarshal(data, &decoded))

	assert.Equal(t, "create_and_verify", decoded.WorkflowID)
	assert.Equal(t, "Create resource and verify it", decoded.Description)
	assert.NotNil(t, decoded.Inputs)
	assert.Equal(t, "object", decoded.Inputs.Type)
	assert.Equal(t, "string", decoded.Inputs.Properties["petName"].Type)
	assert.Equal(t, []string{"petName"}, decoded.Inputs.Required)
	assert.Len(t, decoded.Steps, 1)
	assert.Equal(t, "Create the pet", decoded.Steps[0].Description)
	assert.Equal(t, "$response.body#/id", decoded.Steps[0].Outputs["petId"])
	assert.Equal(t, "$steps.create.outputs.petId", decoded.Outputs["allResults"])
}

func TestDocument_SampleFile_NewFeatures(t *testing.T) {
	data, err := os.ReadFile("../testdata/sample.uws.json")
	require.NoError(t, err)

	var doc Document
	require.NoError(t, json.Unmarshal(data, &doc))
	require.NoError(t, doc.Validate())

	// list_pets has successCriteria and onFailure
	listPets := doc.Operations[0]
	assert.Len(t, listPets.SuccessCriteria, 1)
	assert.Equal(t, "$statusCode == 200", listPets.SuccessCriteria[0].Condition)
	assert.Len(t, listPets.OnFailure, 1)
	assert.Equal(t, "retry", listPets.OnFailure[0].Type)
	assert.Equal(t, 2.0, listPets.OnFailure[0].RetryAfter)
	assert.Equal(t, 3, listPets.OnFailure[0].RetryLimit)

	// create_pet has onSuccess and onFailure
	createPet := doc.Operations[1]
	assert.Len(t, createPet.SuccessCriteria, 1)
	assert.Len(t, createPet.OnSuccess, 1)
	assert.Equal(t, "goto", createPet.OnSuccess[0].Type)
	assert.Len(t, createPet.OnFailure, 1)
	assert.Equal(t, "end", createPet.OnFailure[0].Type)

	// check_disk has regex criterion
	checkDisk := doc.Operations[2]
	assert.Len(t, checkDisk.SuccessCriteria, 2)
	assert.Equal(t, "$exitCode == 0", checkDisk.SuccessCriteria[0].Condition)
	assert.Equal(t, CriterionRegex, checkDisk.SuccessCriteria[1].Type)

	// compute_hash has returnValue criterion
	computeHash := doc.Operations[3]
	assert.Len(t, computeHash.SuccessCriteria, 1)
	assert.Equal(t, "$returnValue != null", computeHash.SuccessCriteria[0].Condition)

	// Workflow has description, inputs, outputs
	wf := doc.Workflows[0]
	assert.Equal(t, "Run validation and logging in parallel after pet creation", wf.Description)
	assert.NotNil(t, wf.Inputs)
	assert.Equal(t, "object", wf.Inputs.Type)
	assert.Equal(t, "integer", wf.Inputs.Properties["petId"].Type)
	assert.Equal(t, "$steps.validate_pet.outputs.isValid", wf.Outputs["validationResult"])

	// Steps have description and outputs
	assert.Equal(t, "Validate the newly created pet record", wf.Steps[0].Description)
	assert.Equal(t, "$response.body.valid", wf.Steps[0].Outputs["isValid"])
	assert.Equal(t, "Log the pet creation event", wf.Steps[1].Description)
}

func TestWorkflow_NestedSteps(t *testing.T) {
	wf := &Workflow{
		WorkflowID: "switch_block",
		Type:       "switch",
		Items:      "$operations.list_pets.response.body",
		Cases: []*Case{
			{
				Name: "dog",
				When: "item.type == 'dog'",
				Steps: []*Step{
					{StepID: "process_dog", Type: "http", OperationRef: "create_pet"},
				},
			},
			{
				Name: "cat",
				When: "item.type == 'cat'",
			},
		},
		Default: []*Step{
			{StepID: "log_unknown", Type: "cmd", Body: map[string]any{"command": "echo"}},
		},
	}

	data, err := json.Marshal(wf)
	require.NoError(t, err)

	var decoded Workflow
	require.NoError(t, json.Unmarshal(data, &decoded))

	assert.Equal(t, "switch_block", decoded.WorkflowID)
	assert.Equal(t, "switch", decoded.Type)
	assert.Len(t, decoded.Cases, 2)
	assert.Equal(t, "dog", decoded.Cases[0].Name)
	assert.Len(t, decoded.Cases[0].Steps, 1)
	assert.Len(t, decoded.Default, 1)
}
