package manifest

import (
	"testing"
)

func TestDependencyGraph_DetectCycles_NoCycle(t *testing.T) {
	graph := NewDependencyGraph()
	graph.AddNode("01")
	graph.AddNode("02")
	graph.AddNode("03")
	graph.AddEdge("02", "01") // 02 depends on 01
	graph.AddEdge("03", "02") // 03 depends on 02

	err := graph.DetectCycles()
	if err != nil {
		t.Errorf("expected no cycle, got: %v", err)
	}
}

func TestDependencyGraph_DetectCycles_SimpleCycle(t *testing.T) {
	graph := NewDependencyGraph()
	graph.AddNode("01")
	graph.AddNode("02")
	graph.AddEdge("01", "02") // 01 depends on 02
	graph.AddEdge("02", "01") // 02 depends on 01

	cycleErr := graph.DetectCycles()
	if cycleErr == nil {
		t.Fatal("expected cycle error, got nil")
	}
	if len(cycleErr.Cycle) < 2 {
		t.Errorf("expected cycle to have at least 2 nodes, got %v", cycleErr.Cycle)
	}
}

func TestDependencyGraph_DetectCycles_ComplexCycle(t *testing.T) {
	graph := NewDependencyGraph()
	graph.AddNode("01")
	graph.AddNode("02")
	graph.AddNode("03")
	graph.AddEdge("01", "02") // 01 depends on 02
	graph.AddEdge("02", "03") // 02 depends on 03
	graph.AddEdge("03", "01") // 03 depends on 01 (cycle)

	err := graph.DetectCycles()
	if err == nil {
		t.Fatal("expected cycle error, got nil")
	}
}

func TestDependencyGraph_DetectCycles_SelfReference(t *testing.T) {
	graph := NewDependencyGraph()
	graph.AddNode("01")
	graph.AddEdge("01", "01") // 01 depends on itself

	err := graph.DetectCycles()
	if err == nil {
		t.Fatal("expected cycle error for self-reference, got nil")
	}
}

func TestManifest_BuildDependencyGraph(t *testing.T) {
	m := &Manifest{
		Features: []ManifestFeature{
			{ID: "01", Title: "Feature 1", DependsOn: []string{}},
			{ID: "02", Title: "Feature 2", DependsOn: []string{"01"}},
			{ID: "03", Title: "Feature 3", DependsOn: []string{"01", "02"}},
		},
	}

	graph := m.BuildDependencyGraph()

	if len(graph.nodes) != 3 {
		t.Errorf("expected 3 nodes, got %d", len(graph.nodes))
	}

	deps := graph.GetDependencies("02")
	if len(deps) != 1 || deps[0] != "01" {
		t.Errorf("expected 02 to depend on [01], got %v", deps)
	}

	deps = graph.GetDependencies("03")
	if len(deps) != 2 {
		t.Errorf("expected 03 to have 2 dependencies, got %v", deps)
	}
}

func TestManifest_ValidateDependencies_Valid(t *testing.T) {
	m := &Manifest{
		Features: []ManifestFeature{
			{ID: "01", Title: "Feature 1", DependsOn: []string{}},
			{ID: "02", Title: "Feature 2", DependsOn: []string{"01"}},
			{ID: "03", Title: "Feature 3", DependsOn: []string{"02"}},
		},
	}

	warnings, err := m.ValidateDependencies()
	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
	if len(warnings) != 0 {
		t.Errorf("expected no warnings, got: %v", warnings)
	}
}

func TestManifest_ValidateDependencies_CircularDep(t *testing.T) {
	m := &Manifest{
		Features: []ManifestFeature{
			{ID: "01", Title: "Feature 1", DependsOn: []string{"03"}},
			{ID: "02", Title: "Feature 2", DependsOn: []string{"01"}},
			{ID: "03", Title: "Feature 3", DependsOn: []string{"02"}},
		},
	}

	_, err := m.ValidateDependencies()
	if err == nil {
		t.Fatal("expected circular dependency error, got nil")
	}
	if _, ok := err.(*CircularDependencyError); !ok {
		t.Errorf("expected CircularDependencyError, got %T", err)
	}
}

