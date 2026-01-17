package runner

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
	"time"
)

type Instance struct {
	mu        sync.RWMutex
	FeatureID string
	Model     string
	Status    string
	StartedAt time.Time
	cmd       *exec.Cmd
	cancel    context.CancelFunc
	output    []OutputLine
	outputCh  chan OutputLine
}

type OutputLine struct {
	Timestamp time.Time
	Type      string // assistant, user, tool_use, tool_result, system, error
	Content   string
	Raw       json.RawMessage
}

type StreamMessage struct {
	Type    string          `json:"type"`
	Message json.RawMessage `json:"message,omitempty"`
	Content string          `json:"content,omitempty"`
	Tool    string          `json:"tool,omitempty"`
	Result  string          `json:"result,omitempty"`
}

type Manager struct {
	mu        sync.RWMutex
	instances map[string]*Instance
	workDir   string
}

func NewManager(workDir string) *Manager {
	return &Manager{
		instances: make(map[string]*Instance),
		workDir:   workDir,
	}
}

func (m *Manager) StartInstance(featureID string, model string, prompt string) (*Instance, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if existing, ok := m.instances[featureID]; ok {
		if existing.Status == "running" {
			return nil, fmt.Errorf("instance for feature %s is already running", featureID)
		}
	}

	ctx, cancel := context.WithCancel(context.Background())

	inst := &Instance{
		FeatureID: featureID,
		Model:     model,
		Status:    "starting",
		StartedAt: time.Now(),
		cancel:    cancel,
		outputCh:  make(chan OutputLine, 100),
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
			switch msg.Type {
			case "assistant":
				outputLine.Content = msg.Content
			case "tool_use":
				outputLine.Content = fmt.Sprintf("[Tool: %s]", msg.Tool)
			case "tool_result":
				outputLine.Content = msg.Result
				if len(outputLine.Content) > 200 {
					outputLine.Content = outputLine.Content[:200] + "..."
				}
			default:
				outputLine.Content = line
			}
		} else {
			outputLine.Type = source
			outputLine.Content = line
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

func (inst *Instance) waitForCompletion() {
	err := inst.cmd.Wait()
	inst.mu.Lock()
	defer inst.mu.Unlock()

	if err != nil {
		inst.Status = "failed"
	} else {
		inst.Status = "completed"
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

func (inst *Instance) GetOutput() string {
	inst.mu.RLock()
	defer inst.mu.RUnlock()

	var sb strings.Builder
	for _, line := range inst.output {
		sb.WriteString(fmt.Sprintf("[%s] %s: %s\n",
			line.Timestamp.Format("15:04:05"),
			line.Type,
			line.Content,
		))
	}
	return sb.String()
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
