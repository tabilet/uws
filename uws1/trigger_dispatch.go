package uws1

import (
	"context"
	"fmt"
	"sort"
	"strconv"
)

// ExecuteTrigger resolves a trigger event to its routed execution targets.
func (o *Orchestrator) ExecuteTrigger(ctx context.Context, triggerID string, output int, payload any) error {
	if o == nil || o.Document == nil {
		return fmt.Errorf("uws1: trigger dispatch requires a document")
	}
	trigger := o.lookupTrigger(triggerID)
	if trigger == nil {
		return fmt.Errorf("uws1: trigger %q not found", triggerID)
	}
	targets, err := o.resolveTriggerTargets(trigger, output)
	if err != nil {
		return err
	}
	triggerCtx := &TriggerExecutionContext{
		ID:         trigger.TriggerID,
		Output:     output,
		OutputName: triggerOutputName(trigger, output),
		Payload:    payload,
	}
	state, _ := ExecutionContextFromContext(ctx)
	if state == nil {
		state = &ExecutionContext{}
	} else {
		state = &ExecutionContext{
			Iteration: cloneIteration(state.Iteration),
			Trigger:   cloneTriggerContext(state.Trigger),
			Records:   state.Records,
			Current:   cloneCurrentExecution(state.Current),
		}
	}
	state.Trigger = triggerCtx
	return o.executeWithSignals(WithExecutionContext(ctx, state), func(ctx context.Context) error {
		for _, target := range targets {
			if err := o.executeTriggerTarget(ctx, target); err != nil {
				return err
			}
		}
		return nil
	})
}

func (o *Orchestrator) lookupTrigger(triggerID string) *Trigger {
	if o == nil || o.Document == nil {
		return nil
	}
	for _, trigger := range o.Document.Triggers {
		if trigger != nil && trigger.TriggerID == triggerID {
			return trigger
		}
	}
	return nil
}

func (o *Orchestrator) resolveTriggerTargets(trigger *Trigger, output int) ([]string, error) {
	if trigger == nil {
		return nil, fmt.Errorf("uws1: trigger is required")
	}
	outputKey := strconv.Itoa(output)
	targetSet := make(map[string]struct{})
	var targets []string
	for _, route := range trigger.Routes {
		if route == nil || !triggerRouteMatchesOutput(route, trigger, output) {
			continue
		}
		for _, target := range route.To {
			if _, ok := targetSet[target]; ok {
				continue
			}
			if err := o.validateTriggerTarget(target); err != nil {
				return nil, err
			}
			targetSet[target] = struct{}{}
			targets = append(targets, target)
		}
	}
	if len(targets) == 0 {
		return nil, fmt.Errorf("uws1: trigger %q output %q has no routed targets", trigger.TriggerID, outputKey)
	}
	return targets, nil
}

func triggerRouteMatchesOutput(route *TriggerRoute, trigger *Trigger, output int) bool {
	if route == nil {
		return false
	}
	if route.Output == strconv.Itoa(output) {
		return true
	}
	if output < 0 || output >= len(trigger.Outputs) {
		return false
	}
	return resolveTriggerOutput(route.Output, trigger.Outputs, map[string]bool{trigger.Outputs[output]: true}) &&
		trigger.Outputs[output] == route.Output
}

func triggerOutputName(trigger *Trigger, output int) string {
	if trigger == nil || output < 0 || output >= len(trigger.Outputs) {
		return ""
	}
	return trigger.Outputs[output]
}

func (o *Orchestrator) validateTriggerTarget(target string) error {
	if target == "" {
		return fmt.Errorf("uws1: trigger route target is required")
	}
	if o.workflowIndex[target] != nil {
		return nil
	}
	if o.topLevelStepIndex[target] != nil {
		return nil
	}
	return fmt.Errorf("uws1: trigger route target %q must reference a top-level stepId or workflowId", target)
}

func (o *Orchestrator) executeTriggerTarget(ctx context.Context, target string) error {
	if workflow := o.workflowIndex[target]; workflow != nil {
		return o.ExecuteWorkflow(ctx, workflow)
	}
	if step := o.topLevelStepIndex[target]; step != nil {
		return o.ExecuteStep(ctx, step)
	}
	return fmt.Errorf("uws1: trigger route target %q must reference a top-level stepId or workflowId", target)
}

func collectTopLevelStepIDs(d *Document) map[string]bool {
	out := make(map[string]bool)
	entry, err := executableEntryWorkflow(d)
	if err != nil || entry == nil {
		return out
	}
	for _, step := range entry.Steps {
		if step != nil && step.StepID != "" {
			out[step.StepID] = true
		}
	}
	return out
}

func sortedTopLevelStepIDs(d *Document) []string {
	out := make([]string, 0, len(collectTopLevelStepIDs(d)))
	for stepID := range collectTopLevelStepIDs(d) {
		out = append(out, stepID)
	}
	sort.Strings(out)
	return out
}
