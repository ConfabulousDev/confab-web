import { describe, it, expect } from 'vitest';
import { getRepoWebURL, getBranchURL } from './git';

describe('getRepoWebURL', () => {
  it('returns null for undefined input', () => {
    expect(getRepoWebURL(undefined)).toBeNull();
  });

  it('returns null for empty string', () => {
    expect(getRepoWebURL('')).toBeNull();
  });

  it('converts SSH github URL to https and strips .git', () => {
    expect(getRepoWebURL('git@github.com:owner/repo.git')).toBe(
      'https://github.com/owner/repo'
    );
  });

  it('converts SSH gitlab URL to https and strips .git', () => {
    expect(getRepoWebURL('git@gitlab.com:owner/repo.git')).toBe(
      'https://gitlab.com/owner/repo'
    );
  });

  it('passes through https github URL and strips .git', () => {
    expect(getRepoWebURL('https://github.com/owner/repo.git')).toBe(
      'https://github.com/owner/repo'
    );
  });

  it('passes through https github URL without .git', () => {
    expect(getRepoWebURL('https://github.com/owner/repo')).toBe(
      'https://github.com/owner/repo'
    );
  });

  it('passes through https gitlab URL', () => {
    expect(getRepoWebURL('https://gitlab.com/owner/repo')).toBe(
      'https://gitlab.com/owner/repo'
    );
  });

  it('returns null for unrecognized host', () => {
    expect(getRepoWebURL('https://bitbucket.org/owner/repo')).toBeNull();
  });
});

describe('getBranchURL', () => {
  it('builds github tree URL with encoded branch name', () => {
    expect(
      getBranchURL({ repo_url: 'https://github.com/owner/repo', branch: 'main' })
    ).toBe('https://github.com/owner/repo/tree/main');
  });

  it('builds gitlab /-/tree/ URL', () => {
    expect(
      getBranchURL({ repo_url: 'https://gitlab.com/owner/repo', branch: 'main' })
    ).toBe('https://gitlab.com/owner/repo/-/tree/main');
  });

  it('URL-encodes branches that contain slashes', () => {
    expect(
      getBranchURL({
        repo_url: 'https://github.com/owner/repo',
        branch: 'feature/new-thing',
      })
    ).toBe('https://github.com/owner/repo/tree/feature%2Fnew-thing');
  });

  it('returns null when branch is missing', () => {
    expect(
      getBranchURL({ repo_url: 'https://github.com/owner/repo', branch: '' })
    ).toBeNull();
    expect(getBranchURL({ repo_url: 'https://github.com/owner/repo' })).toBeNull();
  });

  it('returns null when repo URL is missing or unrecognized', () => {
    expect(getBranchURL({ branch: 'main' })).toBeNull();
    expect(
      getBranchURL({ repo_url: 'https://bitbucket.org/owner/repo', branch: 'main' })
    ).toBeNull();
  });

  it('returns null when gitInfo is undefined', () => {
    expect(getBranchURL(undefined)).toBeNull();
  });
});
