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
	if err := json.Unmarshal(data, &alias); err != nil {
		return fmt.Errorf("unmarshaling criterion: %w", err)
	}
	*c = Criterion(alias)

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("unmarshaling criterion extensions: %w", err)
	}
	if err := rejectUnknownFields(raw, criterionKnownFields, "criterion"); err != nil {
		return err
	}
	extensions, err := extractExtensions(raw, criterionKnownFields)
	if err != nil {
		return fmt.Errorf("unmarshaling criterion extensions: %w", err)
	}
	c.Extensions = extensions
	return nil
}

func (c Criterion) MarshalJSON() ([]byte, error) {
	alias := criterionAlias(c)
	return marshalWithExtensions(&alias, c.Extensions)
}
