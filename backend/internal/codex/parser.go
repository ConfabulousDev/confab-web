package codex

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// maxLineBytes is the bufio.Scanner buffer cap. Codex assistant outputs and
// patch payloads occasionally run into the hundreds of KB.
const maxLineBytes = 4 * 1024 * 1024

// envContextPattern strips the <environment_context>…</environment_context>
// preamble Codex injects on the first user message. Matches greedily across
// newlines (DOTALL behavior via [\s\S]).
var envContextPattern = regexp.MustCompile(`(?s)<environment_context>.*?</environment_context>`)

// execOutputSentinel separates the Codex exec_command output preamble from
// the actual command output. The sentinel sits on its own line, so we match
// either at start-of-string or after a newline.
const execOutputSentinel = "Output:\n"

// execExitPattern parses "Process exited with code N" from the preamble.
var execExitPattern = regexp.MustCompile(`Process exited with code\s+(-?\d+)`)

// execWallPattern parses "Wall time: X seconds" (X may be float).
var execWallPattern = regexp.MustCompile(`Wall time:\s+([0-9.]+)\s*seconds?`)

// rawLine is the top-level JSONL envelope. payload is held as raw bytes so
// each line type can decode its payload through its own typed struct.
type rawLine struct {
	Timestamp string          `json:"timestamp"`
	Type      string          `json:"type"`
	Payload   json.RawMessage `json:"payload"`
}

// payloadType peeks at payload.type without committing to a full schema.
type payloadType struct {
	Type string `json:"type"`
}

// ParseRollout streams a Codex JSONL file and returns a normalized rollout.
// See package documentation for the parsing contract.
func ParseRollout(r io.Reader) (*ParsedRollout, error) {
	rollout := &ParsedRollout{
		GitInfo: map[string]interface{}{},
	}
	p := &parser{out: rollout}

	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64*1024), maxLineBytes)

	lineNum := 0
	for scanner.Scan() {
		lineNum++
		if len(bytes.TrimSpace(scanner.Bytes())) == 0 {
			continue
		}
		rollout.TotalLines++

		var line rawLine
		if err := json.Unmarshal(scanner.Bytes(), &line); err != nil {
			rollout.ValidationErrors = append(rollout.ValidationErrors, ValidationError{
				Line:   lineNum,
				Reason: "JSON decode: " + err.Error(),
			})
			continue
		}

		ts, _ := time.Parse(time.RFC3339Nano, line.Timestamp)
		p.dispatch(line.Type, ts, line.Payload, lineNum)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan rollout: %w", err)
	}

	p.closeOpenTurn()
	return rollout, nil
}

// parser holds the streaming state machine used during ParseRollout.
type parser struct {
	out *ParsedRollout
	// active is the in-progress turn (nil between task_complete and the next
	// task_started / implicit-turn trigger).
	active *Turn
	// callIndex maps call_id → (turnIdx, toolCallIdx) so out-of-order tool
	// outputs can update the right ToolCall even after their turn closed.
	callIndex map[string]callRef
}

type callRef struct {
	turnIdx int
	toolIdx int
}

// dispatch routes a parsed line to its handler based on top-level type.
func (p *parser) dispatch(typ string, ts time.Time, payload json.RawMessage, lineNum int) {
	switch typ {
	case "session_meta":
		p.handleSessionMeta(payload)
	case "turn_context":
		p.handleTurnContext(payload)
	case "response_item":
		p.handleResponseItem(ts, payload, lineNum)
	case "event_msg":
		p.handleEventMsg(ts, payload, lineNum)
	case "compacted":
		p.handleCompacted(ts, payload)
	default:
		// Forward-compat: unknown top-level types are silently skipped.
	}
}

// ----------------------------------------------------------------------------
// session_meta
// ----------------------------------------------------------------------------

type sessionMetaPayload struct {
	Model         string                 `json:"model"`
	ModelProvider string                 `json:"model_provider"`
	CWD           string                 `json:"cwd"`
	Git           map[string]interface{} `json:"git"`
}

