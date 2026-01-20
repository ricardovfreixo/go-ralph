package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type Progress struct {
	mu          sync.RWMutex
	path        string
	Version     string                   `json:"version"`
	StartedAt   time.Time                `json:"started_at"`
	UpdatedAt   time.Time                `json:"updated_at"`
	PRDHash     string                   `json:"prd_hash,omitempty"`
	Features    map[string]*FeatureState `json:"features"`
	GlobalState map[string]interface{}   `json:"global_state"`
	Config      ProgressConfig           `json:"config"`
}

type ProgressConfig struct {
	MaxRetries    int `json:"max_retries"`
	MaxConcurrent int `json:"max_concurrent"`
}

type FeatureState struct {
	ID             string                `json:"id"`
	Title          string                `json:"title,omitempty"`
	Status         string                `json:"status"`
	StartedAt      *time.Time            `json:"started_at,omitempty"`
	CompletedAt    *time.Time            `json:"completed_at,omitempty"`
	Attempts       int                   `json:"attempts"`
	MaxRetries     int                   `json:"max_retries"`
	LastError      string                `json:"last_error,omitempty"`
	Tasks          map[string]*TaskState `json:"tasks"`
	TestResults    *TestResultState      `json:"test_results,omitempty"`
	ParentID       string                `json:"parent_id,omitempty"`
	Depth          int                   `json:"depth,omitempty"`
	CurrentModel   string                `json:"current_model,omitempty"`
	ModelSwitches  []ModelSwitchState    `json:"model_switches,omitempty"`
	IsolationLevel string                `json:"isolation_level,omitempty"`
	FailureReason  string                `json:"failure_reason,omitempty"`
	FailedChildren []string              `json:"failed_children,omitempty"`
	Skipped        bool                  `json:"skipped,omitempty"`
	SkipReason     string                `json:"skip_reason,omitempty"`
	Adjustments    []AdjustmentState     `json:"adjustments,omitempty"`
	MaxAdjustments int                   `json:"max_adjustments,omitempty"`
	OriginalModel  string                `json:"original_model,omitempty"`
	Simplified     bool                  `json:"simplified,omitempty"`
	// Token and cost tracking
	InputTokens   int64   `json:"input_tokens,omitempty"`
	OutputTokens  int64   `json:"output_tokens,omitempty"`
	CacheRead     int64   `json:"cache_read,omitempty"`
	CacheWrite    int64   `json:"cache_write,omitempty"`
	EstimatedCost float64 `json:"estimated_cost,omitempty"`
}

type AdjustmentState struct {
	Timestamp  time.Time `json:"timestamp"`
	Type       string    `json:"type"`
	Reason     string    `json:"reason"`
	FromValue  string    `json:"from_value,omitempty"`
	ToValue    string    `json:"to_value,omitempty"`
	Details    string    `json:"details,omitempty"`
	AttemptNum int       `json:"attempt_num"`
}

type ModelSwitchState struct {
	Timestamp time.Time `json:"timestamp"`
	FromModel string    `json:"from_model"`
	ToModel   string    `json:"to_model"`
	Reason    string    `json:"reason"`
	Details   string    `json:"details,omitempty"`
}

type TaskState struct {
	ID        string `json:"id"`
	Completed bool   `json:"completed"`
}

type TestResultState struct {
	Passed  int    `json:"passed"`
	Failed  int    `json:"failed"`
	Skipped int    `json:"skipped"`
	Total   int    `json:"total"`
	Output  string `json:"output,omitempty"`
}

func NewProgress() *Progress {
	return &Progress{
		Version:     "0.2.0",
		StartedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		Features:    make(map[string]*FeatureState),
		GlobalState: make(map[string]interface{}),
		Config: ProgressConfig{
			MaxRetries:    3,
			MaxConcurrent: 3,
		},
	}
}

func LoadProgress(prdPath string) (*Progress, error) {
	dir := filepath.Dir(prdPath)
	progressPath := filepath.Join(dir, "progress.json")
	return LoadProgressFromPath(progressPath)
}

func LoadProgressFromPath(progressPath string) (*Progress, error) {
	dir := filepath.Dir(progressPath)

	// Try .json first, fall back to .md for backwards compatibility
	data, err := os.ReadFile(progressPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Try legacy .md format
			legacyPath := filepath.Join(dir, "progress.md")
			data, err = os.ReadFile(legacyPath)
			if err != nil {
				if os.IsNotExist(err) {
					return nil, fmt.Errorf("progress file not found")
				}
				return nil, fmt.Errorf("failed to read progress file: %w", err)
			}
			progressPath = legacyPath
		} else {
			return nil, fmt.Errorf("failed to read progress file: %w", err)
		}
	}

	var progress Progress
	if err := json.Unmarshal(data, &progress); err != nil {
		return nil, fmt.Errorf("failed to parse progress file: %w", err)
	}

	progress.path = filepath.Join(dir, "progress.json")

	if progress.Features == nil {
		progress.Features = make(map[string]*FeatureState)
	}
	if progress.GlobalState == nil {
		progress.GlobalState = make(map[string]interface{})
	}

	return &progress, nil
}

