package uws1

import (
	"encoding/json"
	"fmt"
)

// Document is the root object of a UWS 1.x document.
type Document struct {
	UWS                string               `json:"uws" yaml:"uws" hcl:"uws"`
	Info               *Info                `json:"info" yaml:"info" hcl:"info,block"`
	SourceDescriptions []*SourceDescription `json:"sourceDescriptions,omitempty" yaml:"sourceDescriptions,omitempty" hcl:"sourceDescription,block"`
	Variables          map[string]any       `json:"variables,omitempty" yaml:"variables,omitempty" hcl:"variables,optional"`
	Operations         []*Operation         `json:"operations" yaml:"operations" hcl:"operation,block"`
	Workflows          []*Workflow          `json:"workflows,omitempty" yaml:"workflows,omitempty" hcl:"workflow,block"`
	Triggers           []*Trigger           `json:"triggers,omitempty" yaml:"triggers,omitempty" hcl:"trigger,block"`
	Results            []*StructuralResult  `json:"results,omitempty" yaml:"results,omitempty" hcl:"result,block"`
	Components         *Components          `json:"components,omitempty" yaml:"components,omitempty" hcl:"components,block"`
	Extensions         map[string]any       `json:"-" yaml:"-" hcl:"-"`
}

type documentAlias Document

var documentKnownFields = []string{
	"uws", "info", "sourceDescriptions", "variables",
	"operations", "workflows", "triggers", "results",
	"components",
}

func (d *Document) UnmarshalJSON(data []byte) error {
	var alias documentAlias
	if err := json.Unmarshal(data, &alias); err != nil {
		return fmt.Errorf("unmarshaling document: %w", err)
	}
	*d = Document(alias)

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("unmarshaling document extensions: %w", err)
	}
	if err := rejectUnknownFields(raw, documentKnownFields, "document"); err != nil {
		return err
	}
	d.Extensions = extractExtensions(raw, documentKnownFields)
	return nil
}

func (d Document) MarshalJSON() ([]byte, error) {
	alias := documentAlias(d)
	return marshalWithExtensions(&alias, d.Extensions)
}

// Info provides metadata about the UWS document.
type Info struct {
	Title       string         `json:"title" yaml:"title" hcl:"title"`
	Summary     string         `json:"summary,omitempty" yaml:"summary,omitempty" hcl:"summary,optional"`
	Description string         `json:"description,omitempty" yaml:"description,omitempty" hcl:"description,optional"`
	Version     string         `json:"version" yaml:"version" hcl:"version"`
	Extensions  map[string]any `json:"-" yaml:"-" hcl:"-"`
}

type infoAlias Info

var infoKnownFields = []string{
	"title", "summary", "description", "version",
}

func (i *Info) UnmarshalJSON(data []byte) error {
	var alias infoAlias
	if err := json.Unmarshal(data, &alias); err != nil {
		return fmt.Errorf("unmarshaling info: %w", err)
	}
	*i = Info(alias)

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("unmarshaling info extensions: %w", err)
	}
	if err := rejectUnknownFields(raw, infoKnownFields, "info"); err != nil {
		return err
	}
	i.Extensions = extractExtensions(raw, infoKnownFields)
	return nil
}

func (i Info) MarshalJSON() ([]byte, error) {
	alias := infoAlias(i)
	return marshalWithExtensions(&alias, i.Extensions)
}
