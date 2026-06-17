package analytics

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
)

// parseCursorJSONL parses a Cursor agent-transcript into typed conversation
// messages, tolerating turn_ended marker rows (success or error) rather than
// treating them as parse failures. It returns the conversation messages in
// file order plus per-line validation errors (using the package-shared
// LineValidationError wire shape, matching Claude/OpenCode).
//
// Policy mirrors OpenCode's "validate but don't drop": a conversation row that
// fails schema validation still contributes its typed content to compute; only
// rows that fail JSON unmarshal entirely are dropped (with a recorded error).
// turn_ended markers never become messages; error markers are surfaced as a
// LineValidationError so operators can see a turn ended abnormally, but they do
// not fail the parse.
func parseCursorJSONL(ctx context.Context, raw []byte, fileName string) ([]*CursorMessage, []LineValidationError) {
	var messages []*CursorMessage
	var lineErrors []LineValidationError
	lineNum := 0

	for _, line := range bytes.Split(raw, []byte("\n")) {
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		lineNum++

		var rawLine cursorRawLine
		if err := json.Unmarshal(line, &rawLine); err != nil {
			lineErrors = append(lineErrors, LineValidationError{
				Line:    lineNum,
				RawJSON: truncateJSON(string(line), 200),
				Errors: []ValidationError{{
					Path:    "",
					Message: fmt.Sprintf("json unmarshal failed: %v", err),
				}},
			})
			continue
		}

		// turn_ended marker rows are not conversation messages. Surface error
		// turns as validation entries (don't drop silently), but keep parsing.
		if rawLine.Type == "turn_ended" {
			if rawLine.Status == "error" {
				lineErrors = append(lineErrors, LineValidationError{
					Line:        lineNum,
					RawJSON:     truncateJSON(string(line), 200),
					MessageType: "turn_ended",
					Errors: []ValidationError{{
						Path:    "error",
						Message: rawLine.Error,
					}},
				})
			}
			continue
		}

		if schemaErrors := ValidateCursorLine(line); len(schemaErrors) > 0 {
			lineErrors = append(lineErrors, LineValidationError{
				Line:        lineNum,
				RawJSON:     truncateJSON(string(line), 200),
				MessageType: rawLine.Role,
				Errors:      schemaErrors,
			})
		}

		if rawLine.Role == "" || rawLine.Message == nil {
			// Not a conversation row and not a recognized marker — already
			// recorded by ValidateCursorLine above; skip materializing.
			continue
		}

		messages = append(messages, &CursorMessage{
			Role:    rawLine.Role,
			Content: rawLine.Message.Content,
		})
	}

	if len(lineErrors) > 0 {
		slog.WarnContext(ctx, "cursor transcript had validation errors",
			"file", fileName, "errors", len(lineErrors), "parsed", len(messages))
	}
	return messages, lineErrors
}

// ValidateCursorLine validates a single raw Cursor JSONL line. Conversation
// rows require role ∈ {user, assistant} and a message.content array of blocks;
// each block requires a recognized type with its mandatory fields. turn_ended
// markers require a status. Unknown tool names and unknown block types are
// accepted (forward-compat), matching Claude's validator policy.
func ValidateCursorLine(raw json.RawMessage) []ValidationError {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return []ValidationError{{Path: "", Message: "line is not a JSON object"}}
	}

	// turn_ended marker rows.
	if _, isMarker := obj["type"]; isMarker {
		var typed struct {
			Type   string `json:"type"`
			Status string `json:"status"`
		}
		_ = json.Unmarshal(raw, &typed)
		if typed.Type == "turn_ended" {
			switch typed.Status {
			case "success", "error":
				return nil
			default:
				return []ValidationError{{
					Path:     "status",
					Message:  "invalid or missing turn_ended status",
					Expected: `"success" | "error"`,
					Received: fmt.Sprintf("%q", typed.Status),
				}}
			}
		}
		// Unknown marker type — forward-compat, accept.
		return nil
	}

	var line cursorRawLine
	if err := json.Unmarshal(raw, &line); err != nil {
		return []ValidationError{{Path: "", Message: fmt.Sprintf("typed unmarshal failed: %v", err)}}
	}

	var errors []ValidationError
	switch line.Role {
	case "user", "assistant":
		// Valid.
	case "":
		errors = append(errors, ValidationError{
			Path:     "role",
			Message:  "required field missing",
			Expected: `"user" | "assistant"`,
			Received: "undefined",
		})
	default:
		errors = append(errors, ValidationError{
			Path:     "role",
			Message:  "invalid literal value",
			Expected: `"user" | "assistant"`,
			Received: fmt.Sprintf("%q", line.Role),
		})
	}

	if line.Message == nil {
		errors = append(errors, ValidationError{
			Path:     "message",
			Message:  "required field missing or invalid type",
			Expected: "object",
		})
		return errors
	}

	for i, b := range line.Message.Content {
		switch b.Type {
		case "text":
			// text content may be empty in practice; type already enforced by
			// the typed decode (string), so no further check.
		case "tool_use":
			if b.Name == "" {
				errors = append(errors, ValidationError{
					Path:    fmt.Sprintf("message.content[%d].name", i),
					Message: "tool_use block missing name",
				})
			}
		case "":
			errors = append(errors, ValidationError{
				Path:    fmt.Sprintf("message.content[%d].type", i),
				Message: "content block missing type",
			})
		default:
			// Forward-compat: unknown block types pass.
		}
	}
	return errors
}
