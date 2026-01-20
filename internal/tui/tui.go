package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/vx/ralph-go/internal/actions"
	"github.com/vx/ralph-go/internal/escalation"
	"github.com/vx/ralph-go/internal/logger"
	"github.com/vx/ralph-go/internal/manifest"
	"github.com/vx/ralph-go/internal/parser"
	"github.com/vx/ralph-go/internal/retry"
	"github.com/vx/ralph-go/internal/rlm"
	"github.com/vx/ralph-go/internal/runner"
	"github.com/vx/ralph-go/internal/state"
	"github.com/vx/ralph-go/internal/summary"
	"github.com/vx/ralph-go/internal/tui/layout"
	"github.com/vx/ralph-go/internal/usage"
)

type view int

const (
	viewMain view = iota
	viewInspect
)

type Model struct {
	prdPath             string
	workDir             string
	prd                 *parser.PRD
	state               *state.Progress
	manager             *runner.Manager
	spawnHandler        *rlm.SpawnHandler
	childExecutor       *runner.ChildExecutor
	escalationMgr       *escalation.Manager
	retryStrategy       *retry.Strategy
	layout              *layout.Layout
	splitPane           *layout.SplitPane
	taskList            *layout.TaskList
	activityLog         *layout.ActivityLog
	activityPane        *layout.ActivityPane
	modal               *layout.Modal
	helpModal           *layout.HelpModal
	confirmDialog       *layout.ConfirmDialog
	currentView         view
	selected            int
	inspecting          string
	scrollOffset        int
	autoScroll          bool
	showCost            bool
	width               int
	height              int
	err                 error
	quitting            bool
	autoMode            bool
	statusMsg           string
	statusExpiry        time.Time
	budgetAlertShown    bool
	pendingFeatureStart *parser.Feature
	childResults        map[string][]string
	// Manifest mode fields
	manifestMode bool
	manifest     *manifest.Manifest
	prdDir       string
}

func initialModel(prdPath string) Model {
	workDir := filepath.Dir(prdPath)
	actLog := layout.NewActivityLog()
	rlmMgr := rlm.NewManager()
	mgr := runner.NewManager(workDir)
	spawnHandler := rlm.NewSpawnHandler(rlmMgr, nil)
	childExec := runner.NewChildExecutor(mgr, spawnHandler)
	escalationMgr := escalation.NewManager()
	retryStrat := retry.NewStrategy()
	return Model{
		prdPath:       prdPath,
		workDir:       workDir,
		manager:       mgr,
		spawnHandler:  spawnHandler,
		childExecutor: childExec,
		escalationMgr: escalationMgr,
		retryStrategy: retryStrat,
		layout:        layout.New(),
		splitPane:     layout.NewSplitPane(),
		taskList:      layout.NewTaskList(),
		activityLog:   actLog,
		activityPane:  layout.NewActivityPane(actLog),
		modal:         layout.NewModal(),
		helpModal:     layout.NewHelpModal(),
		confirmDialog: layout.NewConfirmDialog(),
		currentView:   viewMain,
		childResults:  make(map[string][]string),
	}
}

