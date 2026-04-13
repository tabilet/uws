// Package convert provides functions to convert UWS documents between JSON and HCL formats.
package convert

import (
	"encoding/json"
	"strings"

	"github.com/genelet/horizon/dethcl"
	"github.com/tabilet/uws/uws1"
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
		op.Description = escapeNewlines(op.Description)
		if op.Request != nil {
			op.Request = transformValue(op.Request, true).(map[string]any)
		}
		if op.Arguments != nil {
			for i, arg := range op.Arguments {
				op.Arguments[i] = transformValue(arg, true)
			}
		}
	}
	for _, wf := range doc.Workflows {
		wf.Description = escapeNewlines(wf.Description)
		for _, step := range wf.Steps {
			step.Description = escapeNewlines(step.Description)
			if step.Body != nil {
				step.Body = transformValue(step.Body, true).(map[string]any)
			}
		}
		for _, c := range wf.Cases {
			if c.Body != nil {
				c.Body = transformValue(c.Body, true).(map[string]any)
			}
		}
		for _, step := range wf.Default {
			if step.Body != nil {
				step.Body = transformValue(step.Body, true).(map[string]any)
			}
		}
	}
	for _, t := range doc.Triggers {
		if t.Options != nil {
			t.Options = transformValue(t.Options, true).(map[string]any)
		}
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
		op.Description = unescapeNewlines(op.Description)
		if op.Request != nil {
			op.Request = transformValue(op.Request, false).(map[string]any)
		}
		if op.Arguments != nil {
			for i, arg := range op.Arguments {
				op.Arguments[i] = transformValue(arg, false)
			}
		}
	}
	for _, wf := range doc.Workflows {
		wf.Description = unescapeNewlines(wf.Description)
		for _, step := range wf.Steps {
			step.Description = unescapeNewlines(step.Description)
			if step.Body != nil {
				step.Body = transformValue(step.Body, false).(map[string]any)
			}
		}
		for _, c := range wf.Cases {
			if c.Body != nil {
				c.Body = transformValue(c.Body, false).(map[string]any)
			}
		}
		for _, step := range wf.Default {
			if step.Body != nil {
				step.Body = transformValue(step.Body, false).(map[string]any)
			}
		}
	}
	for _, t := range doc.Triggers {
		if t.Options != nil {
			t.Options = transformValue(t.Options, false).(map[string]any)
		}
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

// MarshalHCL marshals a UWS document to HCL format.
// Note: This function modifies the document in place.
func MarshalHCL(doc *uws1.Document) ([]byte, error) {
	transformDocumentForHCL(doc)
	return dethcl.Marshal(doc)
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
