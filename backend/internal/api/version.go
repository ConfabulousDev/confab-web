package api

import (
	"net/http"
	"runtime"
)

// BuildInfo carries the compile-time build identity of this backend. Populated
// from -ldflags at release/deploy time (see Dockerfile, release.yml,
// deploy-to-fly.sh) and empty for `go run` dev builds.
type BuildInfo struct {
	Version   string // -X main.version (release tag, e.g. "v0.4.3")
	Commit    string // -X main.commit (full git SHA)
	BuildTime string // -X main.buildTime (RFC 3339 UTC)
}

// versionResponse reports what build this backend is. Dependency-free: no DB,
// no update checker, no network — safe for liveness probes, deploy verification,
// and the CLI capability probe (CF-476). "Is there a newer release?" stays in
// /api/v1/auth/config.
type versionResponse struct {
	Version   string `json:"version"`
	GoVersion string `json:"go_version"`
	Commit    string `json:"commit,omitempty"`     // omitted when unset (e.g. go run dev build)
	BuildTime string `json:"build_time,omitempty"` // RFC 3339; omitted when unset
}

// handleVersion reports the running backend build. Public, no auth, and
// deliberately free of any DB/update-checker/network dependency so it stays
// fast and safe for liveness probes and the CLI capability probe.
func (s *Server) handleVersion(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, versionResponse{
		Version:   versionOrDev(s.buildInfo.Version),
		GoVersion: runtime.Version(),
		Commit:    s.buildInfo.Commit,
		BuildTime: s.buildInfo.BuildTime,
	})
}

// versionOrDev gives non-release builds a sensible non-empty label.
func versionOrDev(v string) string {
	if v == "" {
		return "dev"
	}
	return v
}
