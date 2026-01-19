package rlm

import (
	"sync"
	"time"
)

const (
	DefaultMaxDepth      = 5
	DefaultContextBudget = 100000
	MinContextBudget     = 10000 // Minimum budget at any depth
)

// IsolationLevel determines how child failures affect parent features
type IsolationLevel string

const (
	// IsolationStrict - parent fails if any child fails (no isolation)
	IsolationStrict IsolationLevel = "strict"
	// IsolationLenient - parent continues even if children fail (default)
	IsolationLenient IsolationLevel = "lenient"
)

// DefaultIsolationLevel is the default isolation behavior (children isolated from parent)
const DefaultIsolationLevel = IsolationLenient

// FailureInfo captures details about a feature failure
type FailureInfo struct {
	Timestamp   time.Time `json:"timestamp"`
	Reason      string    `json:"reason"`
	Error       string    `json:"error,omitempty"`
	Recoverable bool      `json:"recoverable"`
	RetryCount  int       `json:"retry_count"`
	MaxRetries  int       `json:"max_retries"`
}

// NewFailureInfo creates a new failure info with the given reason and error
func NewFailureInfo(reason, errMsg string) *FailureInfo {
	return &FailureInfo{
		Timestamp:   time.Now(),
		Reason:      reason,
		Error:       errMsg,
		Recoverable: true,
		RetryCount:  0,
		MaxRetries:  3,
	}
}

// CanRetry returns true if the failure can be retried
func (f *FailureInfo) CanRetry() bool {
	return f.Recoverable && f.RetryCount < f.MaxRetries
}

// IncrementRetry increments the retry counter
func (f *FailureInfo) IncrementRetry() {
	f.RetryCount++
}

// ChildFailureAction represents what a parent should do when a child fails
type ChildFailureAction string

const (
	// ChildFailureRetry - retry the child with same or different parameters
	ChildFailureRetry ChildFailureAction = "retry"
	// ChildFailureSkip - skip the failed child and continue
	ChildFailureSkip ChildFailureAction = "skip"
	// ChildFailureAbort - abort the parent (strict isolation)
	ChildFailureAbort ChildFailureAction = "abort"
	// ChildFailureHandle - parent received failure info and will decide
	ChildFailureHandle ChildFailureAction = "handle"
)

// ChildFailureResult represents the outcome of a child failure
type ChildFailureResult struct {
	ChildID       string             `json:"child_id"`
	ChildTitle    string             `json:"child_title"`
	ParentID      string             `json:"parent_id"`
	FailureInfo   *FailureInfo       `json:"failure_info"`
	Action        ChildFailureAction `json:"action"`
	RetryParams   *SpawnRequest      `json:"retry_params,omitempty"`
	SkipReason    string             `json:"skip_reason,omitempty"`
	ParentContext string             `json:"parent_context,omitempty"`
}

// RecursiveFeature extends the base feature with recursive capabilities
type RecursiveFeature struct {
	mu sync.RWMutex

	ID            string `json:"id"`
	Title         string `json:"title"`
	ParentID      string `json:"parent_id,omitempty"`
	Depth         int    `json:"depth"`
	MaxDepth      int    `json:"max_depth"`
	ContextBudget int64  `json:"context_budget"`
	ContextUsed   int64  `json:"context_used,omitempty"`

	Status      string     `json:"status"`
	StartedAt   *time.Time `json:"started_at,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`

	Tasks       []RecursiveTask     `json:"tasks,omitempty"`
	SubFeatures []*RecursiveFeature `json:"sub_features,omitempty"`

	TokenUsage *TokenUsage `json:"token_usage,omitempty"`
	Actions    []Action    `json:"actions,omitempty"`

	Model         string `json:"model,omitempty"`
	ExecutionMode string `json:"execution_mode,omitempty"`

	// Fault isolation fields
	IsolationLevel IsolationLevel `json:"isolation_level,omitempty"`
	FailureInfo    *FailureInfo   `json:"failure_info,omitempty"`
	FailedChildren []string       `json:"failed_children,omitempty"`
}

// RecursiveTask represents a task within a recursive feature
type RecursiveTask struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	Completed   bool   `json:"completed"`
}

// TokenUsage tracks token consumption for a feature
type TokenUsage struct {
	mu sync.RWMutex

	InputTokens      int64 `json:"input_tokens"`
	OutputTokens     int64 `json:"output_tokens"`
	CacheReadTokens  int64 `json:"cache_read_tokens,omitempty"`
	CacheWriteTokens int64 `json:"cache_write_tokens,omitempty"`
	TotalTokens      int64 `json:"total_tokens"`

	CostUSD float64 `json:"cost_usd,omitempty"`
}

