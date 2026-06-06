package analytics

import (
	"sort"

	"github.com/shopspring/decimal"
)

func ComputeFromOpenCodeRollout(r *opencodeRollout) *ComputeResult {
	if r == nil || len(r.Messages) == 0 {
		return &ComputeResult{}
	}

	result := &ComputeResult{
		ToolStats:         make(map[string]*ToolStats),
		LanguageBreakdown: make(map[string]int),
		AgentStats:        make(map[string]*AgentStats),
		SkillStats:        make(map[string]*SkillStats),
		RedactionCounts:   make(map[string]int),
	}

	computeOpenCodeTokens(result, r)
	computeOpenCodeSession(result, r)
	computeOpenCodeTools(result, r)
	computeOpenCodeCodeActivity(result, r)
	computeOpenCodeConversation(result, r)
	computeOpenCodeAgentsAndSkills(result, r)
	computeOpenCodeRedactions(result, r)

	return result
}

func getStringInput(state *OpenCodeToolState, key string) string {
	if state == nil || state.Input == nil {
		return ""
	}
	if v, ok := state.Input[key].(string); ok {
		return v
	}
	return ""
}

func computeOpenCodeTokens(out *ComputeResult, r *opencodeRollout) {
	type modelKey struct {
		providerID string
		modelID    string
	}
	type modelTokens struct {
		input, output, cacheRead, cacheWrite, reasoning int64
		reportedCost                                    decimal.Decimal // sum of OpenCode's per-message info.cost
	}
	byModel := make(map[modelKey]*modelTokens)

	for _, msg := range r.Messages {
		if msg.Info.Role != "assistant" || msg.Info.Finish == nil {
			continue
		}
		key := modelKey{msg.Info.ProviderID, msg.Info.ModelID}
		mt := byModel[key]
		if mt == nil {
			mt = &modelTokens{}
			byModel[key] = mt
		}
		mt.input += msg.Info.Tokens.Input
		mt.output += msg.Info.Tokens.Output
		mt.cacheRead += msg.Info.Tokens.Cache.Read
		mt.cacheWrite += msg.Info.Tokens.Cache.Write
		mt.reasoning += msg.Info.Tokens.Reasoning
		if msg.Info.Cost > 0 {
			mt.reportedCost = mt.reportedCost.Add(decimal.NewFromFloat(msg.Info.Cost))
		}
	}

	var totalInput, totalOutput, totalCacheRead, totalCacheWrite int64
	var totalCost decimal.Decimal

	// Accumulate the tokens_v2 tree with decimal provider costs, serialized to
	// strings only at the end to avoid repeated string round-trips.
	type provAccum struct {
		models map[string]TokensV2Model
		cost   decimal.Decimal
	}
	byProvider := make(map[string]*provAccum)

	for key, mt := range byModel {
		// Normalize tokens per the provider's billing convention. OpenAI bills
		// cached input as a subset of `input` and never charges for cache
		// writes; everyone else (Anthropic-style) bills cache writes and treats
		// cache reads as independent of input. These normalized counts drive the
		// displayed token breakdown and the pricing-table cost fallback below.
		var input, cacheWrite int64
		switch key.providerID {
		case "openai":
			uncached := mt.input - mt.cacheRead
			if uncached < 0 {
				uncached = 0
			}
			input = uncached
			cacheWrite = 0
		default:
			input = mt.input
			cacheWrite = mt.cacheWrite
		}

		totalInput += input
		totalOutput += mt.output
		totalCacheRead += mt.cacheRead
		totalCacheWrite += cacheWrite

		// Cost source (hybrid): OpenCode reports an authoritative per-message
		// cost that already encodes each of its 75+ providers' real pricing, so
		// prefer the summed reported cost. Fall back to our pricing table only
		// when OpenCode reported nothing for the group (cost 0) — e.g. local
		// models or older daemons — using the family-resolved table. The table
		// prices only a handful of OpenCode models, so without this preference
		// the long tail of providers would bill $0.
		var cost decimal.Decimal
		if mt.reportedCost.IsPositive() {
			cost = mt.reportedCost
		} else {
			cost = CalculateCost(GetPricing(key.modelID), input, mt.output, cacheWrite, mt.cacheRead)
		}
		totalCost = totalCost.Add(cost)

		// Build the hierarchical tokens_v2 tree alongside the flat totals so the
		// nested per-provider/per-model breakdown stays consistent with the flat
		// tokens card (same normalized inputs, same cost).
		prov := byProvider[key.providerID]
		if prov == nil {
			prov = &provAccum{models: make(map[string]TokensV2Model)}
			byProvider[key.providerID] = prov
		}
		prov.models[key.modelID] = TokensV2Model{
			Input:      input,
			Output:     mt.output,
			CacheRead:  mt.cacheRead,
			CacheWrite: cacheWrite,
			Reasoning:  mt.reasoning,
			CostUSD:    cost.StringFixed(decimalCostScale),
		}
		prov.cost = prov.cost.Add(cost)
	}

	providers := make(map[string]TokensV2Provider, len(byProvider))
	for id, acc := range byProvider {
		providers[id] = TokensV2Provider{
			CostUSD: acc.cost.StringFixed(decimalCostScale),
			Models:  acc.models,
		}
	}

	out.InputTokens = totalInput
	out.OutputTokens = totalOutput
	out.CacheReadTokens = totalCacheRead
	out.CacheCreationTokens = totalCacheWrite
	out.EstimatedCostUSD = totalCost
	out.FastTurns = 0
	out.FastCostUSD = decimal.Zero

	out.TokensV2 = &TokensV2Data{
		TotalCostUSD: totalCost.StringFixed(decimalCostScale),
		TotalInput:   totalInput,
		TotalOutput:  totalOutput,
		ByProvider:   providers,
	}
}

