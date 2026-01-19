package automodel

import (
	"encoding/json"
	"testing"
)

func TestNewSelector(t *testing.T) {
	tests := []struct {
		name         string
		isLeaf       bool
		taskCount    int
		expectedInit string
	}{
		{"leaf task", true, 1, ModelHaiku},
		{"simple task 1", false, 1, ModelHaiku},
		{"simple task 2", false, 2, ModelHaiku},
		{"moderate task 3", false, 3, ModelHaiku},
		{"moderate task 5", false, 5, ModelHaiku},
		{"complex task 6", false, 6, ModelSonnet},
		{"complex task 10", false, 10, ModelSonnet},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewSelector("test-feature", tt.isLeaf, tt.taskCount)
			if s.CurrentModel() != tt.expectedInit {
				t.Errorf("expected initial model %s, got %s", tt.expectedInit, s.CurrentModel())
			}
			switches := s.Switches()
			if len(switches) != 1 {
				t.Errorf("expected 1 initial switch, got %d", len(switches))
			}
			if switches[0].Reason != ReasonInitial {
				t.Errorf("expected initial reason, got %s", switches[0].Reason)
			}
		})
	}
}

func TestEscalateOnToolError(t *testing.T) {
	s := NewSelector("test-feature", true, 1)
	if s.CurrentModel() != ModelHaiku {
		t.Fatalf("expected haiku, got %s", s.CurrentModel())
	}

	errorMsg := `{"type":"tool_result","tool":"Bash","is_error":true,"result":"command failed"}`

	// First error shouldn't escalate
	changed, _ := s.ProcessLine(errorMsg)
	if changed {
		t.Errorf("shouldn't escalate on error 1")
	}

	// Second error should escalate (threshold is 2 by default)
	changed, model := s.ProcessLine(errorMsg)
	if !changed {
		t.Error("should escalate on 2nd error")
	}
	if model != ModelSonnet {
		t.Errorf("expected sonnet, got %s", model)
	}
	if s.CurrentModel() != ModelSonnet {
		t.Errorf("current model should be sonnet, got %s", s.CurrentModel())
	}
}

func TestEscalateOnTestFailure(t *testing.T) {
	s := NewSelector("test-feature", true, 1)

	testFailMsg := `{"type":"tool_result","tool":"Bash","is_error":true,"result":"--- FAIL: TestSomething\ntest failed"}`

	s.ProcessLine(testFailMsg)
	if s.CurrentModel() != ModelHaiku {
		t.Errorf("shouldn't escalate on first test failure")
	}

	changed, model := s.ProcessLine(testFailMsg)
	if !changed {
		t.Error("should escalate on 2nd test failure")
	}
	if model != ModelSonnet {
		t.Errorf("expected sonnet, got %s", model)
	}
}

func TestEscalateOnArchitecturalContent(t *testing.T) {
	s := NewSelector("test-feature", true, 1)

	archMsg := `{"type":"tool_result","tool":"Read","result":"This requires a significant architectural decision about the system design and trade-offs between performance and maintainability."}`

	changed, model := s.ProcessLine(archMsg)
	if !changed {
		t.Error("should escalate on architectural content")
	}
	if model != ModelSonnet {
		t.Errorf("expected sonnet, got %s", model)
	}
}

func TestEscalateOnDebugging(t *testing.T) {
	s := NewSelector("test-feature", true, 1)

	debugMsg := `{"type":"tool_result","tool":"Bash","result":"panic: runtime error: invalid memory address\ngoroutine 1 [running]:\nmain.main()\n\tstack trace follows..."}`

	changed, model := s.ProcessLine(debugMsg)
	if !changed {
		t.Error("should escalate on debugging scenario")
	}
	if model != ModelSonnet {
		t.Errorf("expected sonnet, got %s", model)
	}
}

