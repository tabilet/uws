package uws1

import (
	"fmt"
	"regexp"
	"strings"
)

var versionPattern = regexp.MustCompile(`^1\.\d+\.\d+(-.+)?$`)

// ValidServiceTypes lists the service types recognized by UWS 1.x.
var ValidServiceTypes = map[string]bool{
	"http": true, "ssh": true, "cmd": true, "fnct": true,
	"fileio": true, "ftp": true, "sftp": true, "scp": true,
	"smtp": true, "dns": true, "sql": true, "s3": true,
	"docker": true, "ldap": true, "llm": true, "mcp": true,
	"ioreadcloser": true, "ioreadwritecloser": true, "iowriter": true,
}

// Validate checks the document against UWS 1.x structural rules.
func (d *Document) Validate() error {
	if d.UWS == "" {
		return fmt.Errorf("uws version is required")
	}
	if !versionPattern.MatchString(d.UWS) {
		return fmt.Errorf("uws version %q does not match pattern 1.x.x", d.UWS)
	}
	if d.Info == nil {
		return fmt.Errorf("info is required")
	}
	if d.Info.Title == "" {
		return fmt.Errorf("info.title is required")
	}
	if d.Info.Version == "" {
		return fmt.Errorf("info.version is required")
	}
	if len(d.Operations) == 0 {
		return fmt.Errorf("at least one operation is required")
	}

	sdNames := make(map[string]bool)
	for i, sd := range d.SourceDescriptions {
		if sd == nil {
			return fmt.Errorf("sourceDescriptions[%d] is nil", i)
		}
		if sd.Name == "" {
			return fmt.Errorf("sourceDescriptions[%d].name is required", i)
		}
		if sdNames[sd.Name] {
			return fmt.Errorf("duplicate sourceDescription name %q", sd.Name)
		}
		sdNames[sd.Name] = true
		if sd.URL == "" {
			return fmt.Errorf("sourceDescriptions[%d].url is required", i)
		}
		if sd.Type != "" && sd.Type != "openapi" && sd.Type != "arazzo" {
			return fmt.Errorf("sourceDescriptions[%d].type %q is not valid (must be openapi or arazzo)", i, sd.Type)
		}
	}

	opIDs := make(map[string]bool)
	for i, op := range d.Operations {
		if op == nil {
			return fmt.Errorf("operations[%d] is nil", i)
		}
		if op.OperationID == "" {
			return fmt.Errorf("operations[%d].operationId is required", i)
		}
		if opIDs[op.OperationID] {
			return fmt.Errorf("duplicate operationId %q", op.OperationID)
		}
		opIDs[op.OperationID] = true

		if op.ServiceType == "" {
			return fmt.Errorf("operations[%d].serviceType is required", i)
		}
		if !ValidServiceTypes[op.ServiceType] {
			return fmt.Errorf("operations[%d].serviceType %q is not valid", i, op.ServiceType)
		}

		if err := validateOperationFields(op, i); err != nil {
			return err
		}
		if err := validateCriteria(op.SuccessCriteria, fmt.Sprintf("operations[%d].successCriteria", i)); err != nil {
			return err
		}
		if err := validateFailureActions(op.OnFailure, fmt.Sprintf("operations[%d].onFailure", i)); err != nil {
			return err
		}
		if err := validateSuccessActions(op.OnSuccess, fmt.Sprintf("operations[%d].onSuccess", i)); err != nil {
			return err
		}
	}

	wfIDs := make(map[string]bool)
	for i, wf := range d.Workflows {
		if wf == nil {
			return fmt.Errorf("workflows[%d] is nil", i)
		}
		if wf.WorkflowID == "" {
			return fmt.Errorf("workflows[%d].workflowId is required", i)
		}
		if wfIDs[wf.WorkflowID] {
			return fmt.Errorf("duplicate workflowId %q", wf.WorkflowID)
		}
		wfIDs[wf.WorkflowID] = true
	}

	trigIDs := make(map[string]bool)
	for i, t := range d.Triggers {
		if t == nil {
			return fmt.Errorf("triggers[%d] is nil", i)
		}
		if t.TriggerID == "" {
			return fmt.Errorf("triggers[%d].triggerId is required", i)
		}
		if trigIDs[t.TriggerID] {
			return fmt.Errorf("duplicate triggerId %q", t.TriggerID)
		}
		trigIDs[t.TriggerID] = true
	}

	return nil
}

