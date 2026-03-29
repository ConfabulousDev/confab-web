"""IES evaluation harness for Confab Web.

Read-only. Examines source code and computes IES.

Usage:
    python3 prepare.py
"""
from pathlib import Path
import re

REPO = Path("/Users/jackie/dev/confab-web")
BACKEND = REPO / "backend" / "internal"
FRONTEND = REPO / "frontend" / "src"


def check_01_chunk_line_bounds() -> tuple[int, str]:
    """UploadChunk must validate that firstLine/lastLine fit in 8-digit zero-padding.

    Without bounds checking, line numbers >= 100,000,000 produce keys where
    lexicographic order != numeric order, causing silent merge corruption.
    """
    upload_file = BACKEND / "storage" / "s3.go"
    content = upload_file.read_text()

    # Extract UploadChunk function body
    match = re.search(
        r'func\s+\([^)]+\)\s+UploadChunk\(.*?\n(?=func\s|\Z)',
        content, re.DOTALL
    )
    body = match.group() if match else ""

    # Score 3: Type-level enforcement — a bounded type (e.g., ChunkLineNumber
    # with validation in constructor) that makes invalid line numbers unconstructable.
    # Look for a custom type for line numbers with validation.
    types_content = ""
    for f in (BACKEND / "storage").glob("*.go"):
        types_content += f.read_text()

    if re.search(
        r'type\s+\w*[Ll]ine\w*\s+struct\s*\{.*?(?:max|Max|bound|Bound|validate|Validate)',
        types_content, re.DOTALL
    ):
        return (3, "Custom bounded type for chunk line numbers prevents invalid values at construction")

    # Score 2: Runtime validation — explicit bounds check in UploadChunk before formatting.
    # Look for: if firstLine > X || lastLine > X or similar guard.
    if re.search(
        r'(?:firstLine|lastLine|first_line|last_line)\s*(?:>|>=)\s*(?:99999999|MaxLine|maxLine|MaxZeroPad)',
        body
    ):
        return (2, "UploadChunk validates line numbers fit in 8 digits before formatting key")

    # Also check if there's a general validation function called before upload
    if re.search(r'validate.*[Ll]ine|check.*[Bb]ound|[Bb]ound.*[Cc]heck', body):
        return (2, "UploadChunk calls a line number validation function")

    # Score 1: Convention — comment or constant documents the limit but no runtime check.
    if re.search(r'(?:8.digit|zero.pad|%08d|99999999)', body) and re.search(r'//.*(?:limit|bound|max)', body, re.IGNORECASE):
        return (1, "Comment documents 8-digit padding limit but no runtime check")

    # Score 0: No enforcement — raw fmt.Sprintf with %08d, no validation.
    if re.search(r'%08d', body):
        return (0, "UploadChunk formats with %08d but does not validate line numbers fit in 8 digits")

    return (0, "Could not find UploadChunk or line number formatting")


def check_02_pii_redaction_struct() -> tuple[int, str]:
    """PII fields in SessionDetail must be structurally separated so that
    adding a new PII field forces the developer to handle redaction.

    Currently RedactForSharing manually enumerates 4 fields (hostname,
    username, CWD, transcript_path). A new PII field could be missed.
    """
    types_file = BACKEND / "db" / "types.go"
    content = types_file.read_text()

    # Extract SessionDetail struct
    match = re.search(
        r'type\s+SessionDetail\s+struct\s*\{(.*?)\n\}',
        content, re.DOTALL
    )
    struct_body = match.group(1) if match else ""

    # Extract RedactForSharing function
    match = re.search(
        r'func\s+\([^)]+SessionDetail\)\s+RedactForSharing\(\).*?(?=\nfunc\s|\n//\s|\Z)',
        content, re.DOTALL
    )
    redact_body = match.group() if match else ""

    # Score 3: PII fields live in a separate embedded struct (e.g., PIIFields)
    # and RedactForSharing zeros the entire struct in one operation.
    if re.search(r'type\s+PIIFields\s+struct', content) and re.search(r'PIIFields', struct_body):
        return (3, "PII fields in separate struct — adding a field forces redaction handling")

    # Also check if there's an embedded struct pattern with a clear zeroing
    if re.search(r'\.PII\s*=\s*PIIFields\{\}|\.Pii\s*=', redact_body):
        return (3, "PII fields zeroed via struct assignment — structurally complete")

    # Score 2: RedactForSharing exists with an explicit completeness test
    # (a test that checks all *string PII fields are nil after redaction).
    test_files = list((BACKEND / "db").rglob("*_test.go"))
    test_content = ""
    for f in test_files:
        test_content += f.read_text()

    if re.search(r'RedactForSharing', redact_body) and re.search(
        r'(?:TestRedact|redact).*(?:Hostname|Username|CWD|TranscriptPath).*nil',
        test_content, re.DOTALL
    ):
        return (2, "RedactForSharing exists with test verifying all PII fields are redacted")

    # Score 1: RedactForSharing method exists with comment but no test.
    if re.search(r'RedactForSharing', content) and re.search(
        r'(?:Hostname|Username|CWD|TranscriptPath)\s*=\s*nil', redact_body
    ):
        return (1, "RedactForSharing manually enumerates PII fields; no structural guarantee of completeness")

    # Score 0: No redaction mechanism.
    return (0, "No RedactForSharing method or PII redaction found")


