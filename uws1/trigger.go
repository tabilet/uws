package uws1

import (
	"encoding/json"
	"fmt"

	"github.com/tabilet/uws/flowcore"
)

// Trigger defines an entry point that initiates workflow execution.
type Trigger struct {
	TriggerID string `json:"triggerId" yaml:"triggerId" hcl:"triggerId,label"`
	flowcore.TriggerFields
	// Options is an intentionally open-shape map so each trigger implementation
	// can carry its own configuration. UWS does not restrict keys or values
	// beyond the JSON Schema's object shape.
	Options    map[string]any  `json:"options,omitempty" yaml:"options,omitempty" hcl:"options,optional"`
	Outputs    []string        `json:"outputs,omitempty" yaml:"outputs,omitempty" hcl:"outputs,optional"`
	Routes     []*TriggerRoute `json:"routes,omitempty" yaml:"routes,omitempty" hcl:"route,block"`
	Extensions map[string]any  `json:"-" yaml:"-" hcl:"-"`
}

type triggerAlias Trigger

var triggerKnownFields = []string{
	"triggerId", "path", "methods", "authentication", "options", "outputs", "routes",
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
	if err := rejectUnknownFields(raw, triggerKnownFields, "trigger"); err != nil {
		return err
	}
	extensions, err := extractExtensions(raw, triggerKnownFields)
	if err != nil {
		return fmt.Errorf("unmarshaling trigger extensions: %w", err)
	}
	t.Extensions = extensions
	return nil
}

func (t Trigger) MarshalJSON() ([]byte, error) {
	alias := triggerAlias(t)
	return marshalWithExtensions(&alias, t.Extensions)
}

// TriggerRoute maps a trigger output to downstream operations.
type TriggerRoute struct {
	flowcore.TriggerRouteFields
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
	if err := rejectUnknownFields(raw, triggerRouteKnownFields, "triggerRoute"); err != nil {
		return err
	}
	extensions, err := extractExtensions(raw, triggerRouteKnownFields)
	if err != nil {
		return fmt.Errorf("unmarshaling triggerRoute extensions: %w", err)
	}
	t.Extensions = extensions
	return nil
}

func (t TriggerRoute) MarshalJSON() ([]byte, error) {
	alias := triggerRouteAlias(t)
	return marshalWithExtensions(&alias, t.Extensions)
}
