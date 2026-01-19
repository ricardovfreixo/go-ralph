package runner

import (
	"fmt"
	"sync"
	"time"

	"github.com/vx/ralph-go/internal/logger"
	"github.com/vx/ralph-go/internal/rlm"
)

// ChildExecutor manages execution of spawned child features
type ChildExecutor struct {
	mu sync.RWMutex

	manager      *Manager
	spawnHandler *rlm.SpawnHandler

	// Tracks parent → pending children
	pendingChildren map[string][]*rlm.SpawnRequest

	// Tracks parent → running children
	runningChildren map[string][]string

	// Tracks child → parent mapping
	childToParent map[string]string

	// Tracks paused parents waiting for children
	pausedParents map[string]bool

	// Tracks failed children per parent
	failedChildren map[string][]*rlm.ChildFailureResult

	// Tracks skipped children per parent
	skippedChildren map[string][]string

	// Callback when child completes
	onChildComplete func(parentID string, result *rlm.SpawnResult)

	// Callback when child fails (allows parent to decide action)
	onChildFailure func(parentID string, result *rlm.ChildFailureResult) rlm.ChildFailureAction
}

// NewChildExecutor creates a new child executor
func NewChildExecutor(manager *Manager, spawnHandler *rlm.SpawnHandler) *ChildExecutor {
	return &ChildExecutor{
		manager:         manager,
		spawnHandler:    spawnHandler,
		pendingChildren: make(map[string][]*rlm.SpawnRequest),
		runningChildren: make(map[string][]string),
		childToParent:   make(map[string]string),
		pausedParents:   make(map[string]bool),
		failedChildren:  make(map[string][]*rlm.ChildFailureResult),
		skippedChildren: make(map[string][]string),
	}
}

// SetOnChildComplete sets the callback for when a child completes
func (ce *ChildExecutor) SetOnChildComplete(callback func(parentID string, result *rlm.SpawnResult)) {
	ce.mu.Lock()
	defer ce.mu.Unlock()
	ce.onChildComplete = callback
}

// HandleSpawnRequest processes a spawn request for a parent feature
func (ce *ChildExecutor) HandleSpawnRequest(parentID string, req *rlm.SpawnRequest) error {
	if req == nil {
		return nil
	}

	ce.mu.Lock()
	ce.pendingChildren[parentID] = append(ce.pendingChildren[parentID], req)
	ce.mu.Unlock()

	parentShort := parentID
	if len(parentShort) > 8 {
		parentShort = parentShort[:8]
	}

	logger.Info("runner", "Spawn request queued",
		"parentID", parentShort,
		"childTitle", req.Title,
		"model", req.Model)

	return nil
}

// StartPendingChildren starts any pending children for a parent
// Returns the number of children started
func (ce *ChildExecutor) StartPendingChildren(parentID string) (int, error) {
	ce.mu.Lock()
	pending := ce.pendingChildren[parentID]
	if len(pending) == 0 {
		ce.mu.Unlock()
		return 0, nil
	}

	// Clear pending list
	ce.pendingChildren[parentID] = nil
	ce.mu.Unlock()

	started := 0
	for _, req := range pending {
		child, err := ce.startChild(parentID, req)
		if err != nil {
			parentShort := parentID
			if len(parentShort) > 8 {
				parentShort = parentShort[:8]
			}
			logger.Error("runner", "Failed to start child",
				"parentID", parentShort,
				"childTitle", req.Title,
				"error", err)
			continue
		}

		ce.mu.Lock()
		ce.runningChildren[parentID] = append(ce.runningChildren[parentID], child.ID)
		ce.childToParent[child.ID] = parentID
		ce.mu.Unlock()

		started++
	}

	return started, nil
}

