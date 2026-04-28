package uws1

import (
	"fmt"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// ValidationError represents one UWS validation error.
type ValidationError struct {
	Path    string
	Message string
}

// ValidationResult accumulates all validation errors found in a document.
type ValidationResult struct {
	Errors []ValidationError
}

// Valid reports whether validation found no errors.
func (r *ValidationResult) Valid() bool {
	return r == nil || len(r.Errors) == 0
}

// Error returns a compact, path-tagged summary of all validation errors.
func (r *ValidationResult) Error() string {
	if r.Valid() {
		return ""
	}
	msgs := make([]string, 0, len(r.Errors))
	for _, err := range r.Errors {
		msgs = append(msgs, fmt.Sprintf("%s %s", err.Path, err.Message))
	}
	return strings.Join(msgs, "; ")
}

func (r *ValidationResult) addError(path, message string) {
	r.Errors = append(r.Errors, ValidationError{Path: path, Message: message})
}

var (
	versionPattern       = regexp.MustCompile(`^1\.\d+\.\d+(-.+)?$`)
	constructIDPattern   = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
	componentNamePattern = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)
	outputNamePattern    = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)
)

var standardRequestKeys = map[string]bool{
	"path":   true,
	"query":  true,
	"header": true,
	"cookie": true,
	"body":   true,
}

type validationIndex struct {
	operations           map[string]bool
	workflows            map[string]bool
	workflowTypes        map[string]string
	workflowSteps        map[string]map[string]string
	entryWorkflowID      string
	hasEntryWorkflow     bool
	entryWorkflowSteps   map[string]bool
	steps                map[string]bool
	triggers             map[string]bool
	parallelGroups       map[string]bool
	parallelGroupMembers map[string][]string
	sourceDescriptions   map[string]bool
	dependencies         map[string][]string
}

// Validate runs the semantic validation layer and returns the first error as a
// single error, or nil if the document passes.
//
// Validate assumes the document has already been checked against the matching versions/1.x JSON Schema via
// a JSON Schema validator. The schema layer enforces structural shape (required
// fields, enum values, patterns, uniqueness of array items). Validate layers
// semantic rules on top: duplicate identifiers, reference integrity, binding
// mutual exclusivity, structural-type field constraints, and dependsOn cycles.
// Callers that invoke Validate without the schema pre-pass bypass the
// structural checks.
//
// Use ValidateResult when callers need path-tagged errors instead of a single
// collapsed error.
func (d *Document) Validate() error {
	result := d.ValidateResult()
	if result.Valid() {
		return nil
	}
	return result
}

// ValidateResult runs the semantic validation layer and returns every error it
// finds, each tagged with a structured Path. See Validate for the layering
// contract between this method and the versions/1.x JSON Schema pre-pass.
func (d *Document) ValidateResult() *ValidationResult {
	result := &ValidationResult{}
	if d == nil {
		result.addError("document", "is required")
		return result
	}

	if d.UWS == "" {
		result.addError("uws", "version is required")
	} else if !versionPattern.MatchString(d.UWS) {
		result.addError("uws", fmt.Sprintf("version %q does not match pattern 1.x.x", d.UWS))
	}
	if d.Info == nil {
		result.addError("info", "is required")
	} else {
		d.Info.validate("info", result)
	}
	if len(d.Operations) == 0 {
		result.addError("operations", "at least one operation is required")
	}
	d.validateTopLevelSourceDescriptions(result)
	d.validateVersionedFields(result)

	idx := newValidationIndex()
	d.collectDocumentIDs(idx, result)
	d.validateDocumentReferences(idx, result)
	detectDependencyCycles(idx, result)
	return result
}

func (d *Document) validateVersionedFields(result *ValidationResult) {
	supports11 := supportsUWSVersion(d.UWS, 1, 1)
	for i, op := range d.Operations {
		if op == nil {
			continue
		}
		validateVersionedTimeout(op.Timeout, fmt.Sprintf("operations[%d].timeout", i), supports11, result)
	}
	for i, wf := range d.Workflows {
		if wf == nil {
			continue
		}
		workflowPath := fmt.Sprintf("workflows[%d]", i)
		validateVersionedTimeout(wf.Timeout, workflowPath+".timeout", supports11, result)
		validateVersionedIdempotency(wf.Idempotency, workflowPath+".idempotency", supports11, result)
		validateVersionedStepFields(wf.Steps, workflowPath+".steps", supports11, result)
		validateVersionedCaseStepFields(wf.Cases, workflowPath+".cases", supports11, result)
		validateVersionedStepFields(wf.Default, workflowPath+".default", supports11, result)
	}
}

