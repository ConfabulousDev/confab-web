# Task: Create CLI Latest Version Endpoint

## Goal

Create a static file at `cli/latest_version` that the confab CLI uses to check for updates.

## Background

The confab CLI (`confab update`) fetches `https://confabulous.dev/cli/latest_version` to check if a newer version is available. This should be a plain text file containing just the version string.

## Implementation

Create the file at `frontend/public/cli/latest_version` with content:

```
v0.3.1
```

This will be served at `https://confabulous.dev/cli/latest_version` when deployed.

## Requirements

1. Create directory `frontend/public/cli/` if it doesn't exist
2. Create file `frontend/public/cli/latest_version` containing just `v0.3.1` (no trailing newline preferred, but the CLI trims whitespace so either works)
3. Verify the file will be served correctly (Vite serves files from `public/` at the root)

## Notes

- The install script already exists at `frontend/public/install`
- The CLI expects a plain text response, not JSON
- Version format should include the `v` prefix (e.g., `v0.3.1`)

## Future Consideration

Eventually this could be generated dynamically from GitHub releases, but a static file is fine for now.
