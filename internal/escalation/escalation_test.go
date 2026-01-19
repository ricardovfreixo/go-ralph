package escalation

import (
	"encoding/json"
	"testing"
)

func TestDefaultTriggerConfig(t *testing.T) {
	config := DefaultTriggerConfig()

	if config.ErrorThreshold != 2 {
		t.Errorf("expected error threshold 2, got %d", config.ErrorThreshold)
	}
	if !config.Enabled {
		t.Error("config should be enabled by default")
	}
	if len(config.EscalateKeywords) == 0 {
		t.Error("should have escalate keywords")
	}
	if len(config.DeescalateKeywords) == 0 {
		t.Error("should have deescalate keywords")
	}
}

func TestNewTracker(t *testing.T) {
	config := DefaultTriggerConfig()
	tracker := NewTracker("feature-1", "sonnet", config)

	if tracker.CurrentModel() != ModelSonnet {
		t.Errorf("expected sonnet, got %s", tracker.CurrentModel())
	}
	if tracker.ErrorCount() != 0 {
		t.Errorf("expected 0 errors, got %d", tracker.ErrorCount())
	}

	switches := tracker.Switches()
	if len(switches) != 1 {
		t.Errorf("expected 1 initial switch, got %d", len(switches))
	}
	if switches[0].Reason != ReasonInitial {
		t.Errorf("expected initial reason, got %s", switches[0].Reason)
	}
}

func TestTrackerDefaultsToSonnet(t *testing.T) {
	config := DefaultTriggerConfig()

	tests := []struct {
		initial  string
		expected string
	}{
		{"", ModelSonnet},
		{"auto", ModelSonnet},
		{"haiku", "haiku"},
		{"sonnet", "sonnet"},
		{"opus", "opus"},
	}

	for _, tt := range tests {
		tracker := NewTracker("test", tt.initial, config)
		if tracker.CurrentModel() != tt.expected {
			t.Errorf("initial=%q: expected %s, got %s", tt.initial, tt.expected, tracker.CurrentModel())
		}
	}
}

func TestEscalateOnRepeatedErrors(t *testing.T) {
	config := DefaultTriggerConfig()
	tracker := NewTracker("test", "haiku", config)

	errorMsg := `{"type":"tool_result","is_error":true,"result":"command failed"}`

	changed, _ := tracker.ProcessLine(errorMsg)
	if changed {
		t.Error("shouldn't escalate on first error")
	}

	changed, model := tracker.ProcessLine(errorMsg)
	if !changed {
		t.Error("should escalate on 2nd error (threshold=2)")
	}
	if model != ModelSonnet {
		t.Errorf("expected sonnet, got %s", model)
	}
}

func TestEscalateOnExplicitRequest(t *testing.T) {
	config := DefaultTriggerConfig()
	tracker := NewTracker("test", "haiku", config)

	msg := `{"type":"assistant","content":"This task needs opus for the complex architectural decisions."}`
	changed, model := tracker.ProcessLine(msg)
	if !changed {
		t.Error("should escalate on explicit opus request")
	}
	if model != ModelOpus {
		t.Errorf("expected opus, got %s", model)
	}
}

func TestDeescalateOnSimpleKeywords(t *testing.T) {
	config := DefaultTriggerConfig()
	tracker := NewTracker("test", "sonnet", config)

	msg := `{"type":"assistant","content":"This is a simple test formatting task. Just linting."}`
	changed, model := tracker.ProcessLine(msg)
	if !changed {
		t.Error("should deescalate on simple task keywords")
	}
	if model != ModelHaiku {
		t.Errorf("expected haiku, got %s", model)
	}

	last := tracker.LastSwitch()
	if last.Reason != ReasonDeescalate {
		t.Errorf("expected deescalate reason, got %s", last.Reason)
	}
}

func TestNoDeescalateAfterErrors(t *testing.T) {
	config := DefaultTriggerConfig()
	tracker := NewTracker("test", "sonnet", config)

	errorMsg := `{"type":"tool_result","is_error":true,"result":"failed"}`
	tracker.ProcessLine(errorMsg)

	msg := `{"type":"assistant","content":"This is a simple test task."}`
	changed, _ := tracker.ProcessLine(msg)
	if changed {
		t.Error("should not deescalate after errors")
	}
}

func TestNoDeescalateFromHaiku(t *testing.T) {
	config := DefaultTriggerConfig()
	tracker := NewTracker("test", "haiku", config)

	msg := `{"type":"assistant","content":"This is a simple test task. Just testing."}`
	changed, _ := tracker.ProcessLine(msg)
	if changed {
		t.Error("should not change when already at haiku")
	}
}

func TestForceModel(t *testing.T) {
	config := DefaultTriggerConfig()
	tracker := NewTracker("test", "haiku", config)

	tracker.ForceModel(ModelOpus, "user override")
	if tracker.CurrentModel() != ModelOpus {
		t.Errorf("expected opus after force, got %s", tracker.CurrentModel())
	}

	switches := tracker.Switches()
	last := switches[len(switches)-1]
	if last.Reason != ReasonConfiguredByUser {
		t.Errorf("expected configured_by_user reason, got %s", last.Reason)
	}
}

