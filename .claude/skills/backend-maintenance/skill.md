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
- [ ] DRY / code deduplication review
- [ ] Logic simplification review

### Security Review Checklist

**POSITIVE patterns to verify are in place:**

- [ ] SQL queries use parameterized queries (not string concatenation)
- [ ] User input validated via `internal/validation` package
- [ ] Authentication/authorization checks on all protected endpoints
- [ ] Secrets from env vars only (never hardcoded)
- [ ] Rate limiting on sensitive endpoints (`internal/ratelimit`)
- [ ] CSRF protection via gorilla/csrf on state-changing operations
- [ ] Cookie security: HttpOnly, SameSite=Lax, Secure flag
- [ ] API keys hashed with SHA-256 before storage
- [ ] Open redirect prevention (validate redirect URLs)
- [ ] User status checks (block inactive users)

**NEGATIVE patterns to search for (vulnerabilities):**

```bash
# SQL injection risks - string concatenation in queries
grep -r 'fmt.Sprintf.*SELECT\|fmt.Sprintf.*INSERT\|fmt.Sprintf.*UPDATE\|fmt.Sprintf.*DELETE'

# Hardcoded secrets
grep -ri 'password.*=.*"\|secret.*=.*"\|apikey.*=.*"\|api_key.*=.*"'

# Ignored errors on sensitive operations
grep -r 'body, _ := io.ReadAll'
```

### Code Smell Patterns to Search

Use Grep to find these patterns:

```
# Long parameter lists (5+ params)
Pattern: "func.*\(.*,.*,.*,.*,.*,"

# Magic numbers (undocumented constants)
Pattern: "[^0-9][0-9]{3,}[^0-9]"

# Commented-out code blocks
Pattern: "//.*func |//.*if |//.*for "

# Naked returns in long functions
Pattern: "return$"

# Empty error handling
Pattern: "if err != nil {\s*}" (multiline)

# Silent error ignoring
Pattern: ", _ :="
```

### DRY / Code Deduplication Review

Actively search for opportunities to reduce duplication and simplify logic:

**Duplicated patterns to look for:**
- Similar handler functions that could share a common helper (e.g., CRUD handlers, middleware patterns)
- Repeated SQL query fragments or database access patterns across functions
- Copy-pasted error handling, response formatting, or validation logic
- Near-identical struct definitions or type conversions
- Similar test setup/teardown code that could use shared helpers

**How to search:**
1. Look at the largest files first - they often contain logic that could be extracted
2. Use Grep to find repeated patterns: similar function signatures, identical SQL fragments, duplicated struct fields
3. Compare handlers that do similar things (e.g., multiple CRUD endpoints with the same get/validate/respond pattern)
4. Check for inline logic that already exists as a utility function elsewhere

**Logic simplification to look for:**
- Overly nested conditionals that could be flattened (early returns, guard clauses)
- Complex expressions that could be extracted into well-named variables or functions
- Redundant nil/error checks that are already guaranteed by prior logic
- Verbose patterns that have simpler idiomatic Go equivalents (e.g., using `errors.Is` instead of type switches)
- Functions doing too many things that could be split into focused helpers

**Action:** Report findings with specific file locations and a brief description of the simplification. For low-risk improvements (extracting a shared helper, simplifying a conditional), fix directly. For larger refactors, note in the findings table.

### DRY Violations - Known Hotspots

**Reviewed and marked as acceptable (see code comments):**

1. **OAuth Callbacks** (`internal/auth/oauth.go`) - ACCEPTABLE
   - `HandleGitHubCallback` and `HandleGoogleCallback` share similar logic
   - Kept separate for clarity, easier debugging, and provider-specific customization
   - See NOTE comments on each function

2. **Inline HTML Templates** (`internal/auth/oauth.go`, `internal/admin/handlers.go`) - ACCEPTABLE
   - Simple, self-contained pages that rarely change
   - Avoids external template file dependencies
   - See NOTE comments on `generateDevicePageHTML`, `HandleLoginSelector`, `HandleListUsers`

3. **Cookie Operations** (`internal/auth/oauth.go`) - FIXED
   - Now uses `clearCookie(w, name)` helper function