func (m Model) Init() tea.Cmd {
	if m.manifestMode {
		return tea.Batch(
			loadManifest(m.prdDir),
			loadStateFromDir(m.prdDir),
		)
	}
	return tea.Batch(
		loadPRD(m.prdPath),
		loadState(m.prdPath),
	)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKeyPress(msg)
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.layout.SetSize(msg.Width, msg.Height)
		m.splitPane.SetSize(m.layout.ContentWidth(), m.layout.ContentHeight())
		m.taskList.SetSize(m.splitPane.LeftPaneWidth(), m.splitPane.ContentHeight())
		m.activityPane.SetSize(m.splitPane.RightPaneWidth(), m.splitPane.ContentHeight())
		m.modal.SetSize(msg.Width, msg.Height)
		m.helpModal.SetSize(msg.Width, msg.Height)
		m.confirmDialog.SetSize(msg.Width, msg.Height)
		return m, nil
	case prdLoadedMsg:
		if msg.err != nil {
			m.err = msg.err
			logger.Error("tui", "Failed to load PRD", "error", msg.err)
			return m, nil
		}
		m.prd = msg.prd
		m.layout.SetPRDTitle(m.prd.Title)
		m.activityLog.AddPRDLoaded(m.prd.Title)
		logger.Info("tui", "PRD loaded", "title", m.prd.Title, "features", len(m.prd.Features))
		for _, f := range m.prd.Features {
			if m.state != nil {
				m.state.InitFeature(f.ID, f.Title)
			}
		}
		// Set global budget on manager
		if m.prd.BudgetTokens > 0 || m.prd.BudgetUSD > 0 {
			m.manager.SetGlobalBudget(m.prd.BudgetTokens, m.prd.BudgetUSD)
			logger.Info("tui", "Global budget set", "tokens", m.prd.BudgetTokens, "usd", m.prd.BudgetUSD)
		}
		return m, nil
	case manifestLoadedMsg:
		if msg.err != nil {
			m.err = msg.err
			logger.Error("tui", "Failed to load manifest", "error", msg.err)
			return m, nil
		}
		m.manifest = msg.manifest
		m.prd = msg.prd // Synthetic PRD for TUI compatibility
		m.layout.SetPRDTitle(m.prd.Title)
		m.activityLog.AddPRDLoaded(m.prd.Title)
		logger.Info("tui", "Manifest loaded", "title", m.prd.Title, "features", len(m.prd.Features))
		for _, f := range m.prd.Features {
			if m.state != nil {
				m.state.InitFeature(f.ID, f.Title)
			}
		}
		// Set global budget on manager
		if m.prd.BudgetTokens > 0 || m.prd.BudgetUSD > 0 {
			m.manager.SetGlobalBudget(m.prd.BudgetTokens, m.prd.BudgetUSD)
			logger.Info("tui", "Global budget set", "tokens", m.prd.BudgetTokens, "usd", m.prd.BudgetUSD)
		}
		return m, nil
	case stateLoadedMsg:
		if msg.err != nil {
			m.state = state.NewProgress()
		} else {
			m.state = msg.state
		}
		// In manifest mode, state path is already set by loadStateFromDir
		if !m.manifestMode {
			m.state.SetPath(m.prdPath)
		}
		m.manager.SetConfig(runner.Config{
			MaxRetries:    m.state.Config.MaxRetries,
			MaxConcurrent: m.state.Config.MaxConcurrent,
		})
		return m, nil
	case instanceStartedMsg:
		displayID := msg.featureID
		if len(displayID) > 8 {
			displayID = displayID[:8]
		}
		if msg.err != nil {
			logger.Error("tui", "Failed to start instance", "featureID", displayID, "error", msg.err)
			m.setStatus(fmt.Sprintf("Error: %v", msg.err))
			return m, nil
		}
		logger.Info("tui", "Instance started", "featureID", displayID)
		feature := m.findFeature(msg.featureID)
		if feature != nil {
			m.activityLog.AddFeatureStarted(msg.featureID, feature.Title)
			m.spawnHandler.RegisterRootFeature(msg.featureID, feature.Title)
			m.spawnHandler.SetFeatureRunning(msg.featureID)
		}
		m.state.UpdateFeature(msg.featureID, "running")
		// Track initial model for auto model features
		if msg.instance != nil && msg.instance.IsAutoModelEnabled() {
			currentModel := msg.instance.GetCurrentModel()
			m.state.SetCurrentModel(msg.featureID, currentModel)
			m.state.AddModelSwitch(msg.featureID, "", currentModel, "initial", "auto mode initial selection")
			logger.Info("tui", "Auto model enabled", "featureID", displayID, "model", currentModel)
		}
		m.state.Save()
		return m, listenForOutput(msg.featureID, msg.instance)
	case modelChangedMsg:
		displayID := msg.featureID
		if len(displayID) > 8 {
			displayID = displayID[:8]
		}
		logger.Info("tui", "Model changed",
			"featureID", displayID,
			"from", msg.fromModel,
			"to", msg.toModel,
			"reason", msg.reason)
		m.state.AddModelSwitch(msg.featureID, msg.fromModel, msg.toModel, msg.reason, msg.details)
		m.state.Save()
		m.setStatus(fmt.Sprintf("Model escalated: %s → %s (%s)", msg.fromModel, msg.toModel, msg.reason))
		return m, nil
	case instanceOutputMsg:
		inst := m.manager.GetInstance(msg.featureID)
		if inst != nil {
			var cmds []tea.Cmd
			cmds = append(cmds, listenForOutput(msg.featureID, inst))

			// Check for spawn requests
			if spawnReq, err := m.spawnHandler.ProcessLine(msg.featureID, string(msg.line.Raw)); err == nil && spawnReq != nil {
				cmds = append(cmds, func() tea.Msg {
					return spawnRequestMsg{
						parentID: msg.featureID,
						request:  spawnReq,
					}
				})
			}

			// Check for model changes in auto model instances
			if inst.IsAutoModelEnabled() {
				currentModel := inst.GetCurrentModel()
				savedModel := m.state.GetCurrentModel(msg.featureID)
				if savedModel != "" && currentModel != savedModel {
					switches := inst.GetModelSwitches()
					if len(switches) > 0 {
						lastSwitch := switches[len(switches)-1]
						cmds = append(cmds, func() tea.Msg {
							return modelChangedMsg{
								featureID: msg.featureID,
								fromModel: savedModel,
								toModel:   currentModel,
								reason:    string(lastSwitch.Reason),
								details:   lastSwitch.Details,
							}
						})
					}
				}
			}

			return m, tea.Batch(cmds...)
		}
		return m, nil
	case spawnRequestMsg:
		return m.handleSpawnRequest(msg)
	case spawnStartedMsg:
		return m.handleSpawnStarted(msg)
	case instanceDoneMsg:
		return m.handleInstanceDone(msg)
	case tickMsg:
		if m.autoMode {
			return m.autoStartNext()
		}
		return m, nil
	case statusClearMsg:
		if time.Now().After(m.statusExpiry) {
			m.statusMsg = ""
		}
		return m, nil
	}
	return m, nil
}

func (m Model) handleInstanceDone(msg instanceDoneMsg) (tea.Model, tea.Cmd) {
	displayID := msg.featureID
	if len(displayID) > 8 {
		displayID = displayID[:8]
	}
	feature := m.findFeature(msg.featureID)
	featureTitle := ""
	if feature != nil {
		featureTitle = feature.Title
	}

	// Check if this is a child feature
	parentID := m.state.GetFeatureParent(msg.featureID)
	isChildFeature := parentID != ""

	inst := m.manager.GetInstance(msg.featureID)
	if inst != nil {
		testResults := inst.GetTestResults()
		m.state.SetTestResults(msg.featureID, testResults.Passed, testResults.Failed, testResults.Skipped, testResults.Output)

		// Save token usage to state for persistence
		u := inst.GetUsage()
		cost := inst.GetEstimatedCost()
		m.state.SetFeatureUsage(msg.featureID, u.InputTokens, u.OutputTokens, u.CacheReadTokens, u.CacheWriteTokens, cost)

		if msg.status == "failed" {
			errMsg := inst.GetError()
			m.state.SetFeatureError(msg.featureID, errMsg)
			m.activityLog.AddFeatureFailed(msg.featureID, featureTitle)

			if isChildFeature {
				// Handle child feature failure with fault isolation
				m.handleChildFailure(msg.featureID, parentID, featureTitle, errMsg)
			} else if m.autoMode && m.state.CanRetry(msg.featureID) {
				// For root features, consider adjustments before retry
				return m.handleRetryWithAdjustment(msg.featureID, feature, inst, errMsg, displayID)
			}
		} else {
			m.state.UpdateFeature(msg.featureID, msg.status)
			if msg.status == "completed" {
				m.activityLog.AddFeatureCompleted(msg.featureID, featureTitle)
			}
		}
	} else {
		m.state.UpdateFeature(msg.featureID, msg.status)
		if msg.status == "completed" {
			m.activityLog.AddFeatureCompleted(msg.featureID, featureTitle)
		} else if msg.status == "failed" {
			m.activityLog.AddFeatureFailed(msg.featureID, featureTitle)
			if isChildFeature {
				m.handleChildFailure(msg.featureID, parentID, featureTitle, "")
			}
		}
	}

	// Handle child feature completion - generate result for parent
	if isChildFeature {
		resultContext := m.generateChildResultContext(msg.featureID, msg.status)
		if resultContext != "" {
			m.childResults[parentID] = append(m.childResults[parentID], resultContext)

			// Append child result to parent's output for visibility
			parentInst := m.manager.GetInstance(parentID)
			if parentInst != nil {
				resultMsg := fmt.Sprintf("[Child completed: %s (%s)]", featureTitle, msg.status)
				parentInst.AppendOutput(resultMsg)
			}

			logger.Info("tui", "Child result stored for parent",
				"childID", displayID,
				"parentID", parentID[:min(8, len(parentID))],
				"status", msg.status)
		}
	}

	m.state.Save()

	if m.autoMode {
		if m.state.AllCompleted() {
			m.autoMode = false
			m.setStatus("All features completed!")
			return m, nil
		}
		return m, tea.Tick(500*time.Millisecond, func(t time.Time) tea.Msg { return tickMsg{} })
	}

	return m, nil
}

