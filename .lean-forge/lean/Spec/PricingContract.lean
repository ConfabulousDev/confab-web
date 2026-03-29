-- ═══════════════════════════════════════════════════════════════════════════
-- Pricing Contract: Frontend ↔ Backend
--
-- Both the Go backend and TypeScript frontend maintain independent
-- model pricing tables. When computing token costs, both must agree.
--
-- Invariants to discover:
-- 1. Model family extraction must agree between Go and TypeScript
-- 2. Pricing tables must contain the same models with same values
-- 3. Cost calculation formula must be identical
-- 4. Decimal serialization: Go decimal.Decimal → JSON string → JS parseFloat
-- ═══════════════════════════════════════════════════════════════════════════

namespace Spec.PricingContract

-- ── Domain types ────────────────────────────────────────────────────────

/-- Per-million-token pricing for a model family. -/
structure ModelPricing where
  inputPerMillion      : Nat   -- cents * 100 (avoid floating point)
  outputPerMillion     : Nat
  cacheWritePerMillion : Nat
  cacheReadPerMillion  : Nat
deriving Repr, BEq

/-- Token usage for a single message. -/
structure TokenUsage where
  inputTokens      : Nat
  outputTokens     : Nat
  cacheWriteTokens : Nat
  cacheReadTokens  : Nat
deriving Repr

/-- A model name as it appears in transcript data.
    Example: "claude-opus-4-5-20251101" -/
structure ModelName where
  raw : String
deriving Repr

/-- A model family extracted from a full model name.
    Example: "opus-4-5" from "claude-opus-4-5-20251101" -/
structure ModelFamily where
  raw : String
deriving Repr, BEq

-- ── Model family extraction ─────────────────────────────────────────────

/-- Extract model family from full model name.
    DISCOVERY: The Go code strips "claude-" prefix, then strips date
    suffix (last segment after "-" if it's 8+ digits). The TypeScript
    code does something similar but with different regex.

    If the extraction logic disagrees, the same message gets priced
    differently on backend vs frontend. The system still works --
    it just shows wrong costs. Pure silent corruption.

    Furthermore: unknown models fall through to a default/zero pricing.
    Go logs a warning but returns zero cost. TypeScript returns 0.
    Neither surfaces this to the user -- unknown model = free. -/
opaque extractFamily (name : ModelName) : ModelFamily := sorry

-- ── Pricing table contract ──────────────────────────────────────────────

/-- A pricing table maps model families to pricing.
    DISCOVERY: The Go and TypeScript tables are defined independently
    in different files:
      - Go:         backend/internal/analytics/pricing.go (modelPricingTable)
      - TypeScript:  frontend/src/utils/tokenStats.ts (MODEL_PRICING)
    There is NO shared source of truth. A developer adding a new model
    must update both files. CLAUDE.md documents this, but nothing enforces it.
    This is convention-level enforcement. -/
def PricingTable := List (ModelFamily × ModelPricing)

/-- Two pricing tables agree if they contain exactly the same entries. -/
def tablesAgree (backend frontend : PricingTable) : Prop :=
  ∀ (family : ModelFamily),
    (backend.lookup family) = (frontend.lookup family)

-- ── Cost calculation ────────────────────────────────────────────────────

/-- Calculate cost in micro-dollars (avoid floating point).
    cost = (input × inputRate + output × outputRate + ...) / 1_000_000
    DISCOVERY: Go uses decimal.Decimal (arbitrary precision).
    TypeScript uses IEEE 754 float64. For typical session costs
    ($0.01 to $100), float64 is sufficient. But the serialization
    path is: Go decimal.Decimal → .String() → JSON string →
    TypeScript parseFloat(). If the decimal has more than ~15
    significant digits, parseFloat loses precision.
    This is validated-level (Zod checks string type). -/
def calculateCost (pricing : ModelPricing) (usage : TokenUsage) : Nat :=
  (usage.inputTokens * pricing.inputPerMillion +
   usage.outputTokens * pricing.outputPerMillion +
   usage.cacheWriteTokens * pricing.cacheWritePerMillion +
   usage.cacheReadTokens * pricing.cacheReadPerMillion)

/-- FAST MODE INVARIANT: Fast mode costs 6x the normal rate.
    DISCOVERY: Both Go and TypeScript hardcode the 6x multiplier.
    If one changes without the other, fast-mode costs diverge.
    This is another instance of the duplicated-constant problem. -/
def fastModeMultiplier : Nat := 6

/-- Cost with fast mode applied. -/
def calculateCostWithFastMode (pricing : ModelPricing) (usage : TokenUsage) (isFast : Bool) : Nat :=
  let base := calculateCost pricing usage
  if isFast then base * fastModeMultiplier else base

-- ── Serialization contract ──────────────────────────────────────────────

/-- The backend serializes costs as decimal strings.
    The frontend parses them as float64.
    DISCOVERY: Zod validates that the field is a string (not a number).
    This catches the case where the backend accidentally sends a JSON
    number instead of a string. But it does NOT catch precision loss
    from parseFloat. This is validated-level for format, convention-level
    for precision. -/
theorem cost_serialization_roundtrip (cost : Nat) :
    -- Nat → decimal string → parseFloat → approximately equal
    True := by
  sorry

end Spec.PricingContract
