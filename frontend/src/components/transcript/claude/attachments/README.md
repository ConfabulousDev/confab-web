# Transcript Attachment Components

Renders Claude Code `attachment.*` JSONL rows and the `system.away_summary`
resume-context blurb in the transcript view (CF-346). Each component is a leaf
renderer; dispatch by subtype happens in `AttachmentContent.tsx`. The outer
`ClaudeTimelineMessage` card chrome (role label, timestamp, copy button) wraps these
components in the same way it wraps `FileSnapshotContent`.

## Files

- `AttachmentContent.tsx` — Dispatch component. Routes an `AttachmentMessage`
  to the appropriate renderer based on `attachment.type`. Returns `null` for
  noisy or unknown subtypes (defense in depth — the categorizer filters them
  out before they reach here).
- `HookOutput.tsx` — Exports `HookSuccessOutput` and `HookBlockingError`.
  Always-expanded stdout/stderr `<pre>` blocks for the success variant; red
  panel for the blocking-error variant (the text shown back to the model).
- `EditedFileSnippet.tsx` — Strips `cat -n` line-number prefixes from the
  snippet and renders via `CodeBlock` with language inferred from the filename
  extension.
- `QueuedCommand.tsx` — Branches on `commandMode`. `task-notification` payloads
  render as a monospace `<pre>`; everything else renders through the shared
  markdown helper (`@/utils/renderMarkdownToHtml`).
- `ToolDelta.tsx` — Shared component for `deferred_tools_delta` and
  `mcp_instructions_delta`. Header text differs by subtype; added and removed
  name lists render as always-expanded chip lists.
- `AwaySummary.tsx` — Markdown renderer for `system.away_summary` content.
  Returns `null` for empty/whitespace content. Reuses the `.summary` purple
  card style (the distinguishing `Resume Summary` role label is in
  `ClaudeTimelineMessage` via `getClaudeRoleLabel`).
- `index.ts` — Barrel exports.
- `_chrome.module.css` — Shared `.header` and `.badge` rules consumed via CSS
  Modules `composes:` from the per-card `*.module.css` files.

## Stories + tests

Every component has a `.stories.tsx` (sanitized fictional data, never real
session UUIDs) and a shared smoke-test file:

- `attachments.test.tsx` — One block per component covering empty inputs,
  language inference, markdown vs verbatim rendering, etc.
- `HookOutput.stories.tsx`, `EditedFileSnippet.stories.tsx`,
  `QueuedCommand.stories.tsx`, `ToolDelta.stories.tsx`,
  `AwaySummary.stories.tsx`

## Adding a new attachment subtype

1. Add a Zod branch in `frontend/src/schemas/claudeTranscript.ts` (inside
   `AttachmentInnerSchema`).
2. Export the inferred type and a discriminator helper
   (`isXxxAttachment`).
3. Re-export both from `frontend/src/types/index.ts`.
4. Add (or reuse) a subcategory in
   `frontend/src/components/session/claudeCategories.ts` — extend
   `ClaudeAttachmentSubcategory`, `ClaudeHierarchicalCounts`, `ClaudeFilterState`,
   `DEFAULT_CLAUDE_FILTER_STATE`, `categorizeAttachmentMessage`.
5. Add a sub-chip in `ClaudeFilterDropdown.tsx`.
6. Add a renderer here, wire it into `AttachmentContent.tsx`, and add a
   `.stories.tsx` + a test block in `attachments.test.tsx`.

Subtypes that exist in the JSONL but are deliberately not surfaced
(`task_reminder`, `skill_listing`, `command_permissions`) parse via the
catch-all schema branch and are filtered out by `categorizeAttachmentMessage`
returning `null` — they live in `messages[]` for potential future analytics
but never render.
