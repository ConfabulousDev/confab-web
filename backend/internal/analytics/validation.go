package analytics

import (
	"fmt"
	"strings"
)

// ValidationError represents a single field validation error.
type ValidationError struct {
	Path     string `json:"path"`               // Field path, e.g., "message.content[0].type"
	Message  string `json:"message"`            // Human-readable error
	Expected string `json:"expected,omitempty"` // Expected type/value
	Received string `json:"received,omitempty"` // Actual value received
}

// LineValidationError represents all validation errors for a single transcript line.
type LineValidationError struct {
	Line        int               `json:"line"`                   // 1-indexed line number
	RawJSON     string            `json:"raw_json"`               // Truncated raw JSON (max 200 chars)
	MessageType string            `json:"message_type,omitempty"` // "user", "assistant", etc.
	Errors      []ValidationError `json:"errors"`
}

// ValidateLine validates a parsed transcript line against the schema.
// Returns a slice of validation errors (empty if valid).
func ValidateLine(raw map[string]interface{}) []ValidationError {
	var errors []ValidationError

	// Get the message type
	msgType, _ := raw["type"].(string)

	switch msgType {
	case "user":
		errors = validateUserMessage(raw)
	case "assistant":
		errors = validateAssistantMessage(raw)
	case "system":
		errors = validateSystemMessage(raw)
	case "file-history-snapshot":
		errors = validateFileHistorySnapshot(raw)
	case "summary":
		errors = validateSummaryMessage(raw)
	case "queue-operation":
		errors = validateQueueOperationMessage(raw)
	case "":
		errors = append(errors, ValidationError{
			Path:     "type",
			Message:  "required field missing",
			Expected: "string",
			Received: "undefined",
		})
	default:
		// Unknown type - allow for forward compatibility but log it
		// We don't return an error here to support new message types
	}

	return errors
}

// validateUserMessage validates a user message.
func validateUserMessage(raw map[string]interface{}) []ValidationError {
	var errors []ValidationError

	// Validate base message fields
	errors = append(errors, validateBaseMessageFields(raw)...)

	// type must be "user"
	if t, _ := raw["type"].(string); t != "user" {
		errors = append(errors, ValidationError{
			Path:     "type",
			Message:  "invalid literal value",
			Expected: `"user"`,
			Received: fmt.Sprintf("%q", t),
		})
	}

	// message is required
	msg, ok := raw["message"].(map[string]interface{})
	if !ok {
		errors = append(errors, ValidationError{
			Path:     "message",
			Message:  "required field missing or invalid type",
			Expected: "object",
			Received: typeOf(raw["message"]),
		})
		return errors
	}

	// message.role must be "user"
	role, _ := msg["role"].(string)
	if role != "user" {
		errors = append(errors, ValidationError{
			Path:     "message.role",
			Message:  "invalid literal value",
			Expected: `"user"`,
			Received: fmt.Sprintf("%q", role),
		})
	}

	// message.content is required (string or ContentBlock[])
	content := msg["content"]
	if content == nil {
		errors = append(errors, ValidationError{
			Path:     "message.content",
			Message:  "required field missing",
			Expected: "string | ContentBlock[]",
			Received: "undefined",
		})
	} else {
		// Validate content - can be string or array of content blocks
		switch c := content.(type) {
		case string:
			// Valid - human message
		case []interface{}:
			// Validate each content block
			for i, block := range c {
				if blockMap, ok := block.(map[string]interface{}); ok {
					blockErrors := validateContentBlock(blockMap)
					for _, e := range blockErrors {
						e.Path = fmt.Sprintf("message.content[%d].%s", i, e.Path)
						errors = append(errors, e)
					}
				} else {
					errors = append(errors, ValidationError{
						Path:     fmt.Sprintf("message.content[%d]", i),
						Message:  "invalid type",
						Expected: "object",
						Received: typeOf(block),
					})
				}
			}
		default:
			errors = append(errors, ValidationError{
				Path:     "message.content",
				Message:  "invalid type",
				Expected: "string | ContentBlock[]",
				Received: typeOf(content),
			})
		}
	}

	return errors
}

