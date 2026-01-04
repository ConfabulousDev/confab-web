---
name: frontend-maintenance
description: Frontend codebase maintenance - dead code detection, linting, dependency updates, and cleanup for TypeScript/React code.
---

# Frontend Maintenance

Periodic evaluation and cleanup for the frontend codebase.

## Instructions for Claude

1. Use **TodoWrite** to create a checklist and track progress
2. Run commands from project root using full paths
3. Use the **Grep** tool instead of bash grep for searching
4. Collect all findings, then triage and summarize at the end

## Phase 1: Automated Checks

Track with TodoWrite:

- [ ] Dead code detection (knip)
- [ ] Linting warnings (ESLint)
- [ ] Outdated dependencies
- [ ] Security audit
- [ ] TODO/FIXME audit
- [ ] Test coverage

### Dead Code Detection

```bash
# Knip: finds unused files, exports, and dependencies
cd frontend && npm run knip

# ESLint for general linting
cd frontend && npm run lint
```

**IMPORTANT:** The knip report should be **clean** (no output = no issues).
- **Unused files**: Delete immediately (truly dead code)
- **Unused exports**: Remove export keyword or delete if truly unused
- **Unused dependencies**: Verify before removing (@types/* packages may be implicit)
- If knip reports issues, fix them before continuing with other maintenance tasks

Do NOT ask for permission - just fix and report what was cleaned up.

### Dependency Audit

```bash
cd frontend && npm outdated
cd frontend && npm audit
```

**Update strategy:**
- Patch versions: Update immediately
- Minor versions: Update if low-risk
- Major versions: Note for review, don't auto-update

### Test Coverage

```bash
# Use --run to avoid watch mode
cd frontend && npm test -- --coverage --run
```

## Phase 2: Manual Code Review

Track with TodoWrite:

- [ ] Security review
- [ ] Code smell detection
- [ ] Bundle size check

### Security Review Checklist

- [ ] XSS prevention: `dangerouslySetInnerHTML` uses DOMPurify
- [ ] User input sanitized before display
- [ ] No secrets in client-side code
- [ ] API errors handled gracefully

### Code Smell Patterns to Search

Use Grep to find these patterns:

```
# Empty catch blocks
Pattern: "catch.*\{\s*\}"

# Console.log left in code
Pattern: "console\.log"

# Any type usage
Pattern: ": any"

# Disabled ESLint rules
Pattern: "eslint-disable"
```

### Bundle Size Check

```bash
cd frontend && npm run build
```

Watch for chunks exceeding 500 KB - consider code-splitting.

### Files to Prioritize for Review

**Largest/most complex:**
1. `schemas/transcript.ts` (~545 lines) - Validation
2. `pages/ShareLinksPage.tsx` (~347 lines) - Complex UI
3. `pages/APIKeysPage.tsx` (~316 lines) - Complex UI

## Phase 3: Triage and Report

Create a summary with:

### Findings Table

| Category | Severity | Issue | Location | Action |
|----------|----------|-------|----------|--------|
| Security | High/Med/Low | Description | file:line | Fix/Ticket/Ignore |
| Dead Code | ... | ... | ... | ... |
| Code Smell | ... | ... | ... | ... |
| Bundle | ... | ... | ... | ... |

### Severity Guidelines

- **High**: Security vulnerabilities, crashes
- **Medium**: Bugs that affect functionality, significant code smells
- **Low**: Minor issues, style inconsistencies, small improvements

### Action Guidelines

- **Fix now**: Low-risk, high-value improvements (dead code, patch updates)
- **Create ticket**: Larger refactors needing planning
- **Ignore**: Acceptable tradeoffs, false positives

## Risk Categories

### Low-Risk (Do Immediately)
- Remove dead code flagged by knip
- Fix linting warnings
- Update patch-level dependencies

### Higher-Risk (Plan Carefully)
- Major dependency updates
- Restructuring components
- Shared type modifications
