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
	mu           sync.RWMutex
	path         string
	StartedAt    time.Time              `json:"started_at"`
	UpdatedAt    time.Time              `json:"updated_at"`
	Features     map[string]*FeatureState `json:"features"`
	GlobalState  map[string]interface{} `json:"global_state"`
}

type FeatureState struct {
	ID          string    `json:"id"`
	Status      string    `json:"status"` // pending, running, completed, failed, paused
	StartedAt   *time.Time `json:"started_at,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	Attempts    int       `json:"attempts"`
	LastError   string    `json:"last_error,omitempty"`
	Tasks       map[string]*TaskState `json:"tasks"`
	Output      []OutputEntry `json:"output,omitempty"`
}

type TaskState struct {
	ID        string `json:"id"`
	Completed bool   `json:"completed"`
}

type OutputEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Type      string    `json:"type"` // info, error, assistant, user, tool
	Content   string    `json:"content"`
}

func NewProgress() *Progress {
	return &Progress{
		StartedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		Features:    make(map[string]*FeatureState),
		GlobalState: make(map[string]interface{}),
	}
}

func LoadProgress(prdPath string) (*Progress, error) {
	dir := filepath.Dir(prdPath)
	progressPath := filepath.Join(dir, "progress.md")

	data, err := os.ReadFile(progressPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("progress file not found")
		}
		return nil, fmt.Errorf("failed to read progress file: %w", err)
	}

	var progress Progress
	if err := json.Unmarshal(data, &progress); err != nil {
		return nil, fmt.Errorf("failed to parse progress file: %w", err)
	}

	progress.path = progressPath
	return &progress, nil
}

func (p *Progress) Save() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.path == "" {
		return fmt.Errorf("progress path not set")
	}

	p.UpdatedAt = time.Now()

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
	dir := filepath.Dir(prdPath)
	p.path = filepath.Join(dir, "progress.md")
}

func (p *Progress) GetFeature(id string) *FeatureState {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.Features[id]
}

func (p *Progress) UpdateFeature(id string, status string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.Features[id] == nil {
		p.Features[id] = &FeatureState{
			ID:     id,
			Tasks:  make(map[string]*TaskState),
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
	}

	p.UpdatedAt = now
}

func (p *Progress) SetFeatureError(id string, err string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.Features[id] != nil {
		p.Features[id].LastError = err
		p.Features[id].Status = "failed"
	}
}

func (p *Progress) AddOutput(featureID string, entryType string, content string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.Features[featureID] == nil {
		return
	}

	p.Features[featureID].Output = append(p.Features[featureID].Output, OutputEntry{
		Timestamp: time.Now(),
		Type:      entryType,
		Content:   content,
	})
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

func (p *Progress) AllCompleted() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()

	for _, feature := range p.Features {
		if feature.Status != "completed" {
			return false
		}
	}
	return len(p.Features) > 0
}
