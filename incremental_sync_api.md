# Incremental sync API

## Motivation

To improve reliability and latency of session saves, we will use a background daemon architecture on the client machine.

On SessionStart event, we will start local daemon process in the background, which will periodically and incrementally sync
session data to the backend.

This doc describes the backend API for supporting such interactions.

## Build brand new APIs (away from previous save / upload paths)

We will develop the new APIs, whilst not affecting existing full file upload paths.

## Endpoints

Exact endpoint naming TBD.

Notes:
- Create a new session for given claude code session id.  If a session id for that external session id already exists, return that session id.  Otherwise create a new session in confab, and return that id.  End result is the same, the daemon knows the confab session id to sync to from that point.

- It should indicate the latest JSONL line number that has been synced already.  For a brand new session, the line number will be 0.

- Sync the latest incremental chunk of JSONL lines.  The POST request will contain: JSONL records, First line number.  For the initial sync, line number will be 1.


- New session read endpoint - this one should read all chunks from S3 and concatenate them correctly, returning a complete session JSONL file.

- New delete session endpoint - this one should delete from DB and also delete all <user>/<claude_code_session_id> (i.e. including all chunks).

## Backend storage

### S3

<user>/<claude_code_session_id>/files/<file_basename>/chunk_0000001_0000100.jsonl (lowest line number, highest line number).

### DB

sessions / files table can be used in the same way as today.











