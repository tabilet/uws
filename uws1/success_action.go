package uws1

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
	_, extensions, err := unmarshalCoreWithExtensions(data, "successAction", successActionKnownFields, &alias)
	if err != nil {
		return err
	}
	*s = SuccessAction(alias)
	s.Extensions = extensions
	return nil
}

func (s SuccessAction) MarshalJSON() ([]byte, error) {
	alias := successActionAlias(s)
	return marshalWithExtensions(&alias, s.Extensions)
}