// handleChildFailure processes a child feature failure based on isolation level
func (m *Model) handleChildFailure(childID, parentID, childTitle, errMsg string) {
	displayChildID := childID
	if len(displayChildID) > 8 {
		displayChildID = displayChildID[:8]
	}
	displayParentID := parentID
	if len(displayParentID) > 8 {
		displayParentID = displayParentID[:8]
	}

	// Get parent's isolation level
	isolationLevel := m.getParentIsolationLevel(parentID)

	// Track failure in state
	m.state.AddFailedChild(parentID, childID)
	m.state.SetFailureReason(childID, errMsg)

	// Record failure in child executor for parent's context
	failureResult := m.childExecutor.RecordChildFailure(childID, parentID, "execution_failed", errMsg)

	// Determine action based on isolation level
	action := m.childExecutor.DetermineChildFailureAction(childID, parentID, isolationLevel)
	failureResult.Action = action

	logger.Info("tui", "Child failure handled",
		"childID", displayChildID,
		"parentID", displayParentID,
		"isolationLevel", string(isolationLevel),
		"action", string(action))

	switch action {
	case rlm.ChildFailureAbort:
		// Strict isolation: fail the parent
		m.state.SetFeatureError(parentID, fmt.Sprintf("Child feature '%s' failed: %s", childTitle, errMsg))
		m.state.UpdateFeature(parentID, "failed")
		m.manager.StopInstance(parentID)

		parentFeature := m.findFeatureOrChild(parentID)
		if parentFeature != nil {
			m.activityLog.AddFeatureFailed(parentID, fmt.Sprintf("%s (child failed)", parentFeature.Title))
		}
		m.setStatus(fmt.Sprintf("Parent %s failed due to child failure (strict isolation)", displayParentID))

	case rlm.ChildFailureHandle:
		// Lenient isolation: parent continues, receives failure info
		parentInst := m.manager.GetInstance(parentID)
		if parentInst != nil {
			failureMsg := fmt.Sprintf("[Child FAILED: %s - %s (parent continues with lenient isolation)]", childTitle, errMsg)
			parentInst.AppendOutput(failureMsg)
		}
		m.setStatus(fmt.Sprintf("Child %s failed, parent continues (lenient)", displayChildID))

	case rlm.ChildFailureSkip:
		// Skip this child and continue
		m.state.SkipFeature(childID, "Skipped due to failure")
		m.setStatus(fmt.Sprintf("Child %s skipped", displayChildID))

	case rlm.ChildFailureRetry:
		// This would be handled by explicit retry request
		logger.Info("tui", "Child failure marked for retry", "childID", displayChildID)
	}
}

// handleRetryWithAdjustment determines if adjustments should be made before retry
func (m Model) handleRetryWithAdjustment(featureID string, feature *parser.Feature, inst *runner.Instance, errMsg, displayID string) (tea.Model, tea.Cmd) {
	featureTitle := ""
	if feature != nil {
		featureTitle = feature.Title
	}

	attempt := m.state.GetAttempts(featureID)
	testResults := inst.GetTestResults()

	// Build failure context for decision making
	currentModel := m.state.GetCurrentModel(featureID)
	if currentModel == "" && feature != nil {
		currentModel = feature.Model
		if currentModel == "" || currentModel == "auto" {
			currentModel = "sonnet"
		}
	}

	// Track original model if not already set
	if m.state.GetOriginalModel(featureID) == "" {
		m.state.SetOriginalModel(featureID, currentModel)
	}

	// Determine if build errors are present
	hasBuildError := containsBuildError(errMsg)

	failureCtx := retry.FailureContext{
		FeatureID:     featureID,
		AttemptNum:    attempt,
		LastError:     errMsg,
		TestsFailed:   testResults.Failed,
		TestsPassed:   testResults.Passed,
		HasBuildError: hasBuildError,
		HasTimeout:    false,
		TaskCount:     0,
		CurrentModel:  currentModel,
	}
	if feature != nil {
		failureCtx.TaskCount = len(feature.Tasks)
	}

	// Get retry decision from strategy
	decision := m.retryStrategy.DecideRetry(failureCtx)

	// Apply adjustment if recommended
	if decision.ShouldAdjust && m.state.CanAdjust(featureID) {
		adj := state.AdjustmentState{
			Type:       string(decision.AdjustmentType),
			Reason:     string(decision.Reason),
			FromValue:  currentModel,
			ToValue:    decision.NewModel,
			Details:    decision.Details,
			AttemptNum: attempt,
		}
		m.state.AddAdjustment(featureID, adj)

		// Log the adjustment decision
		logger.Info("retry", "Adjustment applied before retry",
			"featureID", displayID,
			"type", string(decision.AdjustmentType),
			"from", currentModel,
			"to", decision.NewModel,
			"reason", string(decision.Reason),
			"details", decision.Details)

		// Apply model escalation
		if decision.AdjustmentType == retry.AdjustmentModelEscalation && decision.NewModel != "" {
			m.state.SetCurrentModel(featureID, decision.NewModel)
			m.activityLog.AddOutput(featureID, fmt.Sprintf("Model escalated: %s → %s (%s)",
				currentModel, decision.NewModel, decision.Details))
			m.setStatus(fmt.Sprintf("Retrying %s with %s (was %s)", displayID, decision.NewModel, currentModel))
		} else if decision.AdjustmentType == retry.AdjustmentTaskSimplify {
			m.state.SetSimplified(featureID, true)
			m.activityLog.AddOutput(featureID, "Tasks simplified for retry")
			m.setStatus(fmt.Sprintf("Retrying %s with simplified tasks", displayID))
		} else {
			m.setStatus(fmt.Sprintf("Auto-retrying %s (attempt %d)", displayID, attempt+1))
		}
	} else {
		m.setStatus(fmt.Sprintf("Auto-retrying %s (attempt %d)", displayID, attempt+1))
	}

	m.activityLog.AddFeatureRetry(featureID, featureTitle, attempt+1)
	m.manager.ClearInstance(featureID)

	if feature != nil {
		// If model was escalated, create adjusted feature with new model
		adjustedFeature := *feature
		if newModel := m.state.GetCurrentModel(featureID); newModel != "" && newModel != feature.Model {
			adjustedFeature.Model = newModel
		}

		m.state.Save()
		return m, tea.Batch(
			startFeatureWithBudget(adjustedFeature, m.prd.Context, m.workDir, m.manager),
			tea.Tick(time.Second, func(t time.Time) tea.Msg { return tickMsg{} }),
		)
	}

	m.state.Save()
	return m, tea.Tick(time.Second, func(t time.Time) tea.Msg { return tickMsg{} })
}

