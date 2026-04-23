package uws1

import (
	"encoding/json"
	"fmt"
)

// Components holds reusable objects scoped to the UWS document.
type Components struct {
	// Variables is an intentionally open-shape map; any JSON-compatible value is
	// allowed. Keys must match componentNamePattern (enforced by Validate);
	// values are not inspected.
	Variables  map[string]any `json:"variables,omitempty" yaml:"variables,omitempty" hcl:"variables,optional"`
	Extensions map[string]any `json:"-" yaml:"-" hcl:"-"`
}

type componentsAlias Components

var componentsKnownFields = []string{
	"variables",
}

func (c *Components) UnmarshalJSON(data []byte) error {
	var alias componentsAlias
	if err := json.Unmarshal(data, &alias); err != nil {
		return fmt.Errorf("unmarshaling components: %w", err)
	}
	*c = Components(alias)

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("unmarshaling components extensions: %w", err)
	}
	if err := rejectUnknownFields(raw, componentsKnownFields, "components"); err != nil {
		return err
	}
	extensions, err := extractExtensions(raw, componentsKnownFields)
	if err != nil {
		return fmt.Errorf("unmarshaling components extensions: %w", err)
	}
	c.Extensions = extensions
	return nil
}

func (c Components) MarshalJSON() ([]byte, error) {
	alias := componentsAlias(c)
	return marshalWithExtensions(&alias, c.Extensions)
}
