package events_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/ConfabulousDev/confab-web/internal/db"
	dbevents "github.com/ConfabulousDev/confab-web/internal/db/events"
	"github.com/ConfabulousDev/confab-web/internal/testutil"
)

// TestInsertSessionEvent_HappyPath verifies a row is written with all fields,
// including the JSONB payload, intact.
func TestInsertSessionEvent_HappyPath(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	env := testutil.SetupTestEnvironment(t)
	defer env.Cleanup(t)

	user := testutil.CreateTestUser(t, env, "events@example.com", "Events User")
	sessionID := testutil.CreateTestSession(t, env, user.ID, "ext-events-happy")

	store := &dbevents.Store{DB: env.DB}
	ctx := context.Background()
	ts := time.Date(2026, 5, 16, 10, 0, 0, 0, time.UTC)
	payload := json.RawMessage(`{"reason":"timeout","seq":42}`)

	if err := store.InsertSessionEvent(ctx, db.SessionEventParams{
		SessionID:      sessionID,
		EventType:      "session_end",
		EventTimestamp: ts,
		Payload:        payload,
	}); err != nil {
		t.Fatalf("InsertSessionEvent: %v", err)
	}

	var (
		gotSessionID string
		gotType      string
		gotTs        time.Time
		gotPayload   []byte
	)
	row := env.DB.QueryRow(ctx,
		"SELECT session_id::text, event_type, event_timestamp, payload FROM session_events WHERE session_id = $1",
		sessionID,
	)
	if err := row.Scan(&gotSessionID, &gotType, &gotTs, &gotPayload); err != nil {
		t.Fatalf("read-back failed: %v", err)
	}
	if gotSessionID != sessionID {
		t.Errorf("session_id = %q, want %q", gotSessionID, sessionID)
	}
	if gotType != "session_end" {
		t.Errorf("event_type = %q, want session_end", gotType)
	}
	if !gotTs.Equal(ts) {
		t.Errorf("event_timestamp = %v, want %v", gotTs, ts)
	}
	var decoded map[string]any
	if err := json.Unmarshal(gotPayload, &decoded); err != nil {
		t.Fatalf("payload is not valid JSON: %v", err)
	}
	if decoded["reason"] != "timeout" {
		t.Errorf("payload.reason = %v, want timeout", decoded["reason"])
	}
	if decoded["seq"] != float64(42) {
		t.Errorf("payload.seq = %v, want 42", decoded["seq"])
	}
}

// TestInsertSessionEvent_NilPayload covers the JSONB-null code path.
func TestInsertSessionEvent_NilPayload(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	env := testutil.SetupTestEnvironment(t)
	defer env.Cleanup(t)

	user := testutil.CreateTestUser(t, env, "events-nil@example.com", "Events Nil User")
	sessionID := testutil.CreateTestSession(t, env, user.ID, "ext-events-nil")

	store := &dbevents.Store{DB: env.DB}
	ctx := context.Background()

	if err := store.InsertSessionEvent(ctx, db.SessionEventParams{
		SessionID:      sessionID,
		EventType:      "session_end",
		EventTimestamp: time.Now().UTC(),
		Payload:        nil,
	}); err != nil {
		t.Fatalf("InsertSessionEvent with nil payload: %v", err)
	}

	var payload *string
	row := env.DB.QueryRow(ctx,
		"SELECT payload::text FROM session_events WHERE session_id = $1",
		sessionID,
	)
	if err := row.Scan(&payload); err != nil {
		t.Fatalf("read-back failed: %v", err)
	}
	if payload != nil {
		t.Errorf("payload = %q, want NULL", *payload)
	}
}

// TestInsertSessionEvent_RejectsBadInput verifies the two server-side guards
// against malformed insertions: the FK on session_id and the CHECK constraint
// on event_type. Both subtests share one env because they only assert that an
// error is returned and write no rows on success.
func TestInsertSessionEvent_RejectsBadInput(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	env := testutil.SetupTestEnvironment(t)
	defer env.Cleanup(t)

	user := testutil.CreateTestUser(t, env, "events-bad@example.com", "Events Bad User")
	validSessionID := testutil.CreateTestSession(t, env, user.ID, "ext-events-bad")
	store := &dbevents.Store{DB: env.DB}

	cases := []struct {
		name      string
		params    db.SessionEventParams
		wantError string
	}{
		{
			name: "nonexistent_session_fk",
			params: db.SessionEventParams{
				SessionID:      uuid.New().String(), // valid UUID, no matching session
				EventType:      "session_end",
				EventTimestamp: time.Now().UTC(),
			},
			wantError: "FK violation for nonexistent session_id",
		},
		{
			name: "invalid_event_type_check_constraint",
			params: db.SessionEventParams{
				SessionID:      validSessionID,
				EventType:      "not_a_real_event_type",
				EventTimestamp: time.Now().UTC(),
			},
			wantError: "CHECK constraint violation for unknown event_type",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := store.InsertSessionEvent(context.Background(), tc.params); err == nil {
				t.Fatalf("expected %s", tc.wantError)
			}
		})
	}
}
