package runner

import (
	"testing"
	"time"

	"github.com/vx/ralph-go/internal/rlm"
)

func TestNewChildExecutor(t *testing.T) {
	mgr := NewManager("/tmp")
	rlmMgr := rlm.NewManager()
	spawnHandler := rlm.NewSpawnHandler(rlmMgr, nil)

	ce := NewChildExecutor(mgr, spawnHandler)
	if ce == nil {
		t.Fatal("expected non-nil child executor")
	}
	if ce.manager != mgr {
		t.Error("manager not set correctly")
	}
	if ce.spawnHandler != spawnHandler {
		t.Error("spawnHandler not set correctly")
	}
}

func TestChildExecutorHandleSpawnRequest(t *testing.T) {
	mgr := NewManager("/tmp")
	rlmMgr := rlm.NewManager()
	spawnHandler := rlm.NewSpawnHandler(rlmMgr, nil)
	ce := NewChildExecutor(mgr, spawnHandler)

	req := &rlm.SpawnRequest{
		Title: "Test Child",
		Tasks: []string{"task1", "task2"},
		Model: "haiku",
	}

	err := ce.HandleSpawnRequest("parent-123", req)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	ce.mu.RLock()
	pending := ce.pendingChildren["parent-123"]
	ce.mu.RUnlock()

	if len(pending) != 1 {
		t.Fatalf("expected 1 pending child, got %d", len(pending))
	}
	if pending[0].Title != "Test Child" {
		t.Errorf("expected title 'Test Child', got '%s'", pending[0].Title)
	}
}

func TestChildExecutorHandleSpawnRequestNil(t *testing.T) {
	mgr := NewManager("/tmp")
	rlmMgr := rlm.NewManager()
	spawnHandler := rlm.NewSpawnHandler(rlmMgr, nil)
	ce := NewChildExecutor(mgr, spawnHandler)

	err := ce.HandleSpawnRequest("parent-123", nil)
	if err != nil {
		t.Fatalf("expected no error for nil request, got: %v", err)
	}
}

func TestChildExecutorPauseResumeParent(t *testing.T) {
	mgr := NewManager("/tmp")
	rlmMgr := rlm.NewManager()
	spawnHandler := rlm.NewSpawnHandler(rlmMgr, nil)
	ce := NewChildExecutor(mgr, spawnHandler)

	parentID := "parent-456"

	if ce.IsParentPaused(parentID) {
		t.Error("parent should not be paused initially")
	}

	ce.PauseParent(parentID)
	if !ce.IsParentPaused(parentID) {
		t.Error("parent should be paused after PauseParent")
	}

	ce.ResumeParent(parentID)
	if ce.IsParentPaused(parentID) {
		t.Error("parent should not be paused after ResumeParent")
	}
}

func TestChildExecutorIsChildFeature(t *testing.T) {
	mgr := NewManager("/tmp")
	rlmMgr := rlm.NewManager()
	spawnHandler := rlm.NewSpawnHandler(rlmMgr, nil)
	ce := NewChildExecutor(mgr, spawnHandler)

	// Initially no child mappings
	if ce.IsChildFeature("child-123") {
		t.Error("should not be a child feature initially")
	}

	// Add mapping
	ce.mu.Lock()
	ce.childToParent["child-123"] = "parent-456"
	ce.mu.Unlock()

	if !ce.IsChildFeature("child-123") {
		t.Error("should be a child feature after mapping")
	}
}

func TestChildExecutorGetParentID(t *testing.T) {
	mgr := NewManager("/tmp")
	rlmMgr := rlm.NewManager()
	spawnHandler := rlm.NewSpawnHandler(rlmMgr, nil)
	ce := NewChildExecutor(mgr, spawnHandler)

	// No mapping
	if parentID := ce.GetParentID("child-123"); parentID != "" {
		t.Errorf("expected empty parent ID, got '%s'", parentID)
	}

	// With mapping
	ce.mu.Lock()
	ce.childToParent["child-123"] = "parent-456"
	ce.mu.Unlock()

	if parentID := ce.GetParentID("child-123"); parentID != "parent-456" {
		t.Errorf("expected 'parent-456', got '%s'", parentID)
	}
}

