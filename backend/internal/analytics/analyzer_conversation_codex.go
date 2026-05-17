package analytics

import (
	"sort"
	"time"

	"github.com/ConfabulousDev/confab-web/internal/codex"
)

// computeCodexConversation fills the Conversation card for Codex sessions.
//
// Counts (preserved from the original Codex adapter):
//   - UserTurns = sum of user messages across turns.
//   - AssistantTurns = user-prompt-triggered sequences that received ≥1
//     assistant message before the next user prompt. Mirrors Claude's
//     deduped per-prompt count, not Codex's raw task cycle count.
//
// Timing (CF-441 addition — mirrors analyzer_conversation_claude.go:55-145):
//   - Walk all message events from all turns flat, sorted by timestamp.
//   - For each user-prompt-triggered window, record:
//   - assistant turn duration = lastAsstTs - userTs
//   - user thinking time     = thisUserTs - prevAsstTs
//   - Aggregate into avg/total and the utilization percentage.
//
// Reasoning-as-active-time (Codex-specific divergence from Claude):
// Encrypted reasoning items have no per-event timestamp but represent real
// assistant activity. For a turn with ReasoningCount > 0 and CompletedAt
// non-nil, we synthesize an assistant event at Turn.CompletedAt so the
// assistant window extends to task_complete. When CompletedAt is nil (open
// turn at end of session), we skip — no anchor to attach reasoning to.
func computeCodexConversation(out *ComputeResult, r *codex.ParsedRollout) {
	// AssistantTurns: window each user prompt against the next prompt within
	// the same turn (mid-stream user input). Rationale in the func docstring.
	for _, turn := range r.Turns {
		out.UserTurns += len(turn.UserMessages)
		for i, prompt := range turn.UserMessages {
			hasNext := i+1 < len(turn.UserMessages)
			for _, asst := range turn.AssistantMessages {
				if asst.Timestamp.Before(prompt.Timestamp) {
					continue
				}
				if hasNext && !asst.Timestamp.Before(turn.UserMessages[i+1].Timestamp) {
					continue
				}
				out.AssistantTurns++
				break
			}
		}
	}

	// Timing: flatten and sort.
	type event struct {
		ts   time.Time
		role string // "user" | "assistant"
	}
	var events []event
	for _, turn := range r.Turns {
		for _, m := range turn.UserMessages {
			events = append(events, event{m.Timestamp, "user"})
		}
		for _, m := range turn.AssistantMessages {
			events = append(events, event{m.Timestamp, "assistant"})
		}
		// Synthetic reasoning anchor: see docstring above.
		if turn.ReasoningCount > 0 && turn.CompletedAt != nil {
			events = append(events, event{*turn.CompletedAt, "assistant"})
		}
	}
	sort.SliceStable(events, func(i, j int) bool { return events[i].ts.Before(events[j].ts) })

	// Walk events, closing one assistant-turn window per user prompt and one
	// user-thinking sample per non-first user prompt that follows an asst.
	var lastUserTs, lastAsstTs *time.Time
	var hadAsstResp bool
	var asstDurs, userDurs []int64

	for _, e := range events {
		if e.ts.IsZero() {
			// Zero-ts events are skipped entirely. Diverges from Claude's
			// analyzer (analyzer_conversation_claude.go:65-67) which
			// triggers a full state reset on a zero-ts user message — but
			// that semantic relies on JSONL stream position, which doesn't
			// translate to Codex's parse-and-sort model (zero-ts events
			// would land at the front of the sorted list, before any real
			// data, making any positional reset a no-op anyway). Skipping
			// achieves the same intent (don't generate misleading data
			// from corrupted timestamps) in a way that fits the structure.
			continue
		}

		if e.role == "user" {
			// Close the previous window before opening a new one.
			if lastUserTs != nil && lastAsstTs != nil && hadAsstResp {
				if d := lastAsstTs.Sub(*lastUserTs).Milliseconds(); d >= 0 {
					asstDurs = append(asstDurs, d)
				}
			}
			if lastAsstTs != nil {
				if t := e.ts.Sub(*lastAsstTs).Milliseconds(); t >= 0 {
					userDurs = append(userDurs, t)
				}
			}
			ts := e.ts
			lastUserTs, lastAsstTs, hadAsstResp = &ts, nil, false
			continue
		}

		// assistant event
		ts := e.ts
		lastAsstTs = &ts
		hadAsstResp = true
	}

	// Trailing assistant window (session ended without another user prompt).
	if lastUserTs != nil && lastAsstTs != nil && hadAsstResp {
		if d := lastAsstTs.Sub(*lastUserTs).Milliseconds(); d >= 0 {
			asstDurs = append(asstDurs, d)
		}
	}

	// Aggregate into the four duration pointers and the utilization percentage.
	out.AvgAssistantTurnMs, out.TotalAssistantDurationMs = avgAndTotal(asstDurs)
	out.AvgUserThinkingMs, out.TotalUserDurationMs = avgAndTotal(userDurs)
	if out.TotalAssistantDurationMs != nil && out.TotalUserDurationMs != nil {
		total := *out.TotalAssistantDurationMs + *out.TotalUserDurationMs
		if total > 0 {
			pct := float64(*out.TotalAssistantDurationMs) / float64(total) * 100
			out.AssistantUtilizationPct = &pct
		}
	}
}

// avgAndTotal returns (avg, total) for a slice of durations, or (nil, nil) if
// the slice is empty. Avg uses int64 division to match Claude's
// ConversationAnalyzer exactly (see analyzer_conversation_claude.go).
func avgAndTotal(durs []int64) (avg, total *int64) {
	if len(durs) == 0 {
		return nil, nil
	}
	var sum int64
	for _, d := range durs {
		sum += d
	}
	a := sum / int64(len(durs))
	return &a, &sum
}
