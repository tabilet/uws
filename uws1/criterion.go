package uws1

import (
	"encoding/json"
	"fmt"
)

// CriterionExpressionType defines how a criterion condition is evaluated.
type CriterionExpressionType string

const (
	CriterionSimple   CriterionExpressionType = "simple"
	CriterionRegex    CriterionExpressionType = "regex"
	CriterionJSONPath CriterionExpressionType = "jsonpath"
	CriterionXPath    CriterionExpressionType = "xpath"
)

// Criterion describes a success or failure condition for an operation.
type Criterion struct {
	Condition  string                  `json:"condition" yaml:"condition" hcl:"condition"`
	Type       CriterionExpressionType `json:"type,omitempty" yaml:"type,omitempty" hcl:"type,optional"`
	Context    string                  `json:"context,omitempty" yaml:"context,omitempty" hcl:"context,optional"`
	Extensions map[string]any          `json:"-" yaml:"-" hcl:"-"`
}

type criterionAlias Criterion

var criterionKnownFields = []string{
	"condition", "type", "context",
}

func (c *Criterion) UnmarshalJSON(data []byte) error {
	var alias criterionAlias
	raw, extensions, err := unmarshalCoreWithExtensions(data, "criterion", criterionKnownFields, &alias)
	if err != nil {
		return err
	}
	*c = Criterion(alias)
	if typeValue, ok := raw["type"]; ok {
		var text string
		if err := json.Unmarshal(typeValue, &text); err == nil && text == "" {
			return fmt.Errorf("criterion.type must be omitted or one of simple, regex, jsonpath, or xpath")
		}
	}
	c.Extensions = extensions
	return nil
}

func (c Criterion) MarshalJSON() ([]byte, error) {
	alias := criterionAlias(c)
	return marshalWithExtensions(&alias, c.Extensions)
}
