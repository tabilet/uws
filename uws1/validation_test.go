package uws1

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func validDocument() *Document {
	return &Document{
		UWS: "1.0.0",
		Info: &Info{
			Title:   "Test",
			Version: "1.0.0",
		},
		Operations: []*Operation{
			{
				OperationID: "get_data",
				ServiceType: "http",
				Method:      "GET",
				Path:        "/data",
			},
		},
	}
}

func TestValidate_Valid(t *testing.T) {
	doc := validDocument()
	assert.NoError(t, doc.Validate())
}

func TestValidate_MissingVersion(t *testing.T) {
	doc := validDocument()
	doc.UWS = ""
	assert.ErrorContains(t, doc.Validate(), "uws version is required")
}

func TestValidate_BadVersionPattern(t *testing.T) {
	doc := validDocument()
	doc.UWS = "2.0.0"
	assert.ErrorContains(t, doc.Validate(), "does not match pattern")
}

func TestValidate_MissingInfo(t *testing.T) {
	doc := validDocument()
	doc.Info = nil
	assert.ErrorContains(t, doc.Validate(), "info is required")
}

func TestValidate_MissingInfoTitle(t *testing.T) {
	doc := validDocument()
	doc.Info.Title = ""
	assert.ErrorContains(t, doc.Validate(), "info.title is required")
}

func TestValidate_MissingInfoVersion(t *testing.T) {
	doc := validDocument()
	doc.Info.Version = ""
	assert.ErrorContains(t, doc.Validate(), "info.version is required")
}

func TestValidate_NoOperations(t *testing.T) {
	doc := validDocument()
	doc.Operations = nil
	assert.ErrorContains(t, doc.Validate(), "at least one operation is required")
}

func TestValidate_MissingOperationID(t *testing.T) {
	doc := validDocument()
	doc.Operations[0].OperationID = ""
	assert.ErrorContains(t, doc.Validate(), "operationId is required")
}

func TestValidate_DuplicateOperationID(t *testing.T) {
	doc := validDocument()
	doc.Operations = append(doc.Operations, &Operation{
		OperationID: "get_data",
		ServiceType: "http",
		Method:      "GET",
	})
	assert.ErrorContains(t, doc.Validate(), "duplicate operationId")
}

func TestValidate_MissingServiceType(t *testing.T) {
	doc := validDocument()
	doc.Operations[0].ServiceType = ""
	assert.ErrorContains(t, doc.Validate(), "serviceType is required")
}

func TestValidate_InvalidServiceType(t *testing.T) {
	doc := validDocument()
	doc.Operations[0].ServiceType = "grpc"
	assert.ErrorContains(t, doc.Validate(), "is not valid")
}

func TestValidate_HTTPRequiresMethod(t *testing.T) {
	doc := validDocument()
	doc.Operations[0].Method = ""
	assert.ErrorContains(t, doc.Validate(), "http operations require method")
}

func TestValidate_InvalidHTTPMethod(t *testing.T) {
	doc := validDocument()
	doc.Operations[0].Method = "PUSH"
	assert.ErrorContains(t, doc.Validate(), "invalid http method")
}

func TestValidate_HTTPMethodRequiresUppercase(t *testing.T) {
	doc := validDocument()
	doc.Operations[0].Method = "get"
	assert.ErrorContains(t, doc.Validate(), "invalid http method")
}

func TestValidate_SSHRequiresCommand(t *testing.T) {
	doc := validDocument()
	doc.Operations[0].ServiceType = "ssh"
	doc.Operations[0].Method = ""
	doc.Operations[0].Command = ""
	assert.ErrorContains(t, doc.Validate(), "ssh operations require command")
}

func TestValidate_CmdRequiresCommand(t *testing.T) {
	doc := validDocument()
	doc.Operations[0].ServiceType = "cmd"
	doc.Operations[0].Method = ""
	doc.Operations[0].Command = ""
	assert.ErrorContains(t, doc.Validate(), "cmd operations require command")
}

func TestValidate_FnctRequiresFunction(t *testing.T) {
	doc := validDocument()
	doc.Operations[0].ServiceType = "fnct"
	doc.Operations[0].Method = ""
	doc.Operations[0].Function = ""
	assert.ErrorContains(t, doc.Validate(), "fnct operations require function")
}

