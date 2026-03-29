# Improvement Targets

Baseline IES: 0.53 (19/36)
Theoretical max IES: 1.00

## Theoretical Maximum Table

| Check function | Max returnable score | What triggers score 3 |
|---|---|---|
| check_01_chunk_line_bounds | 3 | Custom bounded type for chunk line numbers |
| check_02_pii_redaction_struct | 3 | PII fields in separate embedded struct |
| check_03_card_list_exhaustiveness | 3 | Reflection-based or shared card type list |
| check_04_pricing_table_sync | 3 | Test that reads both Go/TS pricing files |
| check_05_filter_param_validation | 3 | Validator functions with max length on filter params |
| check_06_model_family_extraction | 3 | Cross-language test verifying both implementations |
| check_07_cost_serialization | 3 | Zod transform to precise decimal type |
| check_08_chunk_firstline_validation | 3 | Typed positive integer for firstLine |
| check_09_access_priority_tests | 3 | Typed access enum with full precedence tests |
| check_10_redaction_on_nonowner_access | 3 | Separate SharedSessionView type without PII |
| check_11_card_version_validity | 3 | Generic IsValid function or CardValidator interface |
| check_12_sync_session_ownership | 3 | Typed OwnedSession handle |

## Strategic assessment

Risk concentrates at cross-module contracts (pricing sync, model family extraction, transcript schema) and missing input validation (filter params, chunk line bounds). The highest-value targets are the 3 convention-level invariants (1→2) and the 1 unguarded invariant (0→2), as these represent silent-corruption risks with low implementation cost. The 8 validated invariants could potentially reach structural (2→3) but most would require significant type system changes in Go.

## Phase 1 targets (execute sequentially)

### Target 1: filter_param_valid (0→2)
**Invariant:** Query string filter parameters (repo, branch, owner, pr) must have length/count validation
**Category:** input validation
**Strategy:** validated
**Where:** `grep -n 'parseCommaSeparated' backend/internal/api/sessions_view.go`
**What:** Add max-count and max-element-length checks to parseCommaSeparated or at the handler level. Cap filter arrays at e.g. 50 elements and individual values at 512 chars.
**Test:** Add test in sessions_view_test.go verifying rejection of oversized filter params

### Target 2: chunk_line_bounds (1→2)
**Invariant:** UploadChunk must validate firstLine/lastLine fit in 8-digit zero-padding (< 100,000,000)
**Category:** encoding/serialization
**Strategy:** validated
**Where:** `grep -n 'func.*UploadChunk' backend/internal/storage/s3.go`
**What:** Add bounds check: `if firstLine > 99999999 || lastLine > 99999999 { return error }`. Also add `firstLine > 0 && lastLine >= firstLine` guard.
**Test:** Add test verifying rejection of out-of-bounds line numbers

### Target 3: pii_redaction_struct (1→2)
**Invariant:** Adding a new PII field to SessionDetail must force redaction handling
**Category:** scope/isolation
**Strategy:** validated
**Where:** `grep -n 'RedactForSharing' backend/internal/db/types.go`
**What:** Add a test that uses reflect to enumerate *string fields in SessionDetail, calls RedactForSharing, and verifies all PII-tagged fields are nil. Tag PII fields with a struct tag.
**Test:** Test uses reflection to verify redaction completeness

### Target 4: pricing_table_sync (1→2)
**Invariant:** Backend Go and frontend TypeScript pricing tables must contain same models with same values
**Category:** contract consistency
**Strategy:** validated
**Where:** `grep -rn 'modelPricingTable' backend/internal/analytics/pricing.go` and `grep -n 'MODEL_PRICING' frontend/src/utils/tokenStats.ts`
**What:** Add a Go test that reads the TypeScript file, extracts model families and values via regex, and compares to modelPricingTable.
**Test:** TestPricingTableSync in pricing_test.go

## Phase 2 ideas (explore after Phase 1)
- Promote chunk_line_bounds to structural (3) via bounded LineNumber type
- Promote pii_redaction_struct to structural (3) via embedded PIIFields struct
- Add cross-language model family extraction test for model_family_extract (2→3)
- Add cost serialization roundtrip test or Zod refinement for cost_serialization (2→3)
- Add typed access enum for access_priority_test (2→3)