// decimalCostScale is the fixed number of decimal places used when serializing
// tokens_v2 costs as strings. Matches the frontend's decimal-string parsing.
const decimalCostScale = 6

func computeOpenCodeSession(out *ComputeResult, r *opencodeRollout) {
	models := map[string]struct{}{}
	var minTime, maxTime *int64

	for _, msg := range r.Messages {
		if msg.Info.ModelID != "" {
			models[msg.Info.ModelID] = struct{}{}
		}

		if msg.Info.Role == "user" {
			out.UserMessages++
			out.HumanPrompts++
		} else if msg.Info.Role == "assistant" {
			out.AssistantMessages++
			parts := msg.Parts
			hasText := false
			hasReasoning := false
			for _, p := range parts {
				switch p.Type {
				case "text":
					hasText = true
				case "reasoning":
					hasReasoning = true
				case "tool":
					state := p.State
					if state != nil && (state.Status == "completed" || state.Status == "error") {
						out.ToolCalls++
						if state.Output != "" {
							out.ToolResults++
						}
					}
				case "compaction":
					if p.Auto != nil {
						if *p.Auto {
							out.CompactionAuto++
						} else {
							out.CompactionManual++
						}
					}
				}
			}
			if hasText {
				out.TextResponses++
			}
			if hasReasoning {
				out.ThinkingBlocks++
			}
		}

		ts := msg.Info.Time.Created
		if minTime == nil || ts < *minTime {
			minTime = &ts
		}
		if maxTime == nil || ts > *maxTime {
			maxTime = &ts
		}
	}

	out.TotalMessages = out.UserMessages + out.AssistantMessages + (out.ToolCalls * 2)
	out.ModelsUsed = sortedKeys(models)

	if minTime != nil && maxTime != nil {
		d := *maxTime - *minTime
		if d >= 0 {
			out.DurationMs = &d
		}
	}
}

func computeOpenCodeTools(out *ComputeResult, r *opencodeRollout) {
	for _, msg := range r.Messages {
		if msg.Info.Role != "assistant" {
			continue
		}
		parts := msg.Parts
		for _, p := range parts {
			if p.Type != "tool" {
				continue
			}
			state := p.State
			if state == nil {
				continue
			}
			if state.Status != "completed" && state.Status != "error" {
				continue
			}
			name := p.Tool
			if name == "" {
				continue
			}
			out.TotalToolCalls++
			if out.ToolStats[name] == nil {
				out.ToolStats[name] = &ToolStats{}
			}
			if state.Status == "error" {
				out.ToolStats[name].Errors++
				out.ToolErrorCount++
			} else {
				out.ToolStats[name].Success++
			}
		}
	}
}

