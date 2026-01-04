---
allowed-tools: Bash(git status:*), Bash(git diff:*), Bash(git log:*), Bash(git add:*), Bash(git commit:*), Bash(git push:*)
description: Commit conversation changes and push to remote
---

Commit changes from the current conversation and push to the current branch.

## Steps

1. Run `git status` to see what files have changed
2. Review the conversation history to identify which files you modified (via Edit, Write, or Bash commands)
3. Cross-reference: categorize changed files into:
   - **Session files**: Files you modified in this conversation
   - **Other files**: Files with changes not from this conversation
4. Run `git diff` on session files to see the changes
5. Run `git log -3 --oneline` to see recent commit message style
6. Stage only session files with `git add <file1> <file2> ...`
   - If there are other files with uncommitted changes, mention them to the user but do NOT stage them
7. Create a commit with a clear, descriptive message following the repo's commit style
8. Push to the current branch

## Handling Edge Cases

- **No session files changed**: If `git status` shows changes but none are from this conversation, inform the user and ask what they want to commit
- **All changes are session files**: Proceed normally
- **Mix of session and other files**: Stage only session files, note the others in your response

## Commit Message Format

Follow the existing commit message style in this repo. End the commit message with:

```
🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude <noreply@anthropic.com>
```

Use a HEREDOC for the commit message to ensure proper formatting.
