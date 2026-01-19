package budget

import (
	"sync"
	"testing"

	"github.com/vx/ralph-go/internal/usage"
)

// Integration tests for budget enforcement

// Test 1: Budget parsing from various PRD formats
func TestBudgetParsingFromPRDFormats(t *testing.T) {
	tests := []struct {
		name       string
		lines      []string
		wantTokens int64
		wantUSD    float64
	}{
		{
			name:    "dollar format with sign",
			lines:   []string{"# Project", "Budget: $5.00", "## Feature"},
			wantUSD: 5.00,
		},
		{
			name:    "dollar format no cents",
			lines:   []string{"Budget: $10"},
			wantUSD: 10.00,
		},
		{
			name:       "token format simple",
			lines:      []string{"Budget: 10000"},
			wantTokens: 10000,
		},
		{
			name:       "token format with k suffix",
			lines:      []string{"Budget: 50k"},
			wantTokens: 50000,
		},
		{
			name:       "token format with M suffix",
			lines:      []string{"Budget: 1.5M"},
			wantTokens: 1500000,
		},
		{
			name:       "tokens keyword format",
			lines:      []string{"Tokens: 100k"},
			wantTokens: 100000,
		},
		{
			name:       "budget with tokens suffix",
			lines:      []string{"Budget: 50000 tokens"},
			wantTokens: 50000,
		},
		{
			name:    "decimal treated as USD",
			lines:   []string{"Budget: 5.00"},
			wantUSD: 5.00,
		},
		{
			name:       "multi-line PRD - picks first budget",
			lines:      []string{"# Title", "", "Budget: $10", "", "Budget: $5"},
			wantUSD:    10.00,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			content := ""
			for _, line := range tt.lines {
				content += line + "\n"
			}

			budget := ParseBudget(content)

			if tt.wantTokens > 0 && budget.Tokens != tt.wantTokens {
				t.Errorf("Tokens = %d, want %d", budget.Tokens, tt.wantTokens)
			}
			if tt.wantUSD > 0 && budget.USD != tt.wantUSD {
				t.Errorf("USD = %f, want %f", budget.USD, tt.wantUSD)
			}
			if tt.wantTokens > 0 && !budget.HasTokenLimit() {
				t.Error("expected HasTokenLimit to be true")
			}
			if tt.wantUSD > 0 && !budget.HasUSDLimit() {
				t.Error("expected HasUSDLimit to be true")
			}
		})
	}
}

// Test 2: Budget threshold detection at 90%
func TestBudgetThresholdDetection(t *testing.T) {
	tests := []struct {
		name         string
		budget       *Budget
		usageTokens  int64
		usageUSD     float64
		wantPercent  float64
		wantThreshold bool
		wantOver     bool
	}{
		{
			name:         "token budget at 50%",
			budget:       NewWithTokens(10000),
			usageTokens:  5000,
			wantPercent:  50.0,
			wantThreshold: false,
			wantOver:     false,
		},
		{
			name:         "token budget at 89%",
			budget:       NewWithTokens(10000),
			usageTokens:  8900,
			wantPercent:  89.0,
			wantThreshold: false,
			wantOver:     false,
		},
		{
			name:         "token budget at 90% - threshold",
			budget:       NewWithTokens(10000),
			usageTokens:  9000,
			wantPercent:  90.0,
			wantThreshold: true,
			wantOver:     false,
		},
		{
			name:         "token budget at 95%",
			budget:       NewWithTokens(10000),
			usageTokens:  9500,
			wantPercent:  95.0,
			wantThreshold: true,
			wantOver:     false,
		},
		{
			name:         "token budget at 100% - over",
			budget:       NewWithTokens(10000),
			usageTokens:  10000,
			wantPercent:  100.0,
			wantThreshold: true,
			wantOver:     true,
		},
		{
			name:         "token budget at 110% - over",
			budget:       NewWithTokens(10000),
			usageTokens:  11000,
			wantPercent:  110.0,
			wantThreshold: true,
			wantOver:     true,
		},
		{
			name:         "USD budget at 50%",
			budget:       NewWithUSD(10.0),
			usageUSD:     5.0,
			wantPercent:  50.0,
			wantThreshold: false,
			wantOver:     false,
		},
		{
			name:         "USD budget at 90% - threshold",
			budget:       NewWithUSD(10.0),
			usageUSD:     9.0,
			wantPercent:  90.0,
			wantThreshold: true,
			wantOver:     false,
		},
		{
			name:         "USD budget at 100% - over",
			budget:       NewWithUSD(5.0),
			usageUSD:     5.0,
			wantPercent:  100.0,
			wantThreshold: true,
			wantOver:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := usage.New()
			if tt.usageTokens > 0 {
				u.InputTokens = tt.usageTokens / 2
				u.OutputTokens = tt.usageTokens / 2
				u.TotalTokens = tt.usageTokens
			}
			if tt.usageUSD > 0 {
				u.CostUSD = tt.usageUSD
			}

			status := tt.budget.CheckUsage(u)

			if int(status.Percent) != int(tt.wantPercent) {
				t.Errorf("Percent = %.1f, want %.1f", status.Percent, tt.wantPercent)
			}
			if status.AtThreshold != tt.wantThreshold {
				t.Errorf("AtThreshold = %v, want %v", status.AtThreshold, tt.wantThreshold)
			}
			if status.OverBudget != tt.wantOver {
				t.Errorf("OverBudget = %v, want %v", status.OverBudget, tt.wantOver)
			}
		})
	}
}

