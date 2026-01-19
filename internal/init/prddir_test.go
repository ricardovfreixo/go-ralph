package init

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSanitizeDirName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Feature 1: User Auth", "feature-1-user-auth"},
		{"Simple Feature", "simple-feature"},
		{"Feature with CAPS", "feature-with-caps"},
		{"Feature--with--dashes", "feature-with-dashes"},
		{"Feature_with_underscores", "feature_with_underscores"},
		{"Feature  with   spaces", "feature-with-spaces"},
		{"Feature: with: colons", "feature-with-colons"},
		{"Feature!@#$%Special", "featurespecial"},
		{"-Leading and trailing-", "leading-and-trailing"},
		{"A very long feature name that exceeds the fifty character limit and should be truncated", "a-very-long-feature-name-that-exceeds-the-fifty-ch"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := sanitizeDirName(tt.input)
			if result != tt.expected {
				t.Errorf("sanitizeDirName(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestInitFromPRD(t *testing.T) {
	tempDir := t.TempDir()

	prdContent := `# Test Project

This is the project context with technology stack info.

- Language: Go
- Framework: BubbleTea

---

## Feature 1: Setup

Initialize the project structure.

Execution: sequential
Model: haiku

- [ ] Create directory structure
- [ ] Add config files

Acceptance: Project builds

---

## Feature 2: Core API

Build the REST API.

Execution: parallel
Model: sonnet

- [ ] Implement endpoints
- [ ] Add middleware
- [x] Design schema

Acceptance: All endpoints respond
Acceptance: Tests pass
`

	prdPath := filepath.Join(tempDir, "PRD.md")
	if err := os.WriteFile(prdPath, []byte(prdContent), 0644); err != nil {
		t.Fatalf("failed to write test PRD: %v", err)
	}

	if err := InitFromPRD(prdPath, false); err != nil {
		t.Fatalf("InitFromPRD failed: %v", err)
	}

	prdDir := filepath.Join(tempDir, "PRD")
	if _, err := os.Stat(prdDir); os.IsNotExist(err) {
		t.Fatal("PRD/ directory was not created")
	}

	feature1Dir := filepath.Join(prdDir, "01-feature-1-setup")
	if _, err := os.Stat(feature1Dir); os.IsNotExist(err) {
		t.Fatal("01-feature-1-setup/ directory was not created")
	}

	feature2Dir := filepath.Join(prdDir, "02-feature-2-core-api")
	if _, err := os.Stat(feature2Dir); os.IsNotExist(err) {
		t.Fatal("02-feature-2-core-api/ directory was not created")
	}

	feature1MD := filepath.Join(feature1Dir, "feature.md")
	content1, err := os.ReadFile(feature1MD)
	if err != nil {
		t.Fatalf("failed to read feature.md: %v", err)
	}
	content1Str := string(content1)

	if !strings.Contains(content1Str, "# Test Project") {
		t.Error("feature.md should contain global context title")
	}
	if !strings.Contains(content1Str, "This is the project context") {
		t.Error("feature.md should contain global context description")
	}
	if !strings.Contains(content1Str, "## Feature 1: Setup") {
		t.Error("feature.md should contain feature title")
	}
	if !strings.Contains(content1Str, "- [ ] Create directory structure") {
		t.Error("feature.md should contain tasks")
	}
	if !strings.Contains(content1Str, "Execution: sequential") {
		t.Error("feature.md should contain execution mode")
	}
	if !strings.Contains(content1Str, "Model: haiku") {
		t.Error("feature.md should contain model")
	}
	if !strings.Contains(content1Str, "Acceptance: Project builds") {
		t.Error("feature.md should contain acceptance criteria")
	}

	feature2MD := filepath.Join(feature2Dir, "feature.md")
	content2, err := os.ReadFile(feature2MD)
	if err != nil {
		t.Fatalf("failed to read feature.md: %v", err)
	}
	content2Str := string(content2)

	if !strings.Contains(content2Str, "# Test Project") {
		t.Error("feature 2 should contain global context")
	}
	if !strings.Contains(content2Str, "Execution: parallel") {
		t.Error("feature 2 should have parallel execution mode")
	}
	if !strings.Contains(content2Str, "Model: sonnet") {
		t.Error("feature 2 should have sonnet model")
	}
	if !strings.Contains(content2Str, "- [x] Design schema") {
		t.Error("feature 2 should preserve completed task checkbox")
	}

	gitignorePath := filepath.Join(tempDir, ".gitignore")
	gitignoreContent, err := os.ReadFile(gitignorePath)
	if err != nil {
		t.Fatalf("failed to read .gitignore: %v", err)
	}
	if !strings.Contains(string(gitignoreContent), "PRD/") {
		t.Error(".gitignore should contain PRD/")
	}
}

func TestInitFromPRDExistingDir(t *testing.T) {
	tempDir := t.TempDir()

	prdContent := `# Test

## Feature 1

Execution: sequential
Model: sonnet

- [ ] Task
`

	prdPath := filepath.Join(tempDir, "PRD.md")
	if err := os.WriteFile(prdPath, []byte(prdContent), 0644); err != nil {
		t.Fatalf("failed to write test PRD: %v", err)
	}

	prdDir := filepath.Join(tempDir, "PRD")
	if err := os.MkdirAll(prdDir, 0755); err != nil {
		t.Fatalf("failed to create PRD dir: %v", err)
	}

	err := InitFromPRD(prdPath, false)
	if err == nil {
		t.Fatal("expected error when PRD/ already exists")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("expected 'already exists' error, got: %v", err)
	}

	if err := InitFromPRD(prdPath, true); err != nil {
		t.Fatalf("InitFromPRD with force failed: %v", err)
	}
}

func TestInitFromPRDNoFeatures(t *testing.T) {
	tempDir := t.TempDir()

	prdContent := `# Test Project

Just context, no features.
`

	prdPath := filepath.Join(tempDir, "PRD.md")
	if err := os.WriteFile(prdPath, []byte(prdContent), 0644); err != nil {
		t.Fatalf("failed to write test PRD: %v", err)
	}

	err := InitFromPRD(prdPath, false)
	if err == nil {
		t.Fatal("expected error when no features found")
	}
	if !strings.Contains(err.Error(), "no features found") {
		t.Errorf("expected 'no features found' error, got: %v", err)
	}
}

func TestInitFromPRDAppendGitignore(t *testing.T) {
	tempDir := t.TempDir()

	gitignorePath := filepath.Join(tempDir, ".gitignore")
	if err := os.WriteFile(gitignorePath, []byte("node_modules/\n.env\n"), 0644); err != nil {
		t.Fatalf("failed to write .gitignore: %v", err)
	}

	prdContent := `# Test

## Feature 1

- [ ] Task
`

	prdPath := filepath.Join(tempDir, "PRD.md")
	if err := os.WriteFile(prdPath, []byte(prdContent), 0644); err != nil {
		t.Fatalf("failed to write test PRD: %v", err)
	}

	if err := InitFromPRD(prdPath, false); err != nil {
		t.Fatalf("InitFromPRD failed: %v", err)
	}

	content, _ := os.ReadFile(gitignorePath)
	if !strings.Contains(string(content), "node_modules/") {
		t.Error("existing .gitignore content should be preserved")
	}
	if !strings.Contains(string(content), "PRD/") {
		t.Error("PRD/ should be added to .gitignore")
	}
}

func TestInitFromPRDGitignoreAlreadyHasPRD(t *testing.T) {
	tempDir := t.TempDir()

	gitignorePath := filepath.Join(tempDir, ".gitignore")
	if err := os.WriteFile(gitignorePath, []byte("PRD/\n.env\n"), 0644); err != nil {
		t.Fatalf("failed to write .gitignore: %v", err)
	}

	prdContent := `# Test

## Feature 1

- [ ] Task
`

	prdPath := filepath.Join(tempDir, "PRD.md")
	if err := os.WriteFile(prdPath, []byte(prdContent), 0644); err != nil {
		t.Fatalf("failed to write test PRD: %v", err)
	}

	if err := InitFromPRD(prdPath, false); err != nil {
		t.Fatalf("InitFromPRD failed: %v", err)
	}

	content, _ := os.ReadFile(gitignorePath)
	count := strings.Count(string(content), "PRD/")
	if count != 1 {
		t.Errorf("PRD/ should appear exactly once in .gitignore, found %d times", count)
	}
}

func TestBuildGlobalContext(t *testing.T) {
	prdContent := `# My Project

Some context about the project.

- Technology: Go
- Framework: BubbleTea

---

## Feature 1

- [ ] Task
`

	tempDir := t.TempDir()
	prdPath := filepath.Join(tempDir, "PRD.md")
	if err := os.WriteFile(prdPath, []byte(prdContent), 0644); err != nil {
		t.Fatalf("failed to write test PRD: %v", err)
	}

	if err := InitFromPRD(prdPath, false); err != nil {
		t.Fatalf("InitFromPRD failed: %v", err)
	}

	featurePath := filepath.Join(tempDir, "PRD", "01-feature-1", "feature.md")
	content, err := os.ReadFile(featurePath)
	if err != nil {
		t.Fatalf("failed to read feature.md: %v", err)
	}

	contentStr := string(content)
	if !strings.Contains(contentStr, "# My Project") {
		t.Error("should contain project title")
	}
	if !strings.Contains(contentStr, "Some context about the project") {
		t.Error("should contain project context")
	}
	if !strings.Contains(contentStr, "---") {
		t.Error("should contain separator between context and feature")
	}
	if !strings.Contains(contentStr, "## Feature 1") {
		t.Error("should contain feature header")
	}
}

func TestInitFromPRDCircularDependency(t *testing.T) {
	tempDir := t.TempDir()

	prdContent := `# Test Project

## Feature 1: First

Depends: 3

- [ ] Task 1

## Feature 2: Second

Depends: 1

- [ ] Task 2

## Feature 3: Third

Depends: 2

- [ ] Task 3
`

	prdPath := filepath.Join(tempDir, "PRD.md")
	if err := os.WriteFile(prdPath, []byte(prdContent), 0644); err != nil {
		t.Fatalf("failed to write test PRD: %v", err)
	}

	err := InitFromPRD(prdPath, false)
	if err == nil {
		t.Fatal("expected error for circular dependency")
	}
	if !strings.Contains(err.Error(), "circular dependency") {
		t.Errorf("expected circular dependency error, got: %v", err)
	}

	prdDir := filepath.Join(tempDir, "PRD")
	if _, err := os.Stat(prdDir); !os.IsNotExist(err) {
		t.Error("PRD/ directory should be cleaned up on circular dependency error")
	}
}

func TestInitFromPRDValidDependencies(t *testing.T) {
	tempDir := t.TempDir()

	prdContent := `# Test Project

## Feature 1: Setup

- [ ] Task 1

## Feature 2: Core

Depends: 1

- [ ] Task 2

## Feature 3: Final

Depends: 1, 2

- [ ] Task 3
`

	prdPath := filepath.Join(tempDir, "PRD.md")
	if err := os.WriteFile(prdPath, []byte(prdContent), 0644); err != nil {
		t.Fatalf("failed to write test PRD: %v", err)
	}

	if err := InitFromPRD(prdPath, false); err != nil {
		t.Fatalf("InitFromPRD failed for valid dependencies: %v", err)
	}

	manifestPath := filepath.Join(tempDir, "PRD", "manifest.json")
	if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
		t.Error("manifest.json should be created")
	}
}

func TestInitFromPRDMissingDependency(t *testing.T) {
	tempDir := t.TempDir()

	prdContent := `# Test Project

## Feature 1: Setup

- [ ] Task 1

## Feature 2: Core

Depends: 99

- [ ] Task 2
`

	prdPath := filepath.Join(tempDir, "PRD.md")
	if err := os.WriteFile(prdPath, []byte(prdContent), 0644); err != nil {
		t.Fatalf("failed to write test PRD: %v", err)
	}

	err := InitFromPRD(prdPath, false)
	if err != nil {
		t.Fatalf("expected success with warning for missing dep, got error: %v", err)
	}

	manifestPath := filepath.Join(tempDir, "PRD", "manifest.json")
	if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
		t.Error("manifest.json should be created even with missing dep warning")
	}
}
