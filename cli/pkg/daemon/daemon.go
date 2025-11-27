package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/santaclaude2025/confab/pkg/config"
	"github.com/santaclaude2025/confab/pkg/git"
	"github.com/santaclaude2025/confab/pkg/logger"
	"github.com/santaclaude2025/confab/pkg/sync"
)

const (
	// DefaultSyncInterval is the base interval for syncing files
	DefaultSyncInterval = 30 * time.Second

	// syncIntervalJitter is random jitter added to sync interval (0 to this value)
	syncIntervalJitter = 5 * time.Second

	// initialWaitTimeout is how long to wait for transcript file to appear
	initialWaitTimeout = 60 * time.Second

	// initialWaitPollInterval is how often to check for transcript file
	initialWaitPollInterval = 2 * time.Second
)

// Daemon is the background sync process.
//
// The daemon is resilient to backend unavailability - it will keep running
// and retry connecting to the backend on each sync interval. Once connected,
// it will sync any accumulated changes.
type Daemon struct {
	externalID     string
	transcriptPath string
	cwd            string
	syncInterval   time.Duration

	state   *State
	syncer  *Syncer
	stopCh  chan struct{}
	doneCh  chan struct{}
}

// Config holds daemon configuration
type Config struct {
	ExternalID     string
	TranscriptPath string
	CWD            string
	SyncInterval   time.Duration
}

// New creates a new daemon instance
func New(cfg Config) *Daemon {
	interval := cfg.SyncInterval
	if interval == 0 {
		interval = DefaultSyncInterval
	}

	return &Daemon{
		externalID:     cfg.ExternalID,
		transcriptPath: cfg.TranscriptPath,
		cwd:            cfg.CWD,
		syncInterval:   interval,
		stopCh:         make(chan struct{}),
		doneCh:         make(chan struct{}),
	}
}

// Run starts the daemon and blocks until stopped
func (d *Daemon) Run(ctx context.Context) error {
	// Set session context for all log lines
	logger.SetSession(d.externalID, "")

	logger.Info("Daemon starting: transcript=%s interval=%v", d.transcriptPath, d.syncInterval)

	// Setup signal handling as early as possible to catch signals during
	// initialization (waiting for transcript, backend init).
	// See daemon_test.go for rationale.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)

	// Wait for transcript file to exist before doing anything else.
	// Don't save state or set up panic handlers until we have a transcript.
	if err := d.waitForTranscript(ctx, sigCh); err != nil {
		return err
	}

	// Save state for duplicate detection. Done after transcript exists so we
	// don't leave stale state files for sessions that never produced transcripts.
	d.state = NewState(d.externalID, d.transcriptPath, d.cwd)
	if err := d.state.Save(); err != nil {
		logger.Warn("Failed to save initial state: %v", err)
	}

	// Log panics before crashing. We skip final sync since the program is in an
	// undefined state, but we do delete the state file to avoid blocking future
	// daemon spawns. We log the panic since this CLI runs on many local machines
	// and we need the logs for debugging.
	defer func() {
		if r := recover(); r != nil {
			logger.Error("Daemon panic: %v", r)
			if d.state != nil {
				d.state.Delete()
			}
			panic(r)
		}
	}()

	// Create watcher (starts tracking files immediately, even before backend connects)
	watcher := NewWatcher(d.transcriptPath)

	// Try initial connection to backend (non-blocking if it fails)
	// Auth is checked lazily here, not at startup
	if err := d.tryInitAndSyncAll(watcher); err != nil {
		logger.Warn("Backend init failed (will retry): %v", err)
	}

	logger.Info("Daemon running: pid=%d", os.Getpid())

	// Main loop with jittered interval (30-35s) to avoid thundering herd
	for {
		jitter := time.Duration(rand.Int63n(int64(syncIntervalJitter)))
		timer := time.NewTimer(d.syncInterval + jitter)

		select {
		case <-ctx.Done():
			timer.Stop()
			return d.shutdown("context cancelled")

		case <-d.stopCh:
			timer.Stop()
			return d.shutdown("stop requested")

		case sig := <-sigCh:
			timer.Stop()
			return d.shutdown(fmt.Sprintf("signal %v", sig))

		case <-timer.C:
			// If not initialized yet, try to connect to backend
			if d.syncer == nil {
				if err := d.tryInitAndSyncAll(watcher); err != nil {
					logger.Warn("Backend init failed (will retry): %v", err)
					continue
				}
			}

			// Sync if initialized
			if d.syncer != nil {
				if chunks, err := d.syncer.SyncAll(); err != nil {
					logger.Warn("Sync cycle had errors: %v", err)
				} else if chunks > 0 {
					logger.Debug("Sync cycle complete: chunks=%d", chunks)
				}
			}
		}
	}
}

