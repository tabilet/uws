package uws1

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tabilet/uws/flowcore"
)

type mockRuntime struct {
	executedLeafs []string
	expressions   map[string]any
	items         map[string][]any
	eval          func(context.Context, string) (any, error)
}

func (m *mockRuntime) ExecuteLeaf(ctx context.Context, op *Operation) error {
	m.executedLeafs = append(m.executedLeafs, op.OperationID)
	return nil
}

func (m *mockRuntime) ResolveItems(ctx context.Context, itemsExpr string) ([]any, error) {
	if m.items == nil {
		return nil, nil
	}
	return m.items[itemsExpr], nil
}

func (m *mockRuntime) EvaluateExpression(ctx context.Context, expr string) (any, error) {
	if m.eval != nil {
		return m.eval(ctx, expr)
	}
	if m.expressions == nil {
		return nil, nil
	}
	return m.expressions[expr], nil
}

func testDocument(ops ...*Operation) *Document {
	for _, op := range ops {
		if op == nil {
			continue
		}
		if op.Extensions == nil {
			op.Extensions = map[string]any{ExtensionOperationProfile: "test"}
		}
	}
	return &Document{
		UWS: "1.0.0",
		Info: &Info{
			Title:   "test",
			Version: "1.0.0",
		},
		Operations: ops,
	}
}

func TestOrchestratorExecuteSequenceWorkflow(t *testing.T) {
	doc := testDocument(
		&Operation{OperationID: "op1"},
		&Operation{OperationID: "op2"},
	)
	doc.Workflows = []*Workflow{{
		WorkflowID: "main",
		Type:       flowcore.WorkflowTypeSequence,
		Steps: []*Step{
			{StepID: "step1", OperationRef: "op1"},
			{StepID: "step2", OperationRef: "op2"},
		},
	}}
	runtime := &mockRuntime{}
	doc.SetRuntime(runtime)

	if err := doc.Execute(context.Background()); err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if len(runtime.executedLeafs) != 2 || runtime.executedLeafs[0] != "op1" || runtime.executedLeafs[1] != "op2" {
		t.Fatalf("unexpected execution order: %v", runtime.executedLeafs)
	}
}

func TestOrchestratorSkipsWhenFalse(t *testing.T) {
	doc := testDocument(
		&Operation{OperationID: "op1"},
		&Operation{OperationID: "op2"},
	)
	doc.Workflows = []*Workflow{{
		WorkflowID: "main",
		Type:       flowcore.WorkflowTypeSequence,
		Steps: []*Step{
			{StepID: "step1", OperationRef: "op1", StepExecutionFields: flowcore.RunnableExecutionFields{When: "false"}},
			{StepID: "step2", OperationRef: "op2", StepExecutionFields: flowcore.RunnableExecutionFields{When: "true"}},
		},
	}}
	runtime := &mockRuntime{expressions: map[string]any{"false": false, "true": true}}
	doc.SetRuntime(runtime)

	if err := doc.Execute(context.Background()); err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if len(runtime.executedLeafs) != 1 || runtime.executedLeafs[0] != "op2" {
		t.Fatalf("unexpected execution result: %v", runtime.executedLeafs)
	}
}

func TestOrchestratorParallelGroupDependencyBarrier(t *testing.T) {
	doc := testDocument(
		&Operation{OperationID: "op1", RunnableExecutionFields: flowcore.RunnableExecutionFields{ParallelGroup: "grp"}},
		&Operation{OperationID: "op2", RunnableExecutionFields: flowcore.RunnableExecutionFields{ParallelGroup: "grp"}},
		&Operation{OperationID: "op3"},
	)
	doc.Workflows = []*Workflow{{
		WorkflowID: "main",
		Type:       flowcore.WorkflowTypeSequence,
		Steps: []*Step{
			{StepID: "step1", OperationRef: "op1"},
			{StepID: "step2", OperationRef: "op2"},
			{StepID: "step3", OperationRef: "op3", StepExecutionFields: flowcore.RunnableExecutionFields{DependsOn: []string{"grp"}}},
		},
	}}
	runtime := &mockRuntime{}
	doc.SetRuntime(runtime)

	if err := doc.Execute(context.Background()); err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if len(runtime.executedLeafs) != 3 {
		t.Fatalf("expected 3 executions, got %v", runtime.executedLeafs)
	}
}

func TestOrchestratorExecuteSwitch(t *testing.T) {
	doc := testDocument(
		&Operation{OperationID: "op1"},
		&Operation{OperationID: "op2"},
	)
	doc.Workflows = []*Workflow{{
		WorkflowID: "main",
		Type:       flowcore.WorkflowTypeSwitch,
		Cases: []*Case{
			{CaseFields: flowcore.CaseFields{Name: "case1", When: "false"}, Steps: []*Step{{StepID: "s1", OperationRef: "op1"}}},
			{CaseFields: flowcore.CaseFields{Name: "case2", When: "true"}, Steps: []*Step{{StepID: "s2", OperationRef: "op2"}}},
		},
	}}
	runtime := &mockRuntime{expressions: map[string]any{"false": false, "true": true}}
	doc.SetRuntime(runtime)

	if err := doc.Execute(context.Background()); err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if len(runtime.executedLeafs) != 1 || runtime.executedLeafs[0] != "op2" {
		t.Fatalf("unexpected switch execution: %v", runtime.executedLeafs)
	}
}

