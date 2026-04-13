package uws1

import (
	"encoding/json"
	"fmt"
)

// StructuralResult declares a named output from a structural workflow construct.
type StructuralResult struct {
	Name       string         `json:"name,omitempty" yaml:"name,omitempty" hcl:"name,optional"`
	Kind       string         `json:"kind,omitempty" yaml:"kind,omitempty" hcl:"kind,optional"`
	Extensions map[string]any `json:"-" yaml:"-" hcl:"-"`
}

type structuralResultAlias StructuralResult

var structuralResultKnownFields = []string{
	"name", "kind",
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
	s.Extensions = extractExtensions(raw, structuralResultKnownFields)
	return nil
}

func (s StructuralResult) MarshalJSON() ([]byte, error) {
	alias := structuralResultAlias(s)
	return marshalWithExtensions(&alias, s.Extensions)
}
