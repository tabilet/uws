package uws1

import (
	"encoding/json"
	"fmt"
)

// FailureAction describes what to do when an operation fails.
// Type is one of: end, goto, retry.
type FailureAction struct {
	Name       string         `json:"name" yaml:"name" hcl:"name,label"`
	Type       string         `json:"type" yaml:"type" hcl:"type"`
	WorkflowID string         `json:"workflowId,omitempty" yaml:"workflowId,omitempty" hcl:"workflowId,optional"`
	StepID     string         `json:"stepId,omitempty" yaml:"stepId,omitempty" hcl:"stepId,optional"`
	RetryAfter float64        `json:"retryAfter,omitempty" yaml:"retryAfter,omitempty" hcl:"retryAfter,optional"`
	RetryLimit int            `json:"retryLimit,omitempty" yaml:"retryLimit,omitempty" hcl:"retryLimit,optional"`
	Criteria   []*Criterion   `json:"criteria,omitempty" yaml:"criteria,omitempty" hcl:"criterion,block"`
	Extensions map[string]any `json:"-" yaml:"-" hcl:"-"`
}

type failureActionAlias FailureAction

var failureActionKnownFields = []string{
	"name", "type", "workflowId", "stepId",
	"retryAfter", "retryLimit", "criteria",
}

func (f *FailureAction) UnmarshalJSON(data []byte) error {
	var alias failureActionAlias
	if err := json.Unmarshal(data, &alias); err != nil {
		return fmt.Errorf("unmarshaling failureAction: %w", err)
	}
	*f = FailureAction(alias)

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("unmarshaling failureAction extensions: %w", err)
	}
	if err := rejectUnknownFields(raw, failureActionKnownFields, "failureAction"); err != nil {
		return err
	}
	extensions, err := extractExtensions(raw, failureActionKnownFields)
	if err != nil {
		return fmt.Errorf("unmarshaling failureAction extensions: %w", err)
	}
	f.Extensions = extensions
	return nil
}

func (f FailureAction) MarshalJSON() ([]byte, error) {
	alias := failureActionAlias(f)
	return marshalWithExtensions(&alias, f.Extensions)
}