func (p *Progress) Save() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.path == "" {
		return fmt.Errorf("progress path not set")
	}

	p.UpdatedAt = time.Now()
	p.Version = "0.2.0"

	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal progress: %w", err)
	}

	if err := os.WriteFile(p.path, data, 0644); err != nil {
		return fmt.Errorf("failed to write progress file: %w", err)
	}

	return nil
}

func (p *Progress) SetPath(prdPath string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	dir := filepath.Dir(prdPath)
	p.path = filepath.Join(dir, "progress.json")
}

func (p *Progress) SetPathDirect(path string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.path = path
}

func (p *Progress) GetPath() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.path
}

func (p *Progress) GetFeature(id string) *FeatureState {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.Features[id]
}

func (p *Progress) InitFeature(id string, title string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.Features[id] == nil {
		p.Features[id] = &FeatureState{
			ID:         id,
			Title:      title,
			Status:     "pending",
			Tasks:      make(map[string]*TaskState),
			MaxRetries: p.Config.MaxRetries,
		}
	}
}

func (p *Progress) UpdateFeature(id string, status string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.Features[id] == nil {
		p.Features[id] = &FeatureState{
			ID:         id,
			Status:     status,
			Tasks:      make(map[string]*TaskState),
			MaxRetries: p.Config.MaxRetries,
		}
	}

	p.Features[id].Status = status
	now := time.Now()

	switch status {
	case "running":
		if p.Features[id].StartedAt == nil {
			p.Features[id].StartedAt = &now
		}
		p.Features[id].Attempts++
	case "completed":
		p.Features[id].CompletedAt = &now
		p.Features[id].LastError = ""
	case "failed":
		// Don't clear error on failure
	}

	p.UpdatedAt = now
}

func (p *Progress) SetFeatureError(id string, err string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.Features[id] == nil {
		p.Features[id] = &FeatureState{
			ID:    id,
			Tasks: make(map[string]*TaskState),
		}
	}
	p.Features[id].LastError = err
	p.Features[id].Status = "failed"
	p.UpdatedAt = time.Now()
}

func (p *Progress) SetTestResults(id string, passed, failed, skipped int, output string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.Features[id] == nil {
		return
	}

	p.Features[id].TestResults = &TestResultState{
		Passed:  passed,
		Failed:  failed,
		Skipped: skipped,
		Total:   passed + failed + skipped,
		Output:  output,
	}
	p.UpdatedAt = time.Now()
}

func (p *Progress) CanRetry(id string) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.Features[id] == nil {
		return true
	}

	maxRetries := p.Features[id].MaxRetries
	if maxRetries == 0 {
		maxRetries = p.Config.MaxRetries
	}

	return p.Features[id].Attempts < maxRetries
}

func (p *Progress) GetAttempts(id string) int {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.Features[id] == nil {
		return 0
	}
	return p.Features[id].Attempts
}

func (p *Progress) ResetFeature(id string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.Features[id] != nil {
		p.Features[id].Status = "pending"
		p.Features[id].StartedAt = nil
		p.Features[id].CompletedAt = nil
		p.Features[id].Attempts = 0
		p.Features[id].LastError = ""
		p.Features[id].TestResults = nil
	}
	p.UpdatedAt = time.Now()
}

func (p *Progress) ResetAll() {
	p.mu.Lock()
	defer p.mu.Unlock()

	for _, f := range p.Features {
		f.Status = "pending"
		f.StartedAt = nil
		f.CompletedAt = nil
		f.Attempts = 0
		f.LastError = ""
		f.TestResults = nil
	}
	p.UpdatedAt = time.Now()
}

func (p *Progress) GetPendingFeatures() []string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	var pending []string
	for id, feature := range p.Features {
		if feature.Status == "pending" || feature.Status == "" {
			pending = append(pending, id)
		}
	}
	return pending
}

func (p *Progress) GetRunningFeatures() []string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	var running []string
	for id, feature := range p.Features {
		if feature.Status == "running" {
			running = append(running, id)
		}
	}
	return running
}

