package daemon

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/santaclaude2025/confab/pkg/config"
	"github.com/santaclaude2025/confab/pkg/git"
	"github.com/santaclaude2025/confab/pkg/http"
	"github.com/santaclaude2025/confab/pkg/logger"
	"github.com/santaclaude2025/confab/pkg/sync"
	"github.com/santaclaude2025/confab/pkg/types"
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
//
// If ParentPID is set, the daemon monitors the parent process and shuts down
// gracefully when it exits. This handles cases where Claude Code crashes or
// is killed without firing the SessionEnd hook.
type Daemon struct {
	externalID     string
	transcriptPath string
	cwd            string
	parentPID      int
	syncInterval   time.Duration
	syncJitter     time.Duration

	state   *State
	syncer  *Syncer
	stopCh  chan struct{}
	doneCh  chan struct{}
}

// Config holds daemon configuration
type Config struct {
	ExternalID         string
	TranscriptPath     string
	CWD                string
	ParentPID          int           // Claude Code process ID to monitor (0 to disable)
	SyncInterval       time.Duration
	SyncIntervalJitter time.Duration // 0 to disable jitter (for testing)
}

// New creates a new daemon instance
func New(cfg Config) *Daemon {
	interval := cfg.SyncInterval
	if interval == 0 {
		interval = DefaultSyncInterval
	}

	jitter := cfg.SyncIntervalJitter
	if jitter == 0 && cfg.SyncInterval == 0 {
		// Only use default jitter if using default interval
		jitter = syncIntervalJitter
	}

	return &Daemon{
		externalID:     cfg.ExternalID,
		transcriptPath: cfg.TranscriptPath,
		cwd:            cfg.CWD,
		parentPID:      cfg.ParentPID,
		syncInterval:   interval,
		syncJitter:     jitter,
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
	d.state = NewState(d.externalID, d.transcriptPath, d.cwd, d.parentPID)
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

	if d.parentPID > 0 {
		logger.Info("Daemon running: pid=%d parent_pid=%d", os.Getpid(), d.parentPID)
	} else {
		logger.Info("Daemon running: pid=%d (no parent monitoring)", os.Getpid())
	}

	// Main loop with jittered interval to avoid thundering herd
	for {
		var jitter time.Duration
		if d.syncJitter > 0 {
			jitter = time.Duration(rand.Int63n(int64(d.syncJitter)))
		}
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
			// Check if parent Claude Code process is still running.
			// If it crashed or was killed, shut down gracefully.
			if d.parentPID > 0 && !isProcessRunning(d.parentPID) {
				timer.Stop()
				return d.shutdown("parent process exited")
			}

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
					// If auth failed, reset syncer to force config re-read on next cycle.
					// User may have re-authenticated with a new API key.
					if errors.Is(err, http.ErrUnauthorized) {
						logger.Info("Auth failed, will re-read config on next cycle")
						d.syncer = nil
					}
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

	// Read inbox events (e.g., SessionEnd payload from sync stop)
	// We need to extract the session_end event to send to backend after final sync
	events := d.readInboxEvents()
	var sessionEndEvent *types.InboxEvent
	for _, event := range events {
		logger.Info("Processing inbox event: type=%s", event.Type)
		if event.Type == "session_end" && event.HookInput != nil {
			logger.Debug("SessionEnd event: reason=%s", event.HookInput.Reason)
			sessionEndEvent = &event
		}
	}

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

		// Send session_end event to backend (after final sync completes)
		if sessionEndEvent != nil {
			if err := d.syncer.SendSessionEndEvent(sessionEndEvent.HookInput, sessionEndEvent.Timestamp); err != nil {
				logger.Error("Failed to send session_end event: %v", err)
				// Don't fail shutdown for this - the sync already completed
			}
		}
	}

	// Clean up state and inbox files
	if d.state != nil {
		d.cleanupInbox()
		if err := d.state.Delete(); err != nil {
			logger.Warn("Failed to delete state file: %v", err)
		}
	}

	logger.Info("Daemon stopped")
	return nil
}

// readInboxEvents reads all events from the inbox file
func (d *Daemon) readInboxEvents() []types.InboxEvent {
	if d.state == nil || d.state.InboxPath == "" {
		return nil
	}

	f, err := os.Open(d.state.InboxPath)
	if err != nil {
		if !os.IsNotExist(err) {
			logger.Warn("Failed to open inbox file: %v", err)
		}
		return nil
	}
	defer f.Close()

	var events []types.InboxEvent
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var event types.InboxEvent
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			logger.Warn("Failed to parse inbox event: %v", err)
			continue
		}
		events = append(events, event)
	}

	if err := scanner.Err(); err != nil {
		logger.Warn("Error reading inbox file: %v", err)
	}

	return events
}

// cleanupInbox removes the inbox file
func (d *Daemon) cleanupInbox() {
	if d.state == nil || d.state.InboxPath == "" {
		return
	}
	if err := os.Remove(d.state.InboxPath); err != nil && !os.IsNotExist(err) {
		logger.Warn("Failed to delete inbox file: %v", err)
	}
}

// StopDaemon sends SIGTERM to a running daemon by external ID.
// If hookInput is provided, it writes a session_end event to the daemon's inbox
// before signaling, so the daemon can access the full SessionEnd payload.
func StopDaemon(externalID string, hookInput *types.HookInput) error {
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

	// Write event to inbox before signaling (daemon reads on shutdown)
	if hookInput != nil && state.InboxPath != "" {
		if err := writeInboxEvent(state.InboxPath, "session_end", hookInput); err != nil {
			logger.Warn("Failed to write inbox event: %v", err)
			// Continue anyway - daemon can still do final sync without the event
		}
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

// writeInboxEvent appends an event to the inbox JSONL file
func writeInboxEvent(inboxPath string, eventType string, hookInput *types.HookInput) error {
	event := types.InboxEvent{
		Type:      eventType,
		Timestamp: time.Now(),
		HookInput: hookInput,
	}

	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	// Append to file (create if doesn't exist)
	f, err := os.OpenFile(inboxPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("failed to open inbox file: %w", err)
	}
	defer f.Close()

	if _, err := f.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("failed to write event: %w", err)
	}

	return nil
}