func TestValidate_DuplicateWorkflowID(t *testing.T) {
	doc := validDocument()
	doc.Workflows = []*Workflow{
		{WorkflowID: "wf1", Type: "parallel"},
		{WorkflowID: "wf1", Type: "switch"},
	}
	assert.ErrorContains(t, doc.Validate(), "duplicate workflowId")
}

func TestValidate_DuplicateTriggerID(t *testing.T) {
	doc := validDocument()
	doc.Triggers = []*Trigger{
		{TriggerID: "t1"},
		{TriggerID: "t1"},
	}
	assert.ErrorContains(t, doc.Validate(), "duplicate triggerId")
}

func TestValidate_ComponentOperationIDCollision(t *testing.T) {
	doc := validDocument()
	doc.Components = &Components{
		Operations: map[string]*Operation{
			"reused": {OperationID: "get_data", ServiceType: "http", Method: "GET"},
		},
	}
	assert.ErrorContains(t, doc.Validate(), "duplicate operationId")
}

func TestValidate_MultiService(t *testing.T) {
	doc := &Document{
		UWS:  "1.0.0",
		Info: &Info{Title: "Multi", Version: "1.0.0"},
		Operations: []*Operation{
			{OperationID: "api_call", ServiceType: "http", Method: "POST"},
			{OperationID: "remote_cmd", ServiceType: "ssh", Command: "uptime"},
			{OperationID: "local_cmd", ServiceType: "cmd", Command: "ls"},
			{OperationID: "compute", ServiceType: "fnct", Function: "math.Pow"},
		},
	}
	assert.NoError(t, doc.Validate())
}

func TestValidate_PreReleaseVersion(t *testing.T) {
	doc := validDocument()
	doc.UWS = "1.0.0-beta.1"
	assert.NoError(t, doc.Validate())
}

func TestValidate_SourceDescriptions_Valid(t *testing.T) {
	doc := validDocument()
	doc.SourceDescriptions = []*SourceDescription{
		{Name: "petstore", URL: "./petstore.yaml", Type: "openapi"},
		{Name: "payments", URL: "https://api.payments.com/spec.json", Type: "openapi"},
	}
	assert.NoError(t, doc.Validate())
}

func TestValidate_SourceDescriptions_DuplicateName(t *testing.T) {
	doc := validDocument()
	doc.SourceDescriptions = []*SourceDescription{
		{Name: "api", URL: "./a.yaml"},
		{Name: "api", URL: "./b.yaml"},
	}
	assert.ErrorContains(t, doc.Validate(), "duplicate sourceDescription name")
}

func TestValidate_SourceDescriptions_MissingName(t *testing.T) {
	doc := validDocument()
	doc.SourceDescriptions = []*SourceDescription{
		{URL: "./spec.yaml"},
	}
	assert.ErrorContains(t, doc.Validate(), "sourceDescriptions[0].name is required")
}

func TestValidate_SourceDescriptions_MissingURL(t *testing.T) {
	doc := validDocument()
	doc.SourceDescriptions = []*SourceDescription{
		{Name: "api"},
	}
	assert.ErrorContains(t, doc.Validate(), "sourceDescriptions[0].url is required")
}

func TestValidate_SourceDescriptions_InvalidType(t *testing.T) {
	doc := validDocument()
	doc.SourceDescriptions = []*SourceDescription{
		{Name: "api", URL: "./spec.yaml", Type: "graphql"},
	}
	assert.ErrorContains(t, doc.Validate(), "is not valid")
}

func TestValidate_SourceDescriptions_NoType(t *testing.T) {
	doc := validDocument()
	doc.SourceDescriptions = []*SourceDescription{
		{Name: "api", URL: "./spec.yaml"},
	}
	assert.NoError(t, doc.Validate())
}

// --- Criterion validation ---

func TestValidate_SuccessCriteria_Valid(t *testing.T) {
	doc := validDocument()
	doc.Operations[0].SuccessCriteria = []*Criterion{
		{Condition: "$statusCode == 200"},
		{Condition: "^ok", Type: CriterionRegex, Context: "$response.body"},
	}
	assert.NoError(t, doc.Validate())
}