def check_03_card_list_exhaustiveness() -> tuple[int, str]:
    """AllValid(), GetCards(), and UpsertCards() must check/fetch/store
    the same set of card types. If a new card is added to one but not
    the others, staleness detection or persistence breaks silently.
    """
    cards_file = BACKEND / "analytics" / "cards.go"
    store_file = BACKEND / "analytics" / "store.go"
    cards_content = cards_file.read_text()
    store_content = store_file.read_text()

    # Extract Cards struct fields (card record types)
    match = re.search(r'type\s+Cards\s+struct\s*\{(.*?)\n\}', cards_content, re.DOTALL)
    cards_struct = match.group(1) if match else ""
    card_fields = re.findall(r'(\w+)\s+\*\w+CardRecord', cards_struct)

    # Extract AllValid checks
    match = re.search(
        r'func\s+\([^)]+Cards\)\s+AllValid\(.*?\n\}',
        cards_content, re.DOTALL
    )
    allvalid_body = match.group() if match else ""
    allvalid_checks = re.findall(r'c\.(\w+)\.IsValid', allvalid_body)

    # Score 3: A compile-time or generated check ensures all card fields
    # are covered. Look for a go:generate directive, a cardTypes slice
    # built from struct reflection, or a test that uses reflect to check.
    all_test_content = ""
    for f in (BACKEND / "analytics").glob("*_test.go"):
        all_test_content += f.read_text()

    if re.search(r'reflect.*Cards|reflect\.TypeOf.*Cards|go:generate.*card', cards_content + all_test_content):
        return (3, "Card list exhaustiveness enforced via reflection or code generation")

    # Also check for a const/var that lists all card types and is used by all three functions
    if re.search(r'(?:allCardTypes|cardTypeList|CardTypes)\s*=\s*\[', cards_content):
        return (3, "Shared card type list ensures AllValid, GetCards, and UpsertCards agree")

    # Score 2: A test explicitly verifies that AllValid checks the same cards as the struct.
    if re.search(r'Test.*AllValid.*card.*field|Test.*Card.*Exhaustive', all_test_content, re.IGNORECASE):
        return (2, "Test verifies AllValid covers all card fields")

    # Also accept: AllValid field count matches Cards struct field count AND a test exists
    if len(card_fields) > 0 and set(card_fields) == set(allvalid_checks):
        # Fields match, but check if there's any test
        if re.search(r'TestAllValid|Test.*AllValid', all_test_content):
            return (2, f"AllValid checks {len(allvalid_checks)} cards matching struct; covered by test")

    # Score 1: AllValid and Cards struct match but rely on manual sync, no test.
    if len(card_fields) > 0 and set(card_fields) == set(allvalid_checks):
        return (1, f"AllValid checks {len(allvalid_checks)} cards matching struct fields; no exhaustiveness test")

    # Score 0: AllValid doesn't match Cards struct.
    return (0, f"AllValid checks {allvalid_checks} but Cards has {card_fields} — mismatch")


