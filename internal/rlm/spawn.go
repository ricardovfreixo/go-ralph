package rlm

import (
	"fmt"

	"github.com/vx/ralph-go/internal/context"
	"github.com/vx/ralph-go/internal/logger"
	"github.com/vx/ralph-go/internal/manifest"
)

// SpawnHandler handles sub-feature spawning requests
type SpawnHandler struct {
	manager  *Manager
	manifest *manifest.Manifest
}

// NewSpawnHandler creates a new spawn handler
func NewSpawnHandler(mgr *Manager, m *manifest.Manifest) *SpawnHandler {
	return &SpawnHandler{
		manager:  mgr,
		manifest: m,
	}
}

// ProcessLine processes output and returns a spawn request if detected
func (h *SpawnHandler) ProcessLine(featureID string, line string) (*SpawnRequest, error) {
	if h.manager == nil {
		return nil, nil
	}

	spawnReq, err := h.manager.ProcessOutput(featureID, line)
	if err != nil {
		if err == ErrFeatureNotFound {
			return nil, nil
		}
		logger.Warn("rlm", "Spawn request rejected",
			"featureID", featureID,
			"error", err.Error())
		return nil, err
	}

	return spawnReq, nil
}

// SpawnChild creates a child feature from a spawn request
func (h *SpawnHandler) SpawnChild(parentID string, req *SpawnRequest) (*RecursiveFeature, error) {
	if h.manager == nil || req == nil {
		return nil, ErrInvalidSpawnData
	}

	parent := h.manager.GetFeature(parentID)
	if parent == nil {
		return nil, ErrFeatureNotFound
	}

	parentShort := parentID
	if len(parentShort) > 8 {
		parentShort = parentShort[:8]
	}

	logger.Info("rlm", "Spawning sub-feature",
		"parentID", parentShort,
		"title", req.Title,
		"depth", parent.Depth+1,
		"maxDepth", parent.MaxDepth,
		"model", req.Model,
		"taskCount", len(req.Tasks))

	child, err := h.manager.SpawnSubFeature(parentID, req)
	if err != nil {
		logger.Error("rlm", "Failed to spawn sub-feature",
			"parentID", parentShort,
			"title", req.Title,
			"error", err.Error())
		return nil, err
	}

	if h.manifest != nil {
		mf := manifest.ManifestFeature{
			ID:            child.ID,
			Title:         child.Title,
			Status:        "pending",
			Model:         child.Model,
			Execution:     child.ExecutionMode,
			ParentID:      parentID,
			Depth:         child.Depth,
			ContextBudget: int64(child.ContextBudget),
		}

		if err := h.manifest.AddSubFeature(parentID, mf); err != nil {
			logger.Warn("rlm", "Failed to add sub-feature to manifest",
				"childID", child.ID,
				"error", err.Error())
		} else {
			if err := h.manifest.Save(); err != nil {
				logger.Warn("rlm", "Failed to save manifest after spawn",
					"error", err.Error())
			}
		}
	}

	childShort := child.ID
	if len(childShort) > 8 {
		childShort = childShort[:8]
	}

	logger.Info("rlm", "Sub-feature created",
		"childID", childShort,
		"parentID", parentShort,
		"depth", child.Depth,
		"contextBudget", child.ContextBudget)

	return child, nil
}

// RegisterRootFeature registers a root feature with the RLM manager
func (h *SpawnHandler) RegisterRootFeature(id, title string) *RecursiveFeature {
	if h.manager == nil {
		return nil
	}

	feature := h.manager.RegisterFeature(id, title)

	if h.manifest != nil {
		maxDepth := h.manifest.GetMaxDepth()
		if maxDepth > 0 {
			feature.MaxDepth = maxDepth
		}
	}

	idShort := id
	if len(idShort) > 8 {
		idShort = idShort[:8]
	}

	logger.Debug("rlm", "Registered root feature",
		"featureID", idShort,
		"title", title,
		"maxDepth", feature.MaxDepth)

	return feature
}

// SetFeatureRunning marks a feature as running
func (h *SpawnHandler) SetFeatureRunning(featureID string) {
	if h.manager == nil {
		return
	}

	feature := h.manager.GetFeature(featureID)
	if feature != nil {
		feature.SetStatus("running")
	}
}

