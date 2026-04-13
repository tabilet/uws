package uws1

import (
	"fmt"
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

// ValidServiceTypes lists the service types recognized by UWS 1.x.
var ValidServiceTypes = map[string]bool{
	"http": true, "ssh": true, "cmd": true, "fnct": true,
	"fileio": true, "ftp": true, "sftp": true, "scp": true,
	"smtp": true, "dns": true, "sql": true, "s3": true,
	"docker": true, "ldap": true, "llm": true, "mcp": true,
	"ioreadcloser": true, "ioreadwritecloser": true, "iowriter": true,
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

	if d.Components != nil {
		for name, op := range d.Components.Operations {
			if !componentNamePattern.MatchString(name) {
				result.addError(fmt.Sprintf("components.operations.%s", name), fmt.Sprintf("component name %q is not valid", name))
			}
			if op != nil && op.OperationID != "" {
				idx.operations[op.OperationID] = true
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
	for i, security := range d.Security {
		if security != nil {
			security.validate(fmt.Sprintf("security[%d]", i), result)
		} else {
			result.addError(fmt.Sprintf("security[%d]", i), "is nil")
		}
	}
	for i, resultDecl := range d.Results {
		if resultDecl != nil {
			resultDecl.validate(fmt.Sprintf("results[%d]", i), result)
		} else {
			result.addError(fmt.Sprintf("results[%d]", i), "is nil")
		}
	}
	if d.Components != nil {
		d.Components.validate("components", idx, result)
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
	if s.Type != "" && s.Type != SourceDescriptionTypeOpenAPI && s.Type != SourceDescriptionTypeArazzo {
		result.addError(path+".type", fmt.Sprintf("%q is not valid (must be openapi or arazzo)", s.Type))
	}
}

func (op *Operation) validate(path string, idx *validationIndex, result *ValidationResult) {
	if op.OperationID == "" {
		result.addError(path+".operationId", "is required")
	}
	if op.ServiceType == "" {
		result.addError(path+".serviceType", "is required")
	} else if !ValidServiceTypes[op.ServiceType] {
		result.addError(path+".serviceType", fmt.Sprintf("%q is not valid", op.ServiceType))
	}
	if op.SourceDescription != "" && !idx.sourceDescriptions[op.SourceDescription] {
		result.addError(path+".sourceDescription", fmt.Sprintf("references unknown sourceDescription %q", op.SourceDescription))
	}
	validateOperationFields(op, path, result)
	validateDependencyList(op.DependsOn, path+".dependsOn", idx, result)
	validateCriteria(op.SuccessCriteria, path+".successCriteria", result)
	validateFailureActions(op.OnFailure, path+".onFailure", idx, result)
	validateSuccessActions(op.OnSuccess, path+".onSuccess", idx, result)
	validateOutputs(op.Outputs, path+".outputs", result)
	validateSecurityRequirements(op.Security, path+".security", result)
}

func validateOperationFields(op *Operation, path string, result *ValidationResult) {
	switch op.ServiceType {
	case "http":
		if op.Method == "" {
			result.addError(path, fmt.Sprintf("(%s): http operations require method", op.OperationID))
		} else if !isValidHTTPMethod(op.Method) {
			result.addError(path+".method", fmt.Sprintf("invalid http method %q", op.Method))
		}
	case "ssh", "cmd":
		if op.Command == "" {
			result.addError(path, fmt.Sprintf("(%s): %s operations require command", op.OperationID, op.ServiceType))
		}
	case "fnct":
		if op.Function == "" {
			result.addError(path, fmt.Sprintf("(%s): fnct operations require function", op.OperationID))
		}
	}
}

func isValidHTTPMethod(method string) bool {
	switch strings.ToUpper(method) {
	case "GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS", "TRACE":
		return true
	}
	return false
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
	if s.Type == "" {
		result.addError(path+".type", "is required")
	} else if !ValidServiceTypes[s.Type] && !validWorkflowTypes[s.Type] {
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

func validateSecurityRequirements(requirements []*SecurityRequirement, path string, result *ValidationResult) {
	for i, requirement := range requirements {
		requirementPath := fmt.Sprintf("%s[%d]", path, i)
		if requirement == nil {
			result.addError(requirementPath, "is nil")
			continue
		}
		requirement.validate(requirementPath, result)
	}
}

func (s *SecurityRequirement) validate(path string, result *ValidationResult) {
	if s.Scheme != nil {
		s.Scheme.validate(path+".scheme", result)
	}
}

func (s *SecurityScheme) validate(path string, result *ValidationResult) {
	if s.Type == "" {
		result.addError(path+".type", "is required")
		return
	}
	switch s.Type {
	case "oauth2":
		if s.Flows == nil {
			result.addError(path+".flows", "is required for oauth2 security schemes")
		} else {
			s.Flows.validate(path+".flows", result)
		}
	case "apiKey":
		if s.Name == "" {
			result.addError(path+".name", "is required for apiKey security schemes")
		}
		if s.In == "" {
			result.addError(path+".in", "is required for apiKey security schemes")
		} else if s.In != "header" && s.In != "query" && s.In != "cookie" {
			result.addError(path+".in", fmt.Sprintf("%q is not valid (must be header, query, or cookie)", s.In))
		}
	case "http":
		if s.Scheme == "" {
			result.addError(path+".scheme", "is required for http security schemes")
		}
	default:
		result.addError(path+".type", fmt.Sprintf("%q is not valid (must be oauth2, apiKey, or http)", s.Type))
	}
}

func (o *OAuthFlows) validate(path string, result *ValidationResult) {
	if o.Password != nil {
		o.Password.validate(path+".password", false, true, result)
	}
	if o.Implicit != nil {
		o.Implicit.validate(path+".implicit", true, false, result)
	}
	if o.AuthorizationCode != nil {
		o.AuthorizationCode.validate(path+".authorizationCode", true, true, result)
	}
	if o.ClientCredentials != nil {
		o.ClientCredentials.validate(path+".clientCredentials", false, true, result)
	}
}

func (o *OAuthFlow) validate(path string, requireAuthorizationURL, requireTokenURL bool, result *ValidationResult) {
	if requireAuthorizationURL && o.AuthorizationURL == "" {
		result.addError(path+".authorizationUrl", "is required")
	}
	if requireTokenURL && o.TokenURL == "" {
		result.addError(path+".tokenUrl", "is required")
	}
}

func (c *Components) validate(path string, idx *validationIndex, result *ValidationResult) {
	for name, op := range c.Operations {
		componentPath := path + ".operations." + name
		if !componentNamePattern.MatchString(name) {
			result.addError(componentPath, fmt.Sprintf("component name %q is not valid", name))
		}
		if op == nil {
			result.addError(componentPath, "is nil")
			continue
		}
		op.validate(componentPath, idx, result)
	}
	for name, scheme := range c.SecuritySchemes {
		componentPath := path + ".securitySchemes." + name
		if !componentNamePattern.MatchString(name) {
			result.addError(componentPath, fmt.Sprintf("component name %q is not valid", name))
		}
		if scheme == nil {
			result.addError(componentPath, "is nil")
			continue
		}
		scheme.validate(componentPath, result)
	}
	for name := range c.Variables {
		if !componentNamePattern.MatchString(name) {
			result.addError(path+".variables."+name, fmt.Sprintf("component name %q is not valid", name))
		}
	}
}