func TestForceModelSameModel(t *testing.T) {
	config := DefaultTriggerConfig()
	tracker := NewTracker("test", "haiku", config)

	initial := len(tracker.Switches())
	tracker.ForceModel(ModelHaiku, "same")

	if len(tracker.Switches()) != initial {
		t.Error("should not add switch when forcing same model")
	}
}

func TestResetErrors(t *testing.T) {
	config := DefaultTriggerConfig()
	tracker := NewTracker("test", "haiku", config)

	errorMsg := `{"type":"tool_result","is_error":true,"result":"failed"}`
	tracker.ProcessLine(errorMsg)

	if tracker.ErrorCount() != 1 {
		t.Errorf("expected 1 error, got %d", tracker.ErrorCount())
	}

	tracker.ResetErrors()
	if tracker.ErrorCount() != 0 {
		t.Errorf("expected 0 errors after reset, got %d", tracker.ErrorCount())
	}
}

func TestShouldRestart(t *testing.T) {
	config := DefaultTriggerConfig()
	tracker := NewTracker("test", "haiku", config)

	if tracker.ShouldRestart() {
		t.Error("should not need restart initially")
	}

	errorMsg := `{"type":"tool_result","is_error":true,"result":"failed"}`
	tracker.ProcessLine(errorMsg)
	tracker.ProcessLine(errorMsg)

	if !tracker.ShouldRestart() {
		t.Error("should need restart after model change")
	}
}

func TestLastSwitch(t *testing.T) {
	config := DefaultTriggerConfig()
	tracker := NewTracker("test", "haiku", config)

	last := tracker.LastSwitch()
	if last == nil {
		t.Fatal("should have last switch")
	}
	if last.ToModel != ModelHaiku {
		t.Errorf("expected haiku, got %s", last.ToModel)
	}

	tracker.ForceModel(ModelOpus, "test")
	last = tracker.LastSwitch()
	if last.ToModel != ModelOpus {
		t.Errorf("expected opus, got %s", last.ToModel)
	}
}

func TestManager(t *testing.T) {
	m := NewManager()

	t1 := m.Register("f1", "haiku")
	t2 := m.Register("f2", "sonnet")

	if t1.CurrentModel() != ModelHaiku {
		t.Errorf("f1 expected haiku, got %s", t1.CurrentModel())
	}
	if t2.CurrentModel() != ModelSonnet {
		t.Errorf("f2 expected sonnet, got %s", t2.CurrentModel())
	}

	got := m.Get("f1")
	if got != t1 {
		t.Error("should return same tracker")
	}

	if m.Get("nonexistent") != nil {
		t.Error("should return nil for nonexistent")
	}

	m.Remove("f1")
	if m.Get("f1") != nil {
		t.Error("should be removed")
	}
}

func TestManagerWithConfig(t *testing.T) {
	config := TriggerConfig{
		ErrorThreshold: 5,
		Enabled:        true,
	}

	m := NewManagerWithConfig(config)
	got := m.GetConfig()
	if got.ErrorThreshold != 5 {
		t.Errorf("expected 5, got %d", got.ErrorThreshold)
	}
}

func TestManagerSetConfig(t *testing.T) {
	m := NewManager()

	newConfig := TriggerConfig{ErrorThreshold: 10, Enabled: true}
	m.SetConfig(newConfig)

	if m.GetConfig().ErrorThreshold != 10 {
		t.Errorf("expected 10, got %d", m.GetConfig().ErrorThreshold)
	}
}

func TestManagerProcessLine(t *testing.T) {
	m := NewManager()
	m.Register("test", "haiku")

	errorMsg := `{"type":"tool_result","is_error":true,"result":"failed"}`
	m.ProcessLine("test", errorMsg)
	m.ProcessLine("test", errorMsg)

	model := m.GetCurrentModel("test")
	if model != ModelSonnet {
		t.Errorf("expected sonnet, got %s", model)
	}
}

func TestManagerGetAllSwitches(t *testing.T) {
	m := NewManager()
	m.Register("test", "haiku")

	errorMsg := `{"type":"tool_result","is_error":true,"result":"failed"}`
	m.ProcessLine("test", errorMsg)
	m.ProcessLine("test", errorMsg)

	switches := m.GetAllSwitches("test")
	if len(switches) != 2 {
		t.Errorf("expected 2 switches, got %d", len(switches))
	}

	if m.GetAllSwitches("nonexistent") != nil {
		t.Error("should return nil for nonexistent")
	}
}

func TestSelectInitialModel(t *testing.T) {
	tests := []struct {
		taskCount int
		keywords  []string
		expected  string
	}{
		{1, nil, ModelHaiku},
		{2, nil, ModelHaiku},
		{3, nil, ModelSonnet},
		{10, nil, ModelSonnet},
		{1, []string{"architect"}, ModelSonnet},
		{1, []string{"design pattern"}, ModelSonnet},
		{1, []string{"refactor"}, ModelSonnet},
		{1, []string{"simple"}, ModelHaiku},
	}

	for _, tt := range tests {
		got := SelectInitialModel(tt.taskCount, tt.keywords)
		if got != tt.expected {
			t.Errorf("taskCount=%d, keywords=%v: expected %s, got %s",
				tt.taskCount, tt.keywords, tt.expected, got)
		}
	}
}

