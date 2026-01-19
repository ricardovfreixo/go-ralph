package automodel

import (
	"encoding/json"
	"fmt"
	"sync"
	"testing"
)

// Integration tests for model auto-selection and escalation

// Test 1: Initial model selection based on task complexity
func TestInitialModelSelectionByTaskComplexity(t *testing.T) {
	tests := []struct {
		name       string
		isLeafTask bool
		taskCount  int
		wantModel  string
	}{
		{
			name:       "leaf task starts with haiku",
			isLeafTask: true,
			taskCount:  0,
			wantModel:  ModelHaiku,
		},
		{
			name:       "1 task starts with haiku",
			isLeafTask: false,
			taskCount:  1,
			wantModel:  ModelHaiku,
		},
		{
			name:       "2 tasks starts with haiku",
			isLeafTask: false,
			taskCount:  2,
			wantModel:  ModelHaiku,
		},
		{
			name:       "5 tasks starts with haiku",
			isLeafTask: false,
			taskCount:  5,
			wantModel:  ModelHaiku,
		},
		{
			name:       "6 tasks starts with sonnet",
			isLeafTask: false,
			taskCount:  6,
			wantModel:  ModelSonnet,
		},
		{
			name:       "10 tasks starts with sonnet",
			isLeafTask: false,
			taskCount:  10,
			wantModel:  ModelSonnet,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewSelector("test-feature", tt.isLeafTask, tt.taskCount)
			if s.CurrentModel() != tt.wantModel {
				t.Errorf("CurrentModel() = %q, want %q", s.CurrentModel(), tt.wantModel)
			}
		})
	}
}

// Test 2: Model escalation on consecutive errors
func TestModelEscalationOnConsecutiveErrors(t *testing.T) {
	s := NewSelector("test-feature", true, 1)

	// Should start with haiku
	if s.CurrentModel() != ModelHaiku {
		t.Fatalf("expected haiku, got %s", s.CurrentModel())
	}

	// First error - no escalation
	s.ProcessLine(`{"type":"tool_result","is_error":true,"result":"command failed"}`)
	if s.CurrentModel() != ModelHaiku {
		t.Errorf("after 1 error: model = %s, want haiku", s.CurrentModel())
	}

	// Second error - should escalate to sonnet
	changed, model := s.ProcessLine(`{"type":"tool_result","is_error":true,"result":"command failed again"}`)
	if !changed {
		t.Error("expected model change after 2 errors")
	}
	if model != ModelSonnet {
		t.Errorf("after 2 errors: model = %s, want sonnet", model)
	}
}

// Test 3: Model escalation on test failures
func TestModelEscalationOnTestFailures(t *testing.T) {
	s := NewSelector("test-feature", false, 5)

	// Test failures should trigger escalation after threshold
	s.ProcessLine(`{"type":"tool_result","is_error":true,"result":"--- FAIL: TestFoo"}`)
	s.ProcessLine(`{"type":"tool_result","is_error":true,"result":"test failed: expected 1, got 2"}`)

	if s.CurrentModel() != ModelSonnet {
		t.Errorf("after test failures: model = %s, want sonnet", s.CurrentModel())
	}
}

// Test 4: Model escalation on compilation errors
func TestModelEscalationOnCompilationErrors(t *testing.T) {
	s := NewSelector("test-feature", false, 5)

	// Compilation errors should trigger escalation
	s.ProcessLine(`{"type":"tool_result","is_error":true,"result":"build failed: undefined: myFunc"}`)
	s.ProcessLine(`{"type":"tool_result","is_error":true,"result":"compilation failed: syntax error"}`)

	if s.CurrentModel() != ModelSonnet {
		t.Errorf("after compilation errors: model = %s, want sonnet", s.CurrentModel())
	}
}

