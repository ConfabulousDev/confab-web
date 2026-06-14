package db

import (
	"strings"
	"testing"
)

// fullGitInfo is a representative unmarshaled git_info blob carrying every
// known producer key, including credential- and host-bearing ones.
func fullGitInfo() map[string]interface{} {
	return map[string]interface{}{
		"repo_url":       "https://alice:ghp_secrettoken@github.com/acme/widget.git",
		"branch":         "feature/login",
		"commit_sha":     "deadbeefcafebabe",
		"commit_message": "fix: secret internal detail",
		"author":         "Alice Owner <alice@example.com>",
		"is_dirty":       true,
		"remotes": []interface{}{
			map[string]interface{}{"name": "origin", "fetch_url": "https://alice:ghp_secrettoken@github.com/acme/widget.git", "push_url": "git@github.com:acme/widget.git"},
			map[string]interface{}{"name": "upstream", "fetch_url": "https://internal.example.com/acme/widget.git"},
		},
		"tracking_remote": "upstream",
	}
}

func TestSanitizeGitInfoForSharing_KeepsBranchAndDisplayName(t *testing.T) {
	out, ok := SanitizeGitInfoForSharing(fullGitInfo()).(map[string]interface{})
	if !ok {
		t.Fatalf("expected map[string]interface{}, got %T", SanitizeGitInfoForSharing(fullGitInfo()))
	}

	if out["branch"] != "feature/login" {
		t.Errorf("expected branch kept, got %v", out["branch"])
	}
	// repo_url reduced to a bare owner/repo display name — no host, no creds.
	if out["repo_url"] != "acme/widget" {
		t.Errorf("expected repo_url = \"acme/widget\" (host/creds stripped), got %v", out["repo_url"])
	}
}

func TestSanitizeGitInfoForSharing_DropsCredentialAndUrlBearingKeys(t *testing.T) {
	out, ok := SanitizeGitInfoForSharing(fullGitInfo()).(map[string]interface{})
	if !ok {
		t.Fatalf("expected map, got %T", SanitizeGitInfoForSharing(fullGitInfo()))
	}

	for _, dropped := range []string{"remotes", "tracking_remote", "author", "commit_message", "commit_sha", "is_dirty"} {
		if _, present := out[dropped]; present {
			t.Errorf("expected key %q to be dropped for non-owner sharing, but it was present: %v", dropped, out[dropped])
		}
	}

	// Belt-and-suspenders: no value anywhere in the sanitized blob may contain
	// a credential marker, a scheme, an @, or a known host substring.
	for k, v := range out {
		s, isStr := v.(string)
		if !isStr {
			continue
		}
		for _, forbidden := range []string{"ghp_secrettoken", "alice:", "@", "://", "github.com", "internal.example.com"} {
			if strings.Contains(s, forbidden) {
				t.Errorf("sanitized git_info[%q]=%q contains forbidden substring %q", k, s, forbidden)
			}
		}
	}
}

func TestSanitizeGitInfoForSharing_RepoDisplayFromVariousURLForms(t *testing.T) {
	// repo_url is always a full remote URL in stored git_info; every form must
	// reduce to its trailing owner/repo with scheme, credentials, and host gone.
	cases := map[string]string{
		"https://alice:tok@github.com/acme/widget.git":  "acme/widget",
		"git@github.com:acme/widget.git":                "acme/widget",
		"ssh://git@gitlab.example.com:2222/acme/widget": "acme/widget",
		"https://github.com/acme/widget/":               "acme/widget",
	}
	for in, want := range cases {
		out, _ := SanitizeGitInfoForSharing(map[string]interface{}{"repo_url": in}).(map[string]interface{})
		if out["repo_url"] != want {
			t.Errorf("repo_url %q: expected display %q, got %v", in, want, out["repo_url"])
		}
	}
}

// TestSanitizeGitInfoForSharing_SingleSegmentCredentialURLDropped locks in the
// hostile-review finding: a single-path-segment remote carrying credentials
// (no owner/repo pair, just host/repo) must DROP repo_url entirely rather than
// leak the credential or host into the display name.
func TestSanitizeGitInfoForSharing_SingleSegmentCredentialURLDropped(t *testing.T) {
	for _, url := range []string{
		"https://alice:ghp_secrettoken@git.example.com/repo.git",
		"https://git.example.com/repo",
		"git@git.example.com:repo.git",
	} {
		got := SanitizeGitInfoForSharing(map[string]interface{}{"repo_url": url})
		out, _ := got.(map[string]interface{})
		if v, present := out["repo_url"]; present {
			t.Errorf("repo_url %q: expected dropped (no clean owner/repo), got display %q", url, v)
		}
	}
}

func TestSanitizeGitInfoForSharing_NilAndNonMapAreSafe(t *testing.T) {
	if got := SanitizeGitInfoForSharing(nil); got != nil {
		t.Errorf("expected nil for nil input, got %v", got)
	}
	// Unexpected shapes (a bare string, a number) must drop to nil — fail-safe.
	if got := SanitizeGitInfoForSharing("https://alice:tok@github.com/acme/widget"); got != nil {
		t.Errorf("expected nil for non-map input, got %v", got)
	}
	// A map with only unknown/unsafe keys yields nil (nothing to keep).
	if got := SanitizeGitInfoForSharing(map[string]interface{}{"author": "Alice", "tracking_remote": "upstream"}); got != nil {
		t.Errorf("expected nil when no whitelisted keys present, got %v", got)
	}
}
