package validation

import (
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/ConfabulousDev/confab-web/internal/models"
)

// Field size limits (must match DB VARCHAR constraints in migration 000010, 000011)
const (
	MaxExternalIDLength       = 512  // sessions.external_id
	MaxSummaryLength          = 2048 // sessions.summary
	MaxFirstUserMessageLength = 8192 // sessions.first_user_message
	MaxCWDLength              = 8192 // sessions.cwd, runs.cwd
	MaxTranscriptPathLength   = 8192 // sessions.transcript_path, runs.transcript_path
	MaxSyncFileNameLength     = 512  // sync_files.file_name
	MaxHostnameLength         = 255  // sessions.hostname
	MaxUsernameLength         = 255  // sessions.username
	MaxAPIKeyNameLength       = 255  // api_keys.name

	// Filter parameter limits to prevent memory exhaustion from oversized query strings.
	MaxFilterCount    = 50   // max number of values per filter param
	FilterMaxLen      = 512  // maxLen of a single filter value
	MaxSearchQueryLen = 1024 // max length of the search query
)

// ValidateFilterValues validates a filter parameter's value count and individual lengths.
func ValidateFilterValues(name string, values []string) error {
	if len(values) > MaxFilterCount {
		return fmt.Errorf("%s filter exceeds maximum of %d values", name, MaxFilterCount)
	}
	for _, v := range values {
		if len(v) > FilterMaxLen {
			return fmt.Errorf("%s filter value exceeds maximum length of %d", name, FilterMaxLen)
		}
	}
	return nil
}

// ValidateSearchQuery validates a search query string.
func ValidateSearchQuery(q string) error {
	if len(q) > MaxSearchQueryLen {
		return fmt.Errorf("search query exceeds maximum length of %d", MaxSearchQueryLen)
	}
	return nil
}

// ValidateExternalID validates an external ID from URL parameters
// Returns error if external ID is invalid
func ValidateExternalID(externalID string) error {
	if externalID == "" {
		return fmt.Errorf("external_id is required")
	}
	if len(externalID) > MaxExternalIDLength {
		return fmt.Errorf("external_id exceeds maximum length of %d characters", MaxExternalIDLength)
	}
	if !utf8.ValidString(externalID) {
		return fmt.Errorf("external_id must be valid UTF-8")
	}
	return nil
}

// ValidateCWD validates a working directory path
func ValidateCWD(cwd string) error {
	if len(cwd) > MaxCWDLength {
		return fmt.Errorf("cwd exceeds maximum length of %d characters", MaxCWDLength)
	}
	return nil
}

// ValidateTranscriptPath validates a transcript file path
func ValidateTranscriptPath(path string) error {
	if len(path) > MaxTranscriptPathLength {
		return fmt.Errorf("transcript_path exceeds maximum length of %d characters", MaxTranscriptPathLength)
	}
	return nil
}

// ValidateSyncFileName validates a sync file name
func ValidateSyncFileName(fileName string) error {
	if len(fileName) > MaxSyncFileNameLength {
		return fmt.Errorf("file_name exceeds maximum length of %d characters", MaxSyncFileNameLength)
	}
	return nil
}

// ValidateSummary validates a session summary
func ValidateSummary(summary string) error {
	if len(summary) > MaxSummaryLength {
		return fmt.Errorf("summary exceeds maximum length of %d characters", MaxSummaryLength)
	}
	return nil
}

// ValidateFirstUserMessage validates a first user message
func ValidateFirstUserMessage(msg string) error {
	if len(msg) > MaxFirstUserMessageLength {
		return fmt.Errorf("first_user_message exceeds maximum length of %d characters", MaxFirstUserMessageLength)
	}
	return nil
}

// ValidateAPIKeyName validates an API key name
func ValidateAPIKeyName(name string) error {
	if len(name) > MaxAPIKeyNameLength {
		return fmt.Errorf("key name exceeds maximum length of %d characters", MaxAPIKeyNameLength)
	}
	return nil
}