// Test 3: Budget status formatting
func TestBudgetStatusFormatting(t *testing.T) {
	tests := []struct {
		name       string
		budget     *Budget
		usageTokens int64
		usageUSD   float64
		wantFormat string
	}{
		{
			name:       "token budget status",
			budget:     NewWithTokens(100000),
			usageTokens: 50000,
			wantFormat: "50.0k / 100.0k (50%)",
		},
		{
			name:       "USD budget status",
			budget:     NewWithUSD(10.0),
			usageUSD:   5.0,
			wantFormat: "$5.00 / $10.00 (50%)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.budget.HasTokenLimit() {
				result := FormatStatus(float64(tt.usageTokens), float64(tt.budget.Tokens), false)
				if result != tt.wantFormat {
					t.Errorf("got %q, want %q", result, tt.wantFormat)
				}
			}
			if tt.budget.HasUSDLimit() {
				result := FormatStatus(tt.usageUSD, tt.budget.USD, true)
				if result != tt.wantFormat {
					t.Errorf("got %q, want %q", result, tt.wantFormat)
				}
			}
		})
	}
}

// Test 4: Budget compact status
func TestBudgetCompactStatusFormatting(t *testing.T) {
	tests := []struct {
		name       string
		budget     *Budget
		usageTokens int64
		usageUSD   float64
		wantFormat string
	}{
		{
			name:       "token budget compact",
			budget:     NewWithTokens(100000),
			usageTokens: 50000,
			wantFormat: "50.0k/100.0k",
		},
		{
			name:       "USD budget compact",
			budget:     NewWithUSD(10.0),
			usageUSD:   5.0,
			wantFormat: "$5.00/$10.00",
		},
		{
			name:       "no budget set",
			budget:     New(),
			wantFormat: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := usage.New()
			u.TotalTokens = tt.usageTokens
			u.CostUSD = tt.usageUSD

			status := tt.budget.CheckUsage(u)
			result := FormatStatusCompact(status)

			if result != tt.wantFormat {
				t.Errorf("got %q, want %q", result, tt.wantFormat)
			}
		})
	}
}

// Test 5: Budget thread safety
func TestBudgetThreadSafety(t *testing.T) {
	b := New()

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			line := "Budget: $5.00"
			if i%2 == 0 {
				line = "Budget: 10k"
			}
			b.ParseLine(line)
		}(i)
	}

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = b.HasTokenLimit()
			_ = b.HasUSDLimit()
			_ = b.String()
			_ = b.Copy()
		}()
	}

	wg.Wait()

	// Budget should be set to one of the values
	if !b.IsSet {
		t.Error("budget should be set after parsing")
	}
}

// Test 6: Budget with nil usage
func TestBudgetCheckUsageWithNilUsage(t *testing.T) {
	b := NewWithTokens(10000)
	status := b.CheckUsage(nil)

	if status.Percent != 0 {
		t.Errorf("Percent with nil usage = %f, want 0", status.Percent)
	}
	if status.AtThreshold {
		t.Error("AtThreshold should be false with nil usage")
	}
	if status.OverBudget {
		t.Error("OverBudget should be false with nil usage")
	}
}

// Test 7: Budget with unset budget
func TestBudgetCheckUsageWithUnsetBudget(t *testing.T) {
	b := New() // Not set
	u := usage.New()
	u.TotalTokens = 10000

	status := b.CheckUsage(u)

	if status.Percent != 0 {
		t.Errorf("Percent with unset budget = %f, want 0", status.Percent)
	}
	if status.AtThreshold {
		t.Error("AtThreshold should be false with unset budget")
	}
}

// Test 8: Budget copy independence
func TestBudgetCopyIndependence(t *testing.T) {
	b := NewWithUSD(10.0)
	copy := b.Copy()

	// Modify original
	b.USD = 20.0

	if copy.USD != 10.0 {
		t.Errorf("copy.USD = %f, want 10.0 (should be independent)", copy.USD)
	}
}

