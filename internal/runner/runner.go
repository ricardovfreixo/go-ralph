package runner

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/vx/ralph-go/internal/actions"
	"github.com/vx/ralph-go/internal/automodel"
	"github.com/vx/ralph-go/internal/logger"
	"github.com/vx/ralph-go/internal/usage"
)

// SpawnCallback is called when a spawn request is detected in output
type SpawnCallback func(featureID string, line string)

// ModelChangeCallback is called when auto model selection changes the model
type ModelChangeCallback func(featureID string, fromModel, toModel, reason, details string)

type Instance struct {
	mu                  sync.RWMutex
	FeatureID           string
	Model               string
	OriginalModel       string
	IsAutoModel         bool
	Status              string
	StartedAt           time.Time
	CompletedAt         *time.Time
	ExitCode            int
	cmd                 *exec.Cmd
	cancel              context.CancelFunc
	output              []OutputLine
	outputCh            chan OutputLine
	TestResults         *TestResults
	Error               string
	Actions             []actions.Action
	Usage               *usage.TokenUsage
	BudgetTokens        int64
	BudgetUSD           float64
	BudgetPaused        bool
	BudgetThreshold     bool
	SpawnCallback       SpawnCallback
	ModelChangeCallback ModelChangeCallback
	autoSelector        *automodel.Selector
}

type OutputLine struct {
	Timestamp time.Time
	Type      string
	Subtype   string
	Content   string
	Tool      string
	Raw       json.RawMessage
}

type TestResults struct {
	Passed  int
	Failed  int
	Skipped int
	Total   int
	Output  string
}

// Claude Code stream-json message types
type StreamMessage struct {
	Type      string          `json:"type"`
	Subtype   string          `json:"subtype,omitempty"`
	CostUSD   float64         `json:"cost_usd,omitempty"`
	Duration  float64         `json:"duration_ms,omitempty"`
	Message   json.RawMessage `json:"message,omitempty"`
	Content   string          `json:"content,omitempty"`
	Tool      string          `json:"tool,omitempty"`
	ToolInput json.RawMessage `json:"tool_input,omitempty"`
	Result    string          `json:"result,omitempty"`
	IsError   bool            `json:"is_error,omitempty"`
	SessionID string          `json:"session_id,omitempty"`
}

// Nested message content structures - Claude Code uses varying formats
type MessageContent struct {
	Content json.RawMessage `json:"content"`
	Text    string          `json:"text,omitempty"`
}

type ContentBlock struct {
	Type      string `json:"type"`
	Text      string `json:"text,omitempty"`
	Name      string `json:"name,omitempty"`
	Content   string `json:"content,omitempty"`
	ToolUseID string `json:"tool_use_id,omitempty"`
}

func extractTextContent(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}

	var mc MessageContent
	if err := json.Unmarshal(raw, &mc); err != nil {
		return ""
	}

	// Try direct text field first
	if mc.Text != "" {
		return mc.Text
	}

	// Try content as string
	var contentStr string
	if err := json.Unmarshal(mc.Content, &contentStr); err == nil && contentStr != "" {
		return contentStr
	}

	// Try content as array of blocks
	var blocks []ContentBlock
	if err := json.Unmarshal(mc.Content, &blocks); err == nil {
		var parts []string
		for _, block := range blocks {
			switch block.Type {
			case "text":
				if block.Text != "" {
					parts = append(parts, block.Text)
				}
			case "tool_use":
				parts = append(parts, fmt.Sprintf("[Tool: %s]", block.Name))
			case "tool_result":
				if block.Content != "" {
					content := block.Content
					if len(content) > 100 {
						content = content[:100] + "..."
					}
					parts = append(parts, content)
				}
			}
		}
		if len(parts) > 0 {
			return strings.Join(parts, " ")
		}
	}

	return ""
}

type Config struct {
	MaxRetries    int
	RetryDelay    time.Duration
	MaxConcurrent int
}

func DefaultConfig() Config {
	return Config{
		MaxRetries:    3,
		RetryDelay:    5 * time.Second,
		MaxConcurrent: 3,
	}
}

