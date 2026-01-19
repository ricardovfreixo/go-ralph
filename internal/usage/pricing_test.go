package usage

import (
	"math"
	"testing"
)

func TestGetPricingSonnet(t *testing.T) {
	tests := []string{"sonnet", "claude-3-5-sonnet-20241022", "unknown", ""}
	for _, model := range tests {
		pricing := GetPricing(model)
		if pricing.InputPrice != SonnetInputPrice {
			t.Errorf("GetPricing(%q): expected InputPrice %f, got %f", model, SonnetInputPrice, pricing.InputPrice)
		}
		if pricing.OutputPrice != SonnetOutputPrice {
			t.Errorf("GetPricing(%q): expected OutputPrice %f, got %f", model, SonnetOutputPrice, pricing.OutputPrice)
		}
	}
}

func TestGetPricingHaiku(t *testing.T) {
	tests := []string{"haiku", "claude-3-5-haiku-20241022", "claude-3-haiku"}
	for _, model := range tests {
		pricing := GetPricing(model)
		if pricing.InputPrice != HaikuInputPrice {
			t.Errorf("GetPricing(%q): expected InputPrice %f, got %f", model, HaikuInputPrice, pricing.InputPrice)
		}
		if pricing.OutputPrice != HaikuOutputPrice {
			t.Errorf("GetPricing(%q): expected OutputPrice %f, got %f", model, HaikuOutputPrice, pricing.OutputPrice)
		}
	}
}

func TestGetPricingOpus(t *testing.T) {
	tests := []string{"opus", "claude-3-opus-20240229", "claude-3-opus", "claude-opus-4-5-20251101"}
	for _, model := range tests {
		pricing := GetPricing(model)
		if pricing.InputPrice != OpusInputPrice {
			t.Errorf("GetPricing(%q): expected InputPrice %f, got %f", model, OpusInputPrice, pricing.InputPrice)
		}
		if pricing.OutputPrice != OpusOutputPrice {
			t.Errorf("GetPricing(%q): expected OutputPrice %f, got %f", model, OpusOutputPrice, pricing.OutputPrice)
		}
	}
}

func TestCalculateCostSonnet(t *testing.T) {
	u := New()
	u.InputTokens = 1000000
	u.OutputTokens = 1000000

	cost := CalculateCost(u, "sonnet")
	expected := SonnetInputPrice + SonnetOutputPrice // $3 + $15 = $18
	if math.Abs(cost-expected) > 0.001 {
		t.Errorf("expected cost %f, got %f", expected, cost)
	}
}

func TestCalculateCostHaiku(t *testing.T) {
	u := New()
	u.InputTokens = 1000000
	u.OutputTokens = 1000000

	cost := CalculateCost(u, "haiku")
	expected := HaikuInputPrice + HaikuOutputPrice // $0.80 + $4 = $4.80
	if math.Abs(cost-expected) > 0.001 {
		t.Errorf("expected cost %f, got %f", expected, cost)
	}
}

func TestCalculateCostOpus(t *testing.T) {
	u := New()
	u.InputTokens = 1000000
	u.OutputTokens = 1000000

	cost := CalculateCost(u, "opus")
	expected := OpusInputPrice + OpusOutputPrice // $15 + $75 = $90
	if math.Abs(cost-expected) > 0.001 {
		t.Errorf("expected cost %f, got %f", expected, cost)
	}
}

func TestCalculateCostWithCache(t *testing.T) {
	u := New()
	u.InputTokens = 1000000
	u.OutputTokens = 1000000
	u.CacheWriteTokens = 500000
	u.CacheReadTokens = 500000

	cost := CalculateCost(u, "sonnet")
	expected := SonnetInputPrice + SonnetOutputPrice +
		(SonnetCacheWritePrice / 2) + (SonnetCacheReadPrice / 2)
	// $3 + $15 + $1.875 + $0.15 = $20.025
	if math.Abs(cost-expected) > 0.001 {
		t.Errorf("expected cost %f, got %f", expected, cost)
	}
}

func TestCalculateCostNil(t *testing.T) {
	cost := CalculateCost(nil, "sonnet")
	if cost != 0 {
		t.Errorf("expected 0 for nil usage, got %f", cost)
	}
}

