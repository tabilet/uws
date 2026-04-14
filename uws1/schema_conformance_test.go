package uws1

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
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
	require.ElementsMatch(t, []any{"sequence", "parallel", "switch", "merge", "loop", "await"}, workflowType["enum"].([]any))

	step := defs["step-object"].(map[string]any)
	stepRequired := step["required"].([]any)
	require.Contains(t, stepRequired, "stepId")
	require.NotContains(t, stepRequired, "type")
	stepType := step["properties"].(map[string]any)["type"].(map[string]any)
	require.ElementsMatch(t, mapKeys(validWorkflowTypes), stepType["enum"].([]any))

	operation := defs["operation-object"].(map[string]any)
	operationRequired := operation["required"].([]any)
	require.Contains(t, operationRequired, "operationId")
	require.NotContains(t, operationRequired, "sourceDescription")
	require.Contains(t, operation["properties"].(map[string]any), "openapiOperationId")
	require.Contains(t, operation["properties"].(map[string]any), "openapiOperationRef")
	require.Contains(t, operation["properties"].(map[string]any), "request")
	require.NotContains(t, operation["properties"].(map[string]any), "serviceType")

	criterion := defs["criterion-object"].(map[string]any)
	criterionRequired := criterion["required"].([]any)
	require.Contains(t, criterionRequired, "condition")

	route := defs["trigger-route-object"].(map[string]any)
	require.Equal(t, "#/$defs/specification-extensions", route["$ref"])
	require.Contains(t, route["required"].([]any), "to")

	result := defs["structural-result-object"].(map[string]any)
	require.Equal(t, "#/$defs/specification-extensions", result["$ref"])
	resultRequired := result["required"].([]any)
	require.Contains(t, resultRequired, "name")
	require.Contains(t, resultRequired, "kind")
	require.Contains(t, resultRequired, "from")
	require.Contains(t, result["properties"].(map[string]any), "value")

	trigger := defs["trigger-object"].(map[string]any)
	require.Contains(t, trigger["properties"].(map[string]any), "outputs")
}

func mapKeys(m map[string]bool) []any {
	keys := make([]any, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	return keys
}

func TestSchemaConformance_ValidatorMatchesSelectedRules(t *testing.T) {
	doc := validDocument()
	doc.Workflows = []*Workflow{
		{WorkflowID: "wf", Type: "not-a-workflow", Steps: []*Step{{StepID: "s"}}},
	}

	err := doc.Validate()
	require.ErrorContains(t, err, "not-a-workflow")
	require.NotContains(t, err.Error(), "steps[0].type is required")

	doc = validDocument()
	doc.Triggers = []*Trigger{{TriggerID: "t", Outputs: []string{"primary"}, Routes: []*TriggerRoute{{}}}}
	require.ErrorContains(t, doc.Validate(), "routes[0].output is required")

	doc = validDocument()
	doc.Results = []*StructuralResult{{Kind: "await"}}
	require.ErrorContains(t, doc.Validate(), `"await" is not valid`)
}

func TestSchemaConformance_JSONSchemaValidator(t *testing.T) {
	schema := compileUWSSchema(t)

	sampleData, err := os.ReadFile("../testdata/sample.uws.json")
	require.NoError(t, err)
	sample := decodeJSONValue(t, sampleData)
	require.NoError(t, schema.Validate(sample))

	extensionOnly := decodeJSONValue(t, []byte(`{
		"uws": "1.0.0",
		"info": {"title": "Extension", "version": "1.0.0"},
		"operations": [
			{
				"operationId": "build_email",
				"x-uws-operation-profile": "udon",
				"x-udon-runtime": {"type": "fnct", "function": "mail_raw"}
			}
		]
	}`))
	require.NoError(t, schema.Validate(extensionOnly))

	extensionWhitespaceProfile := decodeJSONValue(t, []byte(`{
		"uws": "1.0.0",
		"info": {"title": "Extension", "version": "1.0.0"},
		"operations": [
			{
				"operationId": "build_email",
				"x-uws-operation-profile": "   ",
				"x-udon-runtime": {"type": "fnct", "function": "mail_raw"}
			}
		]
	}`))
	require.Error(t, schema.Validate(extensionWhitespaceProfile))

	extensionWithoutProfile := decodeJSONValue(t, []byte(`{
		"uws": "1.0.0",
		"info": {"title": "Extension", "version": "1.0.0"},
		"operations": [
			{"operationId": "build_email", "x-udon-runtime": {"type": "fnct", "function": "mail_raw"}}
		]
	}`))
	require.Error(t, schema.Validate(extensionWithoutProfile))

	operationIDOnly := decodeJSONValue(t, []byte(`{
		"uws": "1.0.0",
		"info": {"title": "Invalid", "version": "1.0.0"},
		"operations": [
			{"operationId": "op"}
		]
	}`))
	require.Error(t, schema.Validate(operationIDOnly))

	legacyServiceType := decodeJSONValue(t, []byte(`{
		"uws": "1.0.0",
		"info": {"title": "Legacy", "version": "1.0.0"},
		"operations": [
			{"operationId": "fetch", "serviceType": "http"}
		]
	}`))
	require.Error(t, schema.Validate(legacyServiceType))

	badRequest := decodeJSONValue(t, []byte(`{
		"uws": "1.0.0",
		"info": {"title": "Bad Request", "version": "1.0.0"},
		"sourceDescriptions": [{"name": "api", "url": "./openapi.yaml", "type": "openapi"}],
		"operations": [
			{"operationId": "fetch", "sourceDescription": "api", "openapiOperationId": "getData", "request": {"limit": 10}}
		]
	}`))
	require.Error(t, schema.Validate(badRequest))

	badRequestType := decodeJSONValue(t, []byte(`{
		"uws": "1.0.0",
		"info": {"title": "Bad Request", "version": "1.0.0"},
		"sourceDescriptions": [{"name": "api", "url": "./openapi.yaml", "type": "openapi"}],
		"operations": [
			{"operationId": "fetch", "sourceDescription": "api", "openapiOperationId": "getData", "request": {"query": "bad"}}
		]
	}`))
	require.Error(t, schema.Validate(badRequestType))
}

func compileUWSSchema(t *testing.T) *jsonschema.Schema {
	t.Helper()
	data, err := os.ReadFile("../uws.json")
	require.NoError(t, err)
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
	require.NoError(t, err)
	compiler := jsonschema.NewCompiler()
	require.NoError(t, compiler.AddResource("uws.json", doc))
	schema, err := compiler.Compile("uws.json")
	require.NoError(t, err)
	return schema
}

func decodeJSONValue(t *testing.T, data []byte) any {
	t.Helper()
	value, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
	require.NoError(t, err)
	return value
}
