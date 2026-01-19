package retry

import (
	"sync"
	"testing"
	"time"
)

func TestNewAdjustmentHistory(t *testing.T) {
	h := NewAdjustmentHistory("feature-1", "sonnet")

	if h.featureID != "feature-1" {
		t.Errorf("expected featureID 'feature-1', got '%s'", h.featureID)
	}
	if h.currentModel != "sonnet" {
		t.Errorf("expected currentModel 'sonnet', got '%s'", h.currentModel)
	}
	if h.originalModel != "sonnet" {
		t.Errorf("expected originalModel 'sonnet', got '%s'", h.originalModel)
	}
	if len(h.adjustments) != 0 {
		t.Errorf("expected 0 adjustments, got %d", len(h.adjustments))
	}
}

func TestAdjustmentHistoryAddAdjustment(t *testing.T) {
	h := NewAdjustmentHistory("feature-1", "haiku")

	adj := Adjustment{
		Type:       AdjustmentModelEscalation,
		Reason:     ReasonTestFailures,
		FromValue:  "haiku",
		ToValue:    "sonnet",
		Details:    "2 test failures",
		AttemptNum: 1,
	}
	h.AddAdjustment(adj)

	if h.Count() != 1 {
		t.Errorf("expected 1 adjustment, got %d", h.Count())
	}

	adjs := h.GetAdjustments()
	if len(adjs) != 1 {
		t.Fatalf("expected 1 adjustment, got %d", len(adjs))
	}
	if adjs[0].Type != AdjustmentModelEscalation {
		t.Errorf("expected type 'model_escalation', got '%s'", adjs[0].Type)
	}
}

func TestAdjustmentHistoryLastAdjustment(t *testing.T) {
	h := NewAdjustmentHistory("feature-1", "haiku")

	// No adjustments
	if h.LastAdjustment() != nil {
		t.Error("expected nil for no adjustments")
	}

	h.AddAdjustment(Adjustment{Type: AdjustmentModelEscalation, AttemptNum: 1})
	h.AddAdjustment(Adjustment{Type: AdjustmentTaskSimplify, AttemptNum: 2})

	last := h.LastAdjustment()
	if last == nil {
		t.Fatal("expected non-nil last adjustment")
	}
	if last.Type != AdjustmentTaskSimplify {
		t.Errorf("expected type 'task_simplify', got '%s'", last.Type)
	}
}

func TestAdjustmentHistoryModelTracking(t *testing.T) {
	h := NewAdjustmentHistory("feature-1", "haiku")

	if h.GetCurrentModel() != "haiku" {
		t.Errorf("expected current model 'haiku', got '%s'", h.GetCurrentModel())
	}
	if h.GetOriginalModel() != "haiku" {
		t.Errorf("expected original model 'haiku', got '%s'", h.GetOriginalModel())
	}

	h.SetCurrentModel("sonnet")
	if h.GetCurrentModel() != "sonnet" {
		t.Errorf("expected current model 'sonnet', got '%s'", h.GetCurrentModel())
	}
	if h.GetOriginalModel() != "haiku" {
		t.Errorf("expected original model still 'haiku', got '%s'", h.GetOriginalModel())
	}
}

func TestAdjustmentHistorySimplified(t *testing.T) {
	h := NewAdjustmentHistory("feature-1", "sonnet")

	if h.IsSimplified() {
		t.Error("expected not simplified initially")
	}

	h.SetSimplified(true)
	if !h.IsSimplified() {
		t.Error("expected simplified after SetSimplified(true)")
	}
}

func TestAdjustmentHistoryHasModelEscalation(t *testing.T) {
	h := NewAdjustmentHistory("feature-1", "haiku")

	if h.HasModelEscalation() {
		t.Error("expected no model escalation initially")
	}

	h.AddAdjustment(Adjustment{Type: AdjustmentTaskSimplify})
	if h.HasModelEscalation() {
		t.Error("expected no model escalation after task simplify")
	}

	h.AddAdjustment(Adjustment{Type: AdjustmentModelEscalation})
	if !h.HasModelEscalation() {
		t.Error("expected model escalation after adding one")
	}
}

func TestAdjustmentHistorySummary(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(h *AdjustmentHistory)
		contains string
	}{
		{
			name:     "no adjustments",
			setup:    func(h *AdjustmentHistory) {},
			contains: "No adjustments",
		},
		{
			name: "model escalation",
			setup: func(h *AdjustmentHistory) {
				h.AddAdjustment(Adjustment{
					Type:       AdjustmentModelEscalation,
					FromValue:  "haiku",
					ToValue:    "sonnet",
					AttemptNum: 1,
				})
			},
			contains: "Model: haiku → sonnet",
		},
		{
			name: "task simplification",
			setup: func(h *AdjustmentHistory) {
				h.AddAdjustment(Adjustment{
					Type:       AdjustmentTaskSimplify,
					Details:    "reduced tasks",
					AttemptNum: 2,
				})
			},
			contains: "Tasks simplified",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewAdjustmentHistory("test", "haiku")
			tt.setup(h)
			summary := h.Summary()
			if !containsStr(summary, tt.contains) {
				t.Errorf("expected summary to contain '%s', got '%s'", tt.contains, summary)
			}
		})
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.MaxAdjustments != DefaultMaxAdjustments {
		t.Errorf("expected max adjustments %d, got %d", DefaultMaxAdjustments, cfg.MaxAdjustments)
	}
	if cfg.MaxRetries != DefaultMaxRetries {
		t.Errorf("expected max retries %d, got %d", DefaultMaxRetries, cfg.MaxRetries)
	}
	if !cfg.EnableEscalation {
		t.Error("expected escalation enabled by default")
	}
	if !cfg.EnableSimplify {
		t.Error("expected simplify enabled by default")
	}
}

