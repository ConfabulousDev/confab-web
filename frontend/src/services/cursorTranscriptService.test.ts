// 18n2: cursorTranscriptService parse + normalize, grounded in the real Cursor
// wire fixture shape (backend/internal/analytics/testdata/cursor/main.jsonl):
//   {role:"user"|"assistant", message:{content:[{type:"text",text}|{type:"tool_use",name,input}]}}
//   {type:"turn_ended", status:"success"|"error", error?}
// Cursor records tool INPUTS only (no id, no tool_result), file tools key on `path`.

import { describe, expect, it } from 'vitest';
import {
  parseCursorJSONL,
  normalizeCursorLines,
  extractCursorModel,
  cleanCursorAssistantText,
  parseCursorUserText,
  attachCursorTimestamps,
} from './cursorTranscriptService';
import { extractCursorItemText } from '@/components/session/extractCursorItemText';

function line(obj: unknown): string {
  return JSON.stringify(obj);
}

const userLine = {
  role: 'user',
  message: { content: [{ type: 'text', text: 'Add input validation to the session handler.' }] },
};

const assistantLine = {
  role: 'assistant',
  message: {
    content: [
      { type: 'text', text: 'Reading the handler and searching for validation helpers.' },
      { type: 'tool_use', name: 'Read', input: { path: 'internal/api/session_handler.go' } },
      { type: 'tool_use', name: 'Grep', input: { pattern: 'func validate', path: 'internal/api' } },
    ],
  },
};

const turnEndedOk = { type: 'turn_ended', status: 'success' };
const turnEndedErr = { type: 'turn_ended', status: 'error', error: 'usage limit' };

function rawOf(...objs: unknown[]) {
  return parseCursorJSONL(objs.map(line).join('\n')).rawLines;
}

describe('parseCursorJSONL', () => {
  it('parses both line shapes and counts non-empty lines', () => {
    const { rawLines, totalLines } = parseCursorJSONL(
      [userLine, assistantLine, turnEndedOk].map(line).join('\n'),
    );
    expect(totalLines).toBe(3);
    expect(rawLines).toHaveLength(3);
  });

  it('skips blank lines but counts only the non-empty ones', () => {
    const jsonl = `${line(userLine)}\n\n${line(turnEndedOk)}\n`;
    const { totalLines } = parseCursorJSONL(jsonl);
    expect(totalLines).toBe(2);
  });

  it('keeps a malformed line as an invalid entry instead of dropping it', () => {
    const { rawLines, totalLines } = parseCursorJSONL(`${line(userLine)}\n{not json`);
    expect(totalLines).toBe(2);
    expect(rawLines).toHaveLength(2);
  });
});

describe('normalizeCursorLines', () => {
  it('normalizes a user line into a single user render item', () => {
    const items = normalizeCursorLines(rawOf(userLine));
    expect(items).toHaveLength(1);
    const first = items[0];
    expect(first?.kind).toBe('user');
    if (first?.kind === 'user') {
      expect(first.text).toContain('input validation');
    }
  });

  it('normalizes an assistant line into one assistant item plus one tool item per tool_use', () => {
    const items = normalizeCursorLines(rawOf(assistantLine));
    const kinds = items.map((i) => i.kind);
    expect(kinds).toEqual(['assistant', 'tool', 'tool']);
    const tool = items[1];
    expect(tool?.kind).toBe('tool');
    if (tool?.kind === 'tool') {
      expect(tool.toolName).toBe('Read');
      // File tools surface their target via `path` (NOT file_path).
      expect(tool.input).toContain('internal/api/session_handler.go');
    }
  });

  it('excludes turn_ended rows from the render stream (decision 3)', () => {
    const items = normalizeCursorLines(rawOf(turnEndedOk, turnEndedErr));
    expect(items).toHaveLength(0);
  });

  it('assigns synthetic line-based ids (no stable id in the wire format)', () => {
    const items = normalizeCursorLines(rawOf(userLine, assistantLine));
    const ids = items.map((i) => i.id);
    expect(new Set(ids).size).toBe(ids.length); // all distinct
    expect(ids.every((id) => typeof id === 'string' && id.length > 0)).toBe(true);
  });

  it('tool items carry no output/result (Cursor records inputs only)', () => {
    const items = normalizeCursorLines(rawOf(assistantLine));
    const tools = items.filter((i) => i.kind === 'tool');
    expect(tools.length).toBeGreaterThan(0);
    for (const t of tools) {
      expect(t).not.toHaveProperty('output');
    }
  });
});