func validateVersionedStepFields(steps []*Step, path string, supports11 bool, result *ValidationResult) {
	for i, step := range steps {
		if step == nil {
			continue
		}
		stepPath := fmt.Sprintf("%s[%d]", path, i)
		validateVersionedTimeout(step.Timeout, stepPath+".timeout", supports11, result)
		validateVersionedStepFields(step.Steps, stepPath+".steps", supports11, result)
		validateVersionedCaseStepFields(step.Cases, stepPath+".cases", supports11, result)
		validateVersionedStepFields(step.Default, stepPath+".default", supports11, result)
	}
}

func validateVersionedCaseStepFields(cases []*Case, path string, supports11 bool, result *ValidationResult) {
	for i, c := range cases {
		if c == nil {
			continue
		}
		validateVersionedStepFields(c.Steps, fmt.Sprintf("%s[%d].steps", path, i), supports11, result)
	}
}

func validateVersionedTimeout(timeout *float64, path string, supports11 bool, result *ValidationResult) {
	if timeout == nil {
		return
	}
	if !supports11 {
		result.addError(path, "requires UWS 1.1.0 or later")
		return
	}
	if *timeout <= 0 {
		result.addError(path, "must be greater than 0")
	}
}

func validateVersionedIdempotency(idempotency *Idempotency, path string, supports11 bool, result *ValidationResult) {
	if idempotency == nil {
		return
	}
	if !supports11 {
		result.addError(path, "requires UWS 1.1.0 or later")
		return
	}
	if strings.TrimSpace(idempotency.Key) == "" {
		result.addError(path+".key", "is required")
	}
	switch idempotency.OnConflict {
	case "", "reject", "returnPrevious":
	default:
		result.addError(path+".onConflict", fmt.Sprintf("%q is not valid (must be reject or returnPrevious)", idempotency.OnConflict))
	}
	if idempotency.TTL != nil && *idempotency.TTL <= 0 {
		result.addError(path+".ttl", "must be greater than 0")
	}
}

func supportsUWSVersion(version string, major, minor int) bool {
	if !versionPattern.MatchString(version) {
		return false
	}
	base := version
	if idx := strings.Index(base, "-"); idx >= 0 {
		base = base[:idx]
	}
	parts := strings.Split(base, ".")
	if len(parts) < 2 {
		return false
	}
	gotMajor, err := strconv.Atoi(parts[0])
	if err != nil || gotMajor != major {
		return false
	}
	gotMinor, err := strconv.Atoi(parts[1])
	if err != nil {
		return false
	}
	return gotMinor >= minor
}

// validateTopLevelSourceDescriptions mirrors the schema's allOf rule that
// requires sourceDescriptions whenever any operation declares
// sourceDescription. Without this check the per-operation "references unknown
// sourceDescription" diagnostic still fires but points at the wrong field.
func (d *Document) validateTopLevelSourceDescriptions(result *ValidationResult) {
	if len(d.SourceDescriptions) > 0 {
		return
	}
	for i, op := range d.Operations {
		if op == nil {
			continue
		}
		if op.SourceDescription != "" {
			result.addError("sourceDescriptions", fmt.Sprintf("is required when any operation declares sourceDescription; operations[%d].sourceDescription is %q", i, op.SourceDescription))
			return
		}
	}
}

func newValidationIndex() *validationIndex {
	return &validationIndex{
		operations:           make(map[string]bool),
		workflows:            make(map[string]bool),
		workflowTypes:        make(map[string]string),
		workflowSteps:        make(map[string]map[string]string),
		entryWorkflowSteps:   make(map[string]bool),
		steps:                make(map[string]bool),
		triggers:             make(map[string]bool),
		parallelGroups:       make(map[string]bool),
		parallelGroupMembers: make(map[string][]string),
		sourceDescriptions:   make(map[string]bool),
		dependencies:         make(map[string][]string),
	}
}

