package uws1

// Idempotency declares workflow-run de-duplication metadata.
type Idempotency struct {
	Key        string         `json:"key" yaml:"key" hcl:"key"`
	OnConflict string         `json:"onConflict,omitempty" yaml:"onConflict,omitempty" hcl:"onConflict,optional"`
	TTL        *float64       `json:"ttl,omitempty" yaml:"ttl,omitempty" hcl:"ttl,optional"`
	Extensions map[string]any `json:"-" yaml:"-" hcl:"-"`
}

type idempotencyAlias Idempotency

var idempotencyKnownFields = []string{
	"key", "onConflict", "ttl",
}

func (i *Idempotency) UnmarshalJSON(data []byte) error {
	var alias idempotencyAlias
	_, extensions, err := unmarshalCoreWithExtensions(data, "idempotency", idempotencyKnownFields, &alias)
	if err != nil {
		return err
	}
	*i = Idempotency(alias)
	i.Extensions = extensions
	return nil
}

func (i Idempotency) MarshalJSON() ([]byte, error) {
	alias := idempotencyAlias(i)
	return marshalWithExtensions(&alias, i.Extensions)
}
