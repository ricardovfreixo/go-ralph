package context

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

const (
	DefaultBaseBudget     = 100000 // Default base context budget (tokens)
	DefaultMinBudget      = 10000  // Minimum budget at any depth
	SummarizationRatio    = 0.8    // Trigger summarization at 80% budget used
	OverflowTruncateRatio = 0.95   // Hard truncate at 95% budget
)

// Budget tracks context budget for a feature
type Budget struct {
	mu sync.RWMutex

	BaseBudget    int64 `json:"base_budget"`    // Original budget at depth 0
	CurrentBudget int64 `json:"current_budget"` // Budget at current depth
	UsedTokens    int64 `json:"used_tokens"`    // Tokens used in context
	Depth         int   `json:"depth"`          // Current recursion depth
}

// New creates a new context budget with default base
func New() *Budget {
	return &Budget{
		BaseBudget:    DefaultBaseBudget,
		CurrentBudget: DefaultBaseBudget,
		Depth:         0,
	}
}

// NewWithBase creates a context budget with a custom base
func NewWithBase(base int64) *Budget {
	if base <= 0 {
		base = DefaultBaseBudget
	}
	return &Budget{
		BaseBudget:    base,
		CurrentBudget: base,
		Depth:         0,
	}
}

// NewForDepth creates a budget calculated for a specific depth
// Formula: base_budget / (depth + 1) with minimum floor
func NewForDepth(base int64, depth int) *Budget {
	if base <= 0 {
		base = DefaultBaseBudget
	}
	if depth < 0 {
		depth = 0
	}

	current := CalculateBudgetForDepth(base, depth)

	return &Budget{
		BaseBudget:    base,
		CurrentBudget: current,
		Depth:         depth,
	}
}

// CalculateBudgetForDepth calculates the context budget for a given depth
// Formula: base_budget / (depth + 1), with minimum floor
func CalculateBudgetForDepth(base int64, depth int) int64 {
	if depth < 0 {
		depth = 0
	}

	// Formula: base / (depth + 1)
	budget := base / int64(depth+1)

	// Ensure minimum budget
	if budget < DefaultMinBudget {
		budget = DefaultMinBudget
	}

	return budget
}

// CalculateChildBudget calculates the budget for a child at depth+1
func (b *Budget) CalculateChildBudget() int64 {
	b.mu.RLock()
	defer b.mu.RUnlock()

	return CalculateBudgetForDepth(b.BaseBudget, b.Depth+1)
}

// GetCurrentBudget returns the current budget
func (b *Budget) GetCurrentBudget() int64 {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.CurrentBudget
}

// GetUsedTokens returns the number of tokens used
func (b *Budget) GetUsedTokens() int64 {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.UsedTokens
}

// GetRemaining returns the remaining budget
func (b *Budget) GetRemaining() int64 {
	b.mu.RLock()
	defer b.mu.RUnlock()
	remaining := b.CurrentBudget - b.UsedTokens
	if remaining < 0 {
		return 0
	}
	return remaining
}

// GetUsagePercent returns the percentage of budget used (0.0 to 1.0)
func (b *Budget) GetUsagePercent() float64 {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if b.CurrentBudget == 0 {
		return 0
	}
	return float64(b.UsedTokens) / float64(b.CurrentBudget)
}

// AddUsage records token usage against the budget
func (b *Budget) AddUsage(tokens int64) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.UsedTokens += tokens
}

// SetUsage sets the total used tokens (for tracking context size)
func (b *Budget) SetUsage(tokens int64) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.UsedTokens = tokens
}

// SetCurrentBudget overrides the current budget (for PRD Context: override)
func (b *Budget) SetCurrentBudget(budget int64) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if budget > 0 {
		b.CurrentBudget = budget
	}
}

// NeedsSummarization returns true if context should be summarized
func (b *Budget) NeedsSummarization() bool {
	return b.GetUsagePercent() >= SummarizationRatio
}

// IsOverBudget returns true if context exceeds budget
func (b *Budget) IsOverBudget() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.UsedTokens > b.CurrentBudget
}

// NeedsTruncation returns true if context must be truncated
func (b *Budget) NeedsTruncation() bool {
	return b.GetUsagePercent() >= OverflowTruncateRatio
}

// Snapshot returns a copy of the budget
func (b *Budget) Snapshot() Budget {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return Budget{
		BaseBudget:    b.BaseBudget,
		CurrentBudget: b.CurrentBudget,
		UsedTokens:    b.UsedTokens,
		Depth:         b.Depth,
	}
}

// Reset clears usage tracking
func (b *Budget) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.UsedTokens = 0
}

// FormatBudget formats a token count for display
func FormatBudget(tokens int64) string {
	if tokens >= 1000000 {
		return fmt.Sprintf("%.1fM", float64(tokens)/1000000)
	}
	if tokens >= 1000 {
		return fmt.Sprintf("%.0fk", float64(tokens)/1000)
	}
	return fmt.Sprintf("%d", tokens)
}

// Status returns a formatted status string
func (b *Budget) Status() string {
	b.mu.RLock()
	defer b.mu.RUnlock()

	percent := 0.0
	if b.CurrentBudget > 0 {
		percent = float64(b.UsedTokens) / float64(b.CurrentBudget) * 100
	}

	return fmt.Sprintf("%s/%s (%.0f%%)",
		FormatBudget(b.UsedTokens),
		FormatBudget(b.CurrentBudget),
		percent)
}

// CompactStatus returns a compact status
func (b *Budget) CompactStatus() string {
	b.mu.RLock()
	defer b.mu.RUnlock()

	return fmt.Sprintf("ctx:%s/%s",
		FormatBudget(b.UsedTokens),
		FormatBudget(b.CurrentBudget))
}

var contextRegex = regexp.MustCompile(`(?i)^context:\s*(.+)$`)

// ParseContextLine parses a context budget from a PRD line
// Supports formats: Context: 50000, Context: 50k, Context: 1M
func ParseContextLine(line string) (int64, bool) {
	matches := contextRegex.FindStringSubmatch(line)
	if matches == nil {
		return 0, false
	}

	return ParseContextValue(matches[1])
}

// ParseContextValue parses a context budget value
// Supports formats: 50000, 50k, 1.5M, 100k tokens
func ParseContextValue(value string) (int64, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, false
	}

	// Remove optional "tokens" suffix
	value = strings.TrimSuffix(strings.ToLower(value), " tokens")
	value = strings.TrimSuffix(value, " token")
	value = strings.TrimSpace(value)

	multiplier := 1.0
	if strings.HasSuffix(strings.ToLower(value), "k") {
		multiplier = 1000
		value = value[:len(value)-1]
	} else if strings.HasSuffix(strings.ToLower(value), "m") {
		multiplier = 1000000
		value = value[:len(value)-1]
	}

	val, err := strconv.ParseFloat(value, 64)
	if err != nil || val <= 0 {
		return 0, false
	}

	return int64(val * multiplier), true
}