// startChild spawns and starts a child feature
func (ce *ChildExecutor) startChild(parentID string, req *rlm.SpawnRequest) (*rlm.RecursiveFeature, error) {
	if ce.spawnHandler == nil {
		return nil, fmt.Errorf("spawn handler not configured")
	}

	// Get parent context for child prompt
	parentContext := ce.getParentContext(parentID)

	// Spawn child in RLM
	child, err := ce.spawnHandler.SpawnChild(parentID, req)
	if err != nil {
		return nil, err
	}

	// Build prompt for child
	prompt := ce.spawnHandler.BuildChildPrompt(req, parentContext)

	// Determine model
	model := req.Model
	if model == "" {
		parent := ce.spawnHandler.GetFeature(parentID)
		if parent != nil {
			model = parent.Model
		}
	}

	// Start the runner instance
	_, err = ce.manager.StartInstance(child.ID, model, prompt)
	if err != nil {
		return nil, fmt.Errorf("failed to start child instance: %w", err)
	}

	// Mark as running in RLM
	ce.spawnHandler.SetFeatureRunning(child.ID)

	childShort := child.ID
	if len(childShort) > 8 {
		childShort = childShort[:8]
	}
	parentShort := parentID
	if len(parentShort) > 8 {
		parentShort = parentShort[:8]
	}

	logger.Info("runner", "Child feature started",
		"childID", childShort,
		"parentID", parentShort,
		"title", child.Title,
		"depth", child.Depth,
		"model", model)

	return child, nil
}

// getParentContext collects context from parent feature for child
func (ce *ChildExecutor) getParentContext(parentID string) string {
	inst := ce.manager.GetInstance(parentID)
	if inst == nil {
		return ""
	}

	// Get recent output from parent as context
	output := inst.GetOutput()
	if len(output) > 2000 {
		// Keep last 2000 chars
		output = output[len(output)-2000:]
	}

	if output == "" {
		return ""
	}

	return fmt.Sprintf("Recent parent output:\n```\n%s\n```", output)
}

// PauseParent marks a parent as paused (waiting for children)
func (ce *ChildExecutor) PauseParent(parentID string) {
	ce.mu.Lock()
	defer ce.mu.Unlock()
	ce.pausedParents[parentID] = true

	parentShort := parentID
	if len(parentShort) > 8 {
		parentShort = parentShort[:8]
	}
	logger.Info("runner", "Parent paused for children",
		"parentID", parentShort)
}

// ResumeParent marks a parent as no longer paused
func (ce *ChildExecutor) ResumeParent(parentID string) {
	ce.mu.Lock()
	defer ce.mu.Unlock()
	delete(ce.pausedParents, parentID)

	parentShort := parentID
	if len(parentShort) > 8 {
		parentShort = parentShort[:8]
	}
	logger.Info("runner", "Parent resumed",
		"parentID", parentShort)
}

// IsParentPaused returns true if the parent is waiting for children
func (ce *ChildExecutor) IsParentPaused(parentID string) bool {
	ce.mu.RLock()
	defer ce.mu.RUnlock()
	return ce.pausedParents[parentID]
}

// GetParentID returns the parent ID for a child feature
func (ce *ChildExecutor) GetParentID(childID string) string {
	ce.mu.RLock()
	defer ce.mu.RUnlock()
	return ce.childToParent[childID]
}

// IsChildFeature returns true if the feature is a child (has a parent)
func (ce *ChildExecutor) IsChildFeature(featureID string) bool {
	ce.mu.RLock()
	defer ce.mu.RUnlock()
	_, ok := ce.childToParent[featureID]
	return ok
}

// GetRunningChildren returns the list of running children for a parent
func (ce *ChildExecutor) GetRunningChildren(parentID string) []string {
	ce.mu.RLock()
	defer ce.mu.RUnlock()
	result := make([]string, len(ce.runningChildren[parentID]))
	copy(result, ce.runningChildren[parentID])
	return result
}

// HasRunningChildren returns true if parent has running children
func (ce *ChildExecutor) HasRunningChildren(parentID string) bool {
	ce.mu.RLock()
	defer ce.mu.RUnlock()
	return len(ce.runningChildren[parentID]) > 0
}

