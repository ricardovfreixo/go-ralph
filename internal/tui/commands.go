package tui

import (
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/vx/ralph-go/internal/parser"
	"github.com/vx/ralph-go/internal/runner"
	"github.com/vx/ralph-go/internal/state"
)

type prdLoadedMsg struct {
	prd *parser.PRD
	err error
}

type stateLoadedMsg struct {
	state *state.Progress
	err   error
}

type instanceStartedMsg struct {
	featureID string
	instance  *runner.Instance
	err       error
}

type instanceOutputMsg struct {
	featureID string
	line      runner.OutputLine
	done      bool
}

type instanceDoneMsg struct {
	featureID string
	status    string
}

type tickMsg struct{}

type statusClearMsg struct{}

func loadPRD(path string) tea.Cmd {
	return func() tea.Msg {
		prd, err := parser.ParsePRD(path)
		return prdLoadedMsg{prd: prd, err: err}
	}
}

func loadState(prdPath string) tea.Cmd {
	return func() tea.Msg {
		progress, err := state.LoadProgress(prdPath)
		return stateLoadedMsg{state: progress, err: err}
	}
}

func startFeature(feature parser.Feature, context string, workDir string, mgr *runner.Manager) tea.Cmd {
	return func() tea.Msg {
		progressContent := readProgressMD(workDir)
		prompt := feature.ToPromptWithProgress(context, progressContent)
		instance, err := mgr.StartInstance(feature.ID, feature.Model, prompt)
		if err != nil {
			return instanceStartedMsg{
				featureID: feature.ID,
				err:       err,
			}
		}
		return instanceStartedMsg{
			featureID: feature.ID,
			instance:  instance,
		}
	}
}

func readProgressMD(workDir string) string {
	path := filepath.Join(workDir, "progress.md")
	content, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(content)
}

func listenForOutput(featureID string, instance *runner.Instance) tea.Cmd {
	if instance == nil {
		return nil
	}

	return func() tea.Msg {
		ch := instance.OutputChannel()
		line, ok := <-ch
		if !ok {
			return instanceDoneMsg{
				featureID: featureID,
				status:    instance.GetStatus(),
			}
		}
		return instanceOutputMsg{
			featureID: featureID,
			line:      line,
		}
	}
}
