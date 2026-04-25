package uws1

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validDocument() *Document {
	return &Document{
		UWS: "1.0.0",
		Info: &Info{
			Title:   "Test",
			Version: "1.0.0",
		},
		SourceDescriptions: []*SourceDescription{
			{Name: "api", URL: "./openapi.yaml", Type: SourceDescriptionTypeOpenAPI},
		},
		Operations: []*Operation{
			{
				OperationID:        "get_data",
				SourceDescription:  "api",
				OpenAPIOperationID: "getData",
			},
		},
	}
}

func TestValidate_Valid(t *testing.T) {
	doc := validDocument()
	assert.NoError(t, doc.Validate())
}

func TestValidate_OpenAPIOperationRefValid(t *testing.T) {
	doc := validDocument()
	doc.Operations[0].OpenAPIOperationID = ""
	doc.Operations[0].OpenAPIOperationRef = "#/paths/~1data/get"
	assert.NoError(t, doc.Validate())
}

func TestOperationBindingHelpers(t *testing.T) {
	var nilOp *Operation
	assert.False(t, nilOp.HasOpenAPIBinding())
	assert.False(t, nilOp.HasCompleteOpenAPIBinding())
	assert.Empty(t, nilOp.ExtensionProfile())
	assert.False(t, nilOp.IsExtensionOwned())

	openAPIBound := &Operation{
		SourceDescription:  "api",
		OpenAPIOperationID: "getData",
	}
	assert.True(t, openAPIBound.HasOpenAPIBinding())
	assert.True(t, openAPIBound.HasCompleteOpenAPIBinding())
	assert.Empty(t, openAPIBound.ExtensionProfile())
	assert.False(t, openAPIBound.IsExtensionOwned())

	partialOpenAPIBinding := &Operation{OpenAPIOperationID: "getData"}
	assert.True(t, partialOpenAPIBinding.HasOpenAPIBinding())
	assert.False(t, partialOpenAPIBinding.HasCompleteOpenAPIBinding())
	assert.False(t, partialOpenAPIBinding.IsExtensionOwned())

	conflictingOpenAPIBinding := &Operation{
		SourceDescription:   "api",
		OpenAPIOperationID:  "getData",
		OpenAPIOperationRef: "#/paths/~1data/get",
	}
	assert.True(t, conflictingOpenAPIBinding.HasOpenAPIBinding())
	assert.False(t, conflictingOpenAPIBinding.HasCompleteOpenAPIBinding())

	extensionOwned := &Operation{
		Extensions: map[string]any{ExtensionOperationProfile: " udon "},
	}
	assert.False(t, extensionOwned.HasOpenAPIBinding())
	assert.False(t, extensionOwned.HasCompleteOpenAPIBinding())
	assert.Equal(t, "udon", extensionOwned.ExtensionProfile())
	assert.True(t, extensionOwned.IsExtensionOwned())

	whitespaceProfile := &Operation{
		Extensions: map[string]any{ExtensionOperationProfile: "   "},
	}
	assert.Empty(t, whitespaceProfile.ExtensionProfile())
	assert.False(t, whitespaceProfile.IsExtensionOwned())

	nonStringProfile := &Operation{
		Extensions: map[string]any{ExtensionOperationProfile: 1},
	}
	assert.Empty(t, nonStringProfile.ExtensionProfile())
	assert.False(t, nonStringProfile.IsExtensionOwned())
}

func TestValidate_MissingRootFields(t *testing.T) {
	doc := validDocument()
	doc.UWS = ""
	doc.Info = nil
	doc.SourceDescriptions = nil
	doc.Operations = nil

	err := doc.Validate()
	assert.ErrorContains(t, err, "uws version is required")
	assert.ErrorContains(t, err, "info is required")
	assert.ErrorContains(t, err, "operations at least one operation is required")
}

func TestValidate_BadVersionPattern(t *testing.T) {
	doc := validDocument()
	doc.UWS = "2.0.0"
	assert.ErrorContains(t, doc.Validate(), "does not match pattern")
}

func TestValidate_InfoRequiredFields(t *testing.T) {
	doc := validDocument()
	doc.Info.Title = ""
	doc.Info.Version = ""

	err := doc.Validate()
	assert.ErrorContains(t, err, "info.title is required")
	assert.ErrorContains(t, err, "info.version is required")
}

