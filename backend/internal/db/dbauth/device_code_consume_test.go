package dbauth_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/ConfabulousDev/confab-web/internal/auth"
	"github.com/ConfabulousDev/confab-web/internal/db"
	"github.com/ConfabulousDev/confab-web/internal/db/dbauth"
	"github.com/ConfabulousDev/confab-web/internal/testutil"
)

// deviceCodeExists reports whether the device_codes row for a raw device code
// is still present (i.e. not consumed).
func deviceCodeExists(t *testing.T, env *testutil.TestEnvironment, deviceCode string) bool {
	t.Helper()
	var n int
	if err := env.DB.QueryRow(env.Ctx,
		`SELECT COUNT(*) FROM device_codes WHERE device_code = $1`,
		db.HashToken(deviceCode)).Scan(&n); err != nil {
		t.Fatalf("count device codes: %v", err)
	}
	return n > 0
}

// apiKeyCountByName returns how many api_keys rows a user has with a given name.
func apiKeyCountByName(t *testing.T, env *testutil.TestEnvironment, userID int64, name string) int {
	t.Helper()
	var n int
	if err := env.DB.QueryRow(env.Ctx,
		`SELECT COUNT(*) FROM api_keys WHERE user_id = $1 AND name = $2`,
		userID, name).Scan(&n); err != nil {
		t.Fatalf("count api keys: %v", err)
	}
	return n
}

// newAuthorizedDeviceCode creates a device code and authorizes it for userID.
func newAuthorizedDeviceCode(t *testing.T, env *testutil.TestEnvironment, base, userCode, keyName string, userID int64) string {
	t.Helper()
	deviceCode := makeDeviceCode(base)
	testutil.CreateTestDeviceCode(t, env, deviceCode, userCode, keyName, time.Now().UTC().Add(5*time.Minute))
	testutil.AuthorizeTestDeviceCode(t, env, userCode, userID)
	return deviceCode
}

// TestConsumeDeviceCodeAndReplaceKey_IssuesKeyAndConsumesCode: an authorized
// code exchanges once for a valid key bound to the authorizing user and the
// code's key name, and the code is gone afterwards.
func TestConsumeDeviceCodeAndReplaceKey_IssuesKeyAndConsumesCode(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)
	store := &dbauth.Store{DB: env.DB}
	ctx := context.Background()

	user := testutil.CreateTestUser(t, env, "consume@test.com", "Consume User")
	deviceCode := newAuthorizedDeviceCode(t, env, "consume_happy", "CONS-0001", "Laptop (Confab CLI)", user.ID)

	_, keyHash, err := auth.GenerateAPIKey()
	if err != nil {
		t.Fatalf("GenerateAPIKey: %v", err)
	}

	res, err := store.ConsumeDeviceCodeAndReplaceKey(ctx, deviceCode, keyHash)
	if err != nil {
		t.Fatalf("ConsumeDeviceCodeAndReplaceKey failed: %v", err)
	}
	if res.UserID != user.ID {
		t.Errorf("UserID = %d, want %d", res.UserID, user.ID)
	}
	if res.KeyName != "Laptop (Confab CLI)" {
		t.Errorf("KeyName = %q, want %q", res.KeyName, "Laptop (Confab CLI)")
	}
	if res.KeyID == 0 || res.CreatedAt.IsZero() {
		t.Errorf("expected non-zero KeyID and CreatedAt, got %d / %v", res.KeyID, res.CreatedAt)
	}

	userID, keyID, _, _, _, err := store.ValidateAPIKey(ctx, keyHash)
	if err != nil {
		t.Fatalf("issued key must validate: %v", err)
	}
	if userID != user.ID || keyID != res.KeyID {
		t.Errorf("ValidateAPIKey = (user %d, key %d), want (%d, %d)", userID, keyID, user.ID, res.KeyID)
	}
	if deviceCodeExists(t, env, deviceCode) {
		t.Error("device code must be consumed after a successful exchange")
	}

	// Single use: a replay of the same code must fail and mint nothing.
	_, replayHash, _ := auth.GenerateAPIKey()
	if _, err := store.ConsumeDeviceCodeAndReplaceKey(ctx, deviceCode, replayHash); !errors.Is(err, db.ErrDeviceCodeNotFound) {
		t.Errorf("replay err = %v, want ErrDeviceCodeNotFound", err)
	}
	if _, _, _, _, _, err := store.ValidateAPIKey(ctx, replayHash); err == nil {
		t.Error("replayed exchange must not create a key")
	}
}

