package budget

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/vx/ralph-go/internal/usage"
)

// Budget represents a spending limit in either tokens or USD
type Budget struct {
	mu       sync.RWMutex
	Tokens   int64   `json:"tokens,omitempty"`
	USD      float64 `json:"usd,omitempty"`
	IsSet    bool    `json:"is_set"`
}

// BudgetStatus tracks current spend against budget
type BudgetStatus struct {
	Budget       *Budget
	UsedTokens   int64
	UsedUSD      float64
	Percent      float64
	AtThreshold  bool
	OverBudget   bool
	Acknowledged bool
}

const ThresholdPercent = 90.0

var (
	// Matches: Budget: $5.00, Budget: $5 (dollar sign required)
	dollarRegex = regexp.MustCompile(`(?i)^budget:\s*\$([\d.]+)$`)
	// Matches: Budget: 10000, Budget: 10k, Budget: 1.5M
	tokensRegex = regexp.MustCompile(`(?i)^budget:\s*([\d.]+)\s*([kKmM])?(?:\s*tokens?)?$`)
	// Matches: Tokens: 10000, Tokens: 10k
	tokensAltRegex = regexp.MustCompile(`(?i)^tokens:\s*([\d.]+)\s*([kKmM])?$`)
)

// New creates a new empty budget
func New() *Budget {
	return &Budget{}
}

// NewWithTokens creates a budget with a token limit
func NewWithTokens(tokens int64) *Budget {
	return &Budget{
		Tokens: tokens,
		IsSet:  true,
	}
}

// NewWithUSD creates a budget with a dollar limit
func NewWithUSD(usd float64) *Budget {
	return &Budget{
		USD:   usd,
		IsSet: true,
	}
}

// ParseLine parses a budget specification from a line of text
// Returns true if a budget was parsed
func (b *Budget) ParseLine(line string) bool {
	line = strings.TrimSpace(line)
	if line == "" {
		return false
	}

	// Try dollar format first: Budget: $5.00
	if matches := dollarRegex.FindStringSubmatch(line); matches != nil {
		value, err := strconv.ParseFloat(matches[1], 64)
		if err == nil && value > 0 {
			b.mu.Lock()
			b.USD = value
			b.Tokens = 0
			b.IsSet = true
			b.mu.Unlock()
			return true
		}
	}

	// Try tokens alt format: Tokens: 10000
	if matches := tokensAltRegex.FindStringSubmatch(line); matches != nil {
		value, err := strconv.ParseFloat(matches[1], 64)
		if err == nil && value > 0 {
			multiplier := getMultiplier(matches[2])
			b.mu.Lock()
			b.Tokens = int64(value * multiplier)
			b.USD = 0
			b.IsSet = true
			b.mu.Unlock()
			return true
		}
	}

	// Try tokens format: Budget: 10000, Budget: 10k
	if matches := tokensRegex.FindStringSubmatch(line); matches != nil {
		// Check if this looks like a dollar amount (has decimal and no suffix)
		if strings.Contains(matches[1], ".") && matches[2] == "" {
			value, err := strconv.ParseFloat(matches[1], 64)
			if err == nil && value > 0 && value < 1000 {
				// Likely a dollar amount like 5.00
				b.mu.Lock()
				b.USD = value
				b.Tokens = 0
				b.IsSet = true
				b.mu.Unlock()
				return true
			}
		}
		value, err := strconv.ParseFloat(matches[1], 64)
		if err == nil && value > 0 {
			multiplier := getMultiplier(matches[2])
			b.mu.Lock()
			b.Tokens = int64(value * multiplier)
			b.USD = 0
			b.IsSet = true
			b.mu.Unlock()
			return true
		}
	}

	return false
}

func getMultiplier(suffix string) float64 {
	switch strings.ToLower(suffix) {
	case "k":
		return 1000
	case "m":
		return 1000000
	default:
		return 1
	}
}

// ParseBudget parses a budget from content text (searches all lines)
func ParseBudget(content string) *Budget {
	b := New()
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		if b.ParseLine(line) {
			return b
		}
	}
	return b
}

// Copy returns a thread-safe copy of the budget
func (b *Budget) Copy() Budget {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return Budget{
		Tokens: b.Tokens,
		USD:    b.USD,
		IsSet:  b.IsSet,
	}
}

// HasTokenLimit returns true if a token limit is set
func (b *Budget) HasTokenLimit() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.IsSet && b.Tokens > 0
}

// HasUSDLimit returns true if a USD limit is set
func (b *Budget) HasUSDLimit() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.IsSet && b.USD > 0
}

// CheckUsage checks current usage against budget and returns status
func (b *Budget) CheckUsage(u *usage.TokenUsage) BudgetStatus {
	b.mu.RLock()
	defer b.mu.RUnlock()

	status := BudgetStatus{
		Budget: b,
	}

	if u == nil || !b.IsSet {
		return status
	}

	snapshot := u.Snapshot()
	status.UsedTokens = snapshot.TotalTokens
	status.UsedUSD = snapshot.CostUSD

	if b.Tokens > 0 {
		status.Percent = float64(snapshot.TotalTokens) / float64(b.Tokens) * 100
		status.AtThreshold = status.Percent >= ThresholdPercent
		status.OverBudget = snapshot.TotalTokens >= b.Tokens
	} else if b.USD > 0 {
		status.Percent = snapshot.CostUSD / b.USD * 100
		status.AtThreshold = status.Percent >= ThresholdPercent
		status.OverBudget = snapshot.CostUSD >= b.USD
	}

	return status
}

// String returns a human-readable string representation
func (b *Budget) String() string {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if !b.IsSet {
		return ""
	}

	if b.USD > 0 {
		return fmt.Sprintf("$%.2f", b.USD)
	}
	if b.Tokens > 0 {
		return usage.FormatTokens(b.Tokens)
	}
	return ""
}

// FormatStatus returns a formatted string showing usage vs budget
func FormatStatus(used, total float64, isUSD bool) string {
	if total <= 0 {
		return ""
	}
	percent := used / total * 100
	if isUSD {
		return fmt.Sprintf("$%.2f / $%.2f (%.0f%%)", used, total, percent)
	}
	return fmt.Sprintf("%s / %s (%.0f%%)", usage.FormatTokens(int64(used)), usage.FormatTokens(int64(total)), percent)
}

// FormatStatusCompact returns a compact status string
func FormatStatusCompact(status BudgetStatus) string {
	if status.Budget == nil || !status.Budget.IsSet {
		return ""
	}

	if status.Budget.USD > 0 {
		return fmt.Sprintf("$%.2f/$%.2f", status.UsedUSD, status.Budget.USD)
	}
	if status.Budget.Tokens > 0 {
		return fmt.Sprintf("%s/%s", usage.FormatTokens(status.UsedTokens), usage.FormatTokens(status.Budget.Tokens))
	}
	return ""
}
