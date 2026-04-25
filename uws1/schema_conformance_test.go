package uws1

import (
	"bytes"
	"os"
	"sort"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// schemaDefCoverage declares, per $def, which structural rules the Go
// validator is expected to enforce (beyond what the JSON Schema pass does).
// TestSchemaConformance_DrivenRulesAreDeclared reads uws.json, extracts every
// required/enum/pattern rule per $def, and fails if the schema's actual rule
// set differs from this table in either direction:
//
//   - A rule in the schema but not in the table means someone added a new
//     schema rule without declaring where the Go validator should cover it.
//   - A rule in the table but not in the schema means the table is stale.
//
// The `covered` entries are not executed directly by this test; they are
// hand-maintained claims pointing at the validator function or identifier
// registry that enforces the rule. The adjacent
// TestSchemaConformance_ValidatorMatchesSelectedRules test executes a
// representative slice of them to confirm the claims hold.
type schemaDefRules struct {
	required []string
	// enumProps lists property names that carry an `enum` or `const` in the
	// schema. The validator enforces these via shared helpers or package-level
	// tables such as IsWorkflowType / validFailureActionTypes.
	enumProps []string
	// patternProps lists property names that carry a regex `pattern`.
	patternProps []string
	// propertyNamePatternProps lists object-valued properties whose key names
	// are constrained by a propertyNames.pattern rule.
	propertyNamePatternProps []string
}

func schemaDefCoverage() map[string]schemaDefRules {
	return map[string]schemaDefRules{
		"": { // document root
			required:     []string{"uws", "info", "operations"},
			patternProps: []string{"uws"},
		},
		"info": {
			required: []string{"title", "version"},
		},
		"source-description-object": {
			required:     []string{"name", "url"},
			enumProps:    []string{"type"},
			patternProps: []string{"name"},
		},
		"operation-object": {
			required:     []string{"operationId"},
			patternProps: []string{"openapiOperationRef", "x-uws-operation-profile"},
		},
		"request-binding-object": {},
		"workflow-object": {
			required:     []string{"workflowId", "type"},
			enumProps:    []string{"type"},
			patternProps: []string{"workflowId"},
		},
		"step-object": {
			required:     []string{"stepId"},
			enumProps:    []string{"type"},
			patternProps: []string{"stepId"},
		},
		"case-object": {
			required: []string{"name"},
		},
		"trigger-object": {
			required:     []string{"triggerId"},
			patternProps: []string{"outputs"}, // enforced on array items
		},
		"trigger-route-object": {
			required: []string{"output", "to"},
		},
		"param-schema-object": {},
		"structural-result-object": {
			required:     []string{"name", "kind", "from"},
			enumProps:    []string{"kind"},
			patternProps: []string{"name", "from"},
		},
		"components-object": {
			propertyNamePatternProps: []string{"variables"},
		},
		"criterion-object": {
			required:  []string{"condition"},
			enumProps: []string{"type"},
		},
		"failure-action-object": {
			required:     []string{"name", "type"},
			enumProps:    []string{"type"},
			patternProps: []string{"workflowId", "stepId"},
		},
		"success-action-object": {
			required:     []string{"name", "type"},
			enumProps:    []string{"type"},
			patternProps: []string{"workflowId", "stepId"},
		},
	}
}

// TestSchemaConformance_DrivenRulesAreDeclared is the tripwire that fires when
// a rule is added to or removed from uws.json without a matching update to
// schemaDefCoverage. That mismatch is exactly the drift S1 is meant to kill.
func TestSchemaConformance_DrivenRulesAreDeclared(t *testing.T) {
	schema := loadSchemaDoc(t)
	coverage := schemaDefCoverage()

	actual := collectSchemaRules(t, schema)
	declared := make(map[string]schemaDefRules, len(coverage))
	for name, rules := range coverage {
		declared[name] = rules
	}

	var unknown []string
	for name := range actual {
		if _, ok := declared[name]; !ok {
			unknown = append(unknown, name)
		}
	}
	sort.Strings(unknown)
	assert.Empty(t, unknown,
		"schema declares $defs with no coverage entry in schemaDefCoverage: %v", unknown)

	for _, name := range sortedKeys(declared) {
		t.Run(defLabel(name), func(t *testing.T) {
			want := declared[name]
			got, ok := actual[name]
			if !ok {
				t.Fatalf("coverage table lists %q but the schema has no such $def", name)
			}
			assert.ElementsMatch(t, want.required, got.required,
				"$def %q required mismatch", name)
			assert.ElementsMatch(t, want.enumProps, got.enumProps,
				"$def %q enum/const properties mismatch", name)
			assert.ElementsMatch(t, want.patternProps, got.patternProps,
				"$def %q pattern properties mismatch", name)
			assert.ElementsMatch(t, want.propertyNamePatternProps, got.propertyNamePatternProps,
				"$def %q propertyNames pattern mismatch", name)
		})
	}
}

// TestSchemaConformance_ValidatorMatchesSelectedRules exercises a handful of
// rules from the coverage table against (*Document).Validate, making sure the
// claims in schemaDefCoverage actually hold on the Go side. Adding a new
// enum or required-field should slot a case in here; the driven test above
// guarantees you cannot forget to *declare* the rule.
func TestSchemaConformance_ValidatorMatchesSelectedRules(t *testing.T) {
	// workflow-object: enum on `type`
	doc := validDocument()
	doc.Workflows = []*Workflow{
		{WorkflowID: "wf", Type: "not-a-workflow", Steps: []*Step{{StepID: "s"}}},
	}
	require.ErrorContains(t, doc.Validate(), "not-a-workflow")
	require.NotContains(t, doc.Validate().Error(), "steps[0].type is required")

	// trigger-route-object: required `output`
	doc = validDocument()
	doc.Triggers = []*Trigger{{TriggerID: "t", Outputs: []string{"primary"}, Routes: []*TriggerRoute{{}}}}
	require.ErrorContains(t, doc.Validate(), "routes[0].output is required")

	// structural-result-object: enum on `kind`
	doc = validDocument()
	doc.Results = []*StructuralResult{{Kind: "await"}}
	require.ErrorContains(t, doc.Validate(), `"await" is not valid`)

	// failure-action-object: enum on `type` (rejects non-listed action types)
	doc = validDocument()
	doc.Operations[0].OnFailure = []*FailureAction{{Name: "a", Type: "bogus"}}
	require.ErrorContains(t, doc.Validate(), "bogus")

	// success-action-object: enum on `type`
	doc = validDocument()
	doc.Operations[0].OnSuccess = []*SuccessAction{{Name: "a", Type: "retry"}}
	require.ErrorContains(t, doc.Validate(), "retry")

	// criterion-object: enum on `type`
	doc = validDocument()
	doc.Operations[0].SuccessCriteria = []*Criterion{{Condition: "$status == 200", Type: "unsupported"}}
	require.ErrorContains(t, doc.Validate(), "unsupported")

	// source-description-object: enum on `type`
	doc = validDocument()
	doc.SourceDescriptions[0].Type = "swagger"
	require.ErrorContains(t, doc.Validate(), "swagger")
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

	dottedWorkflowID := decodeJSONValue(t, []byte(`{
		"uws": "1.0.0",
		"info": {"title": "Bad Workflow", "version": "1.0.0"},
		"sourceDescriptions": [{"name": "api", "url": "./openapi.yaml", "type": "openapi"}],
		"operations": [
			{"operationId": "fetch", "sourceDescription": "api", "openapiOperationId": "getData"}
		],
		"workflows": [{"workflowId": "daily.v1", "type": "sequence"}]
	}`))
	require.Error(t, schema.Validate(dottedWorkflowID))

	dottedStepID := decodeJSONValue(t, []byte(`{
		"uws": "1.0.0",
		"info": {"title": "Bad Step", "version": "1.0.0"},
		"sourceDescriptions": [{"name": "api", "url": "./openapi.yaml", "type": "openapi"}],
		"operations": [
			{"operationId": "fetch", "sourceDescription": "api", "openapiOperationId": "getData"}
		],
		"workflows": [{"workflowId": "main", "type": "sequence", "steps": [{"stepId": "fetch.user", "operationRef": "fetch"}]}]
	}`))
	require.Error(t, schema.Validate(dottedStepID))

	badComponentVariableKey := decodeJSONValue(t, []byte(`{
		"uws": "1.0.0",
		"info": {"title": "Bad Components", "version": "1.0.0"},
		"operations": [{"operationId": "build_email", "x-uws-operation-profile": "udon"}],
		"components": {"variables": {"bad name": true}}
	}`))
	require.Error(t, schema.Validate(badComponentVariableKey))

	operationWorkflow := decodeJSONValue(t, []byte(`{
		"uws": "1.0.0",
		"info": {"title": "Bad Operation", "version": "1.0.0"},
		"sourceDescriptions": [{"name": "api", "url": "./openapi.yaml", "type": "openapi"}],
		"operations": [
			{"operationId": "fetch", "sourceDescription": "api", "openapiOperationId": "getData", "workflow": "child"}
		]
	}`))
	require.Error(t, schema.Validate(operationWorkflow))

	paramSchemaRef := decodeJSONValue(t, []byte(`{
		"uws": "1.0.0",
		"info": {"title": "With Ref", "version": "1.0.0"},
		"operations": [{"operationId": "build_email", "x-uws-operation-profile": "udon"}],
		"workflows": [
			{"workflowId": "main", "type": "sequence", "inputs": {"$ref": "#/$defs/shared"}}
		]
	}`))
	require.NoError(t, schema.Validate(paramSchemaRef))
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

// collectSchemaRules walks uws.json and extracts, per $def plus the root,
// every `required`, enum-bearing, pattern-bearing, and propertyNames-pattern
// property. Meta defs (specification-extensions, structural-type-constraints)
// are skipped.
func collectSchemaRules(t *testing.T, schema map[string]any) map[string]schemaDefRules {
	t.Helper()
	rules := map[string]schemaDefRules{
		"": extractRules(schema),
	}
	defs, _ := schema["$defs"].(map[string]any)
	for name, raw := range defs {
		switch name {
		case "specification-extensions", "structural-type-constraints":
			continue
		}
		obj, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		rules[name] = extractRules(obj)
	}
	return rules
}

func extractRules(obj map[string]any) schemaDefRules {
	var out schemaDefRules
	if req, ok := obj["required"].([]any); ok {
		for _, r := range req {
			if s, ok := r.(string); ok {
				out.required = append(out.required, s)
			}
		}
	}
	props, _ := obj["properties"].(map[string]any)
	for name, raw := range props {
		p, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if _, ok := p["enum"]; ok {
			out.enumProps = append(out.enumProps, name)
		} else if _, ok := p["const"]; ok {
			out.enumProps = append(out.enumProps, name)
		}
		if hasPattern(p) {
			out.patternProps = append(out.patternProps, name)
		}
		if hasPropertyNamePattern(p) {
			out.propertyNamePatternProps = append(out.propertyNamePatternProps, name)
		}
	}
	return out
}

// hasPattern reports whether a property schema carries a pattern at its top
// level or on its `items` sub-schema (the trigger-object `outputs` array
// patterns each string, for example).
func hasPattern(p map[string]any) bool {
	if _, ok := p["pattern"]; ok {
		return true
	}
	if items, ok := p["items"].(map[string]any); ok {
		if _, ok := items["pattern"]; ok {
			return true
		}
	}
	return false
}

func hasPropertyNamePattern(p map[string]any) bool {
	propertyNames, ok := p["propertyNames"].(map[string]any)
	if !ok {
		return false
	}
	_, ok = propertyNames["pattern"]
	return ok
}

func sortedKeys(m map[string]schemaDefRules) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func defLabel(name string) string {
	if name == "" {
		return "document-root"
	}
	return name
}