func TestNewStrategy(t *testing.T) {
	s := NewStrategy()

	if s == nil {
		t.Fatal("expected non-nil strategy")
	}
	cfg := s.GetConfig()
	if cfg.MaxAdjustments != DefaultMaxAdjustments {
		t.Errorf("expected default max adjustments")
	}
}

func TestNewStrategyWithConfig(t *testing.T) {
	cfg := Config{
		MaxAdjustments:   5,
		MaxRetries:       10,
		EnableEscalation: false,
		EnableSimplify:   true,
	}
	s := NewStrategyWithConfig(cfg)

	got := s.GetConfig()
	if got.MaxAdjustments != 5 {
		t.Errorf("expected max adjustments 5, got %d", got.MaxAdjustments)
	}
	if got.MaxRetries != 10 {
		t.Errorf("expected max retries 10, got %d", got.MaxRetries)
	}
}

func TestStrategyRegisterFeature(t *testing.T) {
	s := NewStrategy()

	h := s.RegisterFeature("feature-1", "sonnet")
	if h == nil {
		t.Fatal("expected non-nil history")
	}

	retrieved := s.GetHistory("feature-1")
	if retrieved != h {
		t.Error("expected same history from GetHistory")
	}

	if s.GetHistory("nonexistent") != nil {
		t.Error("expected nil for nonexistent feature")
	}
}

func TestStrategyRemoveFeature(t *testing.T) {
	s := NewStrategy()

	s.RegisterFeature("feature-1", "sonnet")
	if s.GetHistory("feature-1") == nil {
		t.Fatal("expected history to exist")
	}

	s.RemoveFeature("feature-1")
	if s.GetHistory("feature-1") != nil {
		t.Error("expected history to be nil after removal")
	}
}

func TestStrategyDecideRetryMaxRetries(t *testing.T) {
	s := NewStrategyWithConfig(Config{
		MaxRetries:     3,
		MaxAdjustments: 3,
	})

	decision := s.DecideRetry(FailureContext{
		FeatureID:  "feature-1",
		AttemptNum: 3,
	})

	if decision.ShouldRetry {
		t.Error("expected ShouldRetry false at max retries")
	}
	if decision.Reason != ReasonMaxAttemptsReached {
		t.Errorf("expected reason 'max_attempts_reached', got '%s'", decision.Reason)
	}
}

func TestStrategyDecideRetryModelEscalation(t *testing.T) {
	s := NewStrategy()
	s.RegisterFeature("feature-1", "haiku")

	decision := s.DecideRetry(FailureContext{
		FeatureID:     "feature-1",
		AttemptNum:    2,
		TestsFailed:   2,
		CurrentModel:  "haiku",
		HasBuildError: true,
	})

	if !decision.ShouldRetry {
		t.Error("expected ShouldRetry true")
	}
	if !decision.ShouldAdjust {
		t.Error("expected ShouldAdjust true")
	}
	if decision.AdjustmentType != AdjustmentModelEscalation {
		t.Errorf("expected adjustment type 'model_escalation', got '%s'", decision.AdjustmentType)
	}
	if decision.NewModel != "sonnet" {
		t.Errorf("expected new model 'sonnet', got '%s'", decision.NewModel)
	}
}

func TestStrategyDecideRetryAtOpus(t *testing.T) {
	s := NewStrategy()
	s.RegisterFeature("feature-1", "opus")

	decision := s.DecideRetry(FailureContext{
		FeatureID:     "feature-1",
		AttemptNum:    1,
		TestsFailed:   2,
		CurrentModel:  "opus",
		HasBuildError: true,
	})

	if !decision.ShouldRetry {
		t.Error("expected ShouldRetry true")
	}
	// Should not escalate from opus
	if decision.AdjustmentType == AdjustmentModelEscalation {
		t.Error("should not escalate from opus")
	}
}

func TestStrategyDecideRetryNoEscalationDisabled(t *testing.T) {
	s := NewStrategyWithConfig(Config{
		MaxAdjustments:   3,
		MaxRetries:       3,
		EnableEscalation: false,
		EnableSimplify:   false,
	})
	s.RegisterFeature("feature-1", "haiku")

	decision := s.DecideRetry(FailureContext{
		FeatureID:     "feature-1",
		AttemptNum:    1,
		TestsFailed:   5,
		CurrentModel:  "haiku",
		HasBuildError: true,
	})

	if !decision.ShouldRetry {
		t.Error("expected ShouldRetry true")
	}
	if decision.ShouldAdjust {
		t.Error("expected ShouldAdjust false when escalation disabled")
	}
}