func (p *parser) handleSessionMeta(raw json.RawMessage) {
	var meta sessionMetaPayload
	if err := json.Unmarshal(raw, &meta); err != nil {
		return
	}
	if meta.Model != "" {
		p.out.Model = meta.Model
	}
	if meta.ModelProvider != "" {
		p.out.ModelProvider = meta.ModelProvider
	}
	if meta.CWD != "" {
		p.out.CWD = meta.CWD
	}
	if len(meta.Git) > 0 {
		p.out.GitInfo = meta.Git
	}
}

// ----------------------------------------------------------------------------
// turn_context
// ----------------------------------------------------------------------------

type turnContextPayload struct {
	Model string `json:"model"`
}

// handleTurnContext fills Model from the per-turn envelope. Codex CLI
// ~0.130+ moved `model` out of session_meta into turn_context; without this
// the rollout's Model stays empty and pricing falls back to $0. Session-
// level Model is only filled when unset (session_meta wins for older
// rollouts that carry both); the active turn picks up the model when
// task_started didn't carry one.
func (p *parser) handleTurnContext(raw json.RawMessage) {
	var tc turnContextPayload
	if err := json.Unmarshal(raw, &tc); err != nil {
		return
	}
	if tc.Model == "" {
		return
	}
	if p.out.Model == "" {
		p.out.Model = tc.Model
	}
	if p.active != nil && p.active.Model == "" {
		p.active.Model = tc.Model
	}
}

// ----------------------------------------------------------------------------
// response_item
// ----------------------------------------------------------------------------

type responseMessagePayload struct {
	Role    string                 `json:"role"`
	Content []responseContentBlock `json:"content"`
	Phase   string                 `json:"phase"`
}

type responseContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type functionCallPayload struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
	CallID    string `json:"call_id"`
}

type functionCallOutputPayload struct {
	CallID string `json:"call_id"`
	Output string `json:"output"`
}

type customToolCallPayload struct {
	Name   string `json:"name"`
	Input  string `json:"input"`
	CallID string `json:"call_id"`
	Status string `json:"status"`
}

type customToolCallOutputPayload struct {
	CallID string `json:"call_id"`
	Output string `json:"output"`
}

type webSearchCallPayload struct {
	Status string                 `json:"status"`
	Action map[string]interface{} `json:"action"`
}

func (p *parser) handleResponseItem(ts time.Time, raw json.RawMessage, lineNum int) {
	var pt payloadType
	if err := json.Unmarshal(raw, &pt); err != nil {
		p.out.ValidationErrors = append(p.out.ValidationErrors, ValidationError{
			Line: lineNum, Type: "response_item", Reason: "payload.type decode: " + err.Error(),
		})
		return
	}

	switch pt.Type {
	case "message":
		var msg responseMessagePayload
		if err := json.Unmarshal(raw, &msg); err != nil {
			return
		}
		p.handleMessage(ts, msg)
	case "function_call":
		var fc functionCallPayload
		if err := json.Unmarshal(raw, &fc); err != nil {
			return
		}
		p.ensureTurn(ts)
		p.openToolCall(ts, fc.CallID, fc.Name, fc.Arguments)
	case "function_call_output":
		var out functionCallOutputPayload
		if err := json.Unmarshal(raw, &out); err != nil {
			return
		}
		p.closeToolCallOutput(ts, out.CallID, out.Output)
	case "custom_tool_call":
		var ct customToolCallPayload
		if err := json.Unmarshal(raw, &ct); err != nil {
			return
		}
		p.ensureTurn(ts)
		p.openToolCall(ts, ct.CallID, ct.Name, ct.Input)
		// Some custom tool calls (e.g. apply_patch) report a terminal status
		// inline rather than via a later *_output event. Other statuses fall
		// through to "pending" for a subsequent *_output to resolve. CF-438.
		if ct.Status == "completed" || ct.Status == "failed" {
			if ref, ok := p.callIndex[ct.CallID]; ok {
				p.toolCallAt(ref).Status = ct.Status
			}
		}
	case "custom_tool_call_output":
		var out customToolCallOutputPayload
		if err := json.Unmarshal(raw, &out); err != nil {
			return
		}
		p.closeToolCallOutput(ts, out.CallID, out.Output)
	case "reasoning":
		p.ensureTurn(ts)
		// Encrypted-content reasoning has no displayable text; we only count it.
		// Same applies for non-encrypted reasoning (content may carry summary).
		p.active.ReasoningCount++
	case "web_search_call":
		var ws webSearchCallPayload
		if err := json.Unmarshal(raw, &ws); err != nil {
			return
		}
		p.ensureTurn(ts)
		// Treat web_search_call as a tool call so it counts in analytics.
		args, _ := json.Marshal(ws.Action)
		status := "completed"
		if ws.Status == "failed" {
			status = "failed"
		}
		p.active.ToolCalls = append(p.active.ToolCalls, ToolCall{
			Name:      "web_search_call",
			Arguments: string(args),
			Status:    status,
			Timestamp: ts,
		})
	default:
		// Unknown response_item type — silently skip for forward-compat.
	}
}

