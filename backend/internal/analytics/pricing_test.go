package analytics

import (
	"testing"

	"github.com/shopspring/decimal"
)

func TestGetModelFamily(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"claude-opus-4-5-20251101", "opus-4-5"},
		{"claude-sonnet-4-20241022", "sonnet-4"},
		{"claude-haiku-3-5-20241022", "haiku-3-5"},
		{"opus-4-5-20251101", "opus-4-5"},
		{"sonnet-3-7", "sonnet-3-7"},
		{"haiku-3", "haiku-3"},
		{"unknown-model", "unknown-model"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := getModelFamily(tt.input)
			if result != tt.expected {
				t.Errorf("getModelFamily(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestGetPricing(t *testing.T) {
	tests := []struct {
		model         string
		expectedInput float64
	}{
		{"claude-opus-4-5-20251101", 5},
		{"claude-sonnet-4-20241022", 3},
		{"claude-haiku-3-5-20241022", 0.80},
		{"unknown-model", 3}, // default is Sonnet 4 pricing
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			pricing := GetPricing(tt.model)
			expected := decimal.NewFromFloat(tt.expectedInput)
			if !pricing.Input.Equal(expected) {
				t.Errorf("GetPricing(%q).Input = %s, want %s", tt.model, pricing.Input, expected)
			}
		})
	}
}

func TestCalculateCost(t *testing.T) {
	// Test with Sonnet 4 pricing: input=$3, output=$15, cacheWrite=$3.75, cacheRead=$0.30 per million
	pricing := GetPricing("claude-sonnet-4-20241022")

	// 1 million input tokens = $3
	cost := CalculateCost(pricing, 1_000_000, 0, 0, 0)
	expected := decimal.NewFromFloat(3)
	if !cost.Equal(expected) {
		t.Errorf("1M input tokens cost = %s, want %s", cost, expected)
	}

	// 1 million output tokens = $15
	cost = CalculateCost(pricing, 0, 1_000_000, 0, 0)
	expected = decimal.NewFromFloat(15)
	if !cost.Equal(expected) {
		t.Errorf("1M output tokens cost = %s, want %s", cost, expected)
	}

	// Combined: 500k input, 100k output, 200k cache write, 1M cache read
	// = 1.50 + 1.50 + 0.75 + 0.30 = $4.05
	cost = CalculateCost(pricing, 500_000, 100_000, 200_000, 1_000_000)
	expected = decimal.NewFromFloat(4.05)
	if !cost.Equal(expected) {
		t.Errorf("Combined cost = %s, want %s", cost, expected)
	}
}
