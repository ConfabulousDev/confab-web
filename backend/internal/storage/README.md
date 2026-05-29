# storage

S3/MinIO object storage client for session file chunks: upload, download, list, merge, and delete operations.

## Files

| File | Role |
|------|------|
| `s3.go` | `S3Storage` struct, `NewS3Storage` constructor, core operations (`Download`, `Delete`), provider-aware chunk operations (`UploadChunk`, `ListChunks`, `DeleteAllSessionChunks`), the shared `chunkPrefix` builder, error classification (`classifyStorageError`), sentinel errors, and safety constants (`MaxChunksPerFile`, `MaxAgentFiles`) |
| `chunks.go` | Chunk processing: `ParseChunkKey`, `DownloadAndMergeChunks`, `DownloadChunks` (parallel with bounded concurrency), `MergeChunks` (line-based dedup with overlap handling), and internal helpers (`splitLines`, `ChunkInfo` type) |

## Key Types

- **`S3Storage`** -- Wraps a MinIO client and bucket name. All operations go through this struct.
- **`S3Config`** -- Configuration: endpoint, credentials, bucket name, SSL flag.
- **`ChunkInfo`** -- Parsed chunk metadata (key, first/last line numbers) plus downloaded content.

## Key API

All chunk methods take a `provider string` argument (one of `models.ProviderClaudeCode` or `models.ProviderCodex`, defined in `internal/models/provider.go`). The provider becomes a segment of every S3 key so that the same `(userID, externalID)` pair under two different agents resolves to two distinct subtrees. Storage validates the provider value via `validation.ValidateProvider` before touching S3 — passing an unknown or legacy value (e.g. `"Claude Code"`) errors out immediately. Callers reading from the DB get the canonical value via `db/session`'s `VerifySessionOwnership` / `GetSessionOwnerExternalIDAndProvider` so no further normalization is needed.

- **`NewS3Storage(config)`** -- Creates a MinIO client and verifies the bucket exists. Fails fast if the bucket is missing.
- **`UploadChunk(ctx, userID, provider, externalID, fileName, firstLine, lastLine, data)`** -- Uploads a chunk with a deterministic key: `{userID}/{provider}/{externalID}/chunks/{fileName}/chunk_{first:08d}_{last:08d}.jsonl`.
- **`ListChunks(ctx, userID, provider, externalID, fileName)`** -- Lists all chunk keys for a file under the named provider, sorted lexicographically (correct order due to zero-padded names). Returns `ErrTooManyChunks` if the count exceeds `MaxChunksPerFile`.
- **`DownloadAndMergeChunks(ctx, userID, provider, externalID, fileName)`** -- Convenience method: lists chunks, downloads in parallel, merges with overlap handling. Returns nil for files with no chunks.
- **`DownloadChunks(ctx, chunkKeys)`** -- Downloads chunks in parallel with bounded concurrency (`maxParallelDownloads = 10`). Skips unparseable keys with a warning.
- **`MergeChunks(chunks)`** -- Merges chunks into a single byte slice using line-indexed array. Handles overlapping line ranges (last write wins). Logs warnings for conflicting content on overlaps and for large merges (> 1M lines).
- **`DeleteAllSessionChunks(ctx, userID, provider, externalID)`** -- Deletes all chunks under a session's provider-scoped prefix. Chunks written under a different provider for the same `(userID, externalID)` are untouched.
- **`ParseChunkKey(key)`** -- Extracts first/last line numbers from a chunk S3 key. Opaque to the provider segment.

## How to Extend

1. **New file type in S3**: Follow the key pattern `{userID}/{provider}/{externalID}/{purpose}/{fileName}/...`. Add upload/list/delete methods mirroring the chunk pattern; reuse `chunkPrefix` or write a sibling helper so the path format lives in one place.
2. **New provider**: Add the canonical constant to `internal/validation/input.go` and update `ValidateProvider`. Storage rejects anything else.
3. **Adjusting concurrency**: Change `maxParallelDownloads` in `chunks.go`. The current value (10) balances throughput against S3 connection limits.
4. **Adjusting safety limits**: `MaxChunksPerFile` (30,000) and `MaxMergeLines` (10,000,000) can be tuned based on observed usage patterns.

## Invariants

- Chunk keys include the canonical provider segment (`claude-code` or `codex`). The path is `{userID}/{provider}/{externalID}/chunks/{fileName}/chunk_{first:08d}_{last:08d}.jsonl`. Storage rejects legacy `"Claude Code"` and any non-canonical value.
- Chunk keys use zero-padded 8-digit line numbers to ensure lexicographic sort equals numeric sort.
- `ListChunks` enforces `MaxChunksPerFile` as a hard limit to prevent unbounded memory from listing.
- `MergeChunks` enforces `MaxMergeLines` to prevent memory exhaustion from corrupted chunk filenames.
- The bucket must exist before `NewS3Storage` is called; the server will not auto-create buckets.
- Error classification maps MinIO errors to sentinel errors: `ErrObjectNotFound`, `ErrAccessDenied`, `ErrNetworkError`, `ErrTooManyChunks`.
- `MergeChunks` uses "last write wins" for overlapping line ranges. It logs warnings when overlapping chunks have different content for the same line, but does not fail.

## Design Decisions

- **Line-indexed merge**: `MergeChunks` allocates an array indexed by line number and writes each chunk's lines into it. This handles arbitrary overlaps correctly at the cost of allocating for the full line range. The `MaxMergeLines` limit bounds this allocation.
- **Bounded parallel downloads**: Uses a semaphore channel pattern with `maxParallelDownloads` slots to limit concurrent S3 connections without spawning unbounded goroutines.
- **Error classification**: `classifyStorageError` translates MinIO-specific errors into domain sentinel errors so callers don't need to import MinIO types. Network errors are detected by string matching as a fallback.
- **Chunk count as estimate**: The DB `chunk_count` column is an estimate that can drift. The read path (in the session package) self-heals by comparing against the actual S3 chunk list.
- **No auto-bucket-creation**: Buckets are infrastructure; they should be created out-of-band (e.g., by Terraform or docker-compose) to avoid accidental bucket creation with wrong permissions.

## Testing

- Unit tests: `chunks_test.go` (ParseChunkKey, MergeChunks), `s3_test.go` (`containsAny`, `classifyStorageError`, sentinel errors, `UploadChunk` bounds and provider validation).
- Integration tests: `s3_integration_test.go` exercises real S3 round-trips through `testutil`'s MinIO container — `UploadChunk`/`Download`, missing-key classification, `ListChunks` ordering, `Delete`, `DeleteAllSessionChunks` (session-scoped, cross-provider scoping, and empty-prefix no-op), `DeleteAllUserData` (cross-user `{userID}/` substring-boundary scoping and empty-prefix no-op), and `NewS3Storage` with a missing bucket.

## Dependencies

- `github.com/minio/minio-go/v7` -- S3-compatible object storage client
- `go.opentelemetry.io/otel` -- Distributed tracing
- `log/slog` -- Structured logging for large-merge warnings and overlap diagnostics
