package analytics

import (
	"encoding/json"
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

func parseParts(raw json.RawMessage) []OpenCodePart {
	if len(raw) == 0 {
		return nil
	}
	var parts []OpenCodePart
	if err := json.Unmarshal(raw, &parts); err != nil {
		return nil
	}
	return parts
}

func parseToolState(raw json.RawMessage) *OpenCodeToolState {
	if len(raw) == 0 {
		return nil
	}
	var state OpenCodeToolState
	if err := json.Unmarshal(raw, &state); err != nil {
		return nil
	}
	return &state
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
		input, output, cacheRead, cacheWrite int64
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
	}

	var totalInput, totalOutput, totalCacheRead, totalCacheWrite int64
	var totalCost decimal.Decimal

	for key, mt := range byModel {
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

		pricing := GetPricing(key.modelID)
		cost := CalculateCost(pricing, input, mt.output, cacheWrite, mt.cacheRead)
		totalCost = totalCost.Add(cost)
	}

	out.InputTokens = totalInput
	out.OutputTokens = totalOutput
	out.CacheReadTokens = totalCacheRead
	out.CacheCreationTokens = totalCacheWrite
	out.EstimatedCostUSD = totalCost
	out.FastTurns = 0
	out.FastCostUSD = decimal.Zero
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
			parts := parseParts(msg.Parts)
			hasText := false
			hasReasoning := false
			for _, p := range parts {
				switch p.Type {
				case "text":
					hasText = true
				case "reasoning":
					hasReasoning = true
				case "tool":
					state := parseToolState(p.State)
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
		parts := parseParts(msg.Parts)
		for _, p := range parts {
			if p.Type != "tool" {
				continue
			}
			state := parseToolState(p.State)
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
		parts := parseParts(msg.Parts)
		for _, p := range parts {
			if p.Type != "tool" {
				continue
			}
			state := parseToolState(p.State)
			if state == nil || state.Status != "completed" {
				continue
			}
			switch p.Tool {
			case "Read":
				if fp, ok := state.Input["file_path"].(string); ok && fp != "" {
					out.FilesRead++
					if lang := languageFromPath(fp); lang != "" {
						out.LanguageBreakdown[lang]++
					}
				}
			case "Write":
				if fp, ok := state.Input["file_path"].(string); ok && fp != "" {
					out.FilesModified++
					if lang := languageFromPath(fp); lang != "" {
						out.LanguageBreakdown[lang]++
					}
					out.LinesAdded += countLines(getStringInput(state, "content"))
				}
			case "Edit":
				if fp, ok := state.Input["file_path"].(string); ok && fp != "" {
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
		parts := parseParts(msg.Parts)
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
		parts := parseParts(msg.Parts)
		for _, p := range parts {
			switch p.Type {
			case "text":
				count(p.Text)
			case "tool":
				state := parseToolState(p.State)
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
