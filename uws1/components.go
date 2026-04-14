package uws1

import (
	"encoding/json"
	"fmt"
)

// Components holds reusable objects scoped to the UWS document.
type Components struct {
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
	c.Extensions = extractExtensions(raw, componentsKnownFields)
	return nil
}

func (c Components) MarshalJSON() ([]byte, error) {
	alias := componentsAlias(c)
	return marshalWithExtensions(&alias, c.Extensions)
}
