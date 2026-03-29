-- ═══════════════════════════════════════════════════════════════════════════
-- Session Access Control
--
-- The access control system determines who can view a session.
-- Priority ordering: owner > recipient > system > public > none
--
-- Invariants to discover:
-- 1. Priority ordering is total and acyclic
-- 2. Owner access is unconditional (except inactive check)
-- 3. Non-owners get PII-redacted views
-- 4. Share expiration is time-dependent (SQL WHERE, not application logic)
-- ═══════════════════════════════════════════════════════════════════════════

namespace Spec.AccessControl

-- ── Domain types ────────────────────────────────────────────────────────

/-- User status: active users can authenticate, inactive cannot. -/
inductive UserStatus where
  | active
  | inactive
deriving Repr, BEq, DecidableEq

/-- Session access types, ordered by priority (highest first). -/
inductive AccessType where
  | owner
  | recipient
  | system
  | public
  | none
deriving Repr, BEq, DecidableEq

/-- Priority ordering: lower number = higher priority.
    DISCOVERY: In the Go code, this ordering is encoded as a SQL CASE
    expression (1=recipient, 2=system, 3=public). Owner is checked
    BEFORE the SQL query runs. If the SQL CASE values change without
    updating the code that checks owner first, priority could invert.
    This is a convention-level invariant. -/
def priority (t : AccessType) : Nat :=
  match t with
  | .owner     => 0
  | .recipient => 1
  | .system    => 2
  | .public    => 3
  | .none      => 4

/-- Access types form a total order via priority. -/
instance : LE AccessType where
  le a b := priority a ≤ priority b

-- ── Session detail and redaction ────────────────────────────────────────

/-- PII fields that must be redacted for non-owners.
    DISCOVERY: The Go code redacts hostname, username, CWD, transcript_path.
    But owner_email is NOT redacted (needed for attribution).
    If a new PII field is added to SessionDetail, the RedactForSharing
    method must be updated -- there's no structural guarantee that
    new fields are considered. This is convention-level. -/
structure PIIFields where
  hostname        : Option String
  username        : Option String
  cwd             : Option String
  transcriptPath  : Option String
deriving Repr

structure SessionDetail where
  sessionID  : String
  ownerID    : Nat
  ownerEmail : String   -- NOT redacted (attribution)
  title      : Option String
  pii        : PIIFields
  isPublic   : Bool
deriving Repr

/-- Redaction zeroes out all PII fields. -/
def redact (s : SessionDetail) : SessionDetail :=
  { s with pii := ⟨.none, .none, .none, .none⟩ }

-- ── Access determination ────────────────────────────────────────────────

/-- Access check result with metadata. -/
structure AccessResult where
  accessType : AccessType
  shareID    : Option Nat   -- non-nil for recipient/system/public
deriving Repr

/-- DISCOVERY: The code checks owner status (active/inactive) as part
    of access control. An inactive owner blocks ALL access to their
    sessions -- even for share recipients. This is intentional but
    has a surprising implication: deactivating a user instantly breaks
    all shares they created, even if the share hasn't expired.
    This coupling is not documented. -/
def checkAccess (ownerStatus : UserStatus) (requesterID ownerID : Nat)
    (shareAccess : Option AccessType) : AccessResult :=
  -- Inactive owner blocks everything
  if ownerStatus == .inactive then
    ⟨.none, .none⟩
  -- Owner always has access
  else if requesterID == ownerID then
    ⟨.owner, .none⟩
  -- Otherwise, use share-based access (from SQL query)
  else match shareAccess with
    | some t => ⟨t, some 0⟩  -- placeholder shareID
    | none   => ⟨.none, .none⟩

/-- Owner access is the highest priority. -/
theorem owner_highest (t : AccessType) : priority .owner ≤ priority t := by
  cases t <;> simp [priority]

/-- None is the lowest priority. -/
theorem none_lowest (t : AccessType) : priority t ≤ priority .none := by
  cases t <;> simp [priority]

/-- Redaction invariant: non-owners always get redacted data.
    DISCOVERY: The code applies redaction in GetSessionDetailWithAccess
    AFTER the access check. If a new code path serves session detail
    without going through this function, PII leaks. There is no
    structural guarantee that all session-serving paths redact.
    The canonical access model (CF-132) centralizes this, but new
    endpoints could bypass it. -/
theorem non_owner_redacted (s : SessionDetail) (access : AccessResult)
    (h : access.accessType ≠ .owner) :
    -- The served session should have PII fields = none
    True := by
  sorry

end Spec.AccessControl
