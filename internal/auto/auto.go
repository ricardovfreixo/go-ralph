package auto

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/vx/ralph-go/internal/manifest"
	"github.com/vx/ralph-go/internal/runner"
)

const (
	PRDDirName     = "PRD"
	ManifestFile   = "manifest.json"
	FeatureFile    = "feature.md"
	DefaultRetries = 3
)

type Result struct {
	FeatureID    string
	FeatureTitle string
	Status       string
	Duration     time.Duration
	Error        string
	NoWork       bool
	Blocked      []BlockedFeature
	Archived     bool
	ArchivePath  string
}

type BlockedFeature struct {
	ID               string
	Title            string
	PendingDeps      []string
	PendingDepTitles []string
}

func PRDDirExists() bool {
	cwd, err := os.Getwd()
	if err != nil {
		return false
	}
	prdDir := filepath.Join(cwd, PRDDirName)
	_, err = os.Stat(prdDir)
	return err == nil
}

func FindPRDDir() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get current directory: %w", err)
	}

	prdDir := filepath.Join(cwd, PRDDirName)
	if _, err := os.Stat(prdDir); os.IsNotExist(err) {
		return "", fmt.Errorf("PRD/ directory not found in current directory")
	}

	manifestPath := filepath.Join(prdDir, ManifestFile)
	if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
		return "", fmt.Errorf("manifest.json not found in PRD/ directory (run 'ralph init PRD.md' first)")
	}

	return prdDir, nil
}

func LoadManifest(prdDir string) (*manifest.Manifest, error) {
	return manifest.Load(prdDir)
}

func GetFeaturePrompt(prdDir string, feature *manifest.ManifestFeature) (string, error) {
	featurePath := filepath.Join(prdDir, feature.Dir, FeatureFile)
	content, err := os.ReadFile(featurePath)
	if err != nil {
		return "", fmt.Errorf("failed to read feature file %s: %w", featurePath, err)
	}
	return string(content), nil
}