func TestOrchestratorExecuteLoop(t *testing.T) {
	doc := testDocument(&Operation{OperationID: "op1"})
	doc.Workflows = []*Workflow{{
		WorkflowID:       "main",
		Type:             flowcore.WorkflowTypeLoop,
		StructuralFields: flowcore.StructuralFields{Items: "items"},
		Steps: []*Step{
			{StepID: "step1", OperationRef: "op1"},
		},
	}}
	runtime := &mockRuntime{items: map[string][]any{"items": []any{1, 2, 3}}}
	doc.SetRuntime(runtime)

	if err := doc.Execute(context.Background()); err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if len(runtime.executedLeafs) != 3 {
		t.Fatalf("expected 3 loop executions, got %v", runtime.executedLeafs)
	}
}

func TestOrchestratorExecuteAwait(t *testing.T) {
	doc := testDocument(&Operation{OperationID: "op1"})
	doc.Workflows = []*Workflow{{
		WorkflowID: "main",
		Type:       flowcore.WorkflowTypeAwait,
		WorkflowExecutionFields: flowcore.WorkflowExecutionFields{
			Wait: "ready",
		},
		Steps: []*Step{{StepID: "step1", OperationRef: "op1"}},
	}}
	runtime := &mockRuntime{expressions: map[string]any{"ready": true}}
	doc.SetRuntime(runtime)

	if err := doc.Execute(context.Background()); err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if len(runtime.executedLeafs) != 1 || runtime.executedLeafs[0] != "op1" {
		t.Fatalf("unexpected await execution: %v", runtime.executedLeafs)
	}
}

func TestOrchestratorExecuteMerge(t *testing.T) {
	doc := testDocument(
		&Operation{OperationID: "op1"},
		&Operation{OperationID: "op2"},
	)
	doc.Workflows = []*Workflow{{
		WorkflowID: "main",
		Type:       flowcore.WorkflowTypeSequence,
		Steps: []*Step{
			{StepID: "left", OperationRef: "op1"},
			{StepID: "right", OperationRef: "op2"},
			{
				StepID: "join",
				Type:   flowcore.WorkflowTypeMerge,
				StepExecutionFields: flowcore.RunnableExecutionFields{
					DependsOn: []string{"left", "right"},
				},
			},
		},
	}}
	runtime := &mockRuntime{}
	doc.SetRuntime(runtime)

	if err := doc.Execute(context.Background()); err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if len(runtime.executedLeafs) != 2 {
		t.Fatalf("unexpected merge execution: %v", runtime.executedLeafs)
	}
}

func TestOrchestratorExecuteMergeUsesDeclaredDependenciesOnly(t *testing.T) {
	doc := testDocument(
		&Operation{OperationID: "prep"},
		&Operation{OperationID: "left"},
		&Operation{OperationID: "right"},
	)
	doc.Workflows = []*Workflow{{
		WorkflowID: "main",
		Type:       flowcore.WorkflowTypeSequence,
		Steps: []*Step{
			{StepID: "prep", OperationRef: "prep"},
			{StepID: "left", OperationRef: "left"},
			{StepID: "right", OperationRef: "right"},
			{
				StepID: "join",
				Type:   flowcore.WorkflowTypeMerge,
				StepExecutionFields: flowcore.RunnableExecutionFields{
					DependsOn: []string{"left", "right"},
				},
			},
		},
	}}
	orch := NewOrchestrator(doc, &mockRuntime{})

	if err := orch.Execute(context.Background()); err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	record := orch.records["step:join"]
	results, ok := record.Result.([]map[string]any)
	if !ok {
		t.Fatalf("unexpected merge result type: %#v", record.Result)
	}
	if len(results) != 2 {
		t.Fatalf("expected exactly declared dependencies, got %#v", results)
	}
	if results[0]["id"] != "left" || results[1]["id"] != "right" {
		t.Fatalf("unexpected dependency order in merge result: %#v", results)
	}
}

func TestDocumentValidateExecutableRejectsAmbiguousIDs(t *testing.T) {
	doc := testDocument(&Operation{OperationID: "shared"})
	doc.Workflows = []*Workflow{{
		WorkflowID: "main",
		Type:       flowcore.WorkflowTypeSequence,
		Steps: []*Step{{StepID: "shared", OperationRef: "shared"}},
	}}

	err := doc.ValidateExecutable()
	if err == nil {
		t.Fatal("expected executable validation error")
	}
	if got := err.Error(); got == "" || got == "shared" {
		t.Fatalf("unexpected validation error: %v", err)
	}
}

func TestDocumentValidateExecutableAllowsNonSimpleCriteria(t *testing.T) {
	doc := testDocument(&Operation{
		OperationID: "op1",
		SuccessCriteria: []*Criterion{{
			Type:      CriterionRegex,
			Context:   "$response.body",
			Condition: "^ok",
		}},
	})

	require.NoError(t, doc.ValidateExecutable())
}

