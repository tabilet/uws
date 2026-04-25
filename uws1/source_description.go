package uws1

import (
	"regexp"
)

// SourceDescriptionType represents the type of source description.
type SourceDescriptionType string

const (
	SourceDescriptionTypeOpenAPI SourceDescriptionType = "openapi"
)

var sourceDescriptionNamePattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

// SourceDescription identifies a source document that operations reference.
type SourceDescription struct {
	Name       string                `json:"name" yaml:"name" hcl:"name,label"`
	URL        string                `json:"url" yaml:"url" hcl:"url"`
	Type       SourceDescriptionType `json:"type,omitempty" yaml:"type,omitempty" hcl:"type,optional"`
	Extensions map[string]any        `json:"-" yaml:"-" hcl:"-"`
}

type sourceDescriptionAlias SourceDescription

var sourceDescriptionKnownFields = []string{
	"name", "url", "type",
}

func (s *SourceDescription) UnmarshalJSON(data []byte) error {
	var alias sourceDescriptionAlias
	_, extensions, err := unmarshalCoreWithExtensions(data, "sourceDescription", sourceDescriptionKnownFields, &alias)
	if err != nil {
		return err
	}
	*s = SourceDescription(alias)
	s.Extensions = extensions
	return nil
}

func (s SourceDescription) MarshalJSON() ([]byte, error) {
	alias := sourceDescriptionAlias(s)
	return marshalWithExtensions(&alias, s.Extensions)
}
