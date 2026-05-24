package db

import (
	"reflect"
	"testing"
)

// CF-494 — unit tests for the git_remote-signal resolver (pure functions).
// These define the resolver's contract; api/sync.go wires them into the
// sync handlers.

func TestParseGitInfo_NilEmpty_ReturnsZeroNoError(t *testing.T) {
	cases := map[string][]byte{
		"nil":          nil,
		"empty slice":  {},
		"empty string": []byte(""),
	}
	for name, in := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := ParseGitInfo(in)
			if err != nil {
				t.Errorf("ParseGitInfo returned error: %v", err)
			}
			if !reflect.DeepEqual(got, ParsedGitInfo{}) {
				t.Errorf("ParseGitInfo = %+v, want zero value", got)
			}
		})
	}
}

func TestParseGitInfo_MalformedJSON_ReturnsZeroNoError(t *testing.T) {
	got, err := ParseGitInfo([]byte("not-json"))
	if err != nil {
		t.Errorf("expected nil error for malformed JSON (tolerant parse), got %v", err)
	}
	if !reflect.DeepEqual(got, ParsedGitInfo{}) {
		t.Errorf("expected zero value for malformed JSON, got %+v", got)
	}
}

func TestParseGitInfo_FullPayload_Populated(t *testing.T) {
	in := []byte(`{
		"repo_url": "git@github.com:jackie/confab-web.git",
		"remotes": [
			{"name":"origin","fetch_url":"git@github.com:jackie/confab-web.git","push_url":"git@github.com:jackie/confab-web.git"},
			{"name":"upstream","fetch_url":"https://github.com/ConfabulousDev/confab-web.git","push_url":"https://github.com/ConfabulousDev/confab-web.git"}
		],
		"tracking_remote": "upstream"
	}`)
	got, err := ParseGitInfo(in)
	if err != nil {
		t.Fatalf("ParseGitInfo: %v", err)
	}
	if got.RepoURL != "git@github.com:jackie/confab-web.git" {
		t.Errorf("RepoURL = %q", got.RepoURL)
	}
	if got.TrackingRemote != "upstream" {
		t.Errorf("TrackingRemote = %q", got.TrackingRemote)
	}
	if len(got.Remotes) != 2 {
		t.Fatalf("len(Remotes) = %d, want 2", len(got.Remotes))
	}
	if got.Remotes[0].Name != "origin" || got.Remotes[1].Name != "upstream" {
		t.Errorf("Remotes order/names off: %+v", got.Remotes)
	}
}

func TestResolveForkFromRemotes_CanonicalOriginUpstream(t *testing.T) {
	info := ParsedGitInfo{
		RepoURL: "git@github.com:jackie/confab-web.git",
		Remotes: []GitRemote{
			{Name: "origin", FetchURL: "git@github.com:jackie/confab-web.git"},
			{Name: "upstream", FetchURL: "https://github.com/ConfabulousDev/confab-web.git"},
		},
		TrackingRemote: "upstream",
	}
	fork, root, ok := ResolveForkFromRemotes(info)
	if !ok {
		t.Fatal("expected ok=true for canonical fork+upstream")
	}
	if fork != "jackie/confab-web" {
		t.Errorf("fork = %q, want jackie/confab-web", fork)
	}
	if root != "ConfabulousDev/confab-web" {
		t.Errorf("root = %q, want ConfabulousDev/confab-web", root)
	}
}

func TestResolveForkFromRemotes_NonStandardTrackingName(t *testing.T) {
	info := ParsedGitInfo{
		RepoURL: "https://github.com/me/repo.git",
		Remotes: []GitRemote{
			{Name: "origin", FetchURL: "https://github.com/me/repo.git"},
			{Name: "canonical", FetchURL: "https://github.com/them/repo.git"},
		},
		TrackingRemote: "canonical",
	}
	fork, root, ok := ResolveForkFromRemotes(info)
	if !ok || fork != "me/repo" || root != "them/repo" {
		t.Errorf("got (%q,%q,%v), want (me/repo, them/repo, true)", fork, root, ok)
	}
}

func TestResolveForkFromRemotes_TrackingRemoteUnset(t *testing.T) {
	info := ParsedGitInfo{
		RepoURL: "https://github.com/me/repo.git",
		Remotes: []GitRemote{
			{Name: "origin", FetchURL: "https://github.com/me/repo.git"},
		},
		TrackingRemote: "",
	}
	if _, _, ok := ResolveForkFromRemotes(info); ok {
		t.Error("expected ok=false when TrackingRemote is empty")
	}
}

func TestResolveForkFromRemotes_TrackingPointsAtFork(t *testing.T) {
	// Self-loop: tracking_remote URL extracts to same owner/repo as repo_url.
	info := ParsedGitInfo{
		RepoURL: "https://github.com/me/repo.git",
		Remotes: []GitRemote{
			{Name: "origin", FetchURL: "https://github.com/me/repo.git"},
		},
		TrackingRemote: "origin",
	}
	if _, _, ok := ResolveForkFromRemotes(info); ok {
		t.Error("expected ok=false when tracking remote extracts to same repo (self-loop)")
	}
}

func TestResolveForkFromRemotes_TrackingURLNotExtractable(t *testing.T) {
	info := ParsedGitInfo{
		RepoURL: "https://github.com/me/repo.git",
		Remotes: []GitRemote{
			{Name: "origin", FetchURL: "https://github.com/me/repo.git"},
			{Name: "upstream", FetchURL: ""},
		},
		TrackingRemote: "upstream",
	}
	if _, _, ok := ResolveForkFromRemotes(info); ok {
		t.Error("expected ok=false when tracking remote has no extractable URL")
	}
}

