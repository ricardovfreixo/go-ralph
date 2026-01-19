package state

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewProgress(t *testing.T) {
	p := NewProgress()

	if p == nil {
		t.Fatal("expected non-nil Progress")
	}
	if p.Version != "0.2.0" {
		t.Errorf("expected version 0.2.0, got %s", p.Version)
	}
	if p.Features == nil {
		t.Error("expected non-nil Features map")
	}
	if p.GlobalState == nil {
		t.Error("expected non-nil GlobalState map")
	}
}

func TestInitFeature(t *testing.T) {
	p := NewProgress()
	p.InitFeature("01", "Test Feature")

	f := p.GetFeature("01")
	if f == nil {
		t.Fatal("expected feature to be initialized")
	}
	if f.ID != "01" {
		t.Errorf("expected ID '01', got %s", f.ID)
	}
	if f.Title != "Test Feature" {
		t.Errorf("expected title 'Test Feature', got %s", f.Title)
	}
	if f.Status != "pending" {
		t.Errorf("expected status 'pending', got %s", f.Status)
	}
}

func TestUpdateFeature(t *testing.T) {
	p := NewProgress()
	p.InitFeature("01", "Test")

	p.UpdateFeature("01", "running")
	f := p.GetFeature("01")
	if f.Status != "running" {
		t.Errorf("expected status 'running', got %s", f.Status)
	}
	if f.StartedAt == nil {
		t.Error("expected StartedAt to be set")
	}
	if f.Attempts != 1 {
		t.Errorf("expected Attempts 1, got %d", f.Attempts)
	}

	p.UpdateFeature("01", "completed")
	f = p.GetFeature("01")
	if f.Status != "completed" {
		t.Errorf("expected status 'completed', got %s", f.Status)
	}
	if f.CompletedAt == nil {
		t.Error("expected CompletedAt to be set")
	}
}

func TestSetFeatureParent(t *testing.T) {
	p := NewProgress()
	p.InitFeature("01", "Parent Feature")
	p.InitFeature("01-child", "Child Feature")

	p.SetFeatureParent("01-child", "01")

	child := p.GetFeature("01-child")
	if child.ParentID != "01" {
		t.Errorf("expected ParentID '01', got %s", child.ParentID)
	}
	if child.Depth != 1 {
		t.Errorf("expected Depth 1, got %d", child.Depth)
	}
}

func TestSetFeatureParentDeepNesting(t *testing.T) {
	p := NewProgress()
	p.InitFeature("01", "Root")
	p.InitFeature("01-a", "Child A")
	p.InitFeature("01-a-1", "Child A-1")
	p.InitFeature("01-a-1-x", "Child A-1-x")

	p.SetFeatureParent("01-a", "01")
	p.SetFeatureParent("01-a-1", "01-a")
	p.SetFeatureParent("01-a-1-x", "01-a-1")

	childA := p.GetFeature("01-a")
	if childA.Depth != 1 {
		t.Errorf("expected Depth 1 for 01-a, got %d", childA.Depth)
	}

	childA1 := p.GetFeature("01-a-1")
	if childA1.Depth != 2 {
		t.Errorf("expected Depth 2 for 01-a-1, got %d", childA1.Depth)
	}

	childA1x := p.GetFeature("01-a-1-x")
	if childA1x.Depth != 3 {
		t.Errorf("expected Depth 3 for 01-a-1-x, got %d", childA1x.Depth)
	}
}

func TestGetFeatureParent(t *testing.T) {
	p := NewProgress()
	p.InitFeature("01", "Parent")
	p.InitFeature("01-child", "Child")
	p.SetFeatureParent("01-child", "01")

	parentID := p.GetFeatureParent("01-child")
	if parentID != "01" {
		t.Errorf("expected parent '01', got %s", parentID)
	}

	parentID = p.GetFeatureParent("01")
	if parentID != "" {
		t.Errorf("expected empty parent for root, got %s", parentID)
	}

	parentID = p.GetFeatureParent("nonexistent")
	if parentID != "" {
		t.Errorf("expected empty parent for nonexistent, got %s", parentID)
	}
}

