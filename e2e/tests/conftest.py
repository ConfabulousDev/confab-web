"""Pytest fixtures for E2E tests."""

import json
import os
import subprocess
import tempfile
import time
from collections.abc import Generator
from dataclasses import dataclass
from pathlib import Path
from typing import Any

import pytest
import requests


@dataclass
class ClaudeResult:
    """Result from running Claude Code."""

    session_id: str
    output: list[dict[str, Any]]
    exit_code: int


@dataclass
class BackendClient:
    """Client for interacting with the Confab backend API."""

    base_url: str
    api_key: str

    def get(self, endpoint: str) -> requests.Response:
        """Make a GET request to the backend."""
        return requests.get(
            f"{self.base_url}{endpoint}",
            headers={"Authorization": f"Bearer {self.api_key}"},
            timeout=10,
        )

    def lookup_session_by_external_id(self, external_id: str) -> str | None:
        """Look up internal session ID by external_id. Returns None if not found."""
        resp = self.get(f"/api/v1/sessions/by-external-id/{external_id}")
        if resp.status_code == 200:
            return resp.json().get("session_id")  # type: ignore[no-any-return]
        return None

    def get_session(self, session_id: str) -> dict[str, Any] | None:
        """Get session details via API. Returns None if not found."""
        resp = self.get(f"/api/v1/sessions/{session_id}")
        if resp.status_code == 200:
            return resp.json()  # type: ignore[no-any-return]
        return None

    def get_session_by_external_id(self, external_id: str) -> dict[str, Any] | None:
        """Get session details by external_id. Returns None if not found."""
        internal_id = self.lookup_session_by_external_id(external_id)
        if internal_id is None:
            return None
        return self.get_session(internal_id)

    def wait_for_session(
        self, external_id: str, timeout: float = 30.0, poll_interval: float = 1.0
    ) -> dict[str, Any]:
        """Wait for a session to appear via the API (by external_id)."""
        start = time.time()
        while time.time() - start < timeout:
            session = self.get_session_by_external_id(external_id)
            if session is not None:
                return session
            time.sleep(poll_interval)
        raise TimeoutError(f"Session {external_id} not found after {timeout}s")

    def get_transcript_content(self, session_id: str, line_offset: int = 0) -> str | None:
        """Get transcript file content for a session."""
        endpoint = f"/api/v1/sessions/{session_id}/sync/file?file_name=transcript.jsonl"
        if line_offset > 0:
            endpoint += f"&line_offset={line_offset}"
        resp = self.get(endpoint)
        if resp.status_code == 200:
            return resp.text
        return None

    def wait_for_sync_lines(
        self,
        external_id: str,
        min_lines: int,
        timeout: float = 30.0,
        poll_interval: float = 1.0,
    ) -> dict[str, Any]:
        """Wait for transcript to have at least min_lines synced."""
        start = time.time()
        while time.time() - start < timeout:
            session = self.get_session_by_external_id(external_id)
            if session is not None:
                transcript_files = [
                    f for f in session.get("files", []) if f["file_type"] == "transcript"
                ]
                if transcript_files:
                    lines = transcript_files[0].get("last_synced_line", 0)
                    if lines >= min_lines:
                        return session
            time.sleep(poll_interval)
        raise TimeoutError(
            f"Session {external_id} did not reach {min_lines} lines after {timeout}s"
        )


@pytest.fixture
def backend() -> BackendClient:
    """Fixture providing a backend API client."""
    return BackendClient(
        base_url=os.environ.get("CONFAB_BACKEND_URL", "http://backend:8080"),
        api_key=os.environ.get("CONFAB_API_KEY", ""),
    )


@pytest.fixture
def project_dir() -> Generator[Path, None, None]:
    """Fixture providing a temporary git project directory."""
    with tempfile.TemporaryDirectory() as tmpdir:
        project = Path(tmpdir)

        # Initialize git repo (Claude Code expects this)
        subprocess.run(["git", "init", "-q"], cwd=project, check=True)
        subprocess.run(["git", "config", "user.email", "test@example.com"], cwd=project, check=True)
        subprocess.run(["git", "config", "user.name", "Test User"], cwd=project, check=True)

        # Create initial commit
        readme = project / "README.md"
        readme.write_text("# Test Project\n")
        subprocess.run(["git", "add", "README.md"], cwd=project, check=True)
        subprocess.run(["git", "commit", "-q", "-m", "Initial commit"], cwd=project, check=True)

        yield project


def run_claude(
    prompt: str,
    cwd: Path,
    max_turns: int = 1,
    allowed_tools: list[str] | None = None,
) -> ClaudeResult:
    """Run Claude Code with the given prompt and return the result."""
    if allowed_tools is None:
        allowed_tools = ["Bash(read-only:*)"]

    # ANTHROPIC_MODEL env var controls model selection (set in docker-compose)
    cmd = [
        "claude",
        "-p",
        prompt,
        "--output-format",
        "json",
        "--max-turns",
        str(max_turns),
    ]
    for tool in allowed_tools:
        cmd.extend(["--allowedTools", tool])

    # Use Popen to capture PID for debugging
    proc = subprocess.Popen(
        cmd,
        cwd=cwd,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True,
    )
    print(f"Claude PID: {proc.pid}")
    stdout, stderr = proc.communicate()
    returncode = proc.returncode

    # Parse JSONL output
    output: list[dict[str, Any]] = []
    session_id = ""

    for line in stdout.strip().split("\n"):
        if not line:
            continue
        try:
            obj = json.loads(line)
            output.append(obj)
            if "session_id" in obj and not session_id:
                session_id = obj["session_id"]
        except json.JSONDecodeError:
            pass  # Skip non-JSON lines

    return ClaudeResult(
        session_id=session_id,
        output=output,
        exit_code=returncode,
    )
