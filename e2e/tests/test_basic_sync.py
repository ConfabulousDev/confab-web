"""Test basic sync functionality."""

import time
from pathlib import Path

from conftest import BackendClient, run_claude


def test_basic_sync(backend: BackendClient, project_dir: Path) -> None:
    """Test that a Claude Code session is synced to the backend.

    Steps:
    1. Run a Claude Code prompt that takes long enough for sync daemon
    2. Wait for sync daemon to upload (configured for 2s interval)
    3. Verify session appears in backend via API

    Note: The prompt must keep Claude running long enough for the sync daemon
    to initialize with the backend (~3-4 seconds). Simple prompts exit too fast.
    We use a prompt that triggers tool use, which takes longer.
    """
    # Run Claude Code with a prompt that triggers tool use
    # This takes longer, giving the sync daemon time to initialize and upload
    # The sync interval is 2s, so we need Claude to run for at least 6-8 seconds
    # after the transcript file appears for daemon to init and sync
    result = run_claude(
        "Run 'sleep 15 && echo done' and tell me the result",
        cwd=project_dir,
        max_turns=2,
        allowed_tools=["Bash(*)"],
    )

    assert result.session_id, "Should have a session ID"
    print(f"Session ID: {result.session_id[:8]}...")

    # Wait for sync daemon to upload
    # Sync interval is 2s, give it some buffer
    time.sleep(5)

    # Verify session exists in backend
    session = backend.wait_for_session(result.session_id, timeout=10)

    assert session["external_id"] == result.session_id
    assert "files" in session
    assert len(session["files"]) >= 1, "Should have at least one file"

    # Check transcript file has synced lines
    transcript_files = [f for f in session["files"] if f["file_type"] == "transcript"]
    assert len(transcript_files) == 1, "Should have exactly one transcript file"

    transcript = transcript_files[0]
    assert transcript["last_synced_line"] >= 1, "Should have synced at least 1 line"

    print(f"Transcript synced with {transcript['last_synced_line']} lines")
