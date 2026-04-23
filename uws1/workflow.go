package uws1

import (
	"encoding/json"
	"fmt"

	"github.com/tabilet/uws/flowcore"
)

// Workflow describes a control-flow construct (sequence, parallel, switch, merge, loop, await).
type Workflow struct {
	WorkflowID  string       `json:"workflowId" yaml:"workflowId" hcl:"workflowId,label"`
	Type        string       `json:"type" yaml:"type" hcl:"type"`
	Description string       `json:"description,omitempty" yaml:"description,omitempty" hcl:"description,optional"`
	Inputs      *ParamSchema `json:"inputs,omitempty" yaml:"inputs,omitempty" hcl:"inputs,block"`
	flowcore.WorkflowExecutionFields
	flowcore.StructuralFields
	Steps      []*Step           `json:"steps,omitempty" yaml:"steps,omitempty" hcl:"step,block"`
	Cases      []*Case           `json:"cases,omitempty" yaml:"cases,omitempty" hcl:"case,block"`
	Default    []*Step           `json:"default,omitempty" yaml:"default,omitempty" hcl:"default,block"`
	Outputs    map[string]string `json:"outputs,omitempty" yaml:"outputs,omitempty" hcl:"outputs,optional"`
	Extensions map[string]any    `json:"-" yaml:"-" hcl:"-"`
}

type workflowAlias Workflow

var workflowKnownFields = []string{
	"workflowId", "type", "description", "inputs",
	"dependsOn", "when", "forEach", "wait",
	"items", "mode", "batchSize", "steps", "cases", "default",
	"outputs",
}

func (w *Workflow) UnmarshalJSON(data []byte) error {
	var alias workflowAlias
	if err := json.Unmarshal(data, &alias); err != nil {
		return fmt.Errorf("unmarshaling workflow: %w", err)
	}
	*w = Workflow(alias)

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("unmarshaling workflow extensions: %w", err)
	}
	if err := rejectUnknownFields(raw, workflowKnownFields, "workflow"); err != nil {
		return err
	}
	extensions, err := extractExtensions(raw, workflowKnownFields)
	if err != nil {
		return fmt.Errorf("unmarshaling workflow extensions: %w", err)
	}
	w.Extensions = extensions
	return nil
}

func (w Workflow) MarshalJSON() ([]byte, error) {
	alias := workflowAlias(w)
	return marshalWithExtensions(&alias, w.Extensions)
}

// Step describes a nested step within a structural workflow.
type Step struct {
	StepID       string         `json:"stepId" yaml:"stepId" hcl:"stepId,label"`
	Type         string         `json:"type" yaml:"type" hcl:"type,optional"`
	Description  string         `json:"description,omitempty" yaml:"description,omitempty" hcl:"description,optional"`
	OperationRef string         `json:"operationRef,omitempty" yaml:"operationRef,omitempty" hcl:"operationRef,optional"`
	Body         map[string]any `json:"body,omitempty" yaml:"body,omitempty" hcl:"body,optional"`
	flowcore.StepExecutionFields
	flowcore.StructuralFields
	Steps      []*Step           `json:"steps,omitempty" yaml:"steps,omitempty" hcl:"step,block"`
	Cases      []*Case           `json:"cases,omitempty" yaml:"cases,omitempty" hcl:"case,block"`
	Default    []*Step           `json:"default,omitempty" yaml:"default,omitempty" hcl:"default,block"`
	Outputs    map[string]string `json:"outputs,omitempty" yaml:"outputs,omitempty" hcl:"outputs,optional"`
	Extensions map[string]any    `json:"-" yaml:"-" hcl:"-"`
}

type stepAlias Step

var stepKnownFields = []string{
	"stepId", "type", "description", "operationRef", "body",
	"dependsOn", "when", "forEach", "wait", "workflow", "parallelGroup",
	"items", "mode", "batchSize", "steps", "cases", "default",
	"outputs",
}

func (s *Step) UnmarshalJSON(data []byte) error {
	var alias stepAlias
	if err := json.Unmarshal(data, &alias); err != nil {
		return fmt.Errorf("unmarshaling step: %w", err)
	}
	*s = Step(alias)

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("unmarshaling step extensions: %w", err)
	}
	if err := rejectUnknownFields(raw, stepKnownFields, "step"); err != nil {
		return err
	}
	extensions, err := extractExtensions(raw, stepKnownFields)
	if err != nil {
		return fmt.Errorf("unmarshaling step extensions: %w", err)
	}
	s.Extensions = extensions
	return nil
}

func (s Step) MarshalJSON() ([]byte, error) {
	alias := stepAlias(s)
	return marshalWithExtensions(&alias, s.Extensions)
}

// Case describes a single branch within a switch construct.
type Case struct {
	flowcore.CaseFields
	Body       map[string]any `json:"body,omitempty" yaml:"body,omitempty" hcl:"body,optional"`
	Steps      []*Step        `json:"steps,omitempty" yaml:"steps,omitempty" hcl:"step,block"`
	Extensions map[string]any `json:"-" yaml:"-" hcl:"-"`
}

type caseAlias Case

var caseKnownFields = []string{
	"name", "when", "body", "steps",
}

func (c *Case) UnmarshalJSON(data []byte) error {
	var alias caseAlias
	if err := json.Unmarshal(data, &alias); err != nil {
		return fmt.Errorf("unmarshaling case: %w", err)
	}
	*c = Case(alias)

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("unmarshaling case extensions: %w", err)
	}
	if err := rejectUnknownFields(raw, caseKnownFields, "case"); err != nil {
		return err
	}
	extensions, err := extractExtensions(raw, caseKnownFields)
	if err != nil {
		return fmt.Errorf("unmarshaling case extensions: %w", err)
	}
	c.Extensions = extensions
	return nil
}

func (c Case) MarshalJSON() ([]byte, error) {
	alias := caseAlias(c)
	return marshalWithExtensions(&alias, c.Extensions)
}