4. **Two Rate Limiter Implementations** - ACCEPTABLE
   - `internal/ratelimit`: Token bucket for API rate limiting (allows bursts)
   - `internal/email`: Sliding window for strict email quotas (no bursts)
   - Different algorithms for different requirements
   - See NOTE comment on `EmailRateLimiter` type

5. **Session List Queries** (`internal/db/sessions.go`) - ACCEPTABLE
   - Complex SQL with repeated CTEs for owned/shared/system views
   - Keeping in Go code provides better tooling than database views
   - See NOTE comment on `ListUserSessions` function

**Remaining items to consider (lower priority):**

6. **Analytics Store Operations** (`internal/analytics/store.go`)
   - Repetitive get/upsert patterns for each card type (7 card types)
   - Each has nearly identical structure
   - **Consider**: Generics or code generation (if adding many more card types)

### Files to Prioritize for Review

**By size/complexity (lines of production code):**

| File | Lines | Notes |
|------|-------|-------|
| `internal/auth/oauth.go` | ~1910 | OAuth flows, device auth, login selector |
| `internal/api/sync.go` | ~1150 | Sync init/chunk/read handlers |
| `internal/analytics/store.go` | ~995 | Card storage operations |
| `internal/admin/handlers.go` | ~711 | Admin user management UI |
| `internal/db/sessions.go` | ~693 | Session CRUD operations |
| `internal/api/server.go` | ~715 | Routing, middleware setup |
| `internal/api/shares.go` | ~650 | Share creation/management |
| `internal/storage/s3.go` | ~363 | S3/MinIO operations |

### Simplification Opportunities

1. ~~**Extract shared OAuth logic**~~ - Marked acceptable (see code comments)
2. ~~**Template files for HTML**~~ - Marked acceptable (see code comments)
3. ~~**Cookie helper functions**~~ - DONE (`clearCookie` helper added)
4. **Consolidate error response helpers** - Medium value, low risk (optional)

## Phase 3: Code Simplification

After fixing issues in Phases 1-2, run the **code-simplifier** agent to simplify and refine any modified backend code:

```
Use the Task tool with subagent_type="code-simplifier" and prompt:
"Simplify and refine recently modified Go code in the backend/ directory.
Focus on clarity, consistency, and maintainability while preserving all functionality."
```

This catches additional simplification opportunities (verbose patterns, unnecessary complexity, inconsistent style) that automated tools miss.

## Phase 4: Triage and Report

Create a summary with:

### Findings Table

| Category | Severity | Issue | Location | Action |
|----------|----------|-------|----------|--------|
| Security | High/Med/Low | Description | file:line | Fix/Ticket/Ignore |
| Dead Code | ... | ... | ... | ... |
| Code Smell | ... | ... | ... | ... |
| Duplication | ... | ... | ... | ... |
| Simplification | ... | ... | ... | ... |

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
- Remove unused imports

### Medium-Risk (Review Carefully)
- Extract helper functions
- Move HTML to templates
- Add missing error handling

### Higher-Risk (Plan Carefully)
- Changing function signatures
- Restructuring packages
- Database schema changes
- Shared type modifications
- OAuth flow changes

## Architecture Notes

**Positive patterns in this codebase:**

- Clear package separation (api, db, auth, storage, email, analytics)
- Interface usage for testability (RateLimiter, email.Service)
- Context propagation with timeouts throughout
- OpenTelemetry tracing instrumentation
- Sentinel errors (ErrSessionNotFound, ErrForbidden, etc.)
- Validation package with DB-aligned length limits
- Graceful shutdown handling

**Areas reviewed and marked acceptable:**

- OAuth callbacks: Duplication is intentional for clarity (see code comments)
- Inline HTML: Kept inline to avoid template dependencies (see code comments)
- Cookie operations: Now use `clearCookie` helper

**Minor items to watch:**

- Some silent error ignoring (`_, _ :=` patterns) - low severity

## Tracking Tech Debt

Create Linear tickets with label `tech-debt`:
- What the problem is
- Why it matters
- Effort estimate (S/M/L)
- Blocking dependencies
- Risk level