func TestValidate_OperationBindingRules(t *testing.T) {
	t.Run("missing operationId", func(t *testing.T) {
		doc := validDocument()
		doc.Operations[0].OperationID = ""
		assert.ErrorContains(t, doc.Validate(), "operationId is required")
	})

	t.Run("missing sourceDescription", func(t *testing.T) {
		doc := validDocument()
		doc.Operations[0].SourceDescription = ""
		assert.ErrorContains(t, doc.Validate(), "sourceDescription is required for OpenAPI-bound operations")
	})

	t.Run("unknown sourceDescription", func(t *testing.T) {
		doc := validDocument()
		doc.Operations[0].SourceDescription = "missing"
		assert.ErrorContains(t, doc.Validate(), `references unknown sourceDescription "missing"`)
	})

	t.Run("missing OpenAPI binding", func(t *testing.T) {
		doc := validDocument()
		doc.Operations[0].OpenAPIOperationID = ""
		assert.ErrorContains(t, doc.Validate(), "requires exactly one of openapiOperationId or openapiOperationRef for OpenAPI-bound operations")
	})

	t.Run("conflicting OpenAPI bindings", func(t *testing.T) {
		doc := validDocument()
		doc.Operations[0].OpenAPIOperationRef = "#/paths/~1data/get"
		assert.ErrorContains(t, doc.Validate(), "cannot specify both openapiOperationId and openapiOperationRef")
	})

	t.Run("extension-owned operation", func(t *testing.T) {
		doc := &Document{
			UWS:  "1.0.0",
			Info: &Info{Title: "Extension", Version: "1.0.0"},
			Operations: []*Operation{
				{
					OperationID: "build_email",
					Extensions: map[string]any{
						ExtensionOperationProfile: "udon",
						"x-udon-runtime":          map[string]any{"type": "fnct", "function": "mail_raw"},
					},
				},
			},
		}
		assert.NoError(t, doc.Validate())
	})

	t.Run("missing binding and extension", func(t *testing.T) {
		doc := &Document{
			UWS:        "1.0.0",
			Info:       &Info{Title: "Invalid", Version: "1.0.0"},
			Operations: []*Operation{{OperationID: "op"}},
		}
		assert.ErrorContains(t, doc.Validate(), "requires an OpenAPI binding or x-uws-operation-profile")
	})

	t.Run("extension-owned operation requires profile marker", func(t *testing.T) {
		doc := &Document{
			UWS:  "1.0.0",
			Info: &Info{Title: "Extension", Version: "1.0.0"},
			Operations: []*Operation{
				{
					OperationID: "build_email",
					Extensions: map[string]any{
						"x-udon-runtime": map[string]any{"type": "fnct", "function": "mail_raw"},
					},
				},
			},
		}
		assert.ErrorContains(t, doc.Validate(), "requires an OpenAPI binding or x-uws-operation-profile")
	})

	t.Run("extension-owned operation requires non-whitespace profile marker", func(t *testing.T) {
		doc := &Document{
			UWS:  "1.0.0",
			Info: &Info{Title: "Extension", Version: "1.0.0"},
			Operations: []*Operation{
				{
					OperationID: "build_email",
					Extensions: map[string]any{
						ExtensionOperationProfile: "   ",
						"x-udon-runtime":          map[string]any{"type": "fnct", "function": "mail_raw"},
					},
				},
			},
		}
		assert.ErrorContains(t, doc.Validate(), "requires an OpenAPI binding or x-uws-operation-profile")
	})

	t.Run("non pointer operation ref", func(t *testing.T) {
		doc := validDocument()
		doc.Operations[0].OpenAPIOperationID = ""
		doc.Operations[0].OpenAPIOperationRef = "operation://getData"
		assert.ErrorContains(t, doc.Validate(), "must be a JSON Pointer fragment")
	})

	t.Run("standard request binding keys", func(t *testing.T) {
		doc := validDocument()
		doc.Operations[0].Request = map[string]any{
			"path":   map[string]any{"id": "123"},
			"query":  map[string]any{"limit": 10},
			"header": map[string]any{"X-Test": "ok"},
			"cookie": map[string]any{"session": "abc"},
			"body":   map[string]any{"name": "widget"},
			"x-test": map[string]any{"ok": true},
		}
		assert.NoError(t, doc.Validate())
	})

	t.Run("unknown request binding key", func(t *testing.T) {
		doc := validDocument()
		doc.Operations[0].Request = map[string]any{"limit": 10}
		assert.ErrorContains(t, doc.Validate(), "is not a standard request binding key")
	})

	t.Run("request parameter sections must be objects", func(t *testing.T) {
		doc := validDocument()
		doc.Operations[0].Request = map[string]any{"query": "limit=10"}
		assert.ErrorContains(t, doc.Validate(), "request.query must be an object")
	})
}

func TestValidate_DuplicateIDs(t *testing.T) {
	doc := validDocument()
	doc.SourceDescriptions = append(doc.SourceDescriptions, &SourceDescription{Name: "api", URL: "./other.yaml"})
	doc.Operations = append(doc.Operations, &Operation{
		OperationID:        "get_data",
		SourceDescription:  "api",
		OpenAPIOperationID: "getOtherData",
	})
	doc.Workflows = []*Workflow{
		{WorkflowID: "wf", Type: "parallel"},
		{WorkflowID: "wf", Type: "switch"},
	}
	doc.Triggers = []*Trigger{
		{TriggerID: "t1"},
		{TriggerID: "t1"},
	}

	err := doc.Validate()
	assert.ErrorContains(t, err, "duplicate sourceDescription name")
	assert.ErrorContains(t, err, "duplicate operationId")
	assert.ErrorContains(t, err, "duplicate workflowId")
	assert.ErrorContains(t, err, "duplicate triggerId")
}

func TestValidate_SourceDescriptions(t *testing.T) {
	t.Run("missing name", func(t *testing.T) {
		doc := validDocument()
		doc.SourceDescriptions[0].Name = ""
		assert.ErrorContains(t, doc.Validate(), "sourceDescriptions[0].name is required")
	})

	t.Run("missing url", func(t *testing.T) {
		doc := validDocument()
		doc.SourceDescriptions[0].URL = ""
		assert.ErrorContains(t, doc.Validate(), "sourceDescriptions[0].url is required")
	})

	t.Run("invalid name", func(t *testing.T) {
		doc := validDocument()
		doc.SourceDescriptions[0].Name = "bad name"
		assert.ErrorContains(t, doc.Validate(), "must match pattern")
	})

	t.Run("invalid type", func(t *testing.T) {
		doc := validDocument()
		doc.SourceDescriptions[0].Type = "arazzo"
		assert.ErrorContains(t, doc.Validate(), "must be openapi")
	})

	t.Run("omitted type", func(t *testing.T) {
		doc := validDocument()
		doc.SourceDescriptions[0].Type = ""
		assert.NoError(t, doc.Validate())
	})
}

func TestValidate_SourceDescriptionsRequiredWhenBound(t *testing.T) {
	t.Run("missing top-level array with bound op", func(t *testing.T) {
		doc := validDocument()
		doc.SourceDescriptions = nil
		err := doc.Validate()
		require.Error(t, err)
		assert.ErrorContains(t, err, "sourceDescriptions is required when any operation declares sourceDescription")
	})

	t.Run("empty top-level array with bound op", func(t *testing.T) {
		doc := validDocument()
		doc.SourceDescriptions = []*SourceDescription{}
		err := doc.Validate()
		require.Error(t, err)
		assert.ErrorContains(t, err, "sourceDescriptions is required when any operation declares sourceDescription")
	})

	t.Run("extension-owned op needs no sourceDescriptions", func(t *testing.T) {
		doc := &Document{
			UWS:  "1.0.0",
			Info: &Info{Title: "Ext", Version: "1.0.0"},
			Operations: []*Operation{
				{
					OperationID: "do_thing",
					Extensions:  map[string]any{ExtensionOperationProfile: "udon"},
				},
			},
		}
		assert.NoError(t, doc.Validate())
	})
}