// OnChildComplete handles child completion and returns result for parent
func (ce *ChildExecutor) OnChildComplete(childID string, status string, summary string) *rlm.SpawnResult {
	parentID := ce.GetParentID(childID)
	if parentID == "" {
		return nil
	}

	// Remove from running children
	ce.mu.Lock()
	children := ce.runningChildren[parentID]
	for i, id := range children {
		if id == childID {
			ce.runningChildren[parentID] = append(children[:i], children[i+1:]...)
			break
		}
	}
	delete(ce.childToParent, childID)
	ce.mu.Unlock()

	// Generate result through RLM
	var resultContext string
	if ce.spawnHandler != nil {
		resultContext = ce.spawnHandler.CompleteFeature(childID, status, summary)
	}

	childShort := childID
	if len(childShort) > 8 {
		childShort = childShort[:8]
	}
	parentShort := parentID
	if len(parentShort) > 8 {
		parentShort = parentShort[:8]
	}

	logger.Info("runner", "Child feature completed",
		"childID", childShort,
		"parentID", parentShort,
		"status", status)

	result := &rlm.SpawnResult{
		FeatureID: childID,
		Status:    status,
		Summary:   summary,
	}

	if ce.spawnHandler != nil {
		feature := ce.spawnHandler.GetFeature(childID)
		if feature != nil {
			result.Title = feature.Title
			result.TokenUsage = feature.TokenUsage
		}
	}

	if status == "failed" {
		result.Error = summary
	}

	// Store context for parent injection
	if resultContext != "" {
		ce.storeResultContext(parentID, resultContext)
	}

	// Call completion callback if set
	ce.mu.RLock()
	callback := ce.onChildComplete
	ce.mu.RUnlock()

	if callback != nil {
		callback(parentID, result)
	}

	return result
}

// resultContexts stores result contexts for parent injection
var resultContexts = struct {
	sync.RWMutex
	data map[string][]string
}{data: make(map[string][]string)}

func (ce *ChildExecutor) storeResultContext(parentID string, context string) {
	resultContexts.Lock()
	defer resultContexts.Unlock()
	resultContexts.data[parentID] = append(resultContexts.data[parentID], context)
}

// GetPendingResultContexts returns and clears pending result contexts for a parent
func (ce *ChildExecutor) GetPendingResultContexts(parentID string) []string {
	resultContexts.Lock()
	defer resultContexts.Unlock()

	contexts := resultContexts.data[parentID]
	delete(resultContexts.data, parentID)

	return contexts
}

// GenerateChildResultSummary creates a summary of child execution for injection
func (ce *ChildExecutor) GenerateChildResultSummary(parentID string) string {
	contexts := ce.GetPendingResultContexts(parentID)
	if len(contexts) == 0 {
		return ""
	}

	summary := "## Sub-Feature Results\n\n"
	for _, ctx := range contexts {
		summary += ctx + "\n\n"
	}

	return summary
}

// ShouldPauseForChildren returns true if parent should pause to wait for children
func (ce *ChildExecutor) ShouldPauseForChildren(parentID string) bool {
	ce.mu.RLock()
	defer ce.mu.RUnlock()

	// Has pending children that need to start
	if len(ce.pendingChildren[parentID]) > 0 {
		return true
	}

	// Has running children
	if len(ce.runningChildren[parentID]) > 0 {
		return true
	}

	return false
}

// AllChildrenComplete returns true if all children have completed
func (ce *ChildExecutor) AllChildrenComplete(parentID string) bool {
	ce.mu.RLock()
	defer ce.mu.RUnlock()

	return len(ce.pendingChildren[parentID]) == 0 && len(ce.runningChildren[parentID]) == 0
}

// WaitForChildren blocks until all children complete (with timeout)
func (ce *ChildExecutor) WaitForChildren(parentID string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		if ce.AllChildrenComplete(parentID) {
			return true
		}
		time.Sleep(100 * time.Millisecond)
	}

	return false
}

// ClearParent removes all tracking for a parent
func (ce *ChildExecutor) ClearParent(parentID string) {
	ce.mu.Lock()
	defer ce.mu.Unlock()

	delete(ce.pendingChildren, parentID)

	// Remove child mappings
	for _, childID := range ce.runningChildren[parentID] {
		delete(ce.childToParent, childID)
	}
	delete(ce.runningChildren, parentID)
	delete(ce.pausedParents, parentID)
	delete(ce.failedChildren, parentID)
	delete(ce.skippedChildren, parentID)

	// Clear result contexts
	resultContexts.Lock()
	delete(resultContexts.data, parentID)
	resultContexts.Unlock()
}

// SetOnChildFailure sets the callback for when a child fails
func (ce *ChildExecutor) SetOnChildFailure(callback func(parentID string, result *rlm.ChildFailureResult) rlm.ChildFailureAction) {
	ce.mu.Lock()
	defer ce.mu.Unlock()
	ce.onChildFailure = callback
}