func TestEscalateToOpusOnComplexDebugging(t *testing.T) {
	s := NewSelector("test-feature", false, 6) // starts at sonnet

	debugMsg := `{"type":"tool_result","tool":"Bash","result":"race condition detected in concurrent access to shared map, possible deadlock scenario"}`

	changed, model := s.ProcessLine(debugMsg)
	if !changed {
		t.Error("should escalate to opus on complex debugging")
	}
	if model != ModelOpus {
		t.Errorf("expected opus, got %s", model)
	}
}

func TestNoDowngrade(t *testing.T) {
	s := NewSelector("test-feature", false, 10)
	if s.CurrentModel() != ModelSonnet {
		t.Fatalf("expected sonnet, got %s", s.CurrentModel())
	}

	// Try to trigger what would be haiku-level content
	simpleMsg := `{"type":"tool_result","tool":"Read","result":"simple content here"}`
	changed, _ := s.ProcessLine(simpleMsg)
	if changed {
		t.Error("should never downgrade model")
	}
	if s.CurrentModel() != ModelSonnet {
		t.Errorf("model should remain sonnet, got %s", s.CurrentModel())
	}
}

func TestNoEscalationFromOpus(t *testing.T) {
	s := NewSelector("test-feature", false, 10)
	s.ForceModel(ModelOpus, "testing")

	// Try to trigger escalation
	errorMsg := `{"type":"tool_result","tool":"Bash","is_error":true,"result":"command failed"}`
	for i := 0; i < 5; i++ {
		changed, _ := s.ProcessLine(errorMsg)
		if changed {
			t.Error("should not change when already at opus")
		}
	}
	if s.CurrentModel() != ModelOpus {
		t.Errorf("model should remain opus, got %s", s.CurrentModel())
	}
}

func TestForceModel(t *testing.T) {
	s := NewSelector("test-feature", true, 1)
	s.ForceModel(ModelOpus, "user override")

	if s.CurrentModel() != ModelOpus {
		t.Errorf("expected opus after force, got %s", s.CurrentModel())
	}

	switches := s.Switches()
	if len(switches) != 2 {
		t.Errorf("expected 2 switches, got %d", len(switches))
	}
	last := switches[len(switches)-1]
	if last.Reason != ReasonExplicitRequest {
		t.Errorf("expected explicit request reason, got %s", last.Reason)
	}
}

func TestForceModelSameModel(t *testing.T) {
	s := NewSelector("test-feature", true, 1)
	initialSwitches := len(s.Switches())

	s.ForceModel(ModelHaiku, "same model")

	if len(s.Switches()) != initialSwitches {
		t.Error("should not add switch when forcing same model")
	}
}

func TestSwitchesTracking(t *testing.T) {
	s := NewSelector("test-feature", true, 1)

	// Trigger escalation to sonnet
	errorMsg := `{"type":"tool_result","tool":"Bash","is_error":true,"result":"command failed"}`
	for i := 0; i < 3; i++ {
		s.ProcessLine(errorMsg)
	}

	// Force to opus
	s.ForceModel(ModelOpus, "testing")

	switches := s.Switches()
	if len(switches) != 3 {
		t.Errorf("expected 3 switches, got %d", len(switches))
	}

	// Verify progression
	if switches[0].ToModel != ModelHaiku {
		t.Errorf("first switch should be to haiku, got %s", switches[0].ToModel)
	}
	if switches[1].ToModel != ModelSonnet {
		t.Errorf("second switch should be to sonnet, got %s", switches[1].ToModel)
	}
	if switches[2].ToModel != ModelOpus {
		t.Errorf("third switch should be to opus, got %s", switches[2].ToModel)
	}
}

