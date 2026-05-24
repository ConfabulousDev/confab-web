package db

import (
	"encoding/json"
	"strings"
)

// CF-494 — primary fork→upstream resolver, fed by CLI-shipped git remotes.
//
// The CLI (CF-493) ships the user's full set of git remotes plus the current
// branch's tracking remote on every sync; the backend extracts a definitive
// fork→upstream mapping from those signals. This file is the pure parsing +
// resolution layer. Validation (size caps, required fields) lives in the
// validation package; handlers call validation.ValidateGitInfo first, then
// ParseGitInfo + ResolveForkFromRemotes, then RecordRepoRoot.
//
// The existing PR-link resolver in api/sync.go::HandleSyncChunk remains as
// a fallback for sessions with no tracking_remote configured or shipped
// from old CLIs; first-write-wins via RecordRepoRoot's IS NULL guard means
// git_remote wins when both fire on the same sync.

// GitRemote is one entry in git_info.remotes. JSON tags match CF-493's
// CLI-side struct exactly.
type GitRemote struct {
	Name     string `json:"name"`
	FetchURL string `json:"fetch_url"`
	PushURL  string `json:"push_url"`
}

// ParsedGitInfo is the typed view of the subset of git_info this package
// cares about. Other fields (branch, commit_sha, …) are preserved verbatim
// in the JSONB column via the json.RawMessage pass-through; only fields
// consumed by the resolver are surfaced here.
type ParsedGitInfo struct {
	RepoURL        string      `json:"repo_url"`
	Remotes        []GitRemote `json:"remotes"`
	TrackingRemote string      `json:"tracking_remote"`
}

// ParseGitInfo unmarshals git_info JSON into the typed ParsedGitInfo view.
// Tolerant: nil / empty / malformed input returns the zero value with nil
// error (mirrors ExtractRepoFromGitInfo). Real validation — size caps,
// required fields — lives in validation.ValidateGitInfo and runs at the
// handler layer before this function.
func ParseGitInfo(gitInfo []byte) (ParsedGitInfo, error) {
	if len(gitInfo) == 0 {
		return ParsedGitInfo{}, nil
	}
	var out ParsedGitInfo
	if err := json.Unmarshal(gitInfo, &out); err != nil {
		return ParsedGitInfo{}, nil
	}
	return out, nil
}

// FindRemoteByName returns the first remote whose Name matches exactly, or
// nil. Case-sensitive (matches git's own config semantics). Exposed so the
// handler can build a "tracking_remote names unknown remote" Warn log
// without re-walking the slice.
func FindRemoteByName(remotes []GitRemote, name string) *GitRemote {
	for i := range remotes {
		if remotes[i].Name == name {
			return &remotes[i]
		}
	}
	return nil
}

// ResolveForkFromRemotes returns (fork, root, true) when info shows a clear
// fork→upstream relationship. Returns ("", "", false) otherwise.
//
// Rules (in order):
//   - TrackingRemote must be non-empty (no tracking_remote configured =
//     not a fork in this scheme; PR-link fallback may still apply).
//   - TrackingRemote must name a remote in Remotes (case-sensitive). The
//     handler logs Warn when present-but-unknown — that's a "log + drop"
//     scenario, not a 4xx.
//   - The tracking remote's owner/repo must extract from FetchURL, falling
//     back to PushURL when FetchURL is empty. (Per-entry validation
//     guarantees at least one URL exists; this is the runtime path.)
//   - The fork's owner/repo extracts from info.RepoURL directly.
//   - If extracted fork and root match case-insensitively, it's a self-loop
//     (or both URLs point to the same repo) — not a fork.
//
// Returned values are case-as-typed; the case-fold only applies to the
// self-loop check. This matches CF-491's "two casings produce two chips"
// policy on the storage side.
func ResolveForkFromRemotes(info ParsedGitInfo) (fork, root string, ok bool) {
	if info.TrackingRemote == "" || info.RepoURL == "" || len(info.Remotes) == 0 {
		return "", "", false
	}
	tracking := FindRemoteByName(info.Remotes, info.TrackingRemote)
	if tracking == nil {
		return "", "", false
	}
	trackingURL := tracking.FetchURL
	if trackingURL == "" {
		trackingURL = tracking.PushURL
	}
	// ExtractRepoName never returns nil — its fallback returns &repoURL — but
	// it can return an empty string for an empty input URL. Only the empty
	// check matters here.
	root = *ExtractRepoName(trackingURL)
	fork = *ExtractRepoName(info.RepoURL)
	if root == "" || fork == "" {
		return "", "", false
	}
	if strings.EqualFold(fork, root) {
		return "", "", false
	}
	return fork, root, true
}
