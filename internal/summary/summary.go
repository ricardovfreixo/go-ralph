package summary

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/vx/ralph-go/internal/actions"
)

const (
	DefaultMaxSummaryTokens = 2000
	HaikuThresholdTokens    = 5000
	MinBudgetTokens         = 500
)

// ChildResult captures key outputs from a completed child feature
type ChildResult struct {
	mu sync.RWMutex

	FeatureID string `json:"feature_id"`
	Title     string `json:"title"`
	Status    string `json:"status"`
	Error     string `json:"error,omitempty"`

	FilesChanged []FileChange   `json:"files_changed,omitempty"`
	TestResults  *TestSummary   `json:"test_results,omitempty"`
	Actions      []ActionEntry  `json:"actions,omitempty"`
	Duration     time.Duration  `json:"duration_ms,omitempty"`
	TokensUsed   int64          `json:"tokens_used,omitempty"`
}

// FileChange represents a file modification
type FileChange struct {
	Path      string `json:"path"`
	Operation string `json:"operation"` // "created", "modified", "deleted"
	LinesAdded   int `json:"lines_added,omitempty"`
	LinesRemoved int `json:"lines_removed,omitempty"`
}

// TestSummary captures test execution results
type TestSummary struct {
	Passed   int    `json:"passed"`
	Failed   int    `json:"failed"`
	Skipped  int    `json:"skipped,omitempty"`
	Total    int    `json:"total"`
	Output   string `json:"output,omitempty"`
	Failures []string `json:"failures,omitempty"`
}

// ActionEntry is a simplified action for the summary
type ActionEntry struct {
	Type   string `json:"type"`
	Target string `json:"target"`
	Status string `json:"status,omitempty"`
}

// Summary is the final formatted summary for parent injection
type Summary struct {
	Raw        string `json:"raw"`
	Formatted  string `json:"formatted"`
	TokenCount int    `json:"token_count"`
	Truncated  bool   `json:"truncated"`
}

// NewChildResult creates a new child result from completion data
func NewChildResult(featureID, title, status string) *ChildResult {
	return &ChildResult{
		FeatureID:    featureID,
		Title:        title,
		Status:       status,
		FilesChanged: make([]FileChange, 0),
		Actions:      make([]ActionEntry, 0),
	}
}

// SetError sets the error message for failed features
func (r *ChildResult) SetError(err string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.Error = err
}

// SetTestResults sets the test results
func (r *ChildResult) SetTestResults(passed, failed, skipped int, output string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.TestResults = &TestSummary{
		Passed:  passed,
		Failed:  failed,
		Skipped: skipped,
		Total:   passed + failed + skipped,
		Output:  output,
	}
}

// AddFileChange records a file modification
func (r *ChildResult) AddFileChange(path, operation string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.FilesChanged = append(r.FilesChanged, FileChange{
		Path:      path,
		Operation: operation,
	})
}

// AddAction records a significant action
func (r *ChildResult) AddAction(actionType, target, status string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.Actions = append(r.Actions, ActionEntry{
		Type:   actionType,
		Target: target,
		Status: status,
	})
}

// SetTokensUsed sets the token count
func (r *ChildResult) SetTokensUsed(tokens int64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.TokensUsed = tokens
}

// SetDuration sets the execution duration
func (r *ChildResult) SetDuration(d time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.Duration = d
}

// ExtractFromActions populates file changes from action list
func (r *ChildResult) ExtractFromActions(acts []actions.Action) {
	r.mu.Lock()
	defer r.mu.Unlock()

	seen := make(map[string]bool)
	for _, act := range acts {
		switch act.Type {
		case actions.ActionWrite:
			if !seen[act.Target] {
				r.FilesChanged = append(r.FilesChanged, FileChange{
					Path:      act.Target,
					Operation: "created",
				})
				seen[act.Target] = true
			}
		case actions.ActionEdit:
			if !seen[act.Target] {
				r.FilesChanged = append(r.FilesChanged, FileChange{
					Path:      act.Target,
					Operation: "modified",
				})
				seen[act.Target] = true
			}
		}

		r.Actions = append(r.Actions, ActionEntry{
			Type:   string(act.Type),
			Target: act.Target,
		})
	}
}

