---
name: cut-release
description: Cut a new semver release — tag, write release notes with rigorous DB migration and API change verification, and publish via gh.
---

# Cut a Release

Cut a new semver release for confab-web. Tags, writes release notes, and publishes to GitHub.

## Instructions for Claude

### Step 1: Determine the version and gather commits

- Find the latest tag: `git describe --tags --abbrev=0`
- Bump the patch version (e.g., `v0.3.18` → `v0.3.19`) unless the user specifies otherwise
- Ensure you're on main and up to date: `git checkout main && git pull origin main`
- Gather commits: `git log <prev-tag>..HEAD --oneline`

### Step 2: Write release notes

Do **not** use `--generate-notes`. Write release notes manually.

#### General sections

- **All commits listed with descriptions**, grouped by category (Features, Bug Fixes, Security, Refactoring, Docs, CI, etc.)
- **Link to PRs** where commits have them (e.g., `[#20](url)`)
- **Breaking changes section** covering: renamed/removed env vars, CLI impact, and any other breaking changes not covered by the elevated sections below

#### Elevated sections (mandatory — must not be missed)

Two categories get their own dedicated `##`-level sections in **every** release. Verify these rigorously — **do not rely on commit messages alone; check the actual diff.**

##### DB Migrations

- Run: `git diff <prev-tag>..HEAD --name-only -- backend/internal/db/migrations/`
- If any migration files exist, add a `## DB Migrations` section listing each migration number, name, and what it does
- If none, write: `## DB Migrations` / `None.`

##### API Changes

- Run: `git diff <prev-tag>..HEAD -- backend/API.md` to detect documented API changes
- Also check for new/modified routes: `git diff <prev-tag>..HEAD --name-only -- backend/internal/api/`
- If any public API endpoints were added, removed, or changed, add a `## API Changes` section listing each change with method, path, and description
- If none, write: `## API Changes` / `None.`

### Step 3: Tag, publish, and verify

```bash
git tag v0.X.Y
git push origin v0.X.Y
gh release create v0.X.Y --notes "<release notes>"
```

Use a HEREDOC for the notes body to preserve formatting.

After publishing:
- Run `gh release view v0.X.Y` and confirm the notes look correct
- Confirm the elevated sections (DB Migrations, API Changes) are present and accurate
