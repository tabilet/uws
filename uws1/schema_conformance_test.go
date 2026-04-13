package uws1

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSchemaConformance_KeyValidatorRules(t *testing.T) {
	data, err := os.ReadFile("../uws.json")
	require.NoError(t, err)

	var schema map[string]any
	require.NoError(t, json.Unmarshal(data, &schema))
	defs := schema["$defs"].(map[string]any)

	workflow := defs["workflow-object"].(map[string]any)
	workflowRequired := workflow["required"].([]any)
	require.Contains(t, workflowRequired, "workflowId")
	require.Contains(t, workflowRequired, "type")
	workflowType := workflow["properties"].(map[string]any)["type"].(map[string]any)
	require.ElementsMatch(t, []any{"parallel", "switch", "merge", "loop", "await"}, workflowType["enum"].([]any))

	step := defs["step-object"].(map[string]any)
	stepRequired := step["required"].([]any)
	require.Contains(t, stepRequired, "stepId")
	require.Contains(t, stepRequired, "type")

	criterion := defs["criterion-object"].(map[string]any)
	criterionRequired := criterion["required"].([]any)
	require.Contains(t, criterionRequired, "condition")

	route := defs["trigger-route-object"].(map[string]any)
	require.Equal(t, "#/$defs/specification-extensions", route["$ref"])

	result := defs["structural-result-object"].(map[string]any)
	require.Equal(t, "#/$defs/specification-extensions", result["$ref"])
}

func TestSchemaConformance_ValidatorMatchesSelectedRules(t *testing.T) {
	doc := validDocument()
	doc.Workflows = []*Workflow{
		{WorkflowID: "wf", Type: "not-a-workflow", Steps: []*Step{{StepID: "s"}}},
	}

	err := doc.Validate()
	require.ErrorContains(t, err, "not-a-workflow")
	require.ErrorContains(t, err, "steps[0].type is required")

	doc = validDocument()
	doc.Triggers = []*Trigger{{TriggerID: "t", Routes: []*TriggerRoute{{}}}}
	require.ErrorContains(t, doc.Validate(), "routes[0].output is required")

	doc = validDocument()
	doc.Results = []*StructuralResult{{Kind: "await"}}
	require.ErrorContains(t, doc.Validate(), "await")
}