func TestChildExecutorGetRunningChildren(t *testing.T) {
	mgr := NewManager("/tmp")
	rlmMgr := rlm.NewManager()
	spawnHandler := rlm.NewSpawnHandler(rlmMgr, nil)
	ce := NewChildExecutor(mgr, spawnHandler)

	// No running children
	children := ce.GetRunningChildren("parent-456")
	if len(children) != 0 {
		t.Errorf("expected 0 running children, got %d", len(children))
	}

	// Add running children
	ce.mu.Lock()
	ce.runningChildren["parent-456"] = []string{"child-1", "child-2"}
	ce.mu.Unlock()

	children = ce.GetRunningChildren("parent-456")
	if len(children) != 2 {
		t.Errorf("expected 2 running children, got %d", len(children))
	}
}

func TestChildExecutorHasRunningChildren(t *testing.T) {
	mgr := NewManager("/tmp")
	rlmMgr := rlm.NewManager()
	spawnHandler := rlm.NewSpawnHandler(rlmMgr, nil)
	ce := NewChildExecutor(mgr, spawnHandler)

	if ce.HasRunningChildren("parent-456") {
		t.Error("should have no running children initially")
	}

	ce.mu.Lock()
	ce.runningChildren["parent-456"] = []string{"child-1"}
	ce.mu.Unlock()

	if !ce.HasRunningChildren("parent-456") {
		t.Error("should have running children after adding")
	}
}

func TestChildExecutorAllChildrenComplete(t *testing.T) {
	mgr := NewManager("/tmp")
	rlmMgr := rlm.NewManager()
	spawnHandler := rlm.NewSpawnHandler(rlmMgr, nil)
	ce := NewChildExecutor(mgr, spawnHandler)

	// No children at all - complete
	if !ce.AllChildrenComplete("parent-456") {
		t.Error("should be complete with no children")
	}

	// Pending children - not complete
	ce.mu.Lock()
	ce.pendingChildren["parent-456"] = []*rlm.SpawnRequest{{Title: "test"}}
	ce.mu.Unlock()

	if ce.AllChildrenComplete("parent-456") {
		t.Error("should not be complete with pending children")
	}

	// Clear pending, add running - not complete
	ce.mu.Lock()
	ce.pendingChildren["parent-456"] = nil
	ce.runningChildren["parent-456"] = []string{"child-1"}
	ce.mu.Unlock()

	if ce.AllChildrenComplete("parent-456") {
		t.Error("should not be complete with running children")
	}

	// Clear all - complete
	ce.mu.Lock()
	ce.runningChildren["parent-456"] = nil
	ce.mu.Unlock()

	if !ce.AllChildrenComplete("parent-456") {
		t.Error("should be complete with no pending or running children")
	}
}

func TestChildExecutorShouldPauseForChildren(t *testing.T) {
	mgr := NewManager("/tmp")
	rlmMgr := rlm.NewManager()
	spawnHandler := rlm.NewSpawnHandler(rlmMgr, nil)
	ce := NewChildExecutor(mgr, spawnHandler)

	// No children - don't pause
	if ce.ShouldPauseForChildren("parent-456") {
		t.Error("should not pause with no children")
	}

	// Pending children - pause
	ce.mu.Lock()
	ce.pendingChildren["parent-456"] = []*rlm.SpawnRequest{{Title: "test"}}
	ce.mu.Unlock()

	if !ce.ShouldPauseForChildren("parent-456") {
		t.Error("should pause with pending children")
	}

	// Running children - pause
	ce.mu.Lock()
	ce.pendingChildren["parent-456"] = nil
	ce.runningChildren["parent-456"] = []string{"child-1"}
	ce.mu.Unlock()

	if !ce.ShouldPauseForChildren("parent-456") {
		t.Error("should pause with running children")
	}
}

