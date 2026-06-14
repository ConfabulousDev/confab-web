package admin

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"sync/atomic"

	"github.com/ConfabulousDev/confab-web/internal/validation"
)

// cachedSuperAdmins holds the startup-validated, normalized super-admin set.
// When nil (not yet initialized — tests, or before SetSuperAdmins runs),
// IsSuperAdmin falls back to a live parse of SUPER_ADMIN_EMAILS so behavior is
// preserved without an explicit init.
var cachedSuperAdmins atomic.Pointer[map[string]struct{}]

// ParseSuperAdminEmails parses a raw SUPER_ADMIN_EMAILS value (comma-separated)
// into a normalized, de-duplicated set, returning human-readable warnings for
// empty / invalid / duplicate entries so a typo is surfaced loudly rather than
// silently producing a non-matching entry (g0bq, audit E4). Pure + testable.
func ParseSuperAdminEmails(raw string) (map[string]struct{}, []string) {
	set := make(map[string]struct{})
	var warnings []string
	if strings.TrimSpace(raw) == "" {
		return set, warnings
	}
	for _, part := range strings.Split(raw, ",") {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			warnings = append(warnings, "SUPER_ADMIN_EMAILS: skipped empty entry (stray comma?)")
			continue
		}
		norm := validation.NormalizeEmail(trimmed)
		if !validation.IsValidEmail(norm) {
			warnings = append(warnings, fmt.Sprintf("SUPER_ADMIN_EMAILS: skipped invalid email %q", trimmed))
			continue
		}
		if _, dup := set[norm]; dup {
			warnings = append(warnings, fmt.Sprintf("SUPER_ADMIN_EMAILS: duplicate entry %q", norm))
			continue
		}
		set[norm] = struct{}{}
	}
	return set, warnings
}

// SetSuperAdmins installs the validated super-admin set. Called once at startup
// (cmd/server) after logging any parse warnings, so request-time IsSuperAdmin
// reads the cached set instead of re-parsing the env on every call.
func SetSuperAdmins(set map[string]struct{}) {
	cachedSuperAdmins.Store(&set)
}

// SuperAdminEmails returns the normalized super-admin emails (sorted), from the
// cached set when present, otherwise from a live env parse. Used by the
// last-admin guard to count env-backed admins.
func SuperAdminEmails() []string {
	set := cachedSuperAdmins.Load()
	if set == nil {
		parsed, _ := ParseSuperAdminEmails(os.Getenv("SUPER_ADMIN_EMAILS"))
		set = &parsed
	}
	out := make([]string, 0, len(*set))
	for email := range *set {
		out = append(out, email)
	}
	sort.Strings(out)
	return out
}

// IsSuperAdmin reports whether email is a configured super-admin (one half of
// the admin union with users.is_admin). Reads the startup-validated cached set
// when present; otherwise falls back to a live parse of SUPER_ADMIN_EMAILS.
func IsSuperAdmin(email string) bool {
	norm := validation.NormalizeEmail(email)
	if set := cachedSuperAdmins.Load(); set != nil {
		_, ok := (*set)[norm]
		return ok
	}
	parsed, _ := ParseSuperAdminEmails(os.Getenv("SUPER_ADMIN_EMAILS"))
	_, ok := parsed[norm]
	return ok
}

// wouldOrphanLastAdmin is the pure decision behind the last-effective-admin
// guard (g0bq, audit E5). effectiveAdminIDs is the set of active users that can
// still reach the admin panel (is_admin=true OR email in SUPER_ADMIN_EMAILS).
// The action is blocked only when the target IS currently an effective admin,
// NO other effective admin exists, and the action removes the target from the
// set (targetRemainsEffective=false). Revoking the column flag from a user who
// is also an env super-admin keeps them effective (targetRemainsEffective=true),
// so it is allowed.
func wouldOrphanLastAdmin(effectiveAdminIDs []int64, targetID int64, targetRemainsEffective bool) bool {
	targetIsAdmin, othersExist := false, false
	for _, id := range effectiveAdminIDs {
		if id == targetID {
			targetIsAdmin = true
		} else {
			othersExist = true
		}
	}
	if !targetIsAdmin || othersExist {
		return false
	}
	return !targetRemainsEffective
}