func TestManager(t *testing.T) {
	m := NewManager()

	s1 := m.Register("feature-1", true, 1)
	s2 := m.Register("feature-2", false, 10)

	if s1.CurrentModel() != ModelHaiku {
		t.Errorf("feature-1 should start with haiku, got %s", s1.CurrentModel())
	}
	if s2.CurrentModel() != ModelSonnet {
		t.Errorf("feature-2 should start with sonnet, got %s", s2.CurrentModel())
	}

	got1 := m.Get("feature-1")
	if got1 != s1 {
		t.Error("should return same selector instance")
	}

	got3 := m.Get("nonexistent")
	if got3 != nil {
		t.Error("should return nil for nonexistent feature")
	}

	m.Remove("feature-1")
	if m.Get("feature-1") != nil {
		t.Error("feature should be removed")
	}
}

func TestManagerGetAllSwitches(t *testing.T) {
	m := NewManager()
	s := m.Register("test-feature", true, 1)

	// Trigger some switches
	s.ForceModel(ModelSonnet, "test")
	s.ForceModel(ModelOpus, "test")

	switches := m.GetAllSwitches("test-feature")
	if len(switches) != 3 {
		t.Errorf("expected 3 switches, got %d", len(switches))
	}

	nilSwitches := m.GetAllSwitches("nonexistent")
	if nilSwitches != nil {
		t.Error("expected nil for nonexistent feature")
	}
}

func TestIsAutoMode(t *testing.T) {
	if !IsAutoMode("auto") {
		t.Error("'auto' should be auto mode")
	}
	if IsAutoMode("sonnet") {
		t.Error("'sonnet' should not be auto mode")
	}
	if IsAutoMode("haiku") {
		t.Error("'haiku' should not be auto mode")
	}
	if IsAutoMode("opus") {
		t.Error("'opus' should not be auto mode")
	}
}

func TestIsHigherTier(t *testing.T) {
	tests := []struct {
		target   string
		current  string
		expected bool
	}{
		{ModelSonnet, ModelHaiku, true},
		{ModelOpus, ModelHaiku, true},
		{ModelOpus, ModelSonnet, true},
		{ModelHaiku, ModelSonnet, false},
		{ModelHaiku, ModelOpus, false},
		{ModelSonnet, ModelOpus, false},
		{ModelHaiku, ModelHaiku, false},
		{ModelSonnet, ModelSonnet, false},
		{ModelOpus, ModelOpus, false},
	}

	for _, tt := range tests {
		t.Run(tt.target+"_from_"+tt.current, func(t *testing.T) {
			if isHigherTier(tt.target, tt.current) != tt.expected {
				t.Errorf("isHigherTier(%s, %s) = %v, want %v",
					tt.target, tt.current, !tt.expected, tt.expected)
			}
		})
	}
}

func TestIsArchitecturalContent(t *testing.T) {
	positives := []string{
		"This requires an architectural decision",
		"We need to consider the design pattern here",
		"The system design needs rethinking",
		"There are trade-offs to consider",
		"The API design should be RESTful",
		"database schema migration required",
		"data model changes needed",
	}

	for _, content := range positives {
		if !isArchitecturalContent(content) {
			t.Errorf("expected architectural: %s", content)
		}
	}

	negatives := []string{
		"simple fix here",
		"add a log statement",
		"rename variable",
		"update test",
	}

	for _, content := range negatives {
		if isArchitecturalContent(content) {
			t.Errorf("unexpected architectural: %s", content)
		}
	}
}

func TestIsDebuggingScenario(t *testing.T) {
	positives := []string{
		"debugging the issue",
		"stack trace follows",
		"segmentation fault",
		"race condition detected",
		"deadlock in mutex",
		"memory leak found",
		"panic: runtime error",
	}

	for _, content := range positives {
		if !isDebuggingScenario(content) {
			t.Errorf("expected debugging: %s", content)
		}
	}

	negatives := []string{
		"simple test",
		"normal output",
		"success",
	}

	for _, content := range negatives {
		if isDebuggingScenario(content) {
			t.Errorf("unexpected debugging: %s", content)
		}
	}
}

