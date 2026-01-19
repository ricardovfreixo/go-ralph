package context

import (
	"strings"
	"testing"
)

func TestNew(t *testing.T) {
	b := New()
	if b.BaseBudget != DefaultBaseBudget {
		t.Errorf("expected base %d, got %d", DefaultBaseBudget, b.BaseBudget)
	}
	if b.CurrentBudget != DefaultBaseBudget {
		t.Errorf("expected current %d, got %d", DefaultBaseBudget, b.CurrentBudget)
	}
	if b.UsedTokens != 0 {
		t.Errorf("expected used 0, got %d", b.UsedTokens)
	}
	if b.Depth != 0 {
		t.Errorf("expected depth 0, got %d", b.Depth)
	}
}

func TestNewWithBase(t *testing.T) {
	tests := []struct {
		name     string
		base     int64
		expected int64
	}{
		{"custom base", 50000, 50000},
		{"zero base uses default", 0, DefaultBaseBudget},
		{"negative base uses default", -100, DefaultBaseBudget},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := NewWithBase(tt.base)
			if b.BaseBudget != tt.expected {
				t.Errorf("expected %d, got %d", tt.expected, b.BaseBudget)
			}
		})
	}
}

func TestNewForDepth(t *testing.T) {
	tests := []struct {
		name     string
		base     int64
		depth    int
		expected int64
	}{
		{"depth 0", 100000, 0, 100000},
		{"depth 1", 100000, 1, 50000},
		{"depth 2", 100000, 2, 33333},
		{"depth 3", 100000, 3, 25000},
		{"depth 4", 100000, 4, 20000},
		{"negative depth", 100000, -1, 100000},
		{"minimum floor", 10000, 5, DefaultMinBudget},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := NewForDepth(tt.base, tt.depth)
			if b.CurrentBudget != tt.expected {
				t.Errorf("expected %d, got %d", tt.expected, b.CurrentBudget)
			}
			if tt.depth >= 0 && b.Depth != tt.depth {
				t.Errorf("expected depth %d, got %d", tt.depth, b.Depth)
			}
		})
	}
}

func TestCalculateBudgetForDepth(t *testing.T) {
	tests := []struct {
		name     string
		base     int64
		depth    int
		expected int64
	}{
		{"depth 0", 100000, 0, 100000},
		{"depth 1", 100000, 1, 50000},
		{"depth 2", 100000, 2, 33333},
		{"depth 3", 100000, 3, 25000},
		{"depth 9", 100000, 9, 10000},            // hits minimum
		{"very deep", 100000, 100, DefaultMinBudget}, // way past minimum
		{"small base minimum", 1000, 0, DefaultMinBudget},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CalculateBudgetForDepth(tt.base, tt.depth)
			if result != tt.expected {
				t.Errorf("expected %d, got %d", tt.expected, result)
			}
		})
	}
}

func TestBudgetCalculateChildBudget(t *testing.T) {
	b := NewWithBase(100000)
	b.Depth = 0

	childBudget := b.CalculateChildBudget()
	expected := CalculateBudgetForDepth(100000, 1)
	if childBudget != expected {
		t.Errorf("expected %d, got %d", expected, childBudget)
	}
}

func TestBudgetUsageTracking(t *testing.T) {
	b := NewWithBase(100000)

	b.AddUsage(25000)
	if b.GetUsedTokens() != 25000 {
		t.Errorf("expected used 25000, got %d", b.GetUsedTokens())
	}

	b.AddUsage(25000)
	if b.GetUsedTokens() != 50000 {
		t.Errorf("expected used 50000, got %d", b.GetUsedTokens())
	}

	b.SetUsage(75000)
	if b.GetUsedTokens() != 75000 {
		t.Errorf("expected used 75000, got %d", b.GetUsedTokens())
	}
}

func TestBudgetGetRemaining(t *testing.T) {
	b := NewWithBase(100000)

	if b.GetRemaining() != 100000 {
		t.Errorf("expected remaining 100000, got %d", b.GetRemaining())
	}

	b.SetUsage(40000)
	if b.GetRemaining() != 60000 {
		t.Errorf("expected remaining 60000, got %d", b.GetRemaining())
	}

	b.SetUsage(120000) // over budget
	if b.GetRemaining() != 0 {
		t.Errorf("expected remaining 0 when over budget, got %d", b.GetRemaining())
	}
}

func TestBudgetGetUsagePercent(t *testing.T) {
	b := NewWithBase(100000)

	if b.GetUsagePercent() != 0 {
		t.Errorf("expected 0%%, got %f", b.GetUsagePercent())
	}

	b.SetUsage(50000)
	if b.GetUsagePercent() != 0.5 {
		t.Errorf("expected 50%%, got %f", b.GetUsagePercent())
	}

	b.SetUsage(80000)
	if b.GetUsagePercent() != 0.8 {
		t.Errorf("expected 80%%, got %f", b.GetUsagePercent())
	}
}

func TestBudgetNeedsSummarization(t *testing.T) {
	b := NewWithBase(100000)

	if b.NeedsSummarization() {
		t.Error("should not need summarization at 0%")
	}

	b.SetUsage(79000)
	if b.NeedsSummarization() {
		t.Error("should not need summarization at 79%")
	}

	b.SetUsage(80000)
	if !b.NeedsSummarization() {
		t.Error("should need summarization at 80%")
	}

	b.SetUsage(95000)
	if !b.NeedsSummarization() {
		t.Error("should need summarization at 95%")
	}
}

