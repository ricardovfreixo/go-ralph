package retry

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/vx/ralph-go/internal/logger"
)

// AdjustmentType represents the type of adjustment made to a retry
type AdjustmentType string

const (
	AdjustmentNone             AdjustmentType = "none"
	AdjustmentModelEscalation  AdjustmentType = "model_escalation"
	AdjustmentTaskSimplify     AdjustmentType = "task_simplify"
	AdjustmentContextExpand    AdjustmentType = "context_expand"
	AdjustmentPromptRefine     AdjustmentType = "prompt_refine"
)

// AdjustmentReason provides context for why an adjustment was made
type AdjustmentReason string

const (
	ReasonRepeatedFailures   AdjustmentReason = "repeated_failures"
	ReasonTestFailures       AdjustmentReason = "test_failures"
	ReasonCompilationErrors  AdjustmentReason = "compilation_errors"
	ReasonTimeout            AdjustmentReason = "timeout"
	ReasonComplexTask        AdjustmentReason = "complex_task"
	ReasonMaxAttemptsReached AdjustmentReason = "max_attempts_reached"
)

const (
	DefaultMaxAdjustments = 3
	DefaultMaxRetries     = 3
)

// Adjustment represents a single adjustment made during retry
type Adjustment struct {
	Timestamp   time.Time        `json:"timestamp"`
	Type        AdjustmentType   `json:"type"`
	Reason      AdjustmentReason `json:"reason"`
	FromValue   string           `json:"from_value,omitempty"`
	ToValue     string           `json:"to_value,omitempty"`
	Details     string           `json:"details,omitempty"`
	AttemptNum  int              `json:"attempt_num"`
}

// AdjustmentHistory tracks all adjustments made for a feature
type AdjustmentHistory struct {
	mu            sync.RWMutex
	featureID     string
	adjustments   []Adjustment
	currentModel  string
	originalModel string
	simplified    bool
}

// NewAdjustmentHistory creates a new adjustment history for a feature
func NewAdjustmentHistory(featureID, initialModel string) *AdjustmentHistory {
	return &AdjustmentHistory{
		featureID:     featureID,
		adjustments:   make([]Adjustment, 0),
		currentModel:  initialModel,
		originalModel: initialModel,
	}
}

// AddAdjustment records a new adjustment
func (h *AdjustmentHistory) AddAdjustment(adj Adjustment) {
	h.mu.Lock()
	defer h.mu.Unlock()

	adj.Timestamp = time.Now()
	h.adjustments = append(h.adjustments, adj)

	logger.Info("retry", "Adjustment recorded",
		"featureID", h.featureID,
		"type", string(adj.Type),
		"reason", string(adj.Reason),
		"from", adj.FromValue,
		"to", adj.ToValue)
}

// GetAdjustments returns a copy of all adjustments
func (h *AdjustmentHistory) GetAdjustments() []Adjustment {
	h.mu.RLock()
	defer h.mu.RUnlock()

	result := make([]Adjustment, len(h.adjustments))
	copy(result, h.adjustments)
	return result
}

// Count returns the number of adjustments made
func (h *AdjustmentHistory) Count() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.adjustments)
}

// LastAdjustment returns the most recent adjustment, or nil if none
func (h *AdjustmentHistory) LastAdjustment() *Adjustment {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if len(h.adjustments) == 0 {
		return nil
	}
	adj := h.adjustments[len(h.adjustments)-1]
	return &adj
}

// SetCurrentModel updates the current model
func (h *AdjustmentHistory) SetCurrentModel(model string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.currentModel = model
}

// GetCurrentModel returns the current model
func (h *AdjustmentHistory) GetCurrentModel() string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.currentModel
}

// GetOriginalModel returns the original model
func (h *AdjustmentHistory) GetOriginalModel() string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.originalModel
}

// SetSimplified marks whether task was simplified
func (h *AdjustmentHistory) SetSimplified(simplified bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.simplified = simplified
}

// IsSimplified returns whether the task was simplified
func (h *AdjustmentHistory) IsSimplified() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.simplified
}

// HasModelEscalation returns true if model was escalated at any point
func (h *AdjustmentHistory) HasModelEscalation() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for _, adj := range h.adjustments {
		if adj.Type == AdjustmentModelEscalation {
			return true
		}
	}
	return false
}