func TestValidate_CriteriaAndActions(t *testing.T) {
	doc := validDocument()
	doc.Workflows = []*Workflow{{WorkflowID: "next", Type: "parallel"}}
	doc.Operations[0].SuccessCriteria = []*Criterion{
		{Condition: "$response.statusCode == 200"},
		{Condition: "^ok", Type: CriterionRegex, Context: "$response.body"},
	}
	doc.Operations[0].OnFailure = []*FailureAction{
		{Name: "retry", Type: "retry", RetryAfter: 5, RetryLimit: 3},
		{Name: "abort", Type: "end"},
	}
	doc.Operations[0].OnSuccess = []*SuccessAction{
		{Name: "continue", Type: "end"},
		{Name: "route", Type: "goto", WorkflowID: "next"},
	}
	assert.NoError(t, doc.Validate())
}

func TestValidate_CriteriaAndActionsErrors(t *testing.T) {
	doc := validDocument()
	doc.Operations[0].SuccessCriteria = []*Criterion{
		{Type: CriterionSimple},
		{Condition: "^ok", Type: CriterionRegex},
		{Condition: "test", Type: "invalid"},
	}
	doc.Operations[0].OnFailure = []*FailureAction{
		{Name: "retry", Type: "retry"},
		{Name: "bad", Type: "skip"},
		{Name: "goto", Type: "goto"},
	}
	doc.Operations[0].OnSuccess = []*SuccessAction{
		{Name: "bad", Type: "retry"},
		{Name: "goto", Type: "goto"},
	}

	err := doc.Validate()
	assert.ErrorContains(t, err, "condition is required")
	assert.ErrorContains(t, err, "context is required")
	assert.ErrorContains(t, err, "retry requires retryLimit > 0")
	assert.ErrorContains(t, err, "goto requires workflowId or stepId")
	assert.ErrorContains(t, err, "must be end")
}

func TestCriterionUnmarshalRejectsExplicitEmptyType(t *testing.T) {
	var criterion Criterion
	require.ErrorContains(t, json.Unmarshal([]byte(`{"condition":"true","type":""}`), &criterion), "criterion.type must be omitted")

	require.NoError(t, json.Unmarshal([]byte(`{"condition":"true"}`), &criterion))
	assert.Empty(t, criterion.Type)
}

func TestValidate_ActionTargetsOnlyAllowedForGoto(t *testing.T) {
	t.Run("failure end", func(t *testing.T) {
		doc := validDocument()
		doc.Operations[0].OnFailure = []*FailureAction{
			{Name: "stop", Type: "end", WorkflowID: "main"},
		}
		assert.ErrorContains(t, doc.Validate(), "workflowId/stepId are only valid for goto actions")
	})

	t.Run("failure retry", func(t *testing.T) {
		doc := validDocument()
		doc.Operations[0].OnFailure = []*FailureAction{
			{Name: "retry", Type: "retry", RetryLimit: 1, StepID: "step"},
		}
		assert.ErrorContains(t, doc.Validate(), "workflowId/stepId are only valid for goto actions")
	})

	t.Run("success end", func(t *testing.T) {
		doc := validDocument()
		doc.Operations[0].OnSuccess = []*SuccessAction{
			{Name: "done", Type: "end", StepID: "step"},
		}
		assert.ErrorContains(t, doc.Validate(), "workflowId/stepId are only valid for goto actions")
	})
}

func TestValidate_WorkflowAndStepReferences(t *testing.T) {
	doc := validDocument()
	doc.Workflows = []*Workflow{
		{
			WorkflowID: "wf",
			Type:       "parallel",
			WorkflowExecutionFields: WorkflowExecutionFields{
				DependsOn: []string{"missing_dependency"},
			},
			Steps: []*Step{
				{
					StepID:       "step",
					Type:         "not-a-step-type",
					OperationRef: "missing_operation",
					StepExecutionFields: StepExecutionFields{
						DependsOn: []string{"missing_step"},
					},
				},
			},
		},
	}

	err := doc.Validate()
	assert.ErrorContains(t, err, "references unknown dependency")
	assert.ErrorContains(t, err, "references unknown operationId")
	assert.ErrorContains(t, err, "not-a-step-type")
}

func TestValidate_WorkflowAndStepIDsRejectDots(t *testing.T) {
	t.Run("workflowId", func(t *testing.T) {
		doc := validDocument()
		doc.Workflows = []*Workflow{
			{WorkflowID: "daily.v1", Type: "sequence"},
		}
		assert.ErrorContains(t, doc.Validate(), "workflowId must match pattern")
	})

	t.Run("stepId", func(t *testing.T) {
		doc := validDocument()
		doc.Workflows = []*Workflow{
			{
				WorkflowID: "wf",
				Type:       "sequence",
				Steps:      []*Step{{StepID: "fetch.user", OperationRef: "get_data"}},
			},
		}
		assert.ErrorContains(t, doc.Validate(), "stepId must match pattern")
	})
}

func TestValidate_SequenceWorkflowOperationStep(t *testing.T) {
	doc := validDocument()
	doc.Workflows = []*Workflow{
		{
			WorkflowID: "main",
			Type:       "sequence",
			Steps: []*Step{
				{
					StepID:       "get_data",
					OperationRef: "get_data",
				},
			},
		},
	}

	assert.NoError(t, doc.Validate())
}

func TestValidate_GotoStepID(t *testing.T) {
	doc := validDocument()
	doc.Workflows = []*Workflow{
		{
			WorkflowID: "handler",
			Type:       "parallel",
			Steps:      []*Step{{StepID: "fallback_step", Type: "sequence"}},
		},
	}
	doc.Operations[0].OnFailure = []*FailureAction{
		{Name: "action", Type: "goto", StepID: "fallback_step"},
	}

	assert.NoError(t, doc.Validate())
}

func TestValidate_GotoRejectsBothTargets(t *testing.T) {
	doc := validDocument()
	doc.Workflows = []*Workflow{
		{
			WorkflowID: "handler",
			Type:       "parallel",
			Steps:      []*Step{{StepID: "fallback_step", Type: "sequence"}},
		},
	}
	doc.Operations[0].OnFailure = []*FailureAction{
		{Name: "action", Type: "goto", WorkflowID: "handler", StepID: "fallback_step"},
	}

	assert.ErrorContains(t, doc.Validate(), "goto cannot specify both workflowId and stepId")
}