type Manager struct {
	mu                  sync.RWMutex
	instances           map[string]*Instance
	workDir             string
	config              Config
	globalBudgetTokens  int64
	globalBudgetUSD     float64
	budgetAcknowledged  bool
	spawnCallback       SpawnCallback
	modelChangeCallback ModelChangeCallback
	autoModelManager    *automodel.Manager
}

func NewManager(workDir string) *Manager {
	return &Manager{
		instances:        make(map[string]*Instance),
		workDir:          workDir,
		config:           DefaultConfig(),
		autoModelManager: automodel.NewManager(),
	}
}

func NewManagerWithConfig(workDir string, config Config) *Manager {
	return &Manager{
		instances:        make(map[string]*Instance),
		workDir:          workDir,
		config:           config,
		autoModelManager: automodel.NewManager(),
	}
}

func (m *Manager) SetConfig(config Config) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.config = config
}

// SetSpawnCallback sets the global spawn callback for all instances
func (m *Manager) SetSpawnCallback(callback SpawnCallback) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.spawnCallback = callback
}

// SetModelChangeCallback sets the global model change callback for all instances
func (m *Manager) SetModelChangeCallback(callback ModelChangeCallback) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.modelChangeCallback = callback
}

// StartInstanceOptions contains optional parameters for starting an instance
type StartInstanceOptions struct {
	IsLeafTask bool
	TaskCount  int
}

func (m *Manager) StartInstance(featureID string, model string, prompt string) (*Instance, error) {
	return m.StartInstanceWithOptions(featureID, model, prompt, StartInstanceOptions{})
}

func (m *Manager) StartInstanceWithOptions(featureID string, model string, prompt string, opts StartInstanceOptions) (*Instance, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if existing, ok := m.instances[featureID]; ok {
		if existing.Status == "running" {
			return nil, fmt.Errorf("instance for feature %s is already running", featureID)
		}
	}

	if m.config.MaxConcurrent > 0 {
		running := 0
		for _, inst := range m.instances {
			if inst.GetStatus() == "running" {
				running++
			}
		}
		if running >= m.config.MaxConcurrent {
			return nil, fmt.Errorf("max concurrent instances (%d) reached", m.config.MaxConcurrent)
		}
	}

	ctx, cancel := context.WithCancel(context.Background())

	isAutoModel := automodel.IsAutoMode(model)
	originalModel := model
	actualModel := model

	var selector *automodel.Selector
	if isAutoModel {
		selector = m.autoModelManager.Register(featureID, opts.IsLeafTask, opts.TaskCount)
		actualModel = selector.CurrentModel()
	}

	inst := &Instance{
		FeatureID:           featureID,
		Model:               actualModel,
		OriginalModel:       originalModel,
		IsAutoModel:         isAutoModel,
		Status:              "starting",
		StartedAt:           time.Now(),
		cancel:              cancel,
		outputCh:            make(chan OutputLine, 100),
		TestResults:         &TestResults{},
		Usage:               usage.New(),
		SpawnCallback:       m.spawnCallback,
		ModelChangeCallback: m.modelChangeCallback,
		autoSelector:        selector,
	}

	args := []string{
		"--dangerously-skip-permissions",
		"--verbose",
		"--output-format", "stream-json",
	}

	if actualModel != "" && actualModel != "sonnet" {
		args = append(args, "--model", actualModel)
	}

	args = append(args, "-p", prompt)

	inst.cmd = exec.CommandContext(ctx, "claude", args...)
	inst.cmd.Dir = m.workDir

	displayID := featureID
	if len(displayID) > 8 {
		displayID = displayID[:8]
	}
	logModel := actualModel
	if isAutoModel {
		logModel = fmt.Sprintf("auto->%s", actualModel)
	}
	logger.Info("runner", "Starting claude instance",
		"featureID", displayID,
		"model", logModel,
		"workDir", m.workDir,
		"promptLen", len(prompt))
	logger.Debug("runner", "Full command args", "args", strings.Join(args[:len(args)-1], " ")+" -p <prompt>")

	stdout, err := inst.cmd.StdoutPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderr, err := inst.cmd.StderrPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	if err := inst.cmd.Start(); err != nil {
		cancel()
		logger.Error("runner", "Failed to start claude", "featureID", displayID, "error", err)
		return nil, fmt.Errorf("failed to start claude: %w", err)
	}

	logger.Info("runner", "Claude process started", "featureID", displayID, "pid", inst.cmd.Process.Pid)

	inst.Status = "running"
	m.instances[featureID] = inst

	go inst.readOutput(stdout, "stdout")
	go inst.readOutput(stderr, "stderr")
	go inst.waitForCompletion()

	return inst, nil
}