// validateAssistantMessage validates an assistant message.
func validateAssistantMessage(raw map[string]interface{}) []ValidationError {
	var errors []ValidationError

	// Validate base message fields
	errors = append(errors, validateBaseMessageFields(raw)...)

	// type must be "assistant"
	if t, _ := raw["type"].(string); t != "assistant" {
		errors = append(errors, ValidationError{
			Path:     "type",
			Message:  "invalid literal value",
			Expected: `"assistant"`,
			Received: fmt.Sprintf("%q", t),
		})
	}

	// message is required
	msg, ok := raw["message"].(map[string]interface{})
	if !ok {
		errors = append(errors, ValidationError{
			Path:     "message",
			Message:  "required field missing or invalid type",
			Expected: "object",
			Received: typeOf(raw["message"]),
		})
		return errors
	}

	// message.model is required
	if _, ok := msg["model"].(string); !ok {
		errors = append(errors, ValidationError{
			Path:     "message.model",
			Message:  "required field missing or invalid type",
			Expected: "string",
			Received: typeOf(msg["model"]),
		})
	}

	// message.id is required
	if _, ok := msg["id"].(string); !ok {
		errors = append(errors, ValidationError{
			Path:     "message.id",
			Message:  "required field missing or invalid type",
			Expected: "string",
			Received: typeOf(msg["id"]),
		})
	}

	// message.type must be "message"
	if mt, _ := msg["type"].(string); mt != "message" {
		errors = append(errors, ValidationError{
			Path:     "message.type",
			Message:  "invalid literal value",
			Expected: `"message"`,
			Received: fmt.Sprintf("%q", mt),
		})
	}

	// message.role must be "assistant"
	if role, _ := msg["role"].(string); role != "assistant" {
		errors = append(errors, ValidationError{
			Path:     "message.role",
			Message:  "invalid literal value",
			Expected: `"assistant"`,
			Received: fmt.Sprintf("%q", role),
		})
	}

	// message.content is required and must be array
	content, ok := msg["content"].([]interface{})
	if !ok {
		errors = append(errors, ValidationError{
			Path:     "message.content",
			Message:  "required field missing or invalid type",
			Expected: "ContentBlock[]",
			Received: typeOf(msg["content"]),
		})
	} else {
		// Validate each content block
		for i, block := range content {
			if blockMap, ok := block.(map[string]interface{}); ok {
				blockErrors := validateContentBlock(blockMap)
				for _, e := range blockErrors {
					e.Path = fmt.Sprintf("message.content[%d].%s", i, e.Path)
					errors = append(errors, e)
				}
			} else {
				errors = append(errors, ValidationError{
					Path:     fmt.Sprintf("message.content[%d]", i),
					Message:  "invalid type",
					Expected: "object",
					Received: typeOf(block),
				})
			}
		}
	}

	// message.stop_reason is required (can be null)
	if _, exists := msg["stop_reason"]; !exists {
		errors = append(errors, ValidationError{
			Path:     "message.stop_reason",
			Message:  "required field missing",
			Expected: "string | null",
			Received: "undefined",
		})
	}

	// message.stop_sequence is required (can be null)
	if _, exists := msg["stop_sequence"]; !exists {
		errors = append(errors, ValidationError{
			Path:     "message.stop_sequence",
			Message:  "required field missing",
			Expected: "string | null",
			Received: "undefined",
		})
	}

	// message.usage is required
	usage, ok := msg["usage"].(map[string]interface{})
	if !ok {
		errors = append(errors, ValidationError{
			Path:     "message.usage",
			Message:  "required field missing or invalid type",
			Expected: "object",
			Received: typeOf(msg["usage"]),
		})
	} else {
		usageErrors := validateTokenUsage(usage)
		for _, e := range usageErrors {
			e.Path = "message.usage." + e.Path
			errors = append(errors, e)
		}
	}

	return errors
}

