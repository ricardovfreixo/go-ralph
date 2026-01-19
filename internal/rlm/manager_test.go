package rlm

import (
	"sync"
	"testing"
)

func TestNewManager(t *testing.T) {
	m := NewManager()

	if m.maxDepth != DefaultMaxDepth {
		t.Errorf("expected maxDepth %d, got %d", DefaultMaxDepth, m.maxDepth)
	}
	if m.contextBudget != DefaultContextBudget {
		t.Errorf("expected contextBudget %d, got %d", DefaultContextBudget, m.contextBudget)
	}
}

func TestNewManagerWithConfig(t *testing.T) {
	m := NewManagerWithConfig(3, 50000)

	if m.maxDepth != 3 {
		t.Errorf("expected maxDepth 3, got %d", m.maxDepth)
	}
	if m.contextBudget != 50000 {
		t.Errorf("expected contextBudget 50000, got %d", m.contextBudget)
	}
}

func TestNewManagerWithConfigDefaults(t *testing.T) {
	m := NewManagerWithConfig(0, 0)

	if m.maxDepth != DefaultMaxDepth {
		t.Errorf("expected default maxDepth, got %d", m.maxDepth)
	}
	if m.contextBudget != DefaultContextBudget {
		t.Errorf("expected default contextBudget, got %d", m.contextBudget)
	}
}

func TestRegisterFeature(t *testing.T) {
	m := NewManager()
	f := m.RegisterFeature("test-id", "Test Feature")

	if f == nil {
		t.Fatal("expected feature, got nil")
	}
	if f.ID != "test-id" {
		t.Errorf("expected ID 'test-id', got '%s'", f.ID)
	}
	if f.MaxDepth != m.maxDepth {
		t.Errorf("expected MaxDepth %d, got %d", m.maxDepth, f.MaxDepth)
	}
	if f.ContextBudget != m.contextBudget {
		t.Errorf("expected ContextBudget %d, got %d", m.contextBudget, f.ContextBudget)
	}

	retrieved := m.GetFeature("test-id")
	if retrieved != f {
		t.Error("GetFeature returned different feature")
	}

	tracker := m.GetTracker("test-id")
	if tracker == nil {
		t.Error("expected tracker, got nil")
	}
}

func TestGetFeatureNotFound(t *testing.T) {
	m := NewManager()
	f := m.GetFeature("nonexistent")
	if f != nil {
		t.Error("expected nil for nonexistent feature")
	}
}

func TestProcessOutput(t *testing.T) {
	m := NewManager()
	m.RegisterFeature("test", "Test Feature")

	line := `{"type":"tool_use","tool":"Write","tool_input":{"file_path":"/test.go"}}`

	_, err := m.ProcessOutput("test", line)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	f := m.GetFeature("test")
	actions := f.GetActions()
	if len(actions) != 1 {
		t.Errorf("expected 1 action, got %d", len(actions))
	}
}

func TestProcessOutputFeatureNotFound(t *testing.T) {
	m := NewManager()

	_, err := m.ProcessOutput("nonexistent", `{}`)
	if err != ErrFeatureNotFound {
		t.Errorf("expected ErrFeatureNotFound, got %v", err)
	}
}

