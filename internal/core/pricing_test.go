package core

import (
	"testing"
)

// TestCalculateCost_OpenAIModels verifies that Codex CLI model names return
// non-zero costs using the correct OpenAI per-million-token pricing rates.
func TestCalculateCost_OpenAIModels(t *testing.T) {
	cases := []struct {
		name       string
		model      string
		input      int
		output     int
		cacheRead  int
		wantNonZero bool
		// expectedApprox is the expected cost in USD for the given token counts.
		// Using 1M tokens each for easy calculation.
		expectedInput  float64
		expectedOutput float64
	}{
		{
			name:        "codex-mini-latest input pricing",
			model:       "codex-mini-latest",
			input:       1_000_000,
			output:      0,
			cacheRead:   0,
			wantNonZero: true,
			// codex-mini: $1.50 per million input tokens
			expectedInput: 1.50,
		},
		{
			name:        "codex-mini-latest output pricing",
			model:       "codex-mini-latest",
			input:       0,
			output:      1_000_000,
			cacheRead:   0,
			wantNonZero: true,
			// codex-mini: $6.00 per million output tokens
			expectedOutput: 6.00,
		},
		{
			name:        "codex-latest input pricing",
			model:       "codex-latest",
			input:       1_000_000,
			output:      0,
			cacheRead:   0,
			wantNonZero: true,
			// codex (full): $3.00 per million input tokens
			expectedInput: 3.00,
		},
		{
			name:        "codex-latest output pricing",
			model:       "codex-latest",
			input:       0,
			output:      1_000_000,
			cacheRead:   0,
			wantNonZero: true,
			// codex (full): $12.00 per million output tokens
			expectedOutput: 12.00,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cost := CalculateCost(tc.model, tc.input, tc.output, 0, tc.cacheRead)
			if tc.wantNonZero && cost == 0 {
				t.Errorf("CalculateCost(%q, %d, %d, 0, %d) = 0, want non-zero",
					tc.model, tc.input, tc.output, tc.cacheRead)
			}
			// Verify expected cost when input or output tokens are 1M.
			if tc.input == 1_000_000 && tc.expectedInput > 0 {
				if cost != tc.expectedInput {
					t.Errorf("CalculateCost input cost: got %v, want %v", cost, tc.expectedInput)
				}
			}
			if tc.output == 1_000_000 && tc.expectedOutput > 0 {
				if cost != tc.expectedOutput {
					t.Errorf("CalculateCost output cost: got %v, want %v", cost, tc.expectedOutput)
				}
			}
		})
	}
}

// TestPricingForModel_OpenAI verifies model routing for OpenAI/Codex models.
func TestPricingForModel_OpenAI(t *testing.T) {
	// codex-mini models should use mini pricing.
	miniPricing := pricingForModel("codex-mini-latest")
	if miniPricing.Input != 1.50 {
		t.Errorf("codex-mini-latest Input: want 1.50, got %v", miniPricing.Input)
	}
	if miniPricing.Output != 6.00 {
		t.Errorf("codex-mini-latest Output: want 6.00, got %v", miniPricing.Output)
	}
	if miniPricing.CacheRead != 0.375 {
		t.Errorf("codex-mini-latest CacheRead: want 0.375, got %v", miniPricing.CacheRead)
	}

	// codex-latest (full) should use full codex pricing.
	fullPricing := pricingForModel("codex-latest")
	if fullPricing.Input != 3.00 {
		t.Errorf("codex-latest Input: want 3.00, got %v", fullPricing.Input)
	}
	if fullPricing.Output != 12.00 {
		t.Errorf("codex-latest Output: want 12.00, got %v", fullPricing.Output)
	}

	// Anthropic models should not be affected.
	opusPricing := pricingForModel("claude-opus-4-6")
	if opusPricing.Input != 5.00 {
		t.Errorf("claude-opus-4-6 Input: want 5.00, got %v", opusPricing.Input)
	}
}

// TestPricingForModel_ActualCodexModelNames verifies that the model names actually
// written by Codex CLI (gpt-5-codex, gpt-5.1-codex-mini) map to non-zero pricing.
func TestPricingForModel_ActualCodexModelNames(t *testing.T) {
	cases := []struct {
		model     string
		wantMini  bool // true = mini tier, false = full tier
	}{
		{"gpt-5.1-codex-mini", true},
		{"gpt-5-codex", false},
		{"codex-mini-latest", true},
		{"codex-latest", false},
		{"gpt-4o-mini", true},    // contains "mini" and "gpt"
		{"gpt-4o", false},        // contains "gpt", no "mini"
	}

	for _, tc := range cases {
		t.Run(tc.model, func(t *testing.T) {
			p := pricingForModel(tc.model)
			if p.Input == 0 {
				t.Errorf("model %q: Input pricing is zero", tc.model)
			}
			if p.Output == 0 {
				t.Errorf("model %q: Output pricing is zero", tc.model)
			}
			if tc.wantMini {
				if p.Input != 1.50 {
					t.Errorf("model %q (mini): Input want 1.50, got %v", tc.model, p.Input)
				}
				if p.Output != 6.00 {
					t.Errorf("model %q (mini): Output want 6.00, got %v", tc.model, p.Output)
				}
			} else {
				if p.Input != 3.00 {
					t.Errorf("model %q (full): Input want 3.00, got %v", tc.model, p.Input)
				}
				if p.Output != 12.00 {
					t.Errorf("model %q (full): Output want 12.00, got %v", tc.model, p.Output)
				}
			}
		})
	}
}
