package usage

import (
	"encoding/json"
	"fmt"
	"sync"
)

// TokenUsage tracks token consumption for a runner instance
type TokenUsage struct {
	mu sync.RWMutex

	InputTokens      int64   `json:"input_tokens"`
	OutputTokens     int64   `json:"output_tokens"`
	CacheReadTokens  int64   `json:"cache_read_tokens,omitempty"`
	CacheWriteTokens int64   `json:"cache_write_tokens,omitempty"`
	TotalTokens      int64   `json:"total_tokens"`
	CostUSD          float64 `json:"cost_usd,omitempty"`
}

// StreamUsage represents usage data from Claude Code stream-json format
type StreamUsage struct {
	InputTokens      int64 `json:"input_tokens,omitempty"`
	OutputTokens     int64 `json:"output_tokens,omitempty"`
	CacheReadTokens  int64 `json:"cache_read_input_tokens,omitempty"`
	CacheWriteTokens int64 `json:"cache_creation_input_tokens,omitempty"`
}

// StreamMessage represents a stream-json message that may contain usage data
type StreamMessage struct {
	Type    string          `json:"type"`
	Usage   *StreamUsage    `json:"usage,omitempty"`
	CostUSD float64         `json:"cost_usd,omitempty"`
	Message json.RawMessage `json:"message,omitempty"`
}

// NestedMessage represents the message block that may contain usage
type NestedMessage struct {
	Usage *StreamUsage `json:"usage,omitempty"`
}

// New creates a new TokenUsage tracker
func New() *TokenUsage {
	return &TokenUsage{}
}

// ParseLine extracts usage data from a stream-json line and updates the tracker
func (t *TokenUsage) ParseLine(line string) bool {
	if line == "" {
		return false
	}

	var msg StreamMessage
	if err := json.Unmarshal([]byte(line), &msg); err != nil {
		return false
	}

	return t.parseMessage(&msg)
}

// parseMessage extracts usage from a parsed message
func (t *TokenUsage) parseMessage(msg *StreamMessage) bool {
	var updated bool

	// Check top-level usage field
	if msg.Usage != nil {
		t.addUsage(msg.Usage, msg.CostUSD)
		updated = true
	}

	// Check nested message.usage field
	if len(msg.Message) > 0 {
		var nested NestedMessage
		if err := json.Unmarshal(msg.Message, &nested); err == nil && nested.Usage != nil {
			t.addUsage(nested.Usage, msg.CostUSD)
			updated = true
		}
	}

	// Handle cost_usd without explicit usage (result messages)
	if !updated && msg.CostUSD > 0 {
		t.mu.Lock()
		t.CostUSD += msg.CostUSD
		t.mu.Unlock()
		updated = true
	}

	return updated
}

// addUsage adds stream usage data to the tracker
func (t *TokenUsage) addUsage(su *StreamUsage, cost float64) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.InputTokens += su.InputTokens
	t.OutputTokens += su.OutputTokens
	t.CacheReadTokens += su.CacheReadTokens
	t.CacheWriteTokens += su.CacheWriteTokens
	t.TotalTokens = t.InputTokens + t.OutputTokens
	t.CostUSD += cost
}

// Add merges another TokenUsage into this one
func (t *TokenUsage) Add(other *TokenUsage) {
	if other == nil {
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	other.mu.RLock()
	defer other.mu.RUnlock()

	t.InputTokens += other.InputTokens
	t.OutputTokens += other.OutputTokens
	t.CacheReadTokens += other.CacheReadTokens
	t.CacheWriteTokens += other.CacheWriteTokens
	t.TotalTokens += other.TotalTokens
	t.CostUSD += other.CostUSD
}

// Snapshot returns a copy of the current usage (thread-safe)
func (t *TokenUsage) Snapshot() TokenUsage {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return TokenUsage{
		InputTokens:      t.InputTokens,
		OutputTokens:     t.OutputTokens,
		CacheReadTokens:  t.CacheReadTokens,
		CacheWriteTokens: t.CacheWriteTokens,
		TotalTokens:      t.TotalTokens,
		CostUSD:          t.CostUSD,
	}
}

// Reset clears all usage data
func (t *TokenUsage) Reset() {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.InputTokens = 0
	t.OutputTokens = 0
	t.CacheReadTokens = 0
	t.CacheWriteTokens = 0
	t.TotalTokens = 0
	t.CostUSD = 0
}

// IsEmpty returns true if no tokens have been recorded
func (t *TokenUsage) IsEmpty() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.TotalTokens == 0 && t.CostUSD == 0
}

// FormatTokens formats a token count with k/M suffixes
func FormatTokens(n int64) string {
	if n == 0 {
		return "0"
	}
	if n >= 1000000 {
		return fmt.Sprintf("%.1fM", float64(n)/1000000)
	}
	if n >= 1000 {
		return fmt.Sprintf("%.1fk", float64(n)/1000)
	}
	return fmt.Sprintf("%d", n)
}

// FormatCost formats a USD cost
func FormatCost(cost float64) string {
	if cost == 0 {
		return ""
	}
	if cost < 0.01 {
		return fmt.Sprintf("$%.4f", cost)
	}
	return fmt.Sprintf("$%.2f", cost)
}

// Compact returns a compact string representation (e.g., "1.2k in / 0.8k out")
func (t *TokenUsage) Compact() string {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.TotalTokens == 0 {
		return ""
	}
	return fmt.Sprintf("%s↓ %s↑", FormatTokens(t.InputTokens), FormatTokens(t.OutputTokens))
}

// Detailed returns a detailed breakdown string
func (t *TokenUsage) Detailed() string {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.TotalTokens == 0 && t.CostUSD == 0 {
		return "No usage recorded"
	}

	result := fmt.Sprintf("Input: %s  Output: %s  Total: %s",
		FormatTokens(t.InputTokens),
		FormatTokens(t.OutputTokens),
		FormatTokens(t.TotalTokens))

	if t.CacheReadTokens > 0 || t.CacheWriteTokens > 0 {
		result += fmt.Sprintf("\nCache: %s read, %s write",
			FormatTokens(t.CacheReadTokens),
			FormatTokens(t.CacheWriteTokens))
	}

	if t.CostUSD > 0 {
		result += fmt.Sprintf("\nCost: %s", FormatCost(t.CostUSD))
	}

	return result
}

// GetEstimatedCost returns the cost from stream or calculates it from tokens
func (t *TokenUsage) GetEstimatedCost(model string) float64 {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.CostUSD > 0 {
		return t.CostUSD
	}
	return EstimateCost(t.InputTokens, t.OutputTokens, t.CacheWriteTokens, t.CacheReadTokens, model)
}

// CompactWithCost returns compact string with cost
func (t *TokenUsage) CompactWithCost(model string) string {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.TotalTokens == 0 {
		return ""
	}

	cost := t.CostUSD
	if cost == 0 {
		cost = EstimateCost(t.InputTokens, t.OutputTokens, t.CacheWriteTokens, t.CacheReadTokens, model)
	}

	if cost > 0 {
		return fmt.Sprintf("%s↓ %s↑ %s", FormatTokens(t.InputTokens), FormatTokens(t.OutputTokens), FormatCost(cost))
	}
	return fmt.Sprintf("%s↓ %s↑", FormatTokens(t.InputTokens), FormatTokens(t.OutputTokens))
}