// validateSystemMessage validates a system message.
func validateSystemMessage(raw map[string]interface{}) []ValidationError {
	var errors []ValidationError

	// Validate base message fields
	errors = append(errors, validateBaseMessageFields(raw)...)

	// type must be "system"
	if t, _ := raw["type"].(string); t != "system" {
		errors = append(errors, ValidationError{
			Path:     "type",
			Message:  "invalid literal value",
			Expected: `"system"`,
			Received: fmt.Sprintf("%q", t),
		})
	}

	// subtype is required
	if _, ok := raw["subtype"].(string); !ok {
		errors = append(errors, ValidationError{
			Path:     "subtype",
			Message:  "required field missing or invalid type",
			Expected: "string",
			Received: typeOf(raw["subtype"]),
		})
	}

	// isMeta is required
	if _, ok := raw["isMeta"].(bool); !ok {
		errors = append(errors, ValidationError{
			Path:     "isMeta",
			Message:  "required field missing or invalid type",
			Expected: "boolean",
			Received: typeOf(raw["isMeta"]),
		})
	}

	// content is optional (not present for turn_duration subtype)
	// level is optional (not present for some subtypes)
	// durationMs is optional
	// slug is optional
	// logicalParentUuid is optional
	// compactMetadata is optional but if present, validate it
	if cm, ok := raw["compactMetadata"].(map[string]interface{}); ok {
		// trigger is required within compactMetadata
		if _, ok := cm["trigger"].(string); !ok {
			errors = append(errors, ValidationError{
				Path:     "compactMetadata.trigger",
				Message:  "required field missing or invalid type",
				Expected: "string",
				Received: typeOf(cm["trigger"]),
			})
		}
		// preTokens is required within compactMetadata
		if _, ok := cm["preTokens"].(float64); !ok {
			errors = append(errors, ValidationError{
				Path:     "compactMetadata.preTokens",
				Message:  "required field missing or invalid type",
				Expected: "number",
				Received: typeOf(cm["preTokens"]),
			})
		}
	}

	return errors
}

// validateFileHistorySnapshot validates a file-history-snapshot message.
func validateFileHistorySnapshot(raw map[string]interface{}) []ValidationError {
	var errors []ValidationError

	// type must be "file-history-snapshot"
	if t, _ := raw["type"].(string); t != "file-history-snapshot" {
		errors = append(errors, ValidationError{
			Path:     "type",
			Message:  "invalid literal value",
			Expected: `"file-history-snapshot"`,
			Received: fmt.Sprintf("%q", t),
		})
	}

	// messageId is required
	if _, ok := raw["messageId"].(string); !ok {
		errors = append(errors, ValidationError{
			Path:     "messageId",
			Message:  "required field missing or invalid type",
			Expected: "string",
			Received: typeOf(raw["messageId"]),
		})
	}

	// isSnapshotUpdate is required
	if _, ok := raw["isSnapshotUpdate"].(bool); !ok {
		errors = append(errors, ValidationError{
			Path:     "isSnapshotUpdate",
			Message:  "required field missing or invalid type",
			Expected: "boolean",
			Received: typeOf(raw["isSnapshotUpdate"]),
		})
	}

	// snapshot is required
	snapshot, ok := raw["snapshot"].(map[string]interface{})
	if !ok {
		errors = append(errors, ValidationError{
			Path:     "snapshot",
			Message:  "required field missing or invalid type",
			Expected: "object",
			Received: typeOf(raw["snapshot"]),
		})
	} else {
		// snapshot.messageId is required
		if _, ok := snapshot["messageId"].(string); !ok {
			errors = append(errors, ValidationError{
				Path:     "snapshot.messageId",
				Message:  "required field missing or invalid type",
				Expected: "string",
				Received: typeOf(snapshot["messageId"]),
			})
		}
		// snapshot.timestamp is required
		if _, ok := snapshot["timestamp"].(string); !ok {
			errors = append(errors, ValidationError{
				Path:     "snapshot.timestamp",
				Message:  "required field missing or invalid type",
				Expected: "string",
				Received: typeOf(snapshot["timestamp"]),
			})
		}
		// snapshot.trackedFileBackups is required
		if _, ok := snapshot["trackedFileBackups"].(map[string]interface{}); !ok {
			errors = append(errors, ValidationError{
				Path:     "snapshot.trackedFileBackups",
				Message:  "required field missing or invalid type",
				Expected: "object",
				Received: typeOf(snapshot["trackedFileBackups"]),
			})
		}
	}

	return errors
}

