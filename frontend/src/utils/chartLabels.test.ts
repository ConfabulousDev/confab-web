import { describe, it, expect } from 'vitest';
import { truncateName, truncatedYAxisWidth } from './chartLabels';

describe('truncateName', () => {
  it('returns short names unchanged', () => {
    expect(truncateName('Bash')).toBe('Bash');
    expect(truncateName('')).toBe('');
  });

  it('returns names at the boundary (15 chars) unchanged', () => {
    expect(truncateName('a'.repeat(15))).toBe('a'.repeat(15));
  });

  it('truncates names longer than 15 chars to prefix...suffix', () => {
    expect(truncateName('mcp__claude_ai_Linear__save_issue')).toBe('mcp__c..._issue');
    expect(truncateName('an-extremely-long-agent-name')).toBe('an-ext...t-name');
  });
});

describe('truncatedYAxisWidth', () => {
  it('returns the minimum width (40px) for an empty label list', () => {
    expect(truncatedYAxisWidth([], 4)).toBe(40);
  });

  it('uses minChars as a floor when every label is shorter than the floor', () => {
    // minChars=6 floor → 6*7+8 = 50px
    expect(truncatedYAxisWidth(['a', 'bc'], 6)).toBe(50);
  });

  it('sizes off the truncated form, not the raw label length', () => {
    // Raw length 33 would yield 33*7+8 = 239; truncated to 15 → 15*7+8 = 113.
    const labels = ['mcp__claude_ai_Linear__save_issue'];
    expect(truncatedYAxisWidth(labels, 4)).toBe(113);
  });

  it('picks the longest truncated label across the list', () => {
    const labels = ['Bash', 'mcp__claude_ai_Linear__save_issue', 'Read'];
    expect(truncatedYAxisWidth(labels, 4)).toBe(113);
  });
});