// containsBuildError checks if the error message indicates a build/compilation failure
func containsBuildError(errMsg string) bool {
	lower := errMsg
	if len(lower) > 500 {
		lower = lower[:500]
	}
	lower = toLower(lower)
	return contains(lower, "build failed") ||
		contains(lower, "compilation failed") ||
		contains(lower, "compile error") ||
		contains(lower, "syntax error") ||
		contains(lower, "undefined:") ||
		contains(lower, "cannot find")
}

// toLower converts string to lowercase without importing strings
func toLower(s string) string {
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		result[i] = c
	}
	return string(result)
}

// contains checks if s contains substr without importing strings
func contains(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	if len(substr) > len(s) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// formatDuration formats a duration as human-readable string
func formatDuration(d time.Duration) string {
	d = d.Round(time.Second)
	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	d -= m * time.Minute
	s := d / time.Second

	if h > 0 {
		return fmt.Sprintf("%dh%dm", h, m)
	}
	if m > 0 {
		return fmt.Sprintf("%dm%ds", m, s)
	}
	return fmt.Sprintf("%ds", s)
}

// getParentIsolationLevel returns the isolation level for a parent feature
func (m *Model) getParentIsolationLevel(parentID string) rlm.IsolationLevel {
	// Check state first
	level := m.state.GetIsolationLevel(parentID)
	if level != "" {
		return rlm.IsolationLevel(level)
	}

	// Check PRD feature
	feature := m.findFeature(parentID)
	if feature != nil && feature.IsolationLevel != "" {
		return rlm.IsolationLevel(feature.IsolationLevel)
	}

	// Check RLM manager
	if m.spawnHandler != nil {
		if rlmFeature := m.spawnHandler.GetFeature(parentID); rlmFeature != nil {
			return rlmFeature.GetIsolationLevel()
		}
	}

	// Default to lenient (child failures don't fail parent)
	return rlm.DefaultIsolationLevel
}

// generateChildResultContext creates a context summary for injection into parent
func (m Model) generateChildResultContext(childID string, status string) string {
	childFeature := m.spawnHandler.GetFeature(childID)
	if childFeature == nil {
		return ""
	}

	result := summary.NewChildResult(childID, childFeature.Title, status)

	inst := m.manager.GetInstance(childID)
	if inst != nil {
		testResults := inst.GetTestResults()
		if testResults.Total > 0 {
			failures := summary.ExtractTestFailures(testResults.Output)
			result.SetTestResults(testResults.Passed, testResults.Failed, testResults.Skipped, testResults.Output)
			if len(failures) > 0 && result.TestResults != nil {
				result.TestResults.Failures = failures
			}
		}

		if status == "failed" {
			errMsg := inst.GetError()
			if errMsg != "" {
				result.SetError(errMsg)
			}
		}

		instActions := inst.GetActions()
		result.ExtractFromActions(instActions)

		instUsage := inst.GetUsage()
		result.SetTokensUsed(instUsage.TotalTokens)

		if inst.CompletedAt != nil {
			result.SetDuration(inst.CompletedAt.Sub(inst.StartedAt))
		}
	}

	contextBudget := childFeature.GetContextBudget()
	if contextBudget <= 0 {
		contextBudget = summary.DefaultMaxSummaryTokens
	}
	summaryResult := result.GenerateSummary(contextBudget)

	contextText := m.spawnHandler.CompleteFeature(childID, status, summaryResult.Raw)
	if contextText == "" {
		return summaryResult.Formatted
	}
	return contextText
}

func (m Model) handleSpawnRequest(msg spawnRequestMsg) (tea.Model, tea.Cmd) {
	parentShort := msg.parentID
	if len(parentShort) > 8 {
		parentShort = parentShort[:8]
	}

	logger.Info("tui", "Processing spawn request",
		"parentID", parentShort,
		"childTitle", msg.request.Title,
		"taskCount", len(msg.request.Tasks))

	child, err := m.spawnHandler.SpawnChild(msg.parentID, msg.request)
	if err != nil {
		logger.Error("tui", "Failed to spawn child feature",
			"parentID", parentShort,
			"error", err.Error())
		m.setStatus(fmt.Sprintf("Spawn failed: %v", err))
		return m, nil
	}

	m.state.InitFeature(child.ID, child.Title)
	m.state.SetFeatureParent(child.ID, msg.parentID)
	m.state.Save()

	prompt := m.spawnHandler.BuildChildPrompt(msg.request, "")
	m.activityLog.AddFeatureStarted(child.ID, fmt.Sprintf("[sub] %s", child.Title))

	return m, startSpawnedChild(msg.parentID, child, prompt, m.workDir, m.manager)
}

func (m Model) handleSpawnStarted(msg spawnStartedMsg) (tea.Model, tea.Cmd) {
	childShort := msg.childID
	if len(childShort) > 8 {
		childShort = childShort[:8]
	}

	if msg.err != nil {
		logger.Error("tui", "Failed to start spawned child",
			"childID", childShort,
			"error", msg.err.Error())
		m.setStatus(fmt.Sprintf("Spawn start failed: %v", msg.err))
		return m, nil
	}

	logger.Info("tui", "Spawned child started",
		"childID", childShort,
		"childTitle", msg.childTitle)

	m.spawnHandler.SetFeatureRunning(msg.childID)
	m.state.UpdateFeature(msg.childID, "running")
	m.state.Save()

	return m, listenForOutput(msg.childID, msg.instance)
}

func (m *Model) findFeature(id string) *parser.Feature {
	if m.prd == nil {
		return nil
	}
	for i := range m.prd.Features {
		if m.prd.Features[i].ID == id {
			return &m.prd.Features[i]
		}
	}
	return nil
}

func (m *Model) findFeatureOrChild(id string) *parser.Feature {
	// First check PRD features
	if feature := m.findFeature(id); feature != nil {
		return feature
	}
	// For child features, create a synthetic Feature from state data
	if m.state != nil {
		if fs := m.state.GetFeature(id); fs != nil {
			return &parser.Feature{
				ID:    fs.ID,
				Title: fs.Title,
			}
		}
	}
	return nil
}

func (m *Model) setStatus(msg string) {
	m.statusMsg = msg
	m.statusExpiry = time.Now().Add(5 * time.Second)
}

// getPendingChildResults retrieves and clears any pending child results for a feature
func (m *Model) getPendingChildResults(parentID string) string {
	results := m.childResults[parentID]
	if len(results) == 0 {
		return ""
	}
	delete(m.childResults, parentID)

	context := "## Sub-Feature Results\n\n"
	for _, result := range results {
		context += result + "\n\n"
	}
	return context
}

// hasRunningChildren checks if a feature has any running child features
func (m *Model) hasRunningChildren(parentID string) bool {
	children := m.state.GetChildFeatures(parentID)
	for _, childID := range children {
		fs := m.state.GetFeature(childID)
		if fs != nil && fs.Status == "running" {
			return true
		}
	}
	return false
}

func (m Model) autoStartNext() (tea.Model, tea.Cmd) {
	if m.prd == nil || !m.autoMode {
		return m, nil
	}

	// Check global budget before starting new features
	if m.manager.HasGlobalBudget() && !m.manager.IsBudgetAcknowledged() {
		_, atThreshold, _ := m.manager.CheckGlobalBudget()
		if atThreshold && !m.budgetAlertShown {
			m.confirmDialog.Show(layout.ConfirmTypeBudget)
			return m, nil
		}
	}

	if !m.manager.CanStartMore() {
		return m, tea.Tick(time.Second, func(t time.Time) tea.Msg { return tickMsg{} })
	}

	for _, feature := range m.prd.Features {
		fs := m.state.GetFeature(feature.ID)
		if fs == nil || fs.Status == "pending" || fs.Status == "" {
			m.setStatus(fmt.Sprintf("Starting %s...", feature.Title))
			return m, tea.Batch(
				startFeatureWithBudget(feature, m.prd.Context, m.workDir, m.manager),
				tea.Tick(time.Second, func(t time.Time) tea.Msg { return tickMsg{} }),
			)
		}
	}

	retryable := m.state.GetRetryableFeatures()
	for _, id := range retryable {
		feature := m.findFeature(id)
		if feature != nil {
			m.setStatus(fmt.Sprintf("Retrying %s...", feature.Title))
			m.manager.ClearInstance(id)
			return m, tea.Batch(
				startFeatureWithBudget(*feature, m.prd.Context, m.workDir, m.manager),
				tea.Tick(time.Second, func(t time.Time) tea.Msg { return tickMsg{} }),
			)
		}
	}

	if m.manager.GetRunningCount() == 0 {
		m.autoMode = false
		if m.state.HasFailures() {
			m.setStatus("Stopped: some features failed after max retries")
		} else {
			m.setStatus("All features completed!")
		}
	}

	return m, nil
}

func (m Model) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.confirmDialog.IsVisible() {
		switch msg.String() {
		case "y", "Y":
			dialogType := m.confirmDialog.Type()
			m.confirmDialog.Hide()
			if dialogType == layout.ConfirmTypeQuit {
				m.quitting = true
				m.manager.StopAll()
				m.state.Save()
				return m, tea.Quit
			} else if dialogType == layout.ConfirmTypeReset {
				m.autoMode = false
				m.manager.StopAll()
				m.state.ResetAll()
				m.state.Save()
				deleteProgressMD(m.workDir)
				m.setStatus("Reset all features and cleared progress.md")
				logger.Info("tui", "Reset all features and deleted progress.md")
			} else if dialogType == layout.ConfirmTypeBudget {
				m.manager.AcknowledgeBudget()
				m.budgetAlertShown = true
				m.setStatus("Budget acknowledged - continuing execution")
				logger.Info("tui", "Budget acknowledged by user")
				// If there's a pending feature start, do it now
				if m.pendingFeatureStart != nil {
					feature := *m.pendingFeatureStart
					m.pendingFeatureStart = nil
					return m, startFeature(feature, m.prd.Context, m.workDir, m.manager)
				}
			}
			return m, nil
		case "n", "N", "esc":
			dialogType := m.confirmDialog.Type()
			m.confirmDialog.Hide()
			if dialogType == layout.ConfirmTypeBudget {
				m.pendingFeatureStart = nil
				m.autoMode = false
				m.setStatus("Stopped at budget limit")
			}
			return m, nil
		}
		return m, nil
	}

	if m.helpModal.IsVisible() {
		return m.handleHelpView(msg)
	}

	switch m.currentView {
	case viewMain:
		return m.handleMainView(msg)
	case viewInspect:
		return m.handleInspectView(msg)
	}
	return m, nil
}

