package uws1

// StructuralResult declares a named output from a structural workflow construct.
type StructuralResult struct {
	Name       string         `json:"name" yaml:"name" hcl:"name,label"`
	Kind       string         `json:"kind" yaml:"kind" hcl:"kind"`
	From       string         `json:"from" yaml:"from" hcl:"from"`
	Value      string         `json:"value,omitempty" yaml:"value,omitempty" hcl:"value,optional"`
	Extensions map[string]any `json:"-" yaml:"-" hcl:"-"`
}

type structuralResultAlias StructuralResult

var structuralResultKnownFields = []string{
	"name", "kind", "from", "value",
}

func (s *StructuralResult) UnmarshalJSON(data []byte) error {
	var alias structuralResultAlias
	_, extensions, err := unmarshalCoreWithExtensions(data, "structuralResult", structuralResultKnownFields, &alias)
	if err != nil {
		return err
	}
	*s = StructuralResult(alias)
	s.Extensions = extensions
	return nil
}

func (s StructuralResult) MarshalJSON() ([]byte, error) {
	alias := structuralResultAlias(s)
	return marshalWithExtensions(&alias, s.Extensions)
}
