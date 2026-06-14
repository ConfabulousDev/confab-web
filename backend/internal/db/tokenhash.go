package db

import (
	"crypto/sha256"
	"fmt"
)

// HashToken returns the hex-encoded SHA-256 of a high-entropy token. It is the
// single hashing primitive for tokens stored hashed at rest — API keys,
// web-session IDs, and device codes — so a read of the database (backup leak,
// SQLi, replica, support tooling) cannot replay them.
//
// No salt: these are 192/256-bit cryptographically-random values, so
// brute-forcing the preimage is infeasible, and per-token salt would break the
// single-indexed exact-match lookup these tables rely on. Lives in package db
// (not auth) so both auth (API keys) and db/dbauth (sessions, device codes) can
// share it without an import cycle (auth imports dbauth).
func HashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return fmt.Sprintf("%x", sum)
}