// GenerateSummary creates a formatted summary within the given token budget
func (r *ChildResult) GenerateSummary(maxTokens int64) *Summary {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if maxTokens <= 0 {
		maxTokens = DefaultMaxSummaryTokens
	}

	// Build the raw summary
	var sb strings.Builder

	// Header with status
	sb.WriteString(fmt.Sprintf("## Sub-Feature: %s\n\n", r.Title))
	sb.WriteString(fmt.Sprintf("**Status:** %s\n", r.Status))

	// Error if present
	if r.Error != "" {
		sb.WriteString(fmt.Sprintf("**Error:** %s\n", r.Error))
	}

	// Test results
	if r.TestResults != nil {
		sb.WriteString("\n### Test Results\n")
		sb.WriteString(fmt.Sprintf("- Passed: %d\n", r.TestResults.Passed))
		sb.WriteString(fmt.Sprintf("- Failed: %d\n", r.TestResults.Failed))
		if r.TestResults.Skipped > 0 {
			sb.WriteString(fmt.Sprintf("- Skipped: %d\n", r.TestResults.Skipped))
		}
		if len(r.TestResults.Failures) > 0 {
			sb.WriteString("\n**Failures:**\n")
			for _, f := range r.TestResults.Failures {
				sb.WriteString(fmt.Sprintf("- %s\n", f))
			}
		}
	}

	// Files changed
	if len(r.FilesChanged) > 0 {
		sb.WriteString("\n### Files Changed\n")
		for _, f := range r.FilesChanged {
			sb.WriteString(fmt.Sprintf("- %s: %s\n", f.Operation, f.Path))
		}
	}

	// Key actions (limited)
	if len(r.Actions) > 0 {
		sb.WriteString("\n### Key Actions\n")
		maxActions := 10
		if len(r.Actions) < maxActions {
			maxActions = len(r.Actions)
		}
		for i := 0; i < maxActions; i++ {
			act := r.Actions[i]
			sb.WriteString(fmt.Sprintf("- %s: %s\n", act.Type, act.Target))
		}
		if len(r.Actions) > 10 {
			sb.WriteString(fmt.Sprintf("- ... and %d more actions\n", len(r.Actions)-10))
		}
	}

	// Stats
	if r.TokensUsed > 0 || r.Duration > 0 {
		sb.WriteString("\n### Stats\n")
		if r.TokensUsed > 0 {
			sb.WriteString(fmt.Sprintf("- Tokens: %d\n", r.TokensUsed))
		}
		if r.Duration > 0 {
			sb.WriteString(fmt.Sprintf("- Duration: %s\n", r.Duration.Round(time.Second)))
		}
	}

	raw := sb.String()
	tokenCount := estimateTokens(raw)

	summary := &Summary{
		Raw:        raw,
		TokenCount: tokenCount,
		Truncated:  false,
	}

	// Truncate if over budget
	if int64(tokenCount) > maxTokens {
		summary.Raw = truncateToTokenBudget(raw, maxTokens)
		summary.TokenCount = estimateTokens(summary.Raw)
		summary.Truncated = true
	}

	// Generate formatted JSON version
	summary.Formatted = r.formatForInjection(summary.Raw)

	return summary
}

// formatForInjection creates the JSON structure for parent prompt injection
func (r *ChildResult) formatForInjection(summaryText string) string {
	data := map[string]interface{}{
		"sub_feature_completed": map[string]interface{}{
			"id":      r.FeatureID,
			"title":   r.Title,
			"status":  r.Status,
			"summary": summaryText,
		},
	}

	if r.Error != "" {
		data["sub_feature_completed"].(map[string]interface{})["error"] = r.Error
	}

	if r.TestResults != nil {
		data["sub_feature_completed"].(map[string]interface{})["tests"] = map[string]int{
			"passed": r.TestResults.Passed,
			"failed": r.TestResults.Failed,
			"total":  r.TestResults.Total,
		}
	}

	if len(r.FilesChanged) > 0 {
		files := make([]string, len(r.FilesChanged))
		for i, f := range r.FilesChanged {
			files[i] = f.Path
		}
		data["sub_feature_completed"].(map[string]interface{})["files_changed"] = files
	}

	if r.TokensUsed > 0 {
		data["sub_feature_completed"].(map[string]interface{})["tokens_used"] = r.TokensUsed
	}

	jsonBytes, _ := json.MarshalIndent(data, "", "  ")
	return string(jsonBytes)
}

// Compact returns a one-line summary
func (r *ChildResult) Compact() string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var parts []string
	parts = append(parts, fmt.Sprintf("'%s': %s", r.Title, r.Status))

	if r.TestResults != nil {
		parts = append(parts, fmt.Sprintf("tests: %d/%d passed", r.TestResults.Passed, r.TestResults.Total))
	}

	if len(r.FilesChanged) > 0 {
		parts = append(parts, fmt.Sprintf("%d files changed", len(r.FilesChanged)))
	}

	if r.Error != "" {
		parts = append(parts, fmt.Sprintf("error: %s", truncateString(r.Error, 50)))
	}

	return strings.Join(parts, ", ")
}

// NeedsHaikuSummary returns true if the result is large enough to benefit from AI summarization
func (r *ChildResult) NeedsHaikuSummary() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Large number of actions
	if len(r.Actions) > 50 {
		return true
	}

	// Long test output
	if r.TestResults != nil && len(r.TestResults.Output) > 5000 {
		return true
	}

	return false
}