func TestContainsModelEscalationRequest(t *testing.T) {
	positives := []string{
		"I need opus for this complex task",
		"Should escalate to opus here",
		"Let's switch to opus",
		"I need sonnet for this",
		"require opus for this",
	}

	for _, content := range positives {
		if !containsModelEscalationRequest(content) {
			t.Errorf("expected escalation request: %s", content)
		}
	}

	negatives := []string{
		"simple task here",
		"working fine",
		"haiku should handle this",
	}

	for _, content := range negatives {
		if containsModelEscalationRequest(content) {
			t.Errorf("unexpected escalation request: %s", content)
		}
	}
}

func TestExtractRequestedModel(t *testing.T) {
	tests := []struct {
		content  string
		expected string
	}{
		{"need opus for this", ModelOpus},
		{"escalate to sonnet", ModelSonnet},
		{"simple task", ""},
		{"OPUS required", ModelOpus},
	}

	for _, tt := range tests {
		got := extractRequestedModel(tt.content)
		if got != tt.expected {
			t.Errorf("extractRequestedModel(%q) = %q, want %q", tt.content, got, tt.expected)
		}
	}
}

func TestProcessLineInvalidJSON(t *testing.T) {
	s := NewSelector("test", true, 1)
	changed, model := s.ProcessLine("not json")
	if changed {
		t.Error("should not change on invalid JSON")
	}
	if model != ModelHaiku {
		t.Errorf("should return current model, got %s", model)
	}
}

func TestProcessLineUnknownType(t *testing.T) {
	s := NewSelector("test", true, 1)
	msg := `{"type":"unknown","content":"test"}`
	changed, model := s.ProcessLine(msg)
	if changed {
		t.Error("should not change on unknown type")
	}
	if model != ModelHaiku {
		t.Errorf("should return current model, got %s", model)
	}
}

func TestAssistantMessageWithEscalationRequest(t *testing.T) {
	s := NewSelector("test", true, 1)

	msg := StreamMessage{
		Type:    "assistant",
		Content: "I need opus model to handle this complex architectural decision",
	}
	data, _ := json.Marshal(msg)

	changed, model := s.ProcessLine(string(data))
	if !changed {
		t.Error("should escalate on explicit request")
	}
	if model != ModelOpus {
		t.Errorf("expected opus, got %s", model)
	}
}

func TestCompilationErrorDetection(t *testing.T) {
	errors := []string{
		"compilation failed: undefined: foo",
		"build failed with errors",
		"compile error at line 10",
		"syntax error: unexpected token",
		"undefined: someFunction",
		"cannot find package",
		"type mismatch: expected int",
	}

	for _, err := range errors {
		if !isCompilationError(err) {
			t.Errorf("expected compilation error: %s", err)
		}
	}
}

func TestTestErrorDetection(t *testing.T) {
	errors := []string{
		"test failed",
		"--- FAIL: TestSomething",
		"assertion failed: expected 1, got 2",
		"Expected: true, Actual: false",
	}

	for _, err := range errors {
		if !isTestError(err) {
			t.Errorf("expected test error: %s", err)
		}
	}
}

