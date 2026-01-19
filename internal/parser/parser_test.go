package parser

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParsePRDContent_BasicStructure(t *testing.T) {
	content := `# My Project

This is the project context.

## Feature 1: Setup

Initialize the project.

- [ ] Create directory
- [ ] Add config

Acceptance: Project builds

## Feature 2: Core

Build the core logic.

- [x] Design API
- [ ] Implement endpoints
`

	prd, err := ParsePRDContent(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if prd.Title != "My Project" {
		t.Errorf("expected title 'My Project', got %q", prd.Title)
	}

	if !strings.Contains(prd.Context, "project context") {
		t.Errorf("expected context to contain 'project context', got %q", prd.Context)
	}

	if len(prd.Features) != 2 {
		t.Fatalf("expected 2 features, got %d", len(prd.Features))
	}

	f1 := prd.Features[0]
	if f1.Title != "Feature 1: Setup" {
		t.Errorf("expected feature title 'Feature 1: Setup', got %q", f1.Title)
	}
	if len(f1.Tasks) != 2 {
		t.Errorf("expected 2 tasks for feature 1, got %d", len(f1.Tasks))
	}
	if f1.Tasks[0].Completed {
		t.Error("first task should not be completed")
	}

	f2 := prd.Features[1]
	if !f2.Tasks[0].Completed {
		t.Error("first task of feature 2 should be completed")
	}
}

func TestParsePRDContent_MetadataFields(t *testing.T) {
	content := `# Project

Context.

## Feature: Test Feature

Description.

Execution: parallel
Model: opus

- [ ] Task 1
`

	prd, err := ParsePRDContent(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(prd.Features) != 1 {
		t.Fatalf("expected 1 feature, got %d", len(prd.Features))
	}

	f := prd.Features[0]
	if f.ExecutionMode != "parallel" {
		t.Errorf("expected execution mode 'parallel', got %q", f.ExecutionMode)
	}
	if f.Model != "opus" {
		t.Errorf("expected model 'opus', got %q", f.Model)
	}
}

func TestParsePRDContent_DefaultValues(t *testing.T) {
	content := `# Project

## Feature: No Metadata

Just a feature without explicit metadata.

- [ ] Task 1
`

	prd, err := ParsePRDContent(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	f := prd.Features[0]
	if f.ExecutionMode != "sequential" {
		t.Errorf("expected default execution mode 'sequential', got %q", f.ExecutionMode)
	}
	if f.Model != "sonnet" {
		t.Errorf("expected default model 'sonnet', got %q", f.Model)
	}
}

func TestParsePRDContent_Dependencies(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected []string
	}{
		{
			name: "single numeric dependency",
			content: `# Project

## Feature 1

- [ ] Task

## Feature 2

Depends: 01

- [ ] Task
`,
			expected: []string{"01"},
		},
		{
			name: "multiple dependencies",
			content: `# Project

## Feature 1

- [ ] Task

## Feature 2

Depends: 01, 02, Feature 3

- [ ] Task
`,
			expected: []string{"01", "02", "Feature 3"},
		},
		{
			name: "case insensitive depends",
			content: `# Project

## Feature 1

- [ ] Task

## Feature 2

depends: 01

- [ ] Task
`,
			expected: []string{"01"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prd, err := ParsePRDContent(tt.content)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(prd.Features) < 2 {
				t.Fatalf("expected at least 2 features, got %d", len(prd.Features))
			}

			deps := prd.Features[1].DependsOn
			if len(deps) != len(tt.expected) {
				t.Fatalf("expected %d dependencies, got %d: %v", len(tt.expected), len(deps), deps)
			}

			for i, exp := range tt.expected {
				if deps[i] != exp {
					t.Errorf("dependency %d: expected %q, got %q", i, exp, deps[i])
				}
			}
		})
	}
}

func TestParsePRDContent_AcceptanceCriteria(t *testing.T) {
	content := `# Project

## Feature: With Criteria

Description.

- [ ] Task 1

Acceptance: First criterion
Criteria: Second criterion
Test: Third criterion
`

	prd, err := ParsePRDContent(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	f := prd.Features[0]
	if len(f.AcceptanceCriteria) != 3 {
		t.Errorf("expected 3 acceptance criteria, got %d", len(f.AcceptanceCriteria))
	}
}

func TestParsePRDContent_TaskCheckboxVariants(t *testing.T) {
	content := `# Project

## Feature: Tasks

- [ ] Uncompleted with dash
* [ ] Uncompleted with asterisk
- [x] Completed lowercase x
- [X] Completed uppercase X
`

	prd, err := ParsePRDContent(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	f := prd.Features[0]
	if len(f.Tasks) != 4 {
		t.Fatalf("expected 4 tasks, got %d", len(f.Tasks))
	}

	if f.Tasks[0].Completed || f.Tasks[1].Completed {
		t.Error("first two tasks should be uncompleted")
	}

	if !f.Tasks[2].Completed || !f.Tasks[3].Completed {
		t.Error("last two tasks should be completed")
	}
}

func TestParsePRDContent_ExecutionModeVariants(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected string
	}{
		{
			name: "execution parallel",
			content: `# Project

## Feature

Execution: parallel

- [ ] Task
`,
			expected: "parallel",
		},
		{
			name: "mode parallel",
			content: `# Project

## Feature

Mode: parallel

- [ ] Task
`,
			expected: "parallel",
		},
		{
			name: "run concurrent",
			content: `# Project

## Feature

Run: concurrent

- [ ] Task
`,
			expected: "parallel",
		},
		{
			name: "sequential",
			content: `# Project

## Feature

Execution: sequential

- [ ] Task
`,
			expected: "sequential",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prd, err := ParsePRDContent(tt.content)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if prd.Features[0].ExecutionMode != tt.expected {
				t.Errorf("expected execution mode %q, got %q", tt.expected, prd.Features[0].ExecutionMode)
			}
		})
	}
}

func TestParsePRDContent_ModelVariants(t *testing.T) {
	tests := []struct {
		name     string
		model    string
		expected string
	}{
		{"sonnet", "sonnet", "sonnet"},
		{"opus", "opus", "opus"},
		{"haiku", "haiku", "haiku"},
		{"auto", "auto", "auto"},
		{"Sonnet uppercase", "Sonnet", "sonnet"},
		{"Auto uppercase", "Auto", "auto"},
		{"invalid model", "invalid", "sonnet"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			content := `# Project

## Feature

Model: ` + tt.model + `

- [ ] Task
`
			prd, err := ParsePRDContent(content)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if prd.Features[0].Model != tt.expected {
				t.Errorf("expected model %q, got %q", tt.expected, prd.Features[0].Model)
			}
		})
	}
}

func TestParsePRDContent_RawContent(t *testing.T) {
	content := `# Project

Context here.

## Feature: My Feature

Description line 1.
Description line 2.

Execution: parallel
Model: opus

- [ ] Task 1
- [x] Task 2

Acceptance: Test passes
`

	prd, err := ParsePRDContent(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	f := prd.Features[0]
	if !strings.Contains(f.RawContent, "## Feature: My Feature") {
		t.Error("raw content should contain feature header")
	}
	if !strings.Contains(f.RawContent, "Execution: parallel") {
		t.Error("raw content should contain execution mode")
	}
	if !strings.Contains(f.RawContent, "- [ ] Task 1") {
		t.Error("raw content should contain tasks")
	}
}

func TestParsePRDContent_EmptyContent(t *testing.T) {
	prd, err := ParsePRDContent("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if prd.Title != "" {
		t.Errorf("expected empty title, got %q", prd.Title)
	}
	if len(prd.Features) != 0 {
		t.Errorf("expected 0 features, got %d", len(prd.Features))
	}
}

func TestParsePRDContent_NoFeatures(t *testing.T) {
	content := `# Project Title

Just some context without any features.
`

	prd, err := ParsePRDContent(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if prd.Title != "Project Title" {
		t.Errorf("expected title 'Project Title', got %q", prd.Title)
	}
	if len(prd.Features) != 0 {
		t.Errorf("expected 0 features, got %d", len(prd.Features))
	}
}

func TestParsePRD_FromFile(t *testing.T) {
	tmpDir := t.TempDir()
	prdPath := filepath.Join(tmpDir, "test.md")

	content := `# Test PRD

Context.

## Feature 1

- [ ] Task 1
`

	if err := os.WriteFile(prdPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	prd, err := ParsePRD(prdPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if prd.Title != "Test PRD" {
		t.Errorf("expected title 'Test PRD', got %q", prd.Title)
	}
}

func TestParsePRD_NonExistentFile(t *testing.T) {
	_, err := ParsePRD("/nonexistent/path.md")
	if err == nil {
		t.Error("expected error for non-existent file")
	}
}

func TestFeature_ToPrompt(t *testing.T) {
	f := &Feature{
		Title:       "Test Feature",
		Description: "This is a test feature.",
		Tasks: []Task{
			{Description: "Task 1", Completed: false},
			{Description: "Task 2", Completed: true},
		},
		AcceptanceCriteria: []string{"Tests pass", "Feature works"},
	}

	prompt := f.ToPrompt("Project context here.")

	if !strings.Contains(prompt, "# Project Context") {
		t.Error("prompt should contain project context header")
	}
	if !strings.Contains(prompt, "Project context here.") {
		t.Error("prompt should contain context content")
	}
	if !strings.Contains(prompt, "# Current Feature: Test Feature") {
		t.Error("prompt should contain feature header")
	}
	if !strings.Contains(prompt, "[ ] Task 1") {
		t.Error("prompt should contain uncompleted task")
	}
	if !strings.Contains(prompt, "[x] Task 2") {
		t.Error("prompt should contain completed task")
	}
	if !strings.Contains(prompt, "Tests pass") {
		t.Error("prompt should contain acceptance criteria")
	}
}

func TestFeature_ToPromptWithProgress(t *testing.T) {
	f := &Feature{
		Title:       "Test Feature",
		Description: "Description.",
		Tasks:       []Task{{Description: "Task", Completed: false}},
	}

	prompt := f.ToPromptWithProgress("Context.", "Progress notes here.")

	if !strings.Contains(prompt, "# Progress from Previous Features") {
		t.Error("prompt should contain progress header")
	}
	if !strings.Contains(prompt, "Progress notes here.") {
		t.Error("prompt should contain progress content")
	}
}

func TestFeature_ToPromptWithoutProgress(t *testing.T) {
	f := &Feature{
		Title:       "Test Feature",
		Description: "Description.",
		Tasks:       []Task{{Description: "Task", Completed: false}},
	}

	prompt := f.ToPromptWithProgress("Context.", "")

	if strings.Contains(prompt, "# Progress from Previous Features") {
		t.Error("prompt should not contain progress header when empty")
	}
}

func TestFeature_ToPromptInstructions(t *testing.T) {
	f := &Feature{
		Title:       "Test Feature",
		Description: "Description.",
	}

	prompt := f.ToPrompt("Context.")

	if !strings.Contains(prompt, "## Instructions") {
		t.Error("prompt should contain instructions header")
	}
	if !strings.Contains(prompt, "Update progress.md") {
		t.Error("prompt should mention progress.md")
	}
}

func TestGenerateID(t *testing.T) {
	id1 := generateID("Feature 1")
	id2 := generateID("Feature 2")
	id1Again := generateID("Feature 1")

	if id1 == id2 {
		t.Error("different titles should generate different IDs")
	}
	if id1 != id1Again {
		t.Error("same title should generate same ID")
	}
	if len(id1) != 16 {
		t.Errorf("expected ID length 16, got %d", len(id1))
	}
}

func TestParsePRDContent_MultipleAcceptanceCriteria(t *testing.T) {
	content := `# Project

## Feature

- [ ] Task

Acceptance: First criterion
Acceptance: Second criterion
Acceptance: Third criterion
`

	prd, err := ParsePRDContent(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(prd.Features[0].AcceptanceCriteria) != 3 {
		t.Errorf("expected 3 acceptance criteria, got %d", len(prd.Features[0].AcceptanceCriteria))
	}
}

func TestParsePRDContent_ContextPreserved(t *testing.T) {
	content := `# My Project

Technology Stack:
- Language: Go
- Framework: BubbleTea
- Database: SQLite

Requirements:
- Fast performance
- Easy to use

## Feature 1

- [ ] Task
`

	prd, err := ParsePRDContent(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(prd.Context, "Technology Stack") {
		t.Error("context should contain Technology Stack")
	}
	if !strings.Contains(prd.Context, "Language: Go") {
		t.Error("context should contain Language: Go")
	}
	if !strings.Contains(prd.Context, "Requirements") {
		t.Error("context should contain Requirements")
	}
}

func TestParsePRDContent_FeatureSeparators(t *testing.T) {
	content := `# Project

## Feature 1

- [ ] Task 1

---

## Feature 2

- [ ] Task 2
`

	prd, err := ParsePRDContent(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(prd.Features) != 2 {
		t.Errorf("expected 2 features, got %d", len(prd.Features))
	}
}

func TestParsePRDContent_DescriptionWithBullets(t *testing.T) {
	content := `# Project

## Feature

This feature has bullets in description:
- Point 1
- Point 2
- Point 3

- [ ] Task
`

	prd, err := ParsePRDContent(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	f := prd.Features[0]
	if !strings.Contains(f.Description, "Point 1") {
		t.Error("description should contain bullet points")
	}
	if len(f.Tasks) != 1 {
		t.Errorf("expected 1 task, got %d", len(f.Tasks))
	}
}

func TestParsePRDContent_ComplexPRD(t *testing.T) {
	content := `# Ralph TUI Revamp v0.3.0

Ralph is a TUI application that orchestrates Claude Code instances.

Technology:
- Go 1.25
- BubbleTea
- Lipgloss

---

## Feature 1: Layout Foundation

Build the foundation layout.

Execution: sequential
Model: sonnet

- [ ] Create base layout
- [ ] Add responsive sizing
- [x] Design color scheme

Acceptance: Layout renders correctly
Acceptance: Handles resize events

---

## Feature 2: Content Area

Depends: 01

Build the content area.

Execution: parallel
Model: opus

- [ ] Implement panels
- [ ] Add scrolling

Acceptance: Content displays properly

---

## Feature 3: Integration

Depends: 01, Feature 2: Content Area

Final integration.

- [ ] Connect components
`

	prd, err := ParsePRDContent(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if prd.Title != "Ralph TUI Revamp v0.3.0" {
		t.Errorf("unexpected title: %q", prd.Title)
	}

	if len(prd.Features) != 3 {
		t.Fatalf("expected 3 features, got %d", len(prd.Features))
	}

	f1 := prd.Features[0]
	if f1.ExecutionMode != "sequential" {
		t.Errorf("f1: expected execution mode 'sequential', got %q", f1.ExecutionMode)
	}
	if f1.Model != "sonnet" {
		t.Errorf("f1: expected model 'sonnet', got %q", f1.Model)
	}
	if len(f1.Tasks) != 3 {
		t.Errorf("f1: expected 3 tasks, got %d", len(f1.Tasks))
	}
	if len(f1.AcceptanceCriteria) != 2 {
		t.Errorf("f1: expected 2 acceptance criteria, got %d", len(f1.AcceptanceCriteria))
	}

	f2 := prd.Features[1]
	if f2.ExecutionMode != "parallel" {
		t.Errorf("f2: expected execution mode 'parallel', got %q", f2.ExecutionMode)
	}
	if f2.Model != "opus" {
		t.Errorf("f2: expected model 'opus', got %q", f2.Model)
	}
	if len(f2.DependsOn) != 1 || f2.DependsOn[0] != "01" {
		t.Errorf("f2: expected dependency ['01'], got %v", f2.DependsOn)
	}

	f3 := prd.Features[2]
	if len(f3.DependsOn) != 2 {
		t.Errorf("f3: expected 2 dependencies, got %d", len(f3.DependsOn))
	}
}

func TestParsePRDContent_GlobalBudgetUSD(t *testing.T) {
	content := `# Project

Budget: $5.00

## Feature 1

- [ ] Task 1
`

	prd, err := ParsePRDContent(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if prd.BudgetUSD != 5.00 {
		t.Errorf("expected global budget USD 5.00, got %f", prd.BudgetUSD)
	}
	if prd.BudgetTokens != 0 {
		t.Errorf("expected global budget tokens 0, got %d", prd.BudgetTokens)
	}
}

func TestParsePRDContent_GlobalBudgetTokens(t *testing.T) {
	content := `# Project

Tokens: 100000

## Feature 1

- [ ] Task 1
`

	prd, err := ParsePRDContent(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if prd.BudgetTokens != 100000 {
		t.Errorf("expected global budget tokens 100000, got %d", prd.BudgetTokens)
	}
	if prd.BudgetUSD != 0 {
		t.Errorf("expected global budget USD 0, got %f", prd.BudgetUSD)
	}
}

func TestParsePRDContent_GlobalBudgetWithK(t *testing.T) {
	content := `# Project

Budget: 100k

## Feature 1

- [ ] Task 1
`

	prd, err := ParsePRDContent(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if prd.BudgetTokens != 100000 {
		t.Errorf("expected global budget tokens 100000, got %d", prd.BudgetTokens)
	}
}

func TestParsePRDContent_FeatureBudgetUSD(t *testing.T) {
	content := `# Project

## Feature 1

Budget: $1.50

- [ ] Task 1

## Feature 2

Budget: $2.00

- [ ] Task 2
`

	prd, err := ParsePRDContent(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(prd.Features) != 2 {
		t.Fatalf("expected 2 features, got %d", len(prd.Features))
	}

	f1 := prd.Features[0]
	if f1.BudgetUSD != 1.50 {
		t.Errorf("feature 1: expected budget USD 1.50, got %f", f1.BudgetUSD)
	}

	f2 := prd.Features[1]
	if f2.BudgetUSD != 2.00 {
		t.Errorf("feature 2: expected budget USD 2.00, got %f", f2.BudgetUSD)
	}
}

func TestParsePRDContent_FeatureBudgetTokens(t *testing.T) {
	content := `# Project

## Feature 1

Tokens: 50k

- [ ] Task 1
`

	prd, err := ParsePRDContent(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	f := prd.Features[0]
	if f.BudgetTokens != 50000 {
		t.Errorf("expected feature budget tokens 50000, got %d", f.BudgetTokens)
	}
}

func TestParsePRDContent_MixedBudgets(t *testing.T) {
	content := `# Project

Budget: $10.00

## Feature 1

Budget: $2.00

- [ ] Task 1

## Feature 2

Tokens: 100k

- [ ] Task 2
`

	prd, err := ParsePRDContent(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if prd.BudgetUSD != 10.00 {
		t.Errorf("expected global budget USD 10.00, got %f", prd.BudgetUSD)
	}

	f1 := prd.Features[0]
	if f1.BudgetUSD != 2.00 {
		t.Errorf("feature 1: expected budget USD 2.00, got %f", f1.BudgetUSD)
	}

	f2 := prd.Features[1]
	if f2.BudgetTokens != 100000 {
		t.Errorf("feature 2: expected budget tokens 100000, got %d", f2.BudgetTokens)
	}
}

func TestParseBudgetValue(t *testing.T) {
	tests := []struct {
		name       string
		value      string
		wantTokens int64
		wantUSD    float64
	}{
		{"dollar simple", "$5", 0, 5.0},
		{"dollar with cents", "$5.50", 0, 5.50},
		{"simple number", "10000", 10000, 0},
		{"with k suffix", "10k", 10000, 0},
		{"with K suffix", "10K", 10000, 0},
		{"with M suffix", "1M", 1000000, 0},
		{"decimal with k", "1.5k", 1500, 0},
		{"decimal with M", "1.5M", 1500000, 0},
		{"with tokens suffix", "10000 tokens", 10000, 0},
		{"decimal as USD", "5.00", 0, 5.0},
		{"empty", "", 0, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens, usd := parseBudgetValue(tt.value)
			if tokens != tt.wantTokens {
				t.Errorf("tokens = %d, want %d", tokens, tt.wantTokens)
			}
			if usd != tt.wantUSD {
				t.Errorf("usd = %f, want %f", usd, tt.wantUSD)
			}
		})
	}
}

func TestParsePRDContent_GlobalContextBudget(t *testing.T) {
	content := `# Project

Context: 50000

## Feature 1

- [ ] Task 1
`

	prd, err := ParsePRDContent(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if prd.ContextBudget != 50000 {
		t.Errorf("expected context budget 50000, got %d", prd.ContextBudget)
	}
}

func TestParsePRDContent_GlobalContextBudgetWithK(t *testing.T) {
	content := `# Project

Context: 100k

## Feature 1

- [ ] Task 1
`

	prd, err := ParsePRDContent(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if prd.ContextBudget != 100000 {
		t.Errorf("expected context budget 100000, got %d", prd.ContextBudget)
	}
}

func TestParsePRDContent_GlobalContextBudgetWithM(t *testing.T) {
	content := `# Project

Context: 1.5M

## Feature 1

- [ ] Task 1
`

	prd, err := ParsePRDContent(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if prd.ContextBudget != 1500000 {
		t.Errorf("expected context budget 1500000, got %d", prd.ContextBudget)
	}
}

func TestParsePRDContent_FeatureContextBudget(t *testing.T) {
	content := `# Project

## Feature 1

Context: 50000

- [ ] Task 1

## Feature 2

Context: 100k tokens

- [ ] Task 2
`

	prd, err := ParsePRDContent(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(prd.Features) != 2 {
		t.Fatalf("expected 2 features, got %d", len(prd.Features))
	}

	f1 := prd.Features[0]
	if f1.ContextBudget != 50000 {
		t.Errorf("feature 1: expected context budget 50000, got %d", f1.ContextBudget)
	}

	f2 := prd.Features[1]
	if f2.ContextBudget != 100000 {
		t.Errorf("feature 2: expected context budget 100000, got %d", f2.ContextBudget)
	}
}

func TestParsePRDContent_MixedContextAndBudget(t *testing.T) {
	content := `# Project

Budget: $5.00
Context: 100k

## Feature 1

Budget: $1.00
Context: 50k

- [ ] Task 1
`

	prd, err := ParsePRDContent(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if prd.BudgetUSD != 5.00 {
		t.Errorf("expected global budget USD 5.00, got %f", prd.BudgetUSD)
	}
	if prd.ContextBudget != 100000 {
		t.Errorf("expected global context budget 100000, got %d", prd.ContextBudget)
	}

	f := prd.Features[0]
	if f.BudgetUSD != 1.00 {
		t.Errorf("feature: expected budget USD 1.00, got %f", f.BudgetUSD)
	}
	if f.ContextBudget != 50000 {
		t.Errorf("feature: expected context budget 50000, got %d", f.ContextBudget)
	}
}

func TestParseContextValue(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		expected int64
	}{
		{"simple number", "50000", 50000},
		{"with k suffix", "50k", 50000},
		{"with K suffix", "50K", 50000},
		{"with M suffix", "1M", 1000000},
		{"decimal with k", "1.5k", 1500},
		{"decimal with M", "1.5M", 1500000},
		{"with tokens suffix", "50k tokens", 50000},
		{"with whitespace", " 50000 ", 50000},
		{"empty", "", 0},
		{"negative", "-100", 0},
		{"invalid", "abc", 0},
		{"zero", "0", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseContextValue(tt.value)
			if result != tt.expected {
				t.Errorf("expected %d, got %d", tt.expected, result)
			}
		})
	}
}

// Isolation level parsing tests

func TestParsePRDContent_IsolationLevelStrict(t *testing.T) {
	content := `# Project

## Feature 1

Isolation: strict

- [ ] Task 1
`

	prd, err := ParsePRDContent(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(prd.Features) != 1 {
		t.Fatalf("expected 1 feature, got %d", len(prd.Features))
	}

	f := prd.Features[0]
	if f.IsolationLevel != "strict" {
		t.Errorf("expected isolation level 'strict', got %q", f.IsolationLevel)
	}
}

func TestParsePRDContent_IsolationLevelLenient(t *testing.T) {
	content := `# Project

## Feature 1

Isolation: lenient

- [ ] Task 1
`

	prd, err := ParsePRDContent(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	f := prd.Features[0]
	if f.IsolationLevel != "lenient" {
		t.Errorf("expected isolation level 'lenient', got %q", f.IsolationLevel)
	}
}

func TestParsePRDContent_IsolationLevelCaseInsensitive(t *testing.T) {
	content := `# Project

## Feature 1

Isolation: STRICT

- [ ] Task 1

## Feature 2

Isolation: Lenient

- [ ] Task 2
`

	prd, err := ParsePRDContent(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(prd.Features) != 2 {
		t.Fatalf("expected 2 features, got %d", len(prd.Features))
	}

	f1 := prd.Features[0]
	if f1.IsolationLevel != "strict" {
		t.Errorf("feature 1: expected isolation level 'strict', got %q", f1.IsolationLevel)
	}

	f2 := prd.Features[1]
	if f2.IsolationLevel != "lenient" {
		t.Errorf("feature 2: expected isolation level 'lenient', got %q", f2.IsolationLevel)
	}
}

func TestParsePRDContent_IsolationLevelInvalid(t *testing.T) {
	content := `# Project

## Feature 1

Isolation: unknown

- [ ] Task 1
`

	prd, err := ParsePRDContent(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	f := prd.Features[0]
	if f.IsolationLevel != "" {
		t.Errorf("expected empty isolation level for invalid value, got %q", f.IsolationLevel)
	}
}

func TestParsePRDContent_IsolationLevelDefault(t *testing.T) {
	content := `# Project

## Feature 1

- [ ] Task 1
`

	prd, err := ParsePRDContent(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	f := prd.Features[0]
	if f.IsolationLevel != "" {
		t.Errorf("expected empty isolation level by default, got %q", f.IsolationLevel)
	}
}

func TestParsePRDContent_IsolationWithOtherMetadata(t *testing.T) {
	content := `# Project

## Feature 1

Model: opus
Execution: parallel
Isolation: strict
Budget: $5.00

- [ ] Task 1
`

	prd, err := ParsePRDContent(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	f := prd.Features[0]
	if f.Model != "opus" {
		t.Errorf("expected model 'opus', got %q", f.Model)
	}
	if f.ExecutionMode != "parallel" {
		t.Errorf("expected execution mode 'parallel', got %q", f.ExecutionMode)
	}
	if f.IsolationLevel != "strict" {
		t.Errorf("expected isolation level 'strict', got %q", f.IsolationLevel)
	}
	if f.BudgetUSD != 5.00 {
		t.Errorf("expected budget USD 5.00, got %f", f.BudgetUSD)
	}
}
