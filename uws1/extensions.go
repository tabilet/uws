// Package uws1 provides Go types for parsing and generating Udon Workflow Specification (UWS) 1.x documents.
package uws1

import (
	"encoding/json"
	"fmt"
	"strings"
)

// extractExtensions extracts x-* extension fields from a raw JSON map.
// It filters out known fields and returns only extension fields.
func extractExtensions(raw map[string]json.RawMessage, knownFields []string) map[string]any {
	known := make(map[string]bool)
	for _, f := range knownFields {
		known[f] = true
	}

	extensions := make(map[string]any)
	for key, value := range raw {
		if strings.HasPrefix(key, "x-") && !known[key] {
			var v any
			if err := json.Unmarshal(value, &v); err == nil {
				extensions[key] = v
			}
		}
	}

	if len(extensions) == 0 {
		return nil
	}
	return extensions
}

// marshalWithExtensions marshals an object along with its x-* extensions.
func marshalWithExtensions(v any, extensions map[string]any) ([]byte, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}

	if len(extensions) == 0 {
		return data, nil
	}

	var m map[string]json.RawMessage
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}

	for key, value := range extensions {
		if !strings.HasPrefix(key, "x-") {
			return nil, fmt.Errorf("extension key %q must start with x-", key)
		}
		if _, exists := m[key]; exists {
			return nil, fmt.Errorf("extension key %q conflicts with a fixed field", key)
		}
		extData, err := json.Marshal(value)
		if err != nil {
			return nil, err
		}
		m[key] = extData
	}

	return json.Marshal(m)
}
