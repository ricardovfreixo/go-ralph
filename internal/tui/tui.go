package tui

import (
	"fmt"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

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
			return m, nil
		}
		m.prd = msg.prd
		return m, nil
	case stateLoadedMsg:
		if msg.err != nil {
			m.state = state.NewProgress()
		} else {
			m.state = msg.state
		}
		m.state.SetPath(m.prdPath)
		return m, nil
	case instanceStartedMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.state.UpdateFeature(msg.featureID, "running")
		return m, listenForOutput(msg.featureID, msg.instance)
	case instanceOutputMsg:
		return m, listenForOutput(msg.featureID, m.manager.GetInstance(msg.featureID))
	case instanceDoneMsg:
		m.state.UpdateFeature(msg.featureID, msg.status)
		m.state.Save()
		return m, nil
	}
	return m, nil
}

func (m Model) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.confirmQuit {
		switch msg.String() {
		case "y", "Y":
			m.quitting = true
			m.manager.StopAll()
			return m, tea.Quit
		case "n", "N", "esc":
			m.confirmQuit = false
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
			status := "pending"
			if m.state != nil {
				if fs := m.state.GetFeature(feature.ID); fs != nil {
					status = fs.Status
				}
			}
			if status == "running" {
				return m, nil
			}
			return m, startFeature(feature, m.prd.Context, m.manager)
		}
	case "r":
		if m.prd != nil && len(m.prd.Features) > 0 {
			feature := m.prd.Features[m.selected]
			status := "pending"
			if m.state != nil {
				if fs := m.state.GetFeature(feature.ID); fs != nil {
					status = fs.Status
				}
			}
			if status == "failed" || status == "completed" {
				m.manager.StopInstance(feature.ID)
				return m, startFeature(feature, m.prd.Context, m.manager)
			}
		}
	case "x":
		if m.prd != nil && len(m.prd.Features) > 0 {
			feature := m.prd.Features[m.selected]
			m.manager.StopInstance(feature.ID)
			m.state.UpdateFeature(feature.ID, "stopped")
		}
	case "?":
		m.currentView = viewHelp
	}
	return m, nil
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

	s := titleStyle.Render("ralph-go") + "\n"
	s += headerStyle.Render(m.prd.Title) + "\n\n"

	for i, feature := range m.prd.Features {
		status := "pending"
		if m.state != nil {
			if fs := m.state.GetFeature(feature.ID); fs != nil {
				status = fs.Status
			}
		}

		line := fmt.Sprintf(" %s  %s", statusStyle(status).Render(statusIcon(status)), feature.Title)

		if i == m.selected {
			s += selectedStyle.Render(line) + "\n"
		} else {
			s += normalStyle.Render(line) + "\n"
		}
	}

	if m.confirmQuit {
		s += "\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Render("Quit? (y/n)")
	} else {
		s += "\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Render("j/k: navigate • enter: inspect • s: start • r: retry • x: stop • ?: help • q: quit")
	}

	return s
}

func (m Model) renderInspectView() string {
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("205"))

	var featureTitle string
	for _, f := range m.prd.Features {
		if f.ID == m.inspecting {
			featureTitle = f.Title
			break
		}
	}

	s := titleStyle.Render("Instance Output: "+featureTitle) + "\n\n"

	if inst := m.manager.GetInstance(m.inspecting); inst != nil {
		output := inst.GetOutput()
		if output == "" {
			s += "Waiting for output...\n"
		} else {
			lines := splitLines(output)
			maxLines := m.height - 6
			if maxLines < 5 {
				maxLines = 5
			}
			if len(lines) > maxLines {
				if m.scrollOffset > len(lines)-maxLines {
					m.scrollOffset = len(lines) - maxLines
				}
				start := m.scrollOffset
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
		s += "No output yet. Press 's' on main view to start this feature.\n"
	}

	s += "\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Render("q/esc: back • j/k: scroll")
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
  r             Retry failed/completed feature
  x             Stop running feature

General:
  ?             Toggle help
  q             Quit (stops all instances)

`
	s += lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Render("Press q or ? to close")
	return s
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
	p := tea.NewProgram(initialModel(prdPath), tea.WithAltScreen())
	_, err := p.Run()
	return err
}