func TestChildExecutorSetOnChildComplete(t *testing.T) {
	mgr := NewManager("/tmp")
	rlmMgr := rlm.NewManager()
	spawnHandler := rlm.NewSpawnHandler(rlmMgr, nil)
	ce := NewChildExecutor(mgr, spawnHandler)

	callback := func(parentID string, result *rlm.SpawnResult) {
		// callback set
	}

	ce.SetOnChildComplete(callback)

	ce.mu.RLock()
	if ce.onChildComplete == nil {
		t.Error("callback should be set")
	}
	ce.mu.RUnlock()
}

func TestChildExecutorOnChildComplete(t *testing.T) {
	mgr := NewManager("/tmp")
	rlmMgr := rlm.NewManager()
	spawnHandler := rlm.NewSpawnHandler(rlmMgr, nil)
	ce := NewChildExecutor(mgr, spawnHandler)

	// Setup parent/child mapping
	parentID := "parent-123"
	childID := "child-456"

	ce.mu.Lock()
	ce.childToParent[childID] = parentID
	ce.runningChildren[parentID] = []string{childID}
	ce.mu.Unlock()

	// Track callback
	var callbackParentID string
	var callbackResult *rlm.SpawnResult
	ce.SetOnChildComplete(func(parentID string, result *rlm.SpawnResult) {
		callbackParentID = parentID
		callbackResult = result
	})

	result := ce.OnChildComplete(childID, "completed", "Test completed successfully")

	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.FeatureID != childID {
		t.Errorf("expected feature ID '%s', got '%s'", childID, result.FeatureID)
	}
	if result.Status != "completed" {
		t.Errorf("expected status 'completed', got '%s'", result.Status)
	}

	// Check callback was called
	if callbackParentID != parentID {
		t.Errorf("callback parent ID mismatch: expected '%s', got '%s'", parentID, callbackParentID)
	}
	if callbackResult == nil {
		t.Error("callback result should not be nil")
	}

	// Check child was removed from running
	if ce.HasRunningChildren(parentID) {
		t.Error("child should be removed from running children")
	}

	// Check child-to-parent mapping removed
	if ce.IsChildFeature(childID) {
		t.Error("child-to-parent mapping should be removed")
	}
}

func TestChildExecutorOnChildCompleteFailure(t *testing.T) {
	mgr := NewManager("/tmp")
	rlmMgr := rlm.NewManager()
	spawnHandler := rlm.NewSpawnHandler(rlmMgr, nil)
	ce := NewChildExecutor(mgr, spawnHandler)

	parentID := "parent-123"
	childID := "child-456"

	ce.mu.Lock()
	ce.childToParent[childID] = parentID
	ce.runningChildren[parentID] = []string{childID}
	ce.mu.Unlock()

	result := ce.OnChildComplete(childID, "failed", "Test failed with error")

	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.Status != "failed" {
		t.Errorf("expected status 'failed', got '%s'", result.Status)
	}
	if result.Error != "Test failed with error" {
		t.Errorf("expected error message, got '%s'", result.Error)
	}
}

func TestChildExecutorOnChildCompleteNoParent(t *testing.T) {
	mgr := NewManager("/tmp")
	rlmMgr := rlm.NewManager()
	spawnHandler := rlm.NewSpawnHandler(rlmMgr, nil)
	ce := NewChildExecutor(mgr, spawnHandler)

	// No parent mapping
	result := ce.OnChildComplete("orphan-child", "completed", "Done")

	if result != nil {
		t.Error("expected nil result for orphan child")
	}
}

