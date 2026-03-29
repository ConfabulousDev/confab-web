-- ═══════════════════════════════════════════════════════════════════════════
-- Analytics Card Staleness Contract
--
-- Analytics cards are cached computation results. Each card has:
-- - A version number (bumped when compute logic changes)
-- - An "up_to_line" watermark (which transcript line was last processed)
--
-- A card is valid iff version matches current AND up_to_line matches
-- the session's current total_lines.
--
-- Invariants to discover:
-- 1. All 7 regular cards must agree on staleness (AllValid checks all)
-- 2. Smart recap has different staleness rules (time-based + optimistic lock)
-- 3. Version bump MUST invalidate all existing cards of that type
-- 4. Card data types in API response must match compute output types
-- ═══════════════════════════════════════════════════════════════════════════

namespace Spec.AnalyticsCards

-- ── Domain types ────────────────────────────────────────────────────────

/-- A card version number. Monotonically increasing. -/
structure Version where
  val : Nat
deriving Repr, BEq

/-- Line count watermark: how many transcript lines were processed. -/
structure LineCount where
  val : Nat
deriving Repr, BEq

/-- A generic analytics card record. -/
structure CardRecord where
  cardType   : String
  version    : Version
  upToLine   : LineCount
  computedAt : Nat      -- unix timestamp
deriving Repr

/-- The current expected versions for each card type.
    DISCOVERY: These are Go constants in the analytics package.
    If a version is bumped but the upsert logic doesn't use the new
    version, cards will never be detected as stale -- they'll be
    perpetually "valid" at the old version. The version constant
    and the upsert logic must agree. This is convention-level. -/
structure CardVersions where
  tokens       : Version
  session      : Version
  tools        : Version
  codeActivity : Version
  conversation : Version
  agentsSkills : Version
  redactions   : Version
deriving Repr

-- ── Staleness check ─────────────────────────────────────────────────────

/-- A card is valid iff version matches AND line count matches. -/
def isValid (card : CardRecord) (expectedVersion : Version) (currentLines : LineCount) : Bool :=
  card.version == expectedVersion && card.upToLine == currentLines

/-- AllValid: all 7 regular cards must be valid.
    DISCOVERY: In the Go code, AllValid checks all 7 cards. If a new
    card type is added but AllValid is not updated, the precomputer
    won't detect staleness for the new card. The card list in AllValid
    must match the card list in GetCards and UpsertCards.
    This is a cross-function convention invariant. -/
def allValid (cards : List CardRecord) (versions : CardVersions) (currentLines : LineCount) : Bool :=
  cards.all fun c =>
    let expectedVersion := match c.cardType with
      | "tokens"        => versions.tokens
      | "session"       => versions.session
      | "tools"         => versions.tools
      | "code_activity" => versions.codeActivity
      | "conversation"  => versions.conversation
      | "agents_skills" => versions.agentsSkills
      | "redactions"    => versions.redactions
      | _               => ⟨0⟩  -- unknown card type → always invalid
    isValid c expectedVersion currentLines

-- ── Smart recap (different staleness rules) ─────────────────────────────

/-- Smart recap uses optimistic locking to prevent concurrent generation.
    DISCOVERY: The lock is a timestamp field (computing_started_at).
    If the generator crashes mid-computation, the lock is orphaned.
    There's a timeout (lockTimeoutSeconds) but if it's too short,
    two generators could run concurrently. If too long, a crashed
    generator blocks recap for that duration. The timeout value
    must balance these risks. Currently 300s (5 min). -/
structure SmartRecapState where
  version           : Version
  upToLine          : LineCount
  computingStarted  : Option Nat  -- unix timestamp, None = not computing
  lockTimeout       : Nat         -- seconds
deriving Repr

def canAcquireLock (state : SmartRecapState) (now : Nat) : Bool :=
  match state.computingStarted with
  | none => true
  | some started => now - started > state.lockTimeout

-- ── API response contract ───────────────────────────────────────────────

/-- DISCOVERY: The analytics API returns both legacy flat fields AND
    new cards map. The legacy fields (Tokens, Cost, Compaction) are
    derived from the new card data. If the derivation logic diverges
    from the card compute logic, the same session shows different
    numbers in different UI views. Both must derive from the same
    ComputeResult. -/
structure AnalyticsResponse where
  -- Legacy flat fields
  legacyTokens     : Option Nat
  legacyCost       : Option String
  legacyCompaction : Option Nat
  -- New card-based fields
  cards            : List CardRecord
  cardErrors       : List (String × String)  -- card type → error message
deriving Repr

/-- Legacy/card consistency: legacy fields must derive from card data.
    DISCOVERY: If legacy fields are computed independently from card
    fields, they can diverge. The code should derive legacy from cards,
    not compute both independently. -/
theorem legacy_card_consistency
    (resp : AnalyticsResponse)
    (tokensCard : CardRecord) :
    -- resp.legacyTokens should equal tokensCard's token count
    True := by
  sorry

end Spec.AnalyticsCards
