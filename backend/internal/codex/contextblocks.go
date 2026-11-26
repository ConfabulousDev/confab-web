package codex

import "strings"

// isContextBlock reports whether the text of one user-role input_text content
// block is context Codex injected into the model history (AGENTS.md,
// <environment_context>, <skill>, <turn_aborted>, …) rather than something the
// human typed (z63s).
//
// It ports CONTEXTUAL_USER_FRAGMENT_MATCHERS from openai/codex @ rust-v0.172.0
// (codex-rs/core/src/context/contextual_user_message.rs) and its generic
// matches_marked_text (codex-rs/context-fragments/src/fragment.rs).
// Classification is per content block, never on joined text.
//
// Kept equivalent to isCodexContextBlock in
// frontend/src/services/codexContextBlocks.ts; both table-test against the
// repo-root testdata/codex_contextual_user_blocks.json. Change all three
// together.
func isContextBlock(text string) bool {
	for _, matches := range contextBlockMatchers {
		if matches(text) {
			return true
		}
	}
	return false
}

// contextBlockMatchers is in upstream CONTEXTUAL_USER_FRAGMENT_MATCHERS order.
var contextBlockMatchers = []func(string) bool{
	marked("# AGENTS.md instructions", "</INSTRUCTIONS>"),     // UserInstructions
	marked("<environment_context>", "</environment_context>"), // EnvironmentsState
	isAdditionalContextBlock,
	marked("<agent_message_board_notification>", "</agent_message_board_notification>"),
	isSkillBlock,
	marked("<user_shell_command>", "</user_shell_command>"),
	marked("<turn_aborted>", "</turn_aborted>"),
	marked("<subagent_notification>", "</subagent_notification>"),
	isInternalModelContextBlock,
	marked("<recommended_plugins>", "</recommended_plugins>"),
	func(text string) bool {
		return strings.HasPrefix(strings.TrimSpace(text),
			"Warning: The maximum number of unified exec processes you can keep open is")
	},
	func(text string) bool {
		trimmed := strings.TrimSpace(text)
		return strings.HasPrefix(trimmed, "Warning: apply_patch was requested via ") &&
			strings.HasSuffix(trimmed, "Use the apply_patch tool instead of exec_command.")
	},
	func(text string) bool {
		return strings.HasPrefix(strings.TrimSpace(text),
			"Warning: Your account was flagged for potentially high-risk cyber activity")
	},
}

// isSkillBlock is codex_skills_extension::is_skill_prompt_fragment.
var isSkillBlock = marked("<skill>", "</skill>")

func marked(start, end string) func(string) bool {
	return func(text string) bool { return matchesMarkedText(start, end, text) }
}

// matchesMarkedText reports whether the trimmed text starts with start and
// ends with end, ASCII case-insensitively. strings.TrimSpace strips the same
// Unicode White_Space set as Rust's str::trim.
func matchesMarkedText(start, end, text string) bool {
	trimmed := strings.TrimSpace(text)
	return len(trimmed) >= len(start) && len(trimmed) >= len(end) &&
		equalFoldASCII(trimmed[:len(start)], start) &&
		equalFoldASCII(trimmed[len(trimmed)-len(end):], end)
}

// equalFoldASCII is Rust's eq_ignore_ascii_case: only A-Z fold. Unlike
// strings.EqualFold it does not treat U+212A KELVIN SIGN as "k".
func equalFoldASCII(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		if lowerASCII(a[i]) != lowerASCII(b[i]) {
			return false
		}
	}
	return true
}

func lowerASCII(c byte) byte {
	if 'A' <= c && c <= 'Z' {
		return c + ('a' - 'A')
	}
	return c
}

// isAdditionalContextBlock is AdditionalContextUserFragment::matches_text:
// <external_KEY>…</external_KEY>, case-sensitive.
func isAdditionalContextBlock(text string) bool {
	rest, ok := strings.CutPrefix(strings.TrimSpace(text), "<external_")
	if !ok {
		return false
	}
	key, valueAndClose, ok := strings.Cut(rest, ">")
	return ok && strings.HasSuffix(valueAndClose, "</external_"+key+">")
}

// isInternalModelContextBlock is InternalModelContextFragment::matches_text:
// <codex_internal_context source="[a-z][a-z0-9_]*">…</codex_internal_context>,
// or the legacy <goal_context>…</goal_context>. Case-sensitive.
func isInternalModelContextBlock(text string) bool {
	trimmed := strings.TrimSpace(text)
	if strings.HasPrefix(trimmed, "<goal_context>") && strings.HasSuffix(trimmed, "</goal_context>") {
		return true
	}
	rest, ok := strings.CutPrefix(trimmed, `<codex_internal_context source="`)
	if !ok {
		return false
	}
	source, bodyAndClose, ok := strings.Cut(rest, `">`)
	return ok && isValidInternalContextSource(source) &&
		strings.HasSuffix(bodyAndClose, "</codex_internal_context>")
}

func isValidInternalContextSource(source string) bool {
	if source == "" || source[0] < 'a' || source[0] > 'z' {
		return false
	}
	for i := 1; i < len(source); i++ {
		c := source[i]
		if !('a' <= c && c <= 'z') && !('0' <= c && c <= '9') && c != '_' {
			return false
		}
	}
	return true
}