func TestChildExecutorClearParent(t *testing.T) {
	mgr := NewManager("/tmp")
	rlmMgr := rlm.NewManager()
	spawnHandler := rlm.NewSpawnHandler(rlmMgr, nil)
	ce := NewChildExecutor(mgr, spawnHandler)

	parentID := "parent-123"

	// Setup state
	ce.mu.Lock()
	ce.pendingChildren[parentID] = []*rlm.SpawnRequest{{Title: "test"}}
	ce.runningChildren[parentID] = []string{"child-1", "child-2"}
	ce.childToParent["child-1"] = parentID
	ce.childToParent["child-2"] = parentID
	ce.pausedParents[parentID] = true
	ce.mu.Unlock()

	ce.storeResultContext(parentID, "some result")

	ce.ClearParent(parentID)

	ce.mu.RLock()
	defer ce.mu.RUnlock()

	if len(ce.pendingChildren[parentID]) != 0 {
		t.Error("pending children should be cleared")
	}
	if len(ce.runningChildren[parentID]) != 0 {
		t.Error("running children should be cleared")
	}
	if ce.pausedParents[parentID] {
		t.Error("paused state should be cleared")
	}
	if _, ok := ce.childToParent["child-1"]; ok {
		t.Error("child-to-parent mapping should be cleared")
	}
	if _, ok := ce.childToParent["child-2"]; ok {
		t.Error("child-to-parent mapping should be cleared")
	}
}

func TestChildExecutorResultContextStorage(t *testing.T) {
	mgr := NewManager("/tmp")
	rlmMgr := rlm.NewManager()
	spawnHandler := rlm.NewSpawnHandler(rlmMgr, nil)
	ce := NewChildExecutor(mgr, spawnHandler)

	parentID := "parent-123"

	// Store multiple results
	ce.storeResultContext(parentID, "result 1")
	ce.storeResultContext(parentID, "result 2")

	contexts := ce.GetPendingResultContexts(parentID)
	if len(contexts) != 2 {
		t.Errorf("expected 2 contexts, got %d", len(contexts))
	}
	if contexts[0] != "result 1" {
		t.Errorf("expected 'result 1', got '%s'", contexts[0])
	}

	// Should be cleared after retrieval
	contexts = ce.GetPendingResultContexts(parentID)
	if len(contexts) != 0 {
		t.Error("contexts should be cleared after retrieval")
	}
}

func TestChildExecutorGenerateChildResultSummary(t *testing.T) {
	mgr := NewManager("/tmp")
	rlmMgr := rlm.NewManager()
	spawnHandler := rlm.NewSpawnHandler(rlmMgr, nil)
	ce := NewChildExecutor(mgr, spawnHandler)

	parentID := "parent-123"

	// No results
	summary := ce.GenerateChildResultSummary(parentID)
	if summary != "" {
		t.Error("expected empty summary with no results")
	}

	// With results
	ce.storeResultContext(parentID, `{"child": "result1"}`)
	ce.storeResultContext(parentID, `{"child": "result2"}`)

	summary = ce.GenerateChildResultSummary(parentID)
	if summary == "" {
		t.Error("expected non-empty summary")
	}
	if !containsString(summary, "Sub-Feature Results") {
		t.Error("summary should contain 'Sub-Feature Results' header")
	}
}

func TestChildExecutorWaitForChildrenTimeout(t *testing.T) {
	mgr := NewManager("/tmp")
	rlmMgr := rlm.NewManager()
	spawnHandler := rlm.NewSpawnHandler(rlmMgr, nil)
	ce := NewChildExecutor(mgr, spawnHandler)

	parentID := "parent-123"

	// Add running child
	ce.mu.Lock()
	ce.runningChildren[parentID] = []string{"child-1"}
	ce.mu.Unlock()

	// Should timeout
	start := time.Now()
	completed := ce.WaitForChildren(parentID, 100*time.Millisecond)
	elapsed := time.Since(start)

	if completed {
		t.Error("should not have completed")
	}
	if elapsed < 100*time.Millisecond {
		t.Error("should have waited at least 100ms")
	}
}