func TestConcurrentAccess(t *testing.T) {
	s := NewSelector("test", true, 1)

	done := make(chan bool, 10)

	for i := 0; i < 5; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				_ = s.CurrentModel()
				_ = s.Switches()
			}
			done <- true
		}()
	}

	for i := 0; i < 5; i++ {
		go func() {
			msg := `{"type":"tool_result","tool":"Bash","is_error":true,"result":"error"}`
			for j := 0; j < 100; j++ {
				s.ProcessLine(msg)
			}
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

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

func TestNewSelectorWithConfig(t *testing.T) {
	config := Config{
		ErrorThreshold:     3,
		EscalateKeywords:   []string{"custom"},
		DeescalateKeywords: []string{"simple"},
		Enabled:            true,
	}

	s := NewSelectorWithConfig("test", true, 1, config)
	if s.config.ErrorThreshold != 3 {
		t.Errorf("expected error threshold 3, got %d", s.config.ErrorThreshold)
	}
}

func TestManagerWithConfig(t *testing.T) {
	config := Config{
		ErrorThreshold:     5,
		EscalateKeywords:   []string{"test"},
		DeescalateKeywords: []string{"simple"},
		Enabled:            true,
	}

	m := NewManagerWithConfig(config)
	got := m.GetConfig()
	if got.ErrorThreshold != 5 {
		t.Errorf("expected error threshold 5, got %d", got.ErrorThreshold)
	}

	newConfig := Config{ErrorThreshold: 10, Enabled: true}
	m.SetConfig(newConfig)
	got = m.GetConfig()
	if got.ErrorThreshold != 10 {
		t.Errorf("expected error threshold 10 after set, got %d", got.ErrorThreshold)
	}
}

func TestDeescalationOnSimpleTask(t *testing.T) {
	config := DefaultConfig()
	s := NewSelectorWithConfig("test", false, 10, config)
	if s.CurrentModel() != ModelSonnet {
		t.Fatalf("expected sonnet, got %s", s.CurrentModel())
	}

	simpleMsg := `{"type":"assistant","content":"This is a simple test formatting task. Just running tests and linting."}`
	changed, model := s.ProcessLine(simpleMsg)
	if !changed {
		t.Error("should deescalate on simple task keywords")
	}
	if model != ModelHaiku {
		t.Errorf("expected haiku after deescalation, got %s", model)
	}

	switches := s.Switches()
	found := false
	for _, sw := range switches {
		if sw.Reason == ReasonDeescalate {
			found = true
			break
		}
	}
	if !found {
		t.Error("should have deescalate reason in switches")
	}
}

func TestNoDeescalationAfterErrors(t *testing.T) {
	config := DefaultConfig()
	s := NewSelectorWithConfig("test", false, 10, config)
	if s.CurrentModel() != ModelSonnet {
		t.Fatalf("expected sonnet, got %s", s.CurrentModel())
	}

	errorMsg := `{"type":"tool_result","tool":"Bash","is_error":true,"result":"command failed"}`
	s.ProcessLine(errorMsg)

	simpleMsg := `{"type":"assistant","content":"This is a simple test formatting task. Just running tests and linting."}`
	changed, model := s.ProcessLine(simpleMsg)
	if changed {
		t.Error("should not deescalate after errors")
	}
	if model != ModelSonnet {
		t.Errorf("expected to stay at sonnet, got %s", model)
	}
}

func TestNoDeescalationFromHaiku(t *testing.T) {
	config := DefaultConfig()
	s := NewSelectorWithConfig("test", true, 1, config)
	if s.CurrentModel() != ModelHaiku {
		t.Fatalf("expected haiku, got %s", s.CurrentModel())
	}

	simpleMsg := `{"type":"assistant","content":"This is a simple test formatting task. Just running tests and linting."}`
	changed, _ := s.ProcessLine(simpleMsg)
	if changed {
		t.Error("should not change when already at haiku")
	}
	if s.CurrentModel() != ModelHaiku {
		t.Errorf("should stay at haiku, got %s", s.CurrentModel())
	}
}

func TestDeescalationFromOpusToSonnet(t *testing.T) {
	config := DefaultConfig()
	s := NewSelectorWithConfig("test", false, 10, config)
	s.ForceModel(ModelOpus, "testing")

	simpleMsg := `{"type":"assistant","content":"This is a simple test formatting task. Just running tests and linting."}`
	changed, model := s.ProcessLine(simpleMsg)
	if !changed {
		t.Error("should deescalate from opus")
	}
	if model != ModelSonnet {
		t.Errorf("expected sonnet, got %s", model)
	}
}

func TestExplicitDeescalationRequest(t *testing.T) {
	config := DefaultConfig()
	s := NewSelectorWithConfig("test", false, 10, config)
	if s.CurrentModel() != ModelSonnet {
		t.Fatalf("expected sonnet, got %s", s.CurrentModel())
	}

	msg := `{"type":"assistant","content":"This is simple, switch to haiku for efficiency."}`
	changed, model := s.ProcessLine(msg)
	if !changed {
		t.Error("should deescalate on explicit request for haiku")
	}
	if model != ModelHaiku {
		t.Errorf("expected haiku, got %s", model)
	}
}

func TestIsLowerTier(t *testing.T) {
	tests := []struct {
		target   string
		current  string
		expected bool
	}{
		{ModelHaiku, ModelSonnet, true},
		{ModelHaiku, ModelOpus, true},
		{ModelSonnet, ModelOpus, true},
		{ModelSonnet, ModelHaiku, false},
		{ModelOpus, ModelHaiku, false},
		{ModelOpus, ModelSonnet, false},
		{ModelHaiku, ModelHaiku, false},
		{ModelSonnet, ModelSonnet, false},
		{ModelOpus, ModelOpus, false},
	}

	for _, tt := range tests {
		t.Run(tt.target+"_from_"+tt.current, func(t *testing.T) {
			if isLowerTier(tt.target, tt.current) != tt.expected {
				t.Errorf("isLowerTier(%s, %s) = %v, want %v",
					tt.target, tt.current, !tt.expected, tt.expected)
			}
		})
	}
}

func TestConfigurableErrorThreshold(t *testing.T) {
	config := Config{
		ErrorThreshold:     5,
		EscalateKeywords:   []string{"architect"},
		DeescalateKeywords: []string{"test"},
		Enabled:            true,
	}

	s := NewSelectorWithConfig("test", true, 1, config)
	errorMsg := `{"type":"tool_result","tool":"Bash","is_error":true,"result":"command failed"}`

	for i := 0; i < 4; i++ {
		changed, _ := s.ProcessLine(errorMsg)
		if changed {
			t.Errorf("shouldn't escalate on error %d with threshold 5", i+1)
		}
	}

	changed, model := s.ProcessLine(errorMsg)
	if !changed {
		t.Error("should escalate on 5th error")
	}
	if model != ModelSonnet {
		t.Errorf("expected sonnet, got %s", model)
	}
}

func TestEscalateOnConfiguredKeywords(t *testing.T) {
	config := Config{
		ErrorThreshold:     2,
		EscalateKeywords:   []string{"customkeyword", "another"},
		DeescalateKeywords: []string{},
		Enabled:            true,
	}

	s := NewSelectorWithConfig("test", true, 1, config)

	msg := `{"type":"assistant","content":"This is a customkeyword task with another pattern."}`
	changed, model := s.ProcessLine(msg)
	if !changed {
		t.Error("should escalate on configured keywords")
	}
	if model != ModelSonnet {
		t.Errorf("expected sonnet, got %s", model)
	}
}

func TestExtractRequestedModelWithHaiku(t *testing.T) {
	tests := []struct {
		content  string
		expected string
	}{
		{"need haiku for this", ModelHaiku},
		{"switch to haiku", ModelHaiku},
		{"this is simple enough", ModelHaiku},
		{"need opus for this", ModelOpus},
		{"escalate to sonnet", ModelSonnet},
	}

	for _, tt := range tests {
		got := extractRequestedModel(tt.content)
		if got != tt.expected {
			t.Errorf("extractRequestedModel(%q) = %q, want %q", tt.content, got, tt.expected)
		}
	}
}

func TestContainsModelEscalationRequestWithHaiku(t *testing.T) {
	positives := []string{
		"I need haiku for this simple task",
		"switch to haiku",
		"this is simple enough",
	}

	for _, content := range positives {
		if !containsModelEscalationRequest(content) {
			t.Errorf("expected escalation request: %s", content)
		}
	}
}