func (m Model) handleMainView(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q":
		m.confirmDialog.Show(layout.ConfirmTypeQuit)
		return m, nil
	case "j", "down":
		if m.prd != nil && m.taskList.MoveDown() {
			m.selected = m.taskList.Selected()
		}
	case "k", "up":
		if m.taskList.MoveUp() {
			m.selected = m.taskList.Selected()
		}
	case " ":
		// Toggle expand/collapse for features with children
		if m.taskList.ToggleExpand() {
			// Keep selection in bounds after collapse
			if m.selected >= m.taskList.VisibleCount() {
				m.selected = m.taskList.VisibleCount() - 1
				m.taskList.SetSelected(m.selected)
			}
		}
	case "enter":
		if m.prd != nil && m.taskList.VisibleCount() > 0 {
			item := m.taskList.SelectedItem()
			if item != nil {
				m.inspecting = item.ID
				m.scrollOffset = 999999
				m.autoScroll = true
				m.currentView = viewInspect
			}
		}
	case "s":
		if m.prd != nil && m.taskList.VisibleCount() > 0 {
			item := m.taskList.SelectedItem()
			if item == nil {
				return m, nil
			}
			feature := m.findFeatureOrChild(item.ID)
			if feature == nil {
				return m, nil
			}
			status := m.getFeatureStatus(item.ID)
			if status == "running" {
				return m, nil
			}
			// Check global budget before starting
			if m.manager.HasGlobalBudget() && !m.manager.IsBudgetAcknowledged() {
				_, atThreshold, _ := m.manager.CheckGlobalBudget()
				if atThreshold {
					m.pendingFeatureStart = feature
					m.confirmDialog.Show(layout.ConfirmTypeBudget)
					return m, nil
				}
			}
			m.manager.ClearInstance(feature.ID)
			return m, startFeatureWithBudget(*feature, m.prd.Context, m.workDir, m.manager)
		}
	case "S":
		if m.prd != nil && !m.autoMode {
			m.autoMode = true
			m.setStatus("Auto mode enabled - starting features...")
			return m, tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg { return tickMsg{} })
		}
	case "r":
		if m.prd != nil && m.taskList.VisibleCount() > 0 {
			item := m.taskList.SelectedItem()
			if item == nil {
				return m, nil
			}
			feature := m.findFeatureOrChild(item.ID)
			if feature == nil {
				return m, nil
			}
			status := m.getFeatureStatus(item.ID)
			if status == "failed" || status == "completed" || status == "stopped" {
				// Check global budget before retrying
				if m.manager.HasGlobalBudget() && !m.manager.IsBudgetAcknowledged() {
					_, atThreshold, _ := m.manager.CheckGlobalBudget()
					if atThreshold {
						m.pendingFeatureStart = feature
						m.confirmDialog.Show(layout.ConfirmTypeBudget)
						return m, nil
					}
				}
				m.manager.ClearInstance(item.ID)
				return m, startFeatureWithBudget(*feature, m.prd.Context, m.workDir, m.manager)
			}
		}
	case "R":
		if m.prd != nil && m.taskList.VisibleCount() > 0 {
			item := m.taskList.SelectedItem()
			if item == nil {
				return m, nil
			}
			m.state.ResetFeature(item.ID)
			m.manager.ClearInstance(item.ID)
			m.state.Save()
			m.setStatus(fmt.Sprintf("Reset %s", item.Title))
		}
	case "x":
		if m.prd != nil && m.taskList.VisibleCount() > 0 {
			item := m.taskList.SelectedItem()
			if item == nil {
				return m, nil
			}
			m.manager.StopInstance(item.ID)
			m.state.UpdateFeature(item.ID, "stopped")
			m.activityLog.AddFeatureStopped(item.ID, item.Title)
			m.state.Save()
		}
	case "X":
		m.autoMode = false
		m.manager.StopAll()
		m.setStatus("Stopped all instances")
	case "ctrl+r":
		m.confirmDialog.Show(layout.ConfirmTypeReset)
	case "?":
		m.helpModal.Show()
	case "c":
		m.showCost = !m.showCost
		m.taskList.SetShowCost(m.showCost)
		if m.showCost {
			m.setStatus("Cost display enabled")
		} else {
			m.setStatus("Cost display disabled")
		}
	}
	return m, nil
}