func TestStrategyRecordAdjustment(t *testing.T) {
	s := NewStrategy()
	s.RegisterFeature("feature-1", "haiku")

	adj := Adjustment{
		Type:       AdjustmentModelEscalation,
		FromValue:  "haiku",
		ToValue:    "sonnet",
		AttemptNum: 1,
	}
	s.RecordAdjustment("feature-1", adj)

	h := s.GetHistory("feature-1")
	if h.Count() != 1 {
		t.Errorf("expected 1 adjustment, got %d", h.Count())
	}
	if h.GetCurrentModel() != "sonnet" {
		t.Errorf("expected current model 'sonnet', got '%s'", h.GetCurrentModel())
	}
}

func TestStrategyRecordAdjustmentNewFeature(t *testing.T) {
	s := NewStrategy()

	adj := Adjustment{
		Type:       AdjustmentModelEscalation,
		FromValue:  "haiku",
		ToValue:    "sonnet",
		AttemptNum: 1,
	}
	s.RecordAdjustment("new-feature", adj)

	h := s.GetHistory("new-feature")
	if h == nil {
		t.Fatal("expected history to be created")
	}
	if h.Count() != 1 {
		t.Errorf("expected 1 adjustment, got %d", h.Count())
	}
}

func TestStrategyCanRetry(t *testing.T) {
	s := NewStrategyWithConfig(Config{MaxRetries: 3})

	if !s.CanRetry("feature-1", 0) {
		t.Error("expected CanRetry true at attempt 0")
	}
	if !s.CanRetry("feature-1", 2) {
		t.Error("expected CanRetry true at attempt 2")
	}
	if s.CanRetry("feature-1", 3) {
		t.Error("expected CanRetry false at attempt 3")
	}
}

func TestStrategyCanAdjust(t *testing.T) {
	s := NewStrategyWithConfig(Config{MaxAdjustments: 2})

	if !s.CanAdjust("feature-1") {
		t.Error("expected CanAdjust true for new feature")
	}

	s.RegisterFeature("feature-1", "haiku")
	if !s.CanAdjust("feature-1") {
		t.Error("expected CanAdjust true with 0 adjustments")
	}

	s.RecordAdjustment("feature-1", Adjustment{Type: AdjustmentModelEscalation})
	s.RecordAdjustment("feature-1", Adjustment{Type: AdjustmentTaskSimplify})

	if s.CanAdjust("feature-1") {
		t.Error("expected CanAdjust false at max adjustments")
	}
}

func TestGetEscalatedModel(t *testing.T) {
	s := NewStrategy()

	tests := []struct {
		current  string
		expected string
	}{
		{"haiku", "sonnet"},
		{"sonnet", "opus"},
		{"opus", "opus"},
		{"unknown", "unknown"},
	}

	for _, tt := range tests {
		got := s.getEscalatedModel(tt.current)
		if got != tt.expected {
			t.Errorf("getEscalatedModel(%s) = %s, want %s", tt.current, got, tt.expected)
		}
	}
}

func TestFailureContextFields(t *testing.T) {
	ctx := FailureContext{
		FeatureID:     "test-feature",
		AttemptNum:    2,
		LastError:     "build failed",
		TestsFailed:   3,
		TestsPassed:   10,
		HasBuildError: true,
		HasTimeout:    false,
		TaskCount:     5,
		CurrentModel:  "sonnet",
		LastModel:     "haiku",
	}

	if ctx.FeatureID != "test-feature" {
		t.Error("FeatureID not set correctly")
	}
	if ctx.AttemptNum != 2 {
		t.Error("AttemptNum not set correctly")
	}
	if ctx.TestsFailed != 3 {
		t.Error("TestsFailed not set correctly")
	}
}

func TestConcurrentAccess(t *testing.T) {
	s := NewStrategy()
	s.RegisterFeature("feature-1", "haiku")

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			s.DecideRetry(FailureContext{
				FeatureID:  "feature-1",
				AttemptNum: i % 3,
			})
		}(i)
	}
	wg.Wait()
}

func TestAdjustmentHistoryConcurrent(t *testing.T) {
	h := NewAdjustmentHistory("feature-1", "haiku")

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(2)
		go func(i int) {
			defer wg.Done()
			h.AddAdjustment(Adjustment{
				Type:       AdjustmentModelEscalation,
				AttemptNum: i,
			})
		}(i)
		go func() {
			defer wg.Done()
			h.GetAdjustments()
			h.Count()
			h.Summary()
		}()
	}
	wg.Wait()
}

func TestAdjustmentTimestamp(t *testing.T) {
	h := NewAdjustmentHistory("feature-1", "haiku")

	before := time.Now()
	h.AddAdjustment(Adjustment{Type: AdjustmentModelEscalation})
	after := time.Now()

	adjs := h.GetAdjustments()
	if len(adjs) != 1 {
		t.Fatal("expected 1 adjustment")
	}

	if adjs[0].Timestamp.Before(before) || adjs[0].Timestamp.After(after) {
		t.Error("timestamp not in expected range")
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
