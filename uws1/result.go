package uws1

// StructuralResult declares a named output from a structural workflow construct.
type StructuralResult struct {
	Name string `json:"name,omitempty" yaml:"name,omitempty" hcl:"name,optional"`
	Kind string `json:"kind,omitempty" yaml:"kind,omitempty" hcl:"kind,optional"`
}
