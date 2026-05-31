# CF-268: Amplify Self-Hosted Features — Implementation Spec

## Context

CF-261 already addressed most of the ticket's original asks (Self-Hosted card, modal, DeployCTA on landing page; README rewrite with self-hosted positioning). This ticket is now a **small polish pass** on remaining gaps.

## Scope

Three changes:

### 1. README.md — Split & expand Features section

**Current state:** 8 bullets. "Team Deployment" packs shared-session mode, user limits, and white-label into one line. "Infrastructure" doesn't mention single Docker image or custom domains.

**Target state:** ~10 bullets. Split "Team Deployment" into separate items and expand "Infrastructure":

```markdown
## Features

- **Session Management** — Archive, browse, search sessions; full transcript viewer
- **Analytics & Smart Recaps** (optional) — Cost tracking, AI-powered recaps (requires Anthropic API key)
- **Sharing** — Public and private share links
- **Multi-User Auth** — Password auth, GitHub OAuth, Google OAuth, or OIDC (Okta, Auth0, Azure AD, Keycloak)
- **Team Sharing** — Shared-session mode makes all sessions visible to authenticated team members
- **White-Label** — Disable footer branding and cookie banners for internal deployments
- **Admin Panel** — User management, activation/deactivation, storage monitoring
- **Developer Experience** — GitHub link detection, API keys, per-user rate limiting
- **Infrastructure** — Single Docker image (frontend + backend), Docker Compose one-command deploy, PostgreSQL + MinIO, custom domain support
```

Key changes:
- "Multi-User" → "Multi-User Auth" (clearer)
- "Team Deployment" split into "Team Sharing" and "White-Label"
- "Infrastructure" expanded with single Docker image, one-command deploy, custom domain support

### 2. GitHub repo description

Update via `gh repo edit` to a technical one-liner:

```
Open-source session management platform for Claude Code — analytics, AI recaps, sharing, and team features. Self-hosted with Docker.
```

### 3. GitHub topics

Set via `gh repo edit --add-topic`:

```
self-hosted, open-source, claude-code, developer-tools, docker, anthropic, analytics, ai, session-management
```

## Out of scope

- Landing page changes (modal and cards are fine as-is)
- New SELF-HOSTING.md doc (separate ticket)
- CONFIGURATION.md changes

## Files modified

- `README.md` — Features section rewrite (~10 lines changed)
- GitHub repo settings (via CLI)

## Testing

- `cd frontend && npm run build && npm run lint && npm test` (no frontend code changes but verify nothing breaks)
- Visual review of README rendering on GitHub
