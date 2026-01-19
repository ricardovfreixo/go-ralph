package rlm

import (
	"sync"
	"testing"
)

func TestNewRecursiveFeature(t *testing.T) {
	f := NewRecursiveFeature("test-id", "Test Feature")

	if f.ID != "test-id" {
		t.Errorf("expected ID 'test-id', got '%s'", f.ID)
	}
	if f.Title != "Test Feature" {
		t.Errorf("expected Title 'Test Feature', got '%s'", f.Title)
	}
	if f.Depth != 0 {
		t.Errorf("expected Depth 0, got %d", f.Depth)
	}
	if f.MaxDepth != DefaultMaxDepth {
		t.Errorf("expected MaxDepth %d, got %d", DefaultMaxDepth, f.MaxDepth)
	}
	if f.ContextBudget != DefaultContextBudget {
		t.Errorf("expected ContextBudget %d, got %d", DefaultContextBudget, f.ContextBudget)
	}
	if f.Status != "pending" {
		t.Errorf("expected Status 'pending', got '%s'", f.Status)
	}
	if f.ParentID != "" {
		t.Errorf("expected empty ParentID, got '%s'", f.ParentID)
	}
	if f.TokenUsage == nil {
		t.Error("expected TokenUsage to be initialized")
	}
}

func TestNewChildFeature(t *testing.T) {
	parent := NewRecursiveFeature("parent-id", "Parent Feature")
	parent.Model = "opus"
	parent.ExecutionMode = "sequential"

	child, err := parent.NewChildFeature("child-id", "Child Feature")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if child.ParentID != "parent-id" {
		t.Errorf("expected ParentID 'parent-id', got '%s'", child.ParentID)
	}
	if child.Depth != 1 {
		t.Errorf("expected Depth 1, got %d", child.Depth)
	}
	// Budget should use formula: base / (depth + 1)
	expectedBudget := CalculateContextBudgetForDepth(parent.ContextBudget, 1)
	if child.ContextBudget != expectedBudget {
		t.Errorf("expected ContextBudget %d, got %d", expectedBudget, child.ContextBudget)
	}
	if child.Model != "opus" {
		t.Errorf("expected Model 'opus', got '%s'", child.Model)
	}
	if child.ExecutionMode != "sequential" {
		t.Errorf("expected ExecutionMode 'sequential', got '%s'", child.ExecutionMode)
	}
	if child.ContextUsed != 0 {
		t.Errorf("expected ContextUsed 0, got %d", child.ContextUsed)
	}
}

func TestNewChildFeatureMaxDepthExceeded(t *testing.T) {
	parent := NewRecursiveFeature("parent-id", "Parent Feature")
	parent.Depth = 5
	parent.MaxDepth = 5

	_, err := parent.NewChildFeature("child-id", "Child Feature")
	if err != ErrMaxDepthExceeded {
		t.Errorf("expected ErrMaxDepthExceeded, got %v", err)
	}
}

func TestAddAndGetSubFeatures(t *testing.T) {
	parent := NewRecursiveFeature("parent-id", "Parent")
	child1, _ := parent.NewChildFeature("child1", "Child 1")
	child2, _ := parent.NewChildFeature("child2", "Child 2")

	parent.AddSubFeature(child1)
	parent.AddSubFeature(child2)

	subs := parent.GetSubFeatures()
	if len(subs) != 2 {
		t.Errorf("expected 2 sub-features, got %d", len(subs))
	}
}

func TestIsRootAndCanSpawn(t *testing.T) {
	root := NewRecursiveFeature("root", "Root")
	if !root.IsRoot() {
		t.Error("expected root to be root")
	}
	if !root.CanSpawn() {
		t.Error("expected root to be able to spawn")
	}

	child, _ := root.NewChildFeature("child", "Child")
	if child.IsRoot() {
		t.Error("expected child not to be root")
	}
}

func TestCanSpawnAtMaxDepth(t *testing.T) {
	f := NewRecursiveFeature("id", "Feature")
	f.Depth = 5
	f.MaxDepth = 5

	if f.CanSpawn() {
		t.Error("expected feature at max depth to not be able to spawn")
	}
}

func TestSetAndGetStatus(t *testing.T) {
	f := NewRecursiveFeature("id", "Feature")

	f.SetStatus("running")
	if f.GetStatus() != "running" {
		t.Errorf("expected status 'running', got '%s'", f.GetStatus())
	}
	if f.StartedAt == nil {
		t.Error("expected StartedAt to be set")
	}

	f.SetStatus("completed")
	if f.GetStatus() != "completed" {
		t.Errorf("expected status 'completed', got '%s'", f.GetStatus())
	}
	if f.CompletedAt == nil {
		t.Error("expected CompletedAt to be set")
	}
}

