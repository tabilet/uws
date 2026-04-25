package uws1

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
	_, extensions, err := unmarshalCoreWithExtensions(data, "failureAction", failureActionKnownFields, &alias)
	if err != nil {
		return err
	}
	*f = FailureAction(alias)
	f.Extensions = extensions
	return nil
}

func (f FailureAction) MarshalJSON() ([]byte, error) {
	alias := failureActionAlias(f)
	return marshalWithExtensions(&alias, f.Extensions)
}
