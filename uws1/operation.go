package uws1

import (
	"encoding/json"
	"fmt"
)

// Operation describes a UWS-local operation bound to an OpenAPI operation.
type Operation struct {
	OperationID         string         `json:"operationId" yaml:"operationId" hcl:"operationId,label"`
	SourceDescription   string         `json:"sourceDescription,omitempty" yaml:"sourceDescription,omitempty" hcl:"sourceDescription,optional"`
	OpenAPIOperationID  string         `json:"openapiOperationId,omitempty" yaml:"openapiOperationId,omitempty" hcl:"openapiOperationId,optional"`
	OpenAPIOperationRef string         `json:"openapiOperationRef,omitempty" yaml:"openapiOperationRef,omitempty" hcl:"openapiOperationRef,optional"`
	Description         string         `json:"description,omitempty" yaml:"description,omitempty" hcl:"description,optional"`
	Request             map[string]any `json:"request,omitempty" yaml:"request,omitempty" hcl:"request,optional"`

	// Execution control
	DependsOn     []string `json:"dependsOn,omitempty" yaml:"dependsOn,omitempty" hcl:"dependsOn,optional"`
	When          string   `json:"when,omitempty" yaml:"when,omitempty" hcl:"when,optional"`
	ForEach       string   `json:"forEach,omitempty" yaml:"forEach,omitempty" hcl:"forEach,optional"`
	Wait          string   `json:"wait,omitempty" yaml:"wait,omitempty" hcl:"wait,optional"`
	Workflow      string   `json:"workflow,omitempty" yaml:"workflow,omitempty" hcl:"workflow,optional"`
	ParallelGroup string   `json:"parallelGroup,omitempty" yaml:"parallelGroup,omitempty" hcl:"parallelGroup,optional"`

	// Success criteria and action handlers
	SuccessCriteria []*Criterion     `json:"successCriteria,omitempty" yaml:"successCriteria,omitempty" hcl:"successCriterion,block"`
	OnFailure       []*FailureAction `json:"onFailure,omitempty" yaml:"onFailure,omitempty" hcl:"onFailure,block"`
	OnSuccess       []*SuccessAction `json:"onSuccess,omitempty" yaml:"onSuccess,omitempty" hcl:"onSuccess,block"`

	// Outputs map friendly names to runtime expressions.
	Outputs    map[string]string `json:"outputs,omitempty" yaml:"outputs,omitempty" hcl:"outputs,optional"`
	Extensions map[string]any    `json:"-" yaml:"-" hcl:"-"`
}

type operationAlias Operation

var operationKnownFields = []string{
	"operationId", "sourceDescription", "openapiOperationId", "openapiOperationRef",
	"description", "request",
	"dependsOn", "when", "forEach", "wait", "workflow", "parallelGroup",
	"successCriteria", "onFailure", "onSuccess",
	"outputs",
}

func (o *Operation) UnmarshalJSON(data []byte) error {
	var alias operationAlias
	if err := json.Unmarshal(data, &alias); err != nil {
		return fmt.Errorf("unmarshaling operation: %w", err)
	}
	*o = Operation(alias)

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("unmarshaling operation extensions: %w", err)
	}
	if err := rejectUnknownFields(raw, operationKnownFields, "operation"); err != nil {
		return err
	}
	o.Extensions = extractExtensions(raw, operationKnownFields)
	return nil
}

func (o Operation) MarshalJSON() ([]byte, error) {
	alias := operationAlias(o)
	return marshalWithExtensions(&alias, o.Extensions)
}
