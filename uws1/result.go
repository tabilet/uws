package uws1

import (
	"encoding/json"
	"fmt"
)

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
	if err := json.Unmarshal(data, &alias); err != nil {
		return fmt.Errorf("unmarshaling structuralResult: %w", err)
	}
	*s = StructuralResult(alias)

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("unmarshaling structuralResult extensions: %w", err)
	}
	if err := rejectUnknownFields(raw, structuralResultKnownFields, "structuralResult"); err != nil {
		return err
	}
	extensions, err := extractExtensions(raw, structuralResultKnownFields)
	if err != nil {
		return fmt.Errorf("unmarshaling structuralResult extensions: %w", err)
	}
	s.Extensions = extensions
	return nil
}

func (s StructuralResult) MarshalJSON() ([]byte, error) {
	alias := structuralResultAlias(s)
	return marshalWithExtensions(&alias, s.Extensions)
}