def check_04_pricing_table_sync() -> tuple[int, str]:
    """Backend (Go) and frontend (TypeScript) pricing tables must contain
    the same model families with the same values. Drift means the same
    session shows different costs in different views.
    """
    go_file = BACKEND / "analytics" / "pricing.go"
    ts_file = FRONTEND / "utils" / "tokenStats.ts"
    go_content = go_file.read_text()
    ts_content = ts_file.read_text()

    # Extract Go model families
    go_families = set(re.findall(r'"([\w-]+)":\s*\{?\s*\n?\s*Input:', go_content))
    if not go_families:
        go_families = set(re.findall(r'"([\w-]+)":\s*(?:ModelPricing)?\s*\{', go_content))

    # Extract TS model families
    ts_families = set(re.findall(r"'([\w-]+)':\s*\{", ts_content))

    # Score 3: A shared source of truth (JSON/TOML file) or a CI test that
    # reads both files and compares model sets and values.
    test_files = list((BACKEND / "analytics").glob("*_test.go"))
    test_content = ""
    for f in test_files:
        test_content += f.read_text()

    # Check for a test that explicitly reads the TS/frontend pricing file
    # Must reference the actual frontend file path or MODEL_PRICING constant
    if re.search(r'(?:tokenStats\.ts|MODEL_PRICING|frontend/src/utils/tokenStats)', test_content):
        return (3, "Test reads both pricing tables and verifies they match")

    # Check for a shared pricing data file
    for shared in REPO.glob("**/pricing.json"):
        return (3, f"Shared pricing file: {shared.relative_to(REPO)}")
    for shared in REPO.glob("**/pricing.toml"):
        return (3, f"Shared pricing file: {shared.relative_to(REPO)}")

    # Score 2: Both tables exist and a runtime or build-time check compares them.
    # Look for a script or Makefile target that compares.
    for script in REPO.glob("scripts/*pricing*"):
        return (2, f"Script {script.name} checks pricing table sync")

    # Score 1: Both tables exist and match, but no automated check.
    if go_families and ts_families and go_families == ts_families:
        return (1, f"Both tables have {len(go_families)} models and match; no automated sync check")

    if go_families and ts_families:
        go_only = go_families - ts_families
        ts_only = ts_families - go_families
        if go_only or ts_only:
            return (0, f"Tables diverge: Go-only={go_only}, TS-only={ts_only}")

    # Score 0: Tables don't match or can't be found.
    return (0, "Could not extract or compare pricing tables")


def check_05_filter_param_validation() -> tuple[int, str]:
    """Query string filter parameters (repo, branch, owner, pr) must have
    length validation. Without limits, a client could send megabytes of
    filter values, causing memory exhaustion in the query builder.
    """
    handler_file = BACKEND / "api" / "sessions_view.go"
    content = handler_file.read_text()

    # Extract HandleListSessions body
    match = re.search(
        r'func\s+HandleListSessions\(.*?\nreturn func.*?(?=\n\}\n\}|\Z)',
        content, re.DOTALL
    )
    body = match.group() if match else content

    # Also read parseCommaSeparated
    match_parse = re.search(
        r'func\s+parseCommaSeparated\(.*?(?=\nfunc\s|\Z)',
        content, re.DOTALL
    )
    parse_body = match_parse.group() if match_parse else ""

    # Score 3: Filter params are parsed into a validated type (e.g., a
    # struct with MaxLen tags or a constructor that enforces bounds).
    validation_content = ""
    for f in (BACKEND / "validation").glob("*.go"):
        validation_content += f.read_text()

    if re.search(
        r'(?:ValidateFilter|ValidateRepo|ValidateBranch|ValidateOwner).*(?:MaxLen|maxLen|max_len)',
        validation_content + body, re.DOTALL
    ):
        return (3, "Filter params validated via bounded types or validator functions with max length")

    # Score 2: Explicit length or count checks on filter params before DB query.
    if re.search(
        r'(?:len\(params\.\w+\)|len\(repos\)|len\(branches\)|len\(owners\))\s*(?:>|>=)\s*\d+',
        body + parse_body
    ):
        return (2, "Filter param count/length validated at handler level")

    # Also check for per-element length validation
    if re.search(r'len\(\w+\)\s*>\s*\d+.*(?:repo|branch|owner|filter)', body + parse_body, re.IGNORECASE):
        return (2, "Individual filter values have length bounds")

    # Check for a general query string size limit in middleware
    server_file = BACKEND / "api" / "server.go"
    if server_file.exists():
        server_content = server_file.read_text()
        if re.search(r'(?:MaxQueryLen|QuerySizeLimit|query.*limit|URL.*len)', server_content, re.IGNORECASE):
            return (2, "Query string size limited at middleware level")

    # Score 1: Body size limit exists (withMaxBody) but no query string limit.
    if re.search(r'withMaxBody|MaxBody', content):
        return (1, "Request body limited but query string filter params have no length validation")

    # Score 0: No validation on filter params.
    return (0, "Query string filter params (repo, branch, owner) have no length or count validation")


