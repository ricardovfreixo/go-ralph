package actions

import (
	"encoding/json"
	"testing"
	"time"
)

func TestActionSummaryString(t *testing.T) {
	tests := []struct {
		name     string
		summary  ActionSummary
		expected string
	}{
		{
			name:     "empty summary",
			summary:  ActionSummary{},
			expected: "",
		},
		{
			name:     "files only",
			summary:  ActionSummary{Files: 3},
			expected: "3 files",
		},
		{
			name:     "commands only",
			summary:  ActionSummary{Commands: 2},
			expected: "2 cmds",
		},
		{
			name:     "mixed",
			summary:  ActionSummary{Files: 2, Commands: 1, Agents: 1},
			expected: "2 files, 1 cmds, 1 agents",
		},
		{
			name:     "all types",
			summary:  ActionSummary{Files: 1, Commands: 2, Agents: 1, Fetches: 3, Searches: 2},
			expected: "1 files, 2 cmds, 1 agents, 3 fetches, 2 searches",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.summary.String()
			if got != tt.expected {
				t.Errorf("ActionSummary.String() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestActionSummaryIsEmpty(t *testing.T) {
	empty := ActionSummary{}
	if !empty.IsEmpty() {
		t.Error("Empty ActionSummary.IsEmpty() should return true")
	}

	notEmpty := ActionSummary{Files: 1}
	if notEmpty.IsEmpty() {
		t.Error("Non-empty ActionSummary.IsEmpty() should return false")
	}
}

func TestActionStore(t *testing.T) {
	store := NewActionStore()
	featureID := "test-feature"

	action1 := Action{
		Type:      ActionWrite,
		Tool:      "Write",
		Target:    "test.go",
		Timestamp: time.Now(),
	}
	action2 := Action{
		Type:      ActionBash,
		Tool:      "Bash",
		Target:    "go test ./...",
		Timestamp: time.Now(),
	}

	store.AddAction(featureID, action1)
	store.AddAction(featureID, action2)

	actions := store.GetActions(featureID)
	if len(actions) != 2 {
		t.Errorf("Expected 2 actions, got %d", len(actions))
	}

	summary := store.GetSummary(featureID)
	if summary.Files != 1 {
		t.Errorf("Expected 1 file, got %d", summary.Files)
	}
	if summary.Commands != 1 {
		t.Errorf("Expected 1 command, got %d", summary.Commands)
	}

	store.Clear(featureID)
	actions = store.GetActions(featureID)
	if len(actions) != 0 {
		t.Errorf("Expected 0 actions after clear, got %d", len(actions))
	}
}

func TestExtractAction(t *testing.T) {
	ts := time.Now()

	tests := []struct {
		name        string
		tool        string
		toolInput   string
		expectNil   bool
		expectType  ActionType
		expectTgt   string
	}{
		{
			name:       "empty tool returns nil",
			tool:       "",
			toolInput:  "{}",
			expectNil:  true,
		},
		{
			name:       "bash command",
			tool:       "Bash",
			toolInput:  `{"command": "go test ./..."}`,
			expectType: ActionBash,
			expectTgt:  "go test ./...",
		},
		{
			name:       "write file",
			tool:       "Write",
			toolInput:  `{"file_path": "/home/user/project/internal/test.go"}`,
			expectType: ActionWrite,
			expectTgt:  ".../internal/test.go",
		},
		{
			name:       "edit file",
			tool:       "Edit",
			toolInput:  `{"file_path": "/short/path.go"}`,
			expectType: ActionEdit,
			expectTgt:  "/short/path.go",
		},
		{
			name:       "read file",
			tool:       "Read",
			toolInput:  `{"file_path": "/home/user/file.txt"}`,
			expectType: ActionRead,
			expectTgt:  ".../user/file.txt",
		},
		{
			name:       "webfetch",
			tool:       "WebFetch",
			toolInput:  `{"url": "https://example.com/api/data"}`,
			expectType: ActionWebFetch,
			expectTgt:  "https://example.com/api/data",
		},
		{
			name:       "grep with path",
			tool:       "Grep",
			toolInput:  `{"pattern": "func Test", "path": "/project/internal"}`,
			expectType: ActionGrep,
			expectTgt:  "func Test in /project/internal",
		},
		{
			name:       "glob pattern",
			tool:       "Glob",
			toolInput:  `{"pattern": "**/*.go"}`,
			expectType: ActionGlob,
			expectTgt:  "**/*.go",
		},
		{
			name:       "task agent",
			tool:       "Task",
			toolInput:  `{"subagent_type": "Explore", "description": "Find relevant files"}`,
			expectType: ActionTask,
			expectTgt:  "Explore: Find relevant files",
		},
		{
			name:       "task without subagent",
			tool:       "Task",
			toolInput:  `{"description": "Research implementation"}`,
			expectType: ActionTask,
			expectTgt:  "Research implementation",
		},
		{
			name:      "todowrite ignored",
			tool:      "TodoWrite",
			toolInput: `{}`,
			expectNil: true,
		},
		{
			name:      "websearch ignored",
			tool:      "WebSearch",
			toolInput: `{}`,
			expectNil: true,
		},
		{
			name:       "unknown tool",
			tool:       "CustomTool",
			toolInput:  `{}`,
			expectType: ActionOther,
			expectTgt:  "CustomTool",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := json.RawMessage(tt.toolInput)
			action := ExtractAction(tt.tool, input, ts)

			if tt.expectNil {
				if action != nil {
					t.Errorf("Expected nil action, got %+v", action)
				}
				return
			}

			if action == nil {
				t.Fatal("Expected action, got nil")
			}

			if action.Type != tt.expectType {
				t.Errorf("Expected type %s, got %s", tt.expectType, action.Type)
			}

			if action.Target != tt.expectTgt {
				t.Errorf("Expected target %q, got %q", tt.expectTgt, action.Target)
			}

			if action.Tool != tt.tool {
				t.Errorf("Expected tool %q, got %q", tt.tool, action.Tool)
			}
		})
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		input    string
		maxLen   int
		expected string
	}{
		{"short", 10, "short"},
		{"exactly ten", 11, "exactly ten"},
		{"this is a very long string", 10, "this is..."},
		{"   spaces  ", 20, "spaces"},
		{"has\nnewline", 20, "has newline"},
	}

	for _, tt := range tests {
		got := truncate(tt.input, tt.maxLen)
		if got != tt.expected {
			t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.expected)
		}
	}
}

func TestShortenPath(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"", ""},
		{"/a/b", "/a/b"},
		{"/home/user/project/file.go", ".../project/file.go"},
		{"/very/long/path/to/file.txt", ".../to/file.txt"},
	}

	for _, tt := range tests {
		got := shortenPath(tt.input)
		if got != tt.expected {
			t.Errorf("shortenPath(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestFormatTimeline(t *testing.T) {
	empty := FormatTimeline(nil)
	if empty != "" {
		t.Errorf("FormatTimeline(nil) should return empty string, got %q", empty)
	}

	ts := time.Date(2024, 1, 15, 10, 30, 45, 0, time.UTC)
	actions := []Action{
		{Type: ActionBash, Tool: "Bash", Target: "go test", Timestamp: ts},
		{Type: ActionWrite, Tool: "Write", Target: "file.go", Timestamp: ts.Add(time.Minute)},
	}

	timeline := FormatTimeline(actions)
	if timeline == "" {
		t.Error("FormatTimeline should return non-empty string for actions")
	}

	if !contains(timeline, "BASH") {
		t.Error("Timeline should contain BASH action type")
	}
	if !contains(timeline, "WRITE") {
		t.Error("Timeline should contain WRITE action type")
	}
	if !contains(timeline, "10:30:45") {
		t.Error("Timeline should contain timestamp")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && (s[:len(substr)] == substr || contains(s[1:], substr)))
}

func TestActionIcon(t *testing.T) {
	tests := []struct {
		actionType ActionType
		expectIcon string
	}{
		{ActionTask, "🤖"},
		{ActionAgent, "🤖"},
		{ActionBash, "⚡"},
		{ActionRead, "📖"},
		{ActionWrite, "📝"},
		{ActionEdit, "✏️"},
		{ActionWebFetch, "🌐"},
		{ActionGrep, "🔍"},
		{ActionGlob, "📁"},
		{ActionOther, "•"},
	}

	for _, tt := range tests {
		got := actionIcon(tt.actionType)
		if got != tt.expectIcon {
			t.Errorf("actionIcon(%s) = %q, want %q", tt.actionType, got, tt.expectIcon)
		}
	}
}
