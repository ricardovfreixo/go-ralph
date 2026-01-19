package rlm

import (
	"strings"
	"testing"
)

func TestManualRecursivePRDScenario(t *testing.T) {
	manager := NewManager()

	rootFeature := manager.RegisterFeature("calc-lib", "Calculator Library")
	rootFeature.SetIsolationLevel(IsolationLenient)
	rootFeature.SetStatus("running")

	rootFeature.Tasks = []RecursiveTask{
		{ID: "task-1", Description: "Create a calculator package with Add, Subtract, Multiply, Divide functions"},
		{ID: "task-2", Description: "Add unit tests for each operation"},
		{ID: "task-3", Description: "Handle division by zero gracefully"},
	}

	if rootFeature.GetStatus() != "running" {
		t.Errorf("Root feature status = %q, want running", rootFeature.GetStatus())
	}

	if !rootFeature.IsRoot() {
		t.Error("Root feature should be root")
	}

	spawnReq := &SpawnRequest{
		Title:    "Implement Division Safety",
		Tasks:    []string{"Add zero check", "Return error on division by zero", "Add tests for division by zero"},
		Model:    "haiku",
		MaxDepth: 2,
	}

	child, err := manager.SpawnSubFeature("calc-lib", spawnReq)
	if err != nil {
		t.Fatalf("SpawnSubFeature failed: %v", err)
	}

	if child.ParentID != "calc-lib" {
		t.Errorf("Child ParentID = %q, want calc-lib", child.ParentID)
	}

	if child.Depth != 1 {
		t.Errorf("Child depth = %d, want 1", child.Depth)
	}

	if len(child.Tasks) != 3 {
		t.Errorf("Child has %d tasks, want 3", len(child.Tasks))
	}

	child.SetStatus("running")

	sampleOutput := `{"type":"assistant","message":{"content":"I'll implement the division safety checks."}}`
	_, err = manager.ProcessOutput(child.ID, sampleOutput)
	if err != nil {
		t.Errorf("ProcessOutput failed: %v", err)
	}

	usageOutput := `{"type":"usage","usage":{"input_tokens":500,"output_tokens":100}}`
	_, err = manager.ProcessOutput(child.ID, usageOutput)
	if err != nil {
		t.Errorf("ProcessOutput usage failed: %v", err)
	}

	result := manager.CompleteSubFeature(child.ID, "completed", "Division safety checks implemented successfully")
	if result == nil {
		t.Fatal("CompleteSubFeature returned nil")
	}

	if result.Status != "completed" {
		t.Errorf("Result status = %q, want completed", result.Status)
	}

	context := manager.GenerateSpawnResultContext(result)
	if !strings.Contains(context, "sub_feature_completed") {
		t.Error("Context should contain sub_feature_completed")
	}
	if !strings.Contains(context, "Division safety") {
		t.Error("Context should contain feature title")
	}

	tree := manager.GetFeatureTree("calc-lib")
	if len(tree) != 2 {
		t.Errorf("Feature tree has %d features, want 2", len(tree))
	}

	subFeatures := manager.GetSubFeatures("calc-lib")
	if len(subFeatures) != 1 {
		t.Errorf("SubFeatures count = %d, want 1", len(subFeatures))
	}

	rootFeature.SetStatus("completed")

	roots := manager.GetRootFeatures()
	if len(roots) != 1 {
		t.Errorf("Root features count = %d, want 1", len(roots))
	}
}

func TestManualStrictIsolationScenario(t *testing.T) {
	manager := NewManager()

	parent := manager.RegisterFeature("parent-strict", "Parent with Strict Isolation")
	parent.SetIsolationLevel(IsolationStrict)
	parent.SetStatus("running")

	child, err := manager.SpawnSubFeature("parent-strict", &SpawnRequest{
		Title: "Child Feature",
		Tasks: []string{"Task 1"},
	})
	if err != nil {
		t.Fatalf("SpawnSubFeature failed: %v", err)
	}

	child.SetStatus("running")
	child.RecordFailure("execution_error", "Something went wrong")

	parent.AddFailedChild(child.ID)

	if !parent.HasFailedChildren() {
		t.Error("Parent should have failed children")
	}

	if !parent.ShouldFailOnChildFailure() {
		t.Error("Parent with strict isolation should fail on child failure")
	}

	failedChildren := parent.GetFailedChildren()
	if len(failedChildren) != 1 {
		t.Errorf("Failed children count = %d, want 1", len(failedChildren))
	}

	childFailure := child.GetFailureInfo()
	if childFailure == nil {
		t.Fatal("Child should have failure info")
	}

	if !childFailure.CanRetry() {
		t.Error("Initial failure should be retryable")
	}

	for i := 0; i < 3; i++ {
		childFailure.IncrementRetry()
	}

	if childFailure.CanRetry() {
		t.Error("After max retries, failure should not be retryable")
	}
}