func TestValidate_TriggerRoutes(t *testing.T) {
	t.Run("valid named output", func(t *testing.T) {
		doc := validDocument()
		doc.Triggers = []*Trigger{
			{
				TriggerID: "webhook",
				Outputs:   []string{"primary"},
				Routes: []*TriggerRoute{
					{TriggerRouteFields: TriggerRouteFields{Output: "primary", To: []string{"main"}}},
				},
			},
		}
		doc.Workflows = []*Workflow{{
			WorkflowID: "main",
			Type:       WorkflowTypeSequence,
			Steps:      []*Step{{StepID: "get_data", OperationRef: "get_data"}},
		}}
		assert.NoError(t, doc.Validate())
	})

	t.Run("valid decimal index output", func(t *testing.T) {
		doc := validDocument()
		doc.Triggers = []*Trigger{
			{
				TriggerID: "webhook",
				Outputs:   []string{"primary", "secondary"},
				Routes: []*TriggerRoute{
					{TriggerRouteFields: TriggerRouteFields{Output: "1", To: []string{"step_a"}}},
				},
			},
		}
		doc.Workflows = []*Workflow{{
			WorkflowID: "main",
			Type:       WorkflowTypeSequence,
			Steps:      []*Step{{StepID: "step_a", OperationRef: "get_data"}},
		}}
		assert.NoError(t, doc.Validate())
	})

	t.Run("unknown target", func(t *testing.T) {
		doc := validDocument()
		doc.Triggers = []*Trigger{
			{
				TriggerID: "webhook",
				Outputs:   []string{"primary"},
				Routes: []*TriggerRoute{
					{TriggerRouteFields: TriggerRouteFields{Output: "primary", To: []string{"missing"}}},
				},
			},
		}
		doc.Workflows = []*Workflow{{
			WorkflowID: "main",
			Type:       WorkflowTypeSequence,
			Steps:      []*Step{{StepID: "get_data", OperationRef: "get_data"}},
		}}
		assert.ErrorContains(t, doc.Validate(), "references unknown top-level stepId or workflowId")
	})

	t.Run("empty target list", func(t *testing.T) {
		doc := validDocument()
		doc.Triggers = []*Trigger{
			{
				TriggerID: "webhook",
				Outputs:   []string{"primary"},
				Routes: []*TriggerRoute{
					{TriggerRouteFields: TriggerRouteFields{Output: "primary"}},
				},
			},
		}
		assert.ErrorContains(t, doc.Validate(), "must contain at least one top-level stepId or workflowId")
	})

	t.Run("routes without outputs declaration", func(t *testing.T) {
		doc := validDocument()
		doc.Triggers = []*Trigger{
			{
				TriggerID: "webhook",
				Routes: []*TriggerRoute{
					{TriggerRouteFields: TriggerRouteFields{Output: "primary", To: []string{"main"}}},
				},
			},
		}
		assert.ErrorContains(t, doc.Validate(), "outputs is required when routes is set")
	})

	t.Run("route output not declared", func(t *testing.T) {
		doc := validDocument()
		doc.Triggers = []*Trigger{
			{
				TriggerID: "webhook",
				Outputs:   []string{"primary"},
				Routes: []*TriggerRoute{
					{TriggerRouteFields: TriggerRouteFields{Output: "other", To: []string{"main"}}},
				},
			},
		}
		assert.ErrorContains(t, doc.Validate(), `"other" is not a declared trigger output`)
	})

	t.Run("index out of range", func(t *testing.T) {
		doc := validDocument()
		doc.Triggers = []*Trigger{
			{
				TriggerID: "webhook",
				Outputs:   []string{"primary"},
				Routes: []*TriggerRoute{
					{TriggerRouteFields: TriggerRouteFields{Output: "5", To: []string{"main"}}},
				},
			},
		}
		assert.ErrorContains(t, doc.Validate(), `"5" is not a declared trigger output`)
	})

	t.Run("duplicate outputs rejected", func(t *testing.T) {
		doc := validDocument()
		doc.Triggers = []*Trigger{
			{
				TriggerID: "webhook",
				Outputs:   []string{"primary", "primary"},
				Routes: []*TriggerRoute{
					{TriggerRouteFields: TriggerRouteFields{Output: "primary", To: []string{"main"}}},
				},
			},
		}
		assert.ErrorContains(t, doc.Validate(), `duplicate output "primary"`)
	})

	t.Run("rejects non-top-level step target", func(t *testing.T) {
		doc := validDocument()
		doc.Workflows = []*Workflow{{
			WorkflowID: "main",
			Type:       WorkflowTypeSequence,
			Steps: []*Step{{
				StepID: "outer",
				Type:   WorkflowTypeSequence,
				Steps:  []*Step{{StepID: "nested", OperationRef: "get_data"}},
			}},
		}}
		doc.Triggers = []*Trigger{{
			TriggerID: "webhook",
			Outputs:   []string{"primary"},
			Routes: []*TriggerRoute{
				{TriggerRouteFields: TriggerRouteFields{Output: "primary", To: []string{"nested"}}},
			},
		}}
		assert.ErrorContains(t, doc.Validate(), "references unknown top-level stepId or workflowId")
	})

	t.Run("ambiguous entry workflow defers step-target validation to executable layer", func(t *testing.T) {
		doc := validDocument()
		doc.Workflows = []*Workflow{
			{
				WorkflowID: "alpha",
				Type:       WorkflowTypeSequence,
				Steps:      []*Step{{StepID: "start", OperationRef: "get_data"}},
			},
			{
				WorkflowID: "beta",
				Type:       WorkflowTypeSequence,
				Steps:      []*Step{{StepID: "other", OperationRef: "get_data"}},
			},
		}
		doc.Triggers = []*Trigger{{
			TriggerID: "webhook",
			Outputs:   []string{"primary"},
			Routes: []*TriggerRoute{
				{TriggerRouteFields: TriggerRouteFields{Output: "primary", To: []string{"start"}}},
			},
		}}

		err := doc.Validate()
		require.NoError(t, err)
		require.ErrorContains(t, doc.ValidateExecutable(), `multiple workflows require an explicit "main" entry workflow`)
	})
}