func Run() (*Result, error) {
	prdDir, err := FindPRDDir()
	if err != nil {
		return nil, err
	}

	m, err := LoadManifest(prdDir)
	if err != nil {
		return nil, err
	}

	feature := m.GetNextRunnableFeature()
	if feature == nil {
		return handleNoRunnableFeature(m)
	}

	prompt, err := GetFeaturePrompt(prdDir, feature)
	if err != nil {
		return nil, err
	}

	result := &Result{
		FeatureID:    feature.ID,
		FeatureTitle: feature.Title,
	}

	startTime := time.Now()

	if err := m.UpdateFeatureStatus(feature.ID, "running"); err != nil {
		return nil, fmt.Errorf("failed to update feature status: %w", err)
	}
	if err := m.Save(); err != nil {
		return nil, fmt.Errorf("failed to save manifest: %w", err)
	}

	workDir, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get working directory: %w", err)
	}

	runnerMgr := runner.NewManagerWithConfig(workDir, runner.Config{
		MaxRetries:    DefaultRetries,
		MaxConcurrent: 1,
	})

	instance, err := runnerMgr.StartInstance(feature.ID, feature.Model, prompt)
	if err != nil {
		result.Status = "failed"
		result.Error = err.Error()
		result.Duration = time.Since(startTime)

		_ = m.UpdateFeatureStatus(feature.ID, "failed")
		_ = m.Save()

		return result, nil
	}

	for {
		status := instance.GetStatus()
		if status == "completed" || status == "failed" {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	result.Status = instance.GetStatus()
	result.Duration = time.Since(startTime)
	if result.Status == "failed" {
		result.Error = instance.GetError()
	}

	if err := m.UpdateFeatureStatus(feature.ID, result.Status); err != nil {
		return nil, fmt.Errorf("failed to update feature status: %w", err)
	}
	if err := m.Save(); err != nil {
		return nil, fmt.Errorf("failed to save manifest: %w", err)
	}

	if result.Status == "completed" {
		if archived, archivePath := checkAndArchivePRD(prdDir, m); archived {
			result.Archived = true
			result.ArchivePath = archivePath
		}
	}

	return result, nil
}

func checkAndArchivePRD(prdDir string, m *manifest.Manifest) (bool, string) {
	total, completed, _, _, _, _ := m.GetSummary()
	if completed != total {
		return false, ""
	}

	sourcePRD := m.Source
	if sourcePRD == "" {
		return false, ""
	}

	if strings.Contains(sourcePRD, "_authored.md") {
		return false, ""
	}

	workDir := filepath.Dir(prdDir)
	sourcePath := filepath.Join(workDir, sourcePRD)

	if _, err := os.Stat(sourcePath); os.IsNotExist(err) {
		return false, ""
	}

	baseName := strings.TrimSuffix(sourcePRD, filepath.Ext(sourcePRD))
	archivedName := baseName + "_authored.md"
	archivePath := filepath.Join(prdDir, archivedName)

	if err := os.Rename(sourcePath, archivePath); err != nil {
		return false, ""
	}

	m.Source = archivedName
	if err := m.Save(); err != nil {
		os.Rename(archivePath, sourcePath)
		return false, ""
	}

	return true, archivePath
}

func handleNoRunnableFeature(m *manifest.Manifest) (*Result, error) {
	total, completed, running, failed, pending, blocked := m.GetSummary()

	result := &Result{
		NoWork: true,
	}

	if completed == total {
		result.Status = "all_completed"
		return result, nil
	}

	if running > 0 {
		result.Status = "running_elsewhere"
		return result, nil
	}

	if failed > 0 && pending == 0 && blocked > 0 {
		result.Status = "blocked_by_failures"
		result.Blocked = getBlockedFeatures(m)
		return result, nil
	}

	if blocked > 0 && pending == 0 {
		result.Status = "all_blocked"
		result.Blocked = getBlockedFeatures(m)
		return result, nil
	}

	result.Status = "unknown"
	return result, nil
}

func getBlockedFeatures(m *manifest.Manifest) []BlockedFeature {
	blocked := m.GetBlockedFeatures()
	result := make([]BlockedFeature, 0, len(blocked))

	for _, f := range blocked {
		bf := BlockedFeature{
			ID:          f.ID,
			Title:       f.Title,
			PendingDeps: m.GetPendingDependencies(f.ID),
		}

		for _, depID := range bf.PendingDeps {
			if dep := m.GetFeature(depID); dep != nil {
				bf.PendingDepTitles = append(bf.PendingDepTitles, fmt.Sprintf("%s (%s)", dep.Title, dep.Status))
			}
		}

		result = append(result, bf)
	}

	return result
}

func PrintSummary(result *Result) {
	if result.NoWork {
		printNoWorkSummary(result)
		return
	}

	fmt.Printf("\n")
	fmt.Printf("Feature: %s - %s\n", result.FeatureID, result.FeatureTitle)
	fmt.Printf("Status:  %s\n", result.Status)
	fmt.Printf("Duration: %s\n", result.Duration.Round(time.Second))
	if result.Error != "" {
		fmt.Printf("Error:   %s\n", result.Error)
	}
	if result.Archived {
		fmt.Printf("\nAll features completed. PRD archived to: %s\n", result.ArchivePath)
	}
	fmt.Printf("\n")
}

func printNoWorkSummary(result *Result) {
	fmt.Printf("\n")
	switch result.Status {
	case "all_completed":
		fmt.Println("All features completed.")
	case "running_elsewhere":
		fmt.Println("A feature is currently running in another process.")
	case "blocked_by_failures":
		fmt.Println("No runnable features. Some features are blocked by failed dependencies:")
		printBlockedFeatures(result.Blocked)
	case "all_blocked":
		fmt.Println("No runnable features. All pending features are blocked:")
		printBlockedFeatures(result.Blocked)
	default:
		fmt.Println("No runnable features found.")
	}
	fmt.Printf("\n")
}

func printBlockedFeatures(blocked []BlockedFeature) {
	for _, bf := range blocked {
		fmt.Printf("  - %s (%s): waiting on %v\n", bf.Title, bf.ID, bf.PendingDepTitles)
	}
}

func ExitCode(result *Result) int {
	if result.NoWork {
		return 0
	}
	if result.Status == "completed" {
		return 0
	}
	return 1
}
