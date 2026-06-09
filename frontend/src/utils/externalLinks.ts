// Canonical external URLs surfaced from inside the app (CF-571). These point
// at the hosted project regardless of deployment — self-hosted instances link
// to the same docs site and file issues against the upstream repo.
export const DOCS_URL = 'https://docs.confabulous.dev';
export const GITHUB_ISSUES_URL = 'https://github.com/ConfabulousDev/confab-web/issues';

// CF-574: build a prefilled GitHub "New issue" URL. Title/body are passed as
// query params (GitHub renders `body` as Markdown); `labels` is optional and
// silently ignored by GitHub for users who can't apply it. Generic plumbing —
// callers are responsible for what goes in `body` (see reportUnknown.ts for the
// privacy-conscious redaction layer).
export function buildGitHubIssueUrl(title: string, body: string, labels?: string[]): string {
  const params = new URLSearchParams({ title, body });
  if (labels && labels.length > 0) params.set('labels', labels.join(','));
  return `${GITHUB_ISSUES_URL}/new?${params.toString()}`;
}