func TestValidate_StructuralTypeConstraints(t *testing.T) {
	t.Run("loop requires items", func(t *testing.T) {
		doc := validDocument()
		doc.Workflows = []*Workflow{
			{WorkflowID: "loop_wf", Type: WorkflowTypeLoop},
		}
		assert.ErrorContains(t, doc.Validate(), "items is required for loop")
	})

	t.Run("loop with items is valid", func(t *testing.T) {
		doc := validDocument()
		doc.Workflows = []*Workflow{
			{WorkflowID: "loop_wf", Type: WorkflowTypeLoop, StructuralFields: StructuralFields{Items: "$variables.tags"}},
		}
		assert.NoError(t, doc.Validate())
	})

	t.Run("await requires wait", func(t *testing.T) {
		doc := validDocument()
		doc.Workflows = []*Workflow{
			{WorkflowID: "await_wf", Type: WorkflowTypeAwait},
		}
		assert.ErrorContains(t, doc.Validate(), "wait is required for await")
	})

	t.Run("await with wait is valid", func(t *testing.T) {
		doc := validDocument()
		doc.Workflows = []*Workflow{
			{WorkflowID: "await_wf", Type: WorkflowTypeAwait, WorkflowExecutionFields: WorkflowExecutionFields{Wait: "$response.statusCode == 200"}},
		}
		assert.NoError(t, doc.Validate())
	})

	t.Run("switch rejects items", func(t *testing.T) {
		doc := validDocument()
		doc.Workflows = []*Workflow{
			{WorkflowID: "sw", Type: WorkflowTypeSwitch, StructuralFields: StructuralFields{Items: "$variables.tags"}},
		}
		assert.ErrorContains(t, doc.Validate(), "items is not valid on switch")
	})

	t.Run("sequence rejects default", func(t *testing.T) {
		doc := validDocument()
		doc.Workflows = []*Workflow{
			{
				WorkflowID: "wf",
				Type:       WorkflowTypeSequence,
				Default:    []*Step{{StepID: "fallback", OperationRef: "get_data"}},
			},
		}
		assert.ErrorContains(t, doc.Validate(), "default is not valid on sequence")
	})

	t.Run("parallel rejects cases", func(t *testing.T) {
		doc := validDocument()
		doc.Workflows = []*Workflow{
			{
				WorkflowID: "wf",
				Type:       WorkflowTypeParallel,
				Cases:      []*Case{{CaseFields: CaseFields{Name: "a"}}},
			},
		}
		assert.ErrorContains(t, doc.Validate(), "cases is not valid on parallel")
	})

	t.Run("switch allows cases and default", func(t *testing.T) {
		doc := validDocument()
		doc.Workflows = []*Workflow{
			{
				WorkflowID: "wf",
				Type:       WorkflowTypeSwitch,
				Cases:      []*Case{{CaseFields: CaseFields{Name: "premium"}}},
				Default:    []*Step{{StepID: "fallback", OperationRef: "get_data"}},
			},
		}
		assert.NoError(t, doc.Validate())
	})

	t.Run("step with loop type requires items", func(t *testing.T) {
		doc := validDocument()
		doc.Workflows = []*Workflow{
			{
				WorkflowID: "wf",
				Type:       WorkflowTypeParallel,
				Steps:      []*Step{{StepID: "loop_step", Type: WorkflowTypeLoop}},
			},
		}
		assert.ErrorContains(t, doc.Validate(), "items is required for loop")
	})

	t.Run("merge workflow requires dependsOn", func(t *testing.T) {
		doc := validDocument()
		doc.Workflows = []*Workflow{
			{WorkflowID: "merge_wf", Type: WorkflowTypeMerge},
		}
		assert.ErrorContains(t, doc.Validate(), "dependsOn is required and must name at least one upstream construct for merge")
	})

	t.Run("merge step requires dependsOn", func(t *testing.T) {
		doc := validDocument()
		doc.Workflows = []*Workflow{
			{
				WorkflowID: "wf",
				Type:       WorkflowTypeParallel,
				Steps:      []*Step{{StepID: "merge_step", Type: WorkflowTypeMerge}},
			},
		}
		assert.ErrorContains(t, doc.Validate(), "dependsOn is required and must name at least one upstream construct for merge")
	})
}