func (d *Document) collectDocumentIDs(idx *validationIndex, result *ValidationResult) {
	if entry := semanticEntryWorkflow(d); entry != nil {
		idx.entryWorkflowID = entry.WorkflowID
		idx.hasEntryWorkflow = true
		for _, step := range entry.Steps {
			if step != nil && step.StepID != "" {
				idx.entryWorkflowSteps[step.StepID] = true
			}
		}
	}
	for i, sd := range d.SourceDescriptions {
		path := fmt.Sprintf("sourceDescriptions[%d]", i)
		if sd == nil {
			result.addError(path, "is nil")
			continue
		}
		if sd.Name != "" {
			if idx.sourceDescriptions[sd.Name] {
				result.addError(path+".name", fmt.Sprintf("duplicate sourceDescription name %q", sd.Name))
			}
			idx.sourceDescriptions[sd.Name] = true
		}
	}

	for i, op := range d.Operations {
		path := fmt.Sprintf("operations[%d]", i)
		if op == nil {
			result.addError(path, "is nil")
			continue
		}
		if op.OperationID != "" {
			if idx.operations[op.OperationID] {
				result.addError(path+".operationId", fmt.Sprintf("duplicate operationId %q", op.OperationID))
			}
			idx.operations[op.OperationID] = true
			if len(op.DependsOn) > 0 {
				idx.dependencies[op.OperationID] = append(idx.dependencies[op.OperationID], op.DependsOn...)
			}
		}
		if op.ParallelGroup != "" {
			if idx.operations[op.ParallelGroup] || idx.workflows[op.ParallelGroup] || idx.steps[op.ParallelGroup] {
				result.addError(path+".parallelGroup", fmt.Sprintf("parallelGroup %q collides with an executable identifier", op.ParallelGroup))
			}
			idx.parallelGroups[op.ParallelGroup] = true
			if op.OperationID != "" {
				idx.parallelGroupMembers[op.ParallelGroup] = append(idx.parallelGroupMembers[op.ParallelGroup], op.OperationID)
			}
		}
	}

	for i, wf := range d.Workflows {
		path := fmt.Sprintf("workflows[%d]", i)
		if wf == nil {
			result.addError(path, "is nil")
			continue
		}
		if wf.WorkflowID != "" {
			if idx.workflows[wf.WorkflowID] {
				result.addError(path+".workflowId", fmt.Sprintf("duplicate workflowId %q", wf.WorkflowID))
			}
			idx.workflows[wf.WorkflowID] = true
			idx.workflowTypes[wf.WorkflowID] = wf.Type
			idx.workflowSteps[wf.WorkflowID] = make(map[string]string)
			collectWorkflowStepTypes(wf.WorkflowID, wf.Steps, idx)
			collectWorkflowStepTypes(wf.WorkflowID, wf.Default, idx)
			for _, c := range wf.Cases {
				if c != nil {
					collectWorkflowStepTypes(wf.WorkflowID, c.Steps, idx)
				}
			}
			if len(wf.DependsOn) > 0 {
				idx.dependencies[wf.WorkflowID] = append(idx.dependencies[wf.WorkflowID], wf.DependsOn...)
			}
		}
		collectStepIDs(wf.Steps, path+".steps", idx, result)
		collectCaseStepIDs(wf.Cases, path+".cases", idx, result)
		collectStepIDs(wf.Default, path+".default", idx, result)
	}

	for i, trigger := range d.Triggers {
		path := fmt.Sprintf("triggers[%d]", i)
		if trigger == nil {
			result.addError(path, "is nil")
			continue
		}
		if trigger.TriggerID != "" {
			if idx.triggers[trigger.TriggerID] {
				result.addError(path+".triggerId", fmt.Sprintf("duplicate triggerId %q", trigger.TriggerID))
			}
			idx.triggers[trigger.TriggerID] = true
		}
	}
}

func semanticEntryWorkflow(d *Document) *Workflow {
	if d == nil {
		return nil
	}
	for _, wf := range d.Workflows {
		if wf != nil && wf.WorkflowID == "main" {
			return wf
		}
	}
	var only *Workflow
	for _, wf := range d.Workflows {
		if wf == nil {
			continue
		}
		if only != nil {
			return nil
		}
		only = wf
	}
	return only
}

// collectWorkflowStepTypes populates the workflow→stepID→structural-type index
// used when resolving results[].from references. Nil and unnamed steps are
// skipped here; collectStepIDs runs on the same tree and is responsible for
// reporting nil steps.
func collectWorkflowStepTypes(workflowID string, steps []*Step, idx *validationIndex) {
	for _, step := range steps {
		if step == nil || step.StepID == "" {
			continue
		}
		idx.workflowSteps[workflowID][step.StepID] = step.Type
		collectWorkflowStepTypes(workflowID, step.Steps, idx)
		collectWorkflowStepTypes(workflowID, step.Default, idx)
		for _, c := range step.Cases {
			if c != nil {
				collectWorkflowStepTypes(workflowID, c.Steps, idx)
			}
		}
	}
}

