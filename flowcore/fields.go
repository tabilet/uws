package flowcore

// WorkflowExecutionFields captures execution controls shared by structural
// workflows and steps.
type WorkflowExecutionFields struct {
	DependsOn []string `json:"dependsOn,omitempty" yaml:"dependsOn,omitempty" hcl:"dependsOn,optional"`
	When      string   `json:"when,omitempty" yaml:"when,omitempty" hcl:"when,optional"`
	ForEach   string   `json:"forEach,omitempty" yaml:"forEach,omitempty" hcl:"forEach,optional"`
	Wait      string   `json:"wait,omitempty" yaml:"wait,omitempty" hcl:"wait,optional"`
}

// StepExecutionFields captures execution controls specific to executable or
// nested steps. It includes workflow handoff and parallel grouping metadata
// that are not part of top-level workflow objects.
type StepExecutionFields struct {
	DependsOn     []string `json:"dependsOn,omitempty" yaml:"dependsOn,omitempty" hcl:"dependsOn,optional"`
	When          string   `json:"when,omitempty" yaml:"when,omitempty" hcl:"when,optional"`
	ForEach       string   `json:"forEach,omitempty" yaml:"forEach,omitempty" hcl:"forEach,optional"`
	Wait          string   `json:"wait,omitempty" yaml:"wait,omitempty" hcl:"wait,optional"`
	Workflow      string   `json:"workflow,omitempty" yaml:"workflow,omitempty" hcl:"workflow,optional"`
	ParallelGroup string   `json:"parallelGroup,omitempty" yaml:"parallelGroup,omitempty" hcl:"parallelGroup,optional"`
}

// StructuralFields captures structural-control attributes shared by switch,
// merge, and loop style nodes.
type StructuralFields struct {
	Items     string `json:"items,omitempty" yaml:"items,omitempty" hcl:"items,optional"`
	Mode      string `json:"mode,omitempty" yaml:"mode,omitempty" hcl:"mode,optional"`
	BatchSize string `json:"batchSize,omitempty" yaml:"batchSize,omitempty" hcl:"batchSize,optional"`
}

// CaseFields captures format-agnostic switch-branch metadata.
type CaseFields struct {
	Name string `json:"name" yaml:"name" hcl:"name,label"`
	When string `json:"when,omitempty" yaml:"when,omitempty" hcl:"when,optional"`
}

// TriggerFields captures generic trigger entrypoint fields.
type TriggerFields struct {
	Path           string   `json:"path,omitempty" yaml:"path,omitempty" hcl:"path,optional"`
	Methods        []string `json:"methods,omitempty" yaml:"methods,omitempty" hcl:"methods,optional"`
	Authentication string   `json:"authentication,omitempty" yaml:"authentication,omitempty" hcl:"authentication,optional"`
}

// TriggerRouteFields captures generic trigger routing fields.
type TriggerRouteFields struct {
	Output string   `json:"output" yaml:"output" hcl:"output"`
	To     []string `json:"to,omitempty" yaml:"to,omitempty" hcl:"to,optional"`
}
