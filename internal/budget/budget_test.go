package budget

import (
	"testing"

	"github.com/vx/ralph-go/internal/usage"
)

func TestNewBudget(t *testing.T) {
	b := New()
	if b.IsSet {
		t.Error("New budget should not be set")
	}
	if b.Tokens != 0 {
		t.Error("New budget should have 0 tokens")
	}
	if b.USD != 0 {
		t.Error("New budget should have 0 USD")
	}
}

func TestNewWithTokens(t *testing.T) {
	b := NewWithTokens(10000)
	if !b.IsSet {
		t.Error("Budget should be set")
	}
	if b.Tokens != 10000 {
		t.Errorf("Expected 10000 tokens, got %d", b.Tokens)
	}
	if b.USD != 0 {
		t.Error("USD should be 0")
	}
}

func TestNewWithUSD(t *testing.T) {
	b := NewWithUSD(5.50)
	if !b.IsSet {
		t.Error("Budget should be set")
	}
	if b.USD != 5.50 {
		t.Errorf("Expected 5.50 USD, got %f", b.USD)
	}
	if b.Tokens != 0 {
		t.Error("Tokens should be 0")
	}
}

func TestParseLine_DollarFormat(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		wantUSD  float64
		wantParsed bool
	}{
		{"dollar sign", "Budget: $5.00", 5.00, true},
		{"dollar sign no cents", "Budget: $5", 5.00, true},
		{"dollar sign with space", "Budget: $ 5.00", 0, false},
		{"lowercase", "budget: $10.50", 10.50, true},
		{"mixed case", "Budget: $2.99", 2.99, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := New()
			parsed := b.ParseLine(tt.line)
			if parsed != tt.wantParsed {
				t.Errorf("ParseLine() = %v, want %v", parsed, tt.wantParsed)
			}
			if parsed && b.USD != tt.wantUSD {
				t.Errorf("USD = %f, want %f", b.USD, tt.wantUSD)
			}
		})
	}
}

func TestParseLine_TokenFormat(t *testing.T) {
	tests := []struct {
		name       string
		line       string
		wantTokens int64
		wantParsed bool
	}{
		{"simple number", "Budget: 10000", 10000, true},
		{"k suffix", "Budget: 10k", 10000, true},
		{"K suffix uppercase", "Budget: 10K", 10000, true},
		{"M suffix", "Budget: 1M", 1000000, true},
		{"m suffix lowercase", "Budget: 1m", 1000000, true},
		{"decimal with k", "Budget: 1.5k", 1500, true},
		{"decimal with M", "Budget: 1.5M", 1500000, true},
		{"with tokens suffix", "Budget: 10000 tokens", 10000, true},
		{"tokens alt format", "Tokens: 50000", 50000, true},
		{"tokens alt with k", "Tokens: 50k", 50000, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := New()
			parsed := b.ParseLine(tt.line)
			if parsed != tt.wantParsed {
				t.Errorf("ParseLine() = %v, want %v", parsed, tt.wantParsed)
			}
			if parsed && b.Tokens != tt.wantTokens {
				t.Errorf("Tokens = %d, want %d", b.Tokens, tt.wantTokens)
			}
		})
	}
}

func TestParseLine_DecimalAsUSD(t *testing.T) {
	b := New()
	parsed := b.ParseLine("Budget: 5.00")
	if !parsed {
		t.Error("Should parse 5.00 as USD")
	}
	if b.USD != 5.00 {
		t.Errorf("Expected USD 5.00, got %f", b.USD)
	}
	if b.Tokens != 0 {
		t.Error("Tokens should be 0")
	}
}

func TestParseLine_Invalid(t *testing.T) {
	tests := []string{
		"",
		"Budget:",
		"Budget: ",
		"Budget: -100",
		"Not a budget line",
		"Model: sonnet",
	}

	for _, line := range tests {
		t.Run(line, func(t *testing.T) {
			b := New()
			if b.ParseLine(line) {
				t.Errorf("ParseLine(%q) should return false", line)
			}
		})
	}
}

func TestParseBudget(t *testing.T) {
	content := `# My PRD

Budget: $10.00

## Feature 1
Some description
`
	b := ParseBudget(content)
	if !b.IsSet {
		t.Error("Budget should be set from content")
	}
	if b.USD != 10.00 {
		t.Errorf("Expected USD 10.00, got %f", b.USD)
	}
}

func TestParseBudget_NoMatch(t *testing.T) {
	content := `# My PRD

## Feature 1
Some description
`
	b := ParseBudget(content)
	if b.IsSet {
		t.Error("Budget should not be set")
	}
}

func TestCopy(t *testing.T) {
	b := NewWithUSD(5.00)
	copy := b.Copy()

	if copy.USD != b.USD {
		t.Error("Copy should have same USD")
	}
	if copy.IsSet != b.IsSet {
		t.Error("Copy should have same IsSet")
	}

	b.USD = 10.00
	if copy.USD == 10.00 {
		t.Error("Copy should be independent")
	}
}

