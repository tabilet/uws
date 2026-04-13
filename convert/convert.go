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
	transformProviderForHCL(doc.Provider, true)
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

func transformProviderForHCL(provider *uws1.Provider, toHCL bool) {
	if provider == nil || provider.Appendices == nil {
		return
	}
	provider.Appendices = transformValue(provider.Appendices, toHCL).(map[string]any)
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
	transformProviderForHCL(op.Provider, toHCL)
	if op.Request != nil {
		op.Request = transformValue(op.Request, toHCL).(map[string]any)
	}
	if op.Arguments != nil {
		for i, arg := range op.Arguments {
			op.Arguments[i] = transformValue(arg, toHCL)
		}
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
	for _, op := range components.Operations {
		transformOperationForHCL(op, toHCL)
	}
}

// transformDocumentFromHCL transforms a UWS document's dynamic fields back from HCL.
func transformDocumentFromHCL(doc *uws1.Document) {
	if doc.Info != nil {
		doc.Info.Description = unescapeNewlines(doc.Info.Description)
		doc.Info.Summary = unescapeNewlines(doc.Info.Summary)
	}
	transformProviderForHCL(doc.Provider, false)
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
