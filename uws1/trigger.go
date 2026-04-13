package uws1

import (
	"encoding/json"
	"fmt"
)

// Trigger defines an entry point that initiates workflow execution.
type Trigger struct {
	TriggerID      string          `json:"triggerId" yaml:"triggerId" hcl:"triggerId,label"`
	Path           string          `json:"path,omitempty" yaml:"path,omitempty" hcl:"path,optional"`
	Methods        []string        `json:"methods,omitempty" yaml:"methods,omitempty" hcl:"methods,optional"`
	Authentication string          `json:"authentication,omitempty" yaml:"authentication,omitempty" hcl:"authentication,optional"`
	Options        map[string]any  `json:"options,omitempty" yaml:"options,omitempty" hcl:"options,optional"`
	Routes         []*TriggerRoute `json:"routes,omitempty" yaml:"routes,omitempty" hcl:"route,block"`
	Extensions     map[string]any  `json:"-" yaml:"-" hcl:"-"`
}

type triggerAlias Trigger

var triggerKnownFields = []string{
	"triggerId", "path", "methods", "authentication", "options", "routes",
}

func (t *Trigger) UnmarshalJSON(data []byte) error {
	var alias triggerAlias
	if err := json.Unmarshal(data, &alias); err != nil {
		return fmt.Errorf("unmarshaling trigger: %w", err)
	}
	*t = Trigger(alias)

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("unmarshaling trigger extensions: %w", err)
	}
	t.Extensions = extractExtensions(raw, triggerKnownFields)
	return nil
}

func (t Trigger) MarshalJSON() ([]byte, error) {
	alias := triggerAlias(t)
	return marshalWithExtensions(&alias, t.Extensions)
}

// TriggerRoute maps a trigger output to downstream operations.
type TriggerRoute struct {
	Output     string         `json:"output" yaml:"output" hcl:"output"`
	To         []string       `json:"to,omitempty" yaml:"to,omitempty" hcl:"to,optional"`
	Extensions map[string]any `json:"-" yaml:"-" hcl:"-"`
}

type triggerRouteAlias TriggerRoute

var triggerRouteKnownFields = []string{
	"output", "to",
}

func (t *TriggerRoute) UnmarshalJSON(data []byte) error {
	var alias triggerRouteAlias
	if err := json.Unmarshal(data, &alias); err != nil {
		return fmt.Errorf("unmarshaling triggerRoute: %w", err)
	}
	*t = TriggerRoute(alias)

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("unmarshaling triggerRoute extensions: %w", err)
	}
	t.Extensions = extractExtensions(raw, triggerRouteKnownFields)
	return nil
}

func (t TriggerRoute) MarshalJSON() ([]byte, error) {
	alias := triggerRouteAlias(t)
	return marshalWithExtensions(&alias, t.Extensions)
}