// handleMessage processes a response_item.message. Developer-role messages
// are dropped. User messages have <environment_context> stripped; if nothing
// remains they're dropped entirely. Assistant messages preserve their phase.
func (p *parser) handleMessage(ts time.Time, msg responseMessagePayload) {
	if msg.Role == "developer" {
		return
	}
	text := joinContentText(msg.Content)

	switch msg.Role {
	case "user":
		text = strings.TrimSpace(envContextPattern.ReplaceAllString(text, ""))
		if text == "" {
			return
		}
		p.ensureTurn(ts)
		p.active.UserMessages = append(p.active.UserMessages, Message{
			Role: "user", Text: text, Timestamp: ts,
		})
	case "assistant":
		p.ensureTurn(ts)
		phase := msg.Phase
		if phase == "" {
			phase = "final"
		}
		p.active.AssistantMessages = append(p.active.AssistantMessages, Message{
			Role: "assistant", Text: text, Phase: phase, Timestamp: ts,
		})
	}
}

// joinContentText concatenates input_text and output_text blocks with "\n".
// Other block types contribute no text.
func joinContentText(blocks []responseContentBlock) string {
	parts := make([]string, 0, len(blocks))
	for _, b := range blocks {
		if (b.Type == "input_text" || b.Type == "output_text") && b.Text != "" {
			parts = append(parts, b.Text)
		}
	}
	return strings.Join(parts, "\n")
}

// ----------------------------------------------------------------------------
// Tool-call pairing
// ----------------------------------------------------------------------------

// openToolCall appends a new ToolCall to the active turn and indexes it by call_id.
func (p *parser) openToolCall(ts time.Time, callID, name, args string) {
	if p.callIndex == nil {
		p.callIndex = make(map[string]callRef)
	}
	tc := ToolCall{
		CallID:    callID,
		Name:      name,
		Arguments: args,
		Status:    "pending",
		Timestamp: ts,
	}
	p.active.ToolCalls = append(p.active.ToolCalls, tc)
	p.callIndex[callID] = callRef{
		turnIdx: len(p.out.Turns), // active turn is index len(Turns) (not yet appended)
		toolIdx: len(p.active.ToolCalls) - 1,
	}
}

// closeToolCallOutput attaches an output to a previously-opened ToolCall.
// On orphan output (no matching call_id), emits a synthetic "<unknown>"
// ToolCall so transcript/search still surface the text, and appends a
// ValidationError so callers can discover the anomaly. The analytics Tools
// card drops "<unknown>" entries by design (CF-438).
func (p *parser) closeToolCallOutput(ts time.Time, callID, output string) {
	if ref, ok := p.callIndex[callID]; ok {
		tc := p.toolCallAt(ref)
		applyToolOutput(tc, output)
		return
	}
	p.ensureTurn(ts)
	tc := ToolCall{
		CallID:    callID,
		Name:      "<unknown>",
		Timestamp: ts,
	}
	applyToolOutput(&tc, output)
	p.active.ToolCalls = append(p.active.ToolCalls, tc)
	p.out.ValidationErrors = append(p.out.ValidationErrors, ValidationError{
		Type:   "function_call_output",
		Reason: "orphan output: no matching call_id " + callID,
	})
}