func TestAddAndGetActions(t *testing.T) {
	f := NewRecursiveFeature("id", "Feature")

	action1 := Action{Type: "file_modify", Name: "Write", Details: "/path/to/file.go"}
	action2 := Action{Type: "command", Name: "Bash", Details: "go test ./..."}

	f.AddAction(action1)
	f.AddAction(action2)

	actions := f.GetActions()
	if len(actions) != 2 {
		t.Errorf("expected 2 actions, got %d", len(actions))
	}
	if actions[0].Type != "file_modify" {
		t.Errorf("expected first action type 'file_modify', got '%s'", actions[0].Type)
	}
}

func TestTokenUsageAdd(t *testing.T) {
	t1 := NewTokenUsage()
	t1.Update(100, 50, 10, 5, 0.01)

	t2 := NewTokenUsage()
	t2.Update(200, 100, 20, 10, 0.02)

	t1.Add(t2)

	snapshot := t1.GetSnapshot()
	if snapshot.InputTokens != 300 {
		t.Errorf("expected InputTokens 300, got %d", snapshot.InputTokens)
	}
	if snapshot.OutputTokens != 150 {
		t.Errorf("expected OutputTokens 150, got %d", snapshot.OutputTokens)
	}
	if snapshot.CacheReadTokens != 30 {
		t.Errorf("expected CacheReadTokens 30, got %d", snapshot.CacheReadTokens)
	}
	if snapshot.CacheWriteTokens != 15 {
		t.Errorf("expected CacheWriteTokens 15, got %d", snapshot.CacheWriteTokens)
	}
}

func TestGetTotalTokenUsage(t *testing.T) {
	parent := NewRecursiveFeature("parent", "Parent")
	parent.TokenUsage.Update(1000, 500, 0, 0, 0.1)

	child1, _ := parent.NewChildFeature("child1", "Child 1")
	child1.TokenUsage.Update(500, 250, 0, 0, 0.05)
	parent.AddSubFeature(child1)

	child2, _ := parent.NewChildFeature("child2", "Child 2")
	child2.TokenUsage.Update(500, 250, 0, 0, 0.05)
	parent.AddSubFeature(child2)

	total := parent.GetTotalTokenUsage()
	snapshot := total.GetSnapshot()

	if snapshot.InputTokens != 2000 {
		t.Errorf("expected total InputTokens 2000, got %d", snapshot.InputTokens)
	}
	if snapshot.OutputTokens != 1000 {
		t.Errorf("expected total OutputTokens 1000, got %d", snapshot.OutputTokens)
	}
}

func TestTokenUsageConcurrency(t *testing.T) {
	usage := NewTokenUsage()
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			usage.Update(10, 5, 1, 1, 0.001)
		}()
	}

	wg.Wait()

	snapshot := usage.GetSnapshot()
	if snapshot.InputTokens != 1000 {
		t.Errorf("expected InputTokens 1000, got %d", snapshot.InputTokens)
	}
}

func TestRecursiveFeatureConcurrency(t *testing.T) {
	f := NewRecursiveFeature("id", "Feature")
	var wg sync.WaitGroup

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			f.AddAction(Action{Type: "test", Name: "action"})
		}(i)
	}

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = f.GetActions()
		}()
	}

	wg.Wait()

	actions := f.GetActions()
	if len(actions) != 50 {
		t.Errorf("expected 50 actions, got %d", len(actions))
	}
}

func TestCalculateContextBudgetForDepth(t *testing.T) {
	tests := []struct {
		name     string
		base     int64
		depth    int
		expected int64
	}{
		{"depth 0", 100000, 0, 100000},
		{"depth 1", 100000, 1, 50000},
		{"depth 2", 100000, 2, 33333},
		{"depth 3", 100000, 3, 25000},
		{"depth 4", 100000, 4, 20000},
		{"depth 9 hits minimum", 100000, 9, MinContextBudget},
		{"very deep hits minimum", 100000, 100, MinContextBudget},
		{"negative depth treated as 0", 100000, -1, 100000},
		{"small base hits minimum", 5000, 0, MinContextBudget},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CalculateContextBudgetForDepth(tt.base, tt.depth)
			if result != tt.expected {
				t.Errorf("expected %d, got %d", tt.expected, result)
			}
		})
	}
}

func TestContextBudgetGettersAndSetters(t *testing.T) {
	f := NewRecursiveFeature("id", "Feature")

	if f.GetContextBudget() != int64(DefaultContextBudget) {
		t.Errorf("expected default budget %d, got %d", DefaultContextBudget, f.GetContextBudget())
	}

	f.SetContextBudget(50000)
	if f.GetContextBudget() != 50000 {
		t.Errorf("expected budget 50000, got %d", f.GetContextBudget())
	}

	// Setting to 0 should not change it
	f.SetContextBudget(0)
	if f.GetContextBudget() != 50000 {
		t.Errorf("expected budget unchanged at 50000, got %d", f.GetContextBudget())
	}
}