func TestChildExecutorWaitForChildrenComplete(t *testing.T) {
	mgr := NewManager("/tmp")
	rlmMgr := rlm.NewManager()
	spawnHandler := rlm.NewSpawnHandler(rlmMgr, nil)
	ce := NewChildExecutor(mgr, spawnHandler)

	parentID := "parent-123"

	// No children - should complete immediately
	completed := ce.WaitForChildren(parentID, 100*time.Millisecond)
	if !completed {
		t.Error("should have completed with no children")
	}
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsStringHelper(s, substr))
}

func containsStringHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// Fault isolation tests

func TestChildExecutorSetOnChildFailure(t *testing.T) {
	mgr := NewManager("/tmp")
	rlmMgr := rlm.NewManager()
	spawnHandler := rlm.NewSpawnHandler(rlmMgr, nil)
	ce := NewChildExecutor(mgr, spawnHandler)

	callback := func(parentID string, result *rlm.ChildFailureResult) rlm.ChildFailureAction {
		return rlm.ChildFailureHandle
	}

	ce.SetOnChildFailure(callback)

	ce.mu.RLock()
	if ce.onChildFailure == nil {
		t.Error("callback should be set")
	}
	ce.mu.RUnlock()
}

func TestChildExecutorRecordChildFailure(t *testing.T) {
	mgr := NewManager("/tmp")
	rlmMgr := rlm.NewManager()
	spawnHandler := rlm.NewSpawnHandler(rlmMgr, nil)
	ce := NewChildExecutor(mgr, spawnHandler)

	childID := "child-123"
	parentID := "parent-456"
	reason := "test_failure"
	errMsg := "something went wrong"

	result := ce.RecordChildFailure(childID, parentID, reason, errMsg)

	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.ChildID != childID {
		t.Errorf("ChildID = %q, want %q", result.ChildID, childID)
	}
	if result.ParentID != parentID {
		t.Errorf("ParentID = %q, want %q", result.ParentID, parentID)
	}
	if result.FailureInfo == nil {
		t.Fatal("expected non-nil FailureInfo")
	}
	if result.FailureInfo.Reason != reason {
		t.Errorf("Reason = %q, want %q", result.FailureInfo.Reason, reason)
	}
	if result.FailureInfo.Error != errMsg {
		t.Errorf("Error = %q, want %q", result.FailureInfo.Error, errMsg)
	}
	if result.Action != rlm.ChildFailureHandle {
		t.Errorf("Action = %q, want %q", result.Action, rlm.ChildFailureHandle)
	}

	// Check it was recorded
	failures := ce.GetFailedChildren(parentID)
	if len(failures) != 1 {
		t.Fatalf("Expected 1 failed child, got %d", len(failures))
	}
}

func TestChildExecutorRecordChildFailureWithCallback(t *testing.T) {
	mgr := NewManager("/tmp")
	rlmMgr := rlm.NewManager()
	spawnHandler := rlm.NewSpawnHandler(rlmMgr, nil)
	ce := NewChildExecutor(mgr, spawnHandler)

	var callbackCalled bool
	var callbackParentID string
	ce.SetOnChildFailure(func(parentID string, result *rlm.ChildFailureResult) rlm.ChildFailureAction {
		callbackCalled = true
		callbackParentID = parentID
		return rlm.ChildFailureSkip
	})

	result := ce.RecordChildFailure("child-123", "parent-456", "test", "error")

	if !callbackCalled {
		t.Error("callback should have been called")
	}
	if callbackParentID != "parent-456" {
		t.Errorf("callbackParentID = %q, want %q", callbackParentID, "parent-456")
	}
	if result.Action != rlm.ChildFailureSkip {
		t.Errorf("Action = %q, want %q", result.Action, rlm.ChildFailureSkip)
	}
}