// toolCallAt returns a pointer into Turns[turnIdx].ToolCalls or the active
// turn's ToolCalls if turnIdx == len(Turns). Output may arrive after the
// originating turn has been closed.
func (p *parser) toolCallAt(ref callRef) *ToolCall {
	if ref.turnIdx < len(p.out.Turns) {
		return &p.out.Turns[ref.turnIdx].ToolCalls[ref.toolIdx]
	}
	return &p.active.ToolCalls[ref.toolIdx]
}

// applyToolOutput populates Output/Status (and exec_command preamble fields).
func applyToolOutput(tc *ToolCall, raw string) {
	if tc.Name == "exec_command" {
		body, exit, wallMs := parseExecOutput(raw)
		tc.Output = body
		tc.ExitCode = &exit
		tc.WallTimeMs = &wallMs
		if exit == 0 {
			tc.Status = "completed"
		} else {
			tc.Status = "failed"
		}
		return
	}
	tc.Output = raw
	if tc.Status == "pending" {
		tc.Status = "completed"
	}
}

// parseExecOutput strips the Codex exec_command preamble. Mirrors the
// frontend's parseExecOutput so the displayed/stored body matches the UI.
func parseExecOutput(raw string) (body string, exitCode int, wallTimeMs int) {
	// Sentinel at start of string: no preamble (rare); everything is the body.
	if strings.HasPrefix(raw, execOutputSentinel) {
		return strings.TrimSuffix(raw[len(execOutputSentinel):], "\n"), 0, 0
	}
	// Sentinel must sit on its own line: look for "\n" + sentinel.
	idx := strings.Index(raw, "\n"+execOutputSentinel)
	if idx == -1 {
		// Sentinel missing entirely — treat the whole string as body.
		return strings.TrimSuffix(raw, "\n"), 0, 0
	}
	preamble := raw[:idx+1] // include the trailing newline
	body = strings.TrimSuffix(raw[idx+1+len(execOutputSentinel):], "\n")

	if m := execExitPattern.FindStringSubmatch(preamble); len(m) == 2 {
		if n, err := strconv.Atoi(m[1]); err == nil {
			exitCode = n
		}
	}
	if m := execWallPattern.FindStringSubmatch(preamble); len(m) == 2 {
		if f, err := strconv.ParseFloat(m[1], 64); err == nil {
			wallTimeMs = int(f * 1000)
		}
	}
	return body, exitCode, wallTimeMs
}

// ----------------------------------------------------------------------------
// event_msg
// ----------------------------------------------------------------------------

type eventTaskStartedPayload struct {
	TurnID    string  `json:"turn_id"`
	StartedAt float64 `json:"started_at"` // unix seconds
	Model     string  `json:"model"`
}

type eventTaskCompletePayload struct {
	TurnID             string  `json:"turn_id"`
	CompletedAt        float64 `json:"completed_at"`
	DurationMs         int64   `json:"duration_ms"`
	TimeToFirstTokenMs int64   `json:"time_to_first_token_ms"`
}

type eventTokenCountPayload struct {
	Info *tokenCountInfo `json:"info"`
}

type tokenCountInfo struct {
	TotalTokenUsage *tokenCountTotals `json:"total_token_usage"`
}

type tokenCountTotals struct {
	InputTokens           int64 `json:"input_tokens"`
	CachedInputTokens     int64 `json:"cached_input_tokens"`
	OutputTokens          int64 `json:"output_tokens"`
	ReasoningOutputTokens int64 `json:"reasoning_output_tokens"`
	TotalTokens           int64 `json:"total_tokens"`
}

type eventPatchApplyEndPayload struct {
	CallID  string `json:"call_id"`
	Success *bool  `json:"success"`
}