// fa3h: Cursor's on-disk JSONL appends a bare `[REDACTED]` to nearly every
// assistant turn — either as a trailing suffix after narrative or as the
// entire text block on tool-only turns. Strip it during normalize. Never touch
// Confab CLI `[REDACTED:TYPE]` markers (different contract — stay visible).
describe('cleanCursorAssistantText (fa3h)', () => {
  it('strips a trailing "\\n\\n[REDACTED]" suffix and preserves the narrative', () => {
    expect(cleanCursorAssistantText('Checking the repo for open alerts.\n\n[REDACTED]')).toBe(
      'Checking the repo for open alerts.',
    );
  });

  it('strips a trailing single-newline "\\n[REDACTED]" suffix', () => {
    expect(cleanCursorAssistantText('Fetching details.\n[REDACTED]')).toBe('Fetching details.');
  });

  it('returns "" for a block whose entire content is "[REDACTED]"', () => {
    expect(cleanCursorAssistantText('[REDACTED]')).toBe('');
  });

  it('returns "" for whitespace + "[REDACTED]" only', () => {
    expect(cleanCursorAssistantText('  \n[REDACTED]\n  ')).toBe('');
  });

  it('handles carriage returns before the placeholder', () => {
    expect(cleanCursorAssistantText('Doing work.\r\n\r\n[REDACTED]')).toBe('Doing work.');
  });

  it('leaves text with no placeholder untouched', () => {
    expect(cleanCursorAssistantText('Just narrative, no redaction.')).toBe(
      'Just narrative, no redaction.',
    );
  });

  it('NEVER strips a Confab CLI [REDACTED:TYPE] marker (different contract)', () => {
    expect(cleanCursorAssistantText('See [REDACTED:GITHUB_TOKEN] in the env.')).toBe(
      'See [REDACTED:GITHUB_TOKEN] in the env.',
    );
    // even when trailing
    expect(cleanCursorAssistantText('Token is [REDACTED:GITHUB_TOKEN]')).toBe(
      'Token is [REDACTED:GITHUB_TOKEN]',
    );
  });
});

describe('normalizeCursorLines [REDACTED] handling (fa3h)', () => {
  const narrativePlusRedactedPlusTool = {
    role: 'assistant',
    message: {
      content: [
        { type: 'text', text: 'Checking the repo for open alerts.\n\n[REDACTED]' },
        { type: 'tool_use', name: 'Shell', input: { command: 'gh api alerts' } },
      ],
    },
  };

  const redactedOnlyPlusTool = {
    role: 'assistant',
    message: {
      content: [
        { type: 'text', text: '[REDACTED]' },
        { type: 'tool_use', name: 'Shell', input: { command: 'gh api alerts' } },
      ],
    },
  };

  it('strips the trailing [REDACTED] but keeps the assistant narrative and the tool row', () => {
    const items = normalizeCursorLines(rawOf(narrativePlusRedactedPlusTool));
    expect(items.map((i) => i.kind)).toEqual(['assistant', 'tool']);
    const assistant = items[0];
    if (assistant?.kind === 'assistant') {
      expect(assistant.text).toBe('Checking the repo for open alerts.');
      expect(assistant.text).not.toContain('[REDACTED]');
    }
  });

  it('omits the assistant item entirely on a [REDACTED]-only line but keeps the tool row', () => {
    const items = normalizeCursorLines(rawOf(redactedOnlyPlusTool));
    expect(items.map((i) => i.kind)).toEqual(['tool']);
  });

  it('preserves a Confab CLI [REDACTED:TYPE] marker in normalized assistant text', () => {
    const withTypedMarker = {
      role: 'assistant',
      message: {
        content: [{ type: 'text', text: 'Using [REDACTED:GITHUB_TOKEN] for auth.' }],
      },
    };
    const items = normalizeCursorLines(rawOf(withTypedMarker));
    const assistant = items[0];
    expect(assistant?.kind).toBe('assistant');
    if (assistant?.kind === 'assistant') {
      expect(assistant.text).toContain('[REDACTED:GITHUB_TOKEN]');
    }
  });
});

describe('extractCursorModel', () => {
  it('returns undefined — Cursor lines carry no model field', () => {
    expect(extractCursorModel()).toBeUndefined();
  });
});

