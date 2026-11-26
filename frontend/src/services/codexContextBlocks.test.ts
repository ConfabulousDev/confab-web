// isCodexContextBlock parity — shared cross-language fixture (z63s).
//
// The same testdata/codex_contextual_user_blocks.json is loaded by the backend
// (backend/internal/codex/contextblocks_test.go) and here, so the Go and TS
// classifiers can't silently drift from each other or from upstream Codex.

import { describe, it, expect } from 'vitest';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { isCodexContextBlock } from './codexContextBlocks';

interface ContextBlockCase {
  name: string;
  text: string;
  isContext: boolean;
}

// Resolved from the Vitest root (the `frontend/` dir) up to the repo-root
// testdata, as tokenStats.test.ts does for model_family_parity.json.
const fixturePath = resolve(process.cwd(), '..', 'testdata', 'codex_contextual_user_blocks.json');
const fixture: { cases: ContextBlockCase[] } = JSON.parse(readFileSync(fixturePath, 'utf-8'));

describe('isCodexContextBlock parity fixture', () => {
  it('has cases', () => {
    expect(fixture.cases.length).toBeGreaterThan(0);
  });

  it.each(fixture.cases)('$name → isContext=$isContext', ({ text, isContext }) => {
    expect(isCodexContextBlock(text)).toBe(isContext);
  });
});
