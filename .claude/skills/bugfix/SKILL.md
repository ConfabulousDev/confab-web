---
name: bugfix
description: Fix bugs using Test-Driven Development. Use for bug fixes from Linear tickets or user reports. Emphasizes writing tests FIRST before any implementation.
---

# Bug Fix (TDD Workflow)

Fix bugs using strict Test-Driven Development. **Always write tests first.**

## Critical: TDD is Mandatory

This skill enforces TDD. The workflow is:

1. **RED** - Write a failing test that reproduces the bug
2. **GREEN** - Write the minimal code to make the test pass
3. **REFACTOR** - Clean up while keeping tests green
4. **REVIEW** - Perform extremely thorough code review before commit

**Do NOT write implementation code before the test exists and fails.**

**Do NOT commit or push until code review is complete.**

## Instructions for Claude

1. Use **TodoWrite** to track TDD phases explicitly
2. Understand the bug thoroughly before writing any code
3. Write the test FIRST - it must fail initially
4. Implement the fix - minimal changes only
5. Run all tests to ensure no regressions
6. **Perform an extremely thorough code review before committing**
7. Only after code review passes: update Linear ticket and commit (if requested)

## Phase 1: Understand the Bug

Track with TodoWrite:

- [ ] Read the bug report/ticket thoroughly
- [ ] Identify the affected code path
- [ ] Understand the expected vs actual behavior
- [ ] Identify the test file location

### Locate Existing Tests

**Backend (Go):**
```bash
# Find test file for a source file
# e.g., db.go -> db_test.go
ls backend/internal/*/**_test.go
```

**Frontend (TypeScript):**
```bash
# Tests are usually co-located
# e.g., Component.tsx -> Component.test.tsx
ls frontend/src/**/*.test.tsx
ls frontend/src/**/*.test.ts
```

## Phase 2: RED - Write Failing Test

**This is the most important phase. Do not skip it.**

Track with TodoWrite:

- [ ] Write test that reproduces the bug
- [ ] Run test to confirm it FAILS
- [ ] Verify failure message matches expected bug behavior

### Test Writing Guidelines

**Backend (Go):**
```go
func TestFunctionName_BugDescription(t *testing.T) {
    // Arrange: Set up the conditions that trigger the bug

    // Act: Execute the code path

    // Assert: Verify correct behavior (this will FAIL initially)
}
```

**Frontend (TypeScript/Vitest):**
```typescript
describe('ComponentOrFunction', () => {
  it('should handle the specific bug case', () => {
    // Arrange

    // Act

    // Assert (this will FAIL initially)
  });
});
```

### Run the Test to Confirm Failure

**Backend:**
```bash
cd backend && go test -v -run TestFunctionName ./path/to/package/...
```

**Frontend:**
```bash
cd frontend && npm test -- --run -t "test description"
```

**The test MUST fail before proceeding.** If it passes, either:
- The bug doesn't exist (investigate further)
- The test doesn't correctly reproduce the bug (fix the test)

## Phase 3: GREEN - Implement the Fix

Track with TodoWrite:

- [ ] Write minimal code to fix the bug
- [ ] Run the specific test to see it PASS
- [ ] Run all related tests to check for regressions

### Implementation Guidelines

- Make the **smallest possible change** to fix the bug
- Do NOT refactor unrelated code
- Do NOT add features beyond the bug fix
- Do NOT change function signatures unless necessary

### Run Tests

**Backend:**
```bash
# Run specific test
cd backend && go test -v -run TestFunctionName ./path/to/package/...

# Run all tests in package
cd backend && go test -v ./path/to/package/...

# Run full test suite (with Docker for integration tests)
cd backend && DOCKER_HOST=unix:///Users/santaclaude/.orbstack/run/docker.sock go test ./...
```

**Frontend:**
```bash
# Run all tests
cd frontend && npm run build && npm run lint && npm test
```

## Phase 4: REFACTOR (Optional)

Only if the fix introduced code that could be cleaner:

