// Package convert provides functions to convert UWS documents between JSON and HCL formats.
package convert

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/genelet/horizon/dethcl"
	"github.com/tabilet/uws/uws1"
	"gopkg.in/yaml.v3"
)

const hclDollarKeyPrefix = "__dollar__"

var legacyDollarKeys = map[string]struct{}{
	"$ref":           {},
	"$id":            {},
	"$schema":        {},
	"$defs":          {},
	"$comment":       {},
	"$vocabulary":    {},
	"$anchor":        {},
	"$dynamicRef":    {},
	"$dynamicAnchor": {},
}

func toHCLKey(key string) string {
	if !strings.HasPrefix(key, "$") {
		return key
	}
	if _, ok := legacyDollarKeys[key]; ok {
		return "_" + key[1:]
	}
	return hclDollarKeyPrefix + key[1:]
}

func fromHCLKey(key string) string {
	if strings.HasPrefix(key, hclDollarKeyPrefix) {
		return "$" + key[len(hclDollarKeyPrefix):]
	}
	if strings.HasPrefix(key, "_") {
		candidate := "$" + key[1:]
		if _, ok := legacyDollarKeys[candidate]; ok {
			return candidate
		}
	}
	return key
}

// transformValue recursively transforms values for HCL compatibility.
func transformValue(v any, toHCL bool) any {
	switch val := v.(type) {
	case string:
		if toHCL {
			return escapeForHCL(val)
		}
		return unescapeFromHCL(val)
	case map[string]any:
		result := make(map[string]any)
		for k, v := range val {
			newKey := k
			if toHCL {
				newKey = toHCLKey(k)
			} else {
				newKey = fromHCLKey(k)
			}
			result[newKey] = transformValue(v, toHCL)
		}
		return result
	case []any:
		result := make([]any, len(val))
		for i, item := range val {
			result[i] = transformValue(item, toHCL)
		}
		return result
	default:
		return v
	}
}

func escapeForHCL(s string) string {
	s = strings.ReplaceAll(s, "\\n", "\x00ESCAPED_N\x00")
	s = strings.ReplaceAll(s, "\\\"", "\x00ESCAPED_Q\x00")
	s = strings.ReplaceAll(s, "\n", "\\n")
	s = strings.ReplaceAll(s, "\"", "\\\"")
	s = strings.ReplaceAll(s, "\x00ESCAPED_N\x00", "\\\\n")
	s = strings.ReplaceAll(s, "\x00ESCAPED_Q\x00", "\\\\\"")
	return s
}

func escapeNewlines(s string) string {
	s = strings.ReplaceAll(s, "\\n", "\x00ESCAPED_N\x00")
	s = strings.ReplaceAll(s, "\n", "\\n")
	s = strings.ReplaceAll(s, "\x00ESCAPED_N\x00", "\\\\n")
	return s
}

func unescapeNewlines(s string) string {
	s = strings.ReplaceAll(s, "\\\\n", "\x00ESCAPED_N\x00")
	s = strings.ReplaceAll(s, "\\n", "\n")
	s = strings.ReplaceAll(s, "\x00ESCAPED_N\x00", "\\n")
	return s
}

func unescapeFromHCL(s string) string {
	s = strings.ReplaceAll(s, "\\\\n", "\x00ESCAPED_N\x00")
	s = strings.ReplaceAll(s, "\\\\\"", "\x00ESCAPED_Q\x00")
	s = strings.ReplaceAll(s, "\\n", "\n")
	s = strings.ReplaceAll(s, "\\\"", "\"")
	s = strings.ReplaceAll(s, "\x00ESCAPED_N\x00", "\\n")
	s = strings.ReplaceAll(s, "\x00ESCAPED_Q\x00", "\\\"")
	return s
}

// transformDocumentForHCL transforms a UWS document's dynamic fields for HCL compatibility.
func transformDocumentForHCL(doc *uws1.Document) {
	if doc.Info != nil {
		doc.Info.Description = escapeNewlines(doc.Info.Description)
		doc.Info.Summary = escapeNewlines(doc.Info.Summary)
	}
	if doc.Variables != nil {
		doc.Variables = transformValue(doc.Variables, true).(map[string]any)
	}
	for _, op := range doc.Operations {
		transformOperationForHCL(op, true)
	}
	for _, wf := range doc.Workflows {
		transformWorkflowForHCL(wf, true)
	}
	for _, t := range doc.Triggers {
		transformTriggerForHCL(t, true)
	}
	transformComponentsForHCL(doc.Components, true)
}

