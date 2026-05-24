package validation

import (
	"fmt"
	"strings"
	"testing"
)

func TestValidateExternalID(t *testing.T) {
	tests := []struct {
		name       string
		externalID string
		wantErr    bool
	}{
		{
			name:       "valid external ID",
			externalID: "session-123-abc",
			wantErr:    false,
		},
		{
			name:       "empty external ID",
			externalID: "",
			wantErr:    true,
		},
		{
			name:       "external ID too long",
			externalID: strings.Repeat("a", MaxExternalIDLength+1),
			wantErr:    true,
		},
		{
			name:       "external ID at max length",
			externalID: strings.Repeat("a", MaxExternalIDLength),
			wantErr:    false,
		},
		{
			name:       "external ID with spaces",
			externalID: "session 123",
			wantErr:    false, // Spaces are valid UTF-8
		},
		{
			name:       "external ID with special chars",
			externalID: "session-123_abc.xyz",
			wantErr:    false,
		},
		{
			name:       "invalid UTF-8",
			externalID: string([]byte{0xff, 0xfe, 0xfd}),
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateExternalID(tt.externalID)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateExternalID() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateHostname(t *testing.T) {
	tests := []struct {
		name     string
		hostname string
		wantErr  bool
	}{
		{
			name:     "valid hostname",
			hostname: "macbook.local",
			wantErr:  false,
		},
		{
			name:     "empty hostname",
			hostname: "",
			wantErr:  false, // Empty is allowed (optional field)
		},
		{
			name:     "hostname at max length",
			hostname: strings.Repeat("a", MaxHostnameLength),
			wantErr:  false,
		},
		{
			name:     "hostname too long",
			hostname: strings.Repeat("a", MaxHostnameLength+1),
			wantErr:  true,
		},
		{
			name:     "hostname with special chars",
			hostname: "my-laptop.home.local",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateHostname(tt.hostname)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateHostname() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateUsername(t *testing.T) {
	tests := []struct {
		name     string
		username string
		wantErr  bool
	}{
		{
			name:     "valid username",
			username: "jackie",
			wantErr:  false,
		},
		{
			name:     "empty username",
			username: "",
			wantErr:  false, // Empty is allowed (optional field)
		},
		{
			name:     "username at max length",
			username: strings.Repeat("a", MaxUsernameLength),
			wantErr:  false,
		},
		{
			name:     "username too long",
			username: strings.Repeat("a", MaxUsernameLength+1),
			wantErr:  true,
		},
		{
			name:     "username with special chars",
			username: "user_name-123",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateUsername(tt.username)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateUsername() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestValidateProvider locks the strict-exact-match contract for the
// `provider` field on POST /api/v1/sync/init. The HTTP handler is
// responsible for defaulting a missing (nil) field to ProviderClaudeCode
// before calling here. An explicit empty string is NOT accepted at this
// layer — only "claude-code" and "codex" pass.
func TestValidateProvider(t *testing.T) {
	tests := []struct {
		name     string
		provider string
		wantErr  bool
	}{
		{name: "canonical claude-code is accepted", provider: "claude-code", wantErr: false},
		{name: "canonical codex is accepted", provider: "codex", wantErr: false},
		{name: "explicit empty string is rejected", provider: "", wantErr: true},
		{name: "uppercase Codex is rejected", provider: "Codex", wantErr: true},
		{name: "leading space is rejected", provider: " codex", wantErr: true},
		{name: "trailing space is rejected", provider: "claude-code ", wantErr: true},
		{name: "unknown provider gemini is rejected", provider: "gemini", wantErr: true},
		{name: "uppercase CLAUDE-CODE is rejected", provider: "CLAUDE-CODE", wantErr: true},
		{name: "legacy display form Claude Code is rejected", provider: "Claude Code", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateProvider(tt.provider)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateProvider(%q) error = %v, wantErr %v", tt.provider, err, tt.wantErr)
			}
		})
	}
}

// TestValidateCodexRolloutMetadata locks the field-level contract for the
// codex_rollout sub-block on POST /api/v1/sync/chunk. The provider-mismatch
// check (rejecting codex_rollout on a claude-code session) is enforced in
// the handler, not here.
func TestValidateCodexRolloutMetadata(t *testing.T) {
	const validUUID = "11111111-1111-1111-1111-111111111111"
	const otherUUID = "22222222-2222-2222-2222-222222222222"
	ptr := func(s string) *string { return &s }

	tests := []struct {
		name             string
		threadUUID       string
		parentThreadUUID *string
		rolloutPath      string
		cwd              string
		model            string
		source           string
		threadSource     string
		agentPath        string
		agentRole        string
		agentNickname    string
		wantErr          bool
	}{
		{
			name:        "happy path root rollout (nil parent)",
			threadUUID:  validUUID,
			rolloutPath: "/home/user/.codex/sessions/rollout-2026-05-15-abc.jsonl",
			cwd:         "/home/user/project",
			model:       "gpt-5",
			source:      "codex-cli",
		},
		{
			name:             "happy path child rollout (parent set)",
			threadUUID:       validUUID,
			parentThreadUUID: ptr(otherUUID),
			rolloutPath:      "/home/user/.codex/sessions/rollout-child.jsonl",
		},
		{
			name:        "missing thread_uuid",
			threadUUID:  "",
			rolloutPath: "/path",
			wantErr:     true,
		},
		{
			name:        "thread_uuid not a valid UUID",
			threadUUID:  "not-a-uuid",
			rolloutPath: "/path",
			wantErr:     true,
		},
		{
			name:             "parent_thread_uuid explicit empty string is rejected",
			threadUUID:       validUUID,
			parentThreadUUID: ptr(""),
			rolloutPath:      "/path",
			wantErr:          true,
		},
		{
			name:             "parent_thread_uuid invalid UUID",
			threadUUID:       validUUID,
			parentThreadUUID: ptr("not-a-uuid"),
			rolloutPath:      "/path",
			wantErr:          true,
		},
		{
			name:             "parent_thread_uuid equals thread_uuid (self-link)",
			threadUUID:       validUUID,
			parentThreadUUID: ptr(validUUID),
			rolloutPath:      "/path",
			wantErr:          true,
		},
		{
			name:        "rollout_path empty",
			threadUUID:  validUUID,
			rolloutPath: "",
			wantErr:     true,
		},
		{
			name:        "rollout_path too long",
			threadUUID:  validUUID,
			rolloutPath: strings.Repeat("a", MaxCodexRolloutPathLength+1),
			wantErr:     true,
		},
		{
			name:        "cwd too long",
			threadUUID:  validUUID,
			rolloutPath: "/path",
			cwd:         strings.Repeat("a", MaxCodexCWDLength+1),
			wantErr:     true,
		},
		{
			name:        "model too long",
			threadUUID:  validUUID,
			rolloutPath: "/path",
			model:       strings.Repeat("a", MaxCodexModelLength+1),
			wantErr:     true,
		},
		{
			name:        "source too long",
			threadUUID:  validUUID,
			rolloutPath: "/path",
			source:      strings.Repeat("a", MaxCodexSourceLength+1),
			wantErr:     true,
		},
		{
			name:         "thread_source too long",
			threadUUID:   validUUID,
			rolloutPath:  "/path",
			threadSource: strings.Repeat("a", MaxCodexThreadSourceLength+1),
			wantErr:      true,
		},
		{
			name:        "agent_path too long",
			threadUUID:  validUUID,
			rolloutPath: "/path",
			agentPath:   strings.Repeat("a", MaxCodexAgentPathLength+1),
			wantErr:     true,
		},
		{
			name:        "agent_role too long",
			threadUUID:  validUUID,
			rolloutPath: "/path",
			agentRole:   strings.Repeat("a", MaxCodexAgentRoleLength+1),
			wantErr:     true,
		},
		{
			name:          "agent_nickname too long",
			threadUUID:    validUUID,
			rolloutPath:   "/path",
			agentNickname: strings.Repeat("a", MaxCodexAgentNicknameLength+1),
			wantErr:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateCodexRolloutMetadata(
				tt.threadUUID, tt.parentThreadUUID,
				tt.rolloutPath, tt.cwd, tt.model, tt.source, tt.threadSource,
				tt.agentPath, tt.agentRole, tt.agentNickname,
			)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateCodexRolloutMetadata() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestValidateGitInfo covers CF-494's wire-layer validation of the new
// `remotes` and `tracking_remote` fields. Old-shape payloads pass through
// untouched; the validator only enforces caps and per-entry shape when the
// new fields are present.
func TestValidateGitInfo(t *testing.T) {
	// manyRemotes builds a valid remotes-only payload with n entries, each with
	// a unique name so the cap check (not duplicate-name behavior) is exercised.
	manyRemotes := func(n int) string {
		var sb strings.Builder
		sb.WriteString(`{"remotes":[`)
		for i := 0; i < n; i++ {
			if i > 0 {
				sb.WriteString(",")
			}
			fmt.Fprintf(&sb, `{"name":"r%d","fetch_url":"https://x/y.git"}`, i)
		}
		sb.WriteString(`]}`)
		return sb.String()
	}

	// nil and []byte{} are equivalent under len()==0 short-circuit; we exercise
	// the nil branch only and trust the implementation to behave the same on
	// an empty slice.
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"nil/empty bytes", "", false},
		{"malformed JSON tolerated", "not-json", false},
		{"old shape, no remotes", `{"repo_url":"https://github.com/me/repo.git"}`, false},
		{"valid full payload", `{
			"repo_url":"https://github.com/me/repo.git",
			"remotes":[{"name":"origin","fetch_url":"https://github.com/me/repo.git","push_url":"https://github.com/me/repo.git"}],
			"tracking_remote":"origin"
		}`, false},
		{"empty name in entry", `{"remotes":[{"name":"","fetch_url":"https://x.git"}]}`, true},
		{"both URLs empty in entry", `{"remotes":[{"name":"origin","fetch_url":"","push_url":""}]}`, true},
		{"name too long", `{"remotes":[{"name":"` + strings.Repeat("a", MaxRemoteNameLength+1) + `","fetch_url":"x"}]}`, true},
		{"fetch_url too long", `{"remotes":[{"name":"origin","fetch_url":"` + strings.Repeat("a", MaxRemoteURLLength+1) + `"}]}`, true},
		{"push_url too long", `{"remotes":[{"name":"origin","push_url":"` + strings.Repeat("a", MaxRemoteURLLength+1) + `"}]}`, true},
		{"tracking_remote too long", `{"tracking_remote":"` + strings.Repeat("a", MaxTrackingRemoteLength+1) + `"}`, true},
		{"too many remotes", manyRemotes(MaxGitRemotesCount + 1), true},
		{"exactly at cap", manyRemotes(MaxGitRemotesCount), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var in []byte
			if tt.input != "" {
				in = []byte(tt.input)
			}
			if err := ValidateGitInfo(in); (err != nil) != tt.wantErr {
				t.Errorf("ValidateGitInfo() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