func (m Model) getFeatureStatus(id string) string {
	if m.state == nil {
		return "pending"
	}
	if fs := m.state.GetFeature(id); fs != nil {
		return fs.Status
	}
	return "pending"
}

func (m Model) handleInspectView(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "esc":
		m.currentView = viewMain
		m.inspecting = ""
		m.scrollOffset = 0
		m.autoScroll = false
		m.modal.ResetView()
	case "j", "down":
		m.scrollOffset++
	case "k", "up":
		if m.scrollOffset > 0 {
			m.scrollOffset--
			m.autoScroll = false
		}
	case "G":
		m.scrollOffset = 999999
		m.autoScroll = true
	case "g":
		m.scrollOffset = 0
		m.autoScroll = false
	case "f":
		m.scrollOffset = 999999
		m.autoScroll = true
	case "a":
		m.modal.ToggleActions()
		m.scrollOffset = 0
	case "s":
		if m.inspecting != "" {
			feature := m.findFeature(m.inspecting)
			if feature != nil {
				status := m.getFeatureStatus(m.inspecting)
				if status != "running" {
					m.manager.ClearInstance(m.inspecting)
					return m, startFeature(*feature, m.prd.Context, m.workDir, m.manager)
				}
			}
		}
	case "x":
		if m.inspecting != "" {
			feature := m.findFeature(m.inspecting)
			m.manager.StopInstance(m.inspecting)
			m.state.UpdateFeature(m.inspecting, "stopped")
			if feature != nil {
				m.activityLog.AddFeatureStopped(m.inspecting, feature.Title)
			}
			m.state.Save()
		}
	}
	return m, nil
}

func (m Model) handleHelpView(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "esc", "?":
		m.helpModal.Hide()
	case "j", "down":
		m.helpModal.ScrollDown()
	case "k", "up":
		m.helpModal.ScrollUp()
	case "g":
		m.helpModal.ScrollToTop()
	case "G":
		m.helpModal.ScrollToBottom()
	}
	return m, nil
}

func (m Model) View() string {
	if m.quitting {
		return "Shutting down...\n"
	}

	if m.err != nil {
		return fmt.Sprintf("Error: %v\n\nPress q to quit.", m.err)
	}

	if m.prd == nil {
		return "Loading PRD..."
	}

	switch m.currentView {
	case viewInspect:
		return m.renderInspectView()
	default:
		return m.renderMainView()
	}
}

func (m Model) renderMainView() string {
	var total, completed, running, failed, pending int
	if m.state != nil {
		total, completed, running, failed, pending = m.state.GetSummary()
	}

	// Aggregate tokens: state (completed) + manager (running)
	runningUsage := m.manager.GetTotalUsage()
	var stateIn, stateOut, stateCacheR, stateCacheW int64
	if m.state != nil {
		stateIn, stateOut, stateCacheR, stateCacheW = m.state.GetTotalTokens()
	}
	totalTokens := stateIn + stateOut + stateCacheR + stateCacheW + runningUsage.InputTokens + runningUsage.OutputTokens + runningUsage.CacheReadTokens + runningUsage.CacheWriteTokens
	tokenUsageStr := ""
	if totalTokens > 0 {
		tokenUsageStr = usage.FormatTokens(totalTokens)
	}

	// Aggregate cost: state (completed) + manager (running)
	runningCost := m.manager.GetTotalCost()
	var stateCost float64
	if m.state != nil {
		stateCost = m.state.GetTotalCost()
	}
	totalCost := stateCost + runningCost
	totalCostStr := ""
	if totalCost > 0 {
		totalCostStr = usage.FormatCost(totalCost)
	}

	// Calculate total elapsed time
	elapsedStr := ""
	if m.state != nil {
		elapsed := m.state.GetTotalElapsed()
		if elapsed > 0 {
			elapsedStr = formatDuration(elapsed)
		}
	}

	// Check budget status
	budgetStatus := ""
	budgetAlert := false
	if m.manager.HasGlobalBudget() {
		budgetStatus = m.manager.GetGlobalBudgetStatus()
		_, atThreshold, _ := m.manager.CheckGlobalBudget()
		budgetAlert = atThreshold && !m.manager.IsBudgetAcknowledged()
	}

	headerData := layout.HeaderData{
		Version:      layout.AppName + " " + layout.AppVersion,
		Title:        "Feature Builder",
		AutoMode:     m.autoMode,
		Total:        total,
		Completed:    completed,
		Running:      running,
		Failed:       failed,
		Pending:      pending,
		TokenUsage:   tokenUsageStr,
		TotalCost:    totalCostStr,
		ShowCost:     m.showCost,
		BudgetStatus: budgetStatus,
		BudgetAlert:  budgetAlert,
		ElapsedTime:  elapsedStr,
	}

	keybindings := "s: start • S: start all • r: retry • R: reset • x: stop • X: stop all • ?: help • q: quit"

	var statusMsg string
	var statusColor lipgloss.TerminalColor
	if m.statusMsg != "" && time.Now().Before(m.statusExpiry) {
		statusMsg = m.statusMsg
		statusColor = layout.StatusColor("running")
	}

	footerData := layout.FooterData{
		Keybindings: keybindings,
		StatusMsg:   statusMsg,
		StatusColor: statusColor,
	}

	leftContent := m.renderFeatureList()
	rightContent := m.activityPane.Render()
	content := m.splitPane.Render(leftContent, rightContent)

	output := m.layout.Render(headerData, footerData, content)

	if m.helpModal.IsVisible() {
		output = m.helpModal.Render(output)
	}

	if m.confirmDialog.IsVisible() {
		output = m.confirmDialog.Render(output)
	}

	return output
}

