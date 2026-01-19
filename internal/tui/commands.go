package tui

import (
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/vx/ralph-go/internal/manifest"
	"github.com/vx/ralph-go/internal/parser"
	"github.com/vx/ralph-go/internal/rlm"
	"github.com/vx/ralph-go/internal/runner"
	"github.com/vx/ralph-go/internal/state"
)

type prdLoadedMsg struct {
	prd *parser.PRD
	err error
}

type manifestLoadedMsg struct {
	manifest *manifest.Manifest
	prd      *parser.PRD // Synthetic PRD from manifest
	err      error
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

type spawnRequestMsg struct {
	parentID string
	request  *rlm.SpawnRequest
}

type spawnStartedMsg struct {
	parentID   string
	childID    string
	childTitle string
	instance   *runner.Instance
	err        error
}

type childCompletedMsg struct {
	childID    string
	parentID   string
	status     string
	summary    string
	tokenUsage int64
}

type modelChangedMsg struct {
	featureID string
	fromModel string
	toModel   string
	reason    string
	details   string
}

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

func loadManifest(prdDir string) tea.Cmd {
	return func() tea.Msg {
		m, err := manifest.Load(prdDir)
		if err != nil {
			return manifestLoadedMsg{err: err}
		}
		// Create synthetic PRD from manifest
		prd := manifestToPRD(m, prdDir)
		return manifestLoadedMsg{manifest: m, prd: prd}
	}
}

func loadStateFromDir(prdDir string) tea.Cmd {
	return func() tea.Msg {
		// Load state from PRD/progress.json
		progressPath := filepath.Join(prdDir, "progress.json")
		progress, err := state.LoadProgressFromPath(progressPath)
		if err != nil {
			// Create new progress if doesn't exist
			progress = state.NewProgress()
		}
		progress.SetPathDirect(progressPath)
		return stateLoadedMsg{state: progress, err: nil}
	}
}

// manifestToPRD converts a manifest to a synthetic PRD for TUI compatibility
func manifestToPRD(m *manifest.Manifest, prdDir string) *parser.PRD {
	prd := &parser.PRD{
		Title:        m.Title,
		Context:      "", // Context will be read from feature.md files
		BudgetTokens: m.BudgetTokens,
		BudgetUSD:    m.BudgetUSD,
	}

	for _, mf := range m.Features {
		// Read feature content from feature.md
		featurePath := filepath.Join(prdDir, mf.Dir, "feature.md")
		content, _ := os.ReadFile(featurePath)

		feature := parser.Feature{
			ID:            mf.ID,
			Title:         mf.Title,
			Description:   string(content),
			ExecutionMode: mf.Execution,
			Model:         mf.Model,
			DependsOn:     mf.DependsOn,
		}
		prd.Features = append(prd.Features, feature)
	}

	return prd
}

func startFeature(feature parser.Feature, context string, workDir string, mgr *runner.Manager) tea.Cmd {
	return func() tea.Msg {
		progressContent := readProgressMD(workDir)
		prompt := feature.ToPromptWithProgress(context, progressContent)
		opts := runner.StartInstanceOptions{
			IsLeafTask: len(feature.Tasks) <= 2,
			TaskCount:  len(feature.Tasks),
		}
		instance, err := mgr.StartInstanceWithOptions(feature.ID, feature.Model, prompt, opts)
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

func startFeatureWithBudget(feature parser.Feature, context string, workDir string, mgr *runner.Manager) tea.Cmd {
	return func() tea.Msg {
		progressContent := readProgressMD(workDir)
		prompt := feature.ToPromptWithProgress(context, progressContent)
		opts := runner.StartInstanceOptions{
			IsLeafTask: len(feature.Tasks) <= 2,
			TaskCount:  len(feature.Tasks),
		}
		instance, err := mgr.StartInstanceWithOptions(feature.ID, feature.Model, prompt, opts)
		if err != nil {
			return instanceStartedMsg{
				featureID: feature.ID,
				err:       err,
			}
		}
		// Set feature-level budget if specified
		if feature.BudgetTokens > 0 || feature.BudgetUSD > 0 {
			instance.SetBudget(feature.BudgetTokens, feature.BudgetUSD)
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

func startSpawnedChild(parentID string, child *rlm.RecursiveFeature, prompt string, workDir string, mgr *runner.Manager) tea.Cmd {
	return func() tea.Msg {
		instance, err := mgr.StartInstance(child.ID, child.Model, prompt)
		if err != nil {
			return spawnStartedMsg{
				parentID:   parentID,
				childID:    child.ID,
				childTitle: child.Title,
				err:        err,
			}
		}
		if child.ContextBudget > 0 {
			instance.SetBudget(int64(child.ContextBudget), 0)
		}
		return spawnStartedMsg{
			parentID:   parentID,
			childID:    child.ID,
			childTitle: child.Title,
			instance:   instance,
		}
	}
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