func TestContextUsageTracking(t *testing.T) {
	f := NewRecursiveFeature("id", "Feature")

	if f.GetContextUsed() != 0 {
		t.Errorf("expected initial usage 0, got %d", f.GetContextUsed())
	}

	f.SetContextUsed(10000)
	if f.GetContextUsed() != 10000 {
		t.Errorf("expected usage 10000, got %d", f.GetContextUsed())
	}

	f.AddContextUsage(5000)
	if f.GetContextUsed() != 15000 {
		t.Errorf("expected usage 15000, got %d", f.GetContextUsed())
	}
}

func TestGetContextRemaining(t *testing.T) {
	f := NewRecursiveFeature("id", "Feature")
	f.SetContextBudget(100000)

	if f.GetContextRemaining() != 100000 {
		t.Errorf("expected remaining 100000, got %d", f.GetContextRemaining())
	}

	f.SetContextUsed(40000)
	if f.GetContextRemaining() != 60000 {
		t.Errorf("expected remaining 60000, got %d", f.GetContextRemaining())
	}

	// Over budget should return 0
	f.SetContextUsed(120000)
	if f.GetContextRemaining() != 0 {
		t.Errorf("expected remaining 0 when over budget, got %d", f.GetContextRemaining())
	}
}

func TestGetContextUsagePercent(t *testing.T) {
	f := NewRecursiveFeature("id", "Feature")
	f.SetContextBudget(100000)

	if f.GetContextUsagePercent() != 0 {
		t.Errorf("expected 0%%, got %f", f.GetContextUsagePercent())
	}

	f.SetContextUsed(50000)
	if f.GetContextUsagePercent() != 0.5 {
		t.Errorf("expected 50%%, got %f", f.GetContextUsagePercent())
	}

	f.SetContextUsed(80000)
	if f.GetContextUsagePercent() != 0.8 {
		t.Errorf("expected 80%%, got %f", f.GetContextUsagePercent())
	}
}

func TestNeedsContextSummarization(t *testing.T) {
	f := NewRecursiveFeature("id", "Feature")
	f.SetContextBudget(100000)

	if f.NeedsContextSummarization() {
		t.Error("should not need summarization at 0%")
	}

	f.SetContextUsed(79000)
	if f.NeedsContextSummarization() {
		t.Error("should not need summarization at 79%")
	}

	f.SetContextUsed(80000)
	if !f.NeedsContextSummarization() {
		t.Error("should need summarization at 80%")
	}
}

func TestIsContextOverBudget(t *testing.T) {
	f := NewRecursiveFeature("id", "Feature")
	f.SetContextBudget(100000)

	if f.IsContextOverBudget() {
		t.Error("should not be over budget at 0%")
	}

	f.SetContextUsed(100000)
	if f.IsContextOverBudget() {
		t.Error("should not be over budget at exactly 100%")
	}

	f.SetContextUsed(100001)
	if !f.IsContextOverBudget() {
		t.Error("should be over budget when exceeding limit")
	}
}

func TestCalculateChildContextBudget(t *testing.T) {
	parent := NewRecursiveFeature("parent", "Parent")
	parent.SetContextBudget(100000)

	childBudget := parent.CalculateChildContextBudget()
	expected := CalculateContextBudgetForDepth(100000, 1)
	if childBudget != expected {
		t.Errorf("expected %d, got %d", expected, childBudget)
	}
}

func TestContextBudgetDepthProgression(t *testing.T) {
	// Verify that context budget decreases properly through the depth chain
	baseBudget := int64(100000)

	depth0 := CalculateContextBudgetForDepth(baseBudget, 0) // 100000
	depth1 := CalculateContextBudgetForDepth(baseBudget, 1) // 50000
	depth2 := CalculateContextBudgetForDepth(baseBudget, 2) // 33333
	depth3 := CalculateContextBudgetForDepth(baseBudget, 3) // 25000

	if depth0 <= depth1 {
		t.Errorf("depth 0 (%d) should be > depth 1 (%d)", depth0, depth1)
	}
	if depth1 <= depth2 {
		t.Errorf("depth 1 (%d) should be > depth 2 (%d)", depth1, depth2)
	}
	if depth2 <= depth3 {
		t.Errorf("depth 2 (%d) should be > depth 3 (%d)", depth2, depth3)
	}

	// All should be >= minimum
	if depth3 < MinContextBudget {
		t.Errorf("depth 3 (%d) should be >= minimum (%d)", depth3, MinContextBudget)
	}
}

func TestContextBudgetConcurrency(t *testing.T) {
	f := NewRecursiveFeature("id", "Feature")
	f.SetContextBudget(100000)
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			f.AddContextUsage(100)
			_ = f.GetContextUsed()
			_ = f.GetContextRemaining()
			_ = f.GetContextUsagePercent()
			_ = f.NeedsContextSummarization()
		}()
	}

	wg.Wait()
}
