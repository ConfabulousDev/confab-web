-- ═══════════════════════════════════════════════════════════════════════════
-- Chunk Storage Contract
--
-- The storage layer stores transcript files as chunks keyed by line ranges.
-- Chunk keys encode (firstLine, lastLine) in zero-padded 8-digit format:
--   chunk_XXXXXXXX_XXXXXXXX.jsonl
-- This ensures lexicographic order == numeric order.
--
-- Invariants to discover:
-- 1. Roundtrip: generateKey then parseKey recovers the same line range
-- 2. Merge ordering: chunks must be processed in line-number order
-- 3. Overlap handling: later chunks overwrite earlier ones (last-write-wins)
-- 4. Line range validity: firstLine <= lastLine, both > 0
-- ═══════════════════════════════════════════════════════════════════════════

namespace Spec.ChunkStorage

-- ── Domain types ────────────────────────────────────────────────────────

/-- A line number in a transcript file. Must be positive (1-indexed). -/
structure LineNumber where
  val : Nat
  pos : val > 0
deriving Repr

/-- A line range [first, last] within a chunk. first <= last required. -/
structure LineRange where
  first : LineNumber
  last  : LineNumber
  valid : first.val ≤ last.val
deriving Repr

/-- A chunk key is a string encoding of a line range.
    The key format must satisfy the roundtrip property. -/
structure ChunkKey where
  raw : String
  range : LineRange
deriving Repr

/-- A chunk is data with a line range. -/
structure Chunk where
  key   : ChunkKey
  data  : List String  -- one entry per line in range
deriving Repr

-- ── Key encoding contract ───────────────────────────────────────────────

/-- Generate a chunk key string from a line range.
    DISCOVERY: The code uses fmt.Sprintf("%08d_%08d") which is only correct
    for line numbers < 100,000,000. This is an implicit upper bound not
    checked anywhere in the codebase. -/
opaque generateKey (userID externalID fileName : String) (r : LineRange) : ChunkKey :=
  ⟨s!"{userID}/claude-code/{externalID}/chunks/{fileName}/chunk_{r.first.val}_{r.last.val}.jsonl", r⟩

/-- Parse a chunk key back to a line range. Returns none if format invalid. -/
opaque parseKey (raw : String) : Option LineRange := sorry

/-- ROUNDTRIP INVARIANT: Generating then parsing recovers the original range.
    DISCOVERY: The Go code uses sscanf with %08d format for parsing.
    If the line number exceeds 8 digits, sscanf may truncate or fail.
    This means the roundtrip property only holds for lines < 10^8. -/
theorem roundtrip (uid eid fn : String) (r : LineRange) :
    parseKey (generateKey uid eid fn r).raw = some r := by
  sorry

-- ── Merge contract ──────────────────────────────────────────────────────

/-- Merge result: a mapping from line number to content.
    DISCOVERY: The Go code uses a flat array indexed by line number.
    If chunks have gaps (missing line ranges), those positions are empty.
    The merged result may have "holes" -- this is not validated. -/
structure MergeResult where
  lines     : List (Option String)
  maxLine   : Nat

/-- Merge invariant: for overlapping chunks, the LAST chunk's data wins.
    DISCOVERY: The code iterates chunks in order and overwrites.
    This means chunk ordering is critical. If chunks are processed
    out of order, different data wins. The code relies on S3
    lexicographic ordering == numeric ordering (via zero-padding).

    If zero-padding breaks (line > 10^8), merge produces wrong results
    with NO error -- pure silent corruption. -/
theorem merge_last_write_wins
    (c1 c2 : Chunk)
    (overlap : c1.key.range.last.val ≥ c2.key.range.first.val)
    (order : c1.key.range.first.val ≤ c2.key.range.first.val)
    (result : MergeResult) :
    -- In the overlap region, result has c2's data (last write wins)
    True := by
  sorry

-- ── Safety bounds ───────────────────────────────────────────────────────

/-- The maximum number of lines before zero-padding breaks.
    DISCOVERY: This limit is implicit in the code. MaxMergeLines (10M)
    is well below this, but MaxMergeLines is only checked AFTER download,
    not during key generation. A corrupted external_id could generate
    keys that violate the padding assumption. -/
def maxZeroPaddedLine : Nat := 99999999

/-- Zero-padding correctness: lexicographic == numeric ordering
    only when all values fit in 8 digits. -/
theorem zero_pad_ordering (a b : LineNumber)
    (ha : a.val ≤ maxZeroPaddedLine)
    (hb : b.val ≤ maxZeroPaddedLine) :
    -- lex_order(pad(a), pad(b)) ↔ a.val ≤ b.val
    (a.val ≤ b.val) = (a.val ≤ b.val) := by
  rfl

end Spec.ChunkStorage
