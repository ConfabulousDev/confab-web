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

	// Save state immediately so duplicate detection works even if backend is down
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

	// Get authenticated config
	uploadCfg, err := config.EnsureAuthenticated()
	if err != nil {
		return fmt.Errorf("not authenticated: %w", err)
	}

	// Initialize sync client
	client := sync.NewClient(uploadCfg)

	// Create watcher (starts tracking files immediately, even before backend connects)
	watcher := NewWatcher(d.transcriptPath)

	// Setup signal handling
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)

	// Try initial connection to backend (non-blocking if it fails)
	if err := d.tryInit(client, watcher); err != nil {
		logger.Warn("Backend init failed (will retry): %v", err)
	}

	// Start main loop - handles init retries and sync cycles
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
			// If not initialized yet, try to connect to backend
			if d.syncer == nil {
				if err := d.tryInit(client, watcher); err != nil {
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

// tryInit attempts to initialize the sync session with the backend
func (d *Daemon) tryInit(client *sync.Client, watcher *Watcher) error {
	initResp, err := client.Init(d.externalID, d.transcriptPath, d.cwd)
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
