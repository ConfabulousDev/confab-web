package discovery

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/santaclaude2025/confab/pkg/types"
)

// ReadHookInput reads and parses SessionEnd hook data from stdin
func ReadHookInput() (*types.HookInput, error) {
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		return nil, fmt.Errorf("failed to read stdin: %w", err)
	}

	var input types.HookInput
	if err := json.Unmarshal(data, &input); err != nil {
		return nil, fmt.Errorf("failed to parse hook input: %w", err)
	}

	// Basic validation
	if input.SessionID == "" {
		return nil, fmt.Errorf("session_id is required")
	}
	if input.TranscriptPath == "" {
		return nil, fmt.Errorf("transcript_path is required")
	}

	return &input, nil
}
