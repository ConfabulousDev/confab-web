package analytics

import "context"

// SetTitleRecomputeBeforeWriteHook installs a callback that RecomputeSessionTitle
// invokes after it has read the session and derived a value but before its guarded
// UPDATE. Tests use it to simulate a concurrent ingest write or re-invalidation
// landing in that window. Returns a restore func.
func SetTitleRecomputeBeforeWriteHook(fn func(ctx context.Context, sessionID string)) (restore func()) {
	prev := titleRecomputeBeforeWrite
	titleRecomputeBeforeWrite = fn
	return func() { titleRecomputeBeforeWrite = prev }
}