func TestGetChildFeatures(t *testing.T) {
	p := NewProgress()
	p.InitFeature("01", "Parent")
	p.InitFeature("01-a", "Child A")
	p.InitFeature("01-b", "Child B")
	p.InitFeature("01-a-1", "Grandchild")

	p.SetFeatureParent("01-a", "01")
	p.SetFeatureParent("01-b", "01")
	p.SetFeatureParent("01-a-1", "01-a")

	children := p.GetChildFeatures("01")
	if len(children) != 2 {
		t.Errorf("expected 2 children, got %d", len(children))
	}

	grandchildren := p.GetChildFeatures("01-a")
	if len(grandchildren) != 1 {
		t.Errorf("expected 1 grandchild, got %d", len(grandchildren))
	}

	noChildren := p.GetChildFeatures("01-b")
	if len(noChildren) != 0 {
		t.Errorf("expected 0 children, got %d", len(noChildren))
	}
}

func TestCanRetry(t *testing.T) {
	p := NewProgress()
	p.InitFeature("01", "Test")

	if !p.CanRetry("01") {
		t.Error("should be able to retry with 0 attempts")
	}

	p.UpdateFeature("01", "running")
	p.UpdateFeature("01", "running")
	p.UpdateFeature("01", "running")

	if p.CanRetry("01") {
		t.Error("should not be able to retry after max attempts")
	}
}

func TestGetSummary(t *testing.T) {
	p := NewProgress()
	p.InitFeature("01", "Pending")
	p.InitFeature("02", "Running")
	p.InitFeature("03", "Completed")
	p.InitFeature("04", "Failed")

	p.UpdateFeature("02", "running")
	p.UpdateFeature("03", "completed")
	p.UpdateFeature("04", "failed")

	total, completed, running, failed, pending := p.GetSummary()
	if total != 4 {
		t.Errorf("expected total 4, got %d", total)
	}
	if completed != 1 {
		t.Errorf("expected completed 1, got %d", completed)
	}
	if running != 1 {
		t.Errorf("expected running 1, got %d", running)
	}
	if failed != 1 {
		t.Errorf("expected failed 1, got %d", failed)
	}
	if pending != 1 {
		t.Errorf("expected pending 1, got %d", pending)
	}
}

func TestResetFeature(t *testing.T) {
	p := NewProgress()
	p.InitFeature("01", "Test")
	p.UpdateFeature("01", "running")
	p.UpdateFeature("01", "failed")
	p.SetFeatureError("01", "Some error")

	p.ResetFeature("01")

	f := p.GetFeature("01")
	if f.Status != "pending" {
		t.Errorf("expected status 'pending', got %s", f.Status)
	}
	if f.Attempts != 0 {
		t.Errorf("expected Attempts 0, got %d", f.Attempts)
	}
	if f.LastError != "" {
		t.Errorf("expected empty LastError, got %s", f.LastError)
	}
}

func TestResetAll(t *testing.T) {
	p := NewProgress()
	p.InitFeature("01", "Test1")
	p.InitFeature("02", "Test2")
	p.UpdateFeature("01", "completed")
	p.UpdateFeature("02", "failed")

	p.ResetAll()

	for id, f := range p.Features {
		if f.Status != "pending" {
			t.Errorf("expected status 'pending' for %s, got %s", id, f.Status)
		}
	}
}

func TestSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	prdPath := filepath.Join(dir, "test.md")
	os.WriteFile(prdPath, []byte("# Test"), 0644)

	p := NewProgress()
	p.SetPath(prdPath)
	p.InitFeature("01", "Test Feature")
	p.InitFeature("01-child", "Child Feature")
	p.SetFeatureParent("01-child", "01")
	p.UpdateFeature("01", "completed")

	err := p.Save()
	if err != nil {
		t.Fatalf("failed to save: %v", err)
	}

	loaded, err := LoadProgress(prdPath)
	if err != nil {
		t.Fatalf("failed to load: %v", err)
	}

	f := loaded.GetFeature("01")
	if f == nil {
		t.Fatal("expected feature to be loaded")
	}
	if f.Status != "completed" {
		t.Errorf("expected status 'completed', got %s", f.Status)
	}

	child := loaded.GetFeature("01-child")
	if child == nil {
		t.Fatal("expected child feature to be loaded")
	}
	if child.ParentID != "01" {
		t.Errorf("expected ParentID '01', got %s", child.ParentID)
	}
	if child.Depth != 1 {
		t.Errorf("expected Depth 1, got %d", child.Depth)
	}
}