func collectStepIDs(steps []*Step, path string, idx *validationIndex, result *ValidationResult) {
	for i, step := range steps {
		stepPath := fmt.Sprintf("%s[%d]", path, i)
		if step == nil {
			result.addError(stepPath, "is nil")
			continue
		}
		if step.StepID != "" {
			if idx.steps[step.StepID] {
				result.addError(stepPath+".stepId", fmt.Sprintf("duplicate stepId %q", step.StepID))
			}
			idx.steps[step.StepID] = true
			if len(step.DependsOn) > 0 {
				idx.dependencies[step.StepID] = append(idx.dependencies[step.StepID], step.DependsOn...)
			}
		}
		if step.ParallelGroup != "" {
			if idx.operations[step.ParallelGroup] || idx.workflows[step.ParallelGroup] || idx.steps[step.ParallelGroup] {
				result.addError(stepPath+".parallelGroup", fmt.Sprintf("parallelGroup %q collides with an executable identifier", step.ParallelGroup))
			}
			idx.parallelGroups[step.ParallelGroup] = true
			if step.StepID != "" {
				idx.parallelGroupMembers[step.ParallelGroup] = append(idx.parallelGroupMembers[step.ParallelGroup], step.StepID)
			}
		}
		collectStepIDs(step.Steps, stepPath+".steps", idx, result)
		collectCaseStepIDs(step.Cases, stepPath+".cases", idx, result)
		collectStepIDs(step.Default, stepPath+".default", idx, result)
	}
}

func collectCaseStepIDs(cases []*Case, path string, idx *validationIndex, result *ValidationResult) {
	for i, c := range cases {
		casePath := fmt.Sprintf("%s[%d]", path, i)
		if c == nil {
			result.addError(casePath, "is nil")
			continue
		}
		collectStepIDs(c.Steps, casePath+".steps", idx, result)
	}
}

func (d *Document) validateDocumentReferences(idx *validationIndex, result *ValidationResult) {
	for i, sd := range d.SourceDescriptions {
		if sd != nil {
			sd.validate(fmt.Sprintf("sourceDescriptions[%d]", i), result)
		}
	}
	for i, op := range d.Operations {
		if op != nil {
			op.validate(fmt.Sprintf("operations[%d]", i), idx, result)
		}
	}
	for i, wf := range d.Workflows {
		if wf != nil {
			wf.validate(fmt.Sprintf("workflows[%d]", i), idx, result)
		}
	}
	for i, trigger := range d.Triggers {
		path := fmt.Sprintf("triggers[%d]", i)
		if trigger == nil {
			result.addError(path, "is nil")
			continue
		}
		trigger.validate(path, idx, result)
	}
	seenResultNames := make(map[string]bool)
	for i, resultDecl := range d.Results {
		resultPath := fmt.Sprintf("results[%d]", i)
		if resultDecl == nil {
			result.addError(resultPath, "is nil")
			continue
		}
		resultDecl.validate(resultPath, idx, seenResultNames, result)
	}
	if d.Components != nil {
		d.Components.validate("components", result)
	}
}

func (i *Info) validate(path string, result *ValidationResult) {
	if i.Title == "" {
		result.addError(path+".title", "is required")
	}
	if i.Version == "" {
		result.addError(path+".version", "is required")
	}
}

func (s *SourceDescription) validate(path string, result *ValidationResult) {
	if s.Name == "" {
		result.addError(path+".name", "is required")
	} else if !sourceDescriptionNamePattern.MatchString(s.Name) {
		result.addError(path+".name", fmt.Sprintf("must match pattern ^[A-Za-z0-9_-]+$; got %s", s.Name))
	}
	if s.URL == "" {
		result.addError(path+".url", "is required")
	}
	if s.Type != "" && s.Type != SourceDescriptionTypeOpenAPI {
		result.addError(path+".type", fmt.Sprintf("%q is not valid (must be openapi)", s.Type))
	}
}

func (op *Operation) validate(path string, idx *validationIndex, result *ValidationResult) {
	if op.OperationID == "" {
		result.addError(path+".operationId", "is required")
	}

	hasSource := op.SourceDescription != ""
	hasOpenAPIOperationID := op.OpenAPIOperationID != ""
	hasOpenAPIOperationRef := op.OpenAPIOperationRef != ""
	switch {
	case hasOpenAPIOperationID && hasOpenAPIOperationRef:
		result.addError(path, "cannot specify both openapiOperationId and openapiOperationRef")
	case op.HasOpenAPIBinding():
		if !hasSource {
			result.addError(path+".sourceDescription", "is required for OpenAPI-bound operations")
		} else if !idx.sourceDescriptions[op.SourceDescription] {
			result.addError(path+".sourceDescription", fmt.Sprintf("references unknown sourceDescription %q", op.SourceDescription))
		}
		if !hasOpenAPIOperationID && !hasOpenAPIOperationRef {
			result.addError(path, "requires exactly one of openapiOperationId or openapiOperationRef for OpenAPI-bound operations")
		}
		if hasOpenAPIOperationRef && !strings.HasPrefix(op.OpenAPIOperationRef, "#/") {
			result.addError(path+".openapiOperationRef", "must be a JSON Pointer fragment beginning with #/")
		}
	case !op.IsExtensionOwned():
		result.addError(path, "requires an OpenAPI binding or x-uws-operation-profile for extension-owned operations")
	}
	validateRequest(op.Request, path+".request", result)
	validateDependencyList(op.DependsOn, path+".dependsOn", idx, result)
	validateCriteria(op.SuccessCriteria, path+".successCriteria", result)
	validateFailureActions(op.OnFailure, path+".onFailure", idx, result)
	validateSuccessActions(op.OnSuccess, path+".onSuccess", idx, result)
	validateOutputs(op.Outputs, path+".outputs", result)
}