func TestManifest_ValidateDependencies_MissingDep(t *testing.T) {
	m := &Manifest{
		Features: []ManifestFeature{
			{ID: "01", Title: "Feature 1", DependsOn: []string{}},
			{ID: "02", Title: "Feature 2", DependsOn: []string{"99"}}, // 99 doesn't exist
		},
	}

	warnings, err := m.ValidateDependencies()
	if err != nil {
		t.Errorf("expected no error for missing dep (just warning), got: %v", err)
	}
	if len(warnings) != 1 {
		t.Errorf("expected 1 warning for missing dep, got %d: %v", len(warnings), warnings)
	}
}

func TestManifest_GetNextRunnableFeature_NoPendingDeps(t *testing.T) {
	m := &Manifest{
		Features: []ManifestFeature{
			{ID: "01", Title: "Feature 1", Status: "pending", DependsOn: []string{}},
			{ID: "02", Title: "Feature 2", Status: "pending", DependsOn: []string{"01"}},
		},
	}

	next := m.GetNextRunnableFeature()
	if next == nil {
		t.Fatal("expected a runnable feature, got nil")
	}
	if next.ID != "01" {
		t.Errorf("expected feature 01 (no deps), got %s", next.ID)
	}
}

func TestManifest_GetNextRunnableFeature_DepsCompleted(t *testing.T) {
	m := &Manifest{
		Features: []ManifestFeature{
			{ID: "01", Title: "Feature 1", Status: "completed", DependsOn: []string{}},
			{ID: "02", Title: "Feature 2", Status: "pending", DependsOn: []string{"01"}},
		},
	}

	next := m.GetNextRunnableFeature()
	if next == nil {
		t.Fatal("expected a runnable feature, got nil")
	}
	if next.ID != "02" {
		t.Errorf("expected feature 02 (deps completed), got %s", next.ID)
	}
}

func TestManifest_GetNextRunnableFeature_DepsNotCompleted(t *testing.T) {
	m := &Manifest{
		Features: []ManifestFeature{
			{ID: "01", Title: "Feature 1", Status: "running", DependsOn: []string{}},
			{ID: "02", Title: "Feature 2", Status: "pending", DependsOn: []string{"01"}},
		},
	}

	next := m.GetNextRunnableFeature()
	if next != nil {
		t.Errorf("expected no runnable feature (01 is running, 02 blocked), got %s", next.ID)
	}
}

func TestManifest_GetNextRunnableFeature_AllCompleted(t *testing.T) {
	m := &Manifest{
		Features: []ManifestFeature{
			{ID: "01", Title: "Feature 1", Status: "completed", DependsOn: []string{}},
			{ID: "02", Title: "Feature 2", Status: "completed", DependsOn: []string{"01"}},
		},
	}

	next := m.GetNextRunnableFeature()
	if next != nil {
		t.Errorf("expected no runnable feature (all completed), got %s", next.ID)
	}
}

func TestManifest_GetAllRunnableFeatures(t *testing.T) {
	m := &Manifest{
		Features: []ManifestFeature{
			{ID: "01", Title: "Feature 1", Status: "pending", DependsOn: []string{}},
			{ID: "02", Title: "Feature 2", Status: "pending", DependsOn: []string{}},
			{ID: "03", Title: "Feature 3", Status: "pending", DependsOn: []string{"01"}},
			{ID: "04", Title: "Feature 4", Status: "completed", DependsOn: []string{}},
		},
	}

	runnable := m.GetAllRunnableFeatures()
	if len(runnable) != 2 {
		t.Errorf("expected 2 runnable features, got %d", len(runnable))
	}
}

func TestManifest_GetBlockedFeatures(t *testing.T) {
	m := &Manifest{
		Features: []ManifestFeature{
			{ID: "01", Title: "Feature 1", Status: "pending", DependsOn: []string{}},
			{ID: "02", Title: "Feature 2", Status: "pending", DependsOn: []string{"01"}},
			{ID: "03", Title: "Feature 3", Status: "pending", DependsOn: []string{"02"}},
		},
	}

	blocked := m.GetBlockedFeatures()
	if len(blocked) != 2 {
		t.Errorf("expected 2 blocked features, got %d", len(blocked))
	}
}

