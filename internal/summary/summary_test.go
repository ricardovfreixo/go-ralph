package summary

import (
	"strings"
	"testing"
	"time"

	"github.com/vx/ralph-go/internal/actions"
)

func TestNewChildResult(t *testing.T) {
	r := NewChildResult("test-123", "Test Feature", "completed")

	if r.FeatureID != "test-123" {
		t.Errorf("Expected FeatureID 'test-123', got '%s'", r.FeatureID)
	}
	if r.Title != "Test Feature" {
		t.Errorf("Expected Title 'Test Feature', got '%s'", r.Title)
	}
	if r.Status != "completed" {
		t.Errorf("Expected Status 'completed', got '%s'", r.Status)
	}
	if r.FilesChanged == nil {
		t.Error("FilesChanged should be initialized")
	}
	if r.Actions == nil {
		t.Error("Actions should be initialized")
	}
}

func TestChildResultSetError(t *testing.T) {
	r := NewChildResult("test", "Test", "failed")
	r.SetError("something went wrong")

	if r.Error != "something went wrong" {
		t.Errorf("Expected error 'something went wrong', got '%s'", r.Error)
	}
}

func TestChildResultSetTestResults(t *testing.T) {
	r := NewChildResult("test", "Test", "completed")
	r.SetTestResults(10, 2, 1, "test output")

	if r.TestResults == nil {
		t.Fatal("TestResults should not be nil")
	}
	if r.TestResults.Passed != 10 {
		t.Errorf("Expected Passed 10, got %d", r.TestResults.Passed)
	}
	if r.TestResults.Failed != 2 {
		t.Errorf("Expected Failed 2, got %d", r.TestResults.Failed)
	}
	if r.TestResults.Skipped != 1 {
		t.Errorf("Expected Skipped 1, got %d", r.TestResults.Skipped)
	}
	if r.TestResults.Total != 13 {
		t.Errorf("Expected Total 13, got %d", r.TestResults.Total)
	}
	if r.TestResults.Output != "test output" {
		t.Errorf("Expected Output 'test output', got '%s'", r.TestResults.Output)
	}
}

func TestChildResultAddFileChange(t *testing.T) {
	r := NewChildResult("test", "Test", "completed")
	r.AddFileChange("/path/to/file.go", "created")
	r.AddFileChange("/path/to/other.go", "modified")

	if len(r.FilesChanged) != 2 {
		t.Errorf("Expected 2 files changed, got %d", len(r.FilesChanged))
	}

	if r.FilesChanged[0].Path != "/path/to/file.go" {
		t.Errorf("Expected path '/path/to/file.go', got '%s'", r.FilesChanged[0].Path)
	}
	if r.FilesChanged[0].Operation != "created" {
		t.Errorf("Expected operation 'created', got '%s'", r.FilesChanged[0].Operation)
	}
}

func TestChildResultAddAction(t *testing.T) {
	r := NewChildResult("test", "Test", "completed")
	r.AddAction("write", "/path/file.go", "success")
	r.AddAction("bash", "go test ./...", "success")

	if len(r.Actions) != 2 {
		t.Errorf("Expected 2 actions, got %d", len(r.Actions))
	}

	if r.Actions[0].Type != "write" {
		t.Errorf("Expected type 'write', got '%s'", r.Actions[0].Type)
	}
	if r.Actions[0].Target != "/path/file.go" {
		t.Errorf("Expected target '/path/file.go', got '%s'", r.Actions[0].Target)
	}
}

func TestChildResultExtractFromActions(t *testing.T) {
	r := NewChildResult("test", "Test", "completed")

	acts := []actions.Action{
		{Type: actions.ActionWrite, Target: "/path/new.go", Timestamp: time.Now()},
		{Type: actions.ActionEdit, Target: "/path/existing.go", Timestamp: time.Now()},
		{Type: actions.ActionRead, Target: "/path/read.go", Timestamp: time.Now()},
		{Type: actions.ActionBash, Target: "go build", Timestamp: time.Now()},
		{Type: actions.ActionWrite, Target: "/path/new.go", Timestamp: time.Now()}, // Duplicate
	}

	r.ExtractFromActions(acts)

	if len(r.FilesChanged) != 2 {
		t.Errorf("Expected 2 files changed (deduplicated), got %d", len(r.FilesChanged))
	}

	if len(r.Actions) != 5 {
		t.Errorf("Expected 5 actions, got %d", len(r.Actions))
	}
}

func TestGenerateSummaryBasic(t *testing.T) {
	r := NewChildResult("test-123", "Test Feature", "completed")
	r.SetTestResults(5, 0, 0, "")

	summary := r.GenerateSummary(5000)

	if summary == nil {
		t.Fatal("Summary should not be nil")
	}

	if !strings.Contains(summary.Raw, "Test Feature") {
		t.Error("Summary should contain feature title")
	}
	if !strings.Contains(summary.Raw, "completed") {
		t.Error("Summary should contain status")
	}
	if !strings.Contains(summary.Raw, "Passed: 5") {
		t.Error("Summary should contain test results")
	}
}