func (inst *Instance) readOutput(r io.Reader, source string) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	featureShort := inst.FeatureID
	if len(featureShort) > 8 {
		featureShort = featureShort[:8]
	}

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		outputLine := OutputLine{
			Timestamp: time.Now(),
			Raw:       json.RawMessage(line),
		}

		var msg StreamMessage
		if err := json.Unmarshal([]byte(line), &msg); err == nil {
			outputLine.Type = msg.Type
			outputLine.Subtype = msg.Subtype

			// Parse token usage from stream-json
			inst.Usage.ParseLine(line)

			// Process auto model selection
			inst.processAutoModel(line)

			rawPreview := line
			if len(rawPreview) > 300 {
				rawPreview = rawPreview[:300]
			}
			logger.Debug("runner", "Received message",
				"featureID", featureShort,
				"source", source,
				"type", msg.Type,
				"subtype", msg.Subtype,
				"raw", rawPreview)

			switch msg.Type {
			case "assistant":
				outputLine.Content = extractTextContent(msg.Message)
				if outputLine.Content == "" {
					outputLine.Content = msg.Content
				}
				inst.detectTestResults(outputLine.Content)
				if len(outputLine.Content) > 200 {
					outputLine.Content = outputLine.Content[:200] + "..."
				}
			case "user":
				outputLine.Content = extractTextContent(msg.Message)
				if outputLine.Content == "" {
					outputLine.Content = msg.Content
				}
			case "system":
				outputLine.Content = msg.Content
				outputLine.Subtype = msg.Subtype
			case "tool_use":
				outputLine.Tool = msg.Tool
				outputLine.Content = fmt.Sprintf("[Tool: %s]", msg.Tool)
				if action := actions.ExtractAction(msg.Tool, msg.ToolInput, outputLine.Timestamp); action != nil {
					inst.mu.Lock()
					inst.Actions = append(inst.Actions, *action)
					inst.mu.Unlock()
				}
				// Check for spawn request
				if msg.Tool == "ralph_spawn_feature" {
					inst.mu.RLock()
					callback := inst.SpawnCallback
					inst.mu.RUnlock()
					if callback != nil {
						callback(inst.FeatureID, line)
					}
				}
			case "tool_result":
				outputLine.Content = msg.Result
				if msg.IsError {
					outputLine.Subtype = "error"
				}
				inst.detectTestResults(msg.Result)
				if len(outputLine.Content) > 500 {
					outputLine.Content = outputLine.Content[:500] + "..."
				}
			case "result":
				outputLine.Subtype = msg.Subtype
				if msg.Subtype == "success" {
					outputLine.Content = "[Completed successfully]"
				} else if msg.Subtype == "error" {
					outputLine.Content = fmt.Sprintf("[Error: %s]", msg.Result)
				}
			case "error":
				outputLine.Content = msg.Result
				inst.mu.Lock()
				inst.Error = msg.Result
				inst.mu.Unlock()
			default:
				outputLine.Content = line
			}
		} else {
			outputLine.Type = source
			outputLine.Content = line
			inst.detectTestResults(line)
		}

		inst.mu.Lock()
		inst.output = append(inst.output, outputLine)
		inst.mu.Unlock()

		select {
		case inst.outputCh <- outputLine:
		default:
		}
	}
}

var testPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(\d+)\s+pass(?:ed|ing)?`),
	regexp.MustCompile(`(?i)(\d+)\s+fail(?:ed|ing|ure)?`),
	regexp.MustCompile(`(?i)PASS:\s*(\d+)`),
	regexp.MustCompile(`(?i)FAIL:\s*(\d+)`),
	regexp.MustCompile(`(?i)ok\s+\S+\s+[\d.]+s`),
	regexp.MustCompile(`(?i)---\s*PASS:`),
	regexp.MustCompile(`(?i)---\s*FAIL:`),
	regexp.MustCompile(`(?i)Tests:\s*(\d+)\s+passed`),
	regexp.MustCompile(`(?i)(\d+)\s+tests?\s+passed`),
}

func (inst *Instance) detectTestResults(content string) {
	inst.mu.Lock()
	defer inst.mu.Unlock()

	for _, pattern := range testPatterns {
		if matches := pattern.FindStringSubmatch(content); matches != nil {
			if strings.Contains(strings.ToLower(content), "pass") {
				if len(matches) > 1 {
					var n int
					fmt.Sscanf(matches[1], "%d", &n)
					if n > inst.TestResults.Passed {
						inst.TestResults.Passed = n
					}
				} else {
					inst.TestResults.Passed++
				}
			}
			if strings.Contains(strings.ToLower(content), "fail") {
				if len(matches) > 1 {
					var n int
					fmt.Sscanf(matches[1], "%d", &n)
					if n > inst.TestResults.Failed {
						inst.TestResults.Failed = n
					}
				} else {
					inst.TestResults.Failed++
				}
			}
		}
	}

	if strings.Contains(content, "ok  \t") || strings.Contains(content, "PASS") {
		inst.TestResults.Output += content + "\n"
	}
	if strings.Contains(content, "FAIL") || strings.Contains(content, "--- FAIL") {
		inst.TestResults.Output += content + "\n"
	}

	inst.TestResults.Total = inst.TestResults.Passed + inst.TestResults.Failed + inst.TestResults.Skipped
}

func (inst *Instance) waitForCompletion() {
	featureShort := inst.FeatureID
	if len(featureShort) > 8 {
		featureShort = featureShort[:8]
	}

	err := inst.cmd.Wait()
	inst.mu.Lock()
	defer inst.mu.Unlock()

	now := time.Now()
	inst.CompletedAt = &now
	duration := now.Sub(inst.StartedAt)

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			inst.ExitCode = exitErr.ExitCode()
		}
		inst.Status = "failed"
		if inst.Error == "" {
			inst.Error = err.Error()
		}
		logger.Error("runner", "Instance failed",
			"featureID", featureShort,
			"exitCode", inst.ExitCode,
			"error", inst.Error,
			"duration", duration.Round(time.Second))
	} else {
		inst.ExitCode = 0
		if inst.TestResults.Failed > 0 {
			inst.Status = "failed"
			inst.Error = fmt.Sprintf("%d tests failed", inst.TestResults.Failed)
			logger.Warn("runner", "Instance completed with test failures",
				"featureID", featureShort,
				"passed", inst.TestResults.Passed,
				"failed", inst.TestResults.Failed,
				"duration", duration.Round(time.Second))
		} else {
			inst.Status = "completed"
			logger.Info("runner", "Instance completed successfully",
				"featureID", featureShort,
				"passed", inst.TestResults.Passed,
				"duration", duration.Round(time.Second))
		}
	}
	close(inst.outputCh)
}

func (inst *Instance) Stop() {
	if inst.cancel != nil {
		inst.cancel()
	}
}

func (inst *Instance) GetStatus() string {
	inst.mu.RLock()
	defer inst.mu.RUnlock()
	return inst.Status
}

func (inst *Instance) SetStatus(status string) {
	inst.mu.Lock()
	defer inst.mu.Unlock()
	inst.Status = status
}

// SetSpawnCallback sets the callback for spawn requests
func (inst *Instance) SetSpawnCallback(callback SpawnCallback) {
	inst.mu.Lock()
	defer inst.mu.Unlock()
	inst.SpawnCallback = callback
}

// SetModelChangeCallback sets the callback for model changes
func (inst *Instance) SetModelChangeCallback(callback ModelChangeCallback) {
	inst.mu.Lock()
	defer inst.mu.Unlock()
	inst.ModelChangeCallback = callback
}

// processAutoModel processes output line for auto model selection
func (inst *Instance) processAutoModel(line string) {
	inst.mu.RLock()
	selector := inst.autoSelector
	isAuto := inst.IsAutoModel
	callback := inst.ModelChangeCallback
	featureID := inst.FeatureID
	inst.mu.RUnlock()

	if !isAuto || selector == nil {
		return
	}

	changed, newModel := selector.ProcessLine(line)
	if changed {
		inst.mu.Lock()
		oldModel := inst.Model
		inst.Model = newModel
		inst.mu.Unlock()

		switches := selector.Switches()
		if len(switches) > 0 {
			lastSwitch := switches[len(switches)-1]
			if callback != nil {
				callback(featureID, oldModel, newModel, string(lastSwitch.Reason), lastSwitch.Details)
			}
		}
	}
}

// GetModelSwitches returns the model switches for auto model instances
func (inst *Instance) GetModelSwitches() []automodel.ModelSwitch {
	inst.mu.RLock()
	selector := inst.autoSelector
	inst.mu.RUnlock()

	if selector == nil {
		return nil
	}
	return selector.Switches()
}

// IsAutoModelEnabled returns true if auto model is enabled for this instance
func (inst *Instance) IsAutoModelEnabled() bool {
	inst.mu.RLock()
	defer inst.mu.RUnlock()
	return inst.IsAutoModel
}

// GetOriginalModel returns the original model (before auto selection)
func (inst *Instance) GetOriginalModel() string {
	inst.mu.RLock()
	defer inst.mu.RUnlock()
	return inst.OriginalModel
}

// GetCurrentModel returns the current model (after auto selection)
func (inst *Instance) GetCurrentModel() string {
	inst.mu.RLock()
	defer inst.mu.RUnlock()
	return inst.Model
}

func (inst *Instance) GetError() string {
	inst.mu.RLock()
	defer inst.mu.RUnlock()
	return inst.Error
}

func (inst *Instance) GetTestResults() TestResults {
	inst.mu.RLock()
	defer inst.mu.RUnlock()
	if inst.TestResults == nil {
		return TestResults{}
	}
	return *inst.TestResults
}

func (inst *Instance) GetOutput() string {
	inst.mu.RLock()
	defer inst.mu.RUnlock()

	var sb strings.Builder
	for _, line := range inst.output {
		prefix := line.Type
		if line.Subtype != "" {
			prefix = fmt.Sprintf("%s:%s", line.Type, line.Subtype)
		}
		if line.Tool != "" {
			prefix = fmt.Sprintf("%s[%s]", line.Type, line.Tool)
		}
		sb.WriteString(fmt.Sprintf("[%s] %s: %s\n",
			line.Timestamp.Format("15:04:05"),
			prefix,
			line.Content,
		))
	}
	return sb.String()
}

func (inst *Instance) GetOutputLines() []OutputLine {
	inst.mu.RLock()
	defer inst.mu.RUnlock()
	result := make([]OutputLine, len(inst.output))
	copy(result, inst.output)
	return result
}

func (inst *Instance) GetActions() []actions.Action {
	inst.mu.RLock()
	defer inst.mu.RUnlock()
	result := make([]actions.Action, len(inst.Actions))
	copy(result, inst.Actions)
	return result
}

func (inst *Instance) GetActionSummary() actions.ActionSummary {
	inst.mu.RLock()
	defer inst.mu.RUnlock()

	var summary actions.ActionSummary
	for _, action := range inst.Actions {
		switch action.Type {
		case actions.ActionWrite, actions.ActionEdit:
			summary.Files++
		case actions.ActionBash:
			summary.Commands++
		case actions.ActionTask, actions.ActionAgent:
			summary.Agents++
		case actions.ActionRead:
			summary.Reads++
		case actions.ActionWebFetch:
			summary.Fetches++
		case actions.ActionGrep, actions.ActionGlob:
			summary.Searches++
		}
	}
	return summary
}

func (inst *Instance) GetUsage() usage.TokenUsage {
	if inst.Usage == nil {
		return usage.TokenUsage{}
	}
	return inst.Usage.Snapshot()
}

func (inst *Instance) GetEstimatedCost() float64 {
	if inst.Usage == nil {
		return 0
	}
	return inst.Usage.GetEstimatedCost(inst.Model)
}

func (inst *Instance) GetUsageWithCost() string {
	if inst.Usage == nil {
		return ""
	}
	return inst.Usage.CompactWithCost(inst.Model)
}

// SetBudget sets the budget limits for this instance
func (inst *Instance) SetBudget(tokens int64, usd float64) {
	inst.mu.Lock()
	defer inst.mu.Unlock()
	inst.BudgetTokens = tokens
	inst.BudgetUSD = usd
}

// GetBudget returns the budget limits for this instance
func (inst *Instance) GetBudget() (tokens int64, usd float64) {
	inst.mu.RLock()
	defer inst.mu.RUnlock()
	return inst.BudgetTokens, inst.BudgetUSD
}

// HasBudget returns true if a budget is set for this instance
func (inst *Instance) HasBudget() bool {
	inst.mu.RLock()
	defer inst.mu.RUnlock()
	return inst.BudgetTokens > 0 || inst.BudgetUSD > 0
}

// CheckBudget checks current usage against budget and returns status
// Returns: percent used, at threshold (>=90%), over budget
func (inst *Instance) CheckBudget() (percent float64, atThreshold bool, overBudget bool) {
	inst.mu.RLock()
	defer inst.mu.RUnlock()

	if inst.Usage == nil {
		return 0, false, false
	}

	snapshot := inst.Usage.Snapshot()

	if inst.BudgetTokens > 0 {
		percent = float64(snapshot.TotalTokens) / float64(inst.BudgetTokens) * 100
		atThreshold = percent >= 90
		overBudget = snapshot.TotalTokens >= inst.BudgetTokens
		return
	}

	if inst.BudgetUSD > 0 {
		cost := snapshot.CostUSD
		if cost == 0 {
			cost = inst.Usage.GetEstimatedCost(inst.Model)
		}
		percent = cost / inst.BudgetUSD * 100
		atThreshold = percent >= 90
		overBudget = cost >= inst.BudgetUSD
		return
	}

	return 0, false, false
}

// IsBudgetPaused returns whether instance is paused due to budget
func (inst *Instance) IsBudgetPaused() bool {
	inst.mu.RLock()
	defer inst.mu.RUnlock()
	return inst.BudgetPaused
}

// SetBudgetPaused sets whether the instance is paused due to budget
func (inst *Instance) SetBudgetPaused(paused bool) {
	inst.mu.Lock()
	defer inst.mu.Unlock()
	inst.BudgetPaused = paused
}

// SetBudgetThreshold marks that the instance has reached budget threshold
func (inst *Instance) SetBudgetThreshold(reached bool) {
	inst.mu.Lock()
	defer inst.mu.Unlock()
	inst.BudgetThreshold = reached
}

// IsBudgetThresholdReached returns whether threshold has been reached
func (inst *Instance) IsBudgetThresholdReached() bool {
	inst.mu.RLock()
	defer inst.mu.RUnlock()
	return inst.BudgetThreshold
}

func (inst *Instance) AppendOutput(line string) {
	inst.mu.Lock()
	defer inst.mu.Unlock()

	inst.output = append(inst.output, OutputLine{
		Timestamp: time.Now(),
		Type:      "info",
		Content:   line,
	})
}

func (inst *Instance) OutputChannel() <-chan OutputLine {
	return inst.outputCh
}

func (inst *Instance) ClearInstance(featureID string) {
	inst.mu.Lock()
	defer inst.mu.Unlock()
	inst.output = nil
	inst.TestResults = &TestResults{}
	inst.Error = ""
	inst.Actions = nil
	if inst.Usage != nil {
		inst.Usage.Reset()
	}
}

func (m *Manager) GetInstance(featureID string) *Instance {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.instances[featureID]
}

func (m *Manager) StopInstance(featureID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if inst, ok := m.instances[featureID]; ok {
		inst.Stop()
	}
}

func (m *Manager) ClearInstance(featureID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.instances, featureID)
}

func (m *Manager) StopAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, inst := range m.instances {
		inst.Stop()
	}
}

func (m *Manager) GetRunningCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	count := 0
	for _, inst := range m.instances {
		if inst.GetStatus() == "running" {
			count++
		}
	}
	return count
}

func (m *Manager) GetAllStatuses() map[string]string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	statuses := make(map[string]string)
	for id, inst := range m.instances {
		statuses[id] = inst.GetStatus()
	}
	return statuses
}

func (m *Manager) CanStartMore() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.config.MaxConcurrent <= 0 {
		return true
	}

	running := 0
	for _, inst := range m.instances {
		if inst.GetStatus() == "running" {
			running++
		}
	}
	return running < m.config.MaxConcurrent
}

func (m *Manager) GetTotalUsage() usage.TokenUsage {
	m.mu.RLock()
	defer m.mu.RUnlock()

	total := usage.New()
	for _, inst := range m.instances {
		instUsage := inst.GetUsage()
		total.Add(&instUsage)
	}
	return total.Snapshot()
}

func (m *Manager) GetTotalCost() float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var totalCost float64
	for _, inst := range m.instances {
		totalCost += inst.GetEstimatedCost()
	}
	return totalCost
}

// SetGlobalBudget sets the global budget limits
func (m *Manager) SetGlobalBudget(tokens int64, usd float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.globalBudgetTokens = tokens
	m.globalBudgetUSD = usd
}

// GetGlobalBudget returns the global budget limits
func (m *Manager) GetGlobalBudget() (tokens int64, usd float64) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.globalBudgetTokens, m.globalBudgetUSD
}

// HasGlobalBudget returns true if a global budget is set
func (m *Manager) HasGlobalBudget() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.globalBudgetTokens > 0 || m.globalBudgetUSD > 0
}

// CheckGlobalBudget checks total usage against global budget
// Returns: percent used, at threshold (>=90%), over budget
func (m *Manager) CheckGlobalBudget() (percent float64, atThreshold bool, overBudget bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.globalBudgetTokens == 0 && m.globalBudgetUSD == 0 {
		return 0, false, false
	}

	total := usage.New()
	for _, inst := range m.instances {
		instUsage := inst.GetUsage()
		total.Add(&instUsage)
	}
	snapshot := total.Snapshot()

	if m.globalBudgetTokens > 0 {
		percent = float64(snapshot.TotalTokens) / float64(m.globalBudgetTokens) * 100
		atThreshold = percent >= 90
		overBudget = snapshot.TotalTokens >= m.globalBudgetTokens
		return
	}

	if m.globalBudgetUSD > 0 {
		var totalCost float64
		for _, inst := range m.instances {
			totalCost += inst.GetEstimatedCost()
		}
		percent = totalCost / m.globalBudgetUSD * 100
		atThreshold = percent >= 90
		overBudget = totalCost >= m.globalBudgetUSD
		return
	}

	return 0, false, false
}

// AcknowledgeBudget marks the budget as acknowledged (user chose to continue)
func (m *Manager) AcknowledgeBudget() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.budgetAcknowledged = true
}

// IsBudgetAcknowledged returns true if user has acknowledged budget warning
func (m *Manager) IsBudgetAcknowledged() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.budgetAcknowledged
}

// ResetBudgetAcknowledgement resets the acknowledgement flag
func (m *Manager) ResetBudgetAcknowledgement() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.budgetAcknowledged = false
}

// GetGlobalBudgetStatus returns a formatted budget status string
func (m *Manager) GetGlobalBudgetStatus() string {
	m.mu.RLock()
	tokens := m.globalBudgetTokens
	usd := m.globalBudgetUSD
	m.mu.RUnlock()

	if tokens == 0 && usd == 0 {
		return ""
	}

	total := m.GetTotalUsage()
	snapshot := total

	if usd > 0 {
		totalCost := m.GetTotalCost()
		percent := totalCost / usd * 100
		return fmt.Sprintf("$%.2f/$%.2f (%.0f%%)", totalCost, usd, percent)
	}

	if tokens > 0 {
		percent := float64(snapshot.TotalTokens) / float64(tokens) * 100
		return fmt.Sprintf("%s/%s (%.0f%%)", usage.FormatTokens(snapshot.TotalTokens), usage.FormatTokens(tokens), percent)
	}

	return ""
}