func TestManualLenientIsolationScenario(t *testing.T) {
	manager := NewManager()

	parent := manager.RegisterFeature("parent-lenient", "Parent with Lenient Isolation")
	parent.SetIsolationLevel(IsolationLenient)
	parent.SetStatus("running")

	child1, _ := manager.SpawnSubFeature("parent-lenient", &SpawnRequest{
		Title: "Child 1 - Will Fail",
		Tasks: []string{"Task A"},
	})
	child1.SetStatus("running")
	child1.RecordFailure("test_failure", "Intentional failure")
	parent.AddFailedChild(child1.ID)

	child2, _ := manager.SpawnSubFeature("parent-lenient", &SpawnRequest{
		Title: "Child 2 - Will Succeed",
		Tasks: []string{"Task B"},
	})
	child2.SetStatus("completed")

	if !parent.HasFailedChildren() {
		t.Error("Parent should have failed children")
	}

	if parent.ShouldFailOnChildFailure() {
		t.Error("Lenient parent should NOT fail on child failure")
	}

	subFeatures := manager.GetSubFeatures("parent-lenient")
	if len(subFeatures) != 2 {
		t.Errorf("Should have 2 sub-features, got %d", len(subFeatures))
	}

	var completedCount, failedCount int
	for _, sf := range subFeatures {
		switch sf.GetStatus() {
		case "completed":
			completedCount++
		case "failed":
			failedCount++
		}
	}

	if completedCount != 1 {
		t.Errorf("Completed count = %d, want 1", completedCount)
	}
	if failedCount != 1 {
		t.Errorf("Failed count = %d, want 1", failedCount)
	}

	parent.SetStatus("completed")
	if parent.GetStatus() != "completed" {
		t.Error("Lenient parent should be able to complete despite failed children")
	}
}

func TestManualDeepRecursionScenario(t *testing.T) {
	manager := NewManagerWithConfig(5, 100000)

	root := manager.RegisterFeature("deep-root", "Deep Recursion Test")
	root.SetStatus("running")

	var current = root
	var depth = 0
	maxTestDepth := 4

	for i := 0; i < maxTestDepth; i++ {
		child, err := manager.SpawnSubFeature(current.ID, &SpawnRequest{
			Title: "Level " + string(rune('1'+i)),
			Tasks: []string{"Task at depth " + string(rune('1'+i))},
		})

		if err != nil {
			if err == ErrMaxDepthExceeded {
				break
			}
			t.Fatalf("Unexpected error at depth %d: %v", i+1, err)
		}

		child.SetStatus("running")
		current = child
		depth = i + 1
	}

	tree := manager.GetFeatureTree("deep-root")
	expectedCount := depth + 1
	if len(tree) != expectedCount {
		t.Errorf("Feature tree size = %d, want %d", len(tree), expectedCount)
	}

	lastFeature := tree[len(tree)-1]
	if lastFeature.Depth != depth {
		t.Errorf("Last feature depth = %d, want %d", lastFeature.Depth, depth)
	}

	for _, f := range tree {
		t.Logf("Feature: %s, Depth: %d, Budget: %d", f.Title, f.Depth, f.ContextBudget)
	}
}

func TestManualTokenAccumulationScenario(t *testing.T) {
	manager := NewManager()

	root := manager.RegisterFeature("token-test", "Token Accumulation Test")
	root.SetStatus("running")

	outputs := []string{
		`{"type":"usage","usage":{"input_tokens":1000,"output_tokens":200}}`,
		`{"type":"usage","usage":{"input_tokens":1500,"output_tokens":300}}`,
		`{"type":"usage","usage":{"input_tokens":2000,"output_tokens":400}}`,
	}

	for _, out := range outputs {
		_, err := manager.ProcessOutput("token-test", out)
		if err != nil {
			t.Errorf("ProcessOutput failed: %v", err)
		}
	}

	usage := manager.GetTotalTokenUsage("token-test")
	if usage == nil {
		t.Fatal("Token usage should not be nil")
	}

	snapshot := usage.GetSnapshot()

	// Tokens are accumulated: 1000+1500+2000 = 4500 input, 200+300+400 = 900 output
	if snapshot.InputTokens != 4500 {
		t.Errorf("InputTokens = %d, want 4500 (accumulated)", snapshot.InputTokens)
	}
	if snapshot.OutputTokens != 900 {
		t.Errorf("OutputTokens = %d, want 900 (accumulated)", snapshot.OutputTokens)
	}

	child, _ := manager.SpawnSubFeature("token-test", &SpawnRequest{
		Title: "Child for tokens",
		Tasks: []string{"Generate tokens"},
	})
	child.SetStatus("running")

	childOutputs := []string{
		`{"type":"usage","usage":{"input_tokens":500,"output_tokens":100}}`,
		`{"type":"usage","usage":{"input_tokens":800,"output_tokens":150}}`,
	}

	for _, out := range childOutputs {
		_, err := manager.ProcessOutput(child.ID, out)
		if err != nil {
			t.Errorf("ProcessOutput for child failed: %v", err)
		}
	}

	totalUsage := manager.GetTotalTokenUsage("token-test")
	totalSnapshot := totalUsage.GetSnapshot()

	t.Logf("Root tokens: input=%d, output=%d", snapshot.InputTokens, snapshot.OutputTokens)
	t.Logf("Total tree tokens: input=%d, output=%d", totalSnapshot.InputTokens, totalSnapshot.OutputTokens)
}