// TestConsumeDeviceCodeAndReplaceKey_ReplacesSameNameKey: re-authenticating
// with the same key name replaces the old key instead of adding one.
func TestConsumeDeviceCodeAndReplaceKey_ReplacesSameNameKey(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)
	store := &dbauth.Store{DB: env.DB}
	ctx := context.Background()

	user := testutil.CreateTestUser(t, env, "replace@test.com", "Replace User")
	_, oldHash, _ := auth.GenerateAPIKey()
	testutil.CreateTestAPIKey(t, env, user.ID, oldHash, "Same Name")

	deviceCode := newAuthorizedDeviceCode(t, env, "consume_replace", "CONS-0002", "Same Name", user.ID)
	_, newHash, _ := auth.GenerateAPIKey()
	if _, err := store.ConsumeDeviceCodeAndReplaceKey(ctx, deviceCode, newHash); err != nil {
		t.Fatalf("ConsumeDeviceCodeAndReplaceKey failed: %v", err)
	}

	if _, _, _, _, _, err := store.ValidateAPIKey(ctx, oldHash); err == nil {
		t.Error("old same-name key must be replaced")
	}
	if _, _, _, _, _, err := store.ValidateAPIKey(ctx, newHash); err != nil {
		t.Errorf("new key must validate: %v", err)
	}
	if n := apiKeyCountByName(t, env, user.ID, "Same Name"); n != 1 {
		t.Errorf("api keys named %q = %d, want 1", "Same Name", n)
	}
}

// TestConsumeDeviceCodeAndReplaceKey_RejectsUnconsumableCodes: pending,
// expired-but-authorized, and missing codes are not consumed and mint nothing.
func TestConsumeDeviceCodeAndReplaceKey_RejectsUnconsumableCodes(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)
	store := &dbauth.Store{DB: env.DB}
	ctx := context.Background()

	user := testutil.CreateTestUser(t, env, "reject@test.com", "Reject User")

	pending := makeDeviceCode("consume_pending")
	testutil.CreateTestDeviceCode(t, env, pending, "PEND-0001", "Pending Key", time.Now().UTC().Add(5*time.Minute))

	expired := makeDeviceCode("consume_expired")
	testutil.CreateTestDeviceCode(t, env, expired, "EXPD-0001", "Expired Key", time.Now().UTC().Add(5*time.Minute))
	testutil.AuthorizeTestDeviceCode(t, env, "EXPD-0001", user.ID)
	if _, err := env.DB.Exec(env.Ctx,
		`UPDATE device_codes SET expires_at = NOW() - INTERVAL '1 minute' WHERE user_code = 'EXPD-0001'`); err != nil {
		t.Fatalf("expire device code: %v", err)
	}

	cases := []struct {
		name       string
		deviceCode string
		keyName    string
		rowRemains bool
	}{
		{"pending (not yet authorized)", pending, "Pending Key", true},
		{"expired but authorized", expired, "Expired Key", true},
		{"missing", makeDeviceCode("consume_missing"), "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, keyHash, _ := auth.GenerateAPIKey()
			_, err := store.ConsumeDeviceCodeAndReplaceKey(ctx, tc.deviceCode, keyHash)
			if !errors.Is(err, db.ErrDeviceCodeNotFound) {
				t.Fatalf("err = %v, want ErrDeviceCodeNotFound", err)
			}
			if got := deviceCodeExists(t, env, tc.deviceCode); got != tc.rowRemains {
				t.Errorf("device code row present = %v, want %v", got, tc.rowRemains)
			}
			if _, _, _, _, _, err := store.ValidateAPIKey(ctx, keyHash); err == nil {
				t.Error("a rejected exchange must not create a key")
			}
		})
	}
}

