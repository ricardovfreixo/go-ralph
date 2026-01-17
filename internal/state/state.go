package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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
	ID          string                `json:"id"`
	Title       string                `json:"title,omitempty"`
	Status      string                `json:"status"`
	StartedAt   *time.Time            `json:"started_at,omitempty"`
	CompletedAt *time.Time            `json:"completed_at,omitempty"`
	Attempts    int                   `json:"attempts"`
	MaxRetries  int                   `json:"max_retries"`
	LastError   string                `json:"last_error,omitempty"`
	Tasks       map[string]*TaskState `json:"tasks"`
	TestResults *TestResultState      `json:"test_results,omitempty"`
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
