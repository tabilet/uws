package uws1

// Components holds reusable objects scoped to the UWS document.
type Components struct {
	// Variables is an intentionally open-shape map; any JSON-compatible value is
	// allowed. Keys must match componentNamePattern (enforced by Validate);
	// values are not inspected.
	Variables  map[string]any `json:"variables,omitempty" yaml:"variables,omitempty" hcl:"variables,optional"`
	Extensions map[string]any `json:"-" yaml:"-" hcl:"-"`
}

type componentsAlias Components

var componentsKnownFields = []string{
	"variables",
}

func (c *Components) UnmarshalJSON(data []byte) error {
	var alias componentsAlias
	_, extensions, err := unmarshalCoreWithExtensions(data, "components", componentsKnownFields, &alias)
	if err != nil {
		return err
	}
	*c = Components(alias)
	c.Extensions = extensions
	return nil
}

func (c Components) MarshalJSON() ([]byte, error) {
	alias := componentsAlias(c)
	return marshalWithExtensions(&alias, c.Extensions)
}