func (p *Progress) GetFailedFeatures() []string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	var failed []string
	for id, feature := range p.Features {
		if feature.Status == "failed" {
			failed = append(failed, id)
		}
	}
	return failed
}

func (p *Progress) GetRetryableFeatures() []string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	var retryable []string
	for id, feature := range p.Features {
		if feature.Status == "failed" {
			maxRetries := feature.MaxRetries
			if maxRetries == 0 {
				maxRetries = p.Config.MaxRetries
			}
			if feature.Attempts < maxRetries {
				retryable = append(retryable, id)
			}
		}
	}
	return retryable
}

func (p *Progress) AllCompleted() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if len(p.Features) == 0 {
		return false
	}

	for _, feature := range p.Features {
		if feature.Status != "completed" {
			return false
		}
	}
	return true
}

func (p *Progress) HasFailures() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()

	for _, feature := range p.Features {
		if feature.Status == "failed" {
			return true
		}
	}
	return false
}

func (p *Progress) GetSummary() (total, completed, running, failed, pending int) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	total = len(p.Features)
	for _, feature := range p.Features {
		switch feature.Status {
		case "completed":
			completed++
		case "running":
			running++
		case "failed":
			failed++
		default:
			pending++
		}
	}
	return
}

func (p *Progress) SetConfig(maxRetries, maxConcurrent int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.Config.MaxRetries = maxRetries
	p.Config.MaxConcurrent = maxConcurrent
}

func (p *Progress) SetFeatureParent(id string, parentID string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.Features[id] == nil {
		p.Features[id] = &FeatureState{
			ID:    id,
			Tasks: make(map[string]*TaskState),
		}
	}
	p.Features[id].ParentID = parentID

	if parent := p.Features[parentID]; parent != nil {
		p.Features[id].Depth = parent.Depth + 1
	}
	p.UpdatedAt = time.Now()
}

func (p *Progress) GetFeatureParent(id string) string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if f := p.Features[id]; f != nil {
		return f.ParentID
	}
	return ""
}

func (p *Progress) GetChildFeatures(parentID string) []string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	var children []string
	for id, feature := range p.Features {
		if feature.ParentID == parentID {
			children = append(children, id)
		}
	}
	return children
}

func (p *Progress) SetCurrentModel(id string, model string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.Features[id] == nil {
		p.Features[id] = &FeatureState{
			ID:    id,
			Tasks: make(map[string]*TaskState),
		}
	}
	p.Features[id].CurrentModel = model
	p.UpdatedAt = time.Now()
}

func (p *Progress) GetCurrentModel(id string) string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if f := p.Features[id]; f != nil {
		return f.CurrentModel
	}
	return ""
}

func (p *Progress) AddModelSwitch(id string, fromModel, toModel, reason, details string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.Features[id] == nil {
		p.Features[id] = &FeatureState{
			ID:    id,
			Tasks: make(map[string]*TaskState),
		}
	}

	sw := ModelSwitchState{
		Timestamp: time.Now(),
		FromModel: fromModel,
		ToModel:   toModel,
		Reason:    reason,
		Details:   details,
	}
	p.Features[id].ModelSwitches = append(p.Features[id].ModelSwitches, sw)
	p.Features[id].CurrentModel = toModel
	p.UpdatedAt = time.Now()
}

func (p *Progress) GetModelSwitches(id string) []ModelSwitchState {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if f := p.Features[id]; f != nil {
		result := make([]ModelSwitchState, len(f.ModelSwitches))
		copy(result, f.ModelSwitches)
		return result
	}
	return nil
}

// SetIsolationLevel sets the isolation level for a feature
func (p *Progress) SetIsolationLevel(id string, level string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.Features[id] == nil {
		p.Features[id] = &FeatureState{
			ID:    id,
			Tasks: make(map[string]*TaskState),
		}
	}
	p.Features[id].IsolationLevel = level
	p.UpdatedAt = time.Now()
}

// GetIsolationLevel returns the isolation level for a feature
func (p *Progress) GetIsolationLevel(id string) string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if f := p.Features[id]; f != nil {
		return f.IsolationLevel
	}
	return ""
}

// SetFailureReason sets the failure reason for a feature
func (p *Progress) SetFailureReason(id string, reason string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.Features[id] == nil {
		p.Features[id] = &FeatureState{
			ID:    id,
			Tasks: make(map[string]*TaskState),
		}
	}
	p.Features[id].FailureReason = reason
	p.UpdatedAt = time.Now()
}

// GetFailureReason returns the failure reason for a feature
func (p *Progress) GetFailureReason(id string) string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if f := p.Features[id]; f != nil {
		return f.FailureReason
	}
	return ""
}

