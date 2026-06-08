package analytics

import "fmt"

// ValidateOpenCodeLine validates a single parsed OpenCode JSONL line against
// the {info, parts} schema. Returns a slice of validation errors (empty if
// valid). Reuses the package-shared ValidationError type so the wire shape
// matches Claude's validator output exactly.
//
// Schema derived from OpenCodeMessage / OpenCodeMessageInfo / OpenCodePart in
// opencode_types.go and verified against real captures
// (/tmp/galois-opencode/opencode.db). Unknown top-level fields and unknown
// part types are allowed (forward-compat with future OpenCode releases),
// matching the policy in validation.go::ValidateLine for Claude.
func ValidateOpenCodeLine(raw map[string]interface{}) []ValidationError {
	var errors []ValidationError

	info, ok := raw["info"].(map[string]interface{})
	if !ok {
		errors = append(errors, ValidationError{
			Path:     "info",
			Message:  "required field missing or invalid type",
			Expected: "object",
			Received: typeOf(raw["info"]),
		})
		// Continue checking parts even if info is bad — surface every issue.
	} else {
		errors = append(errors, validateOpenCodeInfo(info)...)
	}

	parts, ok := raw["parts"].([]interface{})
	if !ok {
		errors = append(errors, ValidationError{
			Path:     "parts",
			Message:  "required field missing or invalid type",
			Expected: "array",
			Received: typeOf(raw["parts"]),
		})
		return errors
	}
	for i, part := range parts {
		partMap, ok := part.(map[string]interface{})
		if !ok {
			errors = append(errors, ValidationError{
				Path:     fmt.Sprintf("parts[%d]", i),
				Message:  "invalid type",
				Expected: "object",
				Received: typeOf(part),
			})
			continue
		}
		for _, e := range validateOpenCodePart(partMap) {
			e.Path = fmt.Sprintf("parts[%d].%s", i, e.Path)
			errors = append(errors, e)
		}
	}
	return errors
}

// validateOpenCodeInfo validates the per-message info object: required
// identity fields, role-specific assistant requirements (modelID, providerID,
// tokens), and the nested time.created timestamp every analyzer depends on.
func validateOpenCodeInfo(info map[string]interface{}) []ValidationError {
	var errors []ValidationError

	for _, field := range []string{"id", "sessionID"} {
		if _, ok := info[field].(string); !ok {
			errors = append(errors, ValidationError{
				Path:     "info." + field,
				Message:  "required field missing or invalid type",
				Expected: "string",
				Received: typeOf(info[field]),
			})
		}
	}

	role, _ := info["role"].(string)
	switch role {
	case "user", "assistant":
		// Valid.
	case "":
		errors = append(errors, ValidationError{
			Path:     "info.role",
			Message:  "required field missing",
			Expected: `"user" | "assistant"`,
			Received: "undefined",
		})
	default:
		errors = append(errors, ValidationError{
			Path:     "info.role",
			Message:  "invalid literal value",
			Expected: `"user" | "assistant"`,
			Received: fmt.Sprintf("%q", role),
		})
	}

	timeObj, ok := info["time"].(map[string]interface{})
	if !ok {
		errors = append(errors, ValidationError{
			Path:     "info.time",
			Message:  "required field missing or invalid type",
			Expected: "object",
			Received: typeOf(info["time"]),
		})
	} else if _, ok := timeObj["created"].(float64); !ok {
		errors = append(errors, ValidationError{
			Path:     "info.time.created",
			Message:  "required field missing or invalid type",
			Expected: "number",
			Received: typeOf(timeObj["created"]),
		})
	}

	if role == "assistant" {
		for _, field := range []string{"modelID", "providerID"} {
			if _, ok := info[field].(string); !ok {
				errors = append(errors, ValidationError{
					Path:     "info." + field,
					Message:  "required field missing or invalid type (assistant)",
					Expected: "string",
					Received: typeOf(info[field]),
				})
			}
		}
		tokens, ok := info["tokens"].(map[string]interface{})
		if !ok {
			errors = append(errors, ValidationError{
				Path:     "info.tokens",
				Message:  "required field missing or invalid type (assistant)",
				Expected: "object",
				Received: typeOf(info["tokens"]),
			})
		} else {
			for _, e := range validateOpenCodeTokens(tokens) {
				e.Path = "info.tokens." + e.Path
				errors = append(errors, e)
			}
		}
	}

	return errors
}