// BuildHaikuPrompt creates a prompt for haiku to summarize the result
func (r *ChildResult) BuildHaikuPrompt() string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var sb strings.Builder
	sb.WriteString("Summarize this sub-feature execution result concisely (max 500 words):\n\n")
	sb.WriteString(fmt.Sprintf("Feature: %s\n", r.Title))
	sb.WriteString(fmt.Sprintf("Status: %s\n", r.Status))

	if r.Error != "" {
		sb.WriteString(fmt.Sprintf("Error: %s\n", r.Error))
	}

	if r.TestResults != nil {
		sb.WriteString(fmt.Sprintf("\nTests: %d passed, %d failed\n", r.TestResults.Passed, r.TestResults.Failed))
		if r.TestResults.Output != "" {
			output := r.TestResults.Output
			if len(output) > 3000 {
				output = output[:3000] + "...[truncated]"
			}
			sb.WriteString(fmt.Sprintf("Test Output:\n%s\n", output))
		}
	}

	if len(r.FilesChanged) > 0 {
		sb.WriteString("\nFiles Changed:\n")
		for _, f := range r.FilesChanged {
			sb.WriteString(fmt.Sprintf("- %s (%s)\n", f.Path, f.Operation))
		}
	}

	if len(r.Actions) > 0 {
		sb.WriteString("\nActions performed:\n")
		for _, a := range r.Actions {
			sb.WriteString(fmt.Sprintf("- %s: %s\n", a.Type, a.Target))
		}
	}

	sb.WriteString("\nProvide a concise summary highlighting:")
	sb.WriteString("\n1. What was accomplished")
	sb.WriteString("\n2. Any issues or failures")
	sb.WriteString("\n3. Key files modified")
	sb.WriteString("\n4. Actionable info for the parent feature")

	return sb.String()
}

// estimateTokens provides a rough token count (chars / 4 is a reasonable estimate)
func estimateTokens(text string) int {
	return len(text) / 4
}

// truncateToTokenBudget truncates text to fit within token budget
func truncateToTokenBudget(text string, maxTokens int64) string {
	maxChars := int(maxTokens * 4)
	if len(text) <= maxChars {
		return text
	}

	// Find a good truncation point (end of a line)
	truncated := text[:maxChars]
	lastNewline := strings.LastIndex(truncated, "\n")
	if lastNewline > maxChars/2 {
		truncated = truncated[:lastNewline]
	}

	return truncated + "\n\n[Summary truncated to fit context budget]"
}

// truncateString truncates a string with ellipsis
func truncateString(s string, maxLen int) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// Summarizer handles result summarization with optional AI assistance
type Summarizer struct {
	mu sync.RWMutex

	useHaiku     bool
	maxTokens    int64
	haikuRunner  HaikuRunner
}

// HaikuRunner is an interface for running haiku summarization
type HaikuRunner interface {
	Summarize(prompt string) (string, error)
}

// NewSummarizer creates a new summarizer
func NewSummarizer() *Summarizer {
	return &Summarizer{
		maxTokens: DefaultMaxSummaryTokens,
		useHaiku:  false,
	}
}

// SetMaxTokens sets the token budget for summaries
func (s *Summarizer) SetMaxTokens(tokens int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if tokens > MinBudgetTokens {
		s.maxTokens = tokens
	}
}

// SetHaikuRunner sets the haiku runner for AI summarization
func (s *Summarizer) SetHaikuRunner(runner HaikuRunner) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.haikuRunner = runner
	s.useHaiku = runner != nil
}

// Summarize creates a summary from a child result
func (s *Summarizer) Summarize(result *ChildResult) (*Summary, error) {
	s.mu.RLock()
	maxTokens := s.maxTokens
	useHaiku := s.useHaiku
	runner := s.haikuRunner
	s.mu.RUnlock()

	// Check if we should use haiku for large results
	if useHaiku && runner != nil && result.NeedsHaikuSummary() {
		prompt := result.BuildHaikuPrompt()
		aiSummary, err := runner.Summarize(prompt)
		if err == nil && aiSummary != "" {
			summary := &Summary{
				Raw:        aiSummary,
				TokenCount: estimateTokens(aiSummary),
				Truncated:  false,
			}
			summary.Formatted = result.formatForInjection(aiSummary)
			return summary, nil
		}
		// Fall back to regular summarization on error
	}

	return result.GenerateSummary(maxTokens), nil
}

// ExtractTestFailures parses test output for failure details
func ExtractTestFailures(output string) []string {
	var failures []string

	// Go test failure patterns
	goFailPattern := regexp.MustCompile(`(?m)^--- FAIL: (\S+)`)
	matches := goFailPattern.FindAllStringSubmatch(output, -1)
	for _, m := range matches {
		if len(m) > 1 {
			failures = append(failures, m[1])
		}
	}

	// Jest/Node test patterns
	jestFailPattern := regexp.MustCompile(`(?m)^\s*✕\s+(.+)$`)
	matches = jestFailPattern.FindAllStringSubmatch(output, -1)
	for _, m := range matches {
		if len(m) > 1 {
			failures = append(failures, strings.TrimSpace(m[1]))
		}
	}

	// Python pytest patterns
	pytestFailPattern := regexp.MustCompile(`(?m)^FAILED\s+(\S+)`)
	matches = pytestFailPattern.FindAllStringSubmatch(output, -1)
	for _, m := range matches {
		if len(m) > 1 {
			failures = append(failures, m[1])
		}
	}

	return failures
}
