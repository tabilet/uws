package flowcore

import "testing"

func TestWorkflowTypeHelpers(t *testing.T) {
	for _, typeName := range []string{
		WorkflowTypeSequence,
		WorkflowTypeParallel,
		WorkflowTypeSwitch,
		WorkflowTypeMerge,
		WorkflowTypeLoop,
		WorkflowTypeAwait,
	} {
		if !IsWorkflowType(typeName) {
			t.Fatalf("expected %q to be a valid workflow type", typeName)
		}
	}
	if IsWorkflowType("http") {
		t.Fatalf("unexpected non-workflow type accepted")
	}

	if RequiresItems(WorkflowTypeLoop) != true {
		t.Fatalf("loop should require items")
	}
	if RequiresItems(WorkflowTypeSwitch) {
		t.Fatalf("switch should not require items")
	}
	if RequiresWait(WorkflowTypeAwait) != true {
		t.Fatalf("await should require wait")
	}
	if RequiresWait(WorkflowTypeParallel) {
		t.Fatalf("parallel should not require wait")
	}
	if !AllowsCases(WorkflowTypeSwitch) || !AllowsDefault(WorkflowTypeSwitch) {
		t.Fatalf("switch should allow cases and default")
	}
	if AllowsCases(WorkflowTypeLoop) || AllowsDefault(WorkflowTypeLoop) {
		t.Fatalf("loop should not allow cases or default")
	}
	if !RequiresDependsOnForMerge(WorkflowTypeMerge) {
		t.Fatalf("merge should require dependsOn")
	}
	if RequiresDependsOnForMerge(WorkflowTypeSequence) {
		t.Fatalf("sequence should not require merge dependsOn semantics")
	}
}

func TestStructuralResultKindHelpers(t *testing.T) {
	for _, kind := range []string{
		StructuralResultKindSwitch,
		StructuralResultKindMerge,
		StructuralResultKindLoop,
	} {
		if !IsStructuralResultKind(kind) {
			t.Fatalf("expected %q to be a valid structural result kind", kind)
		}
	}
	if IsStructuralResultKind("parallel") {
		t.Fatalf("unexpected non-result kind accepted")
	}
}