func (m Model) buildTaskItems() []layout.TaskItem {
	if m.prd == nil {
		return nil
	}
	items := make([]layout.TaskItem, 0, len(m.prd.Features)*2)

	// Build a map of children by parent ID for quick lookup
	childrenByParent := make(map[string][]string)
	if m.state != nil {
		for id, fs := range m.state.Features {
			if fs.ParentID != "" {
				childrenByParent[fs.ParentID] = append(childrenByParent[fs.ParentID], id)
			}
		}
	}

	// Recursive function to build task item and its children
	var buildItem func(id, title string, parentID string, depth int) layout.TaskItem
	buildItem = func(id, title string, parentID string, depth int) layout.TaskItem {
		status := "pending"
		attempts := 0
		actionSummary := ""
		tokenUsage := ""
		cost := ""
		budgetStatus := ""
		budgetAlert := false
		model := ""
		modelChanged := false
		elapsedTime := ""

		if m.state != nil {
			if fs := m.state.GetFeature(id); fs != nil {
				status = fs.Status
				attempts = fs.Attempts
				model = fs.CurrentModel
				modelChanged = len(fs.ModelSwitches) > 1
			}
			if elapsed := m.state.GetFeatureElapsed(id); elapsed > 0 {
				elapsedTime = formatDuration(elapsed)
			}
		}
		if inst := m.manager.GetInstance(id); inst != nil {
			summary := inst.GetActionSummary()
			actionSummary = summary.String()
			u := inst.GetUsage()
			tokenUsage = u.Compact()
			estimatedCost := inst.GetEstimatedCost()
			if estimatedCost > 0 {
				cost = usage.FormatCost(estimatedCost)
			}
			if inst.HasBudget() {
				pct, atThreshold, _ := inst.CheckBudget()
				budgetTokens, budgetUSD := inst.GetBudget()
				if budgetUSD > 0 {
					budgetStatus = fmt.Sprintf("$%.2f/$%.2f", estimatedCost, budgetUSD)
				} else if budgetTokens > 0 {
					budgetStatus = fmt.Sprintf("%s/%s", usage.FormatTokens(u.TotalTokens), usage.FormatTokens(budgetTokens))
				}
				budgetAlert = atThreshold && pct >= 90
			}
			if model == "" {
				model = inst.GetCurrentModel()
			}
			if !modelChanged && inst.IsAutoModelEnabled() {
				switches := inst.GetModelSwitches()
				modelChanged = len(switches) > 1
			}
		}

		children := childrenByParent[id]
		hasChildren := len(children) > 0

		return layout.TaskItem{
			ID:            id,
			Title:         title,
			Status:        status,
			Attempts:      attempts,
			ActionSummary: actionSummary,
			TokenUsage:    tokenUsage,
			Cost:          cost,
			BudgetStatus:  budgetStatus,
			BudgetAlert:   budgetAlert,
			Model:         model,
			ModelChanged:  modelChanged,
			ElapsedTime:   elapsedTime,
			ParentID:      parentID,
			Children:      children,
			Depth:         depth,
			HasChildren:   hasChildren,
			ChildCount:    len(children),
		}
	}

	// Recursive function to add item and its children to the list
	var addItemAndChildren func(id, title string, parentID string, depth int, isLastChild bool)
	addItemAndChildren = func(id, title string, parentID string, depth int, isLastChild bool) {
		item := buildItem(id, title, parentID, depth)
		item.IsLastChild = isLastChild
		items = append(items, item)

		// Add children recursively
		children := childrenByParent[id]
		for i, childID := range children {
			childTitle := childID
			if m.state != nil {
				if fs := m.state.GetFeature(childID); fs != nil && fs.Title != "" {
					childTitle = fs.Title
				}
			}
			isLast := i == len(children)-1
			addItemAndChildren(childID, childTitle, id, depth+1, isLast)
		}
	}

	// Add root features and their children
	for i, feature := range m.prd.Features {
		isLast := i == len(m.prd.Features)-1
		addItemAndChildren(feature.ID, feature.Title, "", 0, isLast)
	}

	// Calculate child summaries for all items with children
	for i := range items {
		if items[i].HasChildren {
			items[i].ChildSummary = layout.CalculateChildSummary(items, items[i].ID)
		}
	}

	return items
}

func (m Model) renderFeatureList() string {
	m.taskList.SetSize(m.layout.ContentWidth(), m.layout.ContentHeight())
	m.taskList.SetItems(m.buildTaskItems())
	m.taskList.SetSelected(m.selected)
	return m.taskList.Render()
}