// Test 5: Model escalation on architectural content
func TestModelEscalationOnArchitecturalContent(t *testing.T) {
	tests := []struct {
		name    string
		content string
	}{
		{"architectural decision", `{"type":"tool_result","result":"This requires an architectural decision about the data model"}`},
		{"design pattern", `{"type":"tool_result","result":"We should use a design pattern here for extensibility"}`},
		{"system design", `{"type":"tool_result","result":"The system design needs to consider scalability"}`},
		{"api design", `{"type":"tool_result","result":"The API design should follow REST conventions"}`},
		{"database schema", `{"type":"tool_result","result":"The database schema needs normalization"}`},
		{"trade-off", `{"type":"tool_result","result":"There's a trade-off between performance and maintainability"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewSelector("test-feature", true, 1)
			changed, model := s.ProcessLine(tt.content)

			if !changed {
				t.Error("expected model change on architectural content")
			}
			if model != ModelSonnet {
				t.Errorf("model = %s, want sonnet", model)
			}
		})
	}
}

// Test 6: Model escalation on debugging scenarios
func TestModelEscalationOnDebuggingScenarios(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{"stack trace", `{"type":"tool_result","result":"Got a stack trace:\npanic: runtime error"}`, ModelSonnet},
		{"simple debugging", `{"type":"tool_result","result":"Need to debug this issue"}`, ModelSonnet},
		{"race condition", `{"type":"tool_result","result":"This looks like a race condition"}`, ModelOpus},
		{"deadlock", `{"type":"tool_result","result":"The code has a deadlock"}`, ModelOpus},
		{"memory leak", `{"type":"tool_result","result":"Detected a memory leak"}`, ModelOpus},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewSelector("test-feature", true, 1)
			// May need to escalate to sonnet first for some cases
			if tt.want == ModelOpus {
				// First escalate to sonnet
				s.ProcessLine(`{"type":"tool_result","result":"debugging issue"}`)
			}
			s.ProcessLine(tt.content)

			if s.CurrentModel() != tt.want {
				t.Errorf("model = %s, want %s", s.CurrentModel(), tt.want)
			}
		})
	}
}

// Test 7: Explicit escalation request
func TestExplicitEscalationRequest(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{"need opus", `{"type":"assistant","content":"I need opus for this complex task"}`, ModelOpus},
		{"escalate to sonnet", `{"type":"assistant","content":"Let me escalate to sonnet for better results"}`, ModelSonnet},
		{"require opus", `{"type":"assistant","content":"This will require opus level reasoning"}`, ModelOpus},
		{"switch to opus", `{"type":"assistant","content":"I'll switch to opus for the architecture work"}`, ModelOpus},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewSelector("test-feature", true, 1)
			_, model := s.ProcessLine(tt.content)

			if model != tt.want {
				t.Errorf("model = %s, want %s", model, tt.want)
			}
		})
	}
}

// Test 8: De-escalation for simple tasks
func TestDeescalationForSimpleTasks(t *testing.T) {
	s := NewSelector("test-feature", false, 8) // Starts with sonnet

	if s.CurrentModel() != ModelSonnet {
		t.Fatalf("expected sonnet initial model, got %s", s.CurrentModel())
	}

	// Content with multiple de-escalation keywords
	changed, model := s.ProcessLine(`{"type":"assistant","content":"This is a simple test for formatting the output"}`)

	if !changed {
		t.Error("expected model change for simple task")
	}
	if model != ModelHaiku {
		t.Errorf("model = %s, want haiku for simple task", model)
	}
}

// Test 9: No de-escalation after errors (integration)
func TestNoDeescalationAfterErrorsIntegration(t *testing.T) {
	s := NewSelector("test-feature", false, 8)

	// Record an error
	s.ProcessLine(`{"type":"tool_result","is_error":true,"result":"error occurred"}`)

	// Try to de-escalate
	changed, _ := s.ProcessLine(`{"type":"assistant","content":"This is a simple test for formatting"}`)

	if changed {
		t.Error("should not de-escalate after errors")
	}
}

// Test 10: Model switch tracking
func TestModelSwitchTracking(t *testing.T) {
	s := NewSelector("test-feature", true, 1)

	// Initial switch
	switches := s.Switches()
	if len(switches) != 1 {
		t.Fatalf("expected 1 initial switch, got %d", len(switches))
	}
	if switches[0].Reason != ReasonInitial {
		t.Errorf("first switch reason = %s, want initial", switches[0].Reason)
	}

	// Trigger escalation
	s.ProcessLine(`{"type":"tool_result","is_error":true,"result":"error"}`)
	s.ProcessLine(`{"type":"tool_result","is_error":true,"result":"error again"}`)

	switches = s.Switches()
	if len(switches) != 2 {
		t.Fatalf("expected 2 switches, got %d", len(switches))
	}
	if switches[1].FromModel != ModelHaiku || switches[1].ToModel != ModelSonnet {
		t.Errorf("switch[1] = %s->%s, want haiku->sonnet", switches[1].FromModel, switches[1].ToModel)
	}
}

// Test 11: Force model override
func TestForceModelOverride(t *testing.T) {
	s := NewSelector("test-feature", true, 1)

	s.ForceModel(ModelOpus, "user requested")

	if s.CurrentModel() != ModelOpus {
		t.Errorf("model = %s, want opus", s.CurrentModel())
	}

	switches := s.Switches()
	if len(switches) != 2 {
		t.Fatalf("expected 2 switches, got %d", len(switches))
	}
	if switches[1].Reason != ReasonExplicitRequest {
		t.Errorf("switch reason = %s, want explicit_request", switches[1].Reason)
	}
}

// Test 12: Manager registration and retrieval
func TestManagerRegistrationAndRetrieval(t *testing.T) {
	m := NewManager()

	s1 := m.Register("feature-01", false, 5)
	s2 := m.Register("feature-02", true, 1)

	if s1.CurrentModel() != ModelHaiku {
		t.Errorf("feature-01 model = %s, want haiku", s1.CurrentModel())
	}
	if s2.CurrentModel() != ModelHaiku {
		t.Errorf("feature-02 model = %s, want haiku", s2.CurrentModel())
	}

	// Retrieve by ID
	retrieved := m.Get("feature-01")
	if retrieved != s1 {
		t.Error("expected to retrieve same selector")
	}

	// Non-existent
	if m.Get("nonexistent") != nil {
		t.Error("expected nil for non-existent feature")
	}
}

// Test 13: Manager with custom config
func TestManagerWithCustomConfig(t *testing.T) {
	config := Config{
		ErrorThreshold:     3, // Higher threshold
		EscalateKeywords:   []string{"complex", "difficult"},
		DeescalateKeywords: []string{"easy", "simple"},
		Enabled:            true,
	}

	m := NewManagerWithConfig(config)
	s := m.Register("test", true, 1)

	// Need 3 errors now instead of 2
	s.ProcessLine(`{"type":"tool_result","is_error":true,"result":"error 1"}`)
	s.ProcessLine(`{"type":"tool_result","is_error":true,"result":"error 2"}`)
	if s.CurrentModel() != ModelHaiku {
		t.Error("should not escalate after 2 errors with threshold 3")
	}

	s.ProcessLine(`{"type":"tool_result","is_error":true,"result":"error 3"}`)
	if s.CurrentModel() != ModelSonnet {
		t.Error("should escalate after 3 errors")
	}
}

// Test 14: IsAutoMode check
func TestIsAutoModeCheck(t *testing.T) {
	tests := []struct {
		model string
		want  bool
	}{
		{"auto", true},
		{"haiku", false},
		{"sonnet", false},
		{"opus", false},
		{"AUTO", false}, // Case sensitive
		{"Auto", false},
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			if got := IsAutoMode(tt.model); got != tt.want {
				t.Errorf("IsAutoMode(%q) = %v, want %v", tt.model, got, tt.want)
			}
		})
	}
}

// Test 15: Tier comparison functions
func TestTierComparison(t *testing.T) {
	tests := []struct {
		target  string
		current string
		higher  bool
		lower   bool
	}{
		{ModelSonnet, ModelHaiku, true, false},
		{ModelOpus, ModelHaiku, true, false},
		{ModelOpus, ModelSonnet, true, false},
		{ModelHaiku, ModelSonnet, false, true},
		{ModelHaiku, ModelOpus, false, true},
		{ModelSonnet, ModelOpus, false, true},
		{ModelHaiku, ModelHaiku, false, false},
		{ModelSonnet, ModelSonnet, false, false},
		{ModelOpus, ModelOpus, false, false},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s_vs_%s", tt.target, tt.current), func(t *testing.T) {
			if got := isHigherTier(tt.target, tt.current); got != tt.higher {
				t.Errorf("isHigherTier(%s, %s) = %v, want %v", tt.target, tt.current, got, tt.higher)
			}
			if got := isLowerTier(tt.target, tt.current); got != tt.lower {
				t.Errorf("isLowerTier(%s, %s) = %v, want %v", tt.target, tt.current, got, tt.lower)
			}
		})
	}
}

// Test 16: No escalation from opus ceiling (integration)
func TestNoEscalationFromOpusCeiling(t *testing.T) {
	s := NewSelector("test", true, 1)
	s.ForceModel(ModelOpus, "start at opus")

	// Try to trigger escalation
	s.ProcessLine(`{"type":"tool_result","is_error":true,"result":"error"}`)
	s.ProcessLine(`{"type":"tool_result","is_error":true,"result":"error"}`)
	s.ProcessLine(`{"type":"tool_result","is_error":true,"result":"error"}`)
	s.ProcessLine(`{"type":"assistant","content":"Need opus for this architectural decision"}`)

	if s.CurrentModel() != ModelOpus {
		t.Errorf("model should still be opus, got %s", s.CurrentModel())
	}
}

// Test 17: Concurrent access safety
func TestConcurrentAccessSafety(t *testing.T) {
	s := NewSelector("test", false, 5)

	var wg sync.WaitGroup
	errCh := make(chan error, 100)

	// Concurrent processing
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			var line string
			if i%5 == 0 {
				line = `{"type":"tool_result","is_error":true,"result":"error"}`
			} else {
				line = fmt.Sprintf(`{"type":"assistant","usage":{"input_tokens":%d}}`, i)
			}
			_, _ = s.ProcessLine(line)
		}(i)
	}

	// Concurrent reads
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = s.CurrentModel()
			_ = s.Switches()
		}()
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		t.Errorf("concurrent error: %v", err)
	}
}

// Test 18: Process line with invalid JSON
func TestProcessLineWithInvalidJSON(t *testing.T) {
	s := NewSelector("test", true, 1)

	changed, model := s.ProcessLine("not valid json")
	if changed {
		t.Error("should not change model on invalid JSON")
	}
	if model != ModelHaiku {
		t.Errorf("model = %s, want haiku", model)
	}
}

// Test 19: Process line with unknown message type
func TestProcessLineWithUnknownMessageType(t *testing.T) {
	s := NewSelector("test", true, 1)

	changed, model := s.ProcessLine(`{"type":"unknown","data":"something"}`)
	if changed {
		t.Error("should not change model on unknown message type")
	}
	if model != ModelHaiku {
		t.Errorf("model = %s, want haiku", model)
	}
}

// Test 20: Assistant message content extraction
func TestAssistantMessageContentExtraction(t *testing.T) {
	tests := []struct {
		name    string
		line    string
		escalate bool
	}{
		{
			name: "direct content field",
			line: `{"type":"assistant","content":"I need opus for this architectural decision"}`,
			escalate: true,
		},
		{
			name: "nested message.content",
			line: `{"type":"assistant","message":{"content":"This requires opus for the system design"}}`,
			escalate: true,
		},
		{
			name: "nested message.text",
			line: `{"type":"assistant","message":{"text":"We need opus for API design"}}`,
			escalate: true,
		},
		{
			name: "empty content",
			line: `{"type":"assistant","content":""}`,
			escalate: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewSelector("test", true, 1)
			changed, _ := s.ProcessLine(tt.line)

			if changed != tt.escalate {
				t.Errorf("escalation = %v, want %v", changed, tt.escalate)
			}
		})
	}
}

// Test 21: Real-world session simulation
func TestRealWorldSessionSimulation(t *testing.T) {
	s := NewSelector("feature-auth", false, 8)

	// Session starts with sonnet due to task count
	if s.CurrentModel() != ModelSonnet {
		t.Fatalf("expected sonnet for 8 tasks, got %s", s.CurrentModel())
	}

	session := []struct {
		line      string
		wantModel string
	}{
		// Initial setup and exploration - stays at sonnet
		{`{"type":"assistant","usage":{"input_tokens":1000}}`, ModelSonnet},
		{`{"type":"tool_use","tool":"Glob","tool_input":{"pattern":"**/*.go"}}`, ModelSonnet},
		{`{"type":"tool_result","result":"auth/handler.go\nauth/jwt.go"}`, ModelSonnet},
		// Reading code
		{`{"type":"tool_use","tool":"Read","tool_input":{"file_path":"auth/handler.go"}}`, ModelSonnet},
		{`{"type":"tool_result","result":"package auth\n\nfunc Login()..."}`, ModelSonnet},
		// First test failure
		{`{"type":"tool_use","tool":"Bash","tool_input":{"command":"go test ./..."}}`, ModelSonnet},
		{`{"type":"tool_result","is_error":true,"result":"--- FAIL: TestLogin"}`, ModelSonnet},
		// Second test failure - escalate to opus
		{`{"type":"tool_result","is_error":true,"result":"test failed: expected token"}`, ModelOpus},
		// Continue with opus
		{`{"type":"assistant","content":"I'll fix this authentication issue"}`, ModelOpus},
	}

	for i, step := range session {
		s.ProcessLine(step.line)
		if s.CurrentModel() != step.wantModel {
			t.Errorf("step[%d]: model = %s, want %s", i, s.CurrentModel(), step.wantModel)
		}
	}

	// Verify switch history
	switches := s.Switches()
	if len(switches) < 2 {
		t.Errorf("expected at least 2 switches, got %d", len(switches))
	}
}

// Test 22: Manager GetAllSwitches retrieval (integration)
func TestManagerGetAllSwitchesRetrieval(t *testing.T) {
	m := NewManager()
	s := m.Register("feature-01", true, 1)

	// Trigger escalation
	s.ProcessLine(`{"type":"tool_result","is_error":true,"result":"error"}`)
	s.ProcessLine(`{"type":"tool_result","is_error":true,"result":"error"}`)

	switches := m.GetAllSwitches("feature-01")
	if len(switches) != 2 {
		t.Errorf("expected 2 switches, got %d", len(switches))
	}

	// Non-existent feature
	if m.GetAllSwitches("nonexistent") != nil {
		t.Error("expected nil for non-existent feature")
	}
}

// Test 23: Manager Remove
func TestManagerRemove(t *testing.T) {
	m := NewManager()
	m.Register("feature-01", true, 1)

	if m.Get("feature-01") == nil {
		t.Fatal("expected feature to be registered")
	}

	m.Remove("feature-01")

	if m.Get("feature-01") != nil {
		t.Error("expected feature to be removed")
	}
}

// Test 24: Complex debugging triggers opus
func TestComplexDebuggingTriggersOpus(t *testing.T) {
	s := NewSelector("test", false, 5)

	// First escalate to sonnet
	s.ProcessLine(`{"type":"tool_result","result":"debugging the issue"}`)

	// Then hit complex debugging
	s.ProcessLine(`{"type":"tool_result","result":"Found a race condition in the concurrent code"}`)

	if s.CurrentModel() != ModelOpus {
		t.Errorf("model = %s, want opus for race condition", s.CurrentModel())
	}
}

// Test 25: Keyword configuration
func TestKeywordConfiguration(t *testing.T) {
	config := DefaultConfig()

	// Verify default escalation keywords
	found := false
	for _, kw := range config.EscalateKeywords {
		if kw == "architect" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'architect' in default escalation keywords")
	}

	// Verify default de-escalation keywords
	found = false
	for _, kw := range config.DeescalateKeywords {
		if kw == "simple" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'simple' in default de-escalation keywords")
	}
}

// Test 26: ForceModel same model is no-op
func TestForceModelSameModelIsNoOp(t *testing.T) {
	s := NewSelector("test", true, 1)
	initialSwitchCount := len(s.Switches())

	s.ForceModel(ModelHaiku, "trying to force same")

	if len(s.Switches()) != initialSwitchCount {
		t.Error("should not add switch when forcing same model")
	}
}

// Test 27: ModelSwitch JSON serialization
func TestModelSwitchJSONSerialization(t *testing.T) {
	sw := ModelSwitch{
		FromModel:  ModelHaiku,
		ToModel:    ModelSonnet,
		Reason:     ReasonToolError,
		Details:    "2+ tool errors",
		TokenCount: 1500,
	}

	data, err := json.Marshal(sw)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var decoded ModelSwitch
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if decoded.FromModel != sw.FromModel || decoded.ToModel != sw.ToModel {
		t.Error("decoded switch doesn't match original")
	}
}