// TestConsumeDeviceCodeAndReplaceKey_ConcurrentExchangesYieldExactlyOneSuccess:
// N concurrent exchanges of one authorized code produce exactly one success;
// the rest lose the claim with ErrDeviceCodeNotFound, exactly one key exists
// for the code's name, and the winner's key is the one that validates.
func TestConsumeDeviceCodeAndReplaceKey_ConcurrentExchangesYieldExactlyOneSuccess(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)
	store := &dbauth.Store{DB: env.DB}
	ctx := context.Background()

	user := testutil.CreateTestUser(t, env, "race@test.com", "Race User")
	const keyName = "Race Key"
	deviceCode := newAuthorizedDeviceCode(t, env, "consume_race", "RACE-0001", keyName, user.ID)

	const n = 10
	hashes := make([]string, n)
	for i := range hashes {
		_, h, err := auth.GenerateAPIKey()
		if err != nil {
			t.Fatalf("GenerateAPIKey: %v", err)
		}
		hashes[i] = h
	}

	errs := make([]error, n)
	results := make([]*dbauth.ConsumedDeviceCode, n)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := range n {
		wg.Go(func() {
			<-start
			results[i], errs[i] = store.ConsumeDeviceCodeAndReplaceKey(ctx, deviceCode, hashes[i])
		})
	}
	close(start)
	wg.Wait()

	winner := -1
	for i, err := range errs {
		switch {
		case err == nil:
			if winner != -1 {
				t.Errorf("exchanges %d and %d both succeeded; want exactly one success", winner, i)
			}
			winner = i
		case errors.Is(err, db.ErrDeviceCodeNotFound):
		default:
			t.Errorf("exchange %d: err = %v, want nil or ErrDeviceCodeNotFound", i, err)
		}
	}
	if winner == -1 {
		t.Fatal("no exchange succeeded; want exactly one success")
	}

	if n := apiKeyCountByName(t, env, user.ID, keyName); n != 1 {
		t.Errorf("api keys named %q = %d, want 1", keyName, n)
	}
	userID, keyID, _, _, _, err := store.ValidateAPIKey(ctx, hashes[winner])
	if err != nil {
		t.Fatalf("the successful exchange's key must remain valid: %v", err)
	}
	if userID != user.ID || keyID != results[winner].KeyID {
		t.Errorf("ValidateAPIKey = (user %d, key %d), want (%d, %d)", userID, keyID, user.ID, results[winner].KeyID)
	}
	if deviceCodeExists(t, env, deviceCode) {
		t.Error("device code must be consumed after the winning exchange")
	}
}

// TestConsumeDeviceCodeAndReplaceKey_KeyIssuanceFailureLeavesCodeUnconsumed:
// when key issuance fails (API key limit), the claim rolls back — the device
// code is still present and no key is minted — so the same code exchanges
// successfully once the user frees a slot.
func TestConsumeDeviceCodeAndReplaceKey_KeyIssuanceFailureLeavesCodeUnconsumed(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)
	store := &dbauth.Store{DB: env.DB}
	ctx := context.Background()

	user := testutil.CreateTestUser(t, env, "limit-consume@test.com", "Limit Consume User")
	// Fill the user to the key limit in one statement (sha256 hex = CHAR(64)).
	if _, err := env.DB.Exec(env.Ctx,
		`INSERT INTO api_keys (user_id, key_hash, name)
		 SELECT $1, encode(sha256(('limit-key-' || g)::bytea), 'hex'), 'Limit Key ' || g
		 FROM generate_series(1, $2::int) AS g`,
		user.ID, db.MaxAPIKeysPerUser); err != nil {
		t.Fatalf("fill api keys to limit: %v", err)
	}

	deviceCode := newAuthorizedDeviceCode(t, env, "consume_limit", "LIMT-0001", "Over Limit Key", user.ID)

	_, keyHash, _ := auth.GenerateAPIKey()
	_, err := store.ConsumeDeviceCodeAndReplaceKey(ctx, deviceCode, keyHash)
	if !errors.Is(err, db.ErrAPIKeyLimitExceeded) {
		t.Fatalf("err = %v, want ErrAPIKeyLimitExceeded", err)
	}
	if !deviceCodeExists(t, env, deviceCode) {
		t.Fatal("device code must NOT be consumed when key issuance fails")
	}
	if _, _, _, _, _, err := store.ValidateAPIKey(ctx, keyHash); err == nil {
		t.Error("failed exchange must not create a key")
	}

	if _, err := env.DB.Exec(env.Ctx,
		`DELETE FROM api_keys WHERE user_id = $1 AND name = 'Limit Key 1'`, user.ID); err != nil {
		t.Fatalf("free a key slot: %v", err)
	}

	res, err := store.ConsumeDeviceCodeAndReplaceKey(ctx, deviceCode, keyHash)
	if err != nil {
		t.Fatalf("retry after freeing a slot must succeed: %v", err)
	}
	if res.UserID != user.ID {
		t.Errorf("UserID = %d, want %d", res.UserID, user.ID)
	}
	if _, _, _, _, _, err := store.ValidateAPIKey(ctx, keyHash); err != nil {
		t.Errorf("retried exchange's key must validate: %v", err)
	}
	if deviceCodeExists(t, env, deviceCode) {
		t.Error("device code must be consumed after the successful retry")
	}
}