func TestSetCurrentModel(t *testing.T) {
	p := NewProgress()
	p.InitFeature("01", "Test Feature")

	p.SetCurrentModel("01", "haiku")

	model := p.GetCurrentModel("01")
	if model != "haiku" {
		t.Errorf("expected model 'haiku', got %s", model)
	}

	f := p.GetFeature("01")
	if f.CurrentModel != "haiku" {
		t.Errorf("expected CurrentModel 'haiku', got %s", f.CurrentModel)
	}
}

func TestSetCurrentModelNewFeature(t *testing.T) {
	p := NewProgress()

	p.SetCurrentModel("new-feature", "sonnet")

	model := p.GetCurrentModel("new-feature")
	if model != "sonnet" {
		t.Errorf("expected model 'sonnet', got %s", model)
	}
}

func TestGetCurrentModelNonexistent(t *testing.T) {
	p := NewProgress()

	model := p.GetCurrentModel("nonexistent")
	if model != "" {
		t.Errorf("expected empty model, got %s", model)
	}
}

func TestAddModelSwitch(t *testing.T) {
	p := NewProgress()
	p.InitFeature("01", "Test Feature")

	p.AddModelSwitch("01", "", "haiku", "initial", "auto mode initial selection")

	f := p.GetFeature("01")
	if len(f.ModelSwitches) != 1 {
		t.Fatalf("expected 1 model switch, got %d", len(f.ModelSwitches))
	}

	sw := f.ModelSwitches[0]
	if sw.FromModel != "" {
		t.Errorf("expected FromModel '', got %s", sw.FromModel)
	}
	if sw.ToModel != "haiku" {
		t.Errorf("expected ToModel 'haiku', got %s", sw.ToModel)
	}
	if sw.Reason != "initial" {
		t.Errorf("expected Reason 'initial', got %s", sw.Reason)
	}
	if sw.Details != "auto mode initial selection" {
		t.Errorf("expected Details 'auto mode initial selection', got %s", sw.Details)
	}
	if sw.Timestamp.IsZero() {
		t.Error("expected non-zero Timestamp")
	}
	if f.CurrentModel != "haiku" {
		t.Errorf("expected CurrentModel 'haiku', got %s", f.CurrentModel)
	}
}

func TestAddMultipleModelSwitches(t *testing.T) {
	p := NewProgress()
	p.InitFeature("01", "Test Feature")

	p.AddModelSwitch("01", "", "haiku", "initial", "start cheap")
	p.AddModelSwitch("01", "haiku", "sonnet", "tool_error", "multiple errors")
	p.AddModelSwitch("01", "sonnet", "opus", "architectural", "complex design")

	f := p.GetFeature("01")
	if len(f.ModelSwitches) != 3 {
		t.Fatalf("expected 3 model switches, got %d", len(f.ModelSwitches))
	}

	if f.CurrentModel != "opus" {
		t.Errorf("expected CurrentModel 'opus', got %s", f.CurrentModel)
	}

	if f.ModelSwitches[0].ToModel != "haiku" {
		t.Error("first switch should be to haiku")
	}
	if f.ModelSwitches[1].ToModel != "sonnet" {
		t.Error("second switch should be to sonnet")
	}
	if f.ModelSwitches[2].ToModel != "opus" {
		t.Error("third switch should be to opus")
	}
}

