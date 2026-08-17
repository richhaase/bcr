package provider

import (
	"strings"
)

func EstimateUSD(model string, promptTokens, completionTokens int) float64 {
	promptRate, completionRate := ratesFor(model)
	promptCost := float64(promptTokens) / 1_000_000 * promptRate
	completionCost := float64(completionTokens) / 1_000_000 * completionRate
	return promptCost + completionCost
}

func ratesFor(model string) (prompt, completion float64) {
	switch {
	case strings.Contains(model, "opus"):
		return 15.0, 75.0
	case strings.Contains(model, "sonnet"):
		return 3.0, 15.0
	case strings.Contains(model, "haiku"):
		return 0.8, 4.0
	case strings.Contains(model, "o1"):
		return 15.0, 60.0
	case strings.Contains(model, "o3") || strings.Contains(model, "gpt-4.1"):
		return 2.0, 8.0
	case strings.Contains(model, "gpt-4o"):
		return 2.5, 10.0
	case strings.Contains(model, "deepseek") || strings.Contains(model, "qwen"):
		return 0.27, 1.1
	default:
		return 1.0, 3.0
	}
}
