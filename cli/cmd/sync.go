package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"time"

	"github.com/santaclaude2025/confab/pkg/daemon"
	"github.com/santaclaude2025/confab/pkg/discovery"
	"github.com/santaclaude2025/confab/pkg/logger"
	"github.com/santaclaude2025/confab/pkg/types"
	"github.com/spf13/cobra"
)

var bgDaemonData string // Hidden flag for daemon mode

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Incremental session sync daemon",
	Long: `Manage the incremental sync daemon that uploads session data
during active Claude Code sessions.

The daemon watches transcript files and uploads new content
to the cloud every 30 seconds.`,
}

var syncStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start sync daemon for a session",
	Long: `Start the sync daemon for a session. When called from a hook,
reads session info from stdin and starts a background daemon.

Can also be called manually with session info as JSON argument.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Daemon mode (called by detached process)
		if bgDaemonData != "" {
			return runDaemon(bgDaemonData)
		}

		// Otherwise, hook mode (read from stdin and spawn daemon)
		return syncStartFromHook()
	},
}

var syncStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop sync daemon for a session",
	Long: `Stop the sync daemon for a session. When called from a hook,
reads session info from stdin and signals the daemon to stop.

The daemon will perform a final sync before exiting.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return syncStopFromHook()
	},
}

var syncStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show status of running sync daemons",
	Long:  `Display information about all running sync daemons.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return showSyncStatus()
	},
}

func init() {
	rootCmd.AddCommand(syncCmd)
	syncCmd.AddCommand(syncStartCmd)
	syncCmd.AddCommand(syncStopCmd)
	syncCmd.AddCommand(syncStatusCmd)

	// Hidden flag for daemon mode
	syncStartCmd.Flags().StringVar(&bgDaemonData, "bg-daemon", "", "")
	syncStartCmd.Flags().MarkHidden("bg-daemon")
}

// syncStartFromHook handles starting the daemon from a SessionStart hook
func syncStartFromHook() error {
	logger.Info("Starting sync daemon (hook mode)")

	// Always output valid hook response, even on error
	defer func() {
		response := types.HookResponse{
			Continue:       true,
			StopReason:     "",
			SuppressOutput: false,
		}
		json.NewEncoder(os.Stdout).Encode(response)
	}()

	fmt.Fprintln(os.Stderr, "=== Confab: Starting Sync Daemon ===")
	fmt.Fprintln(os.Stderr)

	// Read hook input from stdin
	hookInput, err := discovery.ReadHookInput()
	if err != nil {
		logger.ErrorPrint("Error reading hook input: %v", err)
		return nil
	}

	// Display session info
	fmt.Fprintf(os.Stderr, "Session: %s\n", hookInput.SessionID[:8])
	fmt.Fprintf(os.Stderr, "Path:    %s\n", hookInput.TranscriptPath)
	fmt.Fprintln(os.Stderr)

	// Check if daemon already running for this session
	existingState, err := daemon.LoadState(hookInput.SessionID)
	if err != nil {
		logger.Warn("Error checking existing state: %v", err)
	}
	if existingState != nil && existingState.IsDaemonRunning() {
		fmt.Fprintln(os.Stderr, "Sync daemon already running")
		logger.Info("Daemon already running: pid=%d", existingState.PID)
		return nil
	}

	// Serialize hook input to pass to daemon
	hookInputJSON, err := json.Marshal(hookInput)
	if err != nil {
		logger.ErrorPrint("Error serializing hook input: %v", err)
		return nil
	}

	// Spawn detached daemon process
	if err := spawnDaemon(string(hookInputJSON)); err != nil {
		logger.ErrorPrint("Error spawning daemon: %v", err)
		return nil
	}

	fmt.Fprintln(os.Stderr, "Sync daemon started in background")
	logger.Info("Daemon spawned successfully")

	return nil
}

// spawnDaemon starts a detached daemon process
func spawnDaemon(hookInputJSON string) error {
	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	cmd := exec.Command(executable, "sync", "start", "--bg-daemon", hookInputJSON)

	// Detach from parent process group
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}

	// Redirect stdout/stderr to /dev/null (logs go to log file)
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.Stdin = nil

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start daemon: %w", err)
	}

	if err := cmd.Process.Release(); err != nil {
		return fmt.Errorf("failed to release daemon: %w", err)
	}

	return nil
}

// runDaemon runs the actual daemon process
func runDaemon(hookInputJSON string) error {
	logger.Info("Daemon process starting")

	var hookInput types.HookInput
	if err := json.Unmarshal([]byte(hookInputJSON), &hookInput); err != nil {
		return fmt.Errorf("failed to parse hook input: %w", err)
	}

	cfg := daemon.Config{
		ExternalID:     hookInput.SessionID,
		TranscriptPath: hookInput.TranscriptPath,
		CWD:            hookInput.CWD,
		SyncInterval:   daemon.DefaultSyncInterval,
	}

	d := daemon.New(cfg)
	return d.Run(context.Background())
}

// syncStopFromHook handles stopping the daemon from a SessionEnd hook
func syncStopFromHook() error {
	logger.Info("Stopping sync daemon (hook mode)")

	// Always output valid hook response, even on error
	defer func() {
		response := types.HookResponse{
			Continue:       true,
			StopReason:     "",
			SuppressOutput: false,
		}
		json.NewEncoder(os.Stdout).Encode(response)
	}()

	fmt.Fprintln(os.Stderr, "=== Confab: Stopping Sync Daemon ===")
	fmt.Fprintln(os.Stderr)

	// Read hook input from stdin
	hookInput, err := discovery.ReadHookInput()
	if err != nil {
		logger.ErrorPrint("Error reading hook input: %v", err)
		return nil
	}

	// Signal daemon to stop (it will do final sync in background)
	if err := daemon.StopDaemon(hookInput.SessionID); err != nil {
		logger.Warn("Could not stop daemon: %v", err)
		fmt.Fprintf(os.Stderr, "Note: %v\n", err)
		// Fall back to regular save
		fmt.Fprintln(os.Stderr, "Falling back to full session upload...")
		// TODO: call existing save logic here
	} else {
		fmt.Fprintln(os.Stderr, "Daemon signaled to stop (final sync in background)")
	}

	return nil
}

// showSyncStatus displays all running sync daemons
func showSyncStatus() error {
	states, err := daemon.ListAllStates()
	if err != nil {
		return fmt.Errorf("failed to list daemon states: %w", err)
	}

	if len(states) == 0 {
		fmt.Println("No sync daemons running")
		return nil
	}

	fmt.Printf("Running sync daemons:\n\n")

	for _, state := range states {
		running := state.IsDaemonRunning()
		status := "running"
		if !running {
			status = "not running (stale)"
		}

		fmt.Printf("Session: %s\n", state.ExternalID[:8])
		fmt.Printf("  Status:  %s\n", status)
		fmt.Printf("  PID:     %d\n", state.PID)
		fmt.Printf("  Started: %s\n", state.StartedAt.Format(time.RFC3339))
		fmt.Printf("  Path:    %s\n", state.TranscriptPath)
		fmt.Println()
	}

	return nil
}
