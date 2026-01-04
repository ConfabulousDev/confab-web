---
name: backend-maintenance
description: Backend codebase maintenance - dead code detection, linting, dependency updates, and cleanup for Go code.
---

# Backend Maintenance

Periodic evaluation and cleanup for the backend codebase.

## Instructions for Claude

1. Use **TodoWrite** to create a checklist and track progress
2. Run commands from backend directory
3. Use the **Grep** tool instead of bash grep for searching
4. Use the **Task** tool with `subagent_type=Explore` for codebase exploration
5. Collect all findings, then triage and summarize at the end

## Phase 1: Automated Checks

Track with TodoWrite:

- [ ] Dead code detection (staticcheck)
- [ ] Dead code detection (deadcode)
- [ ] Linting (go vet)
- [ ] Outdated dependencies
- [ ] TODO/FIXME audit
- [ ] Test coverage

### Dead Code Detection

```bash
# Install if needed
go install honnef.co/go/tools/cmd/staticcheck@latest
go install golang.org/x/tools/cmd/deadcode@latest

# Run from backend dir
cd backend && ~/go/bin/staticcheck ./...
cd backend && ~/go/bin/deadcode -test ./...
```

**IMPORTANT:** Always auto-fix staticcheck and deadcode findings immediately, as long as:
- No functional/logic changes are required
- Just removing unused code, fixing lint warnings, removing unused imports

Do NOT ask for permission - just fix and report what was cleaned up.

### Linting

```bash
cd backend && go vet ./...
```

### Dependency Audit

```bash
cd backend && go mod tidy && git diff go.mod go.sum
cd backend && go list -m -u all | grep '\['
```

### Test Coverage

```bash
# IMPORTANT: Run FULL test suite for accurate coverage
# The -short flag skips integration tests which provide most of the coverage
# Example: internal/db goes from 0% (-short) to 69.5% (full)
cd backend && DOCKER_HOST=unix:///Users/jackie/.orbstack/run/docker.sock go test -cover ./...
```

## Phase 2: Manual Code Review

Track with TodoWrite:

- [ ] Review package structure (use Task/Explore agent)
- [ ] Security review
- [ ] Code smell detection
- [ ] Duplication analysis

### Security Review Checklist

- [ ] SQL queries use parameterized queries (not string concatenation)
- [ ] User input is validated before use
- [ ] Authentication/authorization checks on all protected endpoints
- [ ] Secrets not hardcoded (check env vars)
- [ ] Rate limiting on sensitive endpoints
- [ ] CSRF protection on state-changing operations
- [ ] Proper error handling (no stack traces leaked to users)

### Code Smell Patterns to Search

Use Grep to find these patterns:

```
# Long parameter lists
Pattern: "func.*\(.*,.*,.*,.*,.*,"

# Magic numbers
Pattern: "[^0-9][0-9]{3,}[^0-9]"

# Commented-out code blocks
Pattern: "//.*func |//.*if |//.*for "

# Naked returns in long functions
Pattern: "return$"

# Empty error handling
Pattern: "if err != nil {\s*}"
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

**Largest/most complex:**
1. `internal/db/db.go` (~1765 lines) - All DB operations
2. `internal/auth/oauth.go` (~1641 lines) - OAuth flows
3. `internal/api/sync.go` (~987 lines) - Sync logic
4. `internal/api/server.go` (~670 lines) - Routing
5. `internal/admin/handlers.go` (~737 lines) - Admin ops

## Phase 3: Triage and Report

Create a summary with:

### Findings Table

| Category | Severity | Issue | Location | Action |
|----------|----------|-------|----------|--------|
| Security | High/Med/Low | Description | file:line | Fix/Ticket/Ignore |
| Dead Code | ... | ... | ... | ... |
| Code Smell | ... | ... | ... | ... |
| Duplication | ... | ... | ... | ... |

### Severity Guidelines

- **High**: Security vulnerabilities, data loss risks, crashes
- **Medium**: Bugs that affect functionality, significant code smells
- **Low**: Minor issues, style inconsistencies, small improvements

### Action Guidelines

- **Fix now**: Low-risk, high-value improvements (dead code, unused imports)
- **Create ticket**: Larger refactors needing planning
- **Ignore**: Acceptable tradeoffs, false positives

## Risk Categories

### Low-Risk (Do Immediately)
- Remove dead code flagged by tools
- Delete commented-out code
- Fix linting warnings
- Run go mod tidy

### Higher-Risk (Plan Carefully)
- Changing function signatures
- Restructuring packages
- Database schema changes
- Shared type modifications

## Tracking Tech Debt

Create Linear tickets with label `tech-debt`:
- What the problem is
- Why it matters
- Effort estimate (S/M/L)
- Blocking dependencies