// nfbe: Cursor user `text` blocks arrive wrapped in an envelope. The human
// prompt lives in `<user_query>…</user_query>`; injected context (rules,
// attached files, skills, …) arrives as sibling top-level tagged blocks. The
// user row must show ONLY the prompt — the tags must never render literally.
// Ground truth (real ~/.cursor agent-transcripts): every user text block is
// `<user_query>…</user_query>` (sometimes preceded by a bare `[Image]` line or
// a `<manually_attached_skills>…</manually_attached_skills>` block); `[Image #N]`
// placeholders appear INSIDE the query.
describe('parseCursorUserText (nfbe)', () => {
  it('extracts the <user_query> content as the prompt and trims it', () => {
    const { prompt } = parseCursorUserText(
      '<user_query>\ndoes gh repo have any outstanding dependabot alerts?\n</user_query>',
    );
    expect(prompt).toBe('does gh repo have any outstanding dependabot alerts?');
  });

  it('does not leak the envelope tags into the prompt', () => {
    const { prompt } = parseCursorUserText('<user_query>hello world</user_query>');
    expect(prompt).not.toContain('<user_query>');
    expect(prompt).not.toContain('</user_query>');
  });

  it('falls back to the raw text (trimmed) when no <user_query> tag is present', () => {
    const { prompt, sections } = parseCursorUserText('  plain prompt with no envelope  ');
    expect(prompt).toBe('plain prompt with no envelope');
    expect(sections).toEqual([]);
  });

  it('falls back to raw when the <user_query> tag is unclosed (never drops content)', () => {
    const raw = '<user_query>unterminated prompt body';
    const { prompt } = parseCursorUserText(raw);
    expect(prompt).toBe(raw.trim());
  });

  it('concatenates multiple <user_query> blocks', () => {
    const { prompt } = parseCursorUserText(
      '<user_query>first</user_query>\n<user_query>second</user_query>',
    );
    expect(prompt).toContain('first');
    expect(prompt).toContain('second');
  });

  it('keeps [Image #N] placeholders literal inside the prompt (v1, no image render)', () => {
    const { prompt } = parseCursorUserText(
      '<user_query>\nthis user msg: [Image #1] needs work\n</user_query>',
    );
    expect(prompt).toContain('[Image #1]');
  });

  it('captures other recognized top-level tagged blocks as sections (consumed by 0rcv)', () => {
    const { prompt, sections } = parseCursorUserText(
      '<manually_attached_skills>\nSkill body here.\n</manually_attached_skills>\n<user_query>\nwrite up some tickets.\n</user_query>',
    );
    expect(prompt).toBe('write up some tickets.');
    expect(sections).toHaveLength(1);
    expect(sections[0]?.tag).toBe('manually_attached_skills');
    expect(sections[0]?.content).toContain('Skill body here.');
    expect(sections[0]?.label.length).toBeGreaterThan(0);
  });

  it('returns an empty prompt for an empty query without throwing', () => {
    const { prompt } = parseCursorUserText('<user_query></user_query>');
    expect(prompt).toBe('');
  });

  it('returns an empty prompt for a whitespace-only query', () => {
    const { prompt } = parseCursorUserText('<user_query>   \n  </user_query>');
    expect(prompt).toBe('');
  });
});

describe('normalizeCursorLines user-query extraction (nfbe)', () => {
  const wrappedUserLine = {
    role: 'user',
    message: {
      content: [
        {
          type: 'text',
          text: '<user_query>\ndoes gh repo have any outstanding dependabot alerts?\n</user_query>',
        },
      ],
    },
  };

  it('renders only the extracted prompt in the user row (no envelope tags)', () => {
    const items = normalizeCursorLines(rawOf(wrappedUserLine));
    expect(items).toHaveLength(1);
    const first = items[0];
    expect(first?.kind).toBe('user');
    if (first?.kind === 'user') {
      expect(first.text).toBe('does gh repo have any outstanding dependabot alerts?');
      expect(first.text).not.toContain('<user_query>');
    }
  });

  it('omits the user row entirely when the extracted prompt is empty', () => {
    const emptyQuery = {
      role: 'user',
      message: { content: [{ type: 'text', text: '<user_query>   </user_query>' }] },
    };
    const items = normalizeCursorLines(rawOf(emptyQuery));
    expect(items).toHaveLength(0);
  });

  it('carries the parsed sections on the user render item (shape for 0rcv)', () => {
    const withContext = {
      role: 'user',
      message: {
        content: [
          {
            type: 'text',
            text: '<manually_attached_skills>\nSkill body.\n</manually_attached_skills>\n<user_query>\ngo\n</user_query>',
          },
        ],
      },
    };
    const items = normalizeCursorLines(rawOf(withContext));
    const first = items[0];
    expect(first?.kind).toBe('user');
    if (first?.kind === 'user') {
      expect(first.text).toBe('go');
      expect(first.sections?.[0]?.tag).toBe('manually_attached_skills');
    }
  });
});

describe('extractCursorItemText searches the extracted prompt (nfbe)', () => {
  it('matches the prompt text, not the stripped envelope tags', () => {
    const items = normalizeCursorLines(
      rawOf({
        role: 'user',
        message: {
          content: [{ type: 'text', text: '<user_query>find the validation seam</user_query>' }],
        },
      }),
    );
    const text = extractCursorItemText(items[0]!);
    expect(text).toContain('find the validation seam');
    expect(text).not.toContain('user_query');
  });
});

