package uws1

import (
	"encoding/json"
	"fmt"
)

// ParamSchema describes the schema of a parameter, payload, or response (recursive).
type ParamSchema struct {
	Type       string                  `json:"type,omitempty" yaml:"type,omitempty" hcl:"type,optional"`
	Format     string                  `json:"format,omitempty" yaml:"format,omitempty" hcl:"format,optional"`
	Ref        string                  `json:"$ref,omitempty" yaml:"$ref,omitempty" hcl:"_ref,optional"`
	Properties map[string]*ParamSchema `json:"properties,omitempty" yaml:"properties,omitempty" hcl:"properties,optional"`
	Required   []string                `json:"required,omitempty" yaml:"required,omitempty" hcl:"required,optional"`
	Items      *ParamSchema            `json:"items,omitempty" yaml:"items,omitempty" hcl:"items,block"`
	AllOf      []*ParamSchema          `json:"allOf,omitempty" yaml:"allOf,omitempty" hcl:"allOf,block"`
	OneOf      []*ParamSchema          `json:"oneOf,omitempty" yaml:"oneOf,omitempty" hcl:"oneOf,block"`
	AnyOf      []*ParamSchema          `json:"anyOf,omitempty" yaml:"anyOf,omitempty" hcl:"anyOf,block"`
	Extensions map[string]any          `json:"-" yaml:"-" hcl:"-"`
}

type paramSchemaAlias ParamSchema

var paramSchemaKnownFields = []string{
	"type", "format", "$ref", "properties", "required",
	"items", "allOf", "oneOf", "anyOf",
}

func (p *ParamSchema) UnmarshalJSON(data []byte) error {
	var alias paramSchemaAlias
	if err := json.Unmarshal(data, &alias); err != nil {
		return fmt.Errorf("unmarshaling paramSchema: %w", err)
	}
	*p = ParamSchema(alias)

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("unmarshaling paramSchema extensions: %w", err)
	}
	if err := rejectUnknownFields(raw, paramSchemaKnownFields, "paramSchema"); err != nil {
		return err
	}
	p.Extensions = extractExtensions(raw, paramSchemaKnownFields)
	return nil
}

func (p ParamSchema) MarshalJSON() ([]byte, error) {
	alias := paramSchemaAlias(p)
	return marshalWithExtensions(&alias, p.Extensions)
}