func TestShouldDeescalateForTask(t *testing.T) {
	positives := []string{
		"Write unit tests",
		"Format code",
		"Run linting",
		"Fix typo",
		"Add comments",
		"Simple fix",
	}

	for _, title := range positives {
		if !ShouldDeescalateForTask(title) {
			t.Errorf("expected deescalate for: %s", title)
		}
	}

	negatives := []string{
		"Implement new feature",
		"Refactor architecture",
		"Design API",
	}

	for _, title := range negatives {
		if ShouldDeescalateForTask(title) {
			t.Errorf("unexpected deescalate for: %s", title)
		}
	}
}

func TestShouldEscalateForTask(t *testing.T) {
	positives := []string{
		"Design architecture",
		"Refactor system",
		"Complex algorithm",
		"API design review",
	}

	for _, title := range positives {
		if !ShouldEscalateForTask(title) {
			t.Errorf("expected escalate for: %s", title)
		}
	}

	negatives := []string{
		"Fix typo",
		"Add test",
		"Update comment",
	}

	for _, title := range negatives {
		if ShouldEscalateForTask(title) {
			t.Errorf("unexpected escalate for: %s", title)
		}
	}
}

func TestProcessLineInvalidJSON(t *testing.T) {
	config := DefaultTriggerConfig()
	tracker := NewTracker("test", "haiku", config)

	changed, model := tracker.ProcessLine("not json")
	if changed {
		t.Error("should not change on invalid JSON")
	}
	if model != ModelHaiku {
		t.Errorf("should return current model, got %s", model)
	}
}

func TestProcessLineDisabled(t *testing.T) {
	config := TriggerConfig{
		ErrorThreshold: 2,
		Enabled:        false,
	}
	tracker := NewTracker("test", "haiku", config)

	errorMsg := `{"type":"tool_result","is_error":true,"result":"failed"}`
	for i := 0; i < 5; i++ {
		changed, _ := tracker.ProcessLine(errorMsg)
		if changed {
			t.Error("should not change when disabled")
		}
	}
}

func TestArchitecturalPatterns(t *testing.T) {
	config := DefaultTriggerConfig()
	tracker := NewTracker("test", "haiku", config)

	tests := []struct {
		content  string
		expected bool
	}{
		{"This requires architectural changes", true},
		{"Need to design the API design pattern", true},
		{"Major refactoring strategy needed", true},
		{"Consider the trade-off here", true},
		{"Database schema migration", true},
		{"Simple fix for typo", false},
	}

	for _, tt := range tests {
		if tracker.containsArchitecturalPattern(tt.content) != tt.expected {
			t.Errorf("content=%q: expected %v", tt.content, tt.expected)
		}
	}
}

func TestExplicitRequestPatterns(t *testing.T) {
	config := DefaultTriggerConfig()
	tracker := NewTracker("test", "haiku", config)

	tests := []struct {
		content string
		model   string
	}{
		{"I need opus for this", ModelOpus},
		{"This needs opus model", ModelOpus},
		{"Require opus here", ModelOpus},
		{"Switch to sonnet", ModelSonnet},
		{"Need haiku for simple task", ModelHaiku},
	}

	for _, tt := range tests {
		tracker = NewTracker("test", "haiku", config)
		msg, _ := json.Marshal(StreamMessage{Type: "assistant", Content: tt.content})
		changed, model := tracker.ProcessLine(string(msg))
		if !changed && tt.model != ModelHaiku {
			t.Errorf("content=%q: should have changed to %s", tt.content, tt.model)
		}
		if changed && model != tt.model {
			t.Errorf("content=%q: expected %s, got %s", tt.content, tt.model, model)
		}
	}
}

func TestTestAndBuildErrors(t *testing.T) {
	config := DefaultTriggerConfig()
	tracker := NewTracker("test", "haiku", config)

	testErrors := []string{
		"test failed",
		"--- FAIL: TestSomething",
		"build failed",
		"compilation failed",
		"syntax error",
	}

	for _, errStr := range testErrors {
		if !tracker.isTestOrBuildError(errStr) {
			t.Errorf("expected test/build error: %s", errStr)
		}
	}
}

func TestConcurrentAccess(t *testing.T) {
	config := DefaultTriggerConfig()
	tracker := NewTracker("test", "haiku", config)

	done := make(chan bool, 10)

	for i := 0; i < 5; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				_ = tracker.CurrentModel()
				_ = tracker.Switches()
				_ = tracker.ErrorCount()
			}
			done <- true
		}()
	}

	for i := 0; i < 5; i++ {
		go func() {
			msg := `{"type":"tool_result","is_error":true,"result":"error"}`
			for j := 0; j < 100; j++ {
				tracker.ProcessLine(msg)
			}
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}
