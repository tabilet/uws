package uws1

const (
	WorkflowTypeSequence = "sequence"
	WorkflowTypeParallel = "parallel"
	WorkflowTypeSwitch   = "switch"
	WorkflowTypeMerge    = "merge"
	WorkflowTypeLoop     = "loop"
	WorkflowTypeAwait    = "await"
)

const (
	StructuralResultKindSwitch = WorkflowTypeSwitch
	StructuralResultKindMerge  = WorkflowTypeMerge
	StructuralResultKindLoop   = WorkflowTypeLoop
)

// IsWorkflowType reports whether typeName names a standard structural workflow
// type.
func IsWorkflowType(typeName string) bool {
	switch typeName {
	case WorkflowTypeSequence, WorkflowTypeParallel, WorkflowTypeSwitch, WorkflowTypeMerge, WorkflowTypeLoop, WorkflowTypeAwait:
		return true
	default:
		return false
	}
}

// IsStructuralResultKind reports whether kind names a supported structural
// result source kind.
func IsStructuralResultKind(kind string) bool {
	switch kind {
	case StructuralResultKindSwitch, StructuralResultKindMerge, StructuralResultKindLoop:
		return true
	default:
		return false
	}
}

// RequiresItems reports whether the workflow type requires an items selector.
func RequiresItems(typeName string) bool {
	return typeName == WorkflowTypeLoop
}

// RequiresWait reports whether the workflow type requires a wait selector.
func RequiresWait(typeName string) bool {
	return typeName == WorkflowTypeAwait
}

// AllowsCases reports whether the workflow type permits case blocks.
func AllowsCases(typeName string) bool {
	return typeName == WorkflowTypeSwitch
}

// AllowsDefault reports whether the workflow type permits default blocks.
func AllowsDefault(typeName string) bool {
	return typeName == WorkflowTypeSwitch
}

// RequiresDependsOnForMerge reports whether the workflow type requires
// dependsOn to name upstream constructs.
func RequiresDependsOnForMerge(typeName string) bool {
	return typeName == WorkflowTypeMerge
}

// WorkflowExecutionFields captures execution controls shared by top-level
// workflow objects.
type WorkflowExecutionFields struct {
	DependsOn []string `json:"dependsOn,omitempty" yaml:"dependsOn,omitempty" hcl:"dependsOn,optional"`
	When      string   `json:"when,omitempty" yaml:"when,omitempty" hcl:"when,optional"`
	ForEach   string   `json:"forEach,omitempty" yaml:"forEach,omitempty" hcl:"forEach,optional"`
	Wait      string   `json:"wait,omitempty" yaml:"wait,omitempty" hcl:"wait,optional"`
}

// OperationExecutionFields captures execution controls on leaf operations.
type OperationExecutionFields struct {
	DependsOn     []string `json:"dependsOn,omitempty" yaml:"dependsOn,omitempty" hcl:"dependsOn,optional"`
	When          string   `json:"when,omitempty" yaml:"when,omitempty" hcl:"when,optional"`
	ForEach       string   `json:"forEach,omitempty" yaml:"forEach,omitempty" hcl:"forEach,optional"`
	Wait          string   `json:"wait,omitempty" yaml:"wait,omitempty" hcl:"wait,optional"`
	ParallelGroup string   `json:"parallelGroup,omitempty" yaml:"parallelGroup,omitempty" hcl:"parallelGroup,optional"`
}

// StepExecutionFields captures execution controls on nested steps.
type StepExecutionFields struct {
	DependsOn     []string `json:"dependsOn,omitempty" yaml:"dependsOn,omitempty" hcl:"dependsOn,optional"`
	When          string   `json:"when,omitempty" yaml:"when,omitempty" hcl:"when,optional"`
	ForEach       string   `json:"forEach,omitempty" yaml:"forEach,omitempty" hcl:"forEach,optional"`
	Wait          string   `json:"wait,omitempty" yaml:"wait,omitempty" hcl:"wait,optional"`
	Workflow      string   `json:"workflow,omitempty" yaml:"workflow,omitempty" hcl:"workflow,optional"`
	ParallelGroup string   `json:"parallelGroup,omitempty" yaml:"parallelGroup,omitempty" hcl:"parallelGroup,optional"`
}

// RunnableExecutionFields is the compatibility name for nested-step execution
// controls. New code should use StepExecutionFields or OperationExecutionFields
// depending on which UWS object is being modeled.
type RunnableExecutionFields = StepExecutionFields

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
