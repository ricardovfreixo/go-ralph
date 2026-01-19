package actions

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"
)

type ActionType string

const (
	ActionTask     ActionType = "task"
	ActionAgent    ActionType = "agent"
	ActionBash     ActionType = "bash"
	ActionRead     ActionType = "read"
	ActionWrite    ActionType = "write"
	ActionEdit     ActionType = "edit"
	ActionWebFetch ActionType = "webfetch"
	ActionGrep     ActionType = "grep"
	ActionGlob     ActionType = "glob"
	ActionOther    ActionType = "other"
)

type Action struct {
	Type      ActionType `json:"type"`
	Tool      string     `json:"tool"`
	Target    string     `json:"target"`
	Timestamp time.Time  `json:"timestamp"`
	Raw       string     `json:"raw,omitempty"`
}

type ActionSummary struct {
	Files    int
	Commands int
	Agents   int
	Reads    int
	Fetches  int
	Searches int
}

func (s ActionSummary) String() string {
	var parts []string
	if s.Files > 0 {
		parts = append(parts, fmt.Sprintf("%d files", s.Files))
	}
	if s.Commands > 0 {
		parts = append(parts, fmt.Sprintf("%d cmds", s.Commands))
	}
	if s.Agents > 0 {
		parts = append(parts, fmt.Sprintf("%d agents", s.Agents))
	}
	if s.Fetches > 0 {
		parts = append(parts, fmt.Sprintf("%d fetches", s.Fetches))
	}
	if s.Searches > 0 {
		parts = append(parts, fmt.Sprintf("%d searches", s.Searches))
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, ", ")
}

func (s ActionSummary) IsEmpty() bool {
	return s.Files == 0 && s.Commands == 0 && s.Agents == 0 && s.Reads == 0 && s.Fetches == 0 && s.Searches == 0
}

type ActionStore struct {
	mu      sync.RWMutex
	actions map[string][]Action
}

func NewActionStore() *ActionStore {
	return &ActionStore{
		actions: make(map[string][]Action),
	}
}

func (s *ActionStore) AddAction(featureID string, action Action) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.actions[featureID] = append(s.actions[featureID], action)
}

func (s *ActionStore) GetActions(featureID string) []Action {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]Action, len(s.actions[featureID]))
	copy(result, s.actions[featureID])
	return result
}

func (s *ActionStore) GetSummary(featureID string) ActionSummary {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var summary ActionSummary
	for _, action := range s.actions[featureID] {
		switch action.Type {
		case ActionWrite, ActionEdit:
			summary.Files++
		case ActionBash:
			summary.Commands++
		case ActionTask, ActionAgent:
			summary.Agents++
		case ActionRead:
			summary.Reads++
		case ActionWebFetch:
			summary.Fetches++
		case ActionGrep, ActionGlob:
			summary.Searches++
		}
	}
	return summary
}

func (s *ActionStore) Clear(featureID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.actions, featureID)
}

func (s *ActionStore) ClearAll() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.actions = make(map[string][]Action)
}

type ToolInput struct {
	Command     string `json:"command,omitempty"`
	FilePath    string `json:"file_path,omitempty"`
	Path        string `json:"path,omitempty"`
	URL         string `json:"url,omitempty"`
	Pattern     string `json:"pattern,omitempty"`
	Prompt      string `json:"prompt,omitempty"`
	SubagentType string `json:"subagent_type,omitempty"`
	Description string `json:"description,omitempty"`
	Content     string `json:"content,omitempty"`
	OldString   string `json:"old_string,omitempty"`
	NewString   string `json:"new_string,omitempty"`
}

func ExtractAction(tool string, toolInput json.RawMessage, timestamp time.Time) *Action {
	if tool == "" {
		return nil
	}

	action := &Action{
		Tool:      tool,
		Timestamp: timestamp,
	}

	var input ToolInput
	if len(toolInput) > 0 {
		json.Unmarshal(toolInput, &input)
	}

	switch strings.ToLower(tool) {
	case "task":
		action.Type = ActionTask
		if input.SubagentType != "" {
			action.Target = fmt.Sprintf("%s: %s", input.SubagentType, truncate(input.Description, 40))
		} else if input.Description != "" {
			action.Target = truncate(input.Description, 50)
		} else {
			action.Target = truncate(input.Prompt, 50)
		}

	case "bash":
		action.Type = ActionBash
		action.Target = truncate(input.Command, 60)

	case "read":
		action.Type = ActionRead
		action.Target = shortenPath(input.FilePath)

	case "write":
		action.Type = ActionWrite
		action.Target = shortenPath(input.FilePath)

	case "edit":
		action.Type = ActionEdit
		action.Target = shortenPath(input.FilePath)

	case "webfetch":
		action.Type = ActionWebFetch
		action.Target = truncate(input.URL, 60)

	case "grep":
		action.Type = ActionGrep
		target := input.Pattern
		if input.Path != "" {
			target = fmt.Sprintf("%s in %s", input.Pattern, shortenPath(input.Path))
		}
		action.Target = truncate(target, 60)

	case "glob":
		action.Type = ActionGlob
		action.Target = truncate(input.Pattern, 60)

	case "todowrite", "websearch", "askuserquestion", "skill", "notebookedit":
		return nil

	default:
		action.Type = ActionOther
		action.Target = tool
	}

	return action
}

func truncate(s string, maxLen int) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func shortenPath(path string) string {
	if path == "" {
		return ""
	}
	parts := strings.Split(path, "/")
	if len(parts) <= 3 {
		return path
	}
	return ".../" + strings.Join(parts[len(parts)-2:], "/")
}

func FormatTimeline(acts []Action) string {
	if len(acts) == 0 {
		return ""
	}

	var lines []string
	for _, act := range acts {
		icon := actionIcon(act.Type)
		typeStr := strings.ToUpper(string(act.Type))
		timestamp := act.Timestamp.Format("15:04:05")

		line := fmt.Sprintf("[%s] %s %s: %s", timestamp, icon, typeStr, act.Target)
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

func actionIcon(t ActionType) string {
	switch t {
	case ActionTask, ActionAgent:
		return "🤖"
	case ActionBash:
		return "⚡"
	case ActionRead:
		return "📖"
	case ActionWrite:
		return "📝"
	case ActionEdit:
		return "✏️"
	case ActionWebFetch:
		return "🌐"
	case ActionGrep:
		return "🔍"
	case ActionGlob:
		return "📁"
	default:
		return "•"
	}
}
