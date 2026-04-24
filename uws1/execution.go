package uws1

import (
	"context"
	"fmt"
	"strings"
	"sync"
)

// Runtime defines the interface that specialized executors must implement
// to provide leaf operation execution and expression evaluation for a
// UWS document.
type Runtime interface {
	// ExecuteLeaf executes a single leaf operation.
	ExecuteLeaf(ctx context.Context, op *Operation) error

	// EvaluateExpression evaluates a UWS runtime expression against the
	// current execution context.
	EvaluateExpression(ctx context.Context, expr string) (any, error)

	// ResolveItems resolves the items/forEach expression for iterative constructs.
	ResolveItems(ctx context.Context, itemsExpr string) ([]any, error)
}

// Orchestrator provides the abstract orchestration logic for walking the
// workflow graph and managing structural state transitions.
type Orchestrator struct {
	Document *Document
	Runtime  Runtime

	opIndex        map[string]*Operation
	workflowIndex  map[string]*Workflow
	stepIndex      map[string]*Step
	parallelGroups map[string][]string
	mu             sync.Mutex
	records        map[string]ExecutionRecord
	inFlight       map[string]chan struct{}
}

// NewOrchestrator creates a new Orchestrator for the given document and runtime.
func NewOrchestrator(doc *Document, runtime Runtime) *Orchestrator {
	o := &Orchestrator{
		Document:       doc,
		Runtime:        runtime,
		opIndex:        make(map[string]*Operation),
		workflowIndex:  make(map[string]*Workflow),
		stepIndex:      make(map[string]*Step),
		parallelGroups: make(map[string][]string),
		records:        make(map[string]ExecutionRecord),
		inFlight:       make(map[string]chan struct{}),
	}
	if doc != nil {
		for _, op := range doc.Operations {
			if op != nil && op.OperationID != "" {
				o.opIndex[op.OperationID] = op
				if op.ParallelGroup != "" {
					o.parallelGroups[op.ParallelGroup] = append(o.parallelGroups[op.ParallelGroup], op.OperationID)
				}
			}
		}
		for _, wf := range doc.Workflows {
			if wf != nil && wf.WorkflowID != "" {
				o.workflowIndex[wf.WorkflowID] = wf
				o.indexSteps(wf.Steps)
				o.indexSteps(wf.Default)
				for _, c := range wf.Cases {
					if c != nil {
						o.indexSteps(c.Steps)
					}
				}
			}
		}
	}
	return o
}

func (o *Orchestrator) indexSteps(steps []*Step) {
	for _, step := range steps {
		if step == nil {
			continue
		}
		if step.StepID != "" {
			o.stepIndex[step.StepID] = step
			if step.ParallelGroup != "" {
				o.parallelGroups[step.ParallelGroup] = append(o.parallelGroups[step.ParallelGroup], step.StepID)
			}
		}
		o.indexSteps(step.Steps)
		o.indexSteps(step.Default)
		for _, c := range step.Cases {
			if c != nil {
				o.indexSteps(c.Steps)
			}
		}
	}
}

// Execute executes the main workflow of the document.
func (o *Orchestrator) Execute(ctx context.Context) error {
	if o.Document == nil {
		return nil
	}
	start := func(ctx context.Context) error {
		if wf, err := o.entryWorkflow(); err != nil {
			return err
		} else if wf != nil {
			return o.ExecuteWorkflow(ctx, wf)
		}
		for _, op := range o.Document.Operations {
			if err := o.executeOperationByID(ctx, op.OperationID); err != nil {
				return err
			}
		}
		return nil
	}
	err := o.executeWithSignals(ctx, start)
	o.Document.setExecutionRecords(o.snapshotRecords())
	return err
}

func (o *Orchestrator) executeWithSignals(ctx context.Context, start func(context.Context) error) error {
	for {
		err := start(ctx)
		switch typed := err.(type) {
		case nil:
			return nil
		case *endSignal:
			return nil
		case *gotoSignal:
			target, targetErr := o.gotoTarget(ctx, typed)
			if targetErr != nil {
				return targetErr
			}
			start = target
		default:
			return err
		}
	}
}

// ExecuteWorkflow executes a structural workflow.
func (o *Orchestrator) ExecuteWorkflow(ctx context.Context, wf *Workflow) error {
	if wf == nil {
		return nil
	}
	return o.executeRunnable(ctx, workflowKey(wf.WorkflowID), wf.WorkflowID, "workflow:"+wf.Type, wf.WorkflowID, wf.DependsOn, wf.When, wf.ForEach, wf.Outputs, func(ctx context.Context) error {
		return o.executeStructural(ctx, wf.Type, wf.DependsOn, wf.Steps, wf.Cases, wf.Default, wf.Items, wf.Mode, wf.BatchSize, wf.Wait, workflowKey(wf.WorkflowID))
	})
}

// ExecuteStep executes a single step.
func (o *Orchestrator) ExecuteStep(ctx context.Context, step *Step) error {
	if step == nil {
		return nil
	}
	responseID := step.StepID
	if strings.TrimSpace(step.OperationRef) != "" {
		responseID = strings.TrimSpace(step.OperationRef)
	} else if strings.TrimSpace(step.Workflow) != "" {
		responseID = strings.TrimSpace(step.Workflow)
	}
	return o.executeRunnable(ctx, stepKey(step.StepID), step.StepID, "step:"+step.Type, responseID, step.DependsOn, step.When, step.ForEach, step.Outputs, func(ctx context.Context) error {
		if step.OperationRef != "" {
			return o.executeOperationByID(ctx, step.OperationRef)
		}
		if step.Workflow != "" {
			return o.executeWorkflowByID(ctx, step.Workflow)
		}
		return o.executeStructural(ctx, step.Type, step.DependsOn, step.Steps, step.Cases, step.Default, step.Items, step.Mode, step.BatchSize, step.Wait, stepKey(step.StepID))
	})
}

func (o *Orchestrator) executeWorkflowByID(ctx context.Context, workflowID string) error {
	wf := o.workflowIndex[workflowID]
	if wf == nil {
		return fmt.Errorf("uws1: workflow %q not found", workflowID)
	}
	return o.ExecuteWorkflow(ctx, wf)
}

func (o *Orchestrator) executeOperationByID(ctx context.Context, operationID string) error {
	op := o.opIndex[operationID]
	if op == nil {
		return fmt.Errorf("uws1: operation %q not found", operationID)
	}
	return o.executeRunnable(ctx, operationKey(op.OperationID), op.OperationID, "operation", op.OperationID, op.DependsOn, op.When, op.ForEach, op.Outputs, func(ctx context.Context) error {
		return o.executeOperation(ctx, op)
	})
}
