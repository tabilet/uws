package uws1

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
)

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
			iterationResults := make([]map[string]any, 0, len(items))
			aggregatedOutputs := make(map[string][]any)
			for index, item := range items {
				itemCtx := o.withIterationContext(ctx, item, index, nil, -1)
				itemKey := o.keyForContext(itemCtx, key)
				o.setRecord(itemKey, ExecutionRecord{ID: id, Kind: kind, Status: "running"})
				if err := run(itemCtx); err != nil {
					return err
				}
				var resolved map[string]any
				if len(outputs) > 0 {
					outputsCtx := o.withRecordContext(itemCtx)
					resolved, err = o.resolveOutputs(outputsCtx, itemKey, id, kind, responseID, outputs)
					if err != nil {
						return err
					}
				}
				o.mu.Lock()
				record := o.records[itemKey]
				if record.Status == "running" {
					record.Status = "success"
				}
				if len(resolved) > 0 {
					record.Outputs = resolved
				}
				o.records[itemKey] = record
				o.mu.Unlock()
				iterationResults = append(iterationResults, map[string]any{
					"index":   index,
					"item":    item,
					"status":  record.Status,
					"error":   record.Error,
					"result":  record.Result,
					"outputs": cloneMapAny(record.Outputs),
				})
				for name, value := range resolved {
					aggregatedOutputs[name] = append(aggregatedOutputs[name], value)
				}
			}
			o.mu.Lock()
			record := o.records[execKey]
			record.Result = iterationResults
			record.Status = "success"
			if len(aggregatedOutputs) > 0 {
				record.Outputs = make(map[string]any, len(aggregatedOutputs))
				for name, values := range aggregatedOutputs {
					record.Outputs[name] = append([]any(nil), values...)
				}
			}
			o.records[execKey] = record
			o.mu.Unlock()
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
	return requireExecutableEntryWorkflow(o.Document)
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
	state.Trigger = cloneTriggerContext(state.Trigger)
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
			Trigger:   cloneTriggerContext(state.Trigger),
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
			Trigger:   cloneTriggerContext(state.Trigger),
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