func TestGetModelSwitches(t *testing.T) {
	p := NewProgress()
	p.InitFeature("01", "Test Feature")

	p.AddModelSwitch("01", "", "haiku", "initial", "details")
	p.AddModelSwitch("01", "haiku", "sonnet", "error", "details")

	switches := p.GetModelSwitches("01")
	if len(switches) != 2 {
		t.Errorf("expected 2 switches, got %d", len(switches))
	}

	switches[0].Reason = "modified"
	origSwitches := p.GetModelSwitches("01")
	if origSwitches[0].Reason == "modified" {
		t.Error("GetModelSwitches should return a copy")
	}
}

func TestGetModelSwitchesNonexistent(t *testing.T) {
	p := NewProgress()

	switches := p.GetModelSwitches("nonexistent")
	if switches != nil {
		t.Errorf("expected nil switches, got %v", switches)
	}
}

func TestSaveAndLoadWithModelSwitches(t *testing.T) {
	dir := t.TempDir()
	prdPath := filepath.Join(dir, "test.md")
	os.WriteFile(prdPath, []byte("# Test"), 0644)

	p := NewProgress()
	p.SetPath(prdPath)
	p.InitFeature("01", "Test Feature")
	p.AddModelSwitch("01", "", "haiku", "initial", "auto start")
	p.AddModelSwitch("01", "haiku", "sonnet", "complexity", "complex task")

	err := p.Save()
	if err != nil {
		t.Fatalf("failed to save: %v", err)
	}

	loaded, err := LoadProgress(prdPath)
	if err != nil {
		t.Fatalf("failed to load: %v", err)
	}

	f := loaded.GetFeature("01")
	if f == nil {
		t.Fatal("expected feature to be loaded")
	}
	if f.CurrentModel != "sonnet" {
		t.Errorf("expected CurrentModel 'sonnet', got %s", f.CurrentModel)
	}
	if len(f.ModelSwitches) != 2 {
		t.Errorf("expected 2 model switches, got %d", len(f.ModelSwitches))
	}
	if f.ModelSwitches[1].Reason != "complexity" {
		t.Errorf("expected Reason 'complexity', got %s", f.ModelSwitches[1].Reason)
	}
}

// Fault isolation tests

func TestSetIsolationLevel(t *testing.T) {
	p := NewProgress()
	p.InitFeature("01", "Test Feature")

	p.SetIsolationLevel("01", "strict")

	level := p.GetIsolationLevel("01")
	if level != "strict" {
		t.Errorf("expected level 'strict', got %s", level)
	}

	p.SetIsolationLevel("01", "lenient")
	level = p.GetIsolationLevel("01")
	if level != "lenient" {
		t.Errorf("expected level 'lenient', got %s", level)
	}
}

func TestSetIsolationLevelNewFeature(t *testing.T) {
	p := NewProgress()

	p.SetIsolationLevel("new-feature", "strict")

	level := p.GetIsolationLevel("new-feature")
	if level != "strict" {
		t.Errorf("expected level 'strict', got %s", level)
	}
}

func TestGetIsolationLevelNonexistent(t *testing.T) {
	p := NewProgress()

	level := p.GetIsolationLevel("nonexistent")
	if level != "" {
		t.Errorf("expected empty level, got %s", level)
	}
}

func TestSetFailureReason(t *testing.T) {
	p := NewProgress()
	p.InitFeature("01", "Test Feature")

	p.SetFailureReason("01", "task execution failed")

	reason := p.GetFailureReason("01")
	if reason != "task execution failed" {
		t.Errorf("expected reason 'task execution failed', got %s", reason)
	}
}

func TestSetFailureReasonNewFeature(t *testing.T) {
	p := NewProgress()

	p.SetFailureReason("new-feature", "test failure")

	reason := p.GetFailureReason("new-feature")
	if reason != "test failure" {
		t.Errorf("expected reason 'test failure', got %s", reason)
	}
}

func TestGetFailureReasonNonexistent(t *testing.T) {
	p := NewProgress()

	reason := p.GetFailureReason("nonexistent")
	if reason != "" {
		t.Errorf("expected empty reason, got %s", reason)
	}
}

