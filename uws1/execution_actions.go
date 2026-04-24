package uws1

import (
	"context"
	"fmt"
	"time"
)

func (o *Orchestrator) executeOperation(ctx context.Context, op *Operation) error {
	attempts := 0
	for {
		err := o.Runtime.ExecuteLeaf(ctx, op)
		opCtx := o.withCurrentExecutionContext(o.withRecordContext(ctx), operationKey(op.OperationID), op.OperationID, "operation", op.OperationID, nil)
		if err == nil {
			ok, critErr := o.criteriaMatchAll(opCtx, op.SuccessCriteria)
			if critErr != nil {
				return critErr
			}
			if len(op.SuccessCriteria) > 0 && !ok {
				err = fmt.Errorf("operation %q did not satisfy successCriteria", op.OperationID)
			}
		}

		if err != nil {
			action, actionErr := o.firstMatchingFailureAction(opCtx, op.OnFailure)
			if actionErr != nil {
				return actionErr
			}
			if action == nil {
				return err
			}
			switch action.Type {
			case "retry":
				if attempts >= action.RetryLimit {
					return err
				}
				attempts++
				if action.RetryAfter > 0 {
					timer := time.NewTimer(time.Duration(action.RetryAfter * float64(time.Second)))
					select {
					case <-ctx.Done():
						timer.Stop()
						return ctx.Err()
					case <-timer.C:
					}
				}
				continue
			case "goto":
				return &gotoSignal{workflowID: action.WorkflowID, stepID: action.StepID}
			case "end":
				return &endSignal{}
			default:
				return fmt.Errorf("uws1: unsupported failure action type %q", action.Type)
			}
		}

		action, actionErr := o.firstMatchingSuccessAction(opCtx, op.OnSuccess)
		if actionErr != nil {
			return actionErr
		}
		if action == nil {
			return nil
		}
		switch action.Type {
		case "goto":
			return &gotoSignal{workflowID: action.WorkflowID, stepID: action.StepID}
		case "end":
			return &endSignal{}
		default:
			return fmt.Errorf("uws1: unsupported success action type %q", action.Type)
		}
	}
}

func (o *Orchestrator) firstMatchingFailureAction(ctx context.Context, actions []*FailureAction) (*FailureAction, error) {
	for _, action := range actions {
		if action == nil {
			continue
		}
		match, err := o.criteriaMatchAll(ctx, action.Criteria)
		if err != nil {
			return nil, err
		}
		if match {
			return action, nil
		}
	}
	return nil, nil
}

func (o *Orchestrator) firstMatchingSuccessAction(ctx context.Context, actions []*SuccessAction) (*SuccessAction, error) {
	for _, action := range actions {
		if action == nil {
			continue
		}
		match, err := o.criteriaMatchAll(ctx, action.Criteria)
		if err != nil {
			return nil, err
		}
		if match {
			return action, nil
		}
	}
	return nil, nil
}

func (o *Orchestrator) gotoTarget(ctx context.Context, signal *gotoSignal) (func(context.Context) error, error) {
	if signal == nil {
		return nil, fmt.Errorf("uws1: goto target is required")
	}
	if signal.stepID != "" {
		step := o.stepIndex[signal.stepID]
		if step == nil {
			return nil, fmt.Errorf("uws1: goto step target %q not found", signal.stepID)
		}
		return func(ctx context.Context) error { return o.ExecuteStep(ctx, step) }, nil
	}
	if signal.workflowID != "" {
		workflow := o.workflowIndex[signal.workflowID]
		if workflow == nil {
			return nil, fmt.Errorf("uws1: goto workflow target %q not found", signal.workflowID)
		}
		return func(ctx context.Context) error { return o.ExecuteWorkflow(ctx, workflow) }, nil
	}
	return nil, fmt.Errorf("uws1: goto target is required")
}

type gotoSignal struct {
	workflowID string
	stepID     string
}

func (g *gotoSignal) Error() string {
	if g.stepID != "" {
		return "goto step " + g.stepID
	}
	return "goto workflow " + g.workflowID
}

type endSignal struct{}

func (e *endSignal) Error() string { return "end" }

func isControlSignal(err error) bool {
	switch err.(type) {
	case *gotoSignal, *endSignal:
		return true
	default:
		return false
	}
}