func validateOperationFields(op *Operation, idx int) error {
	switch op.ServiceType {
	case "http":
		if op.Method == "" {
			return fmt.Errorf("operations[%d] (%s): http operations require method", idx, op.OperationID)
		}
		if !isValidHTTPMethod(op.Method) {
			return fmt.Errorf("operations[%d] (%s): invalid http method %q", idx, op.OperationID, op.Method)
		}
	case "ssh", "cmd":
		if op.Command == "" {
			return fmt.Errorf("operations[%d] (%s): %s operations require command", idx, op.OperationID, op.ServiceType)
		}
	case "fnct":
		if op.Function == "" {
			return fmt.Errorf("operations[%d] (%s): fnct operations require function", idx, op.OperationID)
		}
	}
	return nil
}

func isValidHTTPMethod(method string) bool {
	switch strings.ToUpper(method) {
	case "GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS", "TRACE":
		return true
	}
	return false
}

var validCriterionTypes = map[CriterionExpressionType]bool{
	CriterionSimple:   true,
	CriterionRegex:    true,
	CriterionJSONPath: true,
	CriterionXPath:    true,
}

func validateCriteria(criteria []*Criterion, path string) error {
	for i, c := range criteria {
		if c == nil {
			return fmt.Errorf("%s[%d] is nil", path, i)
		}
		if c.Condition == "" {
			return fmt.Errorf("%s[%d].condition is required", path, i)
		}
		if c.Type != "" && !validCriterionTypes[c.Type] {
			return fmt.Errorf("%s[%d].type %q is not valid (must be simple, regex, jsonpath, or xpath)", path, i, c.Type)
		}
	}
	return nil
}

var validFailureActionTypes = map[string]bool{
	"end": true, "goto": true, "retry": true,
}

func validateFailureActions(actions []*FailureAction, path string) error {
	for i, a := range actions {
		if a == nil {
			return fmt.Errorf("%s[%d] is nil", path, i)
		}
		if a.Name == "" {
			return fmt.Errorf("%s[%d].name is required", path, i)
		}
		if a.Type == "" {
			return fmt.Errorf("%s[%d].type is required", path, i)
		}
		if !validFailureActionTypes[a.Type] {
			return fmt.Errorf("%s[%d].type %q is not valid (must be end, goto, or retry)", path, i, a.Type)
		}
		if a.Type == "goto" && a.WorkflowID == "" && a.StepID == "" {
			return fmt.Errorf("%s[%d]: goto requires workflowId or stepId", path, i)
		}
		if a.Type == "retry" && a.RetryLimit <= 0 {
			return fmt.Errorf("%s[%d]: retry requires retryLimit > 0", path, i)
		}
		if err := validateCriteria(a.Criteria, fmt.Sprintf("%s[%d].criteria", path, i)); err != nil {
			return err
		}
	}
	return nil
}

var validSuccessActionTypes = map[string]bool{
	"end": true, "goto": true,
}

func validateSuccessActions(actions []*SuccessAction, path string) error {
	for i, a := range actions {
		if a == nil {
			return fmt.Errorf("%s[%d] is nil", path, i)
		}
		if a.Name == "" {
			return fmt.Errorf("%s[%d].name is required", path, i)
		}
		if a.Type == "" {
			return fmt.Errorf("%s[%d].type is required", path, i)
		}
		if !validSuccessActionTypes[a.Type] {
			return fmt.Errorf("%s[%d].type %q is not valid (must be end or goto)", path, i, a.Type)
		}
		if a.Type == "goto" && a.WorkflowID == "" && a.StepID == "" {
			return fmt.Errorf("%s[%d]: goto requires workflowId or stepId", path, i)
		}
		if err := validateCriteria(a.Criteria, fmt.Sprintf("%s[%d].criteria", path, i)); err != nil {
			return err
		}
	}
	return nil
}
