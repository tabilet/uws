package uws1

import (
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
	_, extensions, err := unmarshalCoreWithExtensions(data, "paramSchema", paramSchemaKnownFields, &alias)
	if err != nil {
		return err
	}
	*p = ParamSchema(alias)
	p.Extensions = extensions
	return nil
}

func (p ParamSchema) MarshalJSON() ([]byte, error) {
	alias := paramSchemaAlias(p)
	return marshalWithExtensions(&alias, p.Extensions)
}

// validate walks the schema recursively. The JSON Schema pass already enforces
// structural shape; this method covers semantic rules it cannot express:
// non-empty property names, resolvable required entries, and non-nil nested
// schemas inside properties / items / allOf / oneOf / anyOf.
func (p *ParamSchema) validate(path string, result *ValidationResult) {
	if p == nil {
		return
	}
	for name, child := range p.Properties {
		childPath := fmt.Sprintf("%s.properties.%s", path, name)
		if name == "" {
			result.addError(path+".properties", "property names must be non-empty")
			continue
		}
		if child == nil {
			result.addError(childPath, "is nil")
			continue
		}
		child.validate(childPath, result)
	}

	seenRequired := make(map[string]bool, len(p.Required))
	for i, name := range p.Required {
		itemPath := fmt.Sprintf("%s.required[%d]", path, i)
		if name == "" {
			result.addError(itemPath, "is required")
			continue
		}
		if seenRequired[name] {
			result.addError(itemPath, fmt.Sprintf("duplicate required entry %q", name))
			continue
		}
		seenRequired[name] = true
		if p.Properties != nil {
			if _, ok := p.Properties[name]; !ok {
				result.addError(itemPath, fmt.Sprintf("references unknown property %q", name))
			}
		}
	}

	if p.Items != nil {
		p.Items.validate(path+".items", result)
	}
	validateParamSchemaList(p.AllOf, path+".allOf", result)
	validateParamSchemaList(p.OneOf, path+".oneOf", result)
	validateParamSchemaList(p.AnyOf, path+".anyOf", result)
}

func validateParamSchemaList(list []*ParamSchema, path string, result *ValidationResult) {
	for i, child := range list {
		childPath := fmt.Sprintf("%s[%d]", path, i)
		if child == nil {
			result.addError(childPath, "is nil")
			continue
		}
		child.validate(childPath, result)
	}
}