func (p *parser) handleEventMsg(ts time.Time, raw json.RawMessage, lineNum int) {
	var pt payloadType
	if err := json.Unmarshal(raw, &pt); err != nil {
		p.out.ValidationErrors = append(p.out.ValidationErrors, ValidationError{
			Line: lineNum, Type: "event_msg", Reason: "payload.type decode: " + err.Error(),
		})
		return
	}

	switch pt.Type {
	case "task_started":
		var ts0 eventTaskStartedPayload
		if err := json.Unmarshal(raw, &ts0); err != nil {
			return
		}
		p.closeOpenTurn()
		started := unixSecondsToTime(ts0.StartedAt)
		p.active = &Turn{
			TurnID:    ts0.TurnID,
			StartedAt: started,
			Model:     ts0.Model,
		}
	case "task_complete":
		var tc eventTaskCompletePayload
		if err := json.Unmarshal(raw, &tc); err != nil {
			return
		}
		if p.active == nil {
			// task_complete with no preceding task_started — implicitly open
			// a turn so we still capture timing.
			p.active = &Turn{TurnID: tc.TurnID}
		}
		if completed := unixSecondsToTime(tc.CompletedAt); completed != nil {
			p.active.CompletedAt = completed
		}
		if tc.DurationMs > 0 {
			d := tc.DurationMs
			p.active.DurationMs = &d
		}
		if tc.TimeToFirstTokenMs > 0 {
			t := tc.TimeToFirstTokenMs
			p.active.TimeToFirstTokenMs = &t
		}
		p.closeOpenTurn()
	case "token_count":
		var tcnt eventTokenCountPayload
		if err := json.Unmarshal(raw, &tcnt); err != nil {
			return
		}
		if tcnt.Info != nil && tcnt.Info.TotalTokenUsage != nil {
			tt := tcnt.Info.TotalTokenUsage
			p.out.TokenUsage = TokenUsage{
				InputTokens:           tt.InputTokens,
				CachedInputTokens:     tt.CachedInputTokens,
				OutputTokens:          tt.OutputTokens,
				ReasoningOutputTokens: tt.ReasoningOutputTokens,
				TotalTokens:           tt.TotalTokens,
			}
		}
	case "patch_apply_end":
		var pae eventPatchApplyEndPayload
		if err := json.Unmarshal(raw, &pae); err != nil {
			return
		}
		if pae.CallID == "" || pae.Success == nil || *pae.Success {
			return
		}
		if ref, ok := p.callIndex[pae.CallID]; ok {
			p.toolCallAt(ref).Status = "failed"
		}
	default:
		// user_message, agent_message, and other event types are intentionally
		// dropped (redundant with response_item, or pure UI metadata).
	}
}

// ----------------------------------------------------------------------------
// compacted
// ----------------------------------------------------------------------------

type compactedPayload struct {
	ReplacementHistory []json.RawMessage `json:"replacement_history"`
}

func (p *parser) handleCompacted(ts time.Time, raw json.RawMessage) {
	var c compactedPayload
	_ = json.Unmarshal(raw, &c)
	p.out.Compactions = append(p.out.Compactions, CompactionEvent{
		Timestamp:        ts,
		ReplacementCount: len(c.ReplacementHistory),
	})
}

// ----------------------------------------------------------------------------
// Turn lifecycle helpers
// ----------------------------------------------------------------------------

// ensureTurn opens an implicit turn if none is active. Called whenever a
// response_item arrives before the first task_started (or after the previous
// turn closed but before the next task_started fires).
func (p *parser) ensureTurn(ts time.Time) {
	if p.active != nil {
		return
	}
	p.active = &Turn{}
	if !ts.IsZero() {
		t := ts
		p.active.StartedAt = &t
	}
	// Inherit session-level model when the implicit turn has no task_started.
	if p.out.Model != "" {
		p.active.Model = p.out.Model
	}
}

// closeOpenTurn finalizes the active turn and appends it to out.Turns.
// Idempotent — safe to call when no turn is open.
func (p *parser) closeOpenTurn() {
	if p.active == nil {
		return
	}
	// Fall back to session-meta model if task_started didn't carry one.
	if p.active.Model == "" && p.out.Model != "" {
		p.active.Model = p.out.Model
	}
	p.out.Turns = append(p.out.Turns, *p.active)
	p.active = nil
}

// unixSecondsToTime converts a Codex unix-seconds float into a UTC time
// pointer. Returns nil for zero/negative values so missing fields show as nil.
func unixSecondsToTime(v float64) *time.Time {
	if v <= 0 {
		return nil
	}
	sec := int64(v)
	nsec := int64((v - float64(sec)) * 1e9)
	t := time.Unix(sec, nsec).UTC()
	return &t
}