func validateRequest(request map[string]any, path string, result *ValidationResult) {
	// Request sections path/query/header/cookie are binding maps. Body is
	// intentionally unconstrained because payload shape is operation-specific.
	for key, value := range request {
		if strings.HasPrefix(key, "x-") {
			continue
		}
		if !standardRequestKeys[key] {
			result.addError(path+"."+key, "is not a standard request binding key; use path, query, header, cookie, body, or x-*")
			continue
		}
		switch key {
		case "path", "query", "header", "cookie":
			if !isObjectValue(value) {
				result.addError(path+"."+key, "must be an object")
			}
		}
	}
}

func isObjectValue(value any) bool {
	if value == nil {
		return false
	}
	rv := reflect.ValueOf(value)
	if rv.Kind() != reflect.Map {
		return false
	}
	if rv.Type().Key().Kind() != reflect.String {
		return false
	}
	return true
}

func validateDependencyList(deps []string, path string, idx *validationIndex, result *ValidationResult) {
	for i, dep := range deps {
		if dep == "" {
			result.addError(fmt.Sprintf("%s[%d]", path, i), "is required")
			continue
		}
		if !idx.operations[dep] && !idx.workflows[dep] && !idx.steps[dep] && !idx.parallelGroups[dep] {
			result.addError(fmt.Sprintf("%s[%d]", path, i), fmt.Sprintf("references unknown dependency %q", dep))
		}
	}
}

func validateOutputs(outputs map[string]string, path string, result *ValidationResult) {
	for key := range outputs {
		if !outputNamePattern.MatchString(key) {
			result.addError(path+"."+key, fmt.Sprintf("output name %q is not valid", key))
		}
	}
}

func (w *Workflow) validate(path string, idx *validationIndex, result *ValidationResult) {
	if w.WorkflowID == "" {
		result.addError(path+".workflowId", "is required")
	} else if !constructIDPattern.MatchString(w.WorkflowID) {
		result.addError(path+".workflowId", fmt.Sprintf("must match pattern ^[A-Za-z0-9_-]+$; got %s", w.WorkflowID))
	}
	if w.Type == "" {
		result.addError(path+".type", "is required")
	} else if !IsWorkflowType(w.Type) {
		result.addError(path+".type", fmt.Sprintf("%q is not valid", w.Type))
	} else {
		validateStructuralTypeFields(w.Type, w.Items, w.Wait, len(w.Cases) > 0, len(w.Default) > 0, path, result)
		if RequiresDependsOnForMerge(w.Type) && len(w.DependsOn) == 0 {
			result.addError(path+".dependsOn", "is required and must name at least one upstream construct for merge")
		}
	}
	validateDependencyList(w.DependsOn, path+".dependsOn", idx, result)
	validateOutputs(w.Outputs, path+".outputs", result)
	w.Inputs.validate(path+".inputs", result)
	validateSteps(w.Steps, path+".steps", idx, result)
	validateCases(w.Cases, path+".cases", idx, result)
	validateSteps(w.Default, path+".default", idx, result)
}

// validateStructuralTypeFields enforces §4.5.6.3 constraints on a workflow or
// step that declares a structural type. The caller passes the relevant fields;
// empty strings indicate the field is unset.
func validateStructuralTypeFields(typeName, items, wait string, hasCases, hasDefault bool, path string, result *ValidationResult) {
	trimmedItems := strings.TrimSpace(items)
	if RequiresItems(typeName) {
		if trimmedItems == "" {
			result.addError(path+".items", fmt.Sprintf("is required for %s", typeName))
		}
	} else if trimmedItems != "" {
		result.addError(path+".items", fmt.Sprintf("is not valid on %s", typeName))
	}
	if RequiresWait(typeName) && strings.TrimSpace(wait) == "" {
		result.addError(path+".wait", fmt.Sprintf("is required for %s", typeName))
	}
	if hasCases && !AllowsCases(typeName) {
		result.addError(path+".cases", fmt.Sprintf("is not valid on %s", typeName))
	}
	if hasDefault && !AllowsDefault(typeName) {
		result.addError(path+".default", fmt.Sprintf("is not valid on %s", typeName))
	}
}

func validateSteps(steps []*Step, path string, idx *validationIndex, result *ValidationResult) {
	for i, step := range steps {
		stepPath := fmt.Sprintf("%s[%d]", path, i)
		if step != nil {
			step.validate(stepPath, idx, result)
		}
	}
}

