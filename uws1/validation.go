package uws1

import (
	"fmt"
	"reflect"
	"regexp"
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

var validWorkflowTypes = map[string]bool{
	"sequence": true,
	"parallel": true,
	"switch":   true,
	"merge":    true,
	"loop":     true,
	"await":    true,
}

var validStructuralResultKinds = map[string]bool{
	"switch": true,
	"merge":  true,
	"loop":   true,
}

type validationIndex struct {
	operations         map[string]bool
	workflows          map[string]bool
	steps              map[string]bool
	triggers           map[string]bool
	parallelGroups     map[string]bool
	sourceDescriptions map[string]bool
}

// Validate checks the document against UWS 1.x structural rules.
func (d *Document) Validate() error {
	result := d.ValidateResult()
	if result.Valid() {
		return nil
	}
	return result
}

// ValidateResult checks the document against UWS 1.x structural rules and returns all errors.
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
	return result
}

func newValidationIndex() *validationIndex {
	return &validationIndex{
		operations:         make(map[string]bool),
		workflows:          make(map[string]bool),
		steps:              make(map[string]bool),
		triggers:           make(map[string]bool),
		parallelGroups:     make(map[string]bool),
		sourceDescriptions: make(map[string]bool),
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
		}
		if op.ParallelGroup != "" {
			idx.parallelGroups[op.ParallelGroup] = true
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
		}
		if step.ParallelGroup != "" {
			idx.parallelGroups[step.ParallelGroup] = true
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
	for i, resultDecl := range d.Results {
		if resultDecl != nil {
			resultDecl.validate(fmt.Sprintf("results[%d]", i), result)
		} else {
			result.addError(fmt.Sprintf("results[%d]", i), "is nil")
		}
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
		result.addError(path+".name", fmt.Sprintf("must match pattern ^[A-Za-z0-9_\\-]+$; got %s", s.Name))
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
	hasOpenAPIBinding := hasSource || hasOpenAPIOperationID || hasOpenAPIOperationRef
	switch {
	case hasOpenAPIOperationID && hasOpenAPIOperationRef:
		result.addError(path, "cannot specify both openapiOperationId and openapiOperationRef")
	case hasOpenAPIBinding:
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
	case !hasExtensionOperationProfile(op.Extensions):
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

func hasExtensionOperationProfile(extensions map[string]any) bool {
	if len(extensions) == 0 {
		return false
	}
	value, ok := extensions[ExtensionOperationProfile]
	if !ok {
		return false
	}
	text, ok := value.(string)
	return ok && strings.TrimSpace(text) != ""
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
	}
	if w.Type == "" {
		result.addError(path+".type", "is required")
	} else if !validWorkflowTypes[w.Type] {
		result.addError(path+".type", fmt.Sprintf("%q is not valid", w.Type))
	}
	validateDependencyList(w.DependsOn, path+".dependsOn", idx, result)
	validateOutputs(w.Outputs, path+".outputs", result)
	validateSteps(w.Steps, path+".steps", idx, result)
	validateCases(w.Cases, path+".cases", idx, result)
	validateSteps(w.Default, path+".default", idx, result)
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
	}
	if s.Type != "" && !validWorkflowTypes[s.Type] {
		result.addError(path+".type", fmt.Sprintf("%q is not valid", s.Type))
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
	for i, route := range t.Routes {
		routePath := fmt.Sprintf("%s.routes[%d]", path, i)
		if route == nil {
			result.addError(routePath, "is nil")
			continue
		}
		route.validate(routePath, idx, result)
	}
}

func (r *TriggerRoute) validate(path string, idx *validationIndex, result *ValidationResult) {
	if r.Output == "" {
		result.addError(path+".output", "is required")
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

func (r *StructuralResult) validate(path string, result *ValidationResult) {
	if r.Kind != "" && !validStructuralResultKinds[r.Kind] {
		result.addError(path+".kind", fmt.Sprintf("%q is not valid", r.Kind))
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