// Summary returns a human-readable summary of adjustments
func (h *AdjustmentHistory) Summary() string {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if len(h.adjustments) == 0 {
		return "No adjustments made"
	}

	var parts []string
	for _, adj := range h.adjustments {
		var desc string
		switch adj.Type {
		case AdjustmentModelEscalation:
			desc = fmt.Sprintf("Model: %s → %s", adj.FromValue, adj.ToValue)
		case AdjustmentTaskSimplify:
			desc = fmt.Sprintf("Tasks simplified: %s", adj.Details)
		case AdjustmentContextExpand:
			desc = "Context expanded"
		case AdjustmentPromptRefine:
			desc = "Prompt refined"
		default:
			desc = string(adj.Type)
		}
		parts = append(parts, fmt.Sprintf("[%d] %s", adj.AttemptNum, desc))
	}
	return strings.Join(parts, " | ")
}

// Config holds retry adjustment configuration
type Config struct {
	MaxAdjustments   int  `json:"max_adjustments"`
	MaxRetries       int  `json:"max_retries"`
	EnableEscalation bool `json:"enable_escalation"`
	EnableSimplify   bool `json:"enable_simplify"`
}

// DefaultConfig returns the default retry configuration
func DefaultConfig() Config {
	return Config{
		MaxAdjustments:   DefaultMaxAdjustments,
		MaxRetries:       DefaultMaxRetries,
		EnableEscalation: true,
		EnableSimplify:   true,
	}
}

// Strategy determines what adjustments to make before retry
type Strategy struct {
	mu      sync.RWMutex
	config  Config
	history map[string]*AdjustmentHistory
}

// NewStrategy creates a new retry strategy with default config
func NewStrategy() *Strategy {
	return NewStrategyWithConfig(DefaultConfig())
}

// NewStrategyWithConfig creates a new retry strategy with custom config
func NewStrategyWithConfig(config Config) *Strategy {
	return &Strategy{
		config:  config,
		history: make(map[string]*AdjustmentHistory),
	}
}

// SetConfig updates the strategy configuration
func (s *Strategy) SetConfig(config Config) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.config = config
}

// GetConfig returns the current configuration
func (s *Strategy) GetConfig() Config {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.config
}

// RegisterFeature initializes adjustment history for a feature
func (s *Strategy) RegisterFeature(featureID, initialModel string) *AdjustmentHistory {
	s.mu.Lock()
	defer s.mu.Unlock()

	history := NewAdjustmentHistory(featureID, initialModel)
	s.history[featureID] = history
	return history
}

// GetHistory returns the adjustment history for a feature
func (s *Strategy) GetHistory(featureID string) *AdjustmentHistory {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.history[featureID]
}

// RemoveFeature removes adjustment history for a feature
func (s *Strategy) RemoveFeature(featureID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.history, featureID)
}

// FailureContext provides context about why a feature failed
type FailureContext struct {
	FeatureID      string
	AttemptNum     int
	LastError      string
	TestsFailed    int
	TestsPassed    int
	HasBuildError  bool
	HasTimeout     bool
	TaskCount      int
	CurrentModel   string
	LastModel      string
}

// RetryDecision contains the recommended adjustments for retry
type RetryDecision struct {
	ShouldRetry        bool             `json:"should_retry"`
	ShouldAdjust       bool             `json:"should_adjust"`
	AdjustmentType     AdjustmentType   `json:"adjustment_type,omitempty"`
	NewModel           string           `json:"new_model,omitempty"`
	SimplifiedTasks    []string         `json:"simplified_tasks,omitempty"`
	Reason             AdjustmentReason `json:"reason,omitempty"`
	Details            string           `json:"details,omitempty"`
	RemainingRetries   int              `json:"remaining_retries"`
	RemainingAdjusts   int              `json:"remaining_adjusts"`
}

// DecideRetry analyzes the failure context and returns a retry decision
func (s *Strategy) DecideRetry(ctx FailureContext) RetryDecision {
	s.mu.RLock()
	config := s.config
	history := s.history[ctx.FeatureID]
	s.mu.RUnlock()

	decision := RetryDecision{
		ShouldRetry: ctx.AttemptNum < config.MaxRetries,
	}

	if !decision.ShouldRetry {
		decision.Reason = ReasonMaxAttemptsReached
		decision.Details = fmt.Sprintf("Max retries (%d) reached", config.MaxRetries)
		return decision
	}

	adjustCount := 0
	if history != nil {
		adjustCount = history.Count()
	}

	decision.RemainingRetries = config.MaxRetries - ctx.AttemptNum
	decision.RemainingAdjusts = config.MaxAdjustments - adjustCount

	if decision.RemainingAdjusts <= 0 {
		decision.Details = "Max adjustments reached, retrying without adjustment"
		return decision
	}

	// Determine best adjustment strategy
	adjustment := s.determineAdjustment(ctx, config, history)
	if adjustment.Type != AdjustmentNone {
		decision.ShouldAdjust = true
		decision.AdjustmentType = adjustment.Type
		decision.NewModel = adjustment.ToValue
		decision.Reason = adjustment.Reason
		decision.Details = adjustment.Details
	}

	return decision
}