func (s *Step) validate(path string, idx *validationIndex, result *ValidationResult) {
	if s.StepID == "" {
		result.addError(path+".stepId", "is required")
	} else if !constructIDPattern.MatchString(s.StepID) {
		result.addError(path+".stepId", fmt.Sprintf("must match pattern ^[A-Za-z0-9_-]+$; got %s", s.StepID))
	}
	if s.Type != "" {
		if !IsWorkflowType(s.Type) {
			result.addError(path+".type", fmt.Sprintf("%q is not valid", s.Type))
		} else {
			validateStructuralTypeFields(s.Type, s.Items, s.Wait, len(s.Cases) > 0, len(s.Default) > 0, path, result)
			if RequiresDependsOnForMerge(s.Type) && len(s.DependsOn) == 0 {
				result.addError(path+".dependsOn", "is required and must name at least one upstream construct for merge")
			}
		}
	}
	if s.OperationRef != "" && !idx.operations[s.OperationRef] {
		result.addError(path+".operationRef", fmt.Sprintf("references unknown operationId %q", s.OperationRef))
	}
	isWorkflowReference := s.Workflow != "" && s.OperationRef == "" && s.Type == ""
	if isWorkflowReference && !idx.workflows[s.Workflow] {
		result.addError(path+".workflow", fmt.Sprintf("references unknown workflowId %q", s.Workflow))
	}
	if isWorkflowReference && (len(s.Steps) > 0 || len(s.Cases) > 0 || len(s.Default) > 0) {
		result.addError(path, "workflow-reference steps cannot also declare structural type or nested child blocks")
	}
	validateDependencyList(s.DependsOn, path+".dependsOn", idx, result)
	validateOutputs(s.Outputs, path+".outputs", result)
	validateSteps(s.Steps, path+".steps", idx, result)
	validateCases(s.Cases, path+".cases", idx, result)
	validateSteps(s.Default, path+".default", idx, result)
}

func validateCases(cases []*Case, path string, idx *validationIndex, result *ValidationResult) {
	for i, c := range cases {
		casePath := fmt.Sprintf("%s[%d]", path, i)
		if c == nil {
			continue
		}
		if c.Name == "" {
			result.addError(casePath+".name", "is required")
		}
		validateSteps(c.Steps, casePath+".steps", idx, result)
	}
}

func (t *Trigger) validate(path string, idx *validationIndex, result *ValidationResult) {
	if t.TriggerID == "" {
		result.addError(path+".triggerId", "is required")
	}
	outputs := make(map[string]bool, len(t.Outputs))
	for i, name := range t.Outputs {
		outputPath := fmt.Sprintf("%s.outputs[%d]", path, i)
		if name == "" {
			result.addError(outputPath, "is required")
			continue
		}
		if !outputNamePattern.MatchString(name) {
			result.addError(outputPath, fmt.Sprintf("output name %q is not valid", name))
			continue
		}
		if outputs[name] {
			result.addError(outputPath, fmt.Sprintf("duplicate output %q", name))
			continue
		}
		outputs[name] = true
	}
	if len(t.Routes) > 0 && len(t.Outputs) == 0 {
		result.addError(path+".outputs", "is required when routes is set")
	}
	for i, route := range t.Routes {
		routePath := fmt.Sprintf("%s.routes[%d]", path, i)
		if route == nil {
			result.addError(routePath, "is nil")
			continue
		}
		route.validate(routePath, t.Outputs, outputs, idx, result)
	}
}

func (r *TriggerRoute) validate(path string, outputList []string, outputs map[string]bool, idx *validationIndex, result *ValidationResult) {
	if r.Output == "" {
		result.addError(path+".output", "is required")
	} else if len(outputList) > 0 && !resolveTriggerOutput(r.Output, outputList, outputs) {
		result.addError(path+".output", fmt.Sprintf("%q is not a declared trigger output", r.Output))
	}
	if len(r.To) == 0 {
		result.addError(path+".to", "must contain at least one top-level stepId or workflowId")
	}
	for i, target := range r.To {
		if target == "" {
			result.addError(fmt.Sprintf("%s.to[%d]", path, i), "is required")
		} else if idx.workflows[target] {
			continue
		} else if idx.hasEntryWorkflow && !idx.entryWorkflowSteps[target] {
			result.addError(fmt.Sprintf("%s.to[%d]", path, i), fmt.Sprintf("references unknown top-level stepId or workflowId %q", target))
		}
	}
}

func resolveTriggerOutput(output string, outputList []string, outputs map[string]bool) bool {
	if outputs[output] {
		return true
	}
	if idx, err := strconv.Atoi(output); err == nil && idx >= 0 && idx < len(outputList) {
		return true
	}
	return false
}

