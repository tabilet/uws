// Package uws1 provides Go types for parsing and generating Udon Workflow Specification (UWS) 1.x documents.
package uws1

import (
	"encoding/json"
	"fmt"
	"strings"
)

const ExtensionOperationProfile = "x-uws-operation-profile"

// extractExtensions extracts x-* extension fields from a raw JSON map.
// It filters out known fields and returns only extension fields.
// A malformed extension value returns an error naming the offending key
// rather than silently dropping the field.
func extractExtensions(raw map[string]json.RawMessage, knownFields []string) (map[string]any, error) {
	known := make(map[string]bool)
	for _, f := range knownFields {
		known[f] = true
	}

	extensions := make(map[string]any)
	for key, value := range raw {
		if strings.HasPrefix(key, "x-") && !known[key] {
			var v any
			if err := json.Unmarshal(value, &v); err != nil {
				return nil, fmt.Errorf("extension %q: %w", key, err)
			}
			extensions[key] = v
		}
	}

	if len(extensions) == 0 {
		return nil, nil
	}
	return extensions, nil
}

func rejectUnknownFields(raw map[string]json.RawMessage, knownFields []string, object string) error {
	known := make(map[string]bool, len(knownFields))
	for _, field := range knownFields {
		known[field] = true
	}
	for key := range raw {
		if known[key] || strings.HasPrefix(key, "x-") {
			continue
		}
		return fmt.Errorf("%s field %q is not defined by UWS core; use an x-* extension for non-core metadata", object, key)
	}
	return nil
}

func unmarshalCoreWithExtensions(data []byte, object string, knownFields []string, dst any) (map[string]json.RawMessage, map[string]any, error) {
	if err := json.Unmarshal(data, dst); err != nil {
		return nil, nil, fmt.Errorf("unmarshaling %s: %w", object, err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, nil, fmt.Errorf("unmarshaling %s extensions: %w", object, err)
	}
	if err := rejectUnknownFields(raw, knownFields, object); err != nil {
		return nil, nil, err
	}
	extensions, err := extractExtensions(raw, knownFields)
	if err != nil {
		return nil, nil, fmt.Errorf("unmarshaling %s extensions: %w", object, err)
	}
	return raw, extensions, nil
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