// determineAdjustment chooses the best adjustment based on failure context
func (s *Strategy) determineAdjustment(ctx FailureContext, config Config, history *AdjustmentHistory) Adjustment {
	adj := Adjustment{
		Type:       AdjustmentNone,
		AttemptNum: ctx.AttemptNum,
	}

	// Priority 1: Model escalation for repeated failures or complex errors
	if config.EnableEscalation && s.shouldEscalateModel(ctx, history) {
		newModel := s.getEscalatedModel(ctx.CurrentModel)
		if newModel != ctx.CurrentModel {
			adj.Type = AdjustmentModelEscalation
			adj.FromValue = ctx.CurrentModel
			adj.ToValue = newModel
			adj.Reason = s.getEscalationReason(ctx)
			adj.Details = s.getEscalationDetails(ctx)
			return adj
		}
	}

	// Priority 2: Task simplification for complex tasks with repeated failures
	if config.EnableSimplify && s.shouldSimplifyTasks(ctx, history) {
		adj.Type = AdjustmentTaskSimplify
		adj.Reason = ReasonComplexTask
		adj.Details = fmt.Sprintf("Simplifying %d tasks to reduce complexity", ctx.TaskCount)
		return adj
	}

	return adj
}

// shouldEscalateModel determines if model escalation is appropriate
func (s *Strategy) shouldEscalateModel(ctx FailureContext, history *AdjustmentHistory) bool {
	// Already at highest model
	if ctx.CurrentModel == "opus" {
		return false
	}

	// Always escalate on build errors with haiku
	if ctx.HasBuildError && ctx.CurrentModel == "haiku" {
		return true
	}

	// Escalate on repeated test failures
	if ctx.TestsFailed > 0 && ctx.AttemptNum >= 2 {
		return true
	}

	// Escalate if previous attempt also failed without model change
	if history != nil && !history.HasModelEscalation() && ctx.AttemptNum >= 2 {
		return true
	}

	return false
}

// shouldSimplifyTasks determines if task simplification is appropriate
func (s *Strategy) shouldSimplifyTasks(ctx FailureContext, history *AdjustmentHistory) bool {
	// Only simplify if we have many tasks and haven't already
	if ctx.TaskCount <= 2 {
		return false
	}

	if history != nil && history.IsSimplified() {
		return false
	}

	// Simplify after model escalation didn't help
	if history != nil && history.HasModelEscalation() && ctx.AttemptNum >= 3 {
		return true
	}

	return false
}

// getEscalatedModel returns the next model tier
func (s *Strategy) getEscalatedModel(current string) string {
	switch strings.ToLower(current) {
	case "haiku":
		return "sonnet"
	case "sonnet":
		return "opus"
	default:
		return current
	}
}

// getEscalationReason determines the reason for escalation
func (s *Strategy) getEscalationReason(ctx FailureContext) AdjustmentReason {
	if ctx.HasBuildError {
		return ReasonCompilationErrors
	}
	if ctx.TestsFailed > 0 {
		return ReasonTestFailures
	}
	if ctx.HasTimeout {
		return ReasonTimeout
	}
	return ReasonRepeatedFailures
}

// getEscalationDetails provides details about escalation
func (s *Strategy) getEscalationDetails(ctx FailureContext) string {
	var details []string
	if ctx.HasBuildError {
		details = append(details, "build errors detected")
	}
	if ctx.TestsFailed > 0 {
		details = append(details, fmt.Sprintf("%d test failures", ctx.TestsFailed))
	}
	if ctx.AttemptNum > 1 {
		details = append(details, fmt.Sprintf("attempt %d", ctx.AttemptNum))
	}
	if len(details) == 0 {
		return "escalating for better capability"
	}
	return strings.Join(details, ", ")
}

// RecordAdjustment records an adjustment that was made
func (s *Strategy) RecordAdjustment(featureID string, adj Adjustment) {
	s.mu.Lock()
	history := s.history[featureID]
	s.mu.Unlock()

	if history == nil {
		history = s.RegisterFeature(featureID, "")
	}

	history.AddAdjustment(adj)

	if adj.Type == AdjustmentModelEscalation {
		history.SetCurrentModel(adj.ToValue)
	}
	if adj.Type == AdjustmentTaskSimplify {
		history.SetSimplified(true)
	}
}

// CanRetry checks if a feature can still be retried
func (s *Strategy) CanRetry(featureID string, attemptNum int) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return attemptNum < s.config.MaxRetries
}

// CanAdjust checks if more adjustments are allowed
func (s *Strategy) CanAdjust(featureID string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	history := s.history[featureID]
	if history == nil {
		return true
	}
	return history.Count() < s.config.MaxAdjustments
}