func TestManifest_GetTopologicalOrder_Simple(t *testing.T) {
	m := &Manifest{
		Features: []ManifestFeature{
			{ID: "01", Title: "Feature 1", DependsOn: []string{}},
			{ID: "02", Title: "Feature 2", DependsOn: []string{"01"}},
			{ID: "03", Title: "Feature 3", DependsOn: []string{"02"}},
		},
	}

	order, err := m.GetTopologicalOrder()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(order) != 3 {
		t.Fatalf("expected 3 items in order, got %d", len(order))
	}

	idx01 := indexOf(order, "01")
	idx02 := indexOf(order, "02")
	idx03 := indexOf(order, "03")

	if idx01 > idx02 {
		t.Error("01 should come before 02")
	}
	if idx02 > idx03 {
		t.Error("02 should come before 03")
	}
}

func TestManifest_GetTopologicalOrder_Parallel(t *testing.T) {
	m := &Manifest{
		Features: []ManifestFeature{
			{ID: "01", Title: "Feature 1", DependsOn: []string{}},
			{ID: "02", Title: "Feature 2", DependsOn: []string{}},
			{ID: "03", Title: "Feature 3", DependsOn: []string{"01", "02"}},
		},
	}

	order, err := m.GetTopologicalOrder()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(order) != 3 {
		t.Fatalf("expected 3 items in order, got %d", len(order))
	}

	idx01 := indexOf(order, "01")
	idx02 := indexOf(order, "02")
	idx03 := indexOf(order, "03")

	if idx01 > idx03 {
		t.Error("01 should come before 03")
	}
	if idx02 > idx03 {
		t.Error("02 should come before 03")
	}
}

func TestManifest_GetTopologicalOrder_CircularDep(t *testing.T) {
	m := &Manifest{
		Features: []ManifestFeature{
			{ID: "01", Title: "Feature 1", DependsOn: []string{"02"}},
			{ID: "02", Title: "Feature 2", DependsOn: []string{"01"}},
		},
	}

	_, err := m.GetTopologicalOrder()
	if err == nil {
		t.Fatal("expected error for circular dependency")
	}
}

func TestManifest_RemoveMissingDependencies(t *testing.T) {
	m := &Manifest{
		Features: []ManifestFeature{
			{ID: "01", Title: "Feature 1", DependsOn: []string{}},
			{ID: "02", Title: "Feature 2", DependsOn: []string{"01", "99", "nonexistent"}},
		},
	}

	removed := m.RemoveMissingDependencies()
	if len(removed) != 2 {
		t.Errorf("expected 2 removed deps, got %d: %v", len(removed), removed)
	}

	f2 := m.GetFeature("02")
	if len(f2.DependsOn) != 1 || f2.DependsOn[0] != "01" {
		t.Errorf("expected feature 02 to only depend on 01, got %v", f2.DependsOn)
	}
}

func TestManifest_IsDependencySatisfied(t *testing.T) {
	m := &Manifest{
		Features: []ManifestFeature{
			{ID: "01", Title: "Feature 1", Status: "completed", DependsOn: []string{}},
			{ID: "02", Title: "Feature 2", Status: "pending", DependsOn: []string{"01"}},
			{ID: "03", Title: "Feature 3", Status: "pending", DependsOn: []string{"02"}},
		},
	}

	if !m.IsDependencySatisfied("01") {
		t.Error("01 has no deps, should be satisfied")
	}
	if !m.IsDependencySatisfied("02") {
		t.Error("02's dep (01) is completed, should be satisfied")
	}
	if m.IsDependencySatisfied("03") {
		t.Error("03's dep (02) is pending, should NOT be satisfied")
	}
}

func TestManifest_GetPendingDependencies(t *testing.T) {
	m := &Manifest{
		Features: []ManifestFeature{
			{ID: "01", Title: "Feature 1", Status: "completed", DependsOn: []string{}},
			{ID: "02", Title: "Feature 2", Status: "running", DependsOn: []string{}},
			{ID: "03", Title: "Feature 3", Status: "pending", DependsOn: []string{"01", "02"}},
		},
	}

	pending := m.GetPendingDependencies("03")
	if len(pending) != 1 || pending[0] != "02" {
		t.Errorf("expected pending deps to be [02], got %v", pending)
	}
}

func indexOf(slice []string, item string) int {
	for i, v := range slice {
		if v == item {
			return i
		}
	}
	return -1
}