// validateSummaryMessage validates a summary message.
func validateSummaryMessage(raw map[string]interface{}) []ValidationError {
	var errors []ValidationError

	// type must be "summary"
	if t, _ := raw["type"].(string); t != "summary" {
		errors = append(errors, ValidationError{
			Path:     "type",
			Message:  "invalid literal value",
			Expected: `"summary"`,
			Received: fmt.Sprintf("%q", t),
		})
	}

	// summary is required
	if _, ok := raw["summary"].(string); !ok {
		errors = append(errors, ValidationError{
			Path:     "summary",
			Message:  "required field missing or invalid type",
			Expected: "string",
			Received: typeOf(raw["summary"]),
		})
	}

	// leafUuid is required
	if _, ok := raw["leafUuid"].(string); !ok {
		errors = append(errors, ValidationError{
			Path:     "leafUuid",
			Message:  "required field missing or invalid type",
			Expected: "string",
			Received: typeOf(raw["leafUuid"]),
		})
	}

	return errors
}

// validateQueueOperationMessage validates a queue-operation message.
func validateQueueOperationMessage(raw map[string]interface{}) []ValidationError {
	var errors []ValidationError

	// type must be "queue-operation"
	if t, _ := raw["type"].(string); t != "queue-operation" {
		errors = append(errors, ValidationError{
			Path:     "type",
			Message:  "invalid literal value",
			Expected: `"queue-operation"`,
			Received: fmt.Sprintf("%q", t),
		})
	}

	// operation is required
	if _, ok := raw["operation"].(string); !ok {
		errors = append(errors, ValidationError{
			Path:     "operation",
			Message:  "required field missing or invalid type",
			Expected: "string",
			Received: typeOf(raw["operation"]),
		})
	}

	// timestamp is required
	if _, ok := raw["timestamp"].(string); !ok {
		errors = append(errors, ValidationError{
			Path:     "timestamp",
			Message:  "required field missing or invalid type",
			Expected: "string",
			Received: typeOf(raw["timestamp"]),
		})
	}

	// sessionId is required
	if _, ok := raw["sessionId"].(string); !ok {
		errors = append(errors, ValidationError{
			Path:     "sessionId",
			Message:  "required field missing or invalid type",
			Expected: "string",
			Received: typeOf(raw["sessionId"]),
		})
	}

	// content is optional

	return errors
}

// validateBaseMessageFields validates fields common to user, assistant, and system messages.
func validateBaseMessageFields(raw map[string]interface{}) []ValidationError {
	var errors []ValidationError

	// uuid is required
	if _, ok := raw["uuid"].(string); !ok {
		errors = append(errors, ValidationError{
			Path:     "uuid",
			Message:  "required field missing or invalid type",
			Expected: "string",
			Received: typeOf(raw["uuid"]),
		})
	}

	// timestamp is required
	if _, ok := raw["timestamp"].(string); !ok {
		errors = append(errors, ValidationError{
			Path:     "timestamp",
			Message:  "required field missing or invalid type",
			Expected: "string",
			Received: typeOf(raw["timestamp"]),
		})
	}

	// parentUuid is required (can be null)
	if _, exists := raw["parentUuid"]; !exists {
		errors = append(errors, ValidationError{
			Path:     "parentUuid",
			Message:  "required field missing",
			Expected: "string | null",
			Received: "undefined",
		})
	}

	// isSidechain is required
	if _, ok := raw["isSidechain"].(bool); !ok {
		errors = append(errors, ValidationError{
			Path:     "isSidechain",
			Message:  "required field missing or invalid type",
			Expected: "boolean",
			Received: typeOf(raw["isSidechain"]),
		})
	}

	// userType is required
	if _, ok := raw["userType"].(string); !ok {
		errors = append(errors, ValidationError{
			Path:     "userType",
			Message:  "required field missing or invalid type",
			Expected: "string",
			Received: typeOf(raw["userType"]),
		})
	}

	// cwd is required
	if _, ok := raw["cwd"].(string); !ok {
		errors = append(errors, ValidationError{
			Path:     "cwd",
			Message:  "required field missing or invalid type",
			Expected: "string",
			Received: typeOf(raw["cwd"]),
		})
	}

	// sessionId is required
	if _, ok := raw["sessionId"].(string); !ok {
		errors = append(errors, ValidationError{
			Path:     "sessionId",
			Message:  "required field missing or invalid type",
			Expected: "string",
			Received: typeOf(raw["sessionId"]),
		})
	}

	// version is required
	if _, ok := raw["version"].(string); !ok {
		errors = append(errors, ValidationError{
			Path:     "version",
			Message:  "required field missing or invalid type",
			Expected: "string",
			Received: typeOf(raw["version"]),
		})
	}

	// gitBranch is optional

	return errors
}