// RecordChildFailure records a child failure and returns the action to take
func (ce *ChildExecutor) RecordChildFailure(childID, parentID, reason, errMsg string) *rlm.ChildFailureResult {
	childTitle := ""
	if ce.spawnHandler != nil {
		if feature := ce.spawnHandler.GetFeature(childID); feature != nil {
			childTitle = feature.Title
		}
	}

	result := &rlm.ChildFailureResult{
		ChildID:    childID,
		ChildTitle: childTitle,
		ParentID:   parentID,
		FailureInfo: &rlm.FailureInfo{
			Timestamp:   time.Now(),
			Reason:      reason,
			Error:       errMsg,
			Recoverable: true,
			RetryCount:  0,
			MaxRetries:  3,
		},
		Action: rlm.ChildFailureHandle, // Default: let parent handle
	}

	ce.mu.Lock()
	ce.failedChildren[parentID] = append(ce.failedChildren[parentID], result)
	ce.mu.Unlock()

	childShort := childID
	if len(childShort) > 8 {
		childShort = childShort[:8]
	}
	parentShort := parentID
	if len(parentShort) > 8 {
		parentShort = parentShort[:8]
	}

	logger.Info("runner", "Child failure recorded",
		"childID", childShort,
		"parentID", parentShort,
		"reason", reason)

	// Call failure callback if set
	ce.mu.RLock()
	callback := ce.onChildFailure
	ce.mu.RUnlock()

	if callback != nil {
		action := callback(parentID, result)
		result.Action = action
	}

	return result
}

// GetFailedChildren returns the list of failed children for a parent
func (ce *ChildExecutor) GetFailedChildren(parentID string) []*rlm.ChildFailureResult {
	ce.mu.RLock()
	defer ce.mu.RUnlock()

	results := ce.failedChildren[parentID]
	if len(results) == 0 {
		return nil
	}

	ret := make([]*rlm.ChildFailureResult, len(results))
	copy(ret, results)
	return ret
}

// HasFailedChildren returns true if a parent has any failed children
func (ce *ChildExecutor) HasFailedChildrenInExecutor(parentID string) bool {
	ce.mu.RLock()
	defer ce.mu.RUnlock()
	return len(ce.failedChildren[parentID]) > 0
}

// RetryChild retries a failed child with optional modified parameters
func (ce *ChildExecutor) RetryChild(childID string, modifiedParams *rlm.SpawnRequest) error {
	parentID := ce.GetParentID(childID)
	if parentID == "" {
		// Child already completed and removed from tracking
		ce.mu.RLock()
		for pid, failures := range ce.failedChildren {
			for _, f := range failures {
				if f.ChildID == childID {
					parentID = pid
					break
				}
			}
			if parentID != "" {
				break
			}
		}
		ce.mu.RUnlock()
	}

	if parentID == "" {
		return fmt.Errorf("cannot find parent for child %s", childID)
	}

	// Get original spawn request from failure record
	var originalReq *rlm.SpawnRequest
	ce.mu.Lock()
	for i, failure := range ce.failedChildren[parentID] {
		if failure.ChildID == childID {
			if failure.RetryParams != nil {
				originalReq = failure.RetryParams
			}
			// Update failure info
			if failure.FailureInfo != nil {
				failure.FailureInfo.IncrementRetry()
			}
			// Remove from failed list
			ce.failedChildren[parentID] = append(
				ce.failedChildren[parentID][:i],
				ce.failedChildren[parentID][i+1:]...,
			)
			break
		}
	}
	ce.mu.Unlock()

	// Use modified params if provided
	retryReq := modifiedParams
	if retryReq == nil && originalReq != nil {
		retryReq = originalReq
	}
	if retryReq == nil {
		// Try to get from spawn handler
		if ce.spawnHandler != nil {
			if feature := ce.spawnHandler.GetFeature(childID); feature != nil {
				retryReq = &rlm.SpawnRequest{
					Title: feature.Title,
					Model: feature.Model,
				}
			}
		}
	}
	if retryReq == nil {
		return fmt.Errorf("no spawn request available for retry")
	}

	childShort := childID
	if len(childShort) > 8 {
		childShort = childShort[:8]
	}
	parentShort := parentID
	if len(parentShort) > 8 {
		parentShort = parentShort[:8]
	}

	logger.Info("runner", "Retrying child feature",
		"childID", childShort,
		"parentID", parentShort,
		"title", retryReq.Title)

	// Queue the retry as a new spawn request
	return ce.HandleSpawnRequest(parentID, retryReq)
}