func TestValidate_StructuralResult(t *testing.T) {
	baseDoc := func() *Document {
		doc := validDocument()
		doc.Workflows = []*Workflow{
			{
				WorkflowID: "wf_merge",
				Type:       WorkflowTypeMerge,
				WorkflowExecutionFields: WorkflowExecutionFields{
					DependsOn: []string{"get_data"},
				},
			},
			{
				WorkflowID: "wf_parallel",
				Type:       WorkflowTypeParallel,
				Steps: []*Step{{StepID: "merge_step", Type: WorkflowTypeMerge, StepExecutionFields: StepExecutionFields{
					DependsOn: []string{"get_data"},
				}}},
			},
		}
		return doc
	}

	t.Run("valid workflow reference", func(t *testing.T) {
		doc := baseDoc()
		doc.Results = []*StructuralResult{
			{Name: "out", Kind: StructuralResultKindMerge, From: "wf_merge"},
		}
		assert.NoError(t, doc.Validate())
	})

	t.Run("valid step reference", func(t *testing.T) {
		doc := baseDoc()
		doc.Results = []*StructuralResult{
			{Name: "out", Kind: StructuralResultKindMerge, From: "wf_parallel.merge_step"},
		}
		assert.NoError(t, doc.Validate())
	})

	t.Run("missing from", func(t *testing.T) {
		doc := baseDoc()
		doc.Results = []*StructuralResult{
			{Name: "out", Kind: StructuralResultKindMerge},
		}
		assert.ErrorContains(t, doc.Validate(), "from is required")
	})

	t.Run("missing name", func(t *testing.T) {
		doc := baseDoc()
		doc.Results = []*StructuralResult{
			{Kind: "merge", From: "wf_merge"},
		}
		assert.ErrorContains(t, doc.Validate(), "name is required")
	})

	t.Run("missing kind", func(t *testing.T) {
		doc := baseDoc()
		doc.Results = []*StructuralResult{
			{Name: "out", From: "wf_merge"},
		}
		assert.ErrorContains(t, doc.Validate(), "kind is required")
	})

	t.Run("invalid kind", func(t *testing.T) {
		doc := baseDoc()
		doc.Results = []*StructuralResult{
			{Name: "out", Kind: "parallel", From: "wf_merge"},
		}
		assert.ErrorContains(t, doc.Validate(), `"parallel" is not valid`)
	})

	t.Run("unknown workflow", func(t *testing.T) {
		doc := baseDoc()
		doc.Results = []*StructuralResult{
			{Name: "out", Kind: "merge", From: "missing_wf"},
		}
		assert.ErrorContains(t, doc.Validate(), `references unknown workflowId "missing_wf"`)
	})

	t.Run("invalid from shape", func(t *testing.T) {
		for _, from := range []string{"a..b", "a.b.c", ".step", "wf."} {
			doc := baseDoc()
			doc.Results = []*StructuralResult{
				{Name: "out", Kind: "merge", From: from},
			}
			assert.ErrorContains(t, doc.Validate(), "is not a valid workflowId or workflowId.stepId")
		}
	})

	t.Run("unknown step in workflow", func(t *testing.T) {
		doc := baseDoc()
		doc.Results = []*StructuralResult{
			{Name: "out", Kind: "merge", From: "wf_parallel.missing_step"},
		}
		assert.ErrorContains(t, doc.Validate(), `references unknown stepId "missing_step"`)
	})

	t.Run("operation step is not structural result source", func(t *testing.T) {
		doc := baseDoc()
		doc.Workflows[1].Steps = append(doc.Workflows[1].Steps, &Step{StepID: "fetch", OperationRef: "get_data"})
		doc.Results = []*StructuralResult{
			{Name: "out", Kind: "merge", From: "wf_parallel.fetch"},
		}
		assert.ErrorContains(t, doc.Validate(), "is not a structural construct")
	})

	t.Run("kind mismatch with workflow type", func(t *testing.T) {
		doc := baseDoc()
		doc.Results = []*StructuralResult{
			{Name: "out", Kind: "loop", From: "wf_merge"},
		}
		assert.ErrorContains(t, doc.Validate(), `kind "loop" does not match`)
	})

	t.Run("duplicate name", func(t *testing.T) {
		doc := baseDoc()
		doc.Results = []*StructuralResult{
			{Name: "out", Kind: "merge", From: "wf_merge"},
			{Name: "out", Kind: "merge", From: "wf_parallel.merge_step"},
		}
		assert.ErrorContains(t, doc.Validate(), `duplicate result name "out"`)
	})
}

func TestValidate_OpenAPIOperationRefRequiresPointer(t *testing.T) {
	doc := validDocument()
	doc.Operations[0].OpenAPIOperationID = ""
	doc.Operations[0].OpenAPIOperationRef = "paths/~1data/get"
	assert.ErrorContains(t, doc.Validate(), "must be a JSON Pointer fragment beginning with #/")
}

func TestValidate_ComponentsVariables(t *testing.T) {
	doc := validDocument()
	doc.Components = &Components{
		Variables: map[string]any{
			"ok_name":  true,
			"bad name": false,
		},
	}

	assert.ErrorContains(t, doc.Validate(), "component name")
}

func TestValidationResult_ErrorFormat(t *testing.T) {
	var nilResult *ValidationResult
	assert.True(t, nilResult.Valid())
	assert.Empty(t, nilResult.Error())

	empty := &ValidationResult{}
	assert.True(t, empty.Valid())
	assert.Empty(t, empty.Error())

	one := &ValidationResult{
		Errors: []ValidationError{
			{Path: "operations[0]", Message: "is invalid"},
		},
	}
	assert.False(t, one.Valid())
	assert.Equal(t, "operations[0] is invalid", one.Error())

	many := &ValidationResult{
		Errors: []ValidationError{
			{Path: "info.title", Message: "is required"},
			{Path: "uws", Message: "must match pattern"},
		},
	}
	assert.Equal(t, "info.title is required; uws must match pattern", many.Error())
}

func TestValidateResult_AccumulatesErrors(t *testing.T) {
	doc := &Document{
		UWS:  "2.0.0",
		Info: &Info{},
		SourceDescriptions: []*SourceDescription{
			{Name: "api", URL: "./openapi.yaml", Type: SourceDescriptionTypeOpenAPI},
		},
		Operations: []*Operation{
			{OperationID: "op", SourceDescription: "api"},
			{OperationID: "op", SourceDescription: "missing", OpenAPIOperationRef: "#/paths/~1x/get"},
		},
	}

	result := doc.ValidateResult()
	assert.False(t, result.Valid())
	want := []ValidationError{
		{Path: "uws", Message: `version "2.0.0" does not match pattern 1.x.x`},
		{Path: "info.title", Message: "is required"},
		{Path: "info.version", Message: "is required"},
		{Path: "operations[1].operationId", Message: `duplicate operationId "op"`},
		{Path: "operations[0]", Message: "requires exactly one of openapiOperationId or openapiOperationRef for OpenAPI-bound operations"},
		{Path: "operations[1].sourceDescription", Message: `references unknown sourceDescription "missing"`},
	}
	assert.Equal(t, want, result.Errors)
}

