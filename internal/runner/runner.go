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
)

type Instance struct {
	mu          sync.RWMutex
	FeatureID   string
	Model       string
	Status      string
	StartedAt   time.Time
	CompletedAt *time.Time
	ExitCode    int
	cmd         *exec.Cmd
	cancel      context.CancelFunc
	output      []OutputLine
	outputCh    chan OutputLine
	TestResults *TestResults
	Error       string
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
	Type       string          `json:"type"`
	Subtype    string          `json:"subtype,omitempty"`
	CostUSD    float64         `json:"cost_usd,omitempty"`
	Duration   float64         `json:"duration_ms,omitempty"`
	Message    json.RawMessage `json:"message,omitempty"`
	Content    string          `json:"content,omitempty"`
	Tool       string          `json:"tool,omitempty"`
	ToolInput  json.RawMessage `json:"tool_input,omitempty"`
	Result     string          `json:"result,omitempty"`
	IsError    bool            `json:"is_error,omitempty"`
	SessionID  string          `json:"session_id,omitempty"`
}

type Config struct {
	MaxRetries     int
	RetryDelay     time.Duration
	MaxConcurrent  int
}

func DefaultConfig() Config {
	return Config{
		MaxRetries:    3,
		RetryDelay:    5 * time.Second,
		MaxConcurrent: 3,
	}
}

type Manager struct {
	mu        sync.RWMutex
	instances map[string]*Instance
	workDir   string
	config    Config
}

func NewManager(workDir string) *Manager {
	return &Manager{
		instances: make(map[string]*Instance),
		workDir:   workDir,
		config:    DefaultConfig(),
	}
}

func NewManagerWithConfig(workDir string, config Config) *Manager {
	return &Manager{
		instances: make(map[string]*Instance),
		workDir:   workDir,
		config:    config,
	}
}

func (m *Manager) SetConfig(config Config) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.config = config
}

func (m *Manager) StartInstance(featureID string, model string, prompt string) (*Instance, error) {
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

	inst := &Instance{
		FeatureID:   featureID,
		Model:       model,
		Status:      "starting",
		StartedAt:   time.Now(),
		cancel:      cancel,
		outputCh:    make(chan OutputLine, 100),
		TestResults: &TestResults{},
	}

	args := []string{
		"--dangerously-skip-permissions",
		"-p",
		"--verbose",
		"--output-format", "stream-json",
	}

	if model != "" && model != "sonnet" {
		args = append(args, "--model", model)
	}

	args = append(args, prompt)

	inst.cmd = exec.CommandContext(ctx, "claude", args...)
	inst.cmd.Dir = m.workDir

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
		return nil, fmt.Errorf("failed to start claude: %w", err)
	}

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

			switch msg.Type {
			case "assistant":
				outputLine.Content = msg.Content
				inst.detectTestResults(msg.Content)
			case "user":
				outputLine.Content = msg.Content
			case "system":
				outputLine.Content = msg.Content
				outputLine.Subtype = msg.Subtype
			case "tool_use":
				outputLine.Tool = msg.Tool
				outputLine.Content = fmt.Sprintf("[Tool: %s]", msg.Tool)
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
	err := inst.cmd.Wait()
	inst.mu.Lock()
	defer inst.mu.Unlock()

	now := time.Now()
	inst.CompletedAt = &now

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			inst.ExitCode = exitErr.ExitCode()
		}
		inst.Status = "failed"
		if inst.Error == "" {
			inst.Error = err.Error()
		}
	} else {
		inst.ExitCode = 0
		if inst.TestResults.Failed > 0 {
			inst.Status = "failed"
			inst.Error = fmt.Sprintf("%d tests failed", inst.TestResults.Failed)
		} else {
			inst.Status = "completed"
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
