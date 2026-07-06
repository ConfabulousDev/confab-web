package analytics

import (
	"context"
	"encoding/json"
	"time"
)

// opencodeRollout holds a parsed OpenCode session: the main thread's messages
// loaded eagerly during Parse, plus the list of subagent agent-file infos kept
// until first traversal then cached on cachedAgents. ValidationErrors
// accumulate per-line schema failures (from loadOpenCodeMessages) and
// whole-file materialize failures (subagent download/parse) using the shared
// LineValidationError type, mirroring Claude's wire shape so a future API
// surface needs no new contract. Single-goroutine per the Rollout contract.
//
// Field naming mirrors codexRollout: `main` is the root rollout's messages,
// agentFileInfo / cachedAgents handle the subagent sidecar files (CF-539).
type opencodeRollout struct {
	main             []*OpenCodeMessage
	agentFileInfo    []opencodeAgentFileInfo
	downloader       opencodeAgentDownloader
	cachedAgents     [][]*OpenCodeMessage
	validationErrors []LineValidationError
	// createdAt is the session's first_seen timestamp, forwarded to
	// computeFromOpenCodeRolloutAt for date-aware pricing (e.g. Sonnet 5 intro rates).
	createdAt        time.Time
}

type opencodeAgentFileInfo struct {
	FileName string
}

type opencodeAgentDownloader func(ctx context.Context, fileName string) ([]byte, error)

type OpenCodeMessage struct {
	Info  OpenCodeMessageInfo `json:"info"`
	Parts []OpenCodePart      `json:"parts"`
}

type OpenCodeMessageInfo struct {
	ID         string          `json:"id"`
	SessionID  string          `json:"sessionID"`
	Role       string          `json:"role"`
	ParentID   string          `json:"parentID,omitempty"`
	ModelID    string          `json:"modelID,omitempty"`
	ProviderID string          `json:"providerID,omitempty"`
	Mode       string          `json:"mode,omitempty"`
	Agent      string          `json:"agent,omitempty"`
	Finish     *string         `json:"finish,omitempty"`
	Cost       float64         `json:"cost"`
	Tokens     OpenCodeTokens  `json:"tokens"`
	Error      json.RawMessage `json:"error,omitempty"`
	Time       OpenCodeTime    `json:"time"`
}

type OpenCodeTokens struct {
	Input     int64         `json:"input"`
	Output    int64         `json:"output"`
	Reasoning int64         `json:"reasoning"`
	Cache     OpenCodeCache `json:"cache"`
}

type OpenCodeCache struct {
	Read  int64 `json:"read"`
	Write int64 `json:"write"`
}

type OpenCodeTime struct {
	Created   int64  `json:"created"`
	Completed *int64 `json:"completed,omitempty"`
}

type OpenCodePart struct {
	ID        string             `json:"id"`
	Type      string             `json:"type"`
	SessionID string             `json:"sessionID,omitempty"`
	MessageID string             `json:"messageID,omitempty"`
	CallID    string             `json:"callID,omitempty"`
	Tool      string             `json:"tool,omitempty"`
	Text      string             `json:"text,omitempty"`
	State     *OpenCodeToolState `json:"state,omitempty"`
	Auto      *bool              `json:"auto,omitempty"`
	Snapshot  string             `json:"snapshot,omitempty"`
	Reason    string             `json:"reason,omitempty"`
	Cost      float64            `json:"cost,omitempty"`
	Tokens    *OpenCodeTokens    `json:"tokens,omitempty"`
	Files     []string           `json:"files,omitempty"`
	Name      string             `json:"name,omitempty"`
	Prompt    string             `json:"prompt,omitempty"`
	Model     json.RawMessage    `json:"model,omitempty"`
}

type OpenCodeToolState struct {
	Status string                 `json:"status"`
	Input  map[string]interface{} `json:"input,omitempty"`
	Output string                 `json:"output,omitempty"`
	Error  string                 `json:"error,omitempty"`
	Title  string                 `json:"title,omitempty"`
}
