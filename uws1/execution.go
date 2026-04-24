package uws1

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/antchfx/xmlquery"
	"github.com/antchfx/xpath"
	"github.com/tabilet/uws/flowcore"
	"golang.org/x/sync/errgroup"
)

const awaitPollInterval = 200 * time.Millisecond

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
	}
	return o.executeRunnable(ctx, stepKey(step.StepID), step.StepID, "step:"+step.Type, responseID, step.DependsOn, step.When, step.ForEach, step.Outputs, func(ctx context.Context) error {
		if step.OperationRef != "" {
			return o.executeOperationByID(ctx, step.OperationRef)
		}
		return o.executeStructural(ctx, step.Type, step.DependsOn, step.Steps, step.Cases, step.Default, step.Items, step.Mode, step.BatchSize, step.Wait, stepKey(step.StepID))
	})
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

func (o *Orchestrator) executeRunnable(ctx context.Context, key, id, kind, responseID string, deps []string, whenExpr, forEachExpr string, outputs map[string]string, run func(context.Context) error) error {
	execKey := o.keyForContext(ctx, key)
	return o.executeOnce(ctx, execKey, id, kind, func(ctx context.Context) error {
		if err := o.executeDependencies(ctx, deps); err != nil {
			return err
		}
		if whenExpr != "" {
			ok, err := o.evaluateTruthy(ctx, whenExpr)
			if err != nil {
				return fmt.Errorf("evaluating when condition for %q: %w", id, err)
			}
			if !ok {
				o.setRecord(execKey, ExecutionRecord{ID: id, Kind: kind, Status: "skipped"})
				return nil
			}
		}
		if forEachExpr != "" {
			items, err := o.Runtime.ResolveItems(ctx, forEachExpr)
			if err != nil {
				return fmt.Errorf("resolving forEach for %q: %w", id, err)
			}
			for index, item := range items {
				itemCtx := o.withIterationContext(ctx, item, index, nil, -1)
				if err := run(itemCtx); err != nil {
					return err
				}
			}
			if len(items) == 0 {
				o.setRecord(execKey, ExecutionRecord{ID: id, Kind: kind, Status: "success", Result: []any{}})
			}
			return nil
		}
		if err := run(ctx); err != nil {
			return err
		}
		if len(outputs) == 0 {
			return nil
		}
		outputsCtx := o.withRecordContext(ctx)
		resolved, err := o.resolveOutputs(outputsCtx, execKey, id, kind, responseID, outputs)
		if err != nil {
			return err
		}
		o.mu.Lock()
		record := o.records[execKey]
		record.Outputs = resolved
		o.records[execKey] = record
		o.mu.Unlock()
		return nil
	})
}

func (o *Orchestrator) executeStructural(ctx context.Context, typeName string, deps []string, steps []*Step, cases []*Case, defaultSteps []*Step, itemsExpr, mode, batchSizeExpr, waitExpr, key string) error {
	switch typeName {
	case flowcore.WorkflowTypeSequence:
		return o.executeSteps(ctx, steps)
	case flowcore.WorkflowTypeParallel:
		return o.executeStepsParallel(ctx, steps)
	case flowcore.WorkflowTypeSwitch:
		return o.executeSwitch(ctx, cases, defaultSteps)
	case flowcore.WorkflowTypeMerge:
		return o.executeMerge(ctx, deps, key)
	case flowcore.WorkflowTypeLoop:
		return o.executeLoop(ctx, steps, itemsExpr, batchSizeExpr, key)
	case flowcore.WorkflowTypeAwait:
		return o.executeAwait(ctx, steps, waitExpr)
	default:
		return fmt.Errorf("uws1: unsupported workflow type %q", typeName)
	}
}