func TestAddFailedChild(t *testing.T) {
	p := NewProgress()
	p.InitFeature("parent", "Parent Feature")

	p.AddFailedChild("parent", "child-1")
	p.AddFailedChild("parent", "child-2")

	children := p.GetFailedChildren("parent")
	if len(children) != 2 {
		t.Errorf("expected 2 failed children, got %d", len(children))
	}

	// Adding duplicate should not add again
	p.AddFailedChild("parent", "child-1")
	children = p.GetFailedChildren("parent")
	if len(children) != 2 {
		t.Errorf("expected 2 failed children after duplicate, got %d", len(children))
	}
}

func TestAddFailedChildNewFeature(t *testing.T) {
	p := NewProgress()

	p.AddFailedChild("new-parent", "child-1")

	children := p.GetFailedChildren("new-parent")
	if len(children) != 1 {
		t.Errorf("expected 1 failed child, got %d", len(children))
	}
}

func TestGetFailedChildrenNonexistent(t *testing.T) {
	p := NewProgress()

	children := p.GetFailedChildren("nonexistent")
	if children != nil {
		t.Errorf("expected nil children, got %v", children)
	}
}

func TestHasFailedChildren(t *testing.T) {
	p := NewProgress()
	p.InitFeature("parent", "Parent Feature")

	if p.HasFailedChildren("parent") {
		t.Error("should not have failed children initially")
	}

	p.AddFailedChild("parent", "child-1")

	if !p.HasFailedChildren("parent") {
		t.Error("should have failed children after adding")
	}
}

func TestSkipFeature(t *testing.T) {
	p := NewProgress()
	p.InitFeature("01", "Test Feature")

	p.SkipFeature("01", "skipped due to parent failure")

	f := p.GetFeature("01")
	if f.Status != "skipped" {
		t.Errorf("expected status 'skipped', got %s", f.Status)
	}
	if !f.Skipped {
		t.Error("expected Skipped to be true")
	}
	if f.SkipReason != "skipped due to parent failure" {
		t.Errorf("expected SkipReason 'skipped due to parent failure', got %s", f.SkipReason)
	}
	if f.CompletedAt == nil {
		t.Error("expected CompletedAt to be set")
	}
}

func TestIsSkipped(t *testing.T) {
	p := NewProgress()
	p.InitFeature("01", "Test Feature")

	if p.IsSkipped("01") {
		t.Error("should not be skipped initially")
	}

	p.SkipFeature("01", "test reason")

	if !p.IsSkipped("01") {
		t.Error("should be skipped after SkipFeature")
	}
}

func TestGetSkipReason(t *testing.T) {
	p := NewProgress()
	p.InitFeature("01", "Test Feature")

	reason := p.GetSkipReason("01")
	if reason != "" {
		t.Errorf("expected empty reason, got %s", reason)
	}

	p.SkipFeature("01", "test skip reason")

	reason = p.GetSkipReason("01")
	if reason != "test skip reason" {
		t.Errorf("expected reason 'test skip reason', got %s", reason)
	}
}

func TestClearFailure(t *testing.T) {
	p := NewProgress()
	p.InitFeature("01", "Test Feature")

	// Set up failure state
	p.SetFeatureError("01", "some error")
	p.SetFailureReason("01", "execution failed")
	p.AddFailedChild("01", "child-1")
	p.SkipFeature("01", "skip reason")

	// Verify state is set
	f := p.GetFeature("01")
	if f.LastError == "" || f.FailureReason == "" || f.SkipReason == "" {
		t.Fatal("expected failure state to be set before clear")
	}

	// Clear failure
	p.ClearFailure("01")

	f = p.GetFeature("01")
	if f.LastError != "" {
		t.Errorf("expected empty LastError, got %s", f.LastError)
	}
	if f.FailureReason != "" {
		t.Errorf("expected empty FailureReason, got %s", f.FailureReason)
	}
	if len(f.FailedChildren) != 0 {
		t.Errorf("expected empty FailedChildren, got %d", len(f.FailedChildren))
	}
	if f.Skipped {
		t.Error("expected Skipped to be false")
	}
	if f.SkipReason != "" {
		t.Errorf("expected empty SkipReason, got %s", f.SkipReason)
	}
}

