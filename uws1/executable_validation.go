package uws1

import "fmt"

// ValidateExecutable checks whether the current core executor can run the
// document without relying on ambiguous identifiers or unsupported semantics.
func (d *Document) ValidateExecutable() error {
	if d == nil {
		return fmt.Errorf("uws1: document is required")
	}
	if err := validateExecutableNames(d); err != nil {
		return err
	}
	if _, err := executableEntryWorkflow(d); err != nil {
		return err
	}
	if err := validateExecutableOperations(d.Operations); err != nil {
		return err
	}
	for _, wf := range d.Workflows {
		if wf == nil {
			continue
		}
		if err := validateExecutableSteps(wf.Steps); err != nil {
			return err
		}
		if err := validateExecutableCases(wf.Cases); err != nil {
			return err
		}
		if err := validateExecutableSteps(wf.Default); err != nil {
			return err
		}
	}
	return nil
}

func validateExecutableNames(d *Document) error {
	seen := make(map[string]string)
	add := func(name, kind string) error {
		if name == "" {
			return nil
		}
		if prev, ok := seen[name]; ok {
			if prev == "parallelGroup" && kind == "parallelGroup" {
				return nil
			}
			return fmt.Errorf("uws1: executable identifier %q is ambiguous between %s and %s", name, prev, kind)
		}
		seen[name] = kind
		return nil
	}
	for _, op := range d.Operations {
		if op == nil {
			continue
		}
		if err := add(op.OperationID, "operation"); err != nil {
			return err
		}
		if err := add(op.ParallelGroup, "parallelGroup"); err != nil {
			return err
		}
	}
	for _, wf := range d.Workflows {
		if wf == nil {
			continue
		}
		if err := add(wf.WorkflowID, "workflow"); err != nil {
			return err
		}
		if err := validateExecutableStepNames(wf.Steps, add); err != nil {
			return err
		}
		if err := validateExecutableStepNames(wf.Default, add); err != nil {
			return err
		}
		for _, c := range wf.Cases {
			if c == nil {
				continue
			}
			if err := validateExecutableStepNames(c.Steps, add); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateExecutableStepNames(steps []*Step, add func(name, kind string) error) error {
	for _, step := range steps {
		if step == nil {
			continue
		}
		if err := add(step.StepID, "step"); err != nil {
			return err
		}
		if err := add(step.ParallelGroup, "parallelGroup"); err != nil {
			return err
		}
		if err := validateExecutableStepNames(step.Steps, add); err != nil {
			return err
		}
		if err := validateExecutableStepNames(step.Default, add); err != nil {
			return err
		}
		for _, c := range step.Cases {
			if c == nil {
				continue
			}
			if err := validateExecutableStepNames(c.Steps, add); err != nil {
				return err
			}
		}
	}
	return nil
}

func executableEntryWorkflow(d *Document) (*Workflow, error) {
	if d == nil || len(d.Workflows) == 0 {
		return nil, nil
	}
	if len(d.Workflows) == 1 {
		return d.Workflows[0], nil
	}
	var main *Workflow
	for _, wf := range d.Workflows {
		if wf != nil && wf.WorkflowID == "main" {
			if main != nil {
				return nil, fmt.Errorf("uws1: multiple workflows use the reserved entry id %q", "main")
			}
			main = wf
		}
	}
	if main == nil {
		return nil, fmt.Errorf("uws1: multiple workflows require an explicit %q entry workflow", "main")
	}
	return main, nil
}

func validateExecutableOperations(ops []*Operation) error {
	for _, op := range ops {
		if op == nil {
			continue
		}
		if err := validateExecutableCriteria(op.SuccessCriteria, fmt.Sprintf("operation %q successCriteria", op.OperationID)); err != nil {
			return err
		}
		for _, action := range op.OnFailure {
			if action == nil {
				continue
			}
			if err := validateExecutableCriteria(action.Criteria, fmt.Sprintf("operation %q onFailure criteria", op.OperationID)); err != nil {
				return err
			}
		}
		for _, action := range op.OnSuccess {
			if action == nil {
				continue
			}
			if err := validateExecutableCriteria(action.Criteria, fmt.Sprintf("operation %q onSuccess criteria", op.OperationID)); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateExecutableSteps(steps []*Step) error {
	for _, step := range steps {
		if step == nil {
			continue
		}
		if step.OperationRef != "" && step.Workflow != "" {
			return fmt.Errorf("uws1: step %q cannot specify both operationRef and workflow", step.StepID)
		}
		if step.Workflow != "" && (step.Type != "" || len(step.Steps) > 0 || len(step.Cases) > 0 || len(step.Default) > 0) {
			return fmt.Errorf("uws1: step %q workflow references cannot also declare structural content", step.StepID)
		}
		if err := validateExecutableSteps(step.Steps); err != nil {
			return err
		}
		if err := validateExecutableCases(step.Cases); err != nil {
			return err
		}
		if err := validateExecutableSteps(step.Default); err != nil {
			return err
		}
	}
	return nil
}

func validateExecutableCases(cases []*Case) error {
	for _, c := range cases {
		if c == nil {
			continue
		}
		if err := validateExecutableSteps(c.Steps); err != nil {
			return err
		}
	}
	return nil
}

func validateExecutableCriteria(criteria []*Criterion, path string) error {
	for _, criterion := range criteria {
		if criterion == nil {
			continue
		}
		switch criterion.Type {
		case "", CriterionSimple, CriterionRegex, CriterionJSONPath, CriterionXPath:
			continue
		default:
			return fmt.Errorf("uws1: %s include unsupported criterion type %q", path, criterion.Type)
		}
	}
	return nil
}
