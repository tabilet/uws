package uws1

import (
	"context"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/tabilet/uws/flowcore"
	"golang.org/x/sync/errgroup"
)

const awaitPollInterval = 200 * time.Millisecond

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
				"id":      record.ID,
				"kind":    record.Kind,
				"status":  record.Status,
				"error":   record.Error,
				"result":  record.Result,
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
