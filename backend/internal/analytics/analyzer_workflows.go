package analytics

import (
	"bytes"
	"encoding/json"
	"sort"
	"time"

	"github.com/shopspring/decimal"
)

// WorkflowsAnalyzer aggregates Claude Code workflow subagent runs (CF-534).
//
// Unlike the 8 card analyzers it is NOT a FileProcessor and is not in the
// generic streaming processors slice: a run needs a runId per agent file plus a
// non-transcript journal, so ComputeStreaming drives it explicitly via
// ProcessAgent / ProcessJournal / Result. Runs group by the <runId> path
// segment of subagents/workflows/<runId>/agent-<id>.jsonl (see
// ExtractWorkflowRunID); runId is resolved by the caller and passed in, so the
// shared TranscriptFile struct stays workflow-agnostic.
type WorkflowsAnalyzer struct {
	runs map[string]*workflowRunAccum
}

type workflowRunAccum struct {
	runID         string
	agentCount    int
	agentIDs      map[string]bool // agent ids that have a file in this run
	input         int64
	output        int64
	cacheCreation int64
	cacheRead     int64
	cost          decimal.Decimal
	hasJournal    bool
	succeeded     int
	started       time.Time
	ended         time.Time
	hasTime       bool
}

func (a *WorkflowsAnalyzer) ensure(runID string) *workflowRunAccum {
	if a.runs == nil {
		a.runs = make(map[string]*workflowRunAccum)
	}
	acc := a.runs[runID]
	if acc == nil {
		acc = &workflowRunAccum{runID: runID, cost: decimal.Zero, agentIDs: map[string]bool{}}
		a.runs[runID] = acc
	}
	return acc
}

// ProcessAgent accumulates one workflow agent transcript into its run. A blank
// runID (ordinary, non-workflow agent file) is ignored. Token/cost arithmetic
// mirrors TokensAnalyzer.ProcessFile so per-run subtotals match the headline.
func (a *WorkflowsAnalyzer) ProcessAgent(file *TranscriptFile, runID string) {
	if runID == "" || file == nil {
		return
	}
	acc := a.ensure(runID)
	acc.agentCount++
	if file.AgentID != "" {
		acc.agentIDs[file.AgentID] = true
	}

	for _, group := range file.AssistantMessageGroups() {
		if group.FinalUsage == nil {
			continue
		}
		usage := group.FinalUsage
		acc.input += usage.InputTokens
		acc.output += usage.OutputTokens
		acc.cacheCreation += usage.CacheCreationInputTokens
		acc.cacheRead += usage.CacheReadInputTokens
		acc.cost = acc.cost.Add(CalculateTotalCost(GetPricing(group.Model), usage))
	}

	// Track the run's activity span from line timestamps.
	for _, line := range file.Lines {
		ts, err := line.GetTimestamp()
		if err != nil {
			continue
		}
		if !acc.hasTime || ts.Before(acc.started) {
			acc.started = ts
		}
		if !acc.hasTime || ts.After(acc.ended) {
			acc.ended = ts
		}
		acc.hasTime = true
	}
}

// workflowJournalLine is one line of journal.jsonl (CF-533 locked schema:
// {type:"started"|"result", key, agentId, result?}). Only type+agentId matter
// for status; the result payload is intentionally ignored.
type workflowJournalLine struct {
	Type    string `json:"type"`
	AgentID string `json:"agentId"`
}

// ProcessJournal records per-agent success for an already-seen run from its
// journal. An agent with a "result" line succeeded; agents with only a
// "started" line are incomplete (errored or still running — indistinguishable
// in the locked schema). Only agents that have a transcript file in this run
// are counted, so SucceededAgents never exceeds AgentCount (a journal can list
// agents whose files were capped or failed to download). Journals for runs with
// no agent files are ignored so no phantom run is fabricated.
func (a *WorkflowsAnalyzer) ProcessJournal(runID string, content []byte) {
	if runID == "" || a.runs == nil {
		return
	}
	acc := a.runs[runID]
	if acc == nil {
		return
	}
	acc.hasJournal = true

	succeeded := make(map[string]bool)
	for _, raw := range bytes.Split(content, []byte("\n")) {
		raw = bytes.TrimSpace(raw)
		if len(raw) == 0 {
			continue
		}
		var line workflowJournalLine
		if err := json.Unmarshal(raw, &line); err != nil {
			continue
		}
		if line.Type == "result" && acc.agentIDs[line.AgentID] {
			succeeded[line.AgentID] = true
		}
	}
	acc.succeeded = len(succeeded)
}

// Result returns per-run aggregates sorted by start time (stable). Cost is
// serialized as a string like the Tokens card.
func (a *WorkflowsAnalyzer) Result() []WorkflowRun {
	runs := make([]WorkflowRun, 0, len(a.runs))
	for _, acc := range a.runs {
		var durationMs int64
		if acc.hasTime {
			durationMs = acc.ended.Sub(acc.started).Milliseconds()
		}
		runs = append(runs, WorkflowRun{
			RunID:           acc.runID,
			AgentCount:      acc.agentCount,
			InputTokens:     acc.input,
			OutputTokens:    acc.output,
			CacheCreation:   acc.cacheCreation,
			CacheRead:       acc.cacheRead,
			EstimatedUSD:    acc.cost.String(),
			SucceededAgents: acc.succeeded,
			HasJournal:      acc.hasJournal,
			DurationMs:      durationMs,
			StartedAt:       acc.started,
		})
	}
	sort.SliceStable(runs, func(i, j int) bool {
		return runs[i].StartedAt.Before(runs[j].StartedAt)
	})
	return runs
}