func TestValidateResult_StructuredErrorShape(t *testing.T) {
	// Each case exercises one distinct error path and asserts the exact
	// ValidationError tuple rather than substring-matching the flattened
	// string.
	t.Run("duplicate workflowId", func(t *testing.T) {
		doc := validDocument()
		doc.Workflows = []*Workflow{
			{WorkflowID: "w", Type: "sequence"},
			{WorkflowID: "w", Type: "sequence"},
		}
		result := doc.ValidateResult()
		assert.Contains(t, result.Errors, ValidationError{
			Path:    "workflows[1].workflowId",
			Message: `duplicate workflowId "w"`,
		})
	})

	t.Run("unknown trigger route output", func(t *testing.T) {
		doc := validDocument()
		doc.Triggers = []*Trigger{
			{
				TriggerID: "hook",
				Outputs:   []string{"ok"},
				Routes: []*TriggerRoute{
					{TriggerRouteFields: TriggerRouteFields{Output: "missing", To: []string{"get_data"}}},
				},
			},
		}
		result := doc.ValidateResult()
		assert.Contains(t, result.Errors, ValidationError{
			Path:    "triggers[0].routes[0].output",
			Message: `"missing" is not a declared trigger output`,
		})
	})

	t.Run("merge workflow missing dependsOn", func(t *testing.T) {
		doc := validDocument()
		doc.Workflows = []*Workflow{
			{WorkflowID: "m", Type: "merge"},
		}
		result := doc.ValidateResult()
		assert.Contains(t, result.Errors, ValidationError{
			Path:    "workflows[0].dependsOn",
			Message: "is required and must name at least one upstream construct for merge",
		})
	})

	t.Run("criterion regex without context", func(t *testing.T) {
		doc := validDocument()
		doc.Operations[0].SuccessCriteria = []*Criterion{
			{Condition: "foo", Type: CriterionRegex},
		}
		result := doc.ValidateResult()
		assert.Contains(t, result.Errors, ValidationError{
			Path:    "operations[0].successCriteria[0].context",
			Message: "is required when type is regex, jsonpath, or xpath",
		})
	})

	t.Run("failure action retry without limit", func(t *testing.T) {
		doc := validDocument()
		doc.Operations[0].OnFailure = []*FailureAction{
			{Name: "r", Type: "retry"},
		}
		result := doc.ValidateResult()
		assert.Contains(t, result.Errors, ValidationError{
			Path:    "operations[0].onFailure[0]",
			Message: "retry requires retryLimit > 0",
		})
	})
}

func TestValidate_DependencyCycle_TwoNodes(t *testing.T) {
	doc := validDocument()
	doc.Operations = []*Operation{
		{
			OperationID:        "a",
			SourceDescription:  "api",
			OpenAPIOperationID: "getA",
			OperationExecutionFields: OperationExecutionFields{
				DependsOn: []string{"b"},
			},
		},
		{
			OperationID:        "b",
			SourceDescription:  "api",
			OpenAPIOperationID: "getB",
			OperationExecutionFields: OperationExecutionFields{
				DependsOn: []string{"a"},
			},
		},
	}

	err := doc.Validate()
	assert.ErrorContains(t, err, "cycle detected")
	assert.ErrorContains(t, err, "a -> b -> a")
}

func TestValidate_DependencyCycle_SelfLoop(t *testing.T) {
	doc := validDocument()
	doc.Operations[0].DependsOn = []string{"get_data"}

	err := doc.Validate()
	assert.ErrorContains(t, err, "cycle detected: get_data -> get_data")
}

func TestValidate_DependencyCycle_ThreeNodes(t *testing.T) {
	doc := validDocument()
	doc.Operations = []*Operation{
		{OperationID: "a", SourceDescription: "api", OpenAPIOperationID: "getA", OperationExecutionFields: OperationExecutionFields{DependsOn: []string{"b"}}},
		{OperationID: "b", SourceDescription: "api", OpenAPIOperationID: "getB", OperationExecutionFields: OperationExecutionFields{DependsOn: []string{"c"}}},
		{OperationID: "c", SourceDescription: "api", OpenAPIOperationID: "getC", OperationExecutionFields: OperationExecutionFields{DependsOn: []string{"a"}}},
	}

	err := doc.Validate()
	assert.ErrorContains(t, err, "cycle detected")
	assert.ErrorContains(t, err, "a -> b -> c -> a")
}

func TestValidate_DependencyCycle_ThroughParallelGroup(t *testing.T) {
	doc := validDocument()
	doc.Operations = []*Operation{
		{
			OperationID:        "a",
			SourceDescription:  "api",
			OpenAPIOperationID: "getA",
			OperationExecutionFields: OperationExecutionFields{
				ParallelGroup: "fanout",
				DependsOn:     []string{"b"},
			},
		},
		{
			OperationID:        "b",
			SourceDescription:  "api",
			OpenAPIOperationID: "getB",
			OperationExecutionFields: OperationExecutionFields{
				DependsOn: []string{"fanout"},
			},
		},
	}

	err := doc.Validate()
	assert.ErrorContains(t, err, "cycle detected")
}

func TestValidate_DependencyCycle_AcyclicIsFine(t *testing.T) {
	doc := validDocument()
	doc.Operations = []*Operation{
		{OperationID: "a", SourceDescription: "api", OpenAPIOperationID: "getA"},
		{OperationID: "b", SourceDescription: "api", OpenAPIOperationID: "getB", OperationExecutionFields: OperationExecutionFields{DependsOn: []string{"a"}}},
		{OperationID: "c", SourceDescription: "api", OpenAPIOperationID: "getC", OperationExecutionFields: OperationExecutionFields{DependsOn: []string{"a", "b"}}},
	}

	assert.NoError(t, doc.Validate())
}

func TestValidate_DependencyCycle_ReportedOnce(t *testing.T) {
	doc := validDocument()
	doc.Operations = []*Operation{
		{OperationID: "a", SourceDescription: "api", OpenAPIOperationID: "getA", OperationExecutionFields: OperationExecutionFields{DependsOn: []string{"b"}}},
		{OperationID: "b", SourceDescription: "api", OpenAPIOperationID: "getB", OperationExecutionFields: OperationExecutionFields{DependsOn: []string{"a"}}},
	}

	result := doc.ValidateResult()
	cycleErrors := 0
	for _, e := range result.Errors {
		if e.Path == "dependsOn" {
			cycleErrors++
		}
	}
	assert.Equal(t, 1, cycleErrors)
}

func TestValidate_ParamSchema_RequiredMustExistInProperties(t *testing.T) {
	doc := validDocument()
	doc.Workflows = []*Workflow{
		{
			WorkflowID: "wf",
			Type:       "sequence",
			Inputs: &ParamSchema{
				Type: "object",
				Properties: map[string]*ParamSchema{
					"limit": {Type: "integer"},
				},
				Required: []string{"missing"},
			},
		},
	}
	err := doc.Validate()
	assert.ErrorContains(t, err, `workflows[0].inputs.required[0]`)
	assert.ErrorContains(t, err, `references unknown property "missing"`)
}

