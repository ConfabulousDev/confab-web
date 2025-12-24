"""Test transcript content verification - local file vs backend API."""

import json
import time
from pathlib import Path

from conftest import BackendClient, find_local_transcript, read_local_transcript, run_claude


def test_transcript_content_matches_backend(backend: BackendClient, project_dir: Path) -> None:
    """Test that backend transcript content matches local file exactly.

    This is the authoritative test for sync correctness:
    1. Run a Claude session that generates transcript content
    2. Wait for sync to complete
    3. Read the local transcript file
    4. Fetch transcript from backend API
    5. Compare line by line - they should match exactly
    """
    # Run Claude with a prompt that generates some output
    result = run_claude(
        "Run 'sleep 12 && echo verification_test' and show me the result",
        cwd=project_dir,
        max_turns=2,
        allowed_tools=["Bash(*)"],
    )

    assert result.session_id, "Should have a session ID"
    print(f"Session ID: {result.session_id[:8]}...")

    # Wait for sync to complete
    time.sleep(8)

    # Get internal session ID
    internal_id = backend.lookup_session_by_external_id(result.session_id)
    assert internal_id, "Session should exist in backend"

    # Wait for some lines to sync
    session = backend.wait_for_sync_lines(result.session_id, min_lines=1, timeout=20)
    synced_lines = 0
    for f in session["files"]:
        if f["file_type"] == "transcript":
            synced_lines = f["last_synced_line"]
            break

    print(f"Backend reports {synced_lines} synced lines")

    # Read local transcript
    local_content = read_local_transcript(result.session_id)
    assert local_content is not None, "Local transcript should exist"

    local_lines = [line for line in local_content.strip().split("\n") if line]
    print(f"Local transcript has {len(local_lines)} lines")

    # Fetch backend transcript
    backend_content = backend.get_transcript_content(internal_id)
    assert backend_content is not None, "Backend should return transcript content"

    backend_lines = [line for line in backend_content.strip().split("\n") if line]
    print(f"Backend returned {len(backend_lines)} lines")

    # Compare synced portion
    # Backend may have fewer lines if sync is still in progress
    lines_to_compare = min(len(backend_lines), synced_lines)
    assert lines_to_compare > 0, "Should have at least one line to compare"

    print(f"Comparing {lines_to_compare} lines...")

    for i in range(lines_to_compare):
        local_line = local_lines[i]
        backend_line = backend_lines[i]

        # Parse as JSON to compare structure (ignoring whitespace differences)
        try:
            local_obj = json.loads(local_line)
            backend_obj = json.loads(backend_line)

            assert local_obj == backend_obj, (
                f"Line {i + 1} mismatch:\n"
                f"  Local:   {json.dumps(local_obj)[:100]}...\n"
                f"  Backend: {json.dumps(backend_obj)[:100]}..."
            )
        except json.JSONDecodeError as e:
            # Fall back to string comparison if not valid JSON
            assert local_line == backend_line, (
                f"Line {i + 1} mismatch (raw):\n"
                f"  Local:   {local_line[:100]}...\n"
                f"  Backend: {backend_line[:100]}...\n"
                f"  JSON error: {e}"
            )

    print(f"All {lines_to_compare} lines match!")


def test_transcript_path_discoverable(backend: BackendClient, project_dir: Path) -> None:
    """Test that we can find the local transcript file.

    This validates our transcript discovery logic works correctly.
    """
    result = run_claude(
        "Run 'sleep 10 && echo path_test'",
        cwd=project_dir,
        max_turns=2,
        allowed_tools=["Bash(*)"],
    )

    assert result.session_id, "Should have a session ID"

    # Find the transcript
    transcript_path = find_local_transcript(result.session_id)
    assert transcript_path is not None, "Should find local transcript"
    assert transcript_path.exists(), "Transcript path should exist"
    assert transcript_path.name == f"{result.session_id}.jsonl", (
        f"Should be {result.session_id}.jsonl"
    )

    print(f"Found transcript at: {transcript_path}")

    # Verify it has content
    content = transcript_path.read_text()
    assert len(content) > 0, "Transcript should have content"

    lines = content.strip().split("\n")
    print(f"Transcript has {len(lines)} lines")

    # First line should be valid JSON with a type field
    first_obj = json.loads(lines[0])
    assert "type" in first_obj, "First line should have type field"
    print(f"First line type: {first_obj['type']}")


def test_full_transcript_sync_after_completion(backend: BackendClient, project_dir: Path) -> None:
    """Test that full transcript is synced after Claude session completes.

    After Claude exits, the sync daemon should complete uploading all remaining
    content. This test verifies the final state matches.
    """
    result = run_claude(
        "Run 'sleep 8 && echo completion_sync_test'",
        cwd=project_dir,
        max_turns=2,
        allowed_tools=["Bash(*)"],
    )

    assert result.session_id, "Should have a session ID"
    assert result.exit_code == 0, "Claude should exit cleanly"

    # Give extra time for final sync after session ends
    time.sleep(12)

    # Read local transcript (final state)
    local_content = read_local_transcript(result.session_id)
    assert local_content is not None, "Local transcript should exist"

    local_lines = [line for line in local_content.strip().split("\n") if line]
    print(f"Local transcript has {len(local_lines)} total lines")

    # Get internal session ID
    internal_id = backend.lookup_session_by_external_id(result.session_id)
    assert internal_id, "Session should exist in backend"

    # Get session details
    session = backend.get_session(internal_id)
    assert session is not None, "Session should be accessible"

    # Check synced line count
    synced_lines = 0
    for f in session["files"]:
        if f["file_type"] == "transcript":
            synced_lines = f["last_synced_line"]
            break

    print(f"Backend synced {synced_lines} of {len(local_lines)} lines")

    # Backend should have synced most/all lines
    # Allow some tolerance for timing
    sync_ratio = synced_lines / len(local_lines) if local_lines else 0
    assert sync_ratio >= 0.8, (
        f"Expected at least 80% of lines synced, got {sync_ratio:.1%} "
        f"({synced_lines}/{len(local_lines)})"
    )

    # Fetch and compare content
    backend_content = backend.get_transcript_content(internal_id)
    assert backend_content is not None, "Backend should return content"

    backend_lines = [line for line in backend_content.strip().split("\n") if line]

    # Compare all synced lines
    for i in range(min(len(backend_lines), len(local_lines))):
        local_obj = json.loads(local_lines[i])
        backend_obj = json.loads(backend_lines[i])
        assert local_obj == backend_obj, f"Line {i + 1} mismatch"

    print(f"Verified {len(backend_lines)} lines match between local and backend")