func TestResolveForkFromRemotes_DuplicateRemoteNames_FirstWins(t *testing.T) {
	info := ParsedGitInfo{
		RepoURL: "https://github.com/me/repo.git",
		Remotes: []GitRemote{
			{Name: "origin", FetchURL: "https://github.com/me/repo.git"},
			{Name: "upstream", FetchURL: "https://github.com/them/repo.git"},
			{Name: "upstream", FetchURL: "https://github.com/other/repo.git"},
		},
		TrackingRemote: "upstream",
	}
	_, root, ok := ResolveForkFromRemotes(info)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if root != "them/repo" {
		t.Errorf("root = %q, want them/repo (first-match-wins)", root)
	}
}

func TestResolveForkFromRemotes_SSHvsHTTPSMix(t *testing.T) {
	// fork via SSH, upstream via HTTPS — both should extract.
	info := ParsedGitInfo{
		RepoURL: "git@github.com:me/repo.git",
		Remotes: []GitRemote{
			{Name: "origin", FetchURL: "git@github.com:me/repo.git"},
			{Name: "upstream", FetchURL: "https://github.com/them/repo.git"},
		},
		TrackingRemote: "upstream",
	}
	fork, root, ok := ResolveForkFromRemotes(info)
	if !ok || fork != "me/repo" || root != "them/repo" {
		t.Errorf("got (%q,%q,%v)", fork, root, ok)
	}
}

func TestResolveForkFromRemotes_RepoURLEmpty(t *testing.T) {
	info := ParsedGitInfo{
		RepoURL: "",
		Remotes: []GitRemote{
			{Name: "origin", FetchURL: "https://github.com/me/repo.git"},
			{Name: "upstream", FetchURL: "https://github.com/them/repo.git"},
		},
		TrackingRemote: "upstream",
	}
	if _, _, ok := ResolveForkFromRemotes(info); ok {
		t.Error("expected ok=false when RepoURL is empty")
	}
}

func TestResolveForkFromRemotes_RemotesEmpty(t *testing.T) {
	info := ParsedGitInfo{
		RepoURL:        "https://github.com/me/repo.git",
		TrackingRemote: "upstream",
	}
	if _, _, ok := ResolveForkFromRemotes(info); ok {
		t.Error("expected ok=false when Remotes is empty")
	}
}

func TestResolveForkFromRemotes_TrackingNamesUnknownRemote(t *testing.T) {
	info := ParsedGitInfo{
		RepoURL: "https://github.com/me/repo.git",
		Remotes: []GitRemote{
			{Name: "origin", FetchURL: "https://github.com/me/repo.git"},
		},
		TrackingRemote: "nonexistent",
	}
	if _, _, ok := ResolveForkFromRemotes(info); ok {
		t.Error("expected ok=false when tracking_remote names a remote not in the list")
	}
}

func TestResolveForkFromRemotes_FetchURLEmpty_FallsBackToPushURL(t *testing.T) {
	info := ParsedGitInfo{
		RepoURL: "https://github.com/me/repo.git",
		Remotes: []GitRemote{
			{Name: "origin", FetchURL: "https://github.com/me/repo.git", PushURL: "https://github.com/me/repo.git"},
			{Name: "upstream", FetchURL: "", PushURL: "https://github.com/them/repo.git"},
		},
		TrackingRemote: "upstream",
	}
	fork, root, ok := ResolveForkFromRemotes(info)
	if !ok || fork != "me/repo" || root != "them/repo" {
		t.Errorf("got (%q,%q,%v), want (me/repo, them/repo, true) via push_url fallback", fork, root, ok)
	}
}

func TestResolveForkFromRemotes_TrackingNameCaseSensitive(t *testing.T) {
	// Q13: case-sensitive matching, mirroring git semantics.
	info := ParsedGitInfo{
		RepoURL: "https://github.com/me/repo.git",
		Remotes: []GitRemote{
			{Name: "upstream", FetchURL: "https://github.com/them/repo.git"},
		},
		TrackingRemote: "Upstream",
	}
	if _, _, ok := ResolveForkFromRemotes(info); ok {
		t.Error("expected ok=false on case-mismatched tracking remote name")
	}
}

func TestResolveForkFromRemotes_SelfLoopAcrossCasings(t *testing.T) {
	// Q2: self-loop check is case-insensitive (EqualFold). Stamps would be
	// wrong if jackie/repo and Jackie/repo were treated as fork+upstream.
	info := ParsedGitInfo{
		RepoURL: "https://github.com/jackie/repo.git",
		Remotes: []GitRemote{
			{Name: "origin", FetchURL: "https://github.com/jackie/repo.git"},
			{Name: "upstream", FetchURL: "https://github.com/Jackie/repo.git"},
		},
		TrackingRemote: "upstream",
	}
	if _, _, ok := ResolveForkFromRemotes(info); ok {
		t.Error("expected ok=false when fork and root differ only by case (self-loop)")
	}
}

func TestFindRemoteByName(t *testing.T) {
	remotes := []GitRemote{
		{Name: "origin", FetchURL: "x"},
		{Name: "upstream", FetchURL: "y"},
	}
	if got := FindRemoteByName(remotes, "upstream"); got == nil || got.FetchURL != "y" {
		t.Errorf("expected upstream entry, got %+v", got)
	}
	if got := FindRemoteByName(remotes, "missing"); got != nil {
		t.Errorf("expected nil for missing name, got %+v", got)
	}
	if got := FindRemoteByName(remotes, "Upstream"); got != nil {
		t.Error("expected nil for case-mismatched lookup (case-sensitive)")
	}
}