func (o *Orchestrator) executeOnce(ctx context.Context, key, id, kind string, run func(context.Context) error) error {
	o.mu.Lock()
	if record, ok := o.records[key]; ok && record.Status != "running" {
		o.mu.Unlock()
		if record.Status == "error" && record.Error != "" {
			return errors.New(record.Error)
		}
		return nil
	}
	if ch, ok := o.inFlight[key]; ok {
		o.mu.Unlock()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ch:
		}
		o.mu.Lock()
		record := o.records[key]
		o.mu.Unlock()
		if record.Status == "error" && record.Error != "" {
			return errors.New(record.Error)
		}
		return nil
	}
	ch := make(chan struct{})
	o.inFlight[key] = ch
	o.records[key] = ExecutionRecord{ID: id, Kind: kind, Status: "running"}
	o.mu.Unlock()

	err := run(o.withRecordContext(ctx))

	o.mu.Lock()
	record := o.records[key]
	switch {
	case err == nil:
		if record.Status == "running" {
			record.Status = "success"
		}
	case isControlSignal(err):
		if record.Status == "running" {
			record.Status = "success"
		}
	default:
		record.Status = "error"
		record.Error = err.Error()
	}
	o.records[key] = record
	delete(o.inFlight, key)
	close(ch)
	o.mu.Unlock()

	return err
}

func (o *Orchestrator) executeDependencies(ctx context.Context, deps []string) error {
	for _, dep := range deps {
		if dep == "" {
			continue
		}
		if members := o.parallelGroups[dep]; len(members) > 0 {
			for _, member := range members {
				if err := o.executeDependency(ctx, member); err != nil {
					return err
				}
			}
			continue
		}
		if err := o.executeDependency(ctx, dep); err != nil {
			return err
		}
	}
	return nil
}

func (o *Orchestrator) executeDependency(ctx context.Context, name string) error {
	if step := o.stepIndex[name]; step != nil {
		return o.ExecuteStep(ctx, step)
	}
	if wf := o.workflowIndex[name]; wf != nil {
		return o.ExecuteWorkflow(ctx, wf)
	}
	if op := o.opIndex[name]; op != nil {
		return o.executeOperationByID(ctx, op.OperationID)
	}
	return fmt.Errorf("uws1: unknown dependency %q", name)
}

func (o *Orchestrator) entryWorkflow() (*Workflow, error) {
	return executableEntryWorkflow(o.Document)
}

// executeSteps executes a list of steps sequentially.
func (o *Orchestrator) executeSteps(ctx context.Context, steps []*Step) error {
	for _, step := range steps {
		if err := o.ExecuteStep(ctx, step); err != nil {
			return err
		}
	}
	return nil
}

// executeStepsParallel executes a list of steps concurrently.
func (o *Orchestrator) executeStepsParallel(ctx context.Context, steps []*Step) error {
	group, groupCtx := errgroup.WithContext(ctx)
	for _, step := range steps {
		step := step
		group.Go(func() error {
			return o.ExecuteStep(groupCtx, step)
		})
	}
	return group.Wait()
}

// executeSwitch executes a switch construct.
func (o *Orchestrator) executeSwitch(ctx context.Context, cases []*Case, defaultSteps []*Step) error {
	for _, c := range cases {
		if c == nil {
			continue
		}
		if c.When == "" {
			return o.executeSteps(ctx, c.Steps)
		}
		matched, err := o.evaluateTruthy(ctx, c.When)
		if err != nil {
			return fmt.Errorf("evaluating switch case %q condition: %w", c.Name, err)
		}
		if matched {
			return o.executeSteps(ctx, c.Steps)
		}
	}
	if len(defaultSteps) > 0 {
		return o.executeSteps(ctx, defaultSteps)
	}
	return nil
}

func (o *Orchestrator) executeMerge(ctx context.Context, deps []string, key string) error {
	result := o.mergeDependencyRecords(deps)
	o.mu.Lock()
	record := o.records[key]
	record.Result = result
	record.Status = "success"
	o.records[key] = record
	o.mu.Unlock()
	return nil
}