func TestChildExecutorGetFailedChildren(t *testing.T) {
	mgr := NewManager("/tmp")
	rlmMgr := rlm.NewManager()
	spawnHandler := rlm.NewSpawnHandler(rlmMgr, nil)
	ce := NewChildExecutor(mgr, spawnHandler)

	parentID := "parent-456"

	// Initially empty
	failures := ce.GetFailedChildren(parentID)
	if failures != nil {
		t.Error("expected nil for no failures")
	}

	// Record failures
	ce.RecordChildFailure("child-1", parentID, "reason1", "error1")
	ce.RecordChildFailure("child-2", parentID, "reason2", "error2")

	failures = ce.GetFailedChildren(parentID)
	if len(failures) != 2 {
		t.Fatalf("Expected 2 failed children, got %d", len(failures))
	}
}

func TestChildExecutorHasFailedChildrenInExecutor(t *testing.T) {
	mgr := NewManager("/tmp")
	rlmMgr := rlm.NewManager()
	spawnHandler := rlm.NewSpawnHandler(rlmMgr, nil)
	ce := NewChildExecutor(mgr, spawnHandler)

	parentID := "parent-456"

	if ce.HasFailedChildrenInExecutor(parentID) {
		t.Error("should have no failed children initially")
	}

	ce.RecordChildFailure("child-1", parentID, "reason", "error")

	if !ce.HasFailedChildrenInExecutor(parentID) {
		t.Error("should have failed children after recording failure")
	}
}

func TestChildExecutorSkipChild(t *testing.T) {
	mgr := NewManager("/tmp")
	rlmMgr := rlm.NewManager()
	spawnHandler := rlm.NewSpawnHandler(rlmMgr, nil)
	ce := NewChildExecutor(mgr, spawnHandler)

	parentID := "parent-456"
	childID := "child-123"

	// Record a failure first
	ce.RecordChildFailure(childID, parentID, "test", "error")

	// Verify failure was recorded
	if !ce.HasFailedChildrenInExecutor(parentID) {
		t.Fatal("expected failed child to be recorded")
	}

	// Skip the child
	err := ce.SkipChild(childID, "skipping for test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check it was removed from failed and added to skipped
	if ce.HasFailedChildrenInExecutor(parentID) {
		t.Error("child should be removed from failed list")
	}

	skipped := ce.GetSkippedChildren(parentID)
	if len(skipped) != 1 {
		t.Fatalf("Expected 1 skipped child, got %d", len(skipped))
	}
	if skipped[0] != childID {
		t.Errorf("Skipped child = %q, want %q", skipped[0], childID)
	}
}

func TestChildExecutorSkipChildNotFound(t *testing.T) {
	mgr := NewManager("/tmp")
	rlmMgr := rlm.NewManager()
	spawnHandler := rlm.NewSpawnHandler(rlmMgr, nil)
	ce := NewChildExecutor(mgr, spawnHandler)

	err := ce.SkipChild("unknown-child", "test reason")
	if err == nil {
		t.Error("expected error for unknown child")
	}
}

func TestChildExecutorGetSkippedChildren(t *testing.T) {
	mgr := NewManager("/tmp")
	rlmMgr := rlm.NewManager()
	spawnHandler := rlm.NewSpawnHandler(rlmMgr, nil)
	ce := NewChildExecutor(mgr, spawnHandler)

	parentID := "parent-456"

	// Initially empty
	skipped := ce.GetSkippedChildren(parentID)
	if skipped != nil {
		t.Error("expected nil for no skipped children")
	}

	// Add skipped children manually
	ce.mu.Lock()
	ce.skippedChildren[parentID] = []string{"child-1", "child-2"}
	ce.mu.Unlock()

	skipped = ce.GetSkippedChildren(parentID)
	if len(skipped) != 2 {
		t.Fatalf("Expected 2 skipped children, got %d", len(skipped))
	}
}

func TestChildExecutorDetermineChildFailureAction(t *testing.T) {
	mgr := NewManager("/tmp")
	rlmMgr := rlm.NewManager()
	spawnHandler := rlm.NewSpawnHandler(rlmMgr, nil)
	ce := NewChildExecutor(mgr, spawnHandler)

	tests := []struct {
		name     string
		level    rlm.IsolationLevel
		expected rlm.ChildFailureAction
	}{
		{
			name:     "strict isolation aborts",
			level:    rlm.IsolationStrict,
			expected: rlm.ChildFailureAbort,
		},
		{
			name:     "lenient isolation handles",
			level:    rlm.IsolationLenient,
			expected: rlm.ChildFailureHandle,
		},
		{
			name:     "empty isolation handles",
			level:    "",
			expected: rlm.ChildFailureHandle,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			action := ce.DetermineChildFailureAction("child", "parent", tt.level)
			if action != tt.expected {
				t.Errorf("DetermineChildFailureAction() = %q, want %q", action, tt.expected)
			}
		})
	}
}