// Test 9: Integration with stream usage tracking
func TestBudgetWithStreamUsageTracking(t *testing.T) {
	b := NewWithTokens(10000)
	u := usage.New()

	// Simulate stream-json usage accumulation
	lines := []string{
		`{"type":"assistant","usage":{"input_tokens":1000,"output_tokens":500}}`,
		`{"type":"assistant","usage":{"input_tokens":2000,"output_tokens":1000}}`,
		`{"type":"assistant","usage":{"input_tokens":3000,"output_tokens":1500}}`,
	}

	for _, line := range lines {
		u.ParseLine(line)
	}

	// Check budget status
	status := b.CheckUsage(u)

	// Total = 6000 + 3000 = 9000 tokens
	if u.TotalTokens != 9000 {
		t.Errorf("TotalTokens = %d, want 9000", u.TotalTokens)
	}

	// 9000/10000 = 90%
	if status.Percent != 90.0 {
		t.Errorf("Percent = %f, want 90.0", status.Percent)
	}

	if !status.AtThreshold {
		t.Error("should be at threshold at 90%")
	}
}

// Test 10: USD budget with real cost tracking
func TestUSDbudgetWithRealCostTracking(t *testing.T) {
	b := NewWithUSD(1.0) // $1.00 budget
	u := usage.New()

	// Simulate costs accumulating
	lines := []string{
		`{"type":"assistant","usage":{"input_tokens":1000,"output_tokens":500},"cost_usd":0.25}`,
		`{"type":"assistant","usage":{"input_tokens":1000,"output_tokens":500},"cost_usd":0.25}`,
		`{"type":"assistant","usage":{"input_tokens":1000,"output_tokens":500},"cost_usd":0.25}`,
	}

	for _, line := range lines {
		u.ParseLine(line)
	}

	status := b.CheckUsage(u)

	// $0.75 / $1.00 = 75%
	if status.Percent != 75.0 {
		t.Errorf("Percent = %f, want 75.0", status.Percent)
	}

	// Add one more message to hit threshold
	u.ParseLine(`{"type":"assistant","usage":{"input_tokens":1000,"output_tokens":500},"cost_usd":0.15}`)

	status = b.CheckUsage(u)
	// $0.90 / $1.00 = 90%
	if !status.AtThreshold {
		t.Errorf("should be at threshold, percent = %f", status.Percent)
	}
}

// Test 11: Edge case - exactly at budget
func TestBudgetExactlyAtLimit(t *testing.T) {
	b := NewWithTokens(10000)
	u := usage.New()
	u.TotalTokens = 10000

	status := b.CheckUsage(u)

	if status.Percent != 100.0 {
		t.Errorf("Percent = %f, want 100.0", status.Percent)
	}
	if !status.AtThreshold {
		t.Error("should be at threshold at 100%")
	}
	if !status.OverBudget {
		t.Error("should be over budget at exactly 100%")
	}
}

// Test 12: Budget string representation
func TestBudgetStringRepresentation(t *testing.T) {
	tests := []struct {
		name   string
		budget *Budget
		want   string
	}{
		{
			name:   "USD budget",
			budget: NewWithUSD(5.50),
			want:   "$5.50",
		},
		{
			name:   "token budget",
			budget: NewWithTokens(100000),
			want:   "100.0k",
		},
		{
			name:   "unset budget",
			budget: New(),
			want:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.budget.String()
			if result != tt.want {
				t.Errorf("String() = %q, want %q", result, tt.want)
			}
		})
	}
}

// Test 13: Budget with case-insensitive parsing
func TestBudgetCaseInsensitiveParsing(t *testing.T) {
	testCases := []string{
		"Budget: $5.00",
		"BUDGET: $5.00",
		"budget: $5.00",
		"Tokens: 10k",
		"TOKENS: 10K",
		"tokens: 10k",
	}

	for _, tc := range testCases {
		b := New()
		if !b.ParseLine(tc) {
			t.Errorf("failed to parse: %q", tc)
		}
		if !b.IsSet {
			t.Errorf("budget not set for: %q", tc)
		}
	}
}

// Test 14: Budget with whitespace variations
func TestBudgetWhitespaceVariations(t *testing.T) {
	testCases := []struct {
		line string
		want float64
	}{
		{"Budget: $5.00", 5.00},
		{"Budget:$5.00", 5.00},
		{"Budget:  $5.00", 5.00},
		{"  Budget: $5.00  ", 5.00},
	}

	for _, tc := range testCases {
		b := New()
		b.ParseLine(tc.line)
		if b.USD != tc.want {
			t.Errorf("ParseLine(%q): USD = %f, want %f", tc.line, b.USD, tc.want)
		}
	}
}

// Test 15: Threshold constant is correct
func TestThresholdConstant(t *testing.T) {
	if ThresholdPercent != 90.0 {
		t.Errorf("ThresholdPercent = %f, want 90.0", ThresholdPercent)
	}
}