func TestCanChildRetry(t *testing.T) {
	p := NewProgress()
	p.InitFeature("parent", "Parent Feature")
	p.InitFeature("child", "Child Feature")
	p.SetFeatureParent("child", "parent")

	// Can retry with 0 attempts
	if !p.CanChildRetry("child", "parent") {
		t.Error("should be able to retry with 0 attempts")
	}

	// Update attempts
	p.UpdateFeature("child", "running")
	p.UpdateFeature("child", "running")
	p.UpdateFeature("child", "running")

	// Cannot retry after max attempts
	if p.CanChildRetry("child", "parent") {
		t.Error("should not be able to retry after max attempts")
	}
}

func TestCanChildRetryNewFeature(t *testing.T) {
	p := NewProgress()

	// New feature can retry
	if !p.CanChildRetry("new-child", "parent") {
		t.Error("new feature should be able to retry")
	}
}

func TestSaveAndLoadWithFaultIsolation(t *testing.T) {
	dir := t.TempDir()
	prdPath := filepath.Join(dir, "test.md")
	os.WriteFile(prdPath, []byte("# Test"), 0644)

	p := NewProgress()
	p.SetPath(prdPath)
	p.InitFeature("01", "Parent Feature")
	p.InitFeature("01-child", "Child Feature")
	p.SetFeatureParent("01-child", "01")
	p.SetIsolationLevel("01", "strict")
	p.SetFailureReason("01-child", "test failure")
	p.AddFailedChild("01", "01-child")
	p.SkipFeature("01-child", "skipped for test")

	err := p.Save()
	if err != nil {
		t.Fatalf("failed to save: %v", err)
	}

	loaded, err := LoadProgress(prdPath)
	if err != nil {
		t.Fatalf("failed to load: %v", err)
	}

	f := loaded.GetFeature("01")
	if f == nil {
		t.Fatal("expected feature to be loaded")
	}
	if f.IsolationLevel != "strict" {
		t.Errorf("expected IsolationLevel 'strict', got %s", f.IsolationLevel)
	}
	if len(f.FailedChildren) != 1 {
		t.Errorf("expected 1 failed child, got %d", len(f.FailedChildren))
	}

	child := loaded.GetFeature("01-child")
	if child == nil {
		t.Fatal("expected child feature to be loaded")
	}
	if child.FailureReason != "test failure" {
		t.Errorf("expected FailureReason 'test failure', got %s", child.FailureReason)
	}
	if !child.Skipped {
		t.Error("expected Skipped to be true")
	}
	if child.SkipReason != "skipped for test" {
		t.Errorf("expected SkipReason 'skipped for test', got %s", child.SkipReason)
	}
}

// Adjustment tests

func TestAddAdjustment(t *testing.T) {
	p := NewProgress()
	p.InitFeature("01", "Test Feature")

	adj := AdjustmentState{
		Type:       "model_escalation",
		Reason:     "test_failures",
		FromValue:  "haiku",
		ToValue:    "sonnet",
		Details:    "2 test failures",
		AttemptNum: 1,
	}
	p.AddAdjustment("01", adj)

	f := p.GetFeature("01")
	if len(f.Adjustments) != 1 {
		t.Fatalf("expected 1 adjustment, got %d", len(f.Adjustments))
	}
	if f.Adjustments[0].Type != "model_escalation" {
		t.Errorf("expected type 'model_escalation', got %s", f.Adjustments[0].Type)
	}
	if f.Adjustments[0].Timestamp.IsZero() {
		t.Error("expected non-zero timestamp")
	}
}

func TestAddAdjustmentNewFeature(t *testing.T) {
	p := NewProgress()

	adj := AdjustmentState{
		Type:       "model_escalation",
		AttemptNum: 1,
	}
	p.AddAdjustment("new-feature", adj)

	f := p.GetFeature("new-feature")
	if f == nil {
		t.Fatal("expected feature to be created")
	}
	if len(f.Adjustments) != 1 {
		t.Errorf("expected 1 adjustment, got %d", len(f.Adjustments))
	}
}