func TestOperationRejectsWorkflowField(t *testing.T) {
	var op Operation
	err := json.Unmarshal([]byte(`{
		"operationId":"op",
		"sourceDescription":"api",
		"openapiOperationId":"getOp",
		"workflow":"child"
	}`), &op)
	assert.ErrorContains(t, err, "not defined by UWS core")
}

func TestValidate_ParamSchema_DuplicateRequired(t *testing.T) {
	doc := validDocument()
	doc.Workflows = []*Workflow{
		{
			WorkflowID: "wf",
			Type:       "sequence",
			Inputs: &ParamSchema{
				Properties: map[string]*ParamSchema{"a": {Type: "string"}},
				Required:   []string{"a", "a"},
			},
		},
	}
	err := doc.Validate()
	assert.ErrorContains(t, err, "duplicate required entry")
}

func TestValidate_ParamSchema_NilNestedSchema(t *testing.T) {
	doc := validDocument()
	doc.Workflows = []*Workflow{
		{
			WorkflowID: "wf",
			Type:       "sequence",
			Inputs: &ParamSchema{
				Type:  "array",
				AllOf: []*ParamSchema{nil},
			},
		},
	}
	err := doc.Validate()
	assert.ErrorContains(t, err, `workflows[0].inputs.allOf[0]`)
	assert.ErrorContains(t, err, "is nil")
}

func TestValidate_ParamSchema_RecursesIntoItems(t *testing.T) {
	doc := validDocument()
	doc.Workflows = []*Workflow{
		{
			WorkflowID: "wf",
			Type:       "sequence",
			Inputs: &ParamSchema{
				Type: "array",
				Items: &ParamSchema{
					Properties: map[string]*ParamSchema{"a": {Type: "string"}},
					Required:   []string{"b"},
				},
			},
		},
	}
	err := doc.Validate()
	assert.ErrorContains(t, err, `workflows[0].inputs.items.required[0]`)
	assert.ErrorContains(t, err, `references unknown property "b"`)
}

func TestValidate_Variables_AcceptsOpenShape(t *testing.T) {
	doc := validDocument()
	doc.Variables = map[string]any{
		"a-string": "hello",
		"a-number": 42,
		"a-bool":   true,
		"a-null":   nil,
		"a-list":   []any{1, "two", map[string]any{"nested": true}},
		"a-obj":    map[string]any{"deep": map[string]any{"deeper": []any{1, 2}}},
	}
	assert.NoError(t, doc.Validate())
}

func TestValidate_Components_Variables_KeyPatternEnforced(t *testing.T) {
	doc := validDocument()
	doc.Components = &Components{
		Variables: map[string]any{
			"valid.key": "ok",
			"bad key":   "nope",
		},
	}
	err := doc.Validate()
	assert.ErrorContains(t, err, `component name "bad key"`)
}

func TestValidate_TriggerOptions_AcceptsOpenShape(t *testing.T) {
	doc := validDocument()
	doc.Triggers = []*Trigger{
		{
			TriggerID: "webhook",
			Options: map[string]any{
				"string": "a",
				"int":    1,
				"nested": map[string]any{"list": []any{"x"}},
			},
		},
	}
	assert.NoError(t, doc.Validate())
}

func TestValidate_ParamSchema_ValidSchemaPasses(t *testing.T) {
	doc := validDocument()
	doc.Workflows = []*Workflow{
		{
			WorkflowID: "wf",
			Type:       "sequence",
			Inputs: &ParamSchema{
				Type: "object",
				Properties: map[string]*ParamSchema{
					"limit": {Type: "integer"},
					"name":  {Type: "string"},
				},
				Required: []string{"limit"},
			},
		},
	}
	assert.NoError(t, doc.Validate())
}

func TestValidate_InvalidFixtures(t *testing.T) {
	cases := []struct {
		name    string
		file    string
		wantErr ValidationError
	}{
		{
			name:    "bad uws version",
			file:    "bad_uws_version.json",
			wantErr: ValidationError{Path: "uws", Message: `version "2.0.0" does not match pattern 1.x.x`},
		},
		{
			name:    "dependency cycle",
			file:    "dependency_cycle.json",
			wantErr: ValidationError{Path: "dependsOn", Message: "cycle detected: a -> b -> a"},
		},
		{
			name:    "duplicate operation id",
			file:    "duplicate_operation_id.json",
			wantErr: ValidationError{Path: "operations[1].operationId", Message: `duplicate operationId "op"`},
		},
		{
			name:    "merge without dependsOn",
			file:    "merge_without_dependson.json",
			wantErr: ValidationError{Path: "workflows[0].dependsOn", Message: "is required and must name at least one upstream construct for merge"},
		},
		{
			name:    "missing openapi binding",
			file:    "missing_openapi_binding.json",
			wantErr: ValidationError{Path: "operations[0]", Message: "requires exactly one of openapiOperationId or openapiOperationRef for OpenAPI-bound operations"},
		},
		{
			name:    "regex criterion without context",
			file:    "regex_criterion_no_context.json",
			wantErr: ValidationError{Path: "operations[0].successCriteria[0].context", Message: "is required when type is regex, jsonpath, or xpath"},
		},
		{
			name:    "retry action without retryLimit",
			file:    "retry_without_limit.json",
			wantErr: ValidationError{Path: "operations[0].onFailure[0]", Message: "retry requires retryLimit > 0"},
		},
		{
			name:    "unknown sourceDescription",
			file:    "unknown_source_description.json",
			wantErr: ValidationError{Path: "operations[0].sourceDescription", Message: `references unknown sourceDescription "missing"`},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join("..", "testdata", "invalid", tc.file))
			require.NoError(t, err)
			var doc Document
			require.NoError(t, json.Unmarshal(data, &doc))

			result := doc.ValidateResult()
			assert.Contains(t, result.Errors, tc.wantErr,
				"expected error %+v in results %+v", tc.wantErr, result.Errors)
		})
	}
}