// validateTokenUsage validates a token usage object.
func validateTokenUsage(usage map[string]interface{}) []ValidationError {
	var errors []ValidationError

	// input_tokens is required
	if _, ok := usage["input_tokens"].(float64); !ok {
		errors = append(errors, ValidationError{
			Path:     "input_tokens",
			Message:  "required field missing or invalid type",
			Expected: "number",
			Received: typeOf(usage["input_tokens"]),
		})
	}

	// output_tokens is required
	if _, ok := usage["output_tokens"].(float64); !ok {
		errors = append(errors, ValidationError{
			Path:     "output_tokens",
			Message:  "required field missing or invalid type",
			Expected: "number",
			Received: typeOf(usage["output_tokens"]),
		})
	}

	// cache_creation_input_tokens is optional
	// cache_read_input_tokens is optional
	// cache_creation is optional
	// service_tier is optional (can be null)

	return errors
}

// validateContentBlock validates a single content block.
func validateContentBlock(block map[string]interface{}) []ValidationError {
	blockType, _ := block["type"].(string)

	switch blockType {
	case "text":
		return validateTextBlock(block)
	case "thinking":
		return validateThinkingBlock(block)
	case "tool_use":
		return validateToolUseBlock(block)
	case "tool_result":
		return validateToolResultBlock(block)
	case "image":
		return validateImageBlock(block)
	case "":
		return []ValidationError{{
			Path:     "type",
			Message:  "required field missing",
			Expected: "string",
			Received: "undefined",
		}}
	default:
		// Unknown block type - allow for forward compatibility
		return nil
	}
}

// validateTextBlock validates a text content block.
func validateTextBlock(block map[string]interface{}) []ValidationError {
	var errors []ValidationError

	// text is required
	if _, ok := block["text"].(string); !ok {
		errors = append(errors, ValidationError{
			Path:     "text",
			Message:  "required field missing or invalid type",
			Expected: "string",
			Received: typeOf(block["text"]),
		})
	}

	return errors
}

// validateThinkingBlock validates a thinking content block.
func validateThinkingBlock(block map[string]interface{}) []ValidationError {
	var errors []ValidationError

	// thinking is required
	if _, ok := block["thinking"].(string); !ok {
		errors = append(errors, ValidationError{
			Path:     "thinking",
			Message:  "required field missing or invalid type",
			Expected: "string",
			Received: typeOf(block["thinking"]),
		})
	}

	// signature is optional

	return errors
}

// validateToolUseBlock validates a tool_use content block.
func validateToolUseBlock(block map[string]interface{}) []ValidationError {
	var errors []ValidationError

	// id is required
	if _, ok := block["id"].(string); !ok {
		errors = append(errors, ValidationError{
			Path:     "id",
			Message:  "required field missing or invalid type",
			Expected: "string",
			Received: typeOf(block["id"]),
		})
	}

	// name is required
	if _, ok := block["name"].(string); !ok {
		errors = append(errors, ValidationError{
			Path:     "name",
			Message:  "required field missing or invalid type",
			Expected: "string",
			Received: typeOf(block["name"]),
		})
	}

	// input is required (must be an object)
	if _, ok := block["input"].(map[string]interface{}); !ok {
		errors = append(errors, ValidationError{
			Path:     "input",
			Message:  "required field missing or invalid type",
			Expected: "object",
			Received: typeOf(block["input"]),
		})
	}

	return errors
}

