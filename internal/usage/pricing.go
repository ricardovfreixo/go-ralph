package usage

// Pricing constants for Claude models (per million tokens)
// Prices as of January 2025 from Anthropic's pricing page
const (
	// Claude 3.5 Sonnet (claude-3-5-sonnet-20241022)
	SonnetInputPrice       = 3.00   // $3.00 per million input tokens
	SonnetOutputPrice      = 15.00  // $15.00 per million output tokens
	SonnetCacheWritePrice  = 3.75   // $3.75 per million cache write tokens
	SonnetCacheReadPrice   = 0.30   // $0.30 per million cache read tokens

	// Claude 3.5 Haiku (claude-3-5-haiku-20241022)
	HaikuInputPrice        = 0.80   // $0.80 per million input tokens
	HaikuOutputPrice       = 4.00   // $4.00 per million output tokens
	HaikuCacheWritePrice   = 1.00   // $1.00 per million cache write tokens
	HaikuCacheReadPrice    = 0.08   // $0.08 per million cache read tokens

	// Claude 3 Opus (claude-3-opus-20240229)
	OpusInputPrice         = 15.00  // $15.00 per million input tokens
	OpusOutputPrice        = 75.00  // $75.00 per million output tokens
	OpusCacheWritePrice    = 18.75  // $18.75 per million cache write tokens
	OpusCacheReadPrice     = 1.50   // $1.50 per million cache read tokens
)

// ModelPricing holds pricing for a specific model
type ModelPricing struct {
	InputPrice      float64
	OutputPrice     float64
	CacheWritePrice float64
	CacheReadPrice  float64
}

// GetPricing returns pricing for a model name
func GetPricing(model string) ModelPricing {
	switch model {
	case "haiku", "claude-3-5-haiku-20241022", "claude-3-haiku":
		return ModelPricing{
			InputPrice:      HaikuInputPrice,
			OutputPrice:     HaikuOutputPrice,
			CacheWritePrice: HaikuCacheWritePrice,
			CacheReadPrice:  HaikuCacheReadPrice,
		}
	case "opus", "claude-3-opus-20240229", "claude-3-opus", "claude-opus-4-5-20251101":
		return ModelPricing{
			InputPrice:      OpusInputPrice,
			OutputPrice:     OpusOutputPrice,
			CacheWritePrice: OpusCacheWritePrice,
			CacheReadPrice:  OpusCacheReadPrice,
		}
	default: // sonnet is default
		return ModelPricing{
			InputPrice:      SonnetInputPrice,
			OutputPrice:     SonnetOutputPrice,
			CacheWritePrice: SonnetCacheWritePrice,
			CacheReadPrice:  SonnetCacheReadPrice,
		}
	}
}

// CalculateCost calculates the cost based on token usage and model
func CalculateCost(u *TokenUsage, model string) float64 {
	if u == nil {
		return 0
	}

	pricing := GetPricing(model)

	u.mu.RLock()
	defer u.mu.RUnlock()

	inputCost := float64(u.InputTokens) * pricing.InputPrice / 1_000_000
	outputCost := float64(u.OutputTokens) * pricing.OutputPrice / 1_000_000
	cacheWriteCost := float64(u.CacheWriteTokens) * pricing.CacheWritePrice / 1_000_000
	cacheReadCost := float64(u.CacheReadTokens) * pricing.CacheReadPrice / 1_000_000

	return inputCost + outputCost + cacheWriteCost + cacheReadCost
}

// EstimateCost calculates cost from token counts and model (without needing TokenUsage struct)
func EstimateCost(input, output, cacheWrite, cacheRead int64, model string) float64 {
	pricing := GetPricing(model)

	inputCost := float64(input) * pricing.InputPrice / 1_000_000
	outputCost := float64(output) * pricing.OutputPrice / 1_000_000
	cacheWriteCost := float64(cacheWrite) * pricing.CacheWritePrice / 1_000_000
	cacheReadCost := float64(cacheRead) * pricing.CacheReadPrice / 1_000_000

	return inputCost + outputCost + cacheWriteCost + cacheReadCost
}