func TestDocumentValidateExecutableAllowsOutputsAndResults(t *testing.T) {
	doc := testDocument(&Operation{
		OperationID: "fetch",
		Outputs: map[string]string{
			"body": "$response.body",
		},
	})
	doc.Workflows = []*Workflow{{
		WorkflowID: "main",
		Type:       flowcore.WorkflowTypeSequence,
		Steps: []*Step{
			{StepID: "fetch_step", OperationRef: "fetch"},
			{
				StepID: "joined_step",
				Type:   flowcore.WorkflowTypeMerge,
				StepExecutionFields: flowcore.RunnableExecutionFields{
					DependsOn: []string{"fetch_step"},
				},
				Outputs: map[string]string{
					"body": "$steps.fetch_step.outputs.body",
				},
			},
		},
	}}
	doc.Results = []*StructuralResult{{
		Name:  "joined_result",
		Kind:  flowcore.StructuralResultKindMerge,
		From:  "main.joined_step",
		Value: "$steps.joined_step.outputs.body",
	}}

	require.NoError(t, doc.ValidateExecutable())
}

func TestOrchestratorExecutesRegexCriterion(t *testing.T) {
	doc := testDocument(&Operation{
		OperationID: "fetch",
		SuccessCriteria: []*Criterion{{
			Type:      CriterionRegex,
			Context:   "$response.body",
			Condition: "^ok$",
		}},
	})
	runtime := &mockRuntime{
		eval: func(ctx context.Context, expr string) (any, error) {
			if expr == "$response.body" {
				return "ok", nil
			}
			return nil, nil
		},
	}
	doc.SetRuntime(runtime)

	require.NoError(t, doc.Execute(context.Background()))
	assert.Equal(t, []string{"fetch"}, runtime.executedLeafs)
}

func TestOrchestratorExecutesJSONPathCriterion(t *testing.T) {
	doc := testDocument(&Operation{
		OperationID: "fetch",
		SuccessCriteria: []*Criterion{{
			Type:      CriterionJSONPath,
			Context:   "$response.body",
			Condition: "#/id",
		}},
	})
	runtime := &mockRuntime{
		eval: func(ctx context.Context, expr string) (any, error) {
			if expr == "$response.body" {
				return map[string]any{"id": "123"}, nil
			}
			return nil, nil
		},
	}
	doc.SetRuntime(runtime)

	require.NoError(t, doc.Execute(context.Background()))
	assert.Equal(t, []string{"fetch"}, runtime.executedLeafs)
}

func TestOrchestratorExecutesXPathCriterion(t *testing.T) {
	doc := testDocument(&Operation{
		OperationID: "fetch",
		SuccessCriteria: []*Criterion{{
			Type:      CriterionXPath,
			Context:   "$response.body",
			Condition: "count(/root/item[@kind='primary'][text()='123']) = 1",
		}},
	})
	runtime := &mockRuntime{
		eval: func(ctx context.Context, expr string) (any, error) {
			if expr == "$response.body" {
				return `<root><item kind="secondary">nope</item><item kind="primary">123</item></root>`, nil
			}
			return nil, nil
		},
	}
	doc.SetRuntime(runtime)

	require.NoError(t, doc.Execute(context.Background()))
	assert.Equal(t, []string{"fetch"}, runtime.executedLeafs)
}

func TestOrchestratorCapturesOutputs(t *testing.T) {
	doc := testDocument(&Operation{
		OperationID: "fetch",
		Outputs: map[string]string{
			"body": "$response.body",
		},
	})
	doc.Workflows = []*Workflow{{
		WorkflowID: "main",
		Type:       flowcore.WorkflowTypeSequence,
		Steps: []*Step{{
			StepID:       "fetch_step",
			OperationRef: "fetch",
			Outputs: map[string]string{
				"copy": "$response.body",
			},
		}},
		Outputs: map[string]string{
			"from_step": "$steps.fetch_step.outputs.copy",
		},
	}}
	runtime := &mockRuntime{
		eval: func(ctx context.Context, expr string) (any, error) {
			state, _ := ExecutionContextFromContext(ctx)
			switch expr {
			case "$response.body":
				return map[string]any{"city": "Toronto"}, nil
			case "$steps.fetch_step.outputs.copy":
				if state == nil {
					return nil, nil
				}
				record, ok := state.Records["step:fetch_step"]
				if !ok {
					return nil, nil
				}
				return record.Outputs["copy"], nil
			default:
				return nil, nil
			}
		},
	}
	doc.SetRuntime(runtime)

	require.NoError(t, doc.Execute(context.Background()))
	records := doc.ExecutionRecords()
	require.Contains(t, records, "op:fetch")
	require.Contains(t, records, "step:fetch_step")
	require.Contains(t, records, "wf:main")
	assert.Equal(t, map[string]any{"city": "Toronto"}, records["op:fetch"].Outputs["body"])
	assert.Equal(t, map[string]any{"city": "Toronto"}, records["step:fetch_step"].Outputs["copy"])
	assert.Equal(t, map[string]any{"city": "Toronto"}, records["wf:main"].Outputs["from_step"])
}
