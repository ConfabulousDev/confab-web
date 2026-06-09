import { describe, it, expect } from 'vitest';
import { buildGitHubIssueUrl, GITHUB_ISSUES_URL } from './externalLinks';

describe('buildGitHubIssueUrl', () => {
  it('points at the canonical issues/new path', () => {
    const url = buildGitHubIssueUrl('Title', 'Body');
    expect(url.startsWith(`${GITHUB_ISSUES_URL}/new?`)).toBe(true);
  });

  it('encodes title and body as query params', () => {
    const url = buildGitHubIssueUrl('hello world', 'a & b');
    const params = new URL(url).searchParams;
    expect(params.get('title')).toBe('hello world');
    expect(params.get('body')).toBe('a & b');
  });

  it('includes the labels param only when labels are provided', () => {
    expect(new URL(buildGitHubIssueUrl('t', 'b')).searchParams.has('labels')).toBe(false);
    expect(new URL(buildGitHubIssueUrl('t', 'b', [])).searchParams.has('labels')).toBe(false);
    expect(new URL(buildGitHubIssueUrl('t', 'b', ['x', 'y'])).searchParams.get('labels')).toBe('x,y');
  });
});
