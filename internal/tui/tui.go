package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/vx/ralph-go/internal/logger"
	"github.com/vx/ralph-go/internal/parser"
	"github.com/vx/ralph-go/internal/runner"
	"github.com/vx/ralph-go/internal/state"
)

type view int

const (
	viewMain view = iota
	viewInspect
	viewHelp
)

type Model struct {
	prdPath      string
	workDir      string
	prd          *parser.PRD
	state        *state.Progress
	manager      *runner.Manager
	currentView  view
	selected     int
	inspecting   string
	scrollOffset int
	width        int
	height       int
	err          error
	quitting     bool
	confirmQuit  bool
	confirmReset bool
	autoMode     bool
	statusMsg    string
	statusExpiry time.Time
}

func initialModel(prdPath string) Model {
	workDir := filepath.Dir(prdPath)
	return Model{
		prdPath:     prdPath,
		workDir:     workDir,
		manager:     runner.NewManager(workDir),
		currentView: viewMain,
	}
}

func (m Model) Init() tea.Cmd {
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
		return m, nil
	case prdLoadedMsg:
		if msg.err != nil {
			m.err = msg.err
			logger.Error("tui", "Failed to load PRD", "error", msg.err)
			return m, nil
		}
		m.prd = msg.prd
		logger.Info("tui", "PRD loaded", "title", m.prd.Title, "features", len(m.prd.Features))
		for _, f := range m.prd.Features {
			if m.state != nil {
				m.state.InitFeature(f.ID, f.Title)
			}
		}
		return m, nil
	case stateLoadedMsg:
		if msg.err != nil {
			m.state = state.NewProgress()
		} else {
			m.state = msg.state
		}
		m.state.SetPath(m.prdPath)
		m.manager.SetConfig(runner.Config{
			MaxRetries:    m.state.Config.MaxRetries,
			MaxConcurrent: m.state.Config.MaxConcurrent,
		})
		return m, nil
	case instanceStartedMsg:
		if msg.err != nil {
			logger.Error("tui", "Failed to start instance", "featureID", msg.featureID[:8], "error", msg.err)
			m.setStatus(fmt.Sprintf("Error: %v", msg.err))
			return m, nil
		}
		logger.Info("tui", "Instance started", "featureID", msg.featureID[:8])
		m.state.UpdateFeature(msg.featureID, "running")
		m.state.Save()
		return m, listenForOutput(msg.featureID, msg.instance)
	case instanceOutputMsg:
		inst := m.manager.GetInstance(msg.featureID)
		if inst != nil {
			return m, listenForOutput(msg.featureID, inst)
		}
		return m, nil
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
	inst := m.manager.GetInstance(msg.featureID)
	if inst != nil {
		testResults := inst.GetTestResults()
		m.state.SetTestResults(msg.featureID, testResults.Passed, testResults.Failed, testResults.Skipped, testResults.Output)

		if msg.status == "failed" {
			errMsg := inst.GetError()
			m.state.SetFeatureError(msg.featureID, errMsg)

			if m.autoMode && m.state.CanRetry(msg.featureID) {
				m.setStatus(fmt.Sprintf("Auto-retrying %s (attempt %d)", msg.featureID[:8], m.state.GetAttempts(msg.featureID)+1))
				m.manager.ClearInstance(msg.featureID)
				feature := m.findFeature(msg.featureID)
				if feature != nil {
					m.state.Save()
					return m, tea.Batch(
						startFeature(*feature, m.prd.Context, m.workDir, m.manager),
						tea.Tick(time.Second, func(t time.Time) tea.Msg { return tickMsg{} }),
					)
				}
			}
		} else {
			m.state.UpdateFeature(msg.featureID, msg.status)
		}
	} else {
		m.state.UpdateFeature(msg.featureID, msg.status)
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

func (m *Model) setStatus(msg string) {
	m.statusMsg = msg
	m.statusExpiry = time.Now().Add(5 * time.Second)
}

func (m Model) autoStartNext() (tea.Model, tea.Cmd) {
	if m.prd == nil || !m.autoMode {
		return m, nil
	}

	if !m.manager.CanStartMore() {
		return m, tea.Tick(time.Second, func(t time.Time) tea.Msg { return tickMsg{} })
	}

	for _, feature := range m.prd.Features {
		fs := m.state.GetFeature(feature.ID)
		if fs == nil || fs.Status == "pending" || fs.Status == "" {
			m.setStatus(fmt.Sprintf("Starting %s...", feature.Title))
			return m, tea.Batch(
				startFeature(feature, m.prd.Context, m.workDir, m.manager),
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
				startFeature(*feature, m.prd.Context, m.workDir, m.manager),
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
	if m.confirmQuit {
		switch msg.String() {
		case "y", "Y":
			m.quitting = true
			m.manager.StopAll()
			m.state.Save()
			return m, tea.Quit
		case "n", "N", "esc":
			m.confirmQuit = false
			return m, nil
		}
		return m, nil
	}

	if m.confirmReset {
		switch msg.String() {
		case "y", "Y":
			m.confirmReset = false
			m.autoMode = false
			m.manager.StopAll()
			m.state.ResetAll()
			m.state.Save()
			deleteProgressMD(m.workDir)
			m.setStatus("Reset all features and cleared progress.md")
			logger.Info("tui", "Reset all features and deleted progress.md")
			return m, nil
		case "n", "N", "esc":
			m.confirmReset = false
			return m, nil
		}
		return m, nil
	}

	switch m.currentView {
	case viewMain:
		return m.handleMainView(msg)
	case viewInspect:
		return m.handleInspectView(msg)
	case viewHelp:
		if msg.String() == "q" || msg.String() == "esc" || msg.String() == "?" {
			m.currentView = viewMain
		}
		return m, nil
	}
	return m, nil
}

func (m Model) handleMainView(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q":
		m.confirmQuit = true
		return m, nil
	case "j", "down":
		if m.prd != nil && m.selected < len(m.prd.Features)-1 {
			m.selected++
		}
	case "k", "up":
		if m.selected > 0 {
			m.selected--
		}
	case "enter":
		if m.prd != nil && len(m.prd.Features) > 0 {
			m.inspecting = m.prd.Features[m.selected].ID
			m.currentView = viewInspect
		}
	case "s":
		if m.prd != nil && len(m.prd.Features) > 0 {
			feature := m.prd.Features[m.selected]
			status := m.getFeatureStatus(feature.ID)
			if status == "running" {
				return m, nil
			}
			m.manager.ClearInstance(feature.ID)
			return m, startFeature(feature, m.prd.Context, m.workDir, m.manager)
		}
	case "S":
		if m.prd != nil && !m.autoMode {
			m.autoMode = true
			m.setStatus("Auto mode enabled - starting features...")
			return m, tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg { return tickMsg{} })
		}
	case "r":
		if m.prd != nil && len(m.prd.Features) > 0 {
			feature := m.prd.Features[m.selected]
			status := m.getFeatureStatus(feature.ID)
			if status == "failed" || status == "completed" || status == "stopped" {
				m.manager.ClearInstance(feature.ID)
				return m, startFeature(feature, m.prd.Context, m.workDir, m.manager)
			}
		}
	case "R":
		if m.prd != nil {
			feature := m.prd.Features[m.selected]
			m.state.ResetFeature(feature.ID)
			m.manager.ClearInstance(feature.ID)
			m.state.Save()
			m.setStatus(fmt.Sprintf("Reset %s", feature.Title))
		}
	case "x":
		if m.prd != nil && len(m.prd.Features) > 0 {
			feature := m.prd.Features[m.selected]
			m.manager.StopInstance(feature.ID)
			m.state.UpdateFeature(feature.ID, "stopped")
			m.state.Save()
		}
	case "X":
		m.autoMode = false
		m.manager.StopAll()
		m.setStatus("Stopped all instances")
	case "ctrl+r":
		m.confirmReset = true
	case "?":
		m.currentView = viewHelp
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
	case "j", "down":
		m.scrollOffset++
	case "k", "up":
		if m.scrollOffset > 0 {
			m.scrollOffset--
		}
	case "G":
		m.scrollOffset = 999999
	case "g":
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
			m.manager.StopInstance(m.inspecting)
			m.state.UpdateFeature(m.inspecting, "stopped")
			m.state.Save()
		}
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
	case viewHelp:
		return m.renderHelpView()
	default:
		return m.renderMainView()
	}
}

func (m Model) renderMainView() string {
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("205")).
		MarginBottom(1)

	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("39"))

	selectedStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("229")).
		Background(lipgloss.Color("57"))

	normalStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("252"))

	dimStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("244"))

	statusStyle := func(status string) lipgloss.Style {
		switch status {
		case "running":
			return lipgloss.NewStyle().Foreground(lipgloss.Color("226"))
		case "completed":
			return lipgloss.NewStyle().Foreground(lipgloss.Color("46"))
		case "failed":
			return lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
		case "stopped":
			return lipgloss.NewStyle().Foreground(lipgloss.Color("208"))
		default:
			return lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
		}
	}

	total, completed, running, failed, pending := m.state.GetSummary()
	summaryStr := fmt.Sprintf("[%d/%d done", completed, total)
	if running > 0 {
		summaryStr += fmt.Sprintf(", %d running", running)
	}
	if failed > 0 {
		summaryStr += fmt.Sprintf(", %d failed", failed)
	}
	if pending > 0 {
		summaryStr += fmt.Sprintf(", %d pending", pending)
	}
	summaryStr += "]"

	autoStr := ""
	if m.autoMode {
		autoStr = " [AUTO]"
	}

	s := titleStyle.Render("ralph-go v0.2.4"+autoStr) + " " + dimStyle.Render(summaryStr) + "\n"
	s += headerStyle.Render(m.prd.Title) + "\n\n"

	for i, feature := range m.prd.Features {
		status := "pending"
		attempts := 0
		if m.state != nil {
			if fs := m.state.GetFeature(feature.ID); fs != nil {
				status = fs.Status
				attempts = fs.Attempts
			}
		}

		icon := statusIcon(status)
		attemptStr := ""
		if attempts > 1 {
			attemptStr = fmt.Sprintf(" (attempt %d)", attempts)
		}

		line := fmt.Sprintf(" %s  %s%s", statusStyle(status).Render(icon), feature.Title, dimStyle.Render(attemptStr))

		if i == m.selected {
			s += selectedStyle.Render(line) + "\n"
		} else {
			s += normalStyle.Render(line) + "\n"
		}
	}

	if m.statusMsg != "" && time.Now().Before(m.statusExpiry) {
		s += "\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("226")).Render(m.statusMsg)
	}

	if m.confirmQuit {
		s += "\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Render("Quit? (y/n)")
	} else if m.confirmReset {
		s += "\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Render("Reset ALL features and delete progress.md? (y/n)")
	} else {
		s += "\n" + dimStyle.Render("s: start • S: start all • r: retry • R: reset • x: stop • X: stop all • ?: help • q: quit")
	}

	return s
}

