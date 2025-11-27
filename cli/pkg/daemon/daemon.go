package daemon

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/santaclaude2025/confab/pkg/config"
	"github.com/santaclaude2025/confab/pkg/logger"
	"github.com/santaclaude2025/confab/pkg/sync"
)

const (
	// DefaultSyncInterval is how often the daemon syncs files
	DefaultSyncInterval = 30 * time.Second
)

// Daemon is the background sync process.
//
// NOTE: There is a race condition where multiple daemons could be spawned for
// the same session if Claude Code is started twice in quick succession. The
// check in syncStartFromHook() looks for an existing state file, but the state
// file is only written after the daemon initializes. If two hooks run before
// either daemon writes its state, both will spawn. The backend's chunk
// continuity validation will reject duplicate/overlapping chunks, so data
// integrity is preserved, but it's wasteful. A proper fix would use a lock
// file before spawning, but this edge case is rare enough to not warrant the
// added complexity.
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
	logger.Info("Daemon starting: external_id=%s transcript=%s interval=%v",
		d.externalID, d.transcriptPath, d.syncInterval)

	// Get authenticated config
	uploadCfg, err := config.EnsureAuthenticated()
	if err != nil {
		return fmt.Errorf("not authenticated: %w", err)
	}

	// Initialize sync client
	client := sync.NewClient(uploadCfg)

	// Initialize or resume session with backend
	initResp, err := client.Init(d.externalID, d.transcriptPath, d.cwd)
	if err != nil {
		return fmt.Errorf("failed to init sync session: %w", err)
	}

	logger.Info("Sync session initialized: session_id=%s existing_files=%d",
		initResp.SessionID, len(initResp.Files))

	// Create watcher and initialize from backend state
	watcher := NewWatcher(d.transcriptPath)
	backendState := make(map[string]FileState)
	for fileName, state := range initResp.Files {
		backendState[fileName] = FileState{LastSyncedLine: state.LastSyncedLine}
	}
	watcher.InitFromState(backendState)

	// Create local state
	d.state = NewState(d.externalID, initResp.SessionID, d.transcriptPath, d.cwd)
	for fileName, state := range backendState {
		d.state.UpdateFileState(fileName, state.LastSyncedLine)
	}

	// Save initial state
	if err := d.state.Save(); err != nil {
		logger.Warn("Failed to save initial state: %v", err)
	}

	// Create syncer
	d.syncer = NewSyncer(client, initResp.SessionID, watcher, d.state)

	// Setup signal handling
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)

	// Do initial sync
	if chunks, err := d.syncer.SyncAll(); err != nil {
		logger.Warn("Initial sync had errors: %v", err)
	} else if chunks > 0 {
		logger.Info("Initial sync complete: chunks=%d", chunks)
	}

	// Start sync loop
	ticker := time.NewTicker(d.syncInterval)
	defer ticker.Stop()

	logger.Info("Daemon running: pid=%d", os.Getpid())

	for {
		select {
		case <-ctx.Done():
			return d.shutdown("context cancelled")

		case <-d.stopCh:
			return d.shutdown("stop requested")

		case sig := <-sigCh:
			return d.shutdown(fmt.Sprintf("signal %v", sig))

		case <-ticker.C:
			if chunks, err := d.syncer.SyncAll(); err != nil {
				logger.Warn("Sync cycle had errors: %v", err)
			} else if chunks > 0 {
				logger.Debug("Sync cycle complete: chunks=%d", chunks)
			}
		}
	}
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