func transformOperationForHCL(op *uws1.Operation, toHCL bool) {
	if op == nil {
		return
	}
	if toHCL {
		op.Description = escapeNewlines(op.Description)
	} else {
		op.Description = unescapeNewlines(op.Description)
	}
	if op.Request != nil {
		op.Request = transformValue(op.Request, toHCL).(map[string]any)
	}
}

func transformWorkflowForHCL(wf *uws1.Workflow, toHCL bool) {
	if wf == nil {
		return
	}
	if toHCL {
		wf.Description = escapeNewlines(wf.Description)
	} else {
		wf.Description = unescapeNewlines(wf.Description)
	}
	transformStepsForHCL(wf.Steps, toHCL)
	transformCasesForHCL(wf.Cases, toHCL)
	transformStepsForHCL(wf.Default, toHCL)
}

func transformStepsForHCL(steps []*uws1.Step, toHCL bool) {
	for _, step := range steps {
		transformStepForHCL(step, toHCL)
	}
}

func transformStepForHCL(step *uws1.Step, toHCL bool) {
	if step == nil {
		return
	}
	if toHCL {
		step.Description = escapeNewlines(step.Description)
	} else {
		step.Description = unescapeNewlines(step.Description)
	}
	if step.Body != nil {
		step.Body = transformValue(step.Body, toHCL).(map[string]any)
	}
	transformStepsForHCL(step.Steps, toHCL)
	transformCasesForHCL(step.Cases, toHCL)
	transformStepsForHCL(step.Default, toHCL)
}

func transformCasesForHCL(cases []*uws1.Case, toHCL bool) {
	for _, c := range cases {
		transformCaseForHCL(c, toHCL)
	}
}

func transformCaseForHCL(c *uws1.Case, toHCL bool) {
	if c == nil {
		return
	}
	if c.Body != nil {
		c.Body = transformValue(c.Body, toHCL).(map[string]any)
	}
	transformStepsForHCL(c.Steps, toHCL)
}

func transformTriggerForHCL(trigger *uws1.Trigger, toHCL bool) {
	if trigger == nil || trigger.Options == nil {
		return
	}
	trigger.Options = transformValue(trigger.Options, toHCL).(map[string]any)
}

func transformComponentsForHCL(components *uws1.Components, toHCL bool) {
	if components == nil {
		return
	}
	if components.Variables != nil {
		components.Variables = transformValue(components.Variables, toHCL).(map[string]any)
	}
}

// transformDocumentFromHCL transforms a UWS document's dynamic fields back from HCL.
func transformDocumentFromHCL(doc *uws1.Document) {
	if doc.Info != nil {
		doc.Info.Description = unescapeNewlines(doc.Info.Description)
		doc.Info.Summary = unescapeNewlines(doc.Info.Summary)
	}
	if doc.Variables != nil {
		doc.Variables = transformValue(doc.Variables, false).(map[string]any)
	}
	for _, op := range doc.Operations {
		transformOperationForHCL(op, false)
	}
	for _, wf := range doc.Workflows {
		transformWorkflowForHCL(wf, false)
	}
	for _, t := range doc.Triggers {
		transformTriggerForHCL(t, false)
	}
	transformComponentsForHCL(doc.Components, false)
}

func cloneDocument(doc *uws1.Document) (*uws1.Document, error) {
	if doc == nil {
		return nil, fmt.Errorf("document is nil")
	}
	data, err := json.Marshal(doc)
	if err != nil {
		return nil, err
	}
	var cloned uws1.Document
	if err := json.Unmarshal(data, &cloned); err != nil {
		return nil, err
	}
	return &cloned, nil
}