func TestGenerateSummaryWithFiles(t *testing.T) {
	r := NewChildResult("test", "Test", "completed")
	r.AddFileChange("/path/to/file.go", "created")
	r.AddFileChange("/path/to/other.go", "modified")

	summary := r.GenerateSummary(5000)

	if !strings.Contains(summary.Raw, "Files Changed") {
		t.Error("Summary should contain Files Changed section")
	}
	if !strings.Contains(summary.Raw, "/path/to/file.go") {
		t.Error("Summary should contain file path")
	}
}

func TestGenerateSummaryWithError(t *testing.T) {
	r := NewChildResult("test", "Test", "failed")
	r.SetError("build failed: exit code 1")

	summary := r.GenerateSummary(5000)

	if !strings.Contains(summary.Raw, "failed") {
		t.Error("Summary should contain status")
	}
	if !strings.Contains(summary.Raw, "build failed") {
		t.Error("Summary should contain error message")
	}
}

func TestGenerateSummaryTruncation(t *testing.T) {
	r := NewChildResult("test", "Test", "completed")

	for i := 0; i < 200; i++ {
		r.AddAction("bash", "go test ./internal/very/long/path/name/here/...", "success")
		r.AddFileChange("/very/long/path/to/file/number/"+string(rune('a'+i%26))+".go", "created")
	}

	summary := r.GenerateSummary(100)

	if summary.TokenCount > 150 {
		t.Errorf("Summary should be truncated to near 100 tokens, got %d", summary.TokenCount)
	}
	if !summary.Truncated {
		t.Error("Summary should be marked as truncated")
	}
}

func TestGenerateSummaryFormatted(t *testing.T) {
	r := NewChildResult("test-123", "Test Feature", "completed")
	r.SetTestResults(5, 0, 0, "")
	r.AddFileChange("/path/file.go", "created")

	summary := r.GenerateSummary(5000)

	if !strings.Contains(summary.Formatted, "sub_feature_completed") {
		t.Error("Formatted summary should contain sub_feature_completed JSON key")
	}
	if !strings.Contains(summary.Formatted, "test-123") {
		t.Error("Formatted summary should contain feature ID")
	}
	if !strings.Contains(summary.Formatted, "files_changed") {
		t.Error("Formatted summary should contain files_changed")
	}
}

func TestCompact(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(*ChildResult)
		expected []string
	}{
		{
			name: "basic completed",
			setup: func(r *ChildResult) {
				r.Status = "completed"
			},
			expected: []string{"'Test': completed"},
		},
		{
			name: "with tests",
			setup: func(r *ChildResult) {
				r.Status = "completed"
				r.SetTestResults(10, 2, 0, "")
			},
			expected: []string{"tests: 10/12 passed"},
		},
		{
			name: "with files",
			setup: func(r *ChildResult) {
				r.Status = "completed"
				r.AddFileChange("/a.go", "created")
				r.AddFileChange("/b.go", "modified")
			},
			expected: []string{"2 files changed"},
		},
		{
			name: "with error",
			setup: func(r *ChildResult) {
				r.Status = "failed"
				r.SetError("build failed")
			},
			expected: []string{"error: build failed"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := NewChildResult("test", "Test", "pending")
			tt.setup(r)
			compact := r.Compact()
			for _, exp := range tt.expected {
				if !strings.Contains(compact, exp) {
					t.Errorf("Compact() = %q, expected to contain %q", compact, exp)
				}
			}
		})
	}
}

func TestNeedsHaikuSummary(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(*ChildResult)
		expected bool
	}{
		{
			name:     "empty result",
			setup:    func(r *ChildResult) {},
			expected: false,
		},
		{
			name: "few actions",
			setup: func(r *ChildResult) {
				for i := 0; i < 10; i++ {
					r.AddAction("bash", "cmd", "")
				}
			},
			expected: false,
		},
		{
			name: "many actions",
			setup: func(r *ChildResult) {
				for i := 0; i < 60; i++ {
					r.AddAction("bash", "cmd", "")
				}
			},
			expected: true,
		},
		{
			name: "large test output",
			setup: func(r *ChildResult) {
				r.SetTestResults(10, 0, 0, strings.Repeat("x", 6000))
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := NewChildResult("test", "Test", "completed")
			tt.setup(r)
			if got := r.NeedsHaikuSummary(); got != tt.expected {
				t.Errorf("NeedsHaikuSummary() = %v, expected %v", got, tt.expected)
			}
		})
	}
}

func TestBuildHaikuPrompt(t *testing.T) {
	r := NewChildResult("test", "Test Feature", "completed")
	r.SetTestResults(10, 2, 0, "test output here")
	r.AddFileChange("/path/file.go", "created")
	r.AddAction("bash", "go build", "")

	prompt := r.BuildHaikuPrompt()

	if !strings.Contains(prompt, "Test Feature") {
		t.Error("Prompt should contain feature title")
	}
	if !strings.Contains(prompt, "10 passed, 2 failed") {
		t.Error("Prompt should contain test stats")
	}
	if !strings.Contains(prompt, "/path/file.go") {
		t.Error("Prompt should contain file path")
	}
	if !strings.Contains(prompt, "concise summary") {
		t.Error("Prompt should contain summarization instructions")
	}
}