func TestValidate_SuccessCriteria_MissingCondition(t *testing.T) {
	doc := validDocument()
	doc.Operations[0].SuccessCriteria = []*Criterion{
		{Type: CriterionSimple},
	}
	assert.ErrorContains(t, doc.Validate(), "condition is required")
}

func TestValidate_SuccessCriteria_InvalidType(t *testing.T) {
	doc := validDocument()
	doc.Operations[0].SuccessCriteria = []*Criterion{
		{Condition: "test", Type: "invalid"},
	}
	assert.ErrorContains(t, doc.Validate(), "is not valid")
}

// --- FailureAction validation ---

func TestValidate_OnFailure_Valid(t *testing.T) {
	doc := validDocument()
	doc.Operations[0].OnFailure = []*FailureAction{
		{Name: "retry", Type: "retry", RetryAfter: 5, RetryLimit: 3},
		{Name: "abort", Type: "end"},
	}
	assert.NoError(t, doc.Validate())
}

func TestValidate_OnFailure_MissingName(t *testing.T) {
	doc := validDocument()
	doc.Operations[0].OnFailure = []*FailureAction{
		{Type: "end"},
	}
	assert.ErrorContains(t, doc.Validate(), "name is required")
}

func TestValidate_OnFailure_MissingType(t *testing.T) {
	doc := validDocument()
	doc.Operations[0].OnFailure = []*FailureAction{
		{Name: "action"},
	}
	assert.ErrorContains(t, doc.Validate(), "type is required")
}

func TestValidate_OnFailure_InvalidType(t *testing.T) {
	doc := validDocument()
	doc.Operations[0].OnFailure = []*FailureAction{
		{Name: "action", Type: "skip"},
	}
	assert.ErrorContains(t, doc.Validate(), "is not valid")
}

func TestValidate_OnFailure_GotoRequiresTarget(t *testing.T) {
	doc := validDocument()
	doc.Operations[0].OnFailure = []*FailureAction{
		{Name: "action", Type: "goto"},
	}
	assert.ErrorContains(t, doc.Validate(), "goto requires workflowId or stepId")
}

func TestValidate_OnFailure_GotoRejectsBothTargets(t *testing.T) {
	doc := validDocument()
	doc.Workflows = []*Workflow{
		{
			WorkflowID: "error_handler",
			Type:       "parallel",
			Steps:      []*Step{{StepID: "fallback_step", Type: "cmd", Body: map[string]any{"command": "true"}}},
		},
	}
	doc.Operations[0].OnFailure = []*FailureAction{
		{Name: "action", Type: "goto", WorkflowID: "error_handler", StepID: "fallback_step"},
	}
	assert.ErrorContains(t, doc.Validate(), "goto cannot specify both workflowId and stepId")
}

func TestValidate_OnFailure_GotoWithWorkflowID(t *testing.T) {
	doc := validDocument()
	doc.Workflows = []*Workflow{{WorkflowID: "error_handler", Type: "parallel"}}
	doc.Operations[0].OnFailure = []*FailureAction{
		{Name: "action", Type: "goto", WorkflowID: "error_handler"},
	}
	assert.NoError(t, doc.Validate())
}

func TestValidate_OnFailure_GotoWithStepID(t *testing.T) {
	doc := validDocument()
	doc.Workflows = []*Workflow{
		{
			WorkflowID: "error_handler",
			Type:       "parallel",
			Steps:      []*Step{{StepID: "fallback_step", Type: "cmd", Body: map[string]any{"command": "true"}}},
		},
	}
	doc.Operations[0].OnFailure = []*FailureAction{
		{Name: "action", Type: "goto", StepID: "fallback_step"},
	}
	assert.NoError(t, doc.Validate())
}

func TestValidate_OnFailure_RetryRequiresLimit(t *testing.T) {
	doc := validDocument()
	doc.Operations[0].OnFailure = []*FailureAction{
		{Name: "retry", Type: "retry", RetryAfter: 5},
	}
	assert.ErrorContains(t, doc.Validate(), "retry requires retryLimit > 0")
}