def check_06_model_family_extraction() -> tuple[int, str]:
    """Go and TypeScript model family extraction must produce the same
    output for all known model names. Different extraction = different
    pricing = silently wrong costs.
    """
    go_file = BACKEND / "analytics" / "pricing.go"
    ts_file = FRONTEND / "utils" / "tokenStats.ts"
    go_content = go_file.read_text()
    ts_content = ts_file.read_text()

    # Score 3: A shared implementation or a cross-language test that
    # verifies both functions produce the same output for known inputs.
    # Must reference BOTH Go and TS implementations (e.g., read TS file from Go test).
    test_content = ""
    for f in (BACKEND / "analytics").glob("*_test.go"):
        test_content += f.read_text()

    if re.search(r'(?:tokenStats\.ts|frontend.*getModelFamily|cross.*lang)', test_content):
        if re.search(r'claude-.*opus|claude-.*sonnet|claude-.*haiku', test_content):
            return (3, "Cross-language test verifies model family extraction for known model names")

    # Score 2: Both implementations exist with unit tests covering the same test cases.
    go_tests = re.findall(r'"(claude-[\w-]+)"', test_content)
    ts_test_content = ""
    for f in (FRONTEND / "utils").glob("*.test.*"):
        ts_test_content += f.read_text()
    ts_tests = re.findall(r'"(claude-[\w-]+)"', ts_test_content)

    if go_tests and ts_tests:
        shared = set(go_tests) & set(ts_tests)
        if len(shared) >= 3:
            return (2, f"Both Go and TS have model family tests; {len(shared)} shared test cases")

    # Check Go tests alone
    if re.search(r'TestGetModelFamily|Test.*ModelFamily|test.*getModelFamily', test_content):
        if re.search(r'claude-.*opus|claude-.*sonnet', test_content):
            return (2, "Go model family extraction has comprehensive tests")

    # Score 1: Both have getModelFamily but no tests or only one side tested.
    go_has = bool(re.search(r'func\s+getModelFamily', go_content))
    ts_has = bool(re.search(r'function\s+getModelFamily', ts_content))
    if go_has and ts_has:
        return (1, "Both Go and TS implement getModelFamily independently; not cross-tested")

    # Score 0: Missing implementation on one side.
    return (0, f"Model family extraction: Go={go_has}, TS={ts_has}")


def check_07_cost_serialization() -> tuple[int, str]:
    """Backend sends cost as decimal string; frontend must validate as
    string (not number). If the backend accidentally sends a JSON number,
    precision loss or type errors occur.
    """
    schema_file = FRONTEND / "schemas" / "api.ts"
    if not schema_file.exists():
        return (0, "Frontend API schema file not found")
    content = schema_file.read_text()

    # Score 3: Frontend uses a custom Zod transform that validates string format
    # AND converts to a precise representation (e.g., a Decimal.js instance).
    # Must be on the SAME field (estimated_usd), not just anywhere in the file.
    # Extract the line(s) defining estimated_usd
    usd_lines = []
    for line in content.split('\n'):
        if 'estimated_usd' in line:
            usd_lines.append(line)
    usd_context = '\n'.join(usd_lines)

    if re.search(r'z\.string\(\).*\.transform.*[Dd]ecimal', usd_context):
        return (3, "Cost validated as string with transform to precise decimal type")

    # Also check for a custom refinement that validates decimal format
    if re.search(r'z\.string\(\).*\.refine', usd_context):
        return (3, "Cost field has string validation with format refinement")

    # Score 2: Zod validates estimated_usd as z.string().
    if re.search(r'estimated_usd.*z\.string\(\)', content) or re.search(r'estimated_usd:\s*z\.string\(\)', content):
        return (2, "Zod validates cost as string type (prevents silent number coercion)")

    # Also check for optional string
    if re.search(r'estimated.*usd.*string', content, re.IGNORECASE):
        return (2, "Cost field validated as string in Zod schema")

    # Score 1: Frontend accepts cost but doesn't validate type.
    if re.search(r'estimated.*usd|estimated_cost', content, re.IGNORECASE):
        return (1, "Cost field exists in schema but no string type validation")

    # Score 0: No cost field in schema.
    return (0, "No estimated_usd field found in frontend schema")


