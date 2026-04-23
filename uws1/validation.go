package uws1

import (
	"fmt"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/tabilet/uws/flowcore"
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
// Validate assumes the document has already been checked against uws.json via
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
// contract between this method and the uws.json JSON Schema pre-pass.
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

	idx := newValidationIndex()
	d.collectDocumentIDs(idx, result)
	d.validateDocumentReferences(idx, result)
	detectDependencyCycles(idx, result)
	return result
}

func newValidationIndex() *validationIndex {
	return &validationIndex{
		operations:           make(map[string]bool),
		workflows:            make(map[string]bool),
		workflowTypes:        make(map[string]string),
		workflowSteps:        make(map[string]map[string]string),
		steps:                make(map[string]bool),
		triggers:             make(map[string]bool),
		parallelGroups:       make(map[string]bool),
		parallelGroupMembers: make(map[string][]string),
		sourceDescriptions:   make(map[string]bool),
		dependencies:         make(map[string][]string),
	}
}

func (d *Document) collectDocumentIDs(idx *validationIndex, result *ValidationResult) {
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
	} else if !flowcore.IsWorkflowType(w.Type) {
		result.addError(path+".type", fmt.Sprintf("%q is not valid", w.Type))
	} else {
		validateStructuralTypeFields(w.Type, w.Items, w.Wait, len(w.Cases) > 0, len(w.Default) > 0, path, result)
		if flowcore.RequiresDependsOnForMerge(w.Type) && len(w.DependsOn) == 0 {
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
	if flowcore.RequiresItems(typeName) {
		if trimmedItems == "" {
			result.addError(path+".items", fmt.Sprintf("is required for %s", typeName))
		}
	} else if trimmedItems != "" {
		result.addError(path+".items", fmt.Sprintf("is not valid on %s", typeName))
	}
	if flowcore.RequiresWait(typeName) && strings.TrimSpace(wait) == "" {
		result.addError(path+".wait", fmt.Sprintf("is required for %s", typeName))
	}
	if hasCases && !flowcore.AllowsCases(typeName) {
		result.addError(path+".cases", fmt.Sprintf("is not valid on %s", typeName))
	}
	if hasDefault && !flowcore.AllowsDefault(typeName) {
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
		if !flowcore.IsWorkflowType(s.Type) {
			result.addError(path+".type", fmt.Sprintf("%q is not valid", s.Type))
		} else {
			validateStructuralTypeFields(s.Type, s.Items, s.Wait, len(s.Cases) > 0, len(s.Default) > 0, path, result)
			if flowcore.RequiresDependsOnForMerge(s.Type) && len(s.DependsOn) == 0 {
				result.addError(path+".dependsOn", "is required and must name at least one upstream construct for merge")
			}
		}
	}
	if s.OperationRef != "" && !idx.operations[s.OperationRef] {
		result.addError(path+".operationRef", fmt.Sprintf("references unknown operationId %q", s.OperationRef))
	}
	if s.Workflow != "" && !idx.workflows[s.Workflow] {
		result.addError(path+".workflow", fmt.Sprintf("references unknown workflowId %q", s.Workflow))
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
		result.addError(path+".to", "must contain at least one operationId")
	}
	for i, target := range r.To {
		if target == "" {
			result.addError(fmt.Sprintf("%s.to[%d]", path, i), "is required")
		} else if !idx.operations[target] {
			result.addError(fmt.Sprintf("%s.to[%d]", path, i), fmt.Sprintf("references unknown operationId %q", target))
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
	} else if !flowcore.IsStructuralResultKind(r.Kind) {
		result.addError(path+".kind", fmt.Sprintf("%q is not valid", r.Kind))
	}
	if r.From == "" {
		result.addError(path+".from", "is required")
		return
	}
	workflowID, stepID, _ := strings.Cut(r.From, ".")
	if workflowID == "" {
		result.addError(path+".from", fmt.Sprintf("%q is not a valid workflowId or workflowId.stepId", r.From))
		return
	}
	if !idx.workflows[workflowID] {
		result.addError(path+".from", fmt.Sprintf("references unknown workflowId %q", workflowID))
		return
	}
	var resolvedType string
	if stepID == "" {
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
		if a.Name == "" {
			result.addError(actionPath+".name", "is required")
		}
		if a.Type == "" {
			result.addError(actionPath+".type", "is required")
		} else if !validFailureActionTypes[a.Type] {
			result.addError(actionPath+".type", fmt.Sprintf("%q is not valid (must be end, goto, or retry)", a.Type))
		}
		validateGotoTarget(a.Type, a.WorkflowID, a.StepID, actionPath, idx, result)
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
		if a.Name == "" {
			result.addError(actionPath+".name", "is required")
		}
		if a.Type == "" {
			result.addError(actionPath+".type", "is required")
		} else if !validSuccessActionTypes[a.Type] {
			result.addError(actionPath+".type", fmt.Sprintf("%q is not valid (must be end or goto)", a.Type))
		}
		validateGotoTarget(a.Type, a.WorkflowID, a.StepID, actionPath, idx, result)
		validateCriteria(a.Criteria, actionPath+".criteria", result)
	}
}

func validateGotoTarget(actionType, workflowID, stepID, path string, idx *validationIndex, result *ValidationResult) {
	if actionType != "goto" {
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

	var stack []string
	var visit func(node string)
	visit = func(node string) {
		color[node] = gray
		stack = append(stack, node)
		for _, nb := range neighbors(node) {
			switch color[nb] {
			case white:
				visit(nb)
			case gray:
				start := -1
				for i, n := range stack {
					if n == nb {
						start = i
						break
					}
				}
				if start < 0 {
					continue
				}
				cycle := append([]string(nil), stack[start:]...)
				key := canonicalCycleKey(cycle)
				if seen[key] {
					continue
				}
				seen[key] = true
				cycle = append(cycle, nb)
				result.addError("dependsOn", fmt.Sprintf("cycle detected: %s", strings.Join(cycle, " -> ")))
			}
		}
		stack = stack[:len(stack)-1]
		color[node] = black
	}

	sources := make([]string, 0, len(idx.dependencies))
	for k := range idx.dependencies {
		sources = append(sources, k)
	}
	sort.Strings(sources)
	for _, s := range sources {
		if color[s] == white {
			visit(s)
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