func TestGetAdjustments(t *testing.T) {
	p := NewProgress()
	p.InitFeature("01", "Test Feature")

	p.AddAdjustment("01", AdjustmentState{Type: "model_escalation", AttemptNum: 1})
	p.AddAdjustment("01", AdjustmentState{Type: "task_simplify", AttemptNum: 2})

	adjs := p.GetAdjustments("01")
	if len(adjs) != 2 {
		t.Fatalf("expected 2 adjustments, got %d", len(adjs))
	}

	// Verify it returns a copy
	adjs[0].Type = "modified"
	origAdjs := p.GetAdjustments("01")
	if origAdjs[0].Type == "modified" {
		t.Error("GetAdjustments should return a copy")
	}
}

func TestGetAdjustmentsNonexistent(t *testing.T) {
	p := NewProgress()

	adjs := p.GetAdjustments("nonexistent")
	if adjs != nil {
		t.Errorf("expected nil, got %v", adjs)
	}
}

func TestGetAdjustmentCount(t *testing.T) {
	p := NewProgress()
	p.InitFeature("01", "Test Feature")

	if p.GetAdjustmentCount("01") != 0 {
		t.Error("expected 0 adjustments initially")
	}

	p.AddAdjustment("01", AdjustmentState{Type: "model_escalation"})
	p.AddAdjustment("01", AdjustmentState{Type: "task_simplify"})

	if p.GetAdjustmentCount("01") != 2 {
		t.Errorf("expected 2 adjustments, got %d", p.GetAdjustmentCount("01"))
	}
}

func TestSetMaxAdjustments(t *testing.T) {
	p := NewProgress()
	p.InitFeature("01", "Test Feature")

	p.SetMaxAdjustments("01", 5)

	max := p.GetMaxAdjustments("01")
	if max != 5 {
		t.Errorf("expected max 5, got %d", max)
	}
}

func TestGetMaxAdjustmentsDefault(t *testing.T) {
	p := NewProgress()
	p.InitFeature("01", "Test Feature")

	// Default should be 3
	max := p.GetMaxAdjustments("01")
	if max != 3 {
		t.Errorf("expected default max 3, got %d", max)
	}
}

func TestCanAdjust(t *testing.T) {
	p := NewProgress()
	p.InitFeature("01", "Test Feature")

	if !p.CanAdjust("01") {
		t.Error("should be able to adjust initially")
	}

	p.AddAdjustment("01", AdjustmentState{Type: "model_escalation"})
	p.AddAdjustment("01", AdjustmentState{Type: "task_simplify"})
	p.AddAdjustment("01", AdjustmentState{Type: "context_expand"})

	if p.CanAdjust("01") {
		t.Error("should not be able to adjust at max (default 3)")
	}
}

func TestCanAdjustNewFeature(t *testing.T) {
	p := NewProgress()

	if !p.CanAdjust("new-feature") {
		t.Error("new feature should be able to adjust")
	}
}

func TestSetOriginalModel(t *testing.T) {
	p := NewProgress()
	p.InitFeature("01", "Test Feature")

	p.SetOriginalModel("01", "haiku")

	orig := p.GetOriginalModel("01")
	if orig != "haiku" {
		t.Errorf("expected 'haiku', got %s", orig)
	}

	// Should not change if already set
	p.SetOriginalModel("01", "sonnet")
	orig = p.GetOriginalModel("01")
	if orig != "haiku" {
		t.Errorf("expected 'haiku' (unchanged), got %s", orig)
	}
}

func TestGetOriginalModelNonexistent(t *testing.T) {
	p := NewProgress()

	orig := p.GetOriginalModel("nonexistent")
	if orig != "" {
		t.Errorf("expected empty, got %s", orig)
	}
}

func TestSetSimplified(t *testing.T) {
	p := NewProgress()
	p.InitFeature("01", "Test Feature")

	if p.IsSimplified("01") {
		t.Error("should not be simplified initially")
	}

	p.SetSimplified("01", true)
	if !p.IsSimplified("01") {
		t.Error("should be simplified after SetSimplified(true)")
	}

	p.SetSimplified("01", false)
	if p.IsSimplified("01") {
		t.Error("should not be simplified after SetSimplified(false)")
	}
}