func computeOpenCodeCodeActivity(out *ComputeResult, r *opencodeRollout) {
	for _, msg := range r.Messages {
		if msg.Info.Role != "assistant" {
			continue
		}
		parts := msg.Parts
		for _, p := range parts {
			if p.Type != "tool" {
				continue
			}
			state := p.State
			if state == nil || state.Status != "completed" {
				continue
			}
			fp := getStringInput(state, "file_path")
			switch p.Tool {
			case "Read":
				if fp != "" {
					out.FilesRead++
					if lang := languageFromPath(fp); lang != "" {
						out.LanguageBreakdown[lang]++
					}
				}
			case "Write":
				if fp != "" {
					out.FilesModified++
					if lang := languageFromPath(fp); lang != "" {
						out.LanguageBreakdown[lang]++
					}
					out.LinesAdded += countLines(getStringInput(state, "content"))
				}
			case "Edit":
				if fp != "" {
					out.FilesModified++
					if lang := languageFromPath(fp); lang != "" {
						out.LanguageBreakdown[lang]++
					}
					out.LinesRemoved += countLines(getStringInput(state, "old_string"))
					out.LinesAdded += countLines(getStringInput(state, "new_string"))
				}
			case "Grep", "Glob":
				out.SearchCount++
			}
		}
	}
}

func computeOpenCodeConversation(out *ComputeResult, r *opencodeRollout) {
	type event struct {
		ts   int64
		role string
	}
	var events []event
	for _, msg := range r.Messages {
		events = append(events, event{msg.Info.Time.Created, msg.Info.Role})
	}
	sort.SliceStable(events, func(i, j int) bool { return events[i].ts < events[j].ts })

	var lastUserTs, lastAsstTs *int64
	var hadAsstResp bool
	var asstDurs, userDurs []int64

	for _, e := range events {
		if e.role == "user" {
			if lastUserTs != nil && lastAsstTs != nil && hadAsstResp {
				if d := *lastAsstTs - *lastUserTs; d >= 0 {
					asstDurs = append(asstDurs, d)
				}
			}
			if lastAsstTs != nil {
				if t := e.ts - *lastAsstTs; t >= 0 {
					userDurs = append(userDurs, t)
				}
			}
			ts := e.ts
			lastUserTs, lastAsstTs, hadAsstResp = &ts, nil, false
		} else if e.role == "assistant" {
			ts := e.ts
			lastAsstTs = &ts
			hadAsstResp = true
		}
	}

	if lastUserTs != nil && lastAsstTs != nil && hadAsstResp {
		if d := *lastAsstTs - *lastUserTs; d >= 0 {
			asstDurs = append(asstDurs, d)
		}
	}

	out.AvgAssistantTurnMs, out.TotalAssistantDurationMs = avgAndTotal(asstDurs)
	out.AvgUserThinkingMs, out.TotalUserDurationMs = avgAndTotal(userDurs)
	if out.TotalAssistantDurationMs != nil && out.TotalUserDurationMs != nil {
		total := *out.TotalAssistantDurationMs + *out.TotalUserDurationMs
		if total > 0 {
			pct := float64(*out.TotalAssistantDurationMs) / float64(total) * 100
			out.AssistantUtilizationPct = &pct
		}
	}

	out.UserTurns = out.UserMessages
	for _, msg := range r.Messages {
		if msg.Info.Role == "assistant" && msg.Info.Finish != nil {
			out.AssistantTurns++
		}
	}
}

func computeOpenCodeAgentsAndSkills(out *ComputeResult, r *opencodeRollout) {
	for _, msg := range r.Messages {
		if msg.Info.Role != "assistant" {
			continue
		}
		parts := msg.Parts
		for _, p := range parts {
			if p.Type != "subtask" {
				continue
			}
			name := p.Name
			if name == "" {
				name = "unknown"
			}
			out.TotalAgentInvocations++
			if out.AgentStats[name] == nil {
				out.AgentStats[name] = &AgentStats{}
			}
			out.AgentStats[name].Success++
		}
	}
}

func computeOpenCodeRedactions(out *ComputeResult, r *opencodeRollout) {
	count := func(s string) {
		matches := redactionPattern.FindAllStringSubmatch(s, -1)
		for _, m := range matches {
			if len(m) < 2 || m[1] == "TYPE" {
				continue
			}
			out.RedactionCounts[m[1]]++
			out.TotalRedactions++
		}
	}

	for _, msg := range r.Messages {
		parts := msg.Parts
		for _, p := range parts {
			switch p.Type {
			case "text":
				count(p.Text)
			case "tool":
				state := p.State
				if state != nil {
					count(state.Output)
					count(state.Error)
					for _, v := range state.Input {
						if s, ok := v.(string); ok {
							count(s)
						}
					}
				}
			case "reasoning":
				count(p.Text)
			}
		}
	}
}
