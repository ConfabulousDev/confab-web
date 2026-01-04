---
allowed-tools: Bash(git status:*), Bash(git diff:*), Bash(git log:*), Bash(git add:*), Bash(git commit:*), Bash(git push:*)
description: Commit all changes and push to remote
---

Commit all staged and unstaged changes, then push to the current branch.

## Steps

1. Run `git status` to see what files have changed
2. Run `git diff` to see the changes (staged and unstaged)
3. Run `git log -3 --oneline` to see recent commit message style
4. Stage all changes with `git add -A`
5. Create a commit with a clear, descriptive message following the repo's commit style
6. Push to the current branch

## Commit Message Format

Follow the existing commit message style in this repo. End the commit message with:

```
🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude <noreply@anthropic.com>
```

Use a HEREDOC for the commit message to ensure proper formatting.
