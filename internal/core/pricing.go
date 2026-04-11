// Package core provides pricing calculation, session analysis, and plan configuration.
package core

import "strings"

// modelPricing holds per-million-token costs for a model tier.
type modelPricing struct {
	// Input is the cost per million input tokens in USD.
	Input float64
	// Output is the cost per million output tokens in USD.
	Output float64
	// CacheCreation is the cost per million cache-write tokens in USD.
	CacheCreation float64
	// CacheRead is the cost per million cache-read tokens in USD.
	CacheRead float64
}

// openAIPricing maps normalized OpenAI model names to pricing tiers.
// Prices per million tokens in USD. Source: OpenAI pricing page / splitrail (2026).
// CacheCreation = 0 (OpenAI cache is read-only from Codex CLI perspective).
var openAIPricing = map[string]modelPricing{
	// codex-mini-latest / gpt-5.1-codex-mini
	"codex-mini": {Input: 1.50, Output: 6.00, CacheCreation: 0, CacheRead: 0.375},
	// codex-latest (full) / gpt-5-codex — legacy fallback
	"codex": {Input: 3.00, Output: 12.00, CacheCreation: 0, CacheRead: 0.750},
	// gpt-5.2 / gpt-5.2-codex
	"gpt-5.2": {Input: 1.75, Output: 14.00, CacheCreation: 0, CacheRead: 0.175},
	// gpt-5.3-codex and variants (gpt-5.3-codex-spark, gpt-5.3-codex-lightning, …)
	"gpt-5.3-codex": {Input: 1.75, Output: 14.00, CacheCreation: 0, CacheRead: 0.175},
	// gpt-5.4: tiered pricing; we use the standard tier ($2.50 input, $0.25 cache).
	// Large-context requests (>272k tokens) are billed at $5.00/$0.50 — use gpt-5.4-high
	// for a conservative upper-bound estimate.
	"gpt-5.4": {Input: 2.50, Output: 15.00, CacheCreation: 0, CacheRead: 0.25},
	// gpt-5.4-mini
	"gpt-5.4-mini": {Input: 1.50, Output: 6.00, CacheCreation: 0, CacheRead: 0.375},
}

// knownPricing maps normalised model names to their pricing tier.
// Prices are per million tokens in USD.
// Source: Anthropic API pricing (2026) — claude-opus-4.6, claude-sonnet-4.6, claude-haiku-4.5.
// Prompt cache: write = 1.25× input price (5-min ephemeral); read = 0.10× input price.
var knownPricing = map[string]modelPricing{
	// claude-opus-4.5, claude-opus-4.6
	"opus": {
		Input:         5.0,
		Output:        25.0,
		CacheCreation: 6.25, // 1.25 × $5
		CacheRead:     0.50, // 0.10 × $5
	},
	// claude-sonnet-4.5, claude-sonnet-4.6
	"sonnet": {
		Input:         3.0,
		Output:        15.0,
		CacheCreation: 3.75, // 1.25 × $3
		CacheRead:     0.30, // 0.10 × $3
	},
	// claude-haiku-4.5
	"haiku": {
		Input:         1.0,
		Output:        5.0,
		CacheCreation: 1.25, // 1.25 × $1
		CacheRead:     0.10, // 0.10 × $1
	},
}

// CalculateCost returns the estimated USD cost for the given token counts and model name.
// The cost is computed from token counts regardless of any cached costUSD field.
func CalculateCost(model string, inputTokens, outputTokens, cacheCreate, cacheRead int) float64 {
	p := pricingForModel(model)
	cost := (float64(inputTokens)/1_000_000)*p.Input +
		(float64(outputTokens)/1_000_000)*p.Output +
		(float64(cacheCreate)/1_000_000)*p.CacheCreation +
		(float64(cacheRead)/1_000_000)*p.CacheRead
	return cost
}

// pricingForModel returns the pricing tier for a given model name.
// For OpenAI models (contains "gpt" or "codex") we first try specific versioned entries
// to get accurate pricing, then fall back to generic tiers.
// Falls back to sonnet pricing for completely unknown models.
func pricingForModel(model string) modelPricing {
	lower := strings.ToLower(model)

	// OpenAI / Codex models — check specific versions before generic fallbacks.
	if strings.Contains(lower, "gpt") || strings.Contains(lower, "codex") {
		// gpt-5.4-mini
		if strings.Contains(lower, "5.4") && strings.Contains(lower, "mini") {
			return openAIPricing["gpt-5.4-mini"]
		}
		// gpt-5.4 (any variant)
		if strings.Contains(lower, "5.4") {
			return openAIPricing["gpt-5.4"]
		}
		// gpt-5.3-codex and variants (gpt-5.3-codex-spark, gpt-5.3-codex-lightning, …)
		if strings.Contains(lower, "5.3") && strings.Contains(lower, "codex") {
			return openAIPricing["gpt-5.3-codex"]
		}
		// gpt-5.2 and gpt-5.2-codex
		if strings.Contains(lower, "5.2") {
			return openAIPricing["gpt-5.2"]
		}
		// codex-mini-latest / gpt-5.1-codex-mini / any remaining mini model
		if strings.Contains(lower, "mini") {
			return openAIPricing["codex-mini"]
		}
		// codex-latest / gpt-5-codex / legacy fallback
		return openAIPricing["codex"]
	}

	if strings.Contains(lower, "opus") {
		return knownPricing["opus"]
	}
	if strings.Contains(lower, "haiku") {
		return knownPricing["haiku"]
	}
	// Default to sonnet for unknown or sonnet models.
	return knownPricing["sonnet"]
}