// AddFailedChild records a failed child for a parent feature
func (p *Progress) AddFailedChild(parentID, childID string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.Features[parentID] == nil {
		p.Features[parentID] = &FeatureState{
			ID:    parentID,
			Tasks: make(map[string]*TaskState),
		}
	}
	for _, id := range p.Features[parentID].FailedChildren {
		if id == childID {
			return // Already recorded
		}
	}
	p.Features[parentID].FailedChildren = append(p.Features[parentID].FailedChildren, childID)
	p.UpdatedAt = time.Now()
}

// GetFailedChildren returns the list of failed children for a feature
func (p *Progress) GetFailedChildren(id string) []string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if f := p.Features[id]; f != nil {
		result := make([]string, len(f.FailedChildren))
		copy(result, f.FailedChildren)
		return result
	}
	return nil
}

// HasFailedChildren returns true if a feature has any failed children
func (p *Progress) HasFailedChildren(id string) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if f := p.Features[id]; f != nil {
		return len(f.FailedChildren) > 0
	}
	return false
}

// SkipFeature marks a feature as skipped with a reason
func (p *Progress) SkipFeature(id string, reason string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.Features[id] == nil {
		p.Features[id] = &FeatureState{
			ID:    id,
			Tasks: make(map[string]*TaskState),
		}
	}
	p.Features[id].Status = "skipped"
	p.Features[id].Skipped = true
	p.Features[id].SkipReason = reason
	now := time.Now()
	p.Features[id].CompletedAt = &now
	p.UpdatedAt = time.Now()
}

// IsSkipped returns true if a feature was skipped
func (p *Progress) IsSkipped(id string) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if f := p.Features[id]; f != nil {
		return f.Skipped
	}
	return false
}

// GetSkipReason returns the skip reason for a feature
func (p *Progress) GetSkipReason(id string) string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if f := p.Features[id]; f != nil {
		return f.SkipReason
	}
	return ""
}

// ClearFailure clears the failure state for a feature (used when retrying)
func (p *Progress) ClearFailure(id string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if f := p.Features[id]; f != nil {
		f.LastError = ""
		f.FailureReason = ""
		f.FailedChildren = nil
		f.Skipped = false
		f.SkipReason = ""
	}
	p.UpdatedAt = time.Now()
}

// CanChildRetry returns true if a child feature can be retried
func (p *Progress) CanChildRetry(childID, parentID string) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()

	child := p.Features[childID]
	if child == nil {
		return true
	}

	maxRetries := child.MaxRetries
	if maxRetries == 0 {
		maxRetries = p.Config.MaxRetries
	}

	return child.Attempts < maxRetries
}

// AddAdjustment records an adjustment made during retry
func (p *Progress) AddAdjustment(id string, adj AdjustmentState) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.Features[id] == nil {
		p.Features[id] = &FeatureState{
			ID:    id,
			Tasks: make(map[string]*TaskState),
		}
	}

	adj.Timestamp = time.Now()
	p.Features[id].Adjustments = append(p.Features[id].Adjustments, adj)
	p.UpdatedAt = time.Now()
}

// GetAdjustments returns the adjustments made for a feature
func (p *Progress) GetAdjustments(id string) []AdjustmentState {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if f := p.Features[id]; f != nil {
		result := make([]AdjustmentState, len(f.Adjustments))
		copy(result, f.Adjustments)
		return result
	}
	return nil
}

// GetAdjustmentCount returns the number of adjustments for a feature
func (p *Progress) GetAdjustmentCount(id string) int {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if f := p.Features[id]; f != nil {
		return len(f.Adjustments)
	}
	return 0
}

// SetMaxAdjustments sets the maximum adjustments allowed for a feature
func (p *Progress) SetMaxAdjustments(id string, max int) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.Features[id] == nil {
		p.Features[id] = &FeatureState{
			ID:    id,
			Tasks: make(map[string]*TaskState),
		}
	}
	p.Features[id].MaxAdjustments = max
	p.UpdatedAt = time.Now()
}

// GetMaxAdjustments returns the max adjustments for a feature (default 3)
func (p *Progress) GetMaxAdjustments(id string) int {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if f := p.Features[id]; f != nil && f.MaxAdjustments > 0 {
		return f.MaxAdjustments
	}
	return 3 // default
}

// CanAdjust returns true if more adjustments are allowed for a feature
func (p *Progress) CanAdjust(id string) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()

	f := p.Features[id]
	if f == nil {
		return true
	}

	maxAdj := f.MaxAdjustments
	if maxAdj == 0 {
		maxAdj = 3 // default
	}

	return len(f.Adjustments) < maxAdj
}

