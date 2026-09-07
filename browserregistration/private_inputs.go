package browserregistration

// InputSlot is a 1.1 private form-field definition, never a supplied value.
// Required and RequiredWhen are mutually exclusive. A false condition makes
// the field inactive. Enum members and condition literals are public choices.
type InputSlot struct {
	Type         string          `json:"type" yaml:"type"`
	Label        string          `json:"label" yaml:"label"`
	Required     *bool           `json:"required,omitempty" yaml:"required,omitempty"`
	RequiredWhen *InputCondition `json:"requiredWhen,omitempty" yaml:"requiredWhen,omitempty"`
	Enum         []any           `json:"enum,omitempty" yaml:"enum,omitempty"`
	MinLength    *int            `json:"minLength,omitempty" yaml:"minLength,omitempty"`
	MaxLength    *int            `json:"maxLength,omitempty" yaml:"maxLength,omitempty"`
	Minimum      *float64        `json:"minimum,omitempty" yaml:"minimum,omitempty"`
	Maximum      *float64        `json:"maximum,omitempty" yaml:"maximum,omitempty"`
}

// InputCondition references a required, unconditional public-choice field.
// It cannot compare credentials, private identifiers or arbitrary expressions.
type InputCondition struct {
	Slot   string `json:"slot" yaml:"slot"`
	Equals any    `json:"equals" yaml:"equals"`
}

// InputCheckpointStep requests an explicitly activated, immutable private
// snapshot for these slots. It never embeds a response or a filesystem path.
type InputCheckpointStep struct {
	ID    string   `json:"id" yaml:"id"`
	Slots []string `json:"slots" yaml:"slots"`
}

// FillInputStep applies a private scalar through a reviewed browser control.
// Fill supports strings/numbers, check booleans, and select enumerated strings.
type FillInputStep struct {
	Slot    string  `json:"slot" yaml:"slot"`
	Locator Locator `json:"locator" yaml:"locator"`
	Control string  `json:"control" yaml:"control"`
}

// Discovery records bounded observation coverage. Even owner_reviewed does
// not assert that every possible route or website state has been discovered.
type Discovery struct {
	Coverage    string   `json:"coverage" yaml:"coverage"`
	EntryPoints []string `json:"entryPoints" yaml:"entryPoints"`
	Limitations []string `json:"limitations" yaml:"limitations"`
}
