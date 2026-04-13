package uws1

import (
	"encoding/json"
	"fmt"
)

// Components holds reusable objects scoped to the UWS document.
type Components struct {
	Operations      map[string]*Operation      `json:"operations,omitempty" yaml:"operations,omitempty" hcl:"operations,optional"`
	SecuritySchemes map[string]*SecurityScheme `json:"securitySchemes,omitempty" yaml:"securitySchemes,omitempty" hcl:"securitySchemes,optional"`
	Variables       map[string]any             `json:"variables,omitempty" yaml:"variables,omitempty" hcl:"variables,optional"`
	Extensions      map[string]any             `json:"-" yaml:"-" hcl:"-"`
}

type componentsAlias Components

var componentsKnownFields = []string{
	"operations", "securitySchemes", "variables",
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
	c.Extensions = extractExtensions(raw, componentsKnownFields)
	return nil
}

func (c Components) MarshalJSON() ([]byte, error) {
	alias := componentsAlias(c)
	return marshalWithExtensions(&alias, c.Extensions)
}
