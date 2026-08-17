package provider

import "testing"

func TestEstimateUSDModelFamilies(t *testing.T) {
	tests := []struct {
		name  string
		model string
		want  float64
	}{
		{"opus", "anthropic/claude-3-opus", 15.0 + 75.0},
		{"sonnet", "anthropic/claude-3.7-sonnet", 3.0 + 15.0},
		{"haiku", "anthropic/claude-3-haiku", 0.8 + 4.0},
		{"o1", "openai/o1", 15.0 + 60.0},
		{"o3", "openai/o3", 2.0 + 8.0},
		{"gpt-4.1", "openai/gpt-4.1", 2.0 + 8.0},
		{"gpt-4o", "openai/gpt-4o", 2.5 + 10.0},
		{"deepseek", "deepseek/deepseek-chat", 0.27 + 1.1},
		{"qwen", "qwen/qwen-2.5-coder-32b-instruct", 0.27 + 1.1},
		{"local", "local/llama3", 1.0 + 3.0},
		{"unknown", "some/vendor-model", 1.0 + 3.0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := EstimateUSD(tc.model, 1_000_000, 1_000_000)
			if got != tc.want {
				t.Errorf("EstimateUSD(%q, 1e6, 1e6) = %v, want %v", tc.model, got, tc.want)
			}
		})
	}
}

func TestEstimateUSDZeroTokens(t *testing.T) {
	for _, model := range []string{"anthropic/claude-3.7-sonnet", "openai/gpt-4o", "local/model"} {
		if got := EstimateUSD(model, 0, 0); got != 0 {
			t.Errorf("EstimateUSD(%q, 0, 0) = %v, want 0", model, got)
		}
	}
}

func TestEstimateUSDGreaterThanZero(t *testing.T) {
	for _, model := range []string{"anthropic/claude-3.7-sonnet", "openai/gpt-4o"} {
		if got := EstimateUSD(model, 1000, 1000); got <= 0 {
			t.Errorf("EstimateUSD(%q, 1000, 1000) = %v, want > 0", model, got)
		}
	}
}
