package analytics

import "encoding/json"

// makeBaseFields returns the common fields needed for all message types.
func makeBaseFields(uuid, timestamp string) map[string]any {
	return map[string]any{
		"uuid":        uuid,
		"timestamp":   timestamp,
		"parentUuid":  nil,
		"isSidechain": false,
		"userType":    "external",
		"cwd":         "/test",
		"sessionId":   "test-session",
		"version":     "1.0.0",
	}
}

// makeUserMessage creates a valid user message JSON.
func makeUserMessage(uuid, timestamp, content string) string {
	m := makeBaseFields(uuid, timestamp)
	m["type"] = "user"
	m["message"] = map[string]any{
		"role":    "user",
		"content": content,
	}
	b, _ := json.Marshal(m)
	return string(b)
}

// makeUserMessageWithToolResults creates a valid user message with tool results.
func makeUserMessageWithToolResults(uuid, timestamp string, toolResults []map[string]any) string {
	m := makeBaseFields(uuid, timestamp)
	m["type"] = "user"
	m["message"] = map[string]any{
		"role":    "user",
		"content": toolResults,
	}
	b, _ := json.Marshal(m)
	return string(b)
}

// makeAssistantMessage creates a valid assistant message JSON.
func makeAssistantMessage(uuid, timestamp, model string, inputTokens, outputTokens int64, content []map[string]any) string {
	m := makeBaseFields(uuid, timestamp)
	m["type"] = "assistant"
	m["message"] = map[string]any{
		"model":         model,
		"id":            "msg-" + uuid,
		"type":          "message",
		"role":          "assistant",
		"content":       content,
		"stop_reason":   "end_turn",
		"stop_sequence": nil,
		"usage": map[string]any{
			"input_tokens":  float64(inputTokens),
			"output_tokens": float64(outputTokens),
		},
	}
	b, _ := json.Marshal(m)
	return string(b)
}

// makeAssistantMessageFull creates a valid assistant message with all token fields.
func makeAssistantMessageFull(uuid, timestamp, model string, inputTokens, outputTokens, cacheCreation, cacheRead int64, content []map[string]any) string {
	m := makeBaseFields(uuid, timestamp)
	m["type"] = "assistant"
	m["message"] = map[string]any{
		"model":         model,
		"id":            "msg-" + uuid,
		"type":          "message",
		"role":          "assistant",
		"content":       content,
		"stop_reason":   "end_turn",
		"stop_sequence": nil,
		"usage": map[string]any{
			"input_tokens":                float64(inputTokens),
			"output_tokens":               float64(outputTokens),
			"cache_creation_input_tokens": float64(cacheCreation),
			"cache_read_input_tokens":     float64(cacheRead),
		},
	}
	b, _ := json.Marshal(m)
	return string(b)
}

// makeCompactBoundaryMessage creates a valid compact_boundary system message.
func makeCompactBoundaryMessage(uuid, timestamp, trigger string, preTokens int64) string {
	m := makeBaseFields(uuid, timestamp)
	m["type"] = "system"
	m["subtype"] = "compact_boundary"
	m["isMeta"] = true
	m["compactMetadata"] = map[string]any{
		"trigger":   trigger,
		"preTokens": float64(preTokens),
	}
	b, _ := json.Marshal(m)
	return string(b)
}

// makeCompactBoundaryMessageWithParent creates a compact_boundary with logicalParentUuid.
func makeCompactBoundaryMessageWithParent(uuid, timestamp, trigger string, preTokens int64, logicalParentUuid string) string {
	m := makeBaseFields(uuid, timestamp)
	m["type"] = "system"
	m["subtype"] = "compact_boundary"
	m["isMeta"] = true
	m["logicalParentUuid"] = logicalParentUuid
	m["compactMetadata"] = map[string]any{
		"trigger":   trigger,
		"preTokens": float64(preTokens),
	}
	b, _ := json.Marshal(m)
	return string(b)
}

// makeTextBlock creates a text content block.
func makeTextBlock(text string) map[string]any {
	return map[string]any{
		"type": "text",
		"text": text,
	}
}

// makeThinkingBlock creates a thinking content block.
func makeThinkingBlock(thinking string) map[string]any {
	return map[string]any{
		"type":     "thinking",
		"thinking": thinking,
	}
}

// makeToolUseBlock creates a tool_use content block.
func makeToolUseBlock(id, name string, input map[string]any) map[string]any {
	return map[string]any{
		"type":  "tool_use",
		"id":    id,
		"name":  name,
		"input": input,
	}
}

// makeToolResultBlock creates a tool_result content block.
func makeToolResultBlock(toolUseID, content string, isError bool) map[string]any {
	return map[string]any{
		"type":        "tool_result",
		"tool_use_id": toolUseID,
		"content":     content,
		"is_error":    isError,
	}
}

// makeUserMessageWithToolUseResult creates a user message with tool results and a top-level toolUseResult.
// Used for agent task results.
func makeUserMessageWithToolUseResult(uuid, timestamp string, toolResults []map[string]any, toolUseResult map[string]any) string {
	m := makeBaseFields(uuid, timestamp)
	m["type"] = "user"
	m["message"] = map[string]any{
		"role":    "user",
		"content": toolResults,
	}
	m["toolUseResult"] = toolUseResult
	b, _ := json.Marshal(m)
	return string(b)
}

// makeAssistantMessageWithMsgID creates an assistant message with a specific API message ID.
// Used to test deduplication of multi-line-per-response and context replay.
func makeAssistantMessageWithMsgID(uuid, timestamp, model, msgID string, inputTokens, outputTokens int64, content []map[string]any) string {
	return makeAssistantMessageWithMsgIDAndSpeed(uuid, timestamp, model, msgID, inputTokens, outputTokens, content, "")
}

// makeAssistantMessageWithMsgIDAndSpeed creates an assistant message with a specific API message ID and speed setting.
func makeAssistantMessageWithMsgIDAndSpeed(uuid, timestamp, model, msgID string, inputTokens, outputTokens int64, content []map[string]any, speed string) string {
	m := makeBaseFields(uuid, timestamp)
	m["type"] = "assistant"
	usage := map[string]any{
		"input_tokens":  float64(inputTokens),
		"output_tokens": float64(outputTokens),
	}
	if speed != "" {
		usage["speed"] = speed
	}
	m["message"] = map[string]any{
		"model":         model,
		"id":            msgID,
		"type":          "message",
		"role":          "assistant",
		"content":       content,
		"stop_reason":   "end_turn",
		"stop_sequence": nil,
		"usage":         usage,
	}
	b, _ := json.Marshal(m)
	return string(b)
}

// makeAssistantMessageWithStopReason creates an assistant message with stop_reason and stop_sequence.
func makeAssistantMessageWithStopReason(uuid, timestamp, model string, inputTokens, outputTokens int64, content []map[string]any, stopReason string) string {
	m := makeBaseFields(uuid, timestamp)
	m["type"] = "assistant"
	m["message"] = map[string]any{
		"model":         model,
		"id":            "msg-" + uuid,
		"type":          "message",
		"role":          "assistant",
		"content":       content,
		"stop_reason":   stopReason,
		"stop_sequence": nil,
		"usage": map[string]any{
			"input_tokens":  float64(inputTokens),
			"output_tokens": float64(outputTokens),
		},
	}
	b, _ := json.Marshal(m)
	return string(b)
}