func TestValidate_OnFailure_NestedCriteria(t *testing.T) {
	doc := validDocument()
	doc.Operations[0].OnFailure = []*FailureAction{
		{
			Name:       "retry",
			Type:       "retry",
			RetryLimit: 3,
			Criteria:   []*Criterion{{Condition: ""}},
		},
	}
	assert.ErrorContains(t, doc.Validate(), "criteria[0].condition is required")
}

// --- SuccessAction validation ---

func TestValidate_OnSuccess_Valid(t *testing.T) {
	doc := validDocument()
	doc.Workflows = []*Workflow{{WorkflowID: "next", Type: "parallel"}}
	doc.Operations[0].OnSuccess = []*SuccessAction{
		{Name: "continue", Type: "end"},
		{Name: "route", Type: "goto", WorkflowID: "next"},
	}
	assert.NoError(t, doc.Validate())
}

func TestValidate_OnSuccess_MissingName(t *testing.T) {
	doc := validDocument()
	doc.Operations[0].OnSuccess = []*SuccessAction{
		{Type: "end"},
	}
	assert.ErrorContains(t, doc.Validate(), "name is required")
}

func TestValidate_OnSuccess_InvalidType(t *testing.T) {
	doc := validDocument()
	doc.Operations[0].OnSuccess = []*SuccessAction{
		{Name: "action", Type: "retry"},
	}
	assert.ErrorContains(t, doc.Validate(), "is not valid")
}

func TestValidate_OnSuccess_GotoRequiresTarget(t *testing.T) {
	doc := validDocument()
	doc.Operations[0].OnSuccess = []*SuccessAction{
		{Name: "action", Type: "goto"},
	}
	assert.ErrorContains(t, doc.Validate(), "goto requires workflowId or stepId")
}

func TestValidate_OnSuccess_GotoRejectsBothTargets(t *testing.T) {
	doc := validDocument()
	doc.Workflows = []*Workflow{
		{
			WorkflowID: "next",
			Type:       "parallel",
			Steps:      []*Step{{StepID: "next_step", Type: "cmd", Body: map[string]any{"command": "true"}}},
		},
	}
	doc.Operations[0].OnSuccess = []*SuccessAction{
		{Name: "action", Type: "goto", WorkflowID: "next", StepID: "next_step"},
	}
	assert.ErrorContains(t, doc.Validate(), "goto cannot specify both workflowId and stepId")
}

func TestValidateResult_AccumulatesErrors(t *testing.T) {
	doc := &Document{
		UWS:  "2.0.0",
		Info: &Info{},
		Operations: []*Operation{
			{OperationID: "op", ServiceType: "http", Method: "PUSH"},
			{OperationID: "op", ServiceType: "fnct"},
		},
	}

	result := doc.ValidateResult()
	assert.False(t, result.Valid())
	assert.GreaterOrEqual(t, len(result.Errors), 4)
	assert.ErrorContains(t, result, "version")
	assert.ErrorContains(t, result, "info.title")
	assert.ErrorContains(t, result, "duplicate operationId")
	assert.ErrorContains(t, result, "invalid http method")
	assert.ErrorContains(t, result, "fnct operations require function")
}

func TestValidate_SourceDescriptions_InvalidNamePattern(t *testing.T) {
	doc := validDocument()
	doc.SourceDescriptions = []*SourceDescription{
		{Name: "bad name", URL: "./spec.yaml"},
	}
	assert.ErrorContains(t, doc.Validate(), "must match pattern")
}

func TestValidate_CriterionTypedRequiresContext(t *testing.T) {
	doc := validDocument()
	doc.Operations[0].SuccessCriteria = []*Criterion{
		{Condition: "^ok", Type: CriterionRegex},
	}
	assert.ErrorContains(t, doc.Validate(), "context is required")
}

func TestValidate_WorkflowAndStepReferences(t *testing.T) {
	doc := validDocument()
	doc.Workflows = []*Workflow{
		{
			WorkflowID: "wf",
			Type:       "parallel",
			DependsOn:  []string{"missing_dependency"},
			Steps: []*Step{
				{
					StepID:       "step",
					Type:         "http",
					OperationRef: "missing_operation",
					DependsOn:    []string{"missing_step"},
				},
			},
		},
	}

	err := doc.Validate()
	assert.ErrorContains(t, err, "references unknown dependency")
	assert.ErrorContains(t, err, "references unknown operationId")
}

