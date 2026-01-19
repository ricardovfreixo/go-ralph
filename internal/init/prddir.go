package init

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/vx/ralph-go/internal/manifest"
	"github.com/vx/ralph-go/internal/parser"
)

func InitFromPRD(prdPath string, force bool) error {
	prd, err := parser.ParsePRD(prdPath)
	if err != nil {
		return fmt.Errorf("failed to parse PRD: %w", err)
	}

	if len(prd.Features) == 0 {
		return fmt.Errorf("no features found in PRD file")
	}

	prdDir := filepath.Dir(prdPath)
	outputDir := filepath.Join(prdDir, "PRD")

	if _, err := os.Stat(outputDir); err == nil {
		if !force {
			return fmt.Errorf("PRD/ directory already exists (use --force to overwrite)")
		}
		if err := os.RemoveAll(outputDir); err != nil {
			return fmt.Errorf("failed to remove existing PRD/ directory: %w", err)
		}
	}

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create PRD/ directory: %w", err)
	}
	fmt.Println("  Created PRD/")

	globalContext := buildGlobalContext(prd)

	for i, feature := range prd.Features {
		dirName := fmt.Sprintf("%02d-%s", i+1, sanitizeDirName(feature.Title))
		featureDir := filepath.Join(outputDir, dirName)

		if err := os.MkdirAll(featureDir, 0755); err != nil {
			return fmt.Errorf("failed to create feature directory %s: %w", dirName, err)
		}

		featureContent := buildFeatureMD(globalContext, feature)
		featurePath := filepath.Join(featureDir, "feature.md")

		if err := os.WriteFile(featurePath, []byte(featureContent), 0644); err != nil {
			return fmt.Errorf("failed to write feature.md for %s: %w", feature.Title, err)
		}

		fmt.Printf("  Created %s/feature.md\n", dirName)
	}

	m, err := manifest.GenerateFromPRD(prd, prdPath)
	if err != nil {
		return fmt.Errorf("failed to generate manifest: %w", err)
	}

	m.ResolveDependencies()

	removed := m.RemoveMissingDependencies()
	for _, warning := range removed {
		fmt.Printf("  Warning: %s\n", warning)
	}

	warnings, cycleErr := m.ValidateDependencies()
	for _, warning := range warnings {
		fmt.Printf("  Warning: %s\n", warning)
	}
	if cycleErr != nil {
		if err := os.RemoveAll(outputDir); err != nil {
			fmt.Printf("  Warning: failed to clean up PRD/ directory: %v\n", err)
		}
		return fmt.Errorf("dependency validation failed: %w", cycleErr)
	}

	manifestPath := filepath.Join(outputDir, "manifest.json")
	m.SetPath(manifestPath)
	if err := m.Save(); err != nil {
		return fmt.Errorf("failed to save manifest: %w", err)
	}
	fmt.Println("  Created manifest.json")

	gitignorePath := filepath.Join(prdDir, ".gitignore")
	if err := appendToGitignore(gitignorePath, "PRD/"); err != nil {
		fmt.Printf("  Warning: could not update .gitignore: %v\n", err)
	} else {
		fmt.Println("  Added PRD/ to .gitignore")
	}

	printSummary(m)

	return nil
}

func printSummary(m *manifest.Manifest) {
	features := m.AllFeatures()
	fmt.Println()
	fmt.Printf("Summary: %d features\n", len(features))

	depsCount := 0
	for _, f := range features {
		depsCount += len(f.DependsOn)
	}

	if depsCount > 0 {
		fmt.Printf("Dependencies: %d total\n", depsCount)
		for _, f := range features {
			if len(f.DependsOn) > 0 {
				fmt.Printf("  %s → %v\n", f.ID, f.DependsOn)
			}
		}
	} else {
		fmt.Println("Dependencies: none")
	}
}

func sanitizeDirName(title string) string {
	title = strings.ToLower(title)

	title = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			return r
		}
		if r == ' ' || r == ':' {
			return '-'
		}
		return -1
	}, title)

	for strings.Contains(title, "--") {
		title = strings.ReplaceAll(title, "--", "-")
	}
	title = strings.Trim(title, "-")

	if len(title) > 50 {
		title = title[:50]
		title = strings.TrimRight(title, "-")
	}

	return title
}

func buildGlobalContext(prd *parser.PRD) string {
	var sb strings.Builder

	sb.WriteString("# ")
	sb.WriteString(prd.Title)
	sb.WriteString("\n\n")

	if prd.Context != "" {
		context := cleanupSeparators(prd.Context)
		sb.WriteString(context)
		sb.WriteString("\n")
	}

	return sb.String()
}

func cleanupSeparators(text string) string {
	lines := strings.Split(text, "\n")
	var result []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "---" {
			continue
		}
		result = append(result, line)
	}
	text = strings.Join(result, "\n")
	for strings.Contains(text, "\n\n\n") {
		text = strings.ReplaceAll(text, "\n\n\n", "\n\n")
	}
	return strings.TrimSpace(text)
}

func buildFeatureMD(globalContext string, feature parser.Feature) string {
	var sb strings.Builder

	sb.WriteString(globalContext)
	sb.WriteString("\n---\n\n")

	sb.WriteString("## ")
	sb.WriteString(feature.Title)
	sb.WriteString("\n\n")

	if feature.Description != "" {
		description := cleanupSeparators(feature.Description)
		if description != "" {
			sb.WriteString(description)
			sb.WriteString("\n\n")
		}
	}

	sb.WriteString("Execution: ")
	sb.WriteString(feature.ExecutionMode)
	sb.WriteString("\n")

	sb.WriteString("Model: ")
	sb.WriteString(feature.Model)
	sb.WriteString("\n\n")

	if len(feature.Tasks) > 0 {
		for _, task := range feature.Tasks {
			checkbox := "[ ]"
			if task.Completed {
				checkbox = "[x]"
			}
			sb.WriteString(fmt.Sprintf("- %s %s\n", checkbox, task.Description))
		}
		sb.WriteString("\n")
	}

	if len(feature.AcceptanceCriteria) > 0 {
		for _, criteria := range feature.AcceptanceCriteria {
			sb.WriteString("Acceptance: ")
			sb.WriteString(strings.TrimSpace(criteria))
			sb.WriteString("\n")
		}
	}

	return sb.String()
}