func TestBudgetIsOverBudget(t *testing.T) {
	b := NewWithBase(100000)

	if b.IsOverBudget() {
		t.Error("should not be over budget at 0%")
	}

	b.SetUsage(100000)
	if b.IsOverBudget() {
		t.Error("should not be over budget at exactly 100%")
	}

	b.SetUsage(100001)
	if !b.IsOverBudget() {
		t.Error("should be over budget at 100001")
	}
}

func TestBudgetNeedsTruncation(t *testing.T) {
	b := NewWithBase(100000)

	b.SetUsage(94000)
	if b.NeedsTruncation() {
		t.Error("should not need truncation at 94%")
	}

	b.SetUsage(95000)
	if !b.NeedsTruncation() {
		t.Error("should need truncation at 95%")
	}
}

func TestBudgetSnapshot(t *testing.T) {
	b := NewWithBase(100000)
	b.SetUsage(30000)

	snap := b.Snapshot()
	if snap.BaseBudget != 100000 {
		t.Errorf("snapshot base mismatch")
	}
	if snap.UsedTokens != 30000 {
		t.Errorf("snapshot used mismatch")
	}

	// Original should not be affected by modifications
	b.SetUsage(50000)
	if snap.UsedTokens != 30000 {
		t.Errorf("snapshot should be independent")
	}
}

func TestBudgetReset(t *testing.T) {
	b := NewWithBase(100000)
	b.SetUsage(50000)
	b.Reset()

	if b.GetUsedTokens() != 0 {
		t.Errorf("expected used 0 after reset, got %d", b.GetUsedTokens())
	}
}

func TestFormatBudget(t *testing.T) {
	tests := []struct {
		tokens   int64
		expected string
	}{
		{0, "0"},
		{500, "500"},
		{1000, "1k"},
		{1500, "2k"},
		{10000, "10k"},
		{100000, "100k"},
		{1000000, "1.0M"},
		{1500000, "1.5M"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := FormatBudget(tt.tokens)
			if result != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestBudgetStatus(t *testing.T) {
	b := NewWithBase(100000)
	b.SetUsage(50000)

	status := b.Status()
	if !strings.Contains(status, "50k") {
		t.Errorf("expected status to contain '50k', got %s", status)
	}
	if !strings.Contains(status, "100k") {
		t.Errorf("expected status to contain '100k', got %s", status)
	}
	if !strings.Contains(status, "50%") {
		t.Errorf("expected status to contain '50%%', got %s", status)
	}
}

func TestBudgetCompactStatus(t *testing.T) {
	b := NewWithBase(100000)
	b.SetUsage(50000)

	status := b.CompactStatus()
	if !strings.HasPrefix(status, "ctx:") {
		t.Errorf("expected compact status to start with 'ctx:', got %s", status)
	}
}

func TestParseContextLine(t *testing.T) {
	tests := []struct {
		line     string
		expected int64
		ok       bool
	}{
		{"Context: 50000", 50000, true},
		{"context: 50k", 50000, true},
		{"CONTEXT: 1M", 1000000, true},
		{"Context: 1.5M", 1500000, true},
		{"Context: 50k tokens", 50000, true},
		{"Budget: 50000", 0, false},
		{"not a context line", 0, false},
		{"Context:", 0, false},
		{"Context: -100", 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.line, func(t *testing.T) {
			result, ok := ParseContextLine(tt.line)
			if ok != tt.ok {
				t.Errorf("expected ok=%v, got ok=%v", tt.ok, ok)
			}
			if result != tt.expected {
				t.Errorf("expected %d, got %d", tt.expected, result)
			}
		})
	}
}

func TestParseContextValue(t *testing.T) {
	tests := []struct {
		value    string
		expected int64
		ok       bool
	}{
		{"50000", 50000, true},
		{"50k", 50000, true},
		{"1M", 1000000, true},
		{"1.5M", 1500000, true},
		{"100k tokens", 100000, true},
		{"50K", 50000, true},    // uppercase
		{" 50k ", 50000, true},  // whitespace
		{"", 0, false},
		{"-100", 0, false},
		{"abc", 0, false},
		{"0", 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			result, ok := ParseContextValue(tt.value)
			if ok != tt.ok {
				t.Errorf("expected ok=%v, got ok=%v", tt.ok, ok)
			}
			if result != tt.expected {
				t.Errorf("expected %d, got %d", tt.expected, result)
			}
		})
	}
}

func TestBudgetSetCurrentBudget(t *testing.T) {
	b := NewWithBase(100000)

	b.SetCurrentBudget(50000)
	if b.GetCurrentBudget() != 50000 {
		t.Errorf("expected current 50000, got %d", b.GetCurrentBudget())
	}

	// Zero should not change it
	b.SetCurrentBudget(0)
	if b.GetCurrentBudget() != 50000 {
		t.Errorf("expected current 50000 unchanged, got %d", b.GetCurrentBudget())
	}

	// Negative should not change it
	b.SetCurrentBudget(-100)
	if b.GetCurrentBudget() != 50000 {
		t.Errorf("expected current 50000 unchanged, got %d", b.GetCurrentBudget())
	}
}

func TestConcurrentAccess(t *testing.T) {
	b := NewWithBase(100000)

	done := make(chan bool, 10)

	// Start concurrent readers and writers
	for i := 0; i < 5; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				b.AddUsage(10)
				_ = b.GetUsedTokens()
				_ = b.GetRemaining()
				_ = b.GetUsagePercent()
				_ = b.NeedsSummarization()
			}
			done <- true
		}()
	}

	for i := 0; i < 5; i++ {
		<-done
	}
}
