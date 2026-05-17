# Incremental Sync API

The incremental sync flow is documented in [`API.md`](API.md) under the sync endpoints (`POST /api/v1/sync/init`, `POST /api/v1/sync/chunk`, `POST /api/v1/sync/event`).

For implementation details, see [`internal/api/sync.go`](internal/api/sync.go) and the chunk merging logic in [`internal/storage/chunks.go`](internal/storage/chunks.go).
