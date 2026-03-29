-- ═══════════════════════════════════════════════════════════════════════════
-- Transcript Schema Contract: Frontend ↔ Backend Parsing
--
-- Both the Go backend and TypeScript frontend parse JSONL transcript
-- files. Each line has a type field that determines the schema.
-- If the parsers disagree, data is silently dropped or misinterpreted.
--
-- Invariants to discover:
-- 1. Both parsers must accept the same set of line types
-- 2. message.id deduplication must agree
-- 3. Token usage extraction must produce the same totals
-- 4. Unknown line types should be handled gracefully (not crash)
-- ═══════════════════════════════════════════════════════════════════════════

namespace Spec.TranscriptSchema

-- ── Domain types ────────────────────────────────────────────────────────

/-- Known transcript line types.
    DISCOVERY: The backend defines types in transcript_types.go.
    The frontend defines types in schemas/transcript.ts.
    These lists must match. If the backend adds a new type that the
    frontend doesn't handle, the frontend silently skips those lines.
    Transcript data is still served, but analytics based on those
    lines will differ between frontend and backend.

    The client-error reporting mechanism (POST /api/v1/client-errors)
    is designed to catch this drift, but it's fire-and-forget and
    depends on users actually viewing sessions in the browser. -/
inductive LineType where
  | user
  | assistant
  | system
  | summary
  | fileHistorySnapshot
  | queueOperation
  | prLink
  | unknown (raw : String)  -- forward compatibility
deriving Repr, BEq

/-- A transcript message with content blocks.
    DISCOVERY: The message.id field is shared across multiple JSONL lines
    (content blocks). The backend deduplicates by message.id via
    AssistantMessageGroups(). The frontend must do the same deduplication
    when counting tokens. If one deduplicates and the other doesn't,
    token counts diverge (double-counting). -/
structure MessageID where
  raw : String
deriving Repr, BEq

structure TokenUsage where
  inputTokens  : Nat
  outputTokens : Nat
deriving Repr

structure TranscriptLine where
  lineType  : LineType
  messageID : Option MessageID
  usage     : Option TokenUsage
  model     : Option String
deriving Repr

-- ── Message deduplication ───────────────────────────────────────────────

/-- Group lines by message.id. Lines with the same ID are content blocks
    of a single assistant response.
    DISCOVERY: The backend groups in AssistantMessageGroups() by iterating
    lines and grouping consecutive lines with the same message.id.
    Key assumption: lines with the same message.id are CONTIGUOUS in
    the JSONL file. If they're not (e.g., interleaved with other
    message types), the grouping breaks and tokens are double-counted.
    This contiguity assumption is enforced by Claude Code's output
    format, not by any code check. Unguarded. -/
def groupByMessageID (lines : List TranscriptLine) : List (MessageID × List TranscriptLine) :=
  sorry -- grouping logic

/-- After deduplication, each message group contributes tokens once.
    DISCOVERY: The backend takes usage from the FIRST line of each group
    (or the line with usage set, if only one has it). The frontend
    may handle this differently. If they disagree on which line's
    usage to use, token totals diverge. -/
def extractTokenUsage (group : List TranscriptLine) : Option TokenUsage :=
  group.findSome? fun l => l.usage

-- ── Validation contract ─────────────────────────────────────────────────

/-- Frontend validation reports errors back to backend.
    DISCOVERY: The error reporting has a max of 50 errors per report.
    If a transcript has >50 validation errors, only the first 50 are
    reported. This is fine for observability but means systematic
    schema drift might be under-reported. -/
def maxClientErrors : Nat := 50

/-- Unknown line type handling: both parsers must skip gracefully.
    DISCOVERY: The backend's ParseTranscriptLine returns a TranscriptLine
    with an empty Type for unknown types (still included in the file).
    The frontend's Zod schema uses z.union with a catchall. Both are
    designed for forward compatibility. But if an "unknown" line has
    token usage, the backend includes it in token counts while the
    frontend might skip it. This would cause divergence. -/
theorem unknown_type_graceful (line : TranscriptLine) :
    match line.lineType with
    | .unknown _ => True  -- should not crash, should not count tokens
    | _ => True := by
  cases line.lineType <;> trivial

-- ── Sync file contract ──────────────────────────────────────────────────

/-- Transcript files are fetched via the sync file API.
    DISCOVERY: The API returns chunks merged by the storage layer.
    The merge uses last-write-wins for overlapping ranges. If the
    frontend requests a file with lineOffset, it gets lines starting
    from that offset. But the offset is line-number based, and the
    storage layer's merge may have gaps (missing line ranges).
    A gap in the merged output means the frontend's line numbering
    shifts, potentially misaligning with the backend's line numbering
    in analytics computation. -/
structure SyncFileRequest where
  sessionID  : String
  fileName   : String
  lineOffset : Option Nat  -- resume from this line
deriving Repr

/-- Line numbering must be consistent between frontend requests
    and backend storage. -/
theorem line_numbering_consistent
    (req : SyncFileRequest)
    (mergedLines : List (Option String))  -- None = gap
    : True := by
  sorry

end Spec.TranscriptSchema
