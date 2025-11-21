package cmd

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/klauspost/compress/zstd"
	"github.com/santaclaude2025/confab/pkg/config"
	"github.com/santaclaude2025/confab/pkg/discovery"
	"github.com/santaclaude2025/confab/pkg/types"
	"github.com/spf13/cobra"
)

// SessionInfo holds metadata about a discovered session
type SessionInfo struct {
	SessionID      string
	TranscriptPath string
	ProjectPath    string
	ModTime        time.Time
	SizeBytes      int64
}

var backfillCmd = &cobra.Command{
	Use:   "backfill",
	Short: "Upload historical sessions from ~/.claude to cloud",
	Long: `Scans ~/.claude/projects/ for existing session transcripts and uploads
them to the cloud backend. Sessions modified within the last hour are skipped
(likely still in progress). Use 'confab save <session-id>' to force upload
a specific session.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("=== Confab: Backfill Historical Sessions ===")
		fmt.Println()

		// Check authentication
		cfg, err := config.GetUploadConfig()
		if err != nil {
			return fmt.Errorf("failed to get config: %w", err)
		}
		if cfg.APIKey == "" {
			return fmt.Errorf("not authenticated. Run 'confab login' first")
		}

		// Scan for sessions
		fmt.Println("Scanning ~/.claude/projects...")
		sessions, err := scanForSessions()
		if err != nil {
			return fmt.Errorf("failed to scan for sessions: %w", err)
		}

		if len(sessions) == 0 {
			fmt.Println("No sessions found in ~/.claude/projects/")
			return nil
		}

		fmt.Printf("Found %d session(s)\n", len(sessions))
		fmt.Println()

		// Separate recent vs old sessions (1 hour threshold)
		threshold := time.Now().Add(-1 * time.Hour)
		var oldSessions, recentSessions []SessionInfo
		for _, s := range sessions {
			if s.ModTime.Before(threshold) {
				oldSessions = append(oldSessions, s)
			} else {
				recentSessions = append(recentSessions, s)
			}
		}

		// Check which old sessions already exist on server
		var toUpload []SessionInfo
		var alreadySynced []string
		if len(oldSessions) > 0 {
			sessionIDs := make([]string, len(oldSessions))
			for i, s := range oldSessions {
				sessionIDs[i] = s.SessionID
			}

			existing, err := checkSessionsExist(cfg, sessionIDs)
			if err != nil {
				return fmt.Errorf("failed to check existing sessions: %w", err)
			}

			existingSet := make(map[string]bool)
			for _, id := range existing {
				existingSet[id] = true
			}

			for _, s := range oldSessions {
				if existingSet[s.SessionID] {
					alreadySynced = append(alreadySynced, s.SessionID)
				} else {
					toUpload = append(toUpload, s)
				}
			}
		}

		// Print summary
		if len(alreadySynced) > 0 {
			fmt.Printf("Already synced: %d\n", len(alreadySynced))
		}
		fmt.Printf("To upload: %d\n", len(toUpload))

		// Show skipped recent sessions
		if len(recentSessions) > 0 {
			fmt.Println()
			fmt.Printf("Skipping %d recent session(s) (modified < 1 hour ago):\n", len(recentSessions))
			for _, s := range recentSessions {
				ago := time.Since(s.ModTime).Round(time.Minute)
				fmt.Printf("  %s  %-20s  modified %s ago\n", s.SessionID[:8], truncatePath(s.ProjectPath, 20), ago)
			}
			fmt.Println()
			fmt.Println("To upload a skipped session later, run: confab save <session-id>")
		}

		if len(toUpload) == 0 {
			fmt.Println()
			fmt.Println("Nothing to upload.")
			return nil
		}

		// Prompt for confirmation
		fmt.Println()
		fmt.Printf("Proceed with uploading %d session(s)? [Y/n]: ", len(toUpload))
		reader := bufio.NewReader(os.Stdin)
		response, _ := reader.ReadString('\n')
		response = strings.TrimSpace(strings.ToLower(response))
		if response != "" && response != "y" && response != "yes" {
			fmt.Println("Cancelled.")
			return nil
		}

		// Upload sessions
		fmt.Println()
		var succeeded, failed int
		for i, session := range toUpload {
			fmt.Printf("\rUploading... [%d/%d] %s", i+1, len(toUpload), session.SessionID[:8])

			err := uploadSession(cfg, session)
			if err != nil {
				fmt.Printf("\n  Error uploading %s: %v\n", session.SessionID[:8], err)
				failed++
			} else {
				succeeded++
			}
		}

		fmt.Printf("\rUploading... [%d/%d] Done.                    \n", len(toUpload), len(toUpload))
		fmt.Println()

		if failed > 0 {
			fmt.Printf("Uploaded %d session(s), %d failed.\n", succeeded, failed)
		} else {
			fmt.Printf("Uploaded %d session(s).\n", succeeded)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(backfillCmd)
}

// scanForSessions finds all session transcript files in ~/.claude/projects/
func scanForSessions() ([]SessionInfo, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	projectsDir := filepath.Join(home, ".claude", "projects")
	if _, err := os.Stat(projectsDir); os.IsNotExist(err) {
		return nil, nil
	}

	var sessions []SessionInfo

	// Walk through all project directories
	err = filepath.WalkDir(projectsDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // Skip errors
		}

		// Look for .jsonl files that are session transcripts
		// Session files: {uuid}.jsonl (36 chars + .jsonl = 42 chars)
		// Agent files: agent-{8chars}.jsonl - skip these
		if !d.IsDir() && strings.HasSuffix(path, ".jsonl") {
			name := d.Name()

			// Skip agent files
			if strings.HasPrefix(name, "agent-") {
				return nil
			}

			// Session ID is the filename without extension
			sessionID := strings.TrimSuffix(name, ".jsonl")

			// Validate it looks like a UUID (36 chars with hyphens)
			if len(sessionID) != 36 {
				return nil
			}

			info, err := d.Info()
			if err != nil {
				return nil
			}

			// Get project path (parent directory relative to projects/)
			relPath, _ := filepath.Rel(projectsDir, filepath.Dir(path))

			sessions = append(sessions, SessionInfo{
				SessionID:      sessionID,
				TranscriptPath: path,
				ProjectPath:    relPath,
				ModTime:        info.ModTime(),
				SizeBytes:      info.Size(),
			})
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	// Sort by mod time (oldest first)
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].ModTime.Before(sessions[j].ModTime)
	})

	return sessions, nil
}

// checkSessionsExist calls the backend to check which sessions already exist
func checkSessionsExist(cfg *config.UploadConfig, sessionIDs []string) ([]string, error) {
	if len(sessionIDs) == 0 {
		return nil, nil
	}

	// Build request
	reqBody := struct {
		SessionIDs []string `json:"session_ids"`
	}{
		SessionIDs: sessionIDs,
	}

	payload, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	url := cfg.BackendURL + "/api/v1/sessions/check"
	req, err := http.NewRequest("POST", url, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Existing []string `json:"existing"`
		Missing  []string `json:"missing"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.Existing, nil
}