func TestIsSimplifiedNonexistent(t *testing.T) {
	p := NewProgress()

	if p.IsSimplified("nonexistent") {
		t.Error("nonexistent feature should not be simplified")
	}
}

func TestLastAdjustment(t *testing.T) {
	p := NewProgress()
	p.InitFeature("01", "Test Feature")

	// No adjustments
	if p.LastAdjustment("01") != nil {
		t.Error("expected nil for no adjustments")
	}

	p.AddAdjustment("01", AdjustmentState{Type: "model_escalation", AttemptNum: 1})
	p.AddAdjustment("01", AdjustmentState{Type: "task_simplify", AttemptNum: 2})

	last := p.LastAdjustment("01")
	if last == nil {
		t.Fatal("expected non-nil last adjustment")
	}
	if last.Type != "task_simplify" {
		t.Errorf("expected type 'task_simplify', got %s", last.Type)
	}
}

func TestHasModelEscalation(t *testing.T) {
	p := NewProgress()
	p.InitFeature("01", "Test Feature")

	if p.HasModelEscalation("01") {
		t.Error("should not have model escalation initially")
	}

	p.AddAdjustment("01", AdjustmentState{Type: "task_simplify"})
	if p.HasModelEscalation("01") {
		t.Error("should not have model escalation after task_simplify")
	}

	p.AddAdjustment("01", AdjustmentState{Type: "model_escalation"})
	if !p.HasModelEscalation("01") {
		t.Error("should have model escalation after adding one")
	}
}

func TestGetAdjustmentSummary(t *testing.T) {
	p := NewProgress()
	p.InitFeature("01", "Test Feature")

	// No adjustments
	summary := p.GetAdjustmentSummary("01")
	if summary != "" {
		t.Errorf("expected empty summary, got %s", summary)
	}

	p.AddAdjustment("01", AdjustmentState{
		Type:       "model_escalation",
		FromValue:  "haiku",
		ToValue:    "sonnet",
		AttemptNum: 1,
	})

	summary = p.GetAdjustmentSummary("01")
	if summary == "" {
		t.Error("expected non-empty summary")
	}
	// Should contain model escalation info
	if !containsStr(summary, "Model:") {
		t.Errorf("expected summary to contain 'Model:', got %s", summary)
	}
}

func TestSaveAndLoadWithAdjustments(t *testing.T) {
	dir := t.TempDir()
	prdPath := filepath.Join(dir, "test.md")
	os.WriteFile(prdPath, []byte("# Test"), 0644)

	p := NewProgress()
	p.SetPath(prdPath)
	p.InitFeature("01", "Test Feature")
	p.SetOriginalModel("01", "haiku")
	p.SetMaxAdjustments("01", 5)
	p.AddAdjustment("01", AdjustmentState{
		Type:       "model_escalation",
		Reason:     "test_failures",
		FromValue:  "haiku",
		ToValue:    "sonnet",
		Details:    "2 failures",
		AttemptNum: 1,
	})
	p.SetSimplified("01", true)

	err := p.Save()
	if err != nil {
		t.Fatalf("failed to save: %v", err)
	}

	loaded, err := LoadProgress(prdPath)
	if err != nil {
		t.Fatalf("failed to load: %v", err)
	}

	f := loaded.GetFeature("01")
	if f == nil {
		t.Fatal("expected feature to be loaded")
	}
	if f.OriginalModel != "haiku" {
		t.Errorf("expected OriginalModel 'haiku', got %s", f.OriginalModel)
	}
	if f.MaxAdjustments != 5 {
		t.Errorf("expected MaxAdjustments 5, got %d", f.MaxAdjustments)
	}
	if len(f.Adjustments) != 1 {
		t.Errorf("expected 1 adjustment, got %d", len(f.Adjustments))
	}
	if f.Adjustments[0].Type != "model_escalation" {
		t.Errorf("expected type 'model_escalation', got %s", f.Adjustments[0].Type)
	}
	if !f.Simplified {
		t.Error("expected Simplified to be true")
	}
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