// SetOriginalModel sets the original model before any adjustments
func (p *Progress) SetOriginalModel(id, model string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.Features[id] == nil {
		p.Features[id] = &FeatureState{
			ID:    id,
			Tasks: make(map[string]*TaskState),
		}
	}
	if p.Features[id].OriginalModel == "" {
		p.Features[id].OriginalModel = model
	}
	p.UpdatedAt = time.Now()
}

// GetOriginalModel returns the original model for a feature
func (p *Progress) GetOriginalModel(id string) string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if f := p.Features[id]; f != nil {
		return f.OriginalModel
	}
	return ""
}

// SetSimplified marks that a feature's tasks were simplified
func (p *Progress) SetSimplified(id string, simplified bool) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.Features[id] == nil {
		p.Features[id] = &FeatureState{
			ID:    id,
			Tasks: make(map[string]*TaskState),
		}
	}
	p.Features[id].Simplified = simplified
	p.UpdatedAt = time.Now()
}

// IsSimplified returns whether a feature's tasks were simplified
func (p *Progress) IsSimplified(id string) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if f := p.Features[id]; f != nil {
		return f.Simplified
	}
	return false
}

// LastAdjustment returns the most recent adjustment for a feature
func (p *Progress) LastAdjustment(id string) *AdjustmentState {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if f := p.Features[id]; f != nil && len(f.Adjustments) > 0 {
		adj := f.Adjustments[len(f.Adjustments)-1]
		return &adj
	}
	return nil
}

// HasModelEscalation returns true if model was escalated during retries
func (p *Progress) HasModelEscalation(id string) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if f := p.Features[id]; f != nil {
		for _, adj := range f.Adjustments {
			if adj.Type == "model_escalation" {
				return true
			}
		}
	}
	return false
}

// GetAdjustmentSummary returns a summary string of adjustments
func (p *Progress) GetAdjustmentSummary(id string) string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	f := p.Features[id]
	if f == nil || len(f.Adjustments) == 0 {
		return ""
	}

	var parts []string
	for _, adj := range f.Adjustments {
		switch adj.Type {
		case "model_escalation":
			parts = append(parts, fmt.Sprintf("[%d] Model: %s→%s", adj.AttemptNum, adj.FromValue, adj.ToValue))
		case "task_simplify":
			parts = append(parts, fmt.Sprintf("[%d] Simplified", adj.AttemptNum))
		default:
			parts = append(parts, fmt.Sprintf("[%d] %s", adj.AttemptNum, adj.Type))
		}
	}
	return strings.Join(parts, " | ")
}

// SetFeatureUsage saves token usage and cost for a feature
func (p *Progress) SetFeatureUsage(id string, inputTokens, outputTokens, cacheRead, cacheWrite int64, cost float64) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if f, ok := p.Features[id]; ok {
		f.InputTokens = inputTokens
		f.OutputTokens = outputTokens
		f.CacheRead = cacheRead
		f.CacheWrite = cacheWrite
		f.EstimatedCost = cost
	}
}

// GetTotalTokens returns aggregated token counts across all features
func (p *Progress) GetTotalTokens() (input, output, cacheRead, cacheWrite int64) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	for _, f := range p.Features {
		input += f.InputTokens
		output += f.OutputTokens
		cacheRead += f.CacheRead
		cacheWrite += f.CacheWrite
	}
	return
}

// GetTotalCost returns aggregated cost across all features
func (p *Progress) GetTotalCost() float64 {
	p.mu.RLock()
	defer p.mu.RUnlock()

	var total float64
	for _, f := range p.Features {
		total += f.EstimatedCost
	}
	return total
}

// GetTotalElapsed returns total time spent on completed features
func (p *Progress) GetTotalElapsed() time.Duration {
	p.mu.RLock()
	defer p.mu.RUnlock()

	var total time.Duration
	for _, f := range p.Features {
		if f.StartedAt != nil && f.CompletedAt != nil {
			total += f.CompletedAt.Sub(*f.StartedAt)
		}
	}
	return total
}

// GetFeatureElapsed returns the elapsed time for a feature (completed or running)
func (p *Progress) GetFeatureElapsed(id string) time.Duration {
	p.mu.RLock()
	defer p.mu.RUnlock()

	f, ok := p.Features[id]
	if !ok || f.StartedAt == nil {
		return 0
	}

	if f.CompletedAt != nil {
		return f.CompletedAt.Sub(*f.StartedAt)
	}
	return time.Since(*f.StartedAt)
}