// waitForTranscript waits for the transcript file to exist before proceeding.
// For fresh sessions, Claude Code may not have written the transcript yet.
func (d *Daemon) waitForTranscript(ctx context.Context, sigCh chan os.Signal) error {
	// Check if file already exists
	if _, err := os.Stat(d.transcriptPath); err == nil {
		return nil
	}

	logger.Info("Waiting for transcript file to appear...")

	ticker := time.NewTicker(initialWaitPollInterval)
	defer ticker.Stop()

	timeout := time.After(initialWaitTimeout)

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("context cancelled while waiting for transcript")
		case <-d.stopCh:
			return fmt.Errorf("stop requested while waiting for transcript")
		case sig := <-sigCh:
			return fmt.Errorf("received signal %v while waiting for transcript", sig)
		case <-timeout:
			return fmt.Errorf("timeout waiting for transcript file after %v", initialWaitTimeout)
		case <-ticker.C:
			if _, err := os.Stat(d.transcriptPath); err == nil {
				logger.Info("Transcript file appeared")
				return nil
			}
		}
	}
}

// tryInitAndSyncAll attempts to initialize the sync session with the backend
// and performs an initial sync. Auth is checked here lazily, not at daemon startup.
func (d *Daemon) tryInitAndSyncAll(watcher *Watcher) error {
	// Get authenticated config (lazy - only when we need to talk to backend)
	cfg, err := config.EnsureAuthenticated()
	if err != nil {
		return fmt.Errorf("not authenticated: %w", err)
	}

	client := sync.NewClient(cfg)

	// Extract git info from transcript (source of truth), fall back to detecting from cwd
	var gitInfoJSON json.RawMessage
	gitInfo, _ := git.ExtractGitInfoFromTranscript(d.transcriptPath)
	if gitInfo == nil {
		// Fallback: detect from directory if transcript doesn't have git info
		gitInfo, _ = git.DetectGitInfo(d.cwd)
	}
	if gitInfo != nil {
		if data, err := json.Marshal(gitInfo); err != nil {
			logger.Warn("Failed to marshal git info: %v", err)
		} else {
			gitInfoJSON = data
			logger.Debug("Git info: branch=%s repo=%s", gitInfo.Branch, gitInfo.RepoURL)
		}
	}

	initResp, err := client.Init(d.externalID, d.transcriptPath, d.cwd, gitInfoJSON)
	if err != nil {
		return err
	}

	// Update session context now that we have the backend session ID
	logger.SetSession(d.externalID, initResp.SessionID)

	logger.Info("Sync session initialized: existing_files=%d", len(initResp.Files))

	// Initialize watcher from backend state
	backendState := make(map[string]FileState)
	for fileName, state := range initResp.Files {
		backendState[fileName] = FileState{LastSyncedLine: state.LastSyncedLine}
	}
	watcher.InitFromState(backendState)

	// Create syncer
	d.syncer = NewSyncer(client, initResp.SessionID, watcher)

	// Do initial sync immediately
	if chunks, err := d.syncer.SyncAll(); err != nil {
		logger.Warn("Initial sync had errors: %v", err)
	} else if chunks > 0 {
		logger.Info("Initial sync complete: chunks=%d", chunks)
	}

	return nil
}

// Stop signals the daemon to stop
func (d *Daemon) Stop() {
	close(d.stopCh)
}

// Done returns a channel that's closed when the daemon exits
func (d *Daemon) Done() <-chan struct{} {
	return d.doneCh
}

// shutdown performs final sync and cleanup
func (d *Daemon) shutdown(reason string) error {
	defer close(d.doneCh)

	logger.Info("Daemon shutting down: reason=%s", reason)

	// Final sync
	if d.syncer != nil {
		logger.Info("Performing final sync...")
		if chunks, err := d.syncer.SyncAll(); err != nil {
			logger.Error("Final sync had errors: %v", err)
		} else if chunks > 0 {
			logger.Info("Final sync complete: chunks=%d", chunks)
		} else {
			logger.Info("Final sync complete: already up to date")
		}

		// Log final stats
		stats := d.syncer.GetSyncStats()
		for file, lines := range stats {
			logger.Info("Final state: file=%s lines_synced=%d", file, lines)
		}
	}

	// Clean up state file
	if d.state != nil {
		if err := d.state.Delete(); err != nil {
			logger.Warn("Failed to delete state file: %v", err)
		}
	}

	logger.Info("Daemon stopped")
	return nil
}

// StopDaemon sends SIGTERM to a running daemon by external ID
func StopDaemon(externalID string) error {
	state, err := LoadState(externalID)
	if err != nil {
		return fmt.Errorf("failed to load state: %w", err)
	}
	if state == nil {
		return fmt.Errorf("no daemon found for session %s", externalID)
	}

	if !state.IsDaemonRunning() {
		// Clean up stale state file
		state.Delete()
		return fmt.Errorf("daemon not running (stale state cleaned up)")
	}

	// Send SIGTERM
	process, err := os.FindProcess(state.PID)
	if err != nil {
		return fmt.Errorf("failed to find process: %w", err)
	}

	if err := process.Signal(syscall.SIGTERM); err != nil {
		return fmt.Errorf("failed to send SIGTERM: %w", err)
	}

	logger.Info("Sent SIGTERM to daemon: pid=%d", state.PID)
	return nil
}
