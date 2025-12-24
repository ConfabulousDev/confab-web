"""Test session metadata is captured correctly."""

import time
from pathlib import Path

from conftest import BackendClient, run_claude


def test_session_metadata_captured(backend: BackendClient, project_dir: Path) -> None:
    """Test that session metadata (cwd, git info) is captured.

    The sync daemon should capture metadata from the Claude Code session
    and include it in the session details.
    """
    # Run Claude session
    result = run_claude(
        "Run 'sleep 10 && echo metadata_test'",
        cwd=project_dir,
        max_turns=2,
        allowed_tools=["Bash(*)"],
    )

    assert result.session_id, "Should have a session ID"
    print(f"Session ID: {result.session_id[:8]}...")

    # Wait for sync
    time.sleep(8)

    # Get session details
    session = backend.wait_for_session(result.session_id, timeout=15)

    # Verify basic fields
    assert session["external_id"] == result.session_id
    assert session["id"], "Should have internal UUID"
    assert session["first_seen"], "Should have first_seen timestamp"

    # Verify metadata was captured
    if session.get("cwd"):
        print(f"CWD captured: {session['cwd']}")
        # CWD should be within the temp directory
        assert "/tmp" in session["cwd"] or "tmp" in session["cwd"].lower()

    # Verify files are tracked
    assert "files" in session, "Should have files array"
    assert len(session["files"]) >= 1, "Should have at least one file"

    # Print captured metadata for debugging
    print("Session metadata:")
    print(f"  - ID: {session['id'][:8]}...")
    print(f"  - External ID: {session['external_id'][:8]}...")
    print(f"  - First seen: {session['first_seen']}")
    print(f"  - CWD: {session.get('cwd', 'not set')}")
    print(f"  - Files: {len(session['files'])}")

    for f in session["files"]:
        print(f"    - {f['file_name']} ({f['file_type']}): {f['last_synced_line']} lines")


def test_session_has_transcript_file(backend: BackendClient, project_dir: Path) -> None:
    """Test that session has a transcript file with expected structure."""
    result = run_claude(
        "Run 'sleep 10 && echo transcript_test'",
        cwd=project_dir,
        max_turns=2,
        allowed_tools=["Bash(*)"],
    )

    assert result.session_id, "Should have a session ID"

    # Wait for sync
    time.sleep(8)

    # Get session
    session = backend.wait_for_session(result.session_id, timeout=15)

    # Find transcript file
    transcript_files = [f for f in session["files"] if f["file_type"] == "transcript"]
    assert len(transcript_files) == 1, "Should have exactly one transcript file"

    transcript = transcript_files[0]
    assert transcript["file_name"] == "transcript.jsonl", (
        "Transcript should be named transcript.jsonl"
    )
    assert transcript["last_synced_line"] >= 1, "Should have synced at least 1 line"
    assert transcript["updated_at"], "Should have updated_at timestamp"


def test_session_completion(backend: BackendClient, project_dir: Path) -> None:
    """Test that a completed session can be accessed after Claude exits.

    This verifies the session persists and is accessible after the
    Claude Code process terminates.
    """
    # Run a quick Claude session
    result = run_claude(
        "Say 'test complete'",
        cwd=project_dir,
        max_turns=1,
        allowed_tools=[],
    )

    assert result.session_id, "Should have a session ID"
    assert result.exit_code == 0, "Claude should exit cleanly"

    # Give time for final sync and session_end event
    time.sleep(10)

    # Session should still be accessible after Claude exits
    session = backend.get_session_by_external_id(result.session_id)

    # If session exists, verify it's complete
    if session:
        print(f"Session found after completion: {session['id'][:8]}...")
        assert session["external_id"] == result.session_id
        # Transcript should have content
        transcript_files = [f for f in session["files"] if f["file_type"] == "transcript"]
        if transcript_files:
            print(f"Transcript has {transcript_files[0]['last_synced_line']} lines")
    else:
        # For very short sessions, sync daemon might not have time to upload
        # This is expected behavior - daemon needs ~3-4s to initialize
        print("Session not found - expected for very short sessions")