func TestHasTokenLimit(t *testing.T) {
	b := New()
	if b.HasTokenLimit() {
		t.Error("New budget should not have token limit")
	}

	b = NewWithTokens(10000)
	if !b.HasTokenLimit() {
		t.Error("Budget with tokens should have token limit")
	}

	b = NewWithUSD(5.00)
	if b.HasTokenLimit() {
		t.Error("Budget with only USD should not have token limit")
	}
}

func TestHasUSDLimit(t *testing.T) {
	b := New()
	if b.HasUSDLimit() {
		t.Error("New budget should not have USD limit")
	}

	b = NewWithUSD(5.00)
	if !b.HasUSDLimit() {
		t.Error("Budget with USD should have USD limit")
	}

	b = NewWithTokens(10000)
	if b.HasUSDLimit() {
		t.Error("Budget with only tokens should not have USD limit")
	}
}

func TestCheckUsage_Tokens(t *testing.T) {
	b := NewWithTokens(10000)
	u := usage.New()

	status := b.CheckUsage(u)
	if status.AtThreshold {
		t.Error("Empty usage should not be at threshold")
	}
	if status.OverBudget {
		t.Error("Empty usage should not be over budget")
	}

	u.ParseLine(`{"type":"assistant","usage":{"input_tokens":4500,"output_tokens":500}}`)
	status = b.CheckUsage(u)
	if status.Percent != 50 {
		t.Errorf("Expected 50%%, got %.0f%%", status.Percent)
	}
	if status.AtThreshold {
		t.Error("50% should not be at threshold")
	}

	u.ParseLine(`{"type":"assistant","usage":{"input_tokens":4000,"output_tokens":0}}`)
	status = b.CheckUsage(u)
	if status.Percent != 90 {
		t.Errorf("Expected 90%%, got %.0f%%", status.Percent)
	}
	if !status.AtThreshold {
		t.Error("90% should be at threshold")
	}
	if status.OverBudget {
		t.Error("90% should not be over budget")
	}
}

func TestCheckUsage_USD(t *testing.T) {
	b := NewWithUSD(1.00)
	u := usage.New()

	u.ParseLine(`{"type":"assistant","cost_usd":0.50}`)
	status := b.CheckUsage(u)
	if status.Percent != 50 {
		t.Errorf("Expected 50%%, got %.0f%%", status.Percent)
	}

	u.ParseLine(`{"type":"assistant","cost_usd":0.40}`)
	status = b.CheckUsage(u)
	if !status.AtThreshold {
		t.Error("90% should be at threshold")
	}
}

func TestCheckUsage_NilUsage(t *testing.T) {
	b := NewWithTokens(10000)
	status := b.CheckUsage(nil)
	if status.AtThreshold || status.OverBudget {
		t.Error("Nil usage should not trigger threshold or over budget")
	}
}

func TestCheckUsage_NoBudget(t *testing.T) {
	b := New()
	u := usage.New()
	u.ParseLine(`{"type":"assistant","usage":{"input_tokens":100000,"output_tokens":50000}}`)

	status := b.CheckUsage(u)
	if status.AtThreshold || status.OverBudget {
		t.Error("No budget should not trigger threshold or over budget")
	}
}

func TestString(t *testing.T) {
	tests := []struct {
		name string
		b    *Budget
		want string
	}{
		{"empty", New(), ""},
		{"usd", NewWithUSD(5.50), "$5.50"},
		{"tokens", NewWithTokens(10000), "10.0k"},
		{"large tokens", NewWithTokens(1500000), "1.5M"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.b.String()
			if got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFormatStatus(t *testing.T) {
	tests := []struct {
		name   string
		used   float64
		total  float64
		isUSD  bool
		want   string
	}{
		{"zero total", 0, 0, false, ""},
		{"usd", 2.50, 5.00, true, "$2.50 / $5.00 (50%)"},
		{"tokens", 5000, 10000, false, "5.0k / 10.0k (50%)"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatStatus(tt.used, tt.total, tt.isUSD)
			if got != tt.want {
				t.Errorf("FormatStatus() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFormatStatusCompact(t *testing.T) {
	status := BudgetStatus{
		Budget:     NewWithUSD(5.00),
		UsedUSD:    2.50,
		UsedTokens: 5000,
	}
	got := FormatStatusCompact(status)
	if got != "$2.50/$5.00" {
		t.Errorf("FormatStatusCompact() = %q, want $2.50/$5.00", got)
	}

	status.Budget = NewWithTokens(10000)
	got = FormatStatusCompact(status)
	if got != "5.0k/10.0k" {
		t.Errorf("FormatStatusCompact() = %q, want 5.0k/10.0k", got)
	}

	status.Budget = New()
	got = FormatStatusCompact(status)
	if got != "" {
		t.Errorf("FormatStatusCompact() = %q, want empty", got)
	}
}
