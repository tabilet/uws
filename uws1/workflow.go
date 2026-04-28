package uws1

import (
	"context"
	"fmt"
)

// Workflow describes a control-flow construct (sequence, parallel, switch, merge, loop, await).
type Workflow struct {
	WorkflowID  string       `json:"workflowId" yaml:"workflowId" hcl:"workflowId,label"`
	Type        string       `json:"type" yaml:"type" hcl:"type"`
	Description string       `json:"description,omitempty" yaml:"description,omitempty" hcl:"description,optional"`
	Inputs      *ParamSchema `json:"inputs,omitempty" yaml:"inputs,omitempty" hcl:"inputs,block"`
	Idempotency *Idempotency `json:"idempotency,omitempty" yaml:"idempotency,omitempty" hcl:"idempotency,block"`
	WorkflowExecutionFields
	StructuralFields
	Steps      []*Step           `json:"steps,omitempty" yaml:"steps,omitempty" hcl:"step,block"`
	Cases      []*Case           `json:"cases,omitempty" yaml:"cases,omitempty" hcl:"case,block"`
	Default    []*Step           `json:"default,omitempty" yaml:"default,omitempty" hcl:"default,block"`
	Outputs    map[string]string `json:"outputs,omitempty" yaml:"outputs,omitempty" hcl:"outputs,optional"`
	Extensions map[string]any    `json:"-" yaml:"-" hcl:"-"`
}

// Execute executes the workflow using the bound runtime in the document.
func (w *Workflow) Execute(ctx context.Context, d *Document) error {
	if d == nil || d.Runtime == nil {
		return fmt.Errorf("uws1: workflow execution requires a bound runtime")
	}
	if err := d.Validate(); err != nil {
		return err
	}
	if err := d.ValidateExecutable(); err != nil {
		return err
	}
	orch := NewOrchestrator(d, d.Runtime)
	err := orch.executeWithSignals(ctx, func(ctx context.Context) error {
		return orch.ExecuteWorkflow(ctx, w)
	})
	d.setExecutionRecords(orch.snapshotRecords())
	return err
}

type workflowAlias Workflow

var workflowKnownFields = []string{
	"workflowId", "type", "description", "inputs",
	"idempotency",
	"dependsOn", "when", "forEach", "wait", "timeout",
	"items", "mode", "batchSize", "steps", "cases", "default",
	"outputs",
}

func (w *Workflow) UnmarshalJSON(data []byte) error {
	var alias workflowAlias
	_, extensions, err := unmarshalCoreWithExtensions(data, "workflow", workflowKnownFields, &alias)
	if err != nil {
		return err
	}
	*w = Workflow(alias)
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
	StepExecutionFields
	StructuralFields
	Steps      []*Step           `json:"steps,omitempty" yaml:"steps,omitempty" hcl:"step,block"`
	Cases      []*Case           `json:"cases,omitempty" yaml:"cases,omitempty" hcl:"case,block"`
	Default    []*Step           `json:"default,omitempty" yaml:"default,omitempty" hcl:"default,block"`
	Outputs    map[string]string `json:"outputs,omitempty" yaml:"outputs,omitempty" hcl:"outputs,optional"`
	Extensions map[string]any    `json:"-" yaml:"-" hcl:"-"`
}

// Execute executes the step using the bound runtime in the document.
func (s *Step) Execute(ctx context.Context, d *Document) error {
	if d == nil || d.Runtime == nil {
		return fmt.Errorf("uws1: step execution requires a bound runtime")
	}
	if err := d.Validate(); err != nil {
		return err
	}
	if err := d.ValidateExecutable(); err != nil {
		return err
	}
	orch := NewOrchestrator(d, d.Runtime)
	err := orch.executeWithSignals(ctx, func(ctx context.Context) error {
		return orch.ExecuteStep(ctx, s)
	})
	d.setExecutionRecords(orch.snapshotRecords())
	return err
}

type stepAlias Step

var stepKnownFields = []string{
	"stepId", "type", "description", "operationRef", "body",
	"dependsOn", "when", "forEach", "wait", "timeout", "workflow", "parallelGroup",
	"items", "mode", "batchSize", "steps", "cases", "default",
	"outputs",
}

func (s *Step) UnmarshalJSON(data []byte) error {
	var alias stepAlias
	_, extensions, err := unmarshalCoreWithExtensions(data, "step", stepKnownFields, &alias)
	if err != nil {
		return err
	}
	*s = Step(alias)
	s.Extensions = extensions
	return nil
}

func (s Step) MarshalJSON() ([]byte, error) {
	alias := stepAlias(s)
	return marshalWithExtensions(&alias, s.Extensions)
}

// Case describes a single branch within a switch construct.
type Case struct {
	CaseFields
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
	_, extensions, err := unmarshalCoreWithExtensions(data, "case", caseKnownFields, &alias)
	if err != nil {
		return err
	}
	*c = Case(alias)
	c.Extensions = extensions
	return nil
}

func (c Case) MarshalJSON() ([]byte, error) {
	alias := caseAlias(c)
	return marshalWithExtensions(&alias, c.Extensions)
}