func (o *Orchestrator) mergeDependencyRecords(deps []string) []map[string]any {
	if len(deps) == 0 {
		return nil
	}
	ordered := make([]string, 0, len(deps))
	seen := make(map[string]struct{})
	for _, dep := range deps {
		if dep == "" {
			continue
		}
		if members := o.parallelGroups[dep]; len(members) > 0 {
			for _, member := range members {
				if _, ok := seen[member]; ok {
					continue
				}
				ordered = append(ordered, member)
				seen[member] = struct{}{}
			}
			continue
		}
		if _, ok := seen[dep]; ok {
			continue
		}
		ordered = append(ordered, dep)
		seen[dep] = struct{}{}
	}

	o.mu.Lock()
	defer o.mu.Unlock()
	out := make([]map[string]any, 0, len(ordered))
	for _, dep := range ordered {
		keys := o.recordKeysForDependencyLocked(dep)
		for _, key := range keys {
			record := o.records[key]
			out = append(out, map[string]any{
				"id":     record.ID,
				"kind":   record.Kind,
				"status": record.Status,
				"error":  record.Error,
				"result": record.Result,
				"outputs": record.Outputs,
			})
		}
	}
	return out
}

func (o *Orchestrator) recordKeysForDependencyLocked(dep string) []string {
	baseKeys := make([]string, 0, 1)
	switch {
	case o.stepIndex[dep] != nil:
		baseKeys = append(baseKeys, stepKey(dep))
	case o.workflowIndex[dep] != nil:
		baseKeys = append(baseKeys, workflowKey(dep))
	case o.opIndex[dep] != nil:
		baseKeys = append(baseKeys, operationKey(dep))
	default:
		return nil
	}
	var matches []string
	for _, base := range baseKeys {
		for key := range o.records {
			if key == base || len(key) > len(base) && key[:len(base)] == base && key[len(base)] == '#' {
				matches = append(matches, key)
			}
		}
	}
	sort.Strings(matches)
	return matches
}

// executeLoop executes a loop construct.
func (o *Orchestrator) executeLoop(ctx context.Context, steps []*Step, itemsExpr, batchSizeExpr, key string) error {
	items, err := o.Runtime.ResolveItems(ctx, itemsExpr)
	if err != nil {
		return fmt.Errorf("resolving loop items: %w", err)
	}
	batchSize, err := o.resolveBatchSize(ctx, batchSizeExpr)
	if err != nil {
		return err
	}
	if batchSize <= 0 {
		batchSize = len(items)
		if batchSize == 0 {
			batchSize = 1
		}
	}

	var results []map[string]any
	for batchIndex, start := 0, 0; start < len(items); batchIndex, start = batchIndex+1, start+batchSize {
		end := start + batchSize
		if end > len(items) {
			end = len(items)
		}
		batch := append([]any(nil), items[start:end]...)
		for i, item := range batch {
			itemCtx := o.withIterationContext(ctx, item, start+i, batch, batchIndex)
			if err := o.executeSteps(itemCtx, steps); err != nil {
				return err
			}
			results = append(results, map[string]any{
				"index":      start + i,
				"batchIndex": batchIndex,
				"item":       item,
			})
		}
	}
	o.mu.Lock()
	record := o.records[key]
	record.Result = results
	record.Status = "success"
	o.records[key] = record
	o.mu.Unlock()
	return nil
}

func (o *Orchestrator) resolveBatchSize(ctx context.Context, batchSizeExpr string) (int, error) {
	if batchSizeExpr == "" {
		return 0, nil
	}
	value, err := o.Runtime.EvaluateExpression(ctx, batchSizeExpr)
	if err != nil {
		return 0, fmt.Errorf("evaluating batchSize: %w", err)
	}
	switch typed := value.(type) {
	case int:
		if typed <= 0 {
			return 0, fmt.Errorf("batchSize must resolve to a positive integer")
		}
		return typed, nil
	case int64:
		if typed <= 0 {
			return 0, fmt.Errorf("batchSize must resolve to a positive integer")
		}
		return int(typed), nil
	case float64:
		if typed <= 0 || math.Trunc(typed) != typed {
			return 0, fmt.Errorf("batchSize must resolve to a positive integer")
		}
		return int(typed), nil
	default:
		return 0, fmt.Errorf("batchSize must resolve to a positive integer")
	}
}