func (m Model) renderInspectView() string {
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("205"))

	dimStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("244"))

	var featureTitle string
	var featureStatus string
	for _, f := range m.prd.Features {
		if f.ID == m.inspecting {
			featureTitle = f.Title
			featureStatus = m.getFeatureStatus(f.ID)
			break
		}
	}

	statusStr := fmt.Sprintf(" [%s]", featureStatus)
	s := titleStyle.Render("Instance Output: "+featureTitle) + dimStyle.Render(statusStr) + "\n\n"

	if inst := m.manager.GetInstance(m.inspecting); inst != nil {
		testResults := inst.GetTestResults()
		if testResults.Total > 0 {
			testStr := fmt.Sprintf("Tests: %d passed, %d failed", testResults.Passed, testResults.Failed)
			if testResults.Failed > 0 {
				s += lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Render(testStr) + "\n\n"
			} else {
				s += lipgloss.NewStyle().Foreground(lipgloss.Color("46")).Render(testStr) + "\n\n"
			}
		}

		output := inst.GetOutput()
		if output == "" {
			s += "Waiting for output...\n"
		} else {
			lines := splitLines(output)
			maxLines := m.height - 8
			if maxLines < 5 {
				maxLines = 5
			}
			scrollOffset := m.scrollOffset
			if len(lines) > maxLines {
				if scrollOffset > len(lines)-maxLines {
					scrollOffset = len(lines) - maxLines
				}
				start := scrollOffset
				end := start + maxLines
				if end > len(lines) {
					end = len(lines)
				}
				lines = lines[start:end]
			}
			for _, line := range lines {
				s += line + "\n"
			}
		}
	} else {
		s += "No output yet. Press 's' to start this feature.\n"
	}

	s += "\n" + dimStyle.Render("s: start • x: stop • j/k: scroll • g/G: top/bottom • q/esc: back")
	return s
}

func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

func (m Model) renderHelpView() string {
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("205"))

	s := titleStyle.Render("Help") + "\n\n"
	s += `Navigation:
  j/k or ↑/↓    Move selection up/down
  Enter         Inspect selected feature's output

Actions:
  s             Start selected feature
  S             Start ALL features (auto mode)
  r             Retry failed/completed feature
  R             Reset feature (clear attempts)
  x             Stop selected feature
  X             Stop ALL features (exit auto mode)
  Ctrl+r        Reset ALL features (start fresh)

General:
  ?             Toggle help
  q             Quit (saves progress)

Auto Mode:
  When started with 'S', ralph will:
  - Start features in order
  - Run up to 3 features in parallel
  - Auto-retry failed features (up to 3 attempts)
  - Stop when all complete or max retries exceeded

`
	s += lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Render("Press q or ? to close")
	return s
}

func deleteProgressMD(workDir string) {
	path := filepath.Join(workDir, "progress.md")
	os.Remove(path)
}

func statusIcon(status string) string {
	switch status {
	case "running":
		return "●"
	case "completed":
		return "✓"
	case "failed":
		return "✗"
	case "stopped":
		return "■"
	default:
		return "○"
	}
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
