package uws1

import (
	"encoding/json"
	"fmt"
)

// Provider describes the default service endpoint and configuration.
type Provider struct {
	Name       string         `json:"name,omitempty" yaml:"name,omitempty" hcl:"name,optional"`
	ServerURL  string         `json:"serverUrl,omitempty" yaml:"serverUrl,omitempty" hcl:"serverUrl,optional"`
	Appendices map[string]any `json:"appendices,omitempty" yaml:"appendices,omitempty" hcl:"appendices,optional"`
	Extensions map[string]any `json:"-" yaml:"-" hcl:"-"`
}

type providerAlias Provider

var providerKnownFields = []string{
	"name", "serverUrl", "appendices",
}

func (p *Provider) UnmarshalJSON(data []byte) error {
	var alias providerAlias
	if err := json.Unmarshal(data, &alias); err != nil {
		return fmt.Errorf("unmarshaling provider: %w", err)
	}
	*p = Provider(alias)

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("unmarshaling provider extensions: %w", err)
	}
	p.Extensions = extractExtensions(raw, providerKnownFields)
	return nil
}

func (p Provider) MarshalJSON() ([]byte, error) {
	alias := providerAlias(p)
	return marshalWithExtensions(&alias, p.Extensions)
}
