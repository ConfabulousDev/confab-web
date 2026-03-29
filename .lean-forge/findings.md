# Formalization Findings

## Discovered Invariants

### 1. Chunk key zero-padding upper bound
**Discovery:** Defining `LineNumber` as a positive `Nat` forced the question: what's the valid range? The Go code uses `%08d` formatting, which means lexicographic == numeric ordering only for values < 10^8. No code checks this bound at key generation time.
**Current enforcement:** unguarded
**Lean evidence:** `ChunkStorage.maxZeroPaddedLine` and `zero_pad_ordering` theorem requiring explicit bounds
**Risk:** If a corrupted external_id or runaway session exceeds 99,999,999 lines, chunk keys sort incorrectly → merge produces silently wrong transcript data. MaxMergeLines (10M) provides a downstream cap but is checked AFTER download, not during key generation.

### 2. PII redaction completeness
**Discovery:** Defining `PIIFields` as an explicit structure revealed that adding a new PII field to `SessionDetail` requires updating `RedactForSharing()`. There's no structural guarantee (e.g., a separate PII struct that must be constructed) that new fields are considered.
**Current enforcement:** convention
**Lean evidence:** `AccessControl.PIIFields` structure vs `SessionDetail` — the redact function must enumerate all PII fields manually
**Risk:** A developer adds a new sensitive field (e.g., `ip_address`) to SessionDetail but forgets to redact it → PII leaks to share recipients.

### 3. Analytics card list consistency
**Discovery:** Formalizing `allValid` revealed that three separate code locations must agree on the card list: `AllValid()`, `GetCards()`, and `UpsertCards()`. Adding a new card type requires updating all three. No shared constant or type enforces this.
**Current enforcement:** convention
**Lean evidence:** `AnalyticsCards.allValid` function with hardcoded card type strings — the match must be exhaustive
**Risk:** New card type added to compute + upsert but not to AllValid → card is computed and stored but never detected as stale → perpetually serves initial computation, even after version bump.

### 4. Frontend-backend pricing table sync
**Discovery:** Defining `PricingTable` and `tablesAgree` forced explicit acknowledgment that two independent tables must be identical. The Lean spec couldn't express "these two files in different languages contain the same data" — that's the invariant gap.
**Current enforcement:** convention (documented in CLAUDE.md but not enforced)
**Lean evidence:** `PricingContract.tablesAgree` proposition — can't be proved without shared source of truth
**Risk:** New model added to backend but not frontend → frontend shows $0.00 cost for that model's sessions. System still works, just shows wrong numbers.

### 5. Message ID contiguity assumption
**Discovery:** Formalizing `groupByMessageID` required specifying whether grouping is by consecutive runs or by global ID. The backend uses consecutive-run grouping. If JSONL lines with the same message.id are non-contiguous (interleaved), tokens get double-counted.
**Current enforcement:** unguarded (relies on Claude Code's output format)
**Lean evidence:** `TranscriptSchema.groupByMessageID` — the grouping semantics depend on input ordering, which is not validated
**Risk:** A future Claude Code version interleaves content blocks → backend double-counts tokens → inflated cost and token analytics.

### 6. Smart recap optimistic lock timeout
**Discovery:** Defining `SmartRecapState` with an explicit lock timeout parameter revealed the timeout is a single constant (300s). If the generator takes longer than 300s, a second generator can start concurrently. If it crashes, the lock blocks for 300s.
**Current enforcement:** validated (timeout check exists, but timeout value is a magic number)
**Lean evidence:** `AnalyticsCards.canAcquireLock` — the correctness depends on the relationship between typical generation time and timeout value
**Risk:** Concurrent smart recap generation wastes API credits and could produce conflicting results.

### 7. Decimal serialization precision
**Discovery:** Defining cost as `Nat` (lossless) in Lean highlighted that the actual path is: Go `decimal.Decimal` → `.String()` → JSON → TypeScript `parseFloat()`. IEEE 754 float64 has ~15 significant digits. Typical costs are fine, but the path is not precision-preserving in general.
**Current enforcement:** validated (Zod checks string type, but not precision)
**Lean evidence:** `PricingContract.cost_serialization_roundtrip` — can't be proved for arbitrary precision
**Risk:** Very large sessions with costs > $999,999.99 could lose precision. Low practical risk but architecturally unsound.

## Lean Spec Summary
- Files: ChunkStorage.lean, AccessControl.lean, AnalyticsCards.lean, PricingContract.lean, TranscriptSchema.lean
- `lake build`: pass (0 errors, 9 sorry warnings as expected, 2 unused variable warnings)
- sorry count: 9 (all proofs are sorry — this is expected)