func (r *StructuralResult) validate(path string, idx *validationIndex, seenNames map[string]bool, result *ValidationResult) {
	if r.Name == "" {
		result.addError(path+".name", "is required")
	} else {
		if !componentNamePattern.MatchString(r.Name) {
			result.addError(path+".name", fmt.Sprintf("%q is not valid", r.Name))
		}
		if seenNames[r.Name] {
			result.addError(path+".name", fmt.Sprintf("duplicate result name %q", r.Name))
		}
		seenNames[r.Name] = true
	}
	if r.Kind == "" {
		result.addError(path+".kind", "is required")
	} else if !IsStructuralResultKind(r.Kind) {
		result.addError(path+".kind", fmt.Sprintf("%q is not valid", r.Kind))
	}
	if r.From == "" {
		result.addError(path+".from", "is required")
		return
	}
	workflowID, stepID, hasStep := strings.Cut(r.From, ".")
	if workflowID == "" || !constructIDPattern.MatchString(workflowID) || (hasStep && (stepID == "" || !constructIDPattern.MatchString(stepID))) || strings.Contains(stepID, ".") {
		result.addError(path+".from", fmt.Sprintf("%q is not a valid workflowId or workflowId.stepId", r.From))
		return
	}
	if !idx.workflows[workflowID] {
		result.addError(path+".from", fmt.Sprintf("references unknown workflowId %q", workflowID))
		return
	}
	var resolvedType string
	if !hasStep {
		resolvedType = idx.workflowTypes[workflowID]
	} else {
		stepTypes, ok := idx.workflowSteps[workflowID]
		if !ok {
			result.addError(path+".from", fmt.Sprintf("references unknown stepId %q in workflow %q", stepID, workflowID))
			return
		}
		stepType, stepFound := stepTypes[stepID]
		if !stepFound {
			result.addError(path+".from", fmt.Sprintf("references unknown stepId %q in workflow %q", stepID, workflowID))
			return
		}
		resolvedType = stepType
		if resolvedType == "" {
			result.addError(path+".from", fmt.Sprintf("references stepId %q in workflow %q, but that step is not a structural construct", stepID, workflowID))
			return
		}
	}
	if r.Kind != "" && resolvedType != "" && resolvedType != r.Kind {
		result.addError(path+".kind", fmt.Sprintf("kind %q does not match %q type %q", r.Kind, r.From, resolvedType))
	}
}

var validCriterionTypes = map[CriterionExpressionType]bool{
	CriterionSimple:   true,
	CriterionRegex:    true,
	CriterionJSONPath: true,
	CriterionXPath:    true,
}

func validateCriteria(criteria []*Criterion, path string, result *ValidationResult) {
	for i, c := range criteria {
		criterionPath := fmt.Sprintf("%s[%d]", path, i)
		if c == nil {
			result.addError(criterionPath, "is nil")
			continue
		}
		if c.Condition == "" {
			result.addError(criterionPath+".condition", "is required")
		}
		if c.Type != "" && !validCriterionTypes[c.Type] {
			result.addError(criterionPath+".type", fmt.Sprintf("%q is not valid (must be simple, regex, jsonpath, or xpath)", c.Type))
		}
		if c.Type != "" && c.Type != CriterionSimple && c.Context == "" {
			result.addError(criterionPath+".context", "is required when type is regex, jsonpath, or xpath")
		}
	}
}

var validFailureActionTypes = map[string]bool{
	"end": true, "goto": true, "retry": true,
}

func validateFailureActions(actions []*FailureAction, path string, idx *validationIndex, result *ValidationResult) {
	for i, a := range actions {
		actionPath := fmt.Sprintf("%s[%d]", path, i)
		if a == nil {
			result.addError(actionPath, "is nil")
			continue
		}
		validateCommonAction(a.Name, a.Type, a.WorkflowID, a.StepID, actionPath, validFailureActionTypes, "end, goto, or retry", idx, result)
		if a.Type == "retry" && a.RetryLimit <= 0 {
			result.addError(actionPath, "retry requires retryLimit > 0")
		}
		if a.RetryAfter < 0 {
			result.addError(actionPath+".retryAfter", "must be non-negative")
		}
		validateCriteria(a.Criteria, actionPath+".criteria", result)
	}
}

var validSuccessActionTypes = map[string]bool{
	"end": true, "goto": true,
}

func validateSuccessActions(actions []*SuccessAction, path string, idx *validationIndex, result *ValidationResult) {
	for i, a := range actions {
		actionPath := fmt.Sprintf("%s[%d]", path, i)
		if a == nil {
			result.addError(actionPath, "is nil")
			continue
		}
		validateCommonAction(a.Name, a.Type, a.WorkflowID, a.StepID, actionPath, validSuccessActionTypes, "end or goto", idx, result)
		validateCriteria(a.Criteria, actionPath+".criteria", result)
	}
}

