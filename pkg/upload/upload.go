package upload

import (
	"github.com/santaclaude2025/confab/pkg/types"
)

// UploadToCloud uploads session data to the cloud (STUB)
// This is a placeholder for future implementation
func UploadToCloud(hookInput *types.HookInput, files []types.SessionFile) error {
	// TODO: Implement cloud upload
	// 1. Compress files to tar.zstd
	// 2. Upload to cloud API
	// 3. Return upload ID / URL

	// For now, this is a no-op
	return nil
}