// CompleteFeature marks a feature as completed and returns context for parent
func (h *SpawnHandler) CompleteFeature(featureID string, status string, summary string) string {
	if h.manager == nil {
		return ""
	}

	result := h.manager.CompleteSubFeature(featureID, status, summary)
	if result == nil {
		return ""
	}

	feature := h.manager.GetFeature(featureID)
	if feature != nil && !feature.IsRoot() {
		idShort := featureID
		if len(idShort) > 8 {
			idShort = idShort[:8]
		}
		logger.Info("rlm", "Sub-feature completed",
			"featureID", idShort,
			"status", status,
			"tokens", feature.TokenUsage.TotalTokens)
	}

	return h.manager.GenerateSpawnResultContext(result)
}

// GetFeature returns a feature by ID
func (h *SpawnHandler) GetFeature(id string) *RecursiveFeature {
	if h.manager == nil {
		return nil
	}
	return h.manager.GetFeature(id)
}

// GetManager returns the underlying RLM manager
func (h *SpawnHandler) GetManager() *Manager {
	return h.manager
}

// CanSpawn returns true if the feature can spawn children
func (h *SpawnHandler) CanSpawn(featureID string) bool {
	if h.manager == nil {
		return false
	}

	feature := h.manager.GetFeature(featureID)
	if feature == nil {
		return false
	}

	return feature.CanSpawn()
}

// GetDepth returns the depth of a feature
func (h *SpawnHandler) GetDepth(featureID string) int {
	if h.manager == nil {
		return 0
	}

	feature := h.manager.GetFeature(featureID)
	if feature == nil {
		return 0
	}

	return feature.Depth
}

// BuildChildPrompt creates a prompt for a spawned child feature
func (h *SpawnHandler) BuildChildPrompt(req *SpawnRequest, parentContext string) string {
	prompt := fmt.Sprintf("# Sub-Feature: %s\n\n", req.Title)

	if req.Description != "" {
		prompt += fmt.Sprintf("## Description\n%s\n\n", req.Description)
	}

	if len(req.Tasks) > 0 {
		prompt += "## Tasks\n"
		for _, task := range req.Tasks {
			prompt += fmt.Sprintf("- [ ] %s\n", task)
		}
		prompt += "\n"
	}

	prompt += "## Instructions\n"
	prompt += "Complete the tasks listed above. When finished, ensure all tests pass.\n\n"

	if parentContext != "" {
		prompt += "## Context from Parent\n"
		prompt += parentContext + "\n"
	}

	return prompt
}

// BuildChildPromptWithBudget creates a budget-aware prompt for a child feature
// It uses the context package to extract and summarize parent context within budget
func (h *SpawnHandler) BuildChildPromptWithBudget(req *SpawnRequest, parentContext string, childBudget int64) string {
	if childBudget <= 0 {
		childBudget = int64(DefaultContextBudget)
	}

	// Use context package to prepare budget-aware child prompt
	return context.PrepareChildContext(parentContext, childBudget, req.Title, req.Tasks)
}

// GetContextBudget returns the context budget for a feature
func (h *SpawnHandler) GetContextBudget(featureID string) int64 {
	if h.manager == nil {
		return int64(DefaultContextBudget)
	}

	feature := h.manager.GetFeature(featureID)
	if feature == nil {
		return int64(DefaultContextBudget)
	}

	return feature.GetContextBudget()
}

// GetChildContextBudget calculates the context budget for a child of the given feature
func (h *SpawnHandler) GetChildContextBudget(parentID string) int64 {
	if h.manager == nil {
		return int64(DefaultContextBudget) / 2
	}

	parent := h.manager.GetFeature(parentID)
	if parent == nil {
		return int64(DefaultContextBudget) / 2
	}

	return parent.CalculateChildContextBudget()
}

// SetContextUsage updates the context usage for a feature
func (h *SpawnHandler) SetContextUsage(featureID string, tokens int64) {
	if h.manager == nil {
		return
	}

	feature := h.manager.GetFeature(featureID)
	if feature != nil {
		feature.SetContextUsed(tokens)

		if feature.NeedsContextSummarization() {
			idShort := featureID
			if len(idShort) > 8 {
				idShort = idShort[:8]
			}
			logger.Warn("rlm", "Context budget at 80%, consider summarizing",
				"featureID", idShort,
				"used", feature.GetContextUsed(),
				"budget", feature.GetContextBudget())
		}
	}
}

// NeedsContextSummarization checks if a feature needs context summarization
func (h *SpawnHandler) NeedsContextSummarization(featureID string) bool {
	if h.manager == nil {
		return false
	}

	feature := h.manager.GetFeature(featureID)
	if feature == nil {
		return false
	}

	return feature.NeedsContextSummarization()
}

// SummarizeContextForChild extracts essential context for a child feature
func (h *SpawnHandler) SummarizeContextForChild(parentContext string, childBudget int64) string {
	return context.ExtractEssentialContext(parentContext, childBudget)
}