func TestSpawnSubFeature(t *testing.T) {
	m := NewManager()
	parent := m.RegisterFeature("parent", "Parent Feature")
	parent.SetStatus("running")

	req := &SpawnRequest{
		Title: "Child Feature",
		Tasks: []string{"Task 1", "Task 2"},
		Model: "haiku",
	}

	child, err := m.SpawnSubFeature("parent", req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if child.ParentID != "parent" {
		t.Errorf("expected ParentID 'parent', got '%s'", child.ParentID)
	}
	if child.Title != "Child Feature" {
		t.Errorf("expected title 'Child Feature', got '%s'", child.Title)
	}
	if len(child.Tasks) != 2 {
		t.Errorf("expected 2 tasks, got %d", len(child.Tasks))
	}
	if child.Model != "haiku" {
		t.Errorf("expected model 'haiku', got '%s'", child.Model)
	}
	if child.Depth != 1 {
		t.Errorf("expected depth 1, got %d", child.Depth)
	}

	subs := m.GetSubFeatures("parent")
	if len(subs) != 1 {
		t.Errorf("expected 1 sub-feature, got %d", len(subs))
	}

	retrievedChild := m.GetFeature(child.ID)
	if retrievedChild == nil {
		t.Error("child feature not registered in manager")
	}
}

func TestSpawnSubFeatureParentNotFound(t *testing.T) {
	m := NewManager()

	req := &SpawnRequest{Title: "Child"}
	_, err := m.SpawnSubFeature("nonexistent", req)
	if err != ErrFeatureNotFound {
		t.Errorf("expected ErrFeatureNotFound, got %v", err)
	}
}

func TestSpawnSubFeatureParentNotRunning(t *testing.T) {
	m := NewManager()
	m.RegisterFeature("parent", "Parent")

	req := &SpawnRequest{Title: "Child"}
	_, err := m.SpawnSubFeature("parent", req)
	if err != ErrParentNotRunning {
		t.Errorf("expected ErrParentNotRunning, got %v", err)
	}
}

func TestSpawnSubFeatureMaxDepth(t *testing.T) {
	m := NewManagerWithConfig(1, DefaultContextBudget)
	parent := m.RegisterFeature("parent", "Parent")
	parent.SetStatus("running")

	req1 := &SpawnRequest{Title: "Child 1"}
	child1, err := m.SpawnSubFeature("parent", req1)
	if err != nil {
		t.Fatalf("unexpected error spawning first child: %v", err)
	}
	child1.SetStatus("running")

	req2 := &SpawnRequest{Title: "Grandchild"}
	_, err = m.SpawnSubFeature(child1.ID, req2)
	if err != ErrMaxDepthExceeded {
		t.Errorf("expected ErrMaxDepthExceeded, got %v", err)
	}
}

func TestSpawnSubFeatureWithRequestMaxDepth(t *testing.T) {
	m := NewManagerWithConfig(10, DefaultContextBudget)
	parent := m.RegisterFeature("parent", "Parent")
	parent.SetStatus("running")

	req := &SpawnRequest{
		Title:    "Child",
		MaxDepth: 2,
	}
	child, err := m.SpawnSubFeature("parent", req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if child.MaxDepth != 2 {
		t.Errorf("expected MaxDepth 2, got %d", child.MaxDepth)
	}
}

func TestGetFeatureTree(t *testing.T) {
	m := NewManager()
	root := m.RegisterFeature("root", "Root")
	root.SetStatus("running")

	req1 := &SpawnRequest{Title: "Child 1"}
	child1, _ := m.SpawnSubFeature("root", req1)
	child1.SetStatus("running")

	req2 := &SpawnRequest{Title: "Child 2"}
	m.SpawnSubFeature("root", req2)

	req3 := &SpawnRequest{Title: "Grandchild"}
	m.SpawnSubFeature(child1.ID, req3)

	tree := m.GetFeatureTree("root")
	if len(tree) != 4 {
		t.Errorf("expected 4 features in tree, got %d", len(tree))
	}
}

func TestManagerGetTotalTokenUsage(t *testing.T) {
	m := NewManager()
	root := m.RegisterFeature("root", "Root")
	root.SetStatus("running")
	root.TokenUsage.Update(1000, 500, 0, 0, 0.1)

	req := &SpawnRequest{Title: "Child"}
	child, _ := m.SpawnSubFeature("root", req)
	child.TokenUsage.Update(500, 250, 0, 0, 0.05)

	total := m.GetTotalTokenUsage("root")
	snapshot := total.GetSnapshot()

	if snapshot.InputTokens != 1500 {
		t.Errorf("expected InputTokens 1500, got %d", snapshot.InputTokens)
	}
	if snapshot.OutputTokens != 750 {
		t.Errorf("expected OutputTokens 750, got %d", snapshot.OutputTokens)
	}
}

func TestGetAllActions(t *testing.T) {
	m := NewManager()
	root := m.RegisterFeature("root", "Root")
	root.SetStatus("running")
	root.AddAction(Action{Type: "file_modify", Name: "Write"})
	root.AddAction(Action{Type: "command", Name: "Bash"})

	req := &SpawnRequest{Title: "Child"}
	child, _ := m.SpawnSubFeature("root", req)
	child.AddAction(Action{Type: "web_fetch", Name: "WebFetch"})

	actions := m.GetAllActions("root")
	if len(actions) != 3 {
		t.Errorf("expected 3 actions, got %d", len(actions))
	}
}

func TestGetRootFeatures(t *testing.T) {
	m := NewManager()
	m.RegisterFeature("root1", "Root 1")
	root2 := m.RegisterFeature("root2", "Root 2")
	root2.SetStatus("running")

	req := &SpawnRequest{Title: "Child"}
	m.SpawnSubFeature("root2", req)

	roots := m.GetRootFeatures()
	if len(roots) != 2 {
		t.Errorf("expected 2 root features, got %d", len(roots))
	}
}

func TestCompleteSubFeature(t *testing.T) {
	m := NewManager()
	parent := m.RegisterFeature("parent", "Parent")
	parent.SetStatus("running")

	req := &SpawnRequest{Title: "Child"}
	child, _ := m.SpawnSubFeature("parent", req)
	child.TokenUsage.Update(100, 50, 0, 0, 0.01)

	result := m.CompleteSubFeature(child.ID, "completed", "Task completed successfully")

	if result == nil {
		t.Fatal("expected result, got nil")
	}
	if result.Status != "completed" {
		t.Errorf("expected status 'completed', got '%s'", result.Status)
	}
	if result.Summary != "Task completed successfully" {
		t.Errorf("unexpected summary: %s", result.Summary)
	}
	if child.GetStatus() != "completed" {
		t.Errorf("expected feature status 'completed', got '%s'", child.GetStatus())
	}
}

func TestCompleteSubFeatureFailed(t *testing.T) {
	m := NewManager()
	parent := m.RegisterFeature("parent", "Parent")
	parent.SetStatus("running")

	req := &SpawnRequest{Title: "Child"}
	child, _ := m.SpawnSubFeature("parent", req)

	result := m.CompleteSubFeature(child.ID, "failed", "Something went wrong")

	if result.Error != "Something went wrong" {
		t.Errorf("expected error message, got '%s'", result.Error)
	}
}

func TestCompleteSubFeatureNotFound(t *testing.T) {
	m := NewManager()
	result := m.CompleteSubFeature("nonexistent", "completed", "done")
	if result != nil {
		t.Error("expected nil result for nonexistent feature")
	}
}

func TestGenerateSpawnResultContext(t *testing.T) {
	m := NewManager()

	usage := NewTokenUsage()
	usage.Update(100, 50, 0, 0, 0.01)

	result := &SpawnResult{
		FeatureID:  "child-id",
		Title:      "Child Feature",
		Status:     "completed",
		Summary:    "Implemented helper module",
		TokenUsage: usage,
	}

	context := m.GenerateSpawnResultContext(result)

	if context == "" {
		t.Error("expected non-empty context")
	}
	if !contains(context, "child-id") {
		t.Error("context should contain feature ID")
	}
	if !contains(context, "Child Feature") {
		t.Error("context should contain title")
	}
	if !contains(context, "completed") {
		t.Error("context should contain status")
	}
}

func TestGenerateSpawnResultContextNil(t *testing.T) {
	m := NewManager()
	context := m.GenerateSpawnResultContext(nil)
	if context != "" {
		t.Error("expected empty context for nil result")
	}
}

func TestSetMaxDepth(t *testing.T) {
	m := NewManager()
	m.SetMaxDepth(10)

	f := m.RegisterFeature("test", "Test")
	if f.MaxDepth != 10 {
		t.Errorf("expected MaxDepth 10, got %d", f.MaxDepth)
	}
}

func TestSetMaxDepthInvalid(t *testing.T) {
	m := NewManager()
	original := m.maxDepth
	m.SetMaxDepth(0)
	m.SetMaxDepth(-1)

	if m.maxDepth != original {
		t.Error("maxDepth should not change for invalid values")
	}
}

func TestSetContextBudget(t *testing.T) {
	m := NewManager()
	m.SetContextBudget(200000)

	f := m.RegisterFeature("test", "Test")
	if f.ContextBudget != 200000 {
		t.Errorf("expected ContextBudget 200000, got %d", f.ContextBudget)
	}
}

func TestClearFeature(t *testing.T) {
	m := NewManager()
	m.RegisterFeature("test", "Test")

	m.ClearFeature("test")

	if m.GetFeature("test") != nil {
		t.Error("feature should be cleared")
	}
	if m.GetTracker("test") != nil {
		t.Error("tracker should be cleared")
	}
}

func TestManagerConcurrency(t *testing.T) {
	m := NewManager()
	var wg sync.WaitGroup

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			id := string(rune('a' + n%26))
			m.RegisterFeature(id, "Feature "+id)
		}(i)
	}

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			id := string(rune('a' + n%26))
			_ = m.GetFeature(id)
		}(i)
	}

	wg.Wait()
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