func validateHCLSerializable(doc *uws1.Document) error {
	if doc == nil {
		return fmt.Errorf("document is nil")
	}
	if err := rejectExtensionsForHCL("document", doc.Extensions); err != nil {
		return err
	}
	if doc.Info != nil {
		if err := rejectExtensionsForHCL("info", doc.Info.Extensions); err != nil {
			return err
		}
	}
	for i, source := range doc.SourceDescriptions {
		if source == nil {
			continue
		}
		if err := rejectExtensionsForHCL(fmt.Sprintf("sourceDescriptions[%d]", i), source.Extensions); err != nil {
			return err
		}
	}
	for i, op := range doc.Operations {
		if err := validateOperationHCLSerializable(fmt.Sprintf("operations[%d]", i), op); err != nil {
			return err
		}
	}
	for i, wf := range doc.Workflows {
		if err := validateWorkflowHCLSerializable(fmt.Sprintf("workflows[%d]", i), wf); err != nil {
			return err
		}
	}
	for i, trigger := range doc.Triggers {
		if err := validateTriggerHCLSerializable(fmt.Sprintf("triggers[%d]", i), trigger); err != nil {
			return err
		}
	}
	for i, result := range doc.Results {
		if result == nil {
			continue
		}
		if err := rejectExtensionsForHCL(fmt.Sprintf("results[%d]", i), result.Extensions); err != nil {
			return err
		}
	}
	if doc.Components != nil {
		if err := rejectExtensionsForHCL("components", doc.Components.Extensions); err != nil {
			return err
		}
	}
	return nil
}

func validateOperationHCLSerializable(path string, op *uws1.Operation) error {
	if op == nil {
		return nil
	}
	if err := rejectExtensionsForHCL(path, op.Extensions); err != nil {
		return err
	}
	for i, criterion := range op.SuccessCriteria {
		if err := validateCriterionHCLSerializable(fmt.Sprintf("%s.successCriteria[%d]", path, i), criterion); err != nil {
			return err
		}
	}
	for i, action := range op.OnFailure {
		if err := validateFailureActionHCLSerializable(fmt.Sprintf("%s.onFailure[%d]", path, i), action); err != nil {
			return err
		}
	}
	for i, action := range op.OnSuccess {
		if err := validateSuccessActionHCLSerializable(fmt.Sprintf("%s.onSuccess[%d]", path, i), action); err != nil {
			return err
		}
	}
	return nil
}

func validateCriterionHCLSerializable(path string, criterion *uws1.Criterion) error {
	if criterion == nil {
		return nil
	}
	return rejectExtensionsForHCL(path, criterion.Extensions)
}

func validateFailureActionHCLSerializable(path string, action *uws1.FailureAction) error {
	if action == nil {
		return nil
	}
	if err := rejectExtensionsForHCL(path, action.Extensions); err != nil {
		return err
	}
	for i, criterion := range action.Criteria {
		if err := validateCriterionHCLSerializable(fmt.Sprintf("%s.criteria[%d]", path, i), criterion); err != nil {
			return err
		}
	}
	return nil
}

func validateSuccessActionHCLSerializable(path string, action *uws1.SuccessAction) error {
	if action == nil {
		return nil
	}
	if err := rejectExtensionsForHCL(path, action.Extensions); err != nil {
		return err
	}
	for i, criterion := range action.Criteria {
		if err := validateCriterionHCLSerializable(fmt.Sprintf("%s.criteria[%d]", path, i), criterion); err != nil {
			return err
		}
	}
	return nil
}

func validateWorkflowHCLSerializable(path string, wf *uws1.Workflow) error {
	if wf == nil {
		return nil
	}
	if err := rejectExtensionsForHCL(path, wf.Extensions); err != nil {
		return err
	}
	if err := validateParamSchemaHCLSerializable(path+".inputs", wf.Inputs); err != nil {
		return err
	}
	for i, step := range wf.Steps {
		if err := validateStepHCLSerializable(fmt.Sprintf("%s.steps[%d]", path, i), step); err != nil {
			return err
		}
	}
	for i, c := range wf.Cases {
		if err := validateCaseHCLSerializable(fmt.Sprintf("%s.cases[%d]", path, i), c); err != nil {
			return err
		}
	}
	for i, step := range wf.Default {
		if err := validateStepHCLSerializable(fmt.Sprintf("%s.default[%d]", path, i), step); err != nil {
			return err
		}
	}
	return nil
}

