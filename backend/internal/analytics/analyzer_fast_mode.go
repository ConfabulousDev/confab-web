package analytics

import "github.com/shopspring/decimal"

// FastModeResult contains fast mode usage metrics.
type FastModeResult struct {
	FastTurns       int
	StandardTurns   int
	FastCostUSD     decimal.Decimal // Total cost of fast mode turns (with 6x multiplier)
	StandardCostUSD decimal.Decimal // Total cost of standard (non-fast) turns
}

// FastModeAnalyzer extracts fast mode usage metrics from transcripts.
// It counts turns using fast mode vs standard, and splits total cost by speed.
type FastModeAnalyzer struct{}

// Analyze processes the file collection and returns fast mode metrics.
func (a *FastModeAnalyzer) Analyze(fc *FileCollection) (*FastModeResult, error) {
	result := &FastModeResult{
		FastCostUSD:     decimal.Zero,
		StandardCostUSD: decimal.Zero,
	}

	// Process all files - main and agents
	for _, file := range fc.AllFiles() {
		for _, line := range file.Lines {
			if !line.IsAssistantMessage() {
				continue
			}

			usage := line.Message.Usage
			pricing := GetPricing(line.Message.Model)
			cost := CalculateTotalCost(pricing, usage)

			if usage.Speed == "fast" {
				result.FastTurns++
				result.FastCostUSD = result.FastCostUSD.Add(cost)
			} else {
				result.StandardTurns++
				result.StandardCostUSD = result.StandardCostUSD.Add(cost)
			}
		}
	}

	// Fallback: count turns from toolUseResult for agents without files
	for _, line := range fc.Main.Lines {
		for _, agentResult := range line.GetAgentResults() {
			if fc.HasAgentFile(agentResult.AgentID) {
				continue // Already counted from agent file
			}
			if agentResult.Usage == nil {
				continue
			}

			usage := agentResult.Usage
			pricing := GetPricing("")
			cost := CalculateTotalCost(pricing, usage)

			if usage.Speed == "fast" {
				result.FastTurns++
				result.FastCostUSD = result.FastCostUSD.Add(cost)
			} else {
				result.StandardTurns++
				result.StandardCostUSD = result.StandardCostUSD.Add(cost)
			}
		}
	}

	return result, nil
}