// ce79: Cursor lines carry no per-message timestamp, so estimated per-row times
// are interpolated frontend-side over the distinct wire lines (each line index
// becomes one conversation row) between the session start (firstSeen) and end
// (lastSyncAt) bounds. Tool render items inherit their parent assistant line's
// timestamp because they share its line index.
describe('attachCursorTimestamps', () => {
  const T0 = '2026-06-17T10:00:00.000Z';
  const T1 = '2026-06-17T10:00:10.000Z';
  const MID = '2026-06-17T10:00:05.000Z';

  // Three distinct wire lines (user, assistant, user) → three conversation rows
  // interpolated to T0, midpoint, T1.
  const threeLineRaw = rawOf(
    { role: 'user', message: { content: [{ type: 'text', text: 'first prompt' }] } },
    { role: 'assistant', message: { content: [{ type: 'text', text: 'a reply' }] } },
    { role: 'user', message: { content: [{ type: 'text', text: 'second prompt' }] } },
  );

  it('interpolates linearly over distinct lines: first=start, last=end, middle=midpoint', () => {
    const items = attachCursorTimestamps(normalizeCursorLines(threeLineRaw), {
      start: T0,
      end: T1,
    });
    expect(items).toHaveLength(3);
    expect(items[0]?.timestamp).toBe(T0);
    expect(items[1]?.timestamp).toBe(MID);
    expect(items[2]?.timestamp).toBe(T1);
  });

  it('produces monotonically non-decreasing timestamps down the transcript', () => {
    const items = attachCursorTimestamps(normalizeCursorLines(threeLineRaw), {
      start: T0,
      end: T1,
    });
    for (let i = 1; i < items.length; i++) {
      const prev = Date.parse(items[i - 1]!.timestamp!);
      const cur = Date.parse(items[i]!.timestamp!);
      expect(cur).toBeGreaterThanOrEqual(prev);
    }
  });

  it('makes tool items inherit their parent assistant line timestamp', () => {
    // One user line, then one assistant line carrying narrative + two tool_use
    // blocks. The assistant row and both tool rows share line index 1, so all
    // three carry the same (end) timestamp.
    const raw = rawOf(
      { role: 'user', message: { content: [{ type: 'text', text: 'do it' }] } },
      {
        role: 'assistant',
        message: {
          content: [
            { type: 'text', text: 'reading and grepping' },
            { type: 'tool_use', name: 'Read', input: { path: 'a.go' } },
            { type: 'tool_use', name: 'Grep', input: { pattern: 'x' } },
          ],
        },
      },
    );
    const items = attachCursorTimestamps(normalizeCursorLines(raw), { start: T0, end: T1 });
    // user(line0)=T0; assistant + 2 tools (line1)=T1
    expect(items[0]?.kind).toBe('user');
    expect(items[0]?.timestamp).toBe(T0);
    const lineOne = items.slice(1);
    expect(lineOne).toHaveLength(3);
    for (const it of lineOne) {
      expect(it.timestamp).toBe(T1);
    }
  });

  it('assigns the single row the start timestamp when there is one line', () => {
    const items = attachCursorTimestamps(
      normalizeCursorLines(
        rawOf({ role: 'user', message: { content: [{ type: 'text', text: 'only one' }] } }),
      ),
      { start: T0, end: T1 },
    );
    expect(items).toHaveLength(1);
    expect(items[0]?.timestamp).toBe(T0);
  });

  it('assigns all rows the same timestamp when bounds are equal', () => {
    const items = attachCursorTimestamps(normalizeCursorLines(threeLineRaw), {
      start: T0,
      end: T0,
    });
    for (const it of items) {
      expect(it.timestamp).toBe(T0);
    }
  });

  it('omits timestamps when a bound is missing', () => {
    const noEnd = attachCursorTimestamps(normalizeCursorLines(threeLineRaw), {
      start: T0,
      end: null,
    });
    for (const it of noEnd) {
      expect(it.timestamp).toBeUndefined();
    }
    const noStart = attachCursorTimestamps(normalizeCursorLines(threeLineRaw), {
      start: undefined,
      end: T1,
    });
    for (const it of noStart) {
      expect(it.timestamp).toBeUndefined();
    }
  });

  it('omits timestamps when bounds are unparseable or inverted', () => {
    const bad = attachCursorTimestamps(normalizeCursorLines(threeLineRaw), {
      start: 'not-a-date',
      end: T1,
    });
    for (const it of bad) {
      expect(it.timestamp).toBeUndefined();
    }
    const inverted = attachCursorTimestamps(normalizeCursorLines(threeLineRaw), {
      start: T1,
      end: T0,
    });
    for (const it of inverted) {
      expect(it.timestamp).toBeUndefined();
    }
  });

  it('returns an empty list unchanged', () => {
    expect(attachCursorTimestamps([], { start: T0, end: T1 })).toEqual([]);
  });
});
