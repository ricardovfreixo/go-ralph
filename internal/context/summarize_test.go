package context

import (
	"strings"
	"testing"
)

func TestEstimateTokens(t *testing.T) {
	tests := []struct {
		text     string
		expected int64
	}{
		{"", 0},
		{"hello", 1},        // 5 chars * 0.25 = 1.25 -> 1
		{"hello world", 2},  // 11 chars * 0.25 = 2.75 -> 2
		{strings.Repeat("a", 100), 25}, // 100 * 0.25 = 25
		{strings.Repeat("a", 1000), 250},
	}

	for _, tt := range tests {
		t.Run(tt.text[:min(10, len(tt.text))], func(t *testing.T) {
			result := EstimateTokens(tt.text)
			if result != tt.expected {
				t.Errorf("expected %d, got %d", tt.expected, result)
			}
		})
	}
}

func TestTruncateToTokens(t *testing.T) {
	// Short text should not be truncated
	short := "Hello world"
	result := TruncateToTokens(short, 1000)
	if result != short {
		t.Errorf("short text should not be truncated")
	}

	// Long text should be truncated
	long := strings.Repeat("a", 10000)
	result = TruncateToTokens(long, 1000)
	if len(result) >= len(long) {
		t.Errorf("long text should be truncated")
	}
	if !strings.Contains(result, "context truncated") {
		t.Errorf("truncated text should contain marker")
	}

	// Zero budget should return original
	result = TruncateToTokens(long, 0)
	if result != long {
		t.Errorf("zero budget should return original")
	}

	// Negative budget should return original
	result = TruncateToTokens(long, -100)
	if result != long {
		t.Errorf("negative budget should return original")
	}
}

func TestExtractEssentialContext(t *testing.T) {
	fullContext := `# Project Context

This is the project context with background info.
It contains important details about the project.

# Progress from Previous Features

## Feature 1 - Completed
Did some work on feature 1.

## Feature 2 - Completed
Did some work on feature 2.

# Current Feature: My Feature

This is the current feature description.
With multiple lines of detail.

## Tasks
- [ ] Task 1
- [ ] Task 2
`

	// Should extract current feature context with sufficient budget
	result := ExtractEssentialContext(fullContext, 50000)
	if !strings.Contains(result, "Current Feature") || !strings.Contains(result, "My Feature") {
		t.Errorf("should contain current feature header")
	}

	// With small budget, should still have some content
	result = ExtractEssentialContext(fullContext, 1000)
	if len(result) == 0 {
		t.Errorf("should have some content even with small budget")
	}
}

func TestParseContextSections(t *testing.T) {
	content := `# Project Context

Project details here.

# Progress from Previous Features

Progress info here.

# Current Feature: Test Feature

Feature description.
`

	sections := parseContextSections(content)

	if _, ok := sections["project"]; !ok {
		t.Errorf("should have project section")
	}
	if _, ok := sections["progress"]; !ok {
		t.Errorf("should have progress section")
	}
	if _, ok := sections["feature"]; !ok {
		t.Errorf("should have feature section")
	}
}

func TestExtractRecentProgress(t *testing.T) {
	progress := `## Feature 1
Did work 1.

## Feature 2
Did work 2.

## Feature 3
Did work 3.
`

	// Should get most recent entries within budget
	result := extractRecentProgress(progress, 10000)
	if !strings.Contains(result, "Feature 3") {
		t.Errorf("should contain most recent feature")
	}

	// With small budget, should get at least one entry
	result = extractRecentProgress(progress, 100)
	if len(result) == 0 {
		t.Errorf("should have at least some content")
	}
}

func TestSplitProgressEntries(t *testing.T) {
	progress := `## Feature 1
Content 1.

## Feature 2
Content 2.
`

	entries := splitProgressEntries(progress)
	if len(entries) != 2 {
		t.Errorf("expected 2 entries, got %d", len(entries))
	}
}

func TestSummarizeContext(t *testing.T) {
	// Small content should not be modified
	small := "Small content"
	result := SummarizeContext(small, 10000)
	if result != small {
		t.Errorf("small content should not be modified")
	}

	// Large content should be summarized
	large := strings.Repeat("Large content. ", 1000)
	result = SummarizeContext(large, 1000)
	if len(result) >= len(large) {
		t.Errorf("large content should be summarized")
	}
}

func TestPrepareChildContext(t *testing.T) {
	parentContext := `# Project Context

Project info.

# Current Feature: Parent Feature

Parent description.
`

	tasks := []string{"Task 1", "Task 2"}

	result := PrepareChildContext(parentContext, 50000, "Child Feature", tasks)

	if !strings.Contains(result, "# Sub-Feature: Child Feature") {
		t.Errorf("should contain child feature header")
	}
	if !strings.Contains(result, "Task 1") {
		t.Errorf("should contain tasks")
	}
	if !strings.Contains(result, "Context from Parent") {
		t.Errorf("should contain parent context section")
	}

	// With minimal budget, should still have feature header
	result = PrepareChildContext(parentContext, 100, "Child Feature", tasks)
	if !strings.Contains(result, "Child Feature") {
		t.Errorf("should contain feature title even with small budget")
	}
}

func TestPrepareChildContextEmptyTasks(t *testing.T) {
	result := PrepareChildContext("", 10000, "Test", nil)
	if !strings.Contains(result, "# Sub-Feature: Test") {
		t.Errorf("should contain feature header")
	}
	if strings.Contains(result, "## Tasks") {
		t.Errorf("should not have tasks section without tasks")
	}
}

func TestPrepareChildContextNoParent(t *testing.T) {
	result := PrepareChildContext("", 10000, "Test", []string{"Task 1"})
	if strings.Contains(result, "Context from Parent") {
		t.Errorf("should not have parent context section without parent context")
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