def check_08_chunk_firstline_validation() -> tuple[int, str]:
    """Sync chunk upload must validate firstLine >= 1 (1-indexed).
    A zero or negative firstLine would corrupt the merge array indexing.
    """
    sync_file = BACKEND / "api" / "sync.go"
    content = sync_file.read_text()

    # Extract the chunk upload handler
    match = re.search(
        r'func\s+\([^)]+\)\s+handleSyncChunk\(.*?(?=\nfunc\s|\Z)',
        content, re.DOTALL
    )
    body = match.group() if match else content

    # Score 3: FirstLine is a typed positive integer (e.g., a custom type
    # that cannot be constructed with values < 1).
    all_storage = ""
    for f in (BACKEND / "storage").glob("*.go"):
        all_storage += f.read_text()

    if re.search(r'type\s+\w*[Ff]irstLine\w*\s+(?:struct|int)', all_storage):
        return (3, "FirstLine uses a typed positive integer that prevents invalid values")

    # Score 2: Explicit runtime check: firstLine < 1 → error.
    if re.search(r'(?:FirstLine|first_line|firstLine)\s*(?:<|<=)\s*(?:0|1)', body):
        return (2, "Handler validates firstLine >= 1 before processing")

    # Also match req.FirstLine < 1 pattern
    if re.search(r'req\.FirstLine\s*<\s*1', body):
        return (2, "Handler validates req.FirstLine >= 1 before processing")

    # Score 1: Comment says must be >= 1 but no check.
    if re.search(r'//.*first.*line.*>.*0|//.*1-based|//.*1-indexed', body, re.IGNORECASE):
        return (1, "Comment documents 1-based line numbering but no validation")

    # Score 0: No check.
    return (0, "No firstLine validation found in sync chunk handler")


def check_09_access_priority_tests() -> tuple[int, str]:
    """Access type priority ordering (owner > recipient > system > public)
    must be verified by tests. Incorrect priority = unauthorized access
    or denial.
    """
    test_files = list((BACKEND / "api").glob("*canonical*test*"))
    test_files += list((BACKEND / "api").glob("*access*test*"))
    test_content = ""
    for f in test_files:
        test_content += f.read_text()

    # Score 3: Tests cover all priority transitions AND use an enum or
    # constant for access types (not string literals).
    access_file = BACKEND / "api" / "access.go"
    access_content = access_file.read_text() if access_file.exists() else ""

    # Check for typed enum or iota constants for access types
    db_types = (BACKEND / "db" / "types.go").read_text() if (BACKEND / "db" / "types.go").exists() else ""

    has_typed_enum = bool(re.search(
        r'type\s+SessionAccessType\s+(?:int|uint)', db_types
    )) or bool(re.search(
        r'(?:Owner|Recipient|System|Public)\s*SessionAccessType\s*=\s*iota', db_types
    ))

    # Check for precedence-specific tests
    has_precedence_tests = bool(re.search(r'Precedence.*Owner.*Recipient', test_content, re.DOTALL))
    has_all_precedence = (
        bool(re.search(r'Precedence.*Owner.*Recipient', test_content)) and
        bool(re.search(r'Precedence.*Recipient.*System', test_content)) and
        bool(re.search(r'Precedence.*System.*Public', test_content))
    )

    if has_typed_enum and has_all_precedence:
        return (3, "Typed access type enum with full precedence test coverage")

    # Score 2: Comprehensive integration tests covering precedence.
    if has_all_precedence:
        return (2, "Integration tests verify owner>recipient>system>public precedence ordering")

    if has_precedence_tests:
        return (2, "Tests cover some access type precedence transitions")

    # Count access-related test functions
    test_fns = re.findall(r'func\s+Test\w*(?:Access|Session|Share)\w*', test_content)
    if len(test_fns) >= 10:
        return (2, f"{len(test_fns)} access/session test functions cover authorization scenarios")

    # Score 1: Some tests exist but don't explicitly test precedence.
    if len(test_fns) >= 3:
        return (1, f"{len(test_fns)} access tests exist but don't explicitly test priority ordering")

    # Score 0: No access control tests.
    return (0, "No access control tests found")


