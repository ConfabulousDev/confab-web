---
name: codebase-maintenance
description: Periodic codebase evaluation, cleanup, and tech debt repayment. Use when asked to run maintenance, cleanup, refactor, find dead code, or address tech debt.
---

# Codebase Maintenance

Periodic evaluation and cleanup to keep the codebase healthy.

## Instructions for Claude

1. Use **TodoWrite** to create a checklist and track progress
2. Run commands from project root using full paths
3. Use the **Grep** tool instead of bash grep for searching
4. Use the **Task** tool with `subagent_type=Explore` for codebase exploration
5. Collect all findings, then triage and summarize at the end

## Phase 1: Automated Checks

Track with TodoWrite:

- [ ] Dead code detection (backend)
- [ ] Dead code detection (frontend)
- [ ] Linting warnings
- [ ] Outdated dependencies
- [ ] TODO/FIXME audit
- [ ] Test coverage gaps

### Dead Code Detection

**Backend (Go)**
```bash
# Install if needed
go install honnef.co/go/tools/cmd/staticcheck@latest
go install golang.org/x/tools/cmd/deadcode@latest

# Run from backend dir
~/go/bin/staticcheck ./...
~/go/bin/deadcode -test ./...
```

**IMPORTANT:** Always auto-fix staticcheck and deadcode findings immediately, as long as:
- No functional/logic changes are required
- Just removing unused code, fixing lint warnings, removing unused imports

Do NOT ask for permission - just fix and report what was cleaned up.

**Frontend (TypeScript)**
```bash
cd frontend && npm run lint
```

### Dependency Audit

```bash
# Backend
cd backend && go mod tidy && git diff go.mod go.sum
cd backend && go list -m -u all | grep '\['

# Frontend
cd frontend && npm outdated
cd frontend && npm audit
```

### Test Coverage

```bash
# Backend - IMPORTANT: Run FULL test suite for accurate coverage
# The -short flag skips integration tests which provide most of the coverage
# Example: internal/db goes from 0% (-short) to 69.5% (full)
cd backend && DOCKER_HOST=unix:///Users/jackie/.orbstack/run/docker.sock go test -cover ./...

# Frontend (use --run to avoid watch mode)
cd frontend && npm test -- --coverage --run
```

## Phase 2: Manual Code Review

**Critical** - Read and analyze actual code, don't just run tools.

Track with TodoWrite:

- [ ] Review backend structure (use Task/Explore agent)
- [ ] Review frontend structure (use Task/Explore agent)
- [ ] Security review
- [ ] Code smell detection
- [ ] Duplication analysis

### Codebase Exploration

Use the Task tool with `subagent_type=Explore` to get an overview:
- Architecture and package organization
- Largest files by line count (prioritize for review)
- File size distribution

### Security Review Checklist

**Backend:**
- [ ] SQL queries use parameterized queries (not string concatenation)
- [ ] User input is validated before use
- [ ] Authentication/authorization checks on all protected endpoints
- [ ] Secrets not hardcoded (check env vars)
- [ ] Rate limiting on sensitive endpoints
- [ ] CSRF protection on state-changing operations
- [ ] Proper error handling (no stack traces leaked to users)

**Frontend:**
- [ ] XSS prevention: `dangerouslySetInnerHTML` uses DOMPurify
- [ ] User input sanitized before display
- [ ] No secrets in client-side code
- [ ] API errors handled gracefully

### Code Smell Patterns to Search

Use Grep to find these patterns:

```
# Deeply nested conditionals
Pattern: "if.*{[^}]*if.*{[^}]*if"

# Long parameter lists
Pattern: "func.*\(.*,.*,.*,.*,.*,"

# Magic numbers
Pattern: "[^0-9][0-9]{3,}[^0-9]"

# Empty catch blocks
Pattern: "catch.*\{\s*\}"

# Commented-out code blocks
Pattern: "//.*func |//.*if |//.*for "
```

### Duplication Patterns to Check

Read the largest files and look for:
- Repeated error handling patterns
- Similar API response structures
- Copy-pasted validation logic
- Nearly identical functions

**Known duplication hotspots in this codebase:**
- OAuth callbacks (GitHub vs Google) share similar flow
- Sync file read handlers (owned vs shared) have similar logic
- Database list queries with similar CTE patterns

### Files to Prioritize for Review

**Backend (largest/most complex):**
1. `internal/db/db.go` (~1765 lines) - All DB operations
2. `internal/auth/oauth.go` (~1641 lines) - OAuth flows
3. `internal/api/sync.go` (~987 lines) - Sync logic
4. `internal/api/server.go` (~670 lines) - Routing
5. `internal/admin/handlers.go` (~737 lines) - Admin ops

**Frontend (largest/most complex):**
1. `schemas/transcript.ts` (~545 lines) - Validation
2. `services/agentTreeBuilder.ts` (~348 lines) - Business logic
3. `pages/ShareLinksPage.tsx` (~347 lines) - Complex UI
4. `pages/APIKeysPage.tsx` (~316 lines) - Complex UI

## Phase 3: Triage and Report

Create a summary with:

### Findings Table

| Category | Severity | Issue | Location | Action |
|----------|----------|-------|----------|--------|
| Security | High/Med/Low | Description | file:line | Fix/Ticket/Ignore |
| Bug | ... | ... | ... | ... |
| Code Smell | ... | ... | ... | ... |
| Duplication | ... | ... | ... | ... |

### Severity Guidelines

- **High**: Security vulnerabilities, data loss risks, crashes
- **Medium**: Bugs that affect functionality, significant code smells
- **Low**: Minor issues, style inconsistencies, small improvements

### Action Guidelines

- **Fix now**: Low-risk, high-value improvements
- **Create ticket**: Larger refactors needing planning
- **Ignore**: Acceptable tradeoffs, false positives

## Risk Categories

### Low-Risk (Do Immediately)
- Remove dead code flagged by tools
- Delete commented-out code
- Fix linting warnings
- Update minor dependencies

### Higher-Risk (Plan Carefully)
- Changing function signatures
- Restructuring modules
- Database schema changes
- Shared type modifications

## Tracking Tech Debt

Create Linear tickets with label `tech-debt`:
- What the problem is
- Why it matters
- Effort estimate (S/M/L)
- Blocking dependencies