func TestEstimateCost(t *testing.T) {
	tests := []struct {
		name         string
		input        int64
		output       int64
		cacheWrite   int64
		cacheRead    int64
		model        string
		expectedCost float64
	}{
		{
			name:         "sonnet 1M tokens each",
			input:        1000000,
			output:       1000000,
			model:        "sonnet",
			expectedCost: SonnetInputPrice + SonnetOutputPrice,
		},
		{
			name:         "haiku 1M tokens each",
			input:        1000000,
			output:       1000000,
			model:        "haiku",
			expectedCost: HaikuInputPrice + HaikuOutputPrice,
		},
		{
			name:         "opus 1M tokens each",
			input:        1000000,
			output:       1000000,
			model:        "opus",
			expectedCost: OpusInputPrice + OpusOutputPrice,
		},
		{
			name:         "zero tokens",
			input:        0,
			output:       0,
			model:        "sonnet",
			expectedCost: 0,
		},
		{
			name:         "small usage sonnet",
			input:        10000, // 10k tokens
			output:       5000,  // 5k tokens
			model:        "sonnet",
			expectedCost: (10000.0/1000000)*SonnetInputPrice + (5000.0/1000000)*SonnetOutputPrice,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cost := EstimateCost(tc.input, tc.output, tc.cacheWrite, tc.cacheRead, tc.model)
			if math.Abs(cost-tc.expectedCost) > 0.0001 {
				t.Errorf("expected %f, got %f", tc.expectedCost, cost)
			}
		})
	}
}

func TestTokenUsageGetEstimatedCost(t *testing.T) {
	u := New()
	u.InputTokens = 100000
	u.OutputTokens = 50000

	cost := u.GetEstimatedCost("sonnet")
	expected := (100000.0/1000000)*SonnetInputPrice + (50000.0/1000000)*SonnetOutputPrice
	if math.Abs(cost-expected) > 0.0001 {
		t.Errorf("expected %f, got %f", expected, cost)
	}
}

func TestTokenUsageGetEstimatedCostWithStreamCost(t *testing.T) {
	u := New()
	u.InputTokens = 100000
	u.OutputTokens = 50000
	u.CostUSD = 0.50

	cost := u.GetEstimatedCost("sonnet")
	if cost != 0.50 {
		t.Errorf("expected stream cost 0.50, got %f", cost)
	}
}

func TestTokenUsageCompactWithCost(t *testing.T) {
	u := New()
	u.InputTokens = 1500
	u.OutputTokens = 800
	u.TotalTokens = 2300

	compact := u.CompactWithCost("sonnet")
	if compact == "" {
		t.Error("CompactWithCost should not return empty for non-zero usage")
	}
	// Should include cost since tokens > 0
	if compact == "1.5k↓ 800↑" {
		t.Error("CompactWithCost should include cost")
	}
}

func TestTokenUsageCompactWithCostEmpty(t *testing.T) {
	u := New()
	compact := u.CompactWithCost("sonnet")
	if compact != "" {
		t.Errorf("expected empty string for zero usage, got %q", compact)
	}
}

func TestPricingConstants(t *testing.T) {
	if SonnetInputPrice <= 0 {
		t.Error("SonnetInputPrice should be positive")
	}
	if SonnetOutputPrice <= 0 {
		t.Error("SonnetOutputPrice should be positive")
	}
	if HaikuInputPrice <= 0 {
		t.Error("HaikuInputPrice should be positive")
	}
	if HaikuOutputPrice <= 0 {
		t.Error("HaikuOutputPrice should be positive")
	}
	if OpusInputPrice <= 0 {
		t.Error("OpusInputPrice should be positive")
	}
	if OpusOutputPrice <= 0 {
		t.Error("OpusOutputPrice should be positive")
	}

	// Verify price ordering: haiku < sonnet < opus
	if HaikuInputPrice >= SonnetInputPrice {
		t.Error("Haiku should be cheaper than Sonnet")
	}
	if SonnetInputPrice >= OpusInputPrice {
		t.Error("Sonnet should be cheaper than Opus")
	}
	if HaikuOutputPrice >= SonnetOutputPrice {
		t.Error("Haiku output should be cheaper than Sonnet")
	}
	if SonnetOutputPrice >= OpusOutputPrice {
		t.Error("Sonnet output should be cheaper than Opus")
	}
}