def check_10_redaction_on_nonowner_access() -> tuple[int, str]:
    """Non-owner access paths must call RedactForSharing before returning
    SessionDetail. Missing redaction = PII leak to share recipients.
    """
    access_file = BACKEND / "db" / "access" / "access.go"
    if not access_file.exists():
        return (0, "Access store file not found")
    content = access_file.read_text()

    # Score 3: The function that serves session detail to non-owners
    # returns a structurally different type (e.g., SharedSessionView)
    # that physically cannot contain PII fields.
    if re.search(r'type\s+SharedSession(?:View|Detail)\s+struct', content):
        return (3, "Non-owner access returns a separate type that structurally excludes PII")

    all_db_content = ""
    for f in (BACKEND / "db").rglob("*.go"):
        if not f.name.endswith("_test.go"):
            all_db_content += f.read_text()

    if re.search(r'type\s+SharedSession(?:View|Detail)\s+struct', all_db_content):
        return (3, "Non-owner access returns a separate type without PII fields")

    # Score 2: RedactForSharing is called on all non-owner paths,
    # verified by code pattern.
    # Look for: if not owner -> RedactForSharing
    match = re.search(
        r'func\s+\([^)]+\)\s+GetSessionDetailWithAccess\(.*?(?=\nfunc\s|\Z)',
        content, re.DOTALL
    )
    detail_body = match.group() if match else ""

    if re.search(r'RedactForSharing', detail_body) and re.search(
        r'(?:accessType|AccessType).*!=.*(?:owner|Owner)|(?:!.*[Oo]wner)', detail_body
    ):
        return (2, "RedactForSharing called for non-owner access in GetSessionDetailWithAccess")

    # Broader check: RedactForSharing called anywhere in access.go
    if re.search(r'RedactForSharing', content):
        return (2, "RedactForSharing called in access control path")

    # Score 1: RedactForSharing exists but not clearly called on non-owner path.
    if re.search(r'RedactForSharing', all_db_content):
        return (1, "RedactForSharing exists but not clearly gating non-owner access")

    # Score 0: No redaction in access path.
    return (0, "No PII redaction found in session access path")


def check_11_card_version_validity() -> tuple[int, str]:
    """Each card's IsValid method must check both Version == current constant
    AND UpToLine == currentLineCount. Missing either check means stale cards
    are silently served as valid.
    """
    cards_file = BACKEND / "analytics" / "cards.go"
    content = cards_file.read_text()

    # Find all IsValid methods
    isvalid_methods = re.findall(
        r'func\s+\([^)]+\w+CardRecord\)\s+IsValid\(.*?\{(.*?)\n\}',
        content, re.DOTALL
    )

    if not isvalid_methods:
        return (0, "No IsValid methods found on card records")

    # Check each method has both Version and UpToLine checks
    all_check_both = True
    for body in isvalid_methods:
        has_version = bool(re.search(r'Version\s*==', body))
        has_line = bool(re.search(r'UpToLine\s*(?:==|>=)', body))
        if not (has_version and has_line):
            all_check_both = False
            break

    # Score 3: Version constants are derived from a single source (e.g.,
    # auto-generated or all cards use a generic IsValid with the version
    # passed in, eliminating per-card version constant drift).
    if re.search(r'func\s+\w+IsValid.*version\s+int.*lineCount\s+int', content):
        return (3, "Generic IsValid function eliminates per-card version constant drift")

    # Also check for a generic method on an interface
    if re.search(r'type\s+CardValidator\s+interface.*IsValid', content, re.DOTALL):
        return (3, "CardValidator interface enforces IsValid contract across all card types")

    # Score 2: All IsValid methods check both Version and UpToLine.
    if all_check_both and len(isvalid_methods) >= 6:
        return (2, f"All {len(isvalid_methods)} card IsValid methods check both Version and UpToLine")

    # Score 1: Some IsValid methods are incomplete.
    complete = sum(1 for body in isvalid_methods
                   if re.search(r'Version\s*==', body) and re.search(r'UpToLine', body))
    if complete > 0:
        return (1, f"{complete}/{len(isvalid_methods)} IsValid methods check both Version and UpToLine")

    # Score 0: No dual check.
    return (0, "IsValid methods do not check both Version and UpToLine")