// validateOpenCodeTokens checks the assistant message's tokens object:
// input/output are required numbers; reasoning + cache.{read,write} are
// optional but type-checked when present.
func validateOpenCodeTokens(tokens map[string]interface{}) []ValidationError {
	var errors []ValidationError
	for _, field := range []string{"input", "output"} {
		if _, ok := tokens[field].(float64); !ok {
			errors = append(errors, ValidationError{
				Path:     field,
				Message:  "required field missing or invalid type",
				Expected: "number",
				Received: typeOf(tokens[field]),
			})
		}
	}
	if v, ok := tokens["reasoning"]; ok {
		if _, ok := v.(float64); !ok {
			errors = append(errors, ValidationError{
				Path:     "reasoning",
				Message:  "invalid type",
				Expected: "number",
				Received: typeOf(v),
			})
		}
	}
	if cache, ok := tokens["cache"].(map[string]interface{}); ok {
		for _, field := range []string{"read", "write"} {
			if v, ok := cache[field]; ok {
				if _, ok := v.(float64); !ok {
					errors = append(errors, ValidationError{
						Path:     "cache." + field,
						Message:  "invalid type",
						Expected: "number",
						Received: typeOf(v),
					})
				}
			}
		}
	}
	return errors
}

// validateOpenCodePart dispatches by `type`, calling the per-type validator.
// Unknown types are accepted without error (forward-compat with future
// OpenCode releases), matching Claude's policy in validation.go.
func validateOpenCodePart(part map[string]interface{}) []ValidationError {
	partType, _ := part["type"].(string)
	if partType == "" {
		return []ValidationError{{
			Path:     "type",
			Message:  "required field missing",
			Expected: "string",
			Received: "undefined",
		}}
	}
	switch partType {
	case "text", "reasoning":
		// `text` part requires non-empty text content; `reasoning` is
		// occasionally emitted with no text (observed in real captures), so
		// type-check only.
		if v, ok := part["text"]; ok {
			if _, ok := v.(string); !ok {
				return []ValidationError{{
					Path:     "text",
					Message:  "invalid type",
					Expected: "string",
					Received: typeOf(v),
				}}
			}
		} else if partType == "text" {
			return []ValidationError{{
				Path:     "text",
				Message:  "required field missing",
				Expected: "string",
				Received: "undefined",
			}}
		}
	case "tool":
		if _, ok := part["tool"].(string); !ok {
			return []ValidationError{{
				Path:     "tool",
				Message:  "required field missing or invalid type",
				Expected: "string",
				Received: typeOf(part["tool"]),
			}}
		}
		if state, ok := part["state"].(map[string]interface{}); ok {
			return validateOpenCodeToolState(state)
		}
	case "subtask":
		if _, ok := part["name"].(string); !ok {
			return []ValidationError{{
				Path:     "name",
				Message:  "required field missing or invalid type",
				Expected: "string",
				Received: typeOf(part["name"]),
			}}
		}
	case "compaction":
		if _, ok := part["auto"].(bool); !ok {
			return []ValidationError{{
				Path:     "auto",
				Message:  "required field missing or invalid type",
				Expected: "boolean",
				Received: typeOf(part["auto"]),
			}}
		}
	default:
		// Forward-compat: unknown part types pass.
	}
	return nil
}

// validateOpenCodeToolState validates the optional tool state sub-object:
// when present, status is required and must be one of the observed states.
func validateOpenCodeToolState(state map[string]interface{}) []ValidationError {
	status, _ := state["status"].(string)
	switch status {
	case "pending", "running", "completed", "error":
		return nil
	case "":
		return []ValidationError{{
			Path:     "state.status",
			Message:  "required field missing",
			Expected: `"pending" | "running" | "completed" | "error"`,
			Received: "undefined",
		}}
	default:
		return []ValidationError{{
			Path:     "state.status",
			Message:  "invalid literal value",
			Expected: `"pending" | "running" | "completed" | "error"`,
			Received: fmt.Sprintf("%q", status),
		}}
	}
}