func TestChildExecutorGenerateFailureSummary(t *testing.T) {
	mgr := NewManager("/tmp")
	rlmMgr := rlm.NewManager()
	spawnHandler := rlm.NewSpawnHandler(rlmMgr, nil)
	ce := NewChildExecutor(mgr, spawnHandler)

	parentID := "parent-456"

	// Empty summary
	summary := ce.GenerateFailureSummary(parentID)
	if summary != "" {
		t.Error("expected empty summary with no failures")
	}

	// Add failures
	ce.RecordChildFailure("child-1", parentID, "reason1", "error1")

	summary = ce.GenerateFailureSummary(parentID)
	if summary == "" {
		t.Error("expected non-empty summary")
	}
	if !containsString(summary, "Child Feature Status") {
		t.Error("summary should contain 'Child Feature Status'")
	}
	if !containsString(summary, "Failed Children") {
		t.Error("summary should contain 'Failed Children'")
	}

	// Add skipped
	ce.mu.Lock()
	ce.skippedChildren[parentID] = []string{"child-2"}
	ce.mu.Unlock()

	summary = ce.GenerateFailureSummary(parentID)
	if !containsString(summary, "Skipped Children") {
		t.Error("summary should contain 'Skipped Children'")
	}
}

func TestChildExecutorClearParentWithFaultIsolation(t *testing.T) {
	mgr := NewManager("/tmp")
	rlmMgr := rlm.NewManager()
	spawnHandler := rlm.NewSpawnHandler(rlmMgr, nil)
	ce := NewChildExecutor(mgr, spawnHandler)

	parentID := "parent-123"

	// Setup state including fault isolation data
	ce.mu.Lock()
	ce.pendingChildren[parentID] = []*rlm.SpawnRequest{{Title: "test"}}
	ce.runningChildren[parentID] = []string{"child-1"}
	ce.childToParent["child-1"] = parentID
	ce.pausedParents[parentID] = true
	ce.failedChildren[parentID] = []*rlm.ChildFailureResult{{ChildID: "child-2"}}
	ce.skippedChildren[parentID] = []string{"child-3"}
	ce.mu.Unlock()

	ce.ClearParent(parentID)

	ce.mu.RLock()
	defer ce.mu.RUnlock()

	if len(ce.failedChildren[parentID]) != 0 {
		t.Error("failed children should be cleared")
	}
	if len(ce.skippedChildren[parentID]) != 0 {
		t.Error("skipped children should be cleared")
	}
}

func TestChildExecutorRetryChildNoParent(t *testing.T) {
	mgr := NewManager("/tmp")
	rlmMgr := rlm.NewManager()
	spawnHandler := rlm.NewSpawnHandler(rlmMgr, nil)
	ce := NewChildExecutor(mgr, spawnHandler)

	err := ce.RetryChild("unknown-child", nil)
	if err == nil {
		t.Error("expected error for unknown child")
	}
}
