package uws1

// Trigger defines an entry point that initiates workflow execution.
type Trigger struct {
	TriggerID string `json:"triggerId" yaml:"triggerId" hcl:"triggerId,label"`
	TriggerFields
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
	_, extensions, err := unmarshalCoreWithExtensions(data, "trigger", triggerKnownFields, &alias)
	if err != nil {
		return err
	}
	*t = Trigger(alias)
	t.Extensions = extensions
	return nil
}

func (t Trigger) MarshalJSON() ([]byte, error) {
	alias := triggerAlias(t)
	return marshalWithExtensions(&alias, t.Extensions)
}

// TriggerRoute maps a trigger output to top-level step or workflow targets.
type TriggerRoute struct {
	TriggerRouteFields
	Extensions map[string]any `json:"-" yaml:"-" hcl:"-"`
}

type triggerRouteAlias TriggerRoute

var triggerRouteKnownFields = []string{
	"output", "to",
}

func (t *TriggerRoute) UnmarshalJSON(data []byte) error {
	var alias triggerRouteAlias
	_, extensions, err := unmarshalCoreWithExtensions(data, "triggerRoute", triggerRouteKnownFields, &alias)
	if err != nil {
		return err
	}
	*t = TriggerRoute(alias)
	t.Extensions = extensions
	return nil
}

func (t TriggerRoute) MarshalJSON() ([]byte, error) {
	alias := triggerRouteAlias(t)
	return marshalWithExtensions(&alias, t.Extensions)
}