func (m Model) renderMainViewContent() string {
	var total, completed, running, failed, pending int
	if m.state != nil {
		total, completed, running, failed, pending = m.state.GetSummary()
	}

	// Aggregate tokens: state (completed) + manager (running)
	runningUsage := m.manager.GetTotalUsage()
	var stateIn, stateOut, stateCacheR, stateCacheW int64
	if m.state != nil {
		stateIn, stateOut, stateCacheR, stateCacheW = m.state.GetTotalTokens()
	}
	totalTokens := stateIn + stateOut + stateCacheR + stateCacheW + runningUsage.InputTokens + runningUsage.OutputTokens + runningUsage.CacheReadTokens + runningUsage.CacheWriteTokens
	tokenUsageStr := ""
	if totalTokens > 0 {
		tokenUsageStr = usage.FormatTokens(totalTokens)
	}

	// Aggregate cost: state (completed) + manager (running)
	runningCost := m.manager.GetTotalCost()
	var stateCost float64
	if m.state != nil {
		stateCost = m.state.GetTotalCost()
	}
	totalCost := stateCost + runningCost
	totalCostStr := ""
	if totalCost > 0 {
		totalCostStr = usage.FormatCost(totalCost)
	}

	// Calculate total elapsed time
	elapsedStr := ""
	if m.state != nil {
		elapsed := m.state.GetTotalElapsed()
		if elapsed > 0 {
			elapsedStr = formatDuration(elapsed)
		}
	}

	// Check budget status
	budgetStatus := ""
	budgetAlert := false
	if m.manager.HasGlobalBudget() {
		budgetStatus = m.manager.GetGlobalBudgetStatus()
		_, atThreshold, _ := m.manager.CheckGlobalBudget()
		budgetAlert = atThreshold && !m.manager.IsBudgetAcknowledged()
	}

	headerData := layout.HeaderData{
		Version:      layout.AppName + " " + layout.AppVersion,
		Title:        "Feature Builder",
		AutoMode:     m.autoMode,
		Total:        total,
		Completed:    completed,
		Running:      running,
		Failed:       failed,
		Pending:      pending,
		TokenUsage:   tokenUsageStr,
		TotalCost:    totalCostStr,
		ShowCost:     m.showCost,
		BudgetStatus: budgetStatus,
		BudgetAlert:  budgetAlert,
		ElapsedTime:  elapsedStr,
	}

	keybindings := "s: start • S: start all • r: retry • R: reset • x: stop • X: stop all • ?: help • q: quit"

	var statusMsg string
	var statusColor lipgloss.TerminalColor
	if m.statusMsg != "" && time.Now().Before(m.statusExpiry) {
		statusMsg = m.statusMsg
		statusColor = layout.StatusColor("running")
	}

	footerData := layout.FooterData{
		Keybindings: keybindings,
		StatusMsg:   statusMsg,
		StatusColor: statusColor,
	}

	leftContent := m.renderFeatureList()
	rightContent := m.activityPane.Render()
	content := m.splitPane.Render(leftContent, rightContent)
	return m.layout.Render(headerData, footerData, content)
}

func (m Model) renderInspectView() string {
	var featureTitle string
	var featureStatus string
	for _, f := range m.prd.Features {
		if f.ID == m.inspecting {
			featureTitle = f.Title
			featureStatus = m.getFeatureStatus(f.ID)
			break
		}
	}

	m.modal.SetTitle(featureTitle)
	m.modal.SetStatus(featureStatus)

	var testSummary string
	var usageSummary string
	var output string
	var actionTimeline string
	if inst := m.manager.GetInstance(m.inspecting); inst != nil {
		testResults := inst.GetTestResults()
		if testResults.Total > 0 {
			testStr := fmt.Sprintf("Tests: %d passed, %d failed", testResults.Passed, testResults.Failed)
			if testResults.Failed > 0 {
				testSummary = lipgloss.NewStyle().Foreground(layout.StatusColor("failed")).Render(testStr)
			} else {
				testSummary = lipgloss.NewStyle().Foreground(layout.StatusColor("completed")).Render(testStr)
			}
		}
		usage := inst.GetUsage()
		if !usage.IsEmpty() {
			usageSummary = lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Render("Tokens: " + usage.Detailed())
		}
		output = inst.GetOutput()
		if output == "" {
			output = "Waiting for output..."
		}
		actionTimeline = actions.FormatTimeline(inst.GetActions())
	} else {
		output = "No output yet. Press 's' to start this feature."
	}

	m.modal.SetTestSummary(testSummary)
	m.modal.SetUsageSummary(usageSummary)

	// Set adjustment summary if any adjustments were made
	adjustmentSummary := m.state.GetAdjustmentSummary(m.inspecting)
	m.modal.SetAdjustmentSummary(adjustmentSummary)

	m.modal.SetContent(output)
	m.modal.SetActionTimeline(actionTimeline)
	m.modal.SetScrollOffset(m.scrollOffset)
	m.modal.SetAutoScroll(m.autoScroll)

	background := m.renderMainViewContent()
	return m.modal.Render(background)
}

func deleteProgressMD(workDir string) {
	path := filepath.Join(workDir, "progress.md")
	os.Remove(path)
}

func Run(prdPath string) error {
	workDir := filepath.Dir(prdPath)
	if err := logger.Init(workDir); err != nil {
		return fmt.Errorf("failed to init logger: %w", err)
	}
	defer logger.Close()

	logger.Info("tui", "Starting ralph", "prd", prdPath)

	p := tea.NewProgram(initialModel(prdPath), tea.WithAltScreen())
	_, err := p.Run()

	logger.Info("tui", "Ralph exiting", "error", err)
	return err
}

func RunWithManifest(prdDir string) error {
	workDir := filepath.Dir(prdDir)
	if err := logger.Init(workDir); err != nil {
		return fmt.Errorf("failed to init logger: %w", err)
	}
	defer logger.Close()

	logger.Info("tui", "Starting ralph in manifest mode", "prdDir", prdDir)

	p := tea.NewProgram(initialModelForManifest(prdDir), tea.WithAltScreen())
	_, err := p.Run()

	logger.Info("tui", "Ralph exiting", "error", err)
	return err
}

func initialModelForManifest(prdDir string) Model {
	workDir := filepath.Dir(prdDir)
	actLog := layout.NewActivityLog()
	rlmMgr := rlm.NewManager()
	mgr := runner.NewManager(workDir)
	spawnHandler := rlm.NewSpawnHandler(rlmMgr, nil)
	childExec := runner.NewChildExecutor(mgr, spawnHandler)
	escalationMgr := escalation.NewManager()
	retryStrat := retry.NewStrategy()
	return Model{
		prdDir:        prdDir,
		workDir:       workDir,
		manifestMode:  true,
		manager:       mgr,
		spawnHandler:  spawnHandler,
		childExecutor: childExec,
		escalationMgr: escalationMgr,
		retryStrategy: retryStrat,
		layout:        layout.New(),
		splitPane:     layout.NewSplitPane(),
		taskList:      layout.NewTaskList(),
		activityLog:   actLog,
		activityPane:  layout.NewActivityPane(actLog),
		modal:         layout.NewModal(),
		helpModal:     layout.NewHelpModal(),
		confirmDialog: layout.NewConfirmDialog(),
		currentView:   viewMain,
		childResults:  make(map[string][]string),
	}
}