func validateStepHCLSerializable(path string, step *uws1.Step) error {
	if step == nil {
		return nil
	}
	if err := rejectExtensionsForHCL(path, step.Extensions); err != nil {
		return err
	}
	for i, child := range step.Steps {
		if err := validateStepHCLSerializable(fmt.Sprintf("%s.steps[%d]", path, i), child); err != nil {
			return err
		}
	}
	for i, c := range step.Cases {
		if err := validateCaseHCLSerializable(fmt.Sprintf("%s.cases[%d]", path, i), c); err != nil {
			return err
		}
	}
	for i, child := range step.Default {
		if err := validateStepHCLSerializable(fmt.Sprintf("%s.default[%d]", path, i), child); err != nil {
			return err
		}
	}
	return nil
}

func validateCaseHCLSerializable(path string, c *uws1.Case) error {
	if c == nil {
		return nil
	}
	if err := rejectExtensionsForHCL(path, c.Extensions); err != nil {
		return err
	}
	for i, step := range c.Steps {
		if err := validateStepHCLSerializable(fmt.Sprintf("%s.steps[%d]", path, i), step); err != nil {
			return err
		}
	}
	return nil
}

func validateTriggerHCLSerializable(path string, trigger *uws1.Trigger) error {
	if trigger == nil {
		return nil
	}
	if err := rejectExtensionsForHCL(path, trigger.Extensions); err != nil {
		return err
	}
	for i, route := range trigger.Routes {
		if route == nil {
			continue
		}
		if err := rejectExtensionsForHCL(fmt.Sprintf("%s.routes[%d]", path, i), route.Extensions); err != nil {
			return err
		}
	}
	return nil
}

func validateParamSchemaHCLSerializable(path string, schema *uws1.ParamSchema) error {
	if schema == nil {
		return nil
	}
	if err := rejectExtensionsForHCL(path, schema.Extensions); err != nil {
		return err
	}
	for name, child := range schema.Properties {
		if err := validateParamSchemaHCLSerializable(path+".properties."+name, child); err != nil {
			return err
		}
	}
	if err := validateParamSchemaHCLSerializable(path+".items", schema.Items); err != nil {
		return err
	}
	for i, child := range schema.AllOf {
		if err := validateParamSchemaHCLSerializable(fmt.Sprintf("%s.allOf[%d]", path, i), child); err != nil {
			return err
		}
	}
	for i, child := range schema.OneOf {
		if err := validateParamSchemaHCLSerializable(fmt.Sprintf("%s.oneOf[%d]", path, i), child); err != nil {
			return err
		}
	}
	for i, child := range schema.AnyOf {
		if err := validateParamSchemaHCLSerializable(fmt.Sprintf("%s.anyOf[%d]", path, i), child); err != nil {
			return err
		}
	}
	return nil
}

func rejectExtensionsForHCL(path string, extensions map[string]any) error {
	if len(extensions) == 0 {
		return nil
	}
	return fmt.Errorf("%s contains x-* extensions; UWS HCL conversion is core-only and cannot preserve extension profiles, use JSON or YAML", path)
}

func toJSONCompatible(v any) any {
	switch val := v.(type) {
	case map[string]any:
		result := make(map[string]any, len(val))
		for k, item := range val {
			result[k] = toJSONCompatible(item)
		}
		return result
	case map[any]any:
		result := make(map[string]any, len(val))
		for k, item := range val {
			result[fmt.Sprint(k)] = toJSONCompatible(item)
		}
		return result
	case []any:
		result := make([]any, len(val))
		for i, item := range val {
			result[i] = toJSONCompatible(item)
		}
		return result
	default:
		return v
	}
}

// JSONToHCL converts a UWS document from JSON format to HCL format.
func JSONToHCL(jsonData []byte) ([]byte, error) {
	var doc uws1.Document
	if err := json.Unmarshal(jsonData, &doc); err != nil {
		return nil, err
	}
	if err := validateHCLSerializable(&doc); err != nil {
		return nil, err
	}
	transformDocumentForHCL(&doc)
	return dethcl.Marshal(&doc)
}

