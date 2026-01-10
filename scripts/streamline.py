#!/usr/bin/env python3
"""
Streamline Claude Code session transcripts for summarization.

Reduces transcript size by 20-30x by:
- Dropping non-essential record types (file-history-snapshot, queue-operation)
- Dropping thinking blocks entirely
- Replacing tool_result content with status only
- Simplifying tool_use to just tool name + key params
- Stripping all metadata overhead
- Collapsing character-by-character user input

Usage:
    python streamline.py input.jsonl > output.jsonl
    python streamline.py input.jsonl -o output.jsonl
    cat input.jsonl | python streamline.py > output.jsonl
"""

import json
import sys
import argparse
from typing import TextIO, Optional


def extract_user_text(content: list) -> Optional[str]:
    """Extract user text from content array, collapsing char-by-char input."""
    text_parts = []
    for item in content:
        if isinstance(item, str):
            text_parts.append(item)
        elif isinstance(item, dict) and item.get("type") == "text":
            text_parts.append(item.get("text", ""))

    text = "".join(text_parts).strip()
    return text if text else None


def simplify_tool_use(tool_use: dict) -> dict:
    """Simplify tool_use to essential fields only."""
    name = tool_use.get("name", "unknown")
    inp = tool_use.get("input", {})
    tool_id = tool_use.get("id", "")

    result = {"tool": name, "id": tool_id}

    # Extract key parameter based on tool type
    if name == "Read":
        result["target"] = inp.get("file_path", "")
    elif name == "Edit":
        result["target"] = inp.get("file_path", "")
    elif name == "Write":
        result["target"] = inp.get("file_path", "")
    elif name == "Bash":
        cmd = inp.get("command", "")
        # Truncate long commands
        result["cmd"] = cmd[:200] + "..." if len(cmd) > 200 else cmd
    elif name == "Grep":
        result["pattern"] = inp.get("pattern", "")[:100]
        if inp.get("path"):
            result["path"] = inp.get("path")
    elif name == "Glob":
        result["pattern"] = inp.get("pattern", "")
    elif name == "Task":
        result["desc"] = inp.get("description", "")
        result["agent"] = inp.get("subagent_type", "")
    elif name == "TodoWrite":
        todos = inp.get("todos", [])
        result["todos"] = [t.get("content", "")[:50] for t in todos[:5]]
    elif name.startswith("mcp__"):
        # MCP tools - just capture the key params
        result["params"] = {k: str(v)[:100] for k, v in list(inp.items())[:3]}
    else:
        # Generic fallback - capture first few params
        for key in ["file_path", "path", "query", "url", "id"]:
            if key in inp:
                result[key] = str(inp[key])[:200]
                break

    return result


def process_tool_result(tool_result: dict, tool_uses: dict) -> dict:
    """Convert tool_result to status-only format."""
    tool_id = tool_result.get("tool_use_id", "")
    is_error = tool_result.get("is_error", False)

    result = {"id": tool_id, "ok": not is_error}

    # If error, include first part of error message
    if is_error:
        content = tool_result.get("content", "")
        if isinstance(content, str):
            result["error"] = content[:200]
        elif isinstance(content, list) and content:
            first = content[0]
            if isinstance(first, dict):
                result["error"] = first.get("text", "")[:200]
            elif isinstance(first, str):
                result["error"] = first[:200]

    return result


def process_assistant_message(content: list, tool_uses: dict) -> list:
    """Process assistant message content, extracting text and tool_use."""
    outputs = []

    for item in content:
        if not isinstance(item, dict):
            continue

        item_type = item.get("type")

        if item_type == "text":
            text = item.get("text", "").strip()
            if text:
                outputs.append({"type": "assistant", "text": text})

        elif item_type == "tool_use":
            simplified = simplify_tool_use(item)
            outputs.append({"type": "tool_use", **simplified})
            # Track for matching with results
            tool_uses[item.get("id", "")] = simplified

        # Skip thinking blocks entirely

    return outputs


def process_user_message(content: list, tool_uses: dict) -> list:
    """Process user message content, extracting text and tool results."""
    outputs = []

    # First, collect any text (may be char-by-char)
    user_text = extract_user_text(content)
    if user_text:
        outputs.append({"type": "user", "text": user_text})

    # Then process tool results
    for item in content:
        if isinstance(item, dict) and item.get("type") == "tool_result":
            result = process_tool_result(item, tool_uses)
            outputs.append({"type": "result", **result})

    return outputs


def streamline(input_file: TextIO, output_file: TextIO) -> dict:
    """Process transcript and write streamlined output."""
    stats = {
        "input_lines": 0,
        "input_bytes": 0,
        "output_lines": 0,
        "output_bytes": 0,
        "dropped_records": 0,
    }

    tool_uses = {}  # Track tool_use id -> simplified info

    for line in input_file:
        stats["input_lines"] += 1
        stats["input_bytes"] += len(line)

        try:
            record = json.loads(line)
        except json.JSONDecodeError:
            continue

        record_type = record.get("type")

        # Drop non-essential record types
        if record_type in ("file-history-snapshot", "queue-operation"):
            stats["dropped_records"] += 1
            continue

        # Process based on record type
        if record_type == "assistant":
            content = record.get("message", {}).get("content", [])
            outputs = process_assistant_message(content, tool_uses)
        elif record_type == "user":
            content = record.get("message", {}).get("content", [])
            outputs = process_user_message(content, tool_uses)
        else:
            # Unknown type - skip
            stats["dropped_records"] += 1
            continue

        # Write outputs
        for output in outputs:
            output_line = json.dumps(output, separators=(",", ":")) + "\n"
            output_file.write(output_line)
            stats["output_lines"] += 1
            stats["output_bytes"] += len(output_line)

    return stats


def main():
    parser = argparse.ArgumentParser(
        description="Streamline Claude Code transcripts for summarization"
    )
    parser.add_argument(
        "input",
        nargs="?",
        help="Input JSONL file (default: stdin)",
    )
    parser.add_argument(
        "-o", "--output",
        help="Output file (default: stdout)",
    )
    parser.add_argument(
        "-s", "--stats",
        action="store_true",
        help="Print stats to stderr",
    )

    args = parser.parse_args()

    # Setup input
    if args.input:
        input_file = open(args.input, "r")
    else:
        input_file = sys.stdin

    # Setup output
    if args.output:
        output_file = open(args.output, "w")
    else:
        output_file = sys.stdout

    try:
        stats = streamline(input_file, output_file)

        if args.stats:
            reduction = stats["input_bytes"] / stats["output_bytes"] if stats["output_bytes"] else 0
            print(f"Input:  {stats['input_lines']} lines, {stats['input_bytes']/1024:.1f}KB", file=sys.stderr)
            print(f"Output: {stats['output_lines']} lines, {stats['output_bytes']/1024:.1f}KB", file=sys.stderr)
            print(f"Reduction: {reduction:.1f}x", file=sys.stderr)
            print(f"Dropped records: {stats['dropped_records']}", file=sys.stderr)
    finally:
        if args.input:
            input_file.close()
        if args.output:
            output_file.close()


if __name__ == "__main__":
    main()