// SkipChild marks a child as skipped and removes it from tracking
func (ce *ChildExecutor) SkipChild(childID, reason string) error {
	parentID := ce.GetParentID(childID)
	if parentID == "" {
		// Check in failed children
		ce.mu.RLock()
		for pid, failures := range ce.failedChildren {
			for _, f := range failures {
				if f.ChildID == childID {
					parentID = pid
					break
				}
			}
			if parentID != "" {
				break
			}
		}
		ce.mu.RUnlock()
	}

	if parentID == "" {
		return fmt.Errorf("cannot find parent for child %s", childID)
	}

	ce.mu.Lock()
	// Remove from failed children
	for i, failure := range ce.failedChildren[parentID] {
		if failure.ChildID == childID {
			failure.Action = rlm.ChildFailureSkip
			failure.SkipReason = reason
			ce.failedChildren[parentID] = append(
				ce.failedChildren[parentID][:i],
				ce.failedChildren[parentID][i+1:]...,
			)
			break
		}
	}

	// Add to skipped children
	ce.skippedChildren[parentID] = append(ce.skippedChildren[parentID], childID)
	ce.mu.Unlock()

	childShort := childID
	if len(childShort) > 8 {
		childShort = childShort[:8]
	}
	parentShort := parentID
	if len(parentShort) > 8 {
		parentShort = parentShort[:8]
	}

	logger.Info("runner", "Child feature skipped",
		"childID", childShort,
		"parentID", parentShort,
		"reason", reason)

	return nil
}

// GetSkippedChildren returns the list of skipped children for a parent
func (ce *ChildExecutor) GetSkippedChildren(parentID string) []string {
	ce.mu.RLock()
	defer ce.mu.RUnlock()

	skipped := ce.skippedChildren[parentID]
	if len(skipped) == 0 {
		return nil
	}

	ret := make([]string, len(skipped))
	copy(ret, skipped)
	return ret
}

// DetermineChildFailureAction determines what action to take for a failed child
// based on the parent's isolation level
func (ce *ChildExecutor) DetermineChildFailureAction(childID, parentID string, isolationLevel rlm.IsolationLevel) rlm.ChildFailureAction {
	switch isolationLevel {
	case rlm.IsolationStrict:
		return rlm.ChildFailureAbort
	case rlm.IsolationLenient:
		return rlm.ChildFailureHandle
	default:
		return rlm.ChildFailureHandle
	}
}

// GenerateFailureSummary creates a summary of all child failures for a parent
func (ce *ChildExecutor) GenerateFailureSummary(parentID string) string {
	ce.mu.RLock()
	failures := ce.failedChildren[parentID]
	skipped := ce.skippedChildren[parentID]
	ce.mu.RUnlock()

	if len(failures) == 0 && len(skipped) == 0 {
		return ""
	}

	summary := "## Child Feature Status\n\n"

	if len(failures) > 0 {
		summary += "### Failed Children\n\n"
		for _, f := range failures {
			summary += fmt.Sprintf("- **%s** (%s)\n", f.ChildTitle, f.ChildID[:min(8, len(f.ChildID))])
			if f.FailureInfo != nil {
				summary += fmt.Sprintf("  - Reason: %s\n", f.FailureInfo.Reason)
				if f.FailureInfo.Error != "" {
					summary += fmt.Sprintf("  - Error: %s\n", f.FailureInfo.Error)
				}
				summary += fmt.Sprintf("  - Retries: %d/%d\n", f.FailureInfo.RetryCount, f.FailureInfo.MaxRetries)
			}
		}
		summary += "\n"
	}

	if len(skipped) > 0 {
		summary += "### Skipped Children\n\n"
		for _, id := range skipped {
			idShort := id
			if len(idShort) > 8 {
				idShort = idShort[:8]
			}
			summary += fmt.Sprintf("- %s\n", idShort)
		}
		summary += "\n"
	}

	return summary
}