// ValidateHostname validates a client hostname
func ValidateHostname(hostname string) error {
	if len(hostname) > MaxHostnameLength {
		return fmt.Errorf("hostname exceeds maximum length of %d characters", MaxHostnameLength)
	}
	return nil
}

// ValidateUsername validates a client username
func ValidateUsername(username string) error {
	if len(username) > MaxUsernameLength {
		return fmt.Errorf("username exceeds maximum length of %d characters", MaxUsernameLength)
	}
	return nil
}

// ValidateProvider returns an error unless p exactly equals one of the
// canonical provider values in models.CanonicalProviders. The handler is
// responsible for defaulting a missing API field to
// models.ProviderClaudeCode before calling this — an explicit empty
// string is not accepted here. No trimming or case folding. Legacy DB
// values like "Claude Code" are NOT accepted on the wire; they exist
// only at the persistence layer via models.LegacyAliases.
func ValidateProvider(p string) error {
	if slices.Contains(models.CanonicalProviders, p) {
		return nil
	}
	return fmt.Errorf("unknown provider %q: must be one of %s",
		p, strings.Join(models.CanonicalProviders, ", "))
}

// Codex rollout metadata length limits. Match codex_rollouts column widths
// in migration 000044.
const (
	MaxCodexSourceLength        = 64
	MaxCodexModelLength         = 255
	MaxCodexThreadSourceLength  = 255
	MaxCodexAgentRoleLength     = 255
	MaxCodexAgentNicknameLength = 255
	MaxCodexRolloutPathLength   = 8192
	MaxCodexCWDLength           = 8192
	MaxCodexAgentPathLength     = 8192
)

// ValidateCodexRolloutMetadata enforces the codex_rollout sub-block contract
// from POST /api/v1/sync/chunk. The handler calls this only when the request
// carries the block; the provider-mismatch check (codex sessions only) is
// handled separately in the handler after session ownership is verified.
//
// parentThreadUUID is a pointer: nil means "field omitted" (root rollout);
// a pointer to an empty string is treated as a client bug and rejected.
func ValidateCodexRolloutMetadata(
	threadUUID string,
	parentThreadUUID *string,
	rolloutPath, cwd, model, source, threadSource,
	agentPath, agentRole, agentNickname string,
) error {
	if threadUUID == "" {
		return fmt.Errorf("thread_uuid is required")
	}
	if _, err := uuid.Parse(threadUUID); err != nil {
		return fmt.Errorf("thread_uuid must be a valid UUID")
	}
	if parentThreadUUID != nil {
		if *parentThreadUUID == "" {
			return fmt.Errorf("parent_thread_uuid must not be empty when provided (omit the field for root rollouts)")
		}
		if _, err := uuid.Parse(*parentThreadUUID); err != nil {
			return fmt.Errorf("parent_thread_uuid must be a valid UUID")
		}
		if *parentThreadUUID == threadUUID {
			return fmt.Errorf("parent_thread_uuid must not equal thread_uuid")
		}
	}
	if rolloutPath == "" {
		return fmt.Errorf("rollout_path is required")
	}
	// Length checks ordered to match the wire field order. Each pair is
	// (field-name-in-error, value, max).
	maxChecks := []struct {
		name  string
		value string
		max   int
	}{
		{"rollout_path", rolloutPath, MaxCodexRolloutPathLength},
		{"cwd", cwd, MaxCodexCWDLength},
		{"model", model, MaxCodexModelLength},
		{"source", source, MaxCodexSourceLength},
		{"thread_source", threadSource, MaxCodexThreadSourceLength},
		{"agent_path", agentPath, MaxCodexAgentPathLength},
		{"agent_role", agentRole, MaxCodexAgentRoleLength},
		{"agent_nickname", agentNickname, MaxCodexAgentNicknameLength},
	}
	for _, c := range maxChecks {
		if len(c.value) > c.max {
			return fmt.Errorf("%s exceeds maximum length of %d characters", c.name, c.max)
		}
	}
	return nil
}
