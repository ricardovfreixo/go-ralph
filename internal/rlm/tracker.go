package rlm

import (
	"encoding/json"
	"strings"
	"time"
)

// StreamUsage represents usage data from Claude Code stream-json
type StreamUsage struct {
	InputTokens      int64 `json:"input_tokens,omitempty"`
	OutputTokens     int64 `json:"output_tokens,omitempty"`
	CacheReadTokens  int64 `json:"cache_read_input_tokens,omitempty"`
	CacheWriteTokens int64 `json:"cache_creation_input_tokens,omitempty"`
}

// StreamMessageWithUsage extends StreamMessage to capture usage data
type StreamMessageWithUsage struct {
	Type      string          `json:"type"`
	Subtype   string          `json:"subtype,omitempty"`
	CostUSD   float64         `json:"cost_usd,omitempty"`
	Duration  float64         `json:"duration_ms,omitempty"`
	Usage     *StreamUsage    `json:"usage,omitempty"`
	Message   json.RawMessage `json:"message,omitempty"`
	Content   string          `json:"content,omitempty"`
	Tool      string          `json:"tool,omitempty"`
	ToolInput json.RawMessage `json:"tool_input,omitempty"`
	Result    string          `json:"result,omitempty"`
	IsError   bool            `json:"is_error,omitempty"`
	SessionID string          `json:"session_id,omitempty"`
}

// MessageWithUsage represents a message block that may contain usage
type MessageWithUsage struct {
	Usage *StreamUsage `json:"usage,omitempty"`
}

// Tracker processes stream-json output and extracts token usage and actions
type Tracker struct {
	feature *RecursiveFeature
}

// NewTracker creates a new stream tracker for a feature
func NewTracker(feature *RecursiveFeature) *Tracker {
	return &Tracker{
		feature: feature,
	}
}

// ProcessLine processes a single line of stream-json output
func (t *Tracker) ProcessLine(line string) (*SpawnRequest, error) {
	if line == "" || t.feature == nil {
		return nil, nil
	}

	var msg StreamMessageWithUsage
	if err := json.Unmarshal([]byte(line), &msg); err != nil {
		return nil, nil
	}

	t.extractUsage(&msg)
	t.extractAction(&msg)

	return t.detectSpawnRequest(&msg)
}

// extractUsage pulls token usage from the message
func (t *Tracker) extractUsage(msg *StreamMessageWithUsage) {
	if t.feature.TokenUsage == nil {
		t.feature.TokenUsage = NewTokenUsage()
	}

	if msg.Usage != nil {
		t.feature.TokenUsage.Update(
			msg.Usage.InputTokens,
			msg.Usage.OutputTokens,
			msg.Usage.CacheReadTokens,
			msg.Usage.CacheWriteTokens,
			msg.CostUSD,
		)
		return
	}

	if len(msg.Message) > 0 {
		var messageBlock MessageWithUsage
		if err := json.Unmarshal(msg.Message, &messageBlock); err == nil && messageBlock.Usage != nil {
			t.feature.TokenUsage.Update(
				messageBlock.Usage.InputTokens,
				messageBlock.Usage.OutputTokens,
				messageBlock.Usage.CacheReadTokens,
				messageBlock.Usage.CacheWriteTokens,
				msg.CostUSD,
			)
		}
	}

	if msg.CostUSD > 0 && t.feature.TokenUsage.CostUSD == 0 {
		t.feature.TokenUsage.mu.Lock()
		t.feature.TokenUsage.CostUSD += msg.CostUSD
		t.feature.TokenUsage.mu.Unlock()
	}
}

// extractAction identifies significant actions from the message
func (t *Tracker) extractAction(msg *StreamMessageWithUsage) {
	if msg.Type != "tool_use" {
		return
	}

	action := Action{
		Timestamp: time.Now(),
		Type:      classifyToolAction(msg.Tool),
		Name:      msg.Tool,
	}

	if len(msg.ToolInput) > 0 {
		action.Details = extractToolDetails(msg.Tool, msg.ToolInput)
	}

	if action.Type != "" {
		t.feature.AddAction(action)
	}
}

// classifyToolAction categorizes a tool use into action types
func classifyToolAction(tool string) string {
	toolLower := strings.ToLower(tool)

	switch {
	case strings.Contains(toolLower, "task") || strings.Contains(toolLower, "agent"):
		return "agent_spawn"
	case toolLower == "webfetch" || toolLower == "websearch":
		return "web_fetch"
	case toolLower == "bash":
		return "command"
	case toolLower == "write" || toolLower == "edit":
		return "file_modify"
	case toolLower == "read":
		return "file_read"
	case toolLower == "glob" || toolLower == "grep":
		return "search"
	default:
		return ""
	}
}

// extractToolDetails extracts relevant details from tool input
func extractToolDetails(tool string, input json.RawMessage) string {
	var data map[string]interface{}
	if err := json.Unmarshal(input, &data); err != nil {
		return ""
	}

	switch strings.ToLower(tool) {
	case "bash":
		if cmd, ok := data["command"].(string); ok {
			if len(cmd) > 100 {
				return cmd[:100] + "..."
			}
			return cmd
		}
	case "write", "edit":
		if path, ok := data["file_path"].(string); ok {
			return path
		}
	case "read":
		if path, ok := data["file_path"].(string); ok {
			return path
		}
	case "webfetch", "websearch":
		if url, ok := data["url"].(string); ok {
			return url
		}
		if query, ok := data["query"].(string); ok {
			return query
		}
	case "task":
		if prompt, ok := data["prompt"].(string); ok {
			if len(prompt) > 100 {
				return prompt[:100] + "..."
			}
			return prompt
		}
	}

	return ""
}

// detectSpawnRequest checks if the message contains a spawn request
func (t *Tracker) detectSpawnRequest(msg *StreamMessageWithUsage) (*SpawnRequest, error) {
	if msg.Type != "tool_use" {
		return nil, nil
	}

	if msg.Tool != "ralph_spawn_feature" {
		return nil, nil
	}

	if !t.feature.CanSpawn() {
		return nil, ErrMaxDepthExceeded
	}

	var req SpawnRequest
	if err := json.Unmarshal(msg.ToolInput, &req); err != nil {
		return nil, ErrInvalidSpawnData
	}

	if req.Title == "" {
		return nil, ErrInvalidSpawnData
	}

	return &req, nil
}

// GetFeature returns the tracked feature
func (t *Tracker) GetFeature() *RecursiveFeature {
	return t.feature
}
