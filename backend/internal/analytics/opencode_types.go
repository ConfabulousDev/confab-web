package analytics

import "encoding/json"

type opencodeRollout struct {
	Messages []*OpenCodeMessage
}

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