// Action represents a significant action extracted from stream output
type Action struct {
	Timestamp   time.Time `json:"timestamp"`
	Type        string    `json:"type"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	Details     string    `json:"details,omitempty"`
}

// SpawnRequest represents a request to spawn a sub-feature
type SpawnRequest struct {
	Title       string   `json:"title"`
	Tasks       []string `json:"tasks,omitempty"`
	Model       string   `json:"model,omitempty"`
	MaxDepth    int      `json:"max_depth,omitempty"`
	Description string   `json:"description,omitempty"`
}

// SpawnResult contains the outcome of a spawned sub-feature
type SpawnResult struct {
	FeatureID  string      `json:"feature_id"`
	Title      string      `json:"title"`
	Status     string      `json:"status"`
	Summary    string      `json:"summary,omitempty"`
	TokenUsage *TokenUsage `json:"token_usage,omitempty"`
	Error      string      `json:"error,omitempty"`
}

// NewRecursiveFeature creates a new recursive feature
func NewRecursiveFeature(id, title string) *RecursiveFeature {
	return &RecursiveFeature{
		ID:            id,
		Title:         title,
		Depth:         0,
		MaxDepth:      DefaultMaxDepth,
		ContextBudget: int64(DefaultContextBudget),
		ContextUsed:   0,
		Status:        "pending",
		TokenUsage:    NewTokenUsage(),
		Actions:       make([]Action, 0),
		SubFeatures:   make([]*RecursiveFeature, 0),
	}
}

// NewChildFeature creates a child feature from a parent
func (f *RecursiveFeature) NewChildFeature(id, title string) (*RecursiveFeature, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	if f.Depth >= f.MaxDepth {
		return nil, ErrMaxDepthExceeded
	}

	childDepth := f.Depth + 1
	// Calculate child context budget using depth formula: base / (depth + 1)
	childBudget := CalculateContextBudgetForDepth(f.ContextBudget, childDepth)

	child := &RecursiveFeature{
		ID:            id,
		Title:         title,
		ParentID:      f.ID,
		Depth:         childDepth,
		MaxDepth:      f.MaxDepth,
		ContextBudget: childBudget,
		ContextUsed:   0,
		Status:        "pending",
		TokenUsage:    NewTokenUsage(),
		Actions:       make([]Action, 0),
		SubFeatures:   make([]*RecursiveFeature, 0),
		Model:         f.Model,
		ExecutionMode: f.ExecutionMode,
	}

	return child, nil
}

// AddSubFeature adds a sub-feature to this feature
func (f *RecursiveFeature) AddSubFeature(child *RecursiveFeature) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.SubFeatures = append(f.SubFeatures, child)
}

// GetSubFeatures returns all sub-features
func (f *RecursiveFeature) GetSubFeatures() []*RecursiveFeature {
	f.mu.RLock()
	defer f.mu.RUnlock()
	result := make([]*RecursiveFeature, len(f.SubFeatures))
	copy(result, f.SubFeatures)
	return result
}

// IsRoot returns true if this is a root feature (no parent)
func (f *RecursiveFeature) IsRoot() bool {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.ParentID == ""
}

// CanSpawn returns true if this feature can spawn children
func (f *RecursiveFeature) CanSpawn() bool {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.Depth < f.MaxDepth
}

// GetTotalTokenUsage returns the aggregate token usage including sub-features
func (f *RecursiveFeature) GetTotalTokenUsage() *TokenUsage {
	f.mu.RLock()
	defer f.mu.RUnlock()

	total := NewTokenUsage()

	if f.TokenUsage != nil {
		total.Add(f.TokenUsage)
	}

	for _, sub := range f.SubFeatures {
		subTotal := sub.GetTotalTokenUsage()
		total.Add(subTotal)
	}

	return total
}

// AddAction records an action for this feature
func (f *RecursiveFeature) AddAction(action Action) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Actions = append(f.Actions, action)
}

// GetActions returns all recorded actions
func (f *RecursiveFeature) GetActions() []Action {
	f.mu.RLock()
	defer f.mu.RUnlock()
	result := make([]Action, len(f.Actions))
	copy(result, f.Actions)
	return result
}

// SetStatus updates the feature status
func (f *RecursiveFeature) SetStatus(status string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Status = status

	now := time.Now()
	switch status {
	case "running":
		if f.StartedAt == nil {
			f.StartedAt = &now
		}
	case "completed", "failed":
		f.CompletedAt = &now
	}
}

// GetStatus returns the current status
func (f *RecursiveFeature) GetStatus() string {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.Status
}

// GetContextBudget returns the context budget for this feature
func (f *RecursiveFeature) GetContextBudget() int64 {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.ContextBudget
}

// SetContextBudget sets a custom context budget (for overrides)
func (f *RecursiveFeature) SetContextBudget(budget int64) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if budget > 0 {
		f.ContextBudget = budget
	}
}

// GetContextUsed returns the context tokens used
func (f *RecursiveFeature) GetContextUsed() int64 {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.ContextUsed
}

// SetContextUsed sets the context tokens used
func (f *RecursiveFeature) SetContextUsed(used int64) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.ContextUsed = used
}

// AddContextUsage adds to the context usage
func (f *RecursiveFeature) AddContextUsage(tokens int64) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.ContextUsed += tokens
}

// GetContextRemaining returns remaining context budget
func (f *RecursiveFeature) GetContextRemaining() int64 {
	f.mu.RLock()
	defer f.mu.RUnlock()
	remaining := f.ContextBudget - f.ContextUsed
	if remaining < 0 {
		return 0
	}
	return remaining
}

// GetContextUsagePercent returns context usage as percentage (0.0 to 1.0)
func (f *RecursiveFeature) GetContextUsagePercent() float64 {
	f.mu.RLock()
	defer f.mu.RUnlock()
	if f.ContextBudget == 0 {
		return 0
	}
	return float64(f.ContextUsed) / float64(f.ContextBudget)
}

// NeedsContextSummarization returns true if context should be summarized (>80% used)
func (f *RecursiveFeature) NeedsContextSummarization() bool {
	return f.GetContextUsagePercent() >= 0.8
}

// IsContextOverBudget returns true if context usage exceeds budget
func (f *RecursiveFeature) IsContextOverBudget() bool {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.ContextUsed > f.ContextBudget
}

// CalculateChildContextBudget returns the context budget for a child at depth+1
func (f *RecursiveFeature) CalculateChildContextBudget() int64 {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return CalculateContextBudgetForDepth(f.ContextBudget, f.Depth+1)
}

// GetIsolationLevel returns the isolation level for this feature
func (f *RecursiveFeature) GetIsolationLevel() IsolationLevel {
	f.mu.RLock()
	defer f.mu.RUnlock()
	if f.IsolationLevel == "" {
		return DefaultIsolationLevel
	}
	return f.IsolationLevel
}

// SetIsolationLevel sets the isolation level for this feature
func (f *RecursiveFeature) SetIsolationLevel(level IsolationLevel) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.IsolationLevel = level
}

// GetFailureInfo returns the failure info for this feature
func (f *RecursiveFeature) GetFailureInfo() *FailureInfo {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.FailureInfo
}

// SetFailureInfo sets failure info for this feature
func (f *RecursiveFeature) SetFailureInfo(info *FailureInfo) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.FailureInfo = info
}

// RecordFailure records a failure with reason and error message
func (f *RecursiveFeature) RecordFailure(reason, errMsg string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.FailureInfo = NewFailureInfo(reason, errMsg)
	f.Status = "failed"
	now := time.Now()
	f.CompletedAt = &now
}

// AddFailedChild records a child feature that failed
func (f *RecursiveFeature) AddFailedChild(childID string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, id := range f.FailedChildren {
		if id == childID {
			return // Already recorded
		}
	}
	f.FailedChildren = append(f.FailedChildren, childID)
}

// GetFailedChildren returns the list of failed child feature IDs
func (f *RecursiveFeature) GetFailedChildren() []string {
	f.mu.RLock()
	defer f.mu.RUnlock()
	result := make([]string, len(f.FailedChildren))
	copy(result, f.FailedChildren)
	return result
}

// HasFailedChildren returns true if any children have failed
func (f *RecursiveFeature) HasFailedChildren() bool {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return len(f.FailedChildren) > 0
}

// ShouldFailOnChildFailure returns true if this feature should fail when a child fails
func (f *RecursiveFeature) ShouldFailOnChildFailure() bool {
	return f.GetIsolationLevel() == IsolationStrict
}

// ClearFailure clears the failure info (used when retrying)
func (f *RecursiveFeature) ClearFailure() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.FailureInfo = nil
	f.FailedChildren = nil
}

// CalculateContextBudgetForDepth calculates context budget for a given depth
// Formula: base_budget / (depth + 1), with minimum floor
func CalculateContextBudgetForDepth(baseBudget int64, depth int) int64 {
	if depth < 0 {
		depth = 0
	}

	// Formula: base / (depth + 1)
	budget := baseBudget / int64(depth+1)

	// Ensure minimum budget
	if budget < MinContextBudget {
		budget = MinContextBudget
	}

	return budget
}

// NewTokenUsage creates a new token usage tracker
func NewTokenUsage() *TokenUsage {
	return &TokenUsage{}
}

// Add adds another TokenUsage to this one
func (t *TokenUsage) Add(other *TokenUsage) {
	if other == nil {
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	other.mu.RLock()
	defer other.mu.RUnlock()

	t.InputTokens += other.InputTokens
	t.OutputTokens += other.OutputTokens
	t.CacheReadTokens += other.CacheReadTokens
	t.CacheWriteTokens += other.CacheWriteTokens
	t.TotalTokens += other.TotalTokens
	t.CostUSD += other.CostUSD
}

// Update updates token usage from stream data
func (t *TokenUsage) Update(input, output, cacheRead, cacheWrite int64, cost float64) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.InputTokens += input
	t.OutputTokens += output
	t.CacheReadTokens += cacheRead
	t.CacheWriteTokens += cacheWrite
	t.TotalTokens = t.InputTokens + t.OutputTokens
	t.CostUSD += cost
}

// GetSnapshot returns a copy of current token usage
func (t *TokenUsage) GetSnapshot() TokenUsage {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return TokenUsage{
		InputTokens:      t.InputTokens,
		OutputTokens:     t.OutputTokens,
		CacheReadTokens:  t.CacheReadTokens,
		CacheWriteTokens: t.CacheWriteTokens,
		TotalTokens:      t.TotalTokens,
		CostUSD:          t.CostUSD,
	}
}
