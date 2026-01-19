package rlm

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sync"
)

// Manager handles recursive feature tracking and sub-feature spawning
type Manager struct {
	mu       sync.RWMutex
	features map[string]*RecursiveFeature
	trackers map[string]*Tracker

	maxDepth      int
	contextBudget int64
}

// NewManager creates a new RLM manager
func NewManager() *Manager {
	return &Manager{
		features:      make(map[string]*RecursiveFeature),
		trackers:      make(map[string]*Tracker),
		maxDepth:      DefaultMaxDepth,
		contextBudget: DefaultContextBudget,
	}
}

// NewManagerWithConfig creates a manager with custom config
func NewManagerWithConfig(maxDepth int, contextBudget int64) *Manager {
	if maxDepth <= 0 {
		maxDepth = DefaultMaxDepth
	}
	if contextBudget <= 0 {
		contextBudget = DefaultContextBudget
	}

	return &Manager{
		features:      make(map[string]*RecursiveFeature),
		trackers:      make(map[string]*Tracker),
		maxDepth:      maxDepth,
		contextBudget: contextBudget,
	}
}

// RegisterFeature registers a root-level feature
func (m *Manager) RegisterFeature(id, title string) *RecursiveFeature {
	m.mu.Lock()
	defer m.mu.Unlock()

	feature := NewRecursiveFeature(id, title)
	feature.MaxDepth = m.maxDepth
	feature.ContextBudget = m.contextBudget

	m.features[id] = feature
	m.trackers[id] = NewTracker(feature)

	return feature
}

// GetFeature retrieves a feature by ID
func (m *Manager) GetFeature(id string) *RecursiveFeature {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.features[id]
}

// GetTracker retrieves the tracker for a feature
func (m *Manager) GetTracker(id string) *Tracker {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.trackers[id]
}

// ProcessOutput processes a line of stream-json output for a feature
func (m *Manager) ProcessOutput(featureID string, line string) (*SpawnRequest, error) {
	tracker := m.GetTracker(featureID)
	if tracker == nil {
		return nil, ErrFeatureNotFound
	}

	return tracker.ProcessLine(line)
}

// SpawnSubFeature creates a new sub-feature from a spawn request
func (m *Manager) SpawnSubFeature(parentID string, req *SpawnRequest) (*RecursiveFeature, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	parent, ok := m.features[parentID]
	if !ok {
		return nil, ErrFeatureNotFound
	}

	if parent.GetStatus() != "running" {
		return nil, ErrParentNotRunning
	}

	childID := generateID(fmt.Sprintf("%s:%s", parentID, req.Title))

	child, err := parent.NewChildFeature(childID, req.Title)
	if err != nil {
		return nil, err
	}

	for _, taskDesc := range req.Tasks {
		child.Tasks = append(child.Tasks, RecursiveTask{
			ID:          generateID(taskDesc),
			Description: taskDesc,
			Completed:   false,
		})
	}

	if req.Model != "" {
		child.Model = req.Model
	}

	if req.MaxDepth > 0 && req.MaxDepth < child.MaxDepth {
		child.MaxDepth = req.MaxDepth
	}

	parent.AddSubFeature(child)
	m.features[childID] = child
	m.trackers[childID] = NewTracker(child)

	return child, nil
}

// GetSubFeatures returns all sub-features of a parent
func (m *Manager) GetSubFeatures(parentID string) []*RecursiveFeature {
	feature := m.GetFeature(parentID)
	if feature == nil {
		return nil
	}
	return feature.GetSubFeatures()
}

// GetFeatureTree returns a feature and all its descendants as a flat list
func (m *Manager) GetFeatureTree(rootID string) []*RecursiveFeature {
	root := m.GetFeature(rootID)
	if root == nil {
		return nil
	}

	var result []*RecursiveFeature
	var collect func(f *RecursiveFeature)
	collect = func(f *RecursiveFeature) {
		result = append(result, f)
		for _, sub := range f.GetSubFeatures() {
			collect(sub)
		}
	}

	collect(root)
	return result
}

// GetTotalTokenUsage returns aggregated token usage for a feature tree
func (m *Manager) GetTotalTokenUsage(rootID string) *TokenUsage {
	root := m.GetFeature(rootID)
	if root == nil {
		return nil
	}
	return root.GetTotalTokenUsage()
}

// GetAllActions returns all actions from a feature tree
func (m *Manager) GetAllActions(rootID string) []Action {
	tree := m.GetFeatureTree(rootID)
	if tree == nil {
		return nil
	}

	var actions []Action
	for _, f := range tree {
		actions = append(actions, f.GetActions()...)
	}

	return actions
}

// GetRootFeatures returns all root-level features
func (m *Manager) GetRootFeatures() []*RecursiveFeature {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var roots []*RecursiveFeature
	for _, f := range m.features {
		if f.IsRoot() {
			roots = append(roots, f)
		}
	}
	return roots
}

// CompleteSubFeature marks a sub-feature as complete and returns a summary
func (m *Manager) CompleteSubFeature(featureID string, status string, summary string) *SpawnResult {
	feature := m.GetFeature(featureID)
	if feature == nil {
		return nil
	}

	feature.SetStatus(status)

	result := &SpawnResult{
		FeatureID:  featureID,
		Title:      feature.Title,
		Status:     status,
		Summary:    summary,
		TokenUsage: feature.TokenUsage,
	}

	if status == "failed" {
		result.Error = summary
	}

	return result
}

// GenerateSpawnResultContext formats a spawn result for injection into parent context
func (m *Manager) GenerateSpawnResultContext(result *SpawnResult) string {
	if result == nil {
		return ""
	}

	data := map[string]interface{}{
		"sub_feature_completed": map[string]interface{}{
			"id":      result.FeatureID,
			"title":   result.Title,
			"status":  result.Status,
			"summary": result.Summary,
		},
	}

	if result.TokenUsage != nil {
		snapshot := result.TokenUsage.GetSnapshot()
		data["sub_feature_completed"].(map[string]interface{})["tokens_used"] = snapshot.TotalTokens
	}

	if result.Error != "" {
		data["sub_feature_completed"].(map[string]interface{})["error"] = result.Error
	}

	jsonBytes, _ := json.MarshalIndent(data, "", "  ")
	return string(jsonBytes)
}

// SetMaxDepth updates the max depth for new features
func (m *Manager) SetMaxDepth(depth int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if depth > 0 {
		m.maxDepth = depth
	}
}

// SetContextBudget updates the context budget for new features
func (m *Manager) SetContextBudget(budget int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if budget > 0 {
		m.contextBudget = budget
	}
}

// ClearFeature removes a feature and its tracker
func (m *Manager) ClearFeature(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.features, id)
	delete(m.trackers, id)
}

func generateID(input string) string {
	hash := sha256.Sum256([]byte(input))
	return fmt.Sprintf("%x", hash[:8])
}