Track with TodoWrite:

- [ ] Identify refactoring opportunities
- [ ] Refactor while keeping tests green
- [ ] Run full test suite after refactoring

**Skip this phase if the fix is already clean.**

## Phase 5: Code Review (MANDATORY)

**Do NOT commit or push until this phase is complete.**

Perform an extremely thorough code review of ALL changes before even considering commit and push. This is a critical gate - bugs that slip through here go to production.

Track with TodoWrite:

- [ ] Review every line of changed code
- [ ] Verify the fix is correct and complete
- [ ] Check for edge cases not covered by tests
- [ ] Look for unintended side effects
- [ ] Ensure code style matches the codebase
- [ ] Verify no debugging code was left behind

### Code Review Checklist

**Correctness:**
- [ ] Does the fix actually solve the reported bug?
- [ ] Are all edge cases handled?
- [ ] Could this fix break other functionality?
- [ ] Is the logic sound under all conditions?

**Completeness:**
- [ ] Are there other places with the same bug pattern?
- [ ] Should similar fixes be applied elsewhere?
- [ ] Is the test comprehensive enough?
- [ ] Are error cases tested?

**Quality:**
- [ ] Is the code readable and self-documenting?
- [ ] Are variable/function names clear?
- [ ] Is the solution the simplest possible?
- [ ] Does it follow existing patterns in the codebase?

**Safety:**
- [ ] No SQL injection vulnerabilities?
- [ ] No XSS vulnerabilities?
- [ ] No sensitive data exposed?
- [ ] No race conditions introduced?

**Cleanliness:**
- [ ] No console.log/fmt.Println debugging left behind?
- [ ] No commented-out code?
- [ ] No TODO comments for things that should be done now?
- [ ] No unrelated formatting changes?

### How to Review

1. **Read the diff carefully** - Use `git diff` to see exactly what changed
2. **Read surrounding context** - Understand the code around your changes
3. **Trace the code path** - Mentally execute the fix with different inputs
4. **Question everything** - If something seems odd, investigate
5. **Sleep on it** - For complex fixes, consider reviewing again later

```bash
# View all changes
git diff

# View changes with more context
git diff -U10

# View staged changes
git diff --cached
```

### Red Flags to Watch For

- **Magic numbers** - Should they be constants?
- **Copy-pasted code** - Should it be abstracted?
- **Long functions** - Should they be broken up?
- **Deep nesting** - Can it be flattened?
- **Implicit assumptions** - Are they documented or validated?

**If you find ANY issues during review, go back and fix them before proceeding.**

## Phase 6: Finalize

Only proceed here after Phase 5 (Code Review) is complete and you are confident in the fix.

Track with TodoWrite:

- [ ] Run full test suite one final time
- [ ] Update Linear ticket with fix summary
- [ ] Commit changes (if user requests)

### Full Test Suite

```bash
# Backend
cd backend && DOCKER_HOST=unix:///Users/santaclaude/.orbstack/run/docker.sock go test ./...

# Frontend
cd frontend && npm run build && npm run lint && npm test
```

### Linear Ticket Update

Add a comment to the ticket with:
- What was the root cause
- What test was added
- What code was changed
- How to verify the fix

## Common Pitfalls to Avoid

1. **Writing the fix before the test** - This defeats TDD
2. **Writing a test that passes immediately** - Test must fail first
3. **Over-engineering the fix** - Keep changes minimal
4. **Skipping the refactor phase entirely** - At least consider it
5. **Not running the full test suite** - Regressions happen
6. **Rushing to commit** - Always do a thorough code review first
7. **Skipping code review because tests pass** - Tests don't catch everything
8. **Reviewing your own code too quickly** - Take time, be critical

## When to Use This Skill

- Bug reports from Linear tickets
- Bugs discovered during development
- Regressions found in testing
- User-reported issues

## When NOT to Use This Skill

- New features (use a feature skill instead)
- Major refactoring (use codebase-maintenance)
- Security vulnerabilities (may need faster response)
