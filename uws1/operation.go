package uws1

import (
	"encoding/json"
	"fmt"
)

// Operation describes a single service call (HTTP, SSH, Cmd, or Fnct).
type Operation struct {
	OperationID       string         `json:"operationId" yaml:"operationId" hcl:"operationId,label"`
	ServiceType       string         `json:"serviceType" yaml:"serviceType" hcl:"serviceType"`
	SourceDescription string         `json:"sourceDescription,omitempty" yaml:"sourceDescription,omitempty" hcl:"sourceDescription,optional"`
	Description       string         `json:"description,omitempty" yaml:"description,omitempty" hcl:"description,optional"`
	IsJSON      bool           `json:"isJson,omitempty" yaml:"isJson,omitempty" hcl:"isJson,optional"`
	Host        string         `json:"host,omitempty" yaml:"host,omitempty" hcl:"host,optional"`
	Method      string         `json:"method,omitempty" yaml:"method,omitempty" hcl:"method,optional"`
	Path        string         `json:"path,omitempty" yaml:"path,omitempty" hcl:"path,optional"`
	Provider    *Provider      `json:"provider,omitempty" yaml:"provider,omitempty" hcl:"provider,block"`
	Request     map[string]any `json:"request,omitempty" yaml:"request,omitempty" hcl:"request,optional"`

	// HTTP parameter schemas
	Security          []*SecurityRequirement `json:"security,omitempty" yaml:"security,omitempty" hcl:"security,block"`
	QueryPars         *ParamSchema           `json:"queryPars,omitempty" yaml:"queryPars,omitempty" hcl:"queryPars,block"`
	PathPars          *ParamSchema           `json:"pathPars,omitempty" yaml:"pathPars,omitempty" hcl:"pathPars,block"`
	HeaderPars        *ParamSchema           `json:"headerPars,omitempty" yaml:"headerPars,omitempty" hcl:"headerPars,block"`
	CookiePars        *ParamSchema           `json:"cookiePars,omitempty" yaml:"cookiePars,omitempty" hcl:"cookiePars,block"`
	PayloadPars       *ParamSchema           `json:"payloadPars,omitempty" yaml:"payloadPars,omitempty" hcl:"payloadPars,block"`
	PayloadRequired   bool                   `json:"payloadRequired,omitempty" yaml:"payloadRequired,omitempty" hcl:"payloadRequired,optional"`
	RequestMediaType  string                 `json:"requestMediaType,omitempty" yaml:"requestMediaType,omitempty" hcl:"requestMediaType,optional"`
	ResponseMediaType string                 `json:"responseMediaType,omitempty" yaml:"responseMediaType,omitempty" hcl:"responseMediaType,optional"`
	ResponseBody      *ParamSchema           `json:"responseBody,omitempty" yaml:"responseBody,omitempty" hcl:"responseBody,block"`
	ResponseHeaders   *ParamSchema           `json:"responseHeaders,omitempty" yaml:"responseHeaders,omitempty" hcl:"responseHeaders,block"`
	ResponseStatusCode *int                  `json:"responseStatusCode,omitempty" yaml:"responseStatusCode,omitempty" hcl:"responseStatusCode,optional"`

	// SSH/Cmd/Fnct fields
	Command    string `json:"command,omitempty" yaml:"command,omitempty" hcl:"command,optional"`
	Arguments  []any  `json:"arguments,omitempty" yaml:"arguments,omitempty" hcl:"arguments,optional"`
	WorkingDir string `json:"workingDir,omitempty" yaml:"workingDir,omitempty" hcl:"workingDir,optional"`
	Function   string `json:"function,omitempty" yaml:"function,omitempty" hcl:"function,optional"`

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
	"operationId", "serviceType", "sourceDescription", "description", "isJson",
	"host", "method", "path", "provider", "request",
	"security", "queryPars", "pathPars", "headerPars",
	"cookiePars", "payloadPars", "payloadRequired",
	"requestMediaType", "responseMediaType",
	"responseBody", "responseHeaders", "responseStatusCode",
	"command", "arguments", "workingDir", "function",
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
	o.Extensions = extractExtensions(raw, operationKnownFields)
	return nil
}

func (o Operation) MarshalJSON() ([]byte, error) {
	alias := operationAlias(o)
	return marshalWithExtensions(&alias, o.Extensions)
}
