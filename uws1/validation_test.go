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
					Type:         "not-a-step-type",
					OperationRef: "missing_operation",
					DependsOn:    []string{"missing_step"},
				},
			},
		},
	}

	err := doc.Validate()
	assert.ErrorContains(t, err, "references unknown dependency")
	assert.ErrorContains(t, err, "references unknown operationId")
	assert.ErrorContains(t, err, "not-a-step-type")
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
	t.Run("valid", func(t *testing.T) {
		doc := validDocument()
		doc.Triggers = []*Trigger{
			{
				TriggerID: "webhook",
				Routes: []*TriggerRoute{
					{Output: "0", To: []string{"get_data"}},
				},
			},
		}
		assert.NoError(t, doc.Validate())
	})

	t.Run("unknown target", func(t *testing.T) {
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
	})

	t.Run("empty target list", func(t *testing.T) {
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
	})
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
	assert.GreaterOrEqual(t, len(result.Errors), 4)
	assert.ErrorContains(t, result, "version")
	assert.ErrorContains(t, result, "info.title")
	assert.ErrorContains(t, result, "duplicate operationId")
	assert.ErrorContains(t, result, "requires exactly one")
	assert.ErrorContains(t, result, "references unknown sourceDescription")
}