// JSONToYAML converts a UWS document from JSON format to YAML format.
func JSONToYAML(jsonData []byte) ([]byte, error) {
	var v any
	if err := json.Unmarshal(jsonData, &v); err != nil {
		return nil, err
	}
	return yaml.Marshal(toJSONCompatible(v))
}

// YAMLToJSON converts a UWS document from YAML format to JSON format.
func YAMLToJSON(yamlData []byte) ([]byte, error) {
	var v any
	if err := yaml.Unmarshal(yamlData, &v); err != nil {
		return nil, err
	}
	return json.Marshal(toJSONCompatible(v))
}

// YAMLToJSONIndent converts a UWS document from YAML format to indented JSON format.
func YAMLToJSONIndent(yamlData []byte, prefix, indent string) ([]byte, error) {
	var v any
	if err := yaml.Unmarshal(yamlData, &v); err != nil {
		return nil, err
	}
	return json.MarshalIndent(toJSONCompatible(v), prefix, indent)
}

// YAMLToHCL converts a UWS document from YAML format to HCL format.
func YAMLToHCL(yamlData []byte) ([]byte, error) {
	jsonData, err := YAMLToJSON(yamlData)
	if err != nil {
		return nil, err
	}
	return JSONToHCL(jsonData)
}

// HCLToJSON converts a UWS document from HCL format to JSON format.
func HCLToJSON(hclData []byte) ([]byte, error) {
	var doc uws1.Document
	if err := dethcl.Unmarshal(hclData, &doc); err != nil {
		return nil, err
	}
	transformDocumentFromHCL(&doc)
	return json.Marshal(&doc)
}

// HCLToJSONIndent converts a UWS document from HCL format to indented JSON format.
func HCLToJSONIndent(hclData []byte, prefix, indent string) ([]byte, error) {
	var doc uws1.Document
	if err := dethcl.Unmarshal(hclData, &doc); err != nil {
		return nil, err
	}
	transformDocumentFromHCL(&doc)
	return json.MarshalIndent(&doc, prefix, indent)
}

// HCLToYAML converts a UWS document from HCL format to YAML format.
func HCLToYAML(hclData []byte) ([]byte, error) {
	jsonData, err := HCLToJSON(hclData)
	if err != nil {
		return nil, err
	}
	return JSONToYAML(jsonData)
}

// MarshalHCL marshals a UWS document to HCL format.
func MarshalHCL(doc *uws1.Document) ([]byte, error) {
	cloned, err := cloneDocument(doc)
	if err != nil {
		return nil, err
	}
	if err := validateHCLSerializable(cloned); err != nil {
		return nil, err
	}
	transformDocumentForHCL(cloned)
	return dethcl.Marshal(cloned)
}

// UnmarshalHCL unmarshals HCL data into a UWS document.
func UnmarshalHCL(hclData []byte, doc *uws1.Document) error {
	if err := dethcl.Unmarshal(hclData, doc); err != nil {
		return err
	}
	transformDocumentFromHCL(doc)
	return nil
}

// MarshalJSON marshals a UWS document to JSON format.
func MarshalJSON(doc *uws1.Document) ([]byte, error) {
	return json.Marshal(doc)
}

// MarshalJSONIndent marshals a UWS document to indented JSON format.
func MarshalJSONIndent(doc *uws1.Document, prefix, indent string) ([]byte, error) {
	return json.MarshalIndent(doc, prefix, indent)
}

// UnmarshalJSON unmarshals JSON data into a UWS document.
func UnmarshalJSON(jsonData []byte, doc *uws1.Document) error {
	return json.Unmarshal(jsonData, doc)
}

// MarshalYAML marshals a UWS document to YAML format.
func MarshalYAML(doc *uws1.Document) ([]byte, error) {
	jsonData, err := json.Marshal(doc)
	if err != nil {
		return nil, err
	}
	return JSONToYAML(jsonData)
}

// UnmarshalYAML unmarshals YAML data into a UWS document.
func UnmarshalYAML(yamlData []byte, doc *uws1.Document) error {
	jsonData, err := YAMLToJSON(yamlData)
	if err != nil {
		return err
	}
	return json.Unmarshal(jsonData, doc)
}
