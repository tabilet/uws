package uws1

import (
	"encoding/json"
	"fmt"
)

// SuccessAction describes what to do when an operation succeeds.
// Type is one of: end, goto.
type SuccessAction struct {
	Name       string         `json:"name" yaml:"name" hcl:"name,label"`
	Type       string         `json:"type" yaml:"type" hcl:"type"`
	WorkflowID string         `json:"workflowId,omitempty" yaml:"workflowId,omitempty" hcl:"workflowId,optional"`
	StepID     string         `json:"stepId,omitempty" yaml:"stepId,omitempty" hcl:"stepId,optional"`
	Criteria   []*Criterion   `json:"criteria,omitempty" yaml:"criteria,omitempty" hcl:"criterion,block"`
	Extensions map[string]any `json:"-" yaml:"-" hcl:"-"`
}

type successActionAlias SuccessAction

var successActionKnownFields = []string{
	"name", "type", "workflowId", "stepId", "criteria",
}

func (s *SuccessAction) UnmarshalJSON(data []byte) error {
	var alias successActionAlias
	if err := json.Unmarshal(data, &alias); err != nil {
		return fmt.Errorf("unmarshaling successAction: %w", err)
	}
	*s = SuccessAction(alias)

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("unmarshaling successAction extensions: %w", err)
	}
	if err := rejectUnknownFields(raw, successActionKnownFields, "successAction"); err != nil {
		return err
	}
	s.Extensions = extractExtensions(raw, successActionKnownFields)
	return nil
}

func (s SuccessAction) MarshalJSON() ([]byte, error) {
	alias := successActionAlias(s)
	return marshalWithExtensions(&alias, s.Extensions)
}