func TestValidate_ComponentOperationRefUsesOperationID(t *testing.T) {
	doc := validDocument()
	doc.Components = &Components{
		Operations: map[string]*Operation{
			"reusable_key": {OperationID: "reusable_operation", ServiceType: "cmd", Command: "true"},
		},
	}
	doc.Workflows = []*Workflow{
		{
			WorkflowID: "wf",
			Type:       "sequence",
			Steps:      []*Step{{StepID: "s", Type: "cmd", OperationRef: "reusable_operation"}},
		},
	}

	assert.NoError(t, doc.Validate())
}

func TestValidate_ComponentOperationRefDoesNotUseMapKey(t *testing.T) {
	doc := validDocument()
	doc.Components = &Components{
		Operations: map[string]*Operation{
			"reusable_key": {OperationID: "reusable_operation", ServiceType: "cmd", Command: "true"},
		},
	}
	doc.Workflows = []*Workflow{
		{
			WorkflowID: "wf",
			Type:       "sequence",
			Steps:      []*Step{{StepID: "s", Type: "cmd", OperationRef: "reusable_key"}},
		},
	}

	assert.ErrorContains(t, doc.Validate(), "references unknown operationId")
}

func TestValidate_InvalidStepType(t *testing.T) {
	doc := validDocument()
	doc.Workflows = []*Workflow{
		{
			WorkflowID: "wf",
			Type:       "sequence",
			Steps:      []*Step{{StepID: "bad", Type: "not-a-step-type"}},
		},
	}
	assert.ErrorContains(t, doc.Validate(), "not-a-step-type")
}

func TestValidate_SequenceWorkflow(t *testing.T) {
	doc := validDocument()
	doc.Workflows = []*Workflow{
		{
			WorkflowID: "main",
			Type:       "sequence",
			Steps: []*Step{
				{
					StepID:       "get_data",
					Type:         "http",
					OperationRef: "get_data",
				},
			},
		},
	}

	assert.NoError(t, doc.Validate())
}

func TestValidate_TriggerRouteReferencesOperation(t *testing.T) {
	doc := validDocument()
	doc.Triggers = []*Trigger{
		{
			TriggerID: "webhook",
			Routes: []*TriggerRoute{
				{Output: "0", To: []string{"missing"}},
			},
		},
	}
	assert.ErrorContains(t, doc.Validate(), "references unknown operationId")
}

func TestValidate_TriggerRouteRequiresTarget(t *testing.T) {
	doc := validDocument()
	doc.Triggers = []*Trigger{
		{
			TriggerID: "webhook",
			Routes: []*TriggerRoute{
				{Output: "0"},
			},
		},
	}
	assert.ErrorContains(t, doc.Validate(), "must contain at least one operationId")
}

func TestValidate_SecuritySchemeRules(t *testing.T) {
	doc := validDocument()
	doc.Security = []*SecurityRequirement{
		{
			Name: "api_key",
			Scheme: &SecurityScheme{
				Type: "apiKey",
				In:   "body",
			},
		},
		{
			Name: "oauth",
			Scheme: &SecurityScheme{
				Type:  "oauth2",
				Flows: &OAuthFlows{AuthorizationCode: &OAuthFlow{AuthorizationURL: "https://auth.example.com"}},
			},
		},
	}

	err := doc.Validate()
	assert.ErrorContains(t, err, "name is required for apiKey")
	assert.ErrorContains(t, err, "body")
	assert.ErrorContains(t, err, "tokenUrl")
}

func TestValidate_SecurityRequirementWithoutInlineSchemeIsMetadataOnly(t *testing.T) {
	doc := validDocument()
	doc.Components = &Components{
		SecuritySchemes: map[string]*SecurityScheme{
			"api_key": {Type: "apiKey", Name: "X-API-Key", In: "header"},
		},
	}
	doc.Security = []*SecurityRequirement{{Name: "api_key"}}

	assert.NoError(t, doc.Validate())
}

func TestValidate_StructuralResultKind(t *testing.T) {
	doc := validDocument()
	doc.Results = []*StructuralResult{{Name: "out", Kind: "parallel"}}
	assert.ErrorContains(t, doc.Validate(), "parallel")
}
