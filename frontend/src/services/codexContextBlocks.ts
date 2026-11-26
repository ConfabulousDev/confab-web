/**
 * Recognize the context Codex injects into the model history as user-role
 * `input_text` content blocks (AGENTS.md, `<environment_context>`, `<skill>`,
 * `<turn_aborted>`, …) so the transcript shows only what the human typed (z63s).
 *
 * Port of `CONTEXTUAL_USER_FRAGMENT_MATCHERS` in openai/codex @ rust-v0.172.0
 * (`codex-rs/core/src/context/contextual_user_message.rs`) and its generic
 * `matches_marked_text` (`codex-rs/context-fragments/src/fragment.rs`).
 * Classification is per content block, never on joined text.
 *
 * Kept equivalent to the Go `isContextBlock` in
 * `backend/internal/codex/contextblocks.go`; both table-test against the
 * repo-root `testdata/codex_contextual_user_blocks.json`. Change all three
 * together.
 */

// Rust `str::trim` strips Unicode White_Space, which differs from JS
// `String.prototype.trim` (JS also strips U+FEFF but not U+0085).
const RUST_WHITESPACE = '\\t\\n\\v\\f\\r \\u0085\\u00a0\\u1680\\u2000-\\u200a\\u2028\\u2029\\u202f\\u205f\\u3000';
const LEADING_WS = new RegExp(`^[${RUST_WHITESPACE}]+`);
const TRAILING_WS = new RegExp(`[${RUST_WHITESPACE}]+$`);

function rustTrim(text: string): string {
  return text.replace(LEADING_WS, '').replace(TRAILING_WS, '');
}

/** Rust `eq_ignore_ascii_case`: only A–Z fold, unlike `toLowerCase`. */
function equalsIgnoreAsciiCase(a: string, b: string): boolean {
  if (a.length !== b.length) return false;
  for (let i = 0; i < a.length; i++) {
    let x = a.charCodeAt(i);
    let y = b.charCodeAt(i);
    if (x >= 65 && x <= 90) x += 32;
    if (y >= 65 && y <= 90) y += 32;
    if (x !== y) return false;
  }
  return true;
}

/** `matches_marked_text`: trimmed text starts with `start` and ends with `end`, ASCII case-insensitive. */
function matchesMarkedText(start: string, end: string, text: string): boolean {
  const trimmed = rustTrim(text);
  return (
    equalsIgnoreAsciiCase(trimmed.slice(0, start.length), start) &&
    trimmed.length >= end.length &&
    equalsIgnoreAsciiCase(trimmed.slice(trimmed.length - end.length), end)
  );
}

const marked = (start: string, end: string) => (text: string) => matchesMarkedText(start, end, text);

/** `codex_skills_extension::is_skill_prompt_fragment`. */
const isSkillBlock = marked('<skill>', '</skill>');

/** `AdditionalContextUserFragment`: `<external_KEY>…</external_KEY>`, case-sensitive. */
function isAdditionalContext(text: string): boolean {
  const trimmed = rustTrim(text);
  const prefix = '<external_';
  if (!trimmed.startsWith(prefix)) return false;
  const rest = trimmed.slice(prefix.length);
  const close = rest.indexOf('>');
  if (close < 0) return false;
  const key = rest.slice(0, close);
  return rest.slice(close + 1).endsWith(`</external_${key}>`);
}

const INTERNAL_CONTEXT_PREFIX = '<codex_internal_context source="';
const INTERNAL_CONTEXT_SOURCE = /^[a-z][a-z0-9_]*$/;

/** `InternalModelContextFragment` (plus legacy `<goal_context>`), case-sensitive. */
function isInternalModelContext(text: string): boolean {
  const trimmed = rustTrim(text);
  if (trimmed.startsWith('<goal_context>') && trimmed.endsWith('</goal_context>')) return true;
  if (!trimmed.startsWith(INTERNAL_CONTEXT_PREFIX)) return false;
  const rest = trimmed.slice(INTERNAL_CONTEXT_PREFIX.length);
  const close = rest.indexOf('">');
  if (close < 0) return false;
  return (
    INTERNAL_CONTEXT_SOURCE.test(rest.slice(0, close)) &&
    rest.slice(close + 2).endsWith('</codex_internal_context>')
  );
}

/** In upstream `CONTEXTUAL_USER_FRAGMENT_MATCHERS` order. */
const MATCHERS: ReadonlyArray<(text: string) => boolean> = [
  marked('# AGENTS.md instructions', '</INSTRUCTIONS>'), // UserInstructions
  marked('<environment_context>', '</environment_context>'), // EnvironmentsState
  isAdditionalContext,
  marked('<agent_message_board_notification>', '</agent_message_board_notification>'),
  isSkillBlock,
  marked('<user_shell_command>', '</user_shell_command>'),
  marked('<turn_aborted>', '</turn_aborted>'),
  marked('<subagent_notification>', '</subagent_notification>'),
  isInternalModelContext,
  marked('<recommended_plugins>', '</recommended_plugins>'),
  (text) =>
    rustTrim(text).startsWith('Warning: The maximum number of unified exec processes you can keep open is'),
  (text) => {
    const trimmed = rustTrim(text);
    return (
      trimmed.startsWith('Warning: apply_patch was requested via ') &&
      trimmed.endsWith('Use the apply_patch tool instead of exec_command.')
    );
  },
  (text) =>
    rustTrim(text).startsWith('Warning: Your account was flagged for potentially high-risk cyber activity'),
];

/** True when one user-role `input_text` block's text is Codex-injected context. */
export function isCodexContextBlock(text: string): boolean {
  return MATCHERS.some((matches) => matches(text));
}
