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
	type modelAgg struct {
		input, output, cacheRead, cacheWrite, reasoning int64
		cost                                            decimal.Decimal
	}
	byModel := make(map[modelKey]*modelAgg)

	for _, msg := range r.Messages {
		// OpenCode emits an assistant JSONL line only once the turn is settled
		// (finish set) or errored, so every assistant message carries final
		// token + cost accounting. No finish gate — gating on finish would drop
		// errored turns' tokens/cost.
		if msg.Info.Role != "assistant" {
			continue
		}

		// Normalize tokens to the provider's billing convention, per message.
		// OpenAI bills cached input as a subset of `input` and never charges
		// cache writes; everyone else (Anthropic-style) bills writes and treats
		// reads as independent of input.
		input := msg.Info.Tokens.Input
		cacheWrite := msg.Info.Tokens.Cache.Write
		if msg.Info.ProviderID == "openai" {
			input -= msg.Info.Tokens.Cache.Read
			if input < 0 {
				input = 0
			}
			cacheWrite = 0
		}

		// Cost source (hybrid), decided per message: prefer OpenCode's
		// authoritative reported `info.cost` (it already encodes each of its 75+
		// providers' real pricing); fall back to our family-resolved pricing
		// table only when this message reported nothing. Per-message (not
		// per-group) so a session mixing reported and unreported messages bills
		// each correctly instead of letting one reported message zero-rate its
		// silent siblings.
		var cost decimal.Decimal
		if msg.Info.Cost > 0 {
			cost = decimal.NewFromFloat(msg.Info.Cost)
		} else {
			cost = CalculateCost(GetPricing(msg.Info.ModelID), input, msg.Info.Tokens.Output, cacheWrite, msg.Info.Tokens.Cache.Read)
		}

		key := modelKey{msg.Info.ProviderID, msg.Info.ModelID}
		agg := byModel[key]
		if agg == nil {
			agg = &modelAgg{}
			byModel[key] = agg
		}
		agg.input += input
		agg.output += msg.Info.Tokens.Output
		agg.cacheRead += msg.Info.Tokens.Cache.Read
		agg.cacheWrite += cacheWrite
		agg.reasoning += msg.Info.Tokens.Reasoning
		agg.cost = agg.cost.Add(cost)
	}

	var totalInput, totalOutput, totalCacheRead, totalCacheWrite int64
	var totalCost decimal.Decimal

	// Accumulate the tokens_v2 tree with decimal provider costs, serialized to
	// strings only at the end. Costs use decimal.String() (full precision, no
	// fixed scale) to match the flat tokens card's serialization exactly.
	type provAccum struct {
		models map[string]TokensV2Model
		cost   decimal.Decimal
	}
	byProvider := make(map[string]*provAccum)

	for key, agg := range byModel {
		totalInput += agg.input
		totalOutput += agg.output
		totalCacheRead += agg.cacheRead
		totalCacheWrite += agg.cacheWrite
		totalCost = totalCost.Add(agg.cost)

		prov := byProvider[key.providerID]
		if prov == nil {
			prov = &provAccum{models: make(map[string]TokensV2Model)}
			byProvider[key.providerID] = prov
		}
		prov.models[key.modelID] = TokensV2Model{
			Input:      agg.input,
			Output:     agg.output,
			CacheRead:  agg.cacheRead,
			CacheWrite: agg.cacheWrite,
			Reasoning:  agg.reasoning,
			CostUSD:    agg.cost.String(),
		}
		prov.cost = prov.cost.Add(agg.cost)
	}

	providers := make(map[string]TokensV2Provider, len(byProvider))
	for id, acc := range byProvider {
		providers[id] = TokensV2Provider{
			CostUSD: acc.cost.String(),
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
		TotalCostUSD: totalCost.String(),
		TotalInput:   totalInput,
		TotalOutput:  totalOutput,
		ByProvider:   providers,
	}
}

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
	var userTurns, assistantTurns int
	for _, msg := range r.Messages {
		events = append(events, event{msg.Info.Time.Created, msg.Info.Role})
		switch msg.Info.Role {
		case "user":
			userTurns++
		case "assistant":
			// Completed turns only (errored/unfinished turns still count as
			// assistant *messages* in the session card, but not as turns here).
			if msg.Info.Finish != nil {
				assistantTurns++
			}
		}
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

	out.UserTurns = userTurns
	out.AssistantTurns = assistantTurns
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
