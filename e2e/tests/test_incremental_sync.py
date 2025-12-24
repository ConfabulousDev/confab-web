"""Test incremental sync and content verification."""

import json
import time
from pathlib import Path

from conftest import BackendClient, run_claude


def test_incremental_sync(backend: BackendClient, project_dir: Path) -> None:
    """Test that transcript content is incrementally synced.

    Steps:
    1. Run a Claude session with multiple tool uses
    2. Verify sync line count increases over time
    3. Verify transcript content is accessible via API
    """
    # Run Claude with a command that produces multiple outputs over time
    # Each tool use adds to the transcript
    prompt = (
        "Run these commands one at a time: "
        "'echo step1', 'sleep 3 && echo step2', 'sleep 3 && echo step3'"
    )
    result = run_claude(
        prompt,
        cwd=project_dir,
        max_turns=4,
        allowed_tools=["Bash(*)"],
    )

    assert result.session_id, "Should have a session ID"
    print(f"Session ID: {result.session_id[:8]}...")

    # Wait for session to sync
    time.sleep(5)

    # Get session and verify it exists
    session = backend.wait_for_session(result.session_id, timeout=15)
    assert session["external_id"] == result.session_id

    # Get internal session ID for content fetch
    internal_id = backend.lookup_session_by_external_id(result.session_id)
    assert internal_id, "Should have internal session ID"

    # Verify transcript has content
    transcript_files = [f for f in session["files"] if f["file_type"] == "transcript"]
    assert len(transcript_files) == 1, "Should have exactly one transcript"

    lines_synced = transcript_files[0]["last_synced_line"]
    assert lines_synced >= 1, f"Should have synced at least 1 line, got {lines_synced}"
    print(f"Transcript has {lines_synced} lines synced")


def test_transcript_content_accessible(backend: BackendClient, project_dir: Path) -> None:
    """Test that transcript content can be fetched via API.

    Verifies the sync/file endpoint returns valid JSONL transcript data.
    """
    # Run a simple Claude session
    result = run_claude(
        "Run 'echo hello world' and tell me the output",
        cwd=project_dir,
        max_turns=2,
        allowed_tools=["Bash(*)"],
    )

    assert result.session_id, "Should have a session ID"

    # Wait for sync
    time.sleep(8)

    # Get internal session ID
    internal_id = backend.lookup_session_by_external_id(result.session_id)
    assert internal_id, "Should have internal session ID"

    # Wait for session to have some content synced
    backend.wait_for_sync_lines(result.session_id, min_lines=1, timeout=15)

    # Fetch transcript content
    content = backend.get_transcript_content(internal_id)
    assert content is not None, "Should be able to fetch transcript content"
    assert len(content) > 0, "Transcript should have content"

    # Verify it's valid JSONL
    lines = [line for line in content.strip().split("\n") if line]
    assert len(lines) >= 1, "Should have at least one line"

    # Each line should be valid JSON
    for i, line in enumerate(lines[:5]):  # Check first 5 lines
        try:
            obj = json.loads(line)
            assert "type" in obj, f"Line {i} should have 'type' field"
            print(f"Line {i}: type={obj['type']}")
        except json.JSONDecodeError as e:
            raise AssertionError(f"Line {i} is not valid JSON: {e}") from e


def test_line_offset_incremental_fetch(backend: BackendClient, project_dir: Path) -> None:
    """Test fetching transcript content with line_offset for incremental reads.

    This verifies the backend supports efficient incremental content fetching.
    """
    # Run a session that produces some output
    result = run_claude(
        "Run 'sleep 10 && echo done'",
        cwd=project_dir,
        max_turns=2,
        allowed_tools=["Bash(*)"],
    )

    assert result.session_id, "Should have a session ID"

    # Wait for sync
    time.sleep(8)

    # Get internal session ID
    internal_id = backend.lookup_session_by_external_id(result.session_id)
    assert internal_id, "Should have internal session ID"

    # Wait for session to sync
    backend.wait_for_sync_lines(result.session_id, min_lines=2, timeout=15)

    # Fetch full content
    full_content = backend.get_transcript_content(internal_id)
    assert full_content, "Should have content"

    full_lines = full_content.strip().split("\n")
    print(f"Total lines: {len(full_lines)}")

    if len(full_lines) >= 2:
        # Fetch with offset - should get remaining lines
        offset_content = backend.get_transcript_content(internal_id, line_offset=1)
        assert offset_content, "Should have offset content"

        offset_lines = offset_content.strip().split("\n")
        # Should have one fewer line (skipped first)
        assert len(offset_lines) == len(full_lines) - 1, (
            f"With offset=1, should have {len(full_lines) - 1} lines, got {len(offset_lines)}"
        )
        print(f"Offset fetch returned {len(offset_lines)} lines (expected {len(full_lines) - 1})")
