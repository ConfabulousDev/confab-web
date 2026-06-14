package admin

import (
	"reflect"
	"testing"
)

// resetSuperAdminCache clears the package-level cache so a test exercises the
// live-env fallback path and doesn't leak state into sibling tests.
func resetSuperAdminCache(t *testing.T) {
	t.Helper()
	t.Cleanup(func() { cachedSuperAdmins.Store(nil) })
	cachedSuperAdmins.Store(nil)
}

func TestParseSuperAdminEmails(t *testing.T) {
	cases := []struct {
		name      string
		raw       string
		wantSet   []string // sorted normalized emails expected in the set
		wantWarns int
	}{
		{"empty → empty, no warnings", "", nil, 0},
		{"whitespace only → empty, no warnings", "   ", nil, 0},
		{"single", "admin@example.com", []string{"admin@example.com"}, 0},
		{"trims + lowercases", "  Admin@Example.com  ", []string{"admin@example.com"}, 0},
		{"multiple", "a@x.com,b@x.com", []string{"a@x.com", "b@x.com"}, 0},
		{"trailing comma warns once", "a@x.com,", []string{"a@x.com"}, 1},
		{"duplicate warns + dedups", "a@x.com,A@x.com", []string{"a@x.com"}, 1},
		{"invalid email warns + dropped", "a@x.com,not-an-email", []string{"a@x.com"}, 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			set, warns := ParseSuperAdminEmails(c.raw)
			got := make([]string, 0, len(set))
			for e := range set {
				got = append(got, e)
			}
			want := append([]string(nil), c.wantSet...)
			// compare as sets (order-independent)
			if !equalStringSet(got, want) {
				t.Errorf("set = %v, want %v", got, want)
			}
			if len(warns) != c.wantWarns {
				t.Errorf("warnings = %d (%v), want %d", len(warns), warns, c.wantWarns)
			}
		})
	}
}

func equalStringSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	m := map[string]int{}
	for _, s := range a {
		m[s]++
	}
	for _, s := range b {
		m[s]--
	}
	for _, v := range m {
		if v != 0 {
			return false
		}
	}
	return true
}

func TestIsSuperAdmin_CachedSet(t *testing.T) {
	resetSuperAdminCache(t)
	SetSuperAdmins(map[string]struct{}{"admin@example.com": {}})

	if !IsSuperAdmin("admin@example.com") {
		t.Error("expected cached super-admin to match")
	}
	if !IsSuperAdmin("  ADMIN@Example.com  ") {
		t.Error("expected case-insensitive + trimmed match against cache")
	}
	if IsSuperAdmin("user@example.com") {
		t.Error("non-member must not match")
	}

	SetSuperAdmins(map[string]struct{}{})
	if IsSuperAdmin("admin@example.com") {
		t.Error("empty cached set must return false")
	}
}

func TestSuperAdminEmails_SortedFromCache(t *testing.T) {
	resetSuperAdminCache(t)
	SetSuperAdmins(map[string]struct{}{"b@x.com": {}, "a@x.com": {}})
	got := SuperAdminEmails()
	if !reflect.DeepEqual(got, []string{"a@x.com", "b@x.com"}) {
		t.Errorf("SuperAdminEmails() = %v, want sorted [a@x.com b@x.com]", got)
	}
}

func TestWouldOrphanLastAdmin(t *testing.T) {
	cases := []struct {
		name          string
		ids           []int64
		target        int64
		targetRemains bool
		wantBlock     bool
	}{
		{"sole admin, action removes → block", []int64{7}, 7, false, true},
		{"sole admin but remains effective (env super-admin revoke) → allow", []int64{7}, 7, true, false},
		{"another admin exists → allow", []int64{7, 9}, 7, false, false},
		{"target not an admin → allow", []int64{9}, 7, false, false},
		{"no admins at all, target not admin → allow", nil, 7, false, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := wouldOrphanLastAdmin(c.ids, c.target, c.targetRemains); got != c.wantBlock {
				t.Errorf("wouldOrphanLastAdmin(%v, %d, %v) = %v, want %v",
					c.ids, c.target, c.targetRemains, got, c.wantBlock)
			}
		})
	}

	// Explicit case: sole admin who remains effective after the action (revoke on
	// a user who is also an env super-admin) must be allowed.
	if wouldOrphanLastAdmin([]int64{7}, 7, true) {
		t.Error("sole admin who remains effective must NOT be blocked")
	}
}