func (o *Orchestrator) executeAwait(ctx context.Context, steps []*Step, waitExpr string) error {
	ticker := time.NewTicker(awaitPollInterval)
	defer ticker.Stop()
	for {
		ok, err := o.evaluateTruthy(ctx, waitExpr)
		if err != nil {
			return fmt.Errorf("evaluating await wait expression: %w", err)
		}
		if ok {
			return o.executeSteps(ctx, steps)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

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

func (o *Orchestrator) criteriaMatchAll(ctx context.Context, criteria []*Criterion) (bool, error) {
	if len(criteria) == 0 {
		return true, nil
	}
	for _, criterion := range criteria {
		if criterion == nil {
			continue
		}
		ok, err := o.evaluateCriterion(ctx, criterion)
		if err != nil {
			return false, fmt.Errorf("evaluating criterion %q (%s): %w", criterion.Condition, criterion.Type, err)
		}
		if !ok {
			return false, nil
		}
	}
	return true, nil
}

func (o *Orchestrator) evaluateCriterion(ctx context.Context, criterion *Criterion) (bool, error) {
	if criterion == nil {
		return true, nil
	}
	switch criterion.Type {
	case "", CriterionSimple:
		return o.evaluateTruthy(ctx, criterion.Condition)
	case CriterionRegex:
		return o.evaluateRegexCriterion(ctx, criterion)
	case CriterionJSONPath:
		return o.evaluateJSONPathCriterion(ctx, criterion)
	case CriterionXPath:
		return o.evaluateXPathCriterion(ctx, criterion)
	default:
		return false, fmt.Errorf("uws1: criterion type %q is not executable by the core orchestrator", criterion.Type)
	}
}

func (o *Orchestrator) evaluateRegexCriterion(ctx context.Context, criterion *Criterion) (bool, error) {
	source, err := o.Runtime.EvaluateExpression(ctx, criterion.Context)
	if err != nil {
		return false, fmt.Errorf("evaluating regex context %q: %w", criterion.Context, err)
	}
	pattern := strings.TrimSpace(criterion.Condition)
	re, err := regexp.Compile(pattern)
	if err != nil {
		return false, fmt.Errorf("compile regex %q: %w", pattern, err)
	}
	text, err := criterionSourceString(source)
	if err != nil {
		return false, err
	}
	return re.MatchString(text), nil
}

func (o *Orchestrator) evaluateJSONPathCriterion(ctx context.Context, criterion *Criterion) (bool, error) {
	source, err := o.Runtime.EvaluateExpression(ctx, criterion.Context)
	if err != nil {
		return false, fmt.Errorf("evaluating jsonpath context %q: %w", criterion.Context, err)
	}
	target := normalizeCriterionTarget(criterion.Context, criterion.Condition)
	switch {
	case target == "":
		return truthyValue(source), nil
	case strings.HasPrefix(target, "#"):
		value, err := resolveCriterionJSONPointer(source, target)
		if err != nil {
			return false, err
		}
		return truthyValue(value), nil
	case strings.HasPrefix(target, "/"):
		value, err := resolveCriterionJSONPointer(source, "#"+target)
		if err != nil {
			return false, err
		}
		return truthyValue(value), nil
	default:
		value, err := o.Runtime.EvaluateExpression(ctx, criterion.Condition)
		if err != nil {
			return false, fmt.Errorf("evaluating jsonpath condition %q: %w", criterion.Condition, err)
		}
		return truthyValue(value), nil
	}
}

func (o *Orchestrator) evaluateXPathCriterion(ctx context.Context, criterion *Criterion) (bool, error) {
	source, err := o.Runtime.EvaluateExpression(ctx, criterion.Context)
	if err != nil {
		return false, fmt.Errorf("evaluating xpath context %q: %w", criterion.Context, err)
	}
	xmlText, err := criterionSourceString(source)
	if err != nil {
		return false, err
	}
	root, err := xmlquery.Parse(strings.NewReader(strings.TrimSpace(xmlText)))
	if err != nil {
		return false, fmt.Errorf("parse xpath XML: %w", err)
	}
	target := normalizeCriterionTarget(criterion.Context, criterion.Condition)
	if target == "" {
		return truthyValue(xmlText), nil
	}
	compiled, err := xpath.Compile(target)
	if err != nil {
		return false, fmt.Errorf("compile xpath %q: %w", target, err)
	}
	value := compiled.Evaluate(xmlquery.CreateXPathNavigator(root))
	return truthyXPathValue(value), nil
}

func normalizeCriterionTarget(contextExpr, condition string) string {
	contextExpr = strings.TrimSpace(contextExpr)
	condition = strings.TrimSpace(condition)
	if contextExpr != "" && strings.HasPrefix(condition, contextExpr) {
		return strings.TrimSpace(strings.TrimPrefix(condition, contextExpr))
	}
	return condition
}

func criterionSourceString(value any) (string, error) {
	switch typed := value.(type) {
	case nil:
		return "", nil
	case string:
		return typed, nil
	case []byte:
		return string(typed), nil
	default:
		if data, err := json.Marshal(typed); err == nil {
			return string(data), nil
		}
		return fmt.Sprint(typed), nil
	}
}

func truthyValue(value any) bool {
	switch typed := value.(type) {
	case nil:
		return false
	case bool:
		return typed
	case string:
		return typed != ""
	case int:
		return typed != 0
	case int64:
		return typed != 0
	case float64:
		return typed != 0
	case []any:
		return len(typed) > 0
	case map[string]any:
		return len(typed) > 0
	default:
		return true
	}
}

func resolveCriterionJSONPointer(root any, pointer string) (any, error) {
	if pointer == "" || pointer == "#" {
		return root, nil
	}
	if !strings.HasPrefix(pointer, "#") {
		return nil, fmt.Errorf("invalid JSON pointer %q", pointer)
	}
	path := strings.TrimPrefix(pointer, "#")
	if path == "" {
		return root, nil
	}
	if !strings.HasPrefix(path, "/") {
		return nil, fmt.Errorf("invalid JSON pointer %q", pointer)
	}
	current := root
	for _, rawToken := range strings.Split(path[1:], "/") {
		token := strings.ReplaceAll(strings.ReplaceAll(rawToken, "~1", "/"), "~0", "~")
		switch typed := current.(type) {
		case map[string]any:
			current = typed[token]
		case []any:
			index, err := parseCriterionIndex(token)
			if err != nil {
				return nil, err
			}
			if index < 0 || index >= len(typed) {
				return nil, nil
			}
			current = typed[index]
		default:
			return nil, nil
		}
	}
	return current, nil
}

func parseCriterionIndex(token string) (int, error) {
	var index int
	if _, err := fmt.Sscanf(token, "%d", &index); err != nil {
		return 0, fmt.Errorf("invalid array index %q", token)
	}
	return index, nil
}

func truthyXPathValue(value any) bool {
	switch typed := value.(type) {
	case nil:
		return false
	case bool:
		return typed
	case string:
		return typed != ""
	case float64:
		return typed != 0
	case *xpath.NodeIterator:
		if typed == nil {
			return false
		}
		return typed.MoveNext()
	default:
		return truthyValue(value)
	}
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

func (o *Orchestrator) evaluateTruthy(ctx context.Context, expr string) (bool, error) {
	value, err := o.Runtime.EvaluateExpression(ctx, expr)
	if err != nil {
		return false, err
	}
	switch typed := value.(type) {
	case bool:
		return typed, nil
	case string:
		return typed != "", nil
	case nil:
		return false, nil
	case int:
		return typed != 0, nil
	case int64:
		return typed != 0, nil
	case float64:
		return typed != 0, nil
	default:
		return true, nil
	}
}

func (o *Orchestrator) withRecordContext(ctx context.Context) context.Context {
	state, ok := ExecutionContextFromContext(ctx)
	if !ok || state == nil {
		state = &ExecutionContext{}
	}
	state.Iteration = cloneIteration(state.Iteration)
	state.Records = o.snapshotRecords()
	state.Current = cloneCurrentExecution(state.Current)
	return WithExecutionContext(ctx, state)
}

func (o *Orchestrator) withIterationContext(ctx context.Context, item any, index int, batch []any, batchIndex int) context.Context {
	state, ok := ExecutionContextFromContext(ctx)
	if !ok || state == nil {
		state = &ExecutionContext{}
	} else {
		state = &ExecutionContext{
			Iteration: cloneIteration(state.Iteration),
			Records:   state.Records,
			Current:   cloneCurrentExecution(state.Current),
		}
	}
	state.Iteration = &IterationContext{
		Item:       item,
		Index:      index,
		Batch:      append([]any(nil), batch...),
		BatchIndex: batchIndex,
	}
	if state.Records == nil {
		state.Records = o.snapshotRecords()
	}
	return WithExecutionContext(ctx, state)
}

func cloneIteration(iteration *IterationContext) *IterationContext {
	if iteration == nil {
		return nil
	}
	return &IterationContext{
		Item:       iteration.Item,
		Index:      iteration.Index,
		Batch:      append([]any(nil), iteration.Batch...),
		BatchIndex: iteration.BatchIndex,
	}
}

func (o *Orchestrator) snapshotRecords() map[string]ExecutionRecord {
	o.mu.Lock()
	defer o.mu.Unlock()
	out := make(map[string]ExecutionRecord, len(o.records))
	for key, record := range o.records {
		out[key] = cloneExecutionRecord(record)
	}
	return out
}

func (o *Orchestrator) setRecord(key string, record ExecutionRecord) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.records[key] = cloneExecutionRecord(record)
}

func (o *Orchestrator) withCurrentExecutionContext(ctx context.Context, key, id, kind, responseID string, outputs map[string]any) context.Context {
	state, ok := ExecutionContextFromContext(ctx)
	if !ok || state == nil {
		state = &ExecutionContext{}
	} else {
		state = &ExecutionContext{
			Iteration: cloneIteration(state.Iteration),
			Records:   state.Records,
			Current:   cloneCurrentExecution(state.Current),
		}
	}
	state.Current = &CurrentExecutionContext{
		Key:        key,
		ID:         id,
		Kind:       kind,
		ResponseID: responseID,
		Outputs:    cloneMapAny(outputs),
	}
	if state.Records == nil {
		state.Records = o.snapshotRecords()
	}
	return WithExecutionContext(ctx, state)
}

func (o *Orchestrator) resolveOutputs(ctx context.Context, key, id, kind, responseID string, definitions map[string]string) (map[string]any, error) {
	if len(definitions) == 0 {
		return nil, nil
	}
	names := make([]string, 0, len(definitions))
	for name := range definitions {
		if strings.TrimSpace(name) == "" {
			continue
		}
		names = append(names, name)
	}
	sort.Strings(names)
	if len(names) == 0 {
		return nil, nil
	}
	resolved := make(map[string]any, len(names))
	for _, name := range names {
		expr := strings.TrimSpace(definitions[name])
		if expr == "" {
			continue
		}
		exprCtx := o.withCurrentExecutionContext(ctx, key, id, kind, responseID, resolved)
		value, err := o.Runtime.EvaluateExpression(exprCtx, expr)
		if err != nil {
			return nil, fmt.Errorf("evaluating output %q for %s %q: %w", name, kind, id, err)
		}
		resolved[name] = value
	}
	return resolved, nil
}

func cloneMapAny(src map[string]any) map[string]any {
	if len(src) == 0 {
		return nil
	}
	out := make(map[string]any, len(src))
	for key, value := range src {
		out[key] = value
	}
	return out
}

func (o *Orchestrator) keyForContext(ctx context.Context, key string) string {
	state, ok := ExecutionContextFromContext(ctx)
	if !ok || state == nil || state.Iteration == nil {
		return key
	}
	return fmt.Sprintf("%s#iter:%d", key, state.Iteration.Index)
}

func operationKey(id string) string { return "op:" + id }
func workflowKey(id string) string  { return "wf:" + id }
func stepKey(id string) string      { return "step:" + id }

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