func validateCommonAction(name, actionType, workflowID, stepID, path string, validTypes map[string]bool, typeList string, idx *validationIndex, result *ValidationResult) {
	if name == "" {
		result.addError(path+".name", "is required")
	}
	if actionType == "" {
		result.addError(path+".type", "is required")
	} else if !validTypes[actionType] {
		result.addError(path+".type", fmt.Sprintf("%q is not valid (must be %s)", actionType, typeList))
	}
	validateGotoTarget(actionType, workflowID, stepID, path, idx, result)
}

func validateGotoTarget(actionType, workflowID, stepID, path string, idx *validationIndex, result *ValidationResult) {
	if actionType != "goto" {
		if workflowID != "" || stepID != "" {
			result.addError(path, "workflowId/stepId are only valid for goto actions")
		}
		return
	}
	if workflowID == "" && stepID == "" {
		result.addError(path, "goto requires workflowId or stepId")
		return
	}
	if workflowID != "" && stepID != "" {
		result.addError(path, "goto cannot specify both workflowId and stepId")
	}
	if workflowID != "" && !idx.workflows[workflowID] {
		result.addError(path+".workflowId", fmt.Sprintf("references unknown workflowId %q", workflowID))
	}
	if stepID != "" && !idx.steps[stepID] {
		result.addError(path+".stepId", fmt.Sprintf("references unknown stepId %q", stepID))
	}
}

func (c *Components) validate(path string, result *ValidationResult) {
	for name := range c.Variables {
		if !componentNamePattern.MatchString(name) {
			result.addError(path+".variables."+name, fmt.Sprintf("component name %q is not valid", name))
		}
	}
}

// detectDependencyCycles walks the dependsOn graph and reports any cycles.
// Parallel-group dependencies fan out to each member of the group (excluding
// the node itself) so that indirect self-dependencies surface too. Unknown
// dependency targets are ignored here because validateDependencyList already
// flags them.
func detectDependencyCycles(idx *validationIndex, result *ValidationResult) {
	if len(idx.dependencies) == 0 {
		return
	}

	const (
		white = 0
		gray  = 1
		black = 2
	)
	color := make(map[string]int)
	seen := make(map[string]bool)

	neighbors := func(node string) []string {
		deps := idx.dependencies[node]
		if len(deps) == 0 {
			return nil
		}
		out := make([]string, 0, len(deps))
		for _, d := range deps {
			if idx.operations[d] || idx.workflows[d] || idx.steps[d] {
				out = append(out, d)
				continue
			}
			if members, ok := idx.parallelGroupMembers[d]; ok {
				for _, m := range members {
					if m != node {
						out = append(out, m)
					}
				}
			}
		}
		return out
	}

	sources := make([]string, 0, len(idx.dependencies))
	for k := range idx.dependencies {
		sources = append(sources, k)
	}
	sort.Strings(sources)
	for _, s := range sources {
		if color[s] != white {
			continue
		}
		type frame struct {
			node      string
			neighbors []string
			next      int
		}
		stack := []frame{{node: s, neighbors: neighbors(s)}}
		path := []string{s}
		color[s] = gray
		for len(stack) > 0 {
			top := &stack[len(stack)-1]
			if top.next >= len(top.neighbors) {
				color[top.node] = black
				stack = stack[:len(stack)-1]
				path = path[:len(path)-1]
				continue
			}
			nb := top.neighbors[top.next]
			top.next++
			switch color[nb] {
			case white:
				color[nb] = gray
				stack = append(stack, frame{node: nb, neighbors: neighbors(nb)})
				path = append(path, nb)
			case gray:
				start := -1
				for i, n := range path {
					if n == nb {
						start = i
						break
					}
				}
				if start < 0 {
					continue
				}
				cycle := append([]string(nil), path[start:]...)
				key := canonicalCycleKey(cycle)
				if seen[key] {
					continue
				}
				seen[key] = true
				cycle = append(cycle, nb)
				result.addError("dependsOn", fmt.Sprintf("cycle detected: %s", strings.Join(cycle, " -> ")))
			}
		}
	}
}

func canonicalCycleKey(cycle []string) string {
	if len(cycle) == 0 {
		return ""
	}
	minIdx := 0
	for i, n := range cycle {
		if n < cycle[minIdx] {
			minIdx = i
		}
	}
	rotated := make([]string, 0, len(cycle))
	rotated = append(rotated, cycle[minIdx:]...)
	rotated = append(rotated, cycle[:minIdx]...)
	return strings.Join(rotated, "->")
}