func TestExtractTestFailures(t *testing.T) {
	tests := []struct {
		name     string
		output   string
		expected []string
	}{
		{
			name:     "go test failures",
			output:   "--- FAIL: TestSomething\n--- FAIL: TestOther",
			expected: []string{"TestSomething", "TestOther"},
		},
		{
			name:     "jest failures",
			output:   "  ✕ should do something\n  ✕ should do other",
			expected: []string{"should do something", "should do other"},
		},
		{
			name:     "pytest failures",
			output:   "FAILED test_something.py::test_one\nFAILED test_other.py::test_two",
			expected: []string{"test_something.py::test_one", "test_other.py::test_two"},
		},
		{
			name:     "no failures",
			output:   "PASS\nok  test 0.001s",
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			failures := ExtractTestFailures(tt.output)
			if len(failures) != len(tt.expected) {
				t.Errorf("Expected %d failures, got %d", len(tt.expected), len(failures))
			}
			for i, exp := range tt.expected {
				if i < len(failures) && failures[i] != exp {
					t.Errorf("Failure[%d] = %q, expected %q", i, failures[i], exp)
				}
			}
		})
	}
}

func TestEstimateTokens(t *testing.T) {
	tests := []struct {
		text     string
		expected int
	}{
		{"", 0},
		{"test", 1},
		{"hello world", 2},
		{strings.Repeat("x", 100), 25},
	}

	for _, tt := range tests {
		got := estimateTokens(tt.text)
		if got != tt.expected {
			t.Errorf("estimateTokens(%q) = %d, expected %d", tt.text[:min(10, len(tt.text))], got, tt.expected)
		}
	}
}

func TestTruncateToTokenBudget(t *testing.T) {
	longText := strings.Repeat("line of text\n", 100)
	truncated := truncateToTokenBudget(longText, 100)

	if len(truncated) > 500 {
		t.Errorf("Truncated text too long: %d chars", len(truncated))
	}
	if !strings.Contains(truncated, "truncated") {
		t.Error("Truncated text should contain truncation notice")
	}
}

func TestTruncateString(t *testing.T) {
	tests := []struct {
		s        string
		maxLen   int
		expected string
	}{
		{"short", 10, "short"},
		{"this is a very long string", 10, "this is..."},
		{"with\nnewlines", 20, "with newlines"},
	}

	for _, tt := range tests {
		got := truncateString(tt.s, tt.maxLen)
		if got != tt.expected {
			t.Errorf("truncateString(%q, %d) = %q, expected %q", tt.s, tt.maxLen, got, tt.expected)
		}
	}
}

func TestSummarizer(t *testing.T) {
	s := NewSummarizer()

	if s.maxTokens != DefaultMaxSummaryTokens {
		t.Errorf("Expected default maxTokens %d, got %d", DefaultMaxSummaryTokens, s.maxTokens)
	}

	s.SetMaxTokens(5000)
	if s.maxTokens != 5000 {
		t.Errorf("Expected maxTokens 5000, got %d", s.maxTokens)
	}

	s.SetMaxTokens(100)
	if s.maxTokens != 5000 {
		t.Error("Should not set maxTokens below minimum")
	}
}

func TestSummarizerSummarize(t *testing.T) {
	s := NewSummarizer()

	r := NewChildResult("test", "Test", "completed")
	r.SetTestResults(10, 0, 0, "")

	summary, err := s.Summarize(r)
	if err != nil {
		t.Fatalf("Summarize() error = %v", err)
	}

	if summary == nil {
		t.Fatal("Summary should not be nil")
	}
	if !strings.Contains(summary.Raw, "Test") {
		t.Error("Summary should contain feature name")
	}
}

type mockHaikuRunner struct {
	result string
	err    error
}

func (m *mockHaikuRunner) Summarize(prompt string) (string, error) {
	return m.result, m.err
}

func TestSummarizerWithHaiku(t *testing.T) {
	s := NewSummarizer()
	s.SetHaikuRunner(&mockHaikuRunner{result: "AI generated summary"})

	r := NewChildResult("test", "Test", "completed")
	for i := 0; i < 60; i++ {
		r.AddAction("bash", "cmd", "")
	}

	summary, err := s.Summarize(r)
	if err != nil {
		t.Fatalf("Summarize() error = %v", err)
	}

	if !strings.Contains(summary.Raw, "AI generated summary") {
		t.Error("Should use haiku-generated summary for large results")
	}
}

func TestConcurrentAccess(t *testing.T) {
	r := NewChildResult("test", "Test", "pending")

	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(n int) {
			r.SetError("error")
			r.SetTestResults(n, 0, 0, "")
			r.AddFileChange("/path", "created")
			r.AddAction("bash", "cmd", "")
			_ = r.Compact()
			_ = r.GenerateSummary(1000)
			done <- true
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
