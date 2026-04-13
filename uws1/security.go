package uws1

import (
	"encoding/json"
	"fmt"
)

// SecurityRequirement describes a security mechanism applied to operations.
type SecurityRequirement struct {
	Name       string          `json:"name,omitempty" yaml:"name,omitempty" hcl:"name,label"`
	Scopes     []string        `json:"scopes,omitempty" yaml:"scopes,omitempty" hcl:"scopes,optional"`
	Scheme     *SecurityScheme `json:"scheme,omitempty" yaml:"scheme,omitempty" hcl:"scheme,block"`
	Initialize string          `json:"initialize,omitempty" yaml:"initialize,omitempty" hcl:"initialize,optional"`
	DataFile   string          `json:"dataFile,omitempty" yaml:"dataFile,omitempty" hcl:"dataFile,optional"`
	Extensions map[string]any  `json:"-" yaml:"-" hcl:"-"`
}

type securityRequirementAlias SecurityRequirement

var securityRequirementKnownFields = []string{
	"name", "scopes", "scheme", "initialize", "dataFile",
}

func (s *SecurityRequirement) UnmarshalJSON(data []byte) error {
	var alias securityRequirementAlias
	if err := json.Unmarshal(data, &alias); err != nil {
		return fmt.Errorf("unmarshaling securityRequirement: %w", err)
	}
	*s = SecurityRequirement(alias)

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("unmarshaling securityRequirement extensions: %w", err)
	}
	s.Extensions = extractExtensions(raw, securityRequirementKnownFields)
	return nil
}

func (s SecurityRequirement) MarshalJSON() ([]byte, error) {
	alias := securityRequirementAlias(s)
	return marshalWithExtensions(&alias, s.Extensions)
}

// SecurityScheme defines a security scheme that can be used by operations.
type SecurityScheme struct {
	Type        string         `json:"type" yaml:"type" hcl:"type"`
	Name        string         `json:"name,omitempty" yaml:"name,omitempty" hcl:"name,optional"`
	In          string         `json:"in,omitempty" yaml:"in,omitempty" hcl:"in,optional"`
	Scheme      string         `json:"scheme,omitempty" yaml:"scheme,omitempty" hcl:"scheme,optional"`
	Description string         `json:"description,omitempty" yaml:"description,omitempty" hcl:"description,optional"`
	Flows       *OAuthFlows    `json:"flows,omitempty" yaml:"flows,omitempty" hcl:"flows,block"`
	Extensions  map[string]any `json:"-" yaml:"-" hcl:"-"`
}

type securitySchemeAlias SecurityScheme

var securitySchemeKnownFields = []string{
	"type", "name", "in", "scheme", "description", "flows",
}

func (s *SecurityScheme) UnmarshalJSON(data []byte) error {
	var alias securitySchemeAlias
	if err := json.Unmarshal(data, &alias); err != nil {
		return fmt.Errorf("unmarshaling securityScheme: %w", err)
	}
	*s = SecurityScheme(alias)

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("unmarshaling securityScheme extensions: %w", err)
	}
	s.Extensions = extractExtensions(raw, securitySchemeKnownFields)
	return nil
}

func (s SecurityScheme) MarshalJSON() ([]byte, error) {
	alias := securitySchemeAlias(s)
	return marshalWithExtensions(&alias, s.Extensions)
}

// OAuthFlows describes the available OAuth2 flows.
type OAuthFlows struct {
	Password          *OAuthFlow     `json:"password,omitempty" yaml:"password,omitempty" hcl:"password,block"`
	Implicit          *OAuthFlow     `json:"implicit,omitempty" yaml:"implicit,omitempty" hcl:"implicit,block"`
	AuthorizationCode *OAuthFlow     `json:"authorizationCode,omitempty" yaml:"authorizationCode,omitempty" hcl:"authorizationCode,block"`
	ClientCredentials *OAuthFlow     `json:"clientCredentials,omitempty" yaml:"clientCredentials,omitempty" hcl:"clientCredentials,block"`
	Extensions        map[string]any `json:"-" yaml:"-" hcl:"-"`
}

type oauthFlowsAlias OAuthFlows

var oauthFlowsKnownFields = []string{
	"password", "implicit", "authorizationCode", "clientCredentials",
}

func (o *OAuthFlows) UnmarshalJSON(data []byte) error {
	var alias oauthFlowsAlias
	if err := json.Unmarshal(data, &alias); err != nil {
		return fmt.Errorf("unmarshaling oauthFlows: %w", err)
	}
	*o = OAuthFlows(alias)

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("unmarshaling oauthFlows extensions: %w", err)
	}
	o.Extensions = extractExtensions(raw, oauthFlowsKnownFields)
	return nil
}

func (o OAuthFlows) MarshalJSON() ([]byte, error) {
	alias := oauthFlowsAlias(o)
	return marshalWithExtensions(&alias, o.Extensions)
}

// OAuthFlow describes a single OAuth2 flow configuration.
type OAuthFlow struct {
	AuthorizationURL string            `json:"authorizationUrl,omitempty" yaml:"authorizationUrl,omitempty" hcl:"authorizationUrl,optional"`
	TokenURL         string            `json:"tokenUrl,omitempty" yaml:"tokenUrl,omitempty" hcl:"tokenUrl,optional"`
	RefreshURL       string            `json:"refreshUrl,omitempty" yaml:"refreshUrl,omitempty" hcl:"refreshUrl,optional"`
	Scopes           map[string]string `json:"scopes,omitempty" yaml:"scopes,omitempty" hcl:"scopes,optional"`
	Extensions       map[string]any    `json:"-" yaml:"-" hcl:"-"`
}

type oauthFlowAlias OAuthFlow

var oauthFlowKnownFields = []string{
	"authorizationUrl", "tokenUrl", "refreshUrl", "scopes",
}

func (o *OAuthFlow) UnmarshalJSON(data []byte) error {
	var alias oauthFlowAlias
	if err := json.Unmarshal(data, &alias); err != nil {
		return fmt.Errorf("unmarshaling oauthFlow: %w", err)
	}
	*o = OAuthFlow(alias)

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("unmarshaling oauthFlow extensions: %w", err)
	}
	o.Extensions = extractExtensions(raw, oauthFlowKnownFields)
	return nil
}

func (o OAuthFlow) MarshalJSON() ([]byte, error) {
	alias := oauthFlowAlias(o)
	return marshalWithExtensions(&alias, o.Extensions)
}