def check_12_sync_session_ownership() -> tuple[int, str]:
    """Sync chunk upload must verify that the authenticated user owns the
    session before allowing data upload. Missing check = data injection.
    """
    sync_file = BACKEND / "api" / "sync.go"
    content = sync_file.read_text()

    # Extract handleSyncChunk
    match = re.search(
        r'func\s+\([^)]+\)\s+handleSyncChunk\(.*?(?=\nfunc\s+\(|\nfunc\s+Handle|\Z)',
        content, re.DOTALL
    )
    body = match.group() if match else content

    # Score 3: Session ownership is enforced at the type level — the handler
    # can only receive a session handle that has already been verified.
    # Look for a typed session handle or middleware-level ownership check.
    if re.search(r'type\s+OwnedSession|type\s+VerifiedSession', content):
        return (3, "Typed OwnedSession handle enforces ownership at the type level")

    # Also check for middleware that injects verified session into context
    server_file = BACKEND / "api" / "server.go"
    if server_file.exists():
        server_content = server_file.read_text()
        if re.search(r'RequireSessionOwnership|VerifyOwnership.*middleware', server_content):
            return (3, "Middleware-level session ownership verification")

    # Score 2: Explicit ownership verification call before processing.
    if re.search(r'VerifySessionOwnership|verifySessionOwnership', body):
        return (2, "handleSyncChunk calls VerifySessionOwnership before uploading")

    # Also check for GetSessionDetail with userID (implies ownership check)
    if re.search(r'GetSessionDetail.*userID|sessionStore.*Verify', body):
        return (2, "handleSyncChunk verifies session ownership via DB query")

    # Score 1: userID is extracted but ownership not explicitly checked.
    if re.search(r'userID|user_id', body) and not re.search(r'Ownership|Verify', body):
        return (1, "userID extracted but no explicit ownership verification call")

    # Score 0: No ownership check.
    return (0, "No session ownership verification in sync chunk handler")


INVARIANTS = [
    ("chunk_line_bounds", check_01_chunk_line_bounds),
    ("pii_redaction_struct", check_02_pii_redaction_struct),
    ("card_list_exhaust", check_03_card_list_exhaustiveness),
    ("pricing_table_sync", check_04_pricing_table_sync),
    ("filter_param_valid", check_05_filter_param_validation),
    ("model_family_extract", check_06_model_family_extraction),
    ("cost_serialization", check_07_cost_serialization),
    ("chunk_firstline_val", check_08_chunk_firstline_validation),
    ("access_priority_test", check_09_access_priority_tests),
    ("redaction_nonowner", check_10_redaction_on_nonowner_access),
    ("card_version_valid", check_11_card_version_validity),
    ("sync_ownership", check_12_sync_session_ownership),
]

LEVEL_NAMES = {3: "structural", 2: "validated", 1: "convention", 0: "unguarded"}


def main() -> None:
    results = []
    for name, check_fn in INVARIANTS:
        score, explanation = check_fn()
        results.append((name, score, explanation))

    print("=" * 78)
    print(f"{'Invariant':<28} {'Score':<6} {'Level':<12} Explanation")
    print("-" * 78)
    for name, score, explanation in results:
        level = LEVEL_NAMES[score]
        print(f"{name:<28} {score:<6} {level:<12} {explanation}")
    print("=" * 78)

    scores = [r[1] for r in results]
    n = len(scores)
    total = sum(scores)
    ies = total / (3 * n) if n > 0 else 0

    print()
    print(f"ies_score: {ies:.4f}")
    print(f"ies_numerator: {total}")
    print(f"ies_denominator: {3 * n}")
    print(f"invariant_count: {n}")
    print(f"structural_count: {sum(1 for s in scores if s == 3)}")
    print(f"validated_count: {sum(1 for s in scores if s == 2)}")
    print(f"convention_count: {sum(1 for s in scores if s == 1)}")
    print(f"unguarded_count: {sum(1 for s in scores if s == 0)}")


if __name__ == "__main__":
    main()