// uploadSession uploads a single session to the backend
func uploadSession(cfg *config.UploadConfig, session SessionInfo) error {
	// Create a hook input for discovery
	hookInput := &types.HookInput{
		SessionID:      session.SessionID,
		TranscriptPath: session.TranscriptPath,
		CWD:            filepath.Dir(session.TranscriptPath),
		Reason:         "backfill",
	}

	// Discover session files
	files, err := discovery.DiscoverSessionFiles(hookInput)
	if err != nil {
		return err
	}

	// Read file contents
	fileUploads := make([]fileUpload, 0, len(files))
	for _, f := range files {
		content, err := os.ReadFile(f.Path)
		if err != nil {
			return fmt.Errorf("failed to read file %s: %w", f.Path, err)
		}

		fileUploads = append(fileUploads, fileUpload{
			Path:      f.Path,
			Type:      f.Type,
			SizeBytes: f.SizeBytes,
			Content:   content,
		})
	}

	// Create request payload
	request := saveSessionRequest{
		SessionID:      session.SessionID,
		TranscriptPath: session.TranscriptPath,
		CWD:            hookInput.CWD,
		Reason:         "backfill",
		Source:         "backfill",
		Files:          fileUploads,
	}

	// Marshal to JSON
	payload, err := json.Marshal(request)
	if err != nil {
		return err
	}

	// Compress with zstd
	var compressedPayload bytes.Buffer
	encoder, err := zstd.NewWriter(&compressedPayload, zstd.WithEncoderLevel(zstd.SpeedDefault))
	if err != nil {
		return err
	}

	_, err = encoder.Write(payload)
	if err != nil {
		encoder.Close()
		return err
	}

	if err := encoder.Close(); err != nil {
		return err
	}

	// Send HTTP request
	url := cfg.BackendURL + "/api/v1/sessions/save"
	req, err := http.NewRequest("POST", url, bytes.NewReader(compressedPayload.Bytes()))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Encoding", "zstd")
	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)

	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// Local types for backfill (to avoid circular imports)
type saveSessionRequest struct {
	SessionID      string       `json:"session_id"`
	TranscriptPath string       `json:"transcript_path"`
	CWD            string       `json:"cwd"`
	Reason         string       `json:"reason"`
	Source         string       `json:"source,omitempty"`
	Files          []fileUpload `json:"files"`
}

type fileUpload struct {
	Path      string `json:"path"`
	Type      string `json:"type"`
	SizeBytes int64  `json:"size_bytes"`
	Content   []byte `json:"content"`
}

// truncatePath shortens a path for display
func truncatePath(path string, maxLen int) string {
	if len(path) <= maxLen {
		return path
	}
	return "..." + path[len(path)-maxLen+3:]
}