// validateToolResultBlock validates a tool_result content block.
func validateToolResultBlock(block map[string]interface{}) []ValidationError {
	var errors []ValidationError

	// tool_use_id is required
	if _, ok := block["tool_use_id"].(string); !ok {
		errors = append(errors, ValidationError{
			Path:     "tool_use_id",
			Message:  "required field missing or invalid type",
			Expected: "string",
			Received: typeOf(block["tool_use_id"]),
		})
	}

	// content is required (string or ContentBlock[])
	content := block["content"]
	if content == nil {
		errors = append(errors, ValidationError{
			Path:     "content",
			Message:  "required field missing",
			Expected: "string | ContentBlock[]",
			Received: "undefined",
		})
	} else {
		switch c := content.(type) {
		case string:
			// Valid - string content
		case []interface{}:
			// Validate nested content blocks recursively
			for i, nested := range c {
				if nestedMap, ok := nested.(map[string]interface{}); ok {
					nestedErrors := validateContentBlock(nestedMap)
					for _, e := range nestedErrors {
						e.Path = fmt.Sprintf("content[%d].%s", i, e.Path)
						errors = append(errors, e)
					}
				} else {
					errors = append(errors, ValidationError{
						Path:     fmt.Sprintf("content[%d]", i),
						Message:  "invalid type",
						Expected: "object",
						Received: typeOf(nested),
					})
				}
			}
		default:
			errors = append(errors, ValidationError{
				Path:     "content",
				Message:  "invalid type",
				Expected: "string | ContentBlock[]",
				Received: typeOf(content),
			})
		}
	}

	// is_error is optional

	return errors
}

// validateImageBlock validates an image content block.
func validateImageBlock(block map[string]interface{}) []ValidationError {
	var errors []ValidationError

	// source is required
	source, ok := block["source"].(map[string]interface{})
	if !ok {
		errors = append(errors, ValidationError{
			Path:     "source",
			Message:  "required field missing or invalid type",
			Expected: "object",
			Received: typeOf(block["source"]),
		})
		return errors
	}

	// source.type is required
	if _, ok := source["type"].(string); !ok {
		errors = append(errors, ValidationError{
			Path:     "source.type",
			Message:  "required field missing or invalid type",
			Expected: "string",
			Received: typeOf(source["type"]),
		})
	}

	// source.media_type is required
	if _, ok := source["media_type"].(string); !ok {
		errors = append(errors, ValidationError{
			Path:     "source.media_type",
			Message:  "required field missing or invalid type",
			Expected: "string",
			Received: typeOf(source["media_type"]),
		})
	}

	// source.data is optional (for base64)
	// source.url is optional (for url)

	return errors
}

// typeOf returns a string representation of the type for error messages.
func typeOf(v interface{}) string {
	if v == nil {
		return "undefined"
	}
	switch v.(type) {
	case string:
		return "string"
	case float64:
		return "number"
	case bool:
		return "boolean"
	case []interface{}:
		return "array"
	case map[string]interface{}:
		return "object"
	default:
		return fmt.Sprintf("%T", v)
	}
}

// truncateJSON truncates a JSON string for error reporting.
func truncateJSON(json string, maxLen int) string {
	if len(json) <= maxLen {
		return json
	}
	return json[:maxLen-3] + "..."
}

// FormatValidationErrors formats validation errors for logging.
func FormatValidationErrors(errors []LineValidationError, maxErrors int) string {
	if len(errors) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Transcript validation errors (%d total):\n", len(errors)))

	count := len(errors)
	if count > maxErrors {
		count = maxErrors
	}

	for i := 0; i < count; i++ {
		err := errors[i]
		sb.WriteString(fmt.Sprintf("  Line %d (type=%s):\n", err.Line, err.MessageType))
		for _, ve := range err.Errors {
			sb.WriteString(fmt.Sprintf("    - %s: %s", ve.Path, ve.Message))
			if ve.Expected != "" {
				sb.WriteString(fmt.Sprintf(" (expected %s, got %s)", ve.Expected, ve.Received))
			}
			sb.WriteString("\n")
		}
	}

	if len(errors) > maxErrors {
		sb.WriteString(fmt.Sprintf("  ... and %d more errors\n", len(errors)-maxErrors))
	}

	return sb.String()
}
