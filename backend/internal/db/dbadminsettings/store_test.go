package dbadminsettings_test

import (
	"context"
	"testing"

	"github.com/ConfabulousDev/confab-web/internal/db/dbadminsettings"
	"github.com/ConfabulousDev/confab-web/internal/testutil"
)

func TestAdminSettings_SetThenGet(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)
	store := &dbadminsettings.Store{DB: env.DB}
	ctx := context.Background()

	// Set a value
	if err := store.Set(ctx, "test_key", "test_value"); err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Get it back
	setting, err := store.Get(ctx, "test_key")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if setting == nil {
		t.Fatal("expected non-nil setting after Set")
	}
	if setting.Key != "test_key" {
		t.Errorf("Key = %q, want %q", setting.Key, "test_key")
	}
	if setting.Value != "test_value" {
		t.Errorf("Value = %q, want %q", setting.Value, "test_value")
	}
	if setting.UpdatedAt.IsZero() {
		t.Error("expected non-zero UpdatedAt")
	}
}

func TestAdminSettings_GetNonExistent(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)
	store := &dbadminsettings.Store{DB: env.DB}
	ctx := context.Background()

	setting, err := store.Get(ctx, "nonexistent_key")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if setting != nil {
		t.Errorf("expected nil for non-existent key, got %+v", setting)
	}
}

func TestAdminSettings_SetEmptyString(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)
	store := &dbadminsettings.Store{DB: env.DB}
	ctx := context.Background()

	// Set empty string value
	if err := store.Set(ctx, "empty_key", ""); err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Get returns Setting with empty Value (distinct from nil)
	setting, err := store.Get(ctx, "empty_key")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if setting == nil {
		t.Fatal("expected non-nil setting for empty string value")
	}
	if setting.Value != "" {
		t.Errorf("Value = %q, want empty string", setting.Value)
	}
}

func TestAdminSettings_DeleteThenGet(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)
	store := &dbadminsettings.Store{DB: env.DB}
	ctx := context.Background()

	// Set a value
	if err := store.Set(ctx, "delete_key", "to_delete"); err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Delete it
	if err := store.Delete(ctx, "delete_key"); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Get returns nil
	setting, err := store.Get(ctx, "delete_key")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if setting != nil {
		t.Errorf("expected nil after Delete, got %+v", setting)
	}
}

func TestAdminSettings_DeleteNonExistent(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)
	store := &dbadminsettings.Store{DB: env.DB}
	ctx := context.Background()

	// Delete a key that was never set — should not error
	if err := store.Delete(ctx, "never_existed"); err != nil {
		t.Fatalf("Delete non-existent key should not error, got: %v", err)
	}
}

func TestAdminSettings_SetOverwritesExisting(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)
	store := &dbadminsettings.Store{DB: env.DB}
	ctx := context.Background()

	// Set initial value
	if err := store.Set(ctx, "overwrite_key", "first_value"); err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Overwrite with new value
	if err := store.Set(ctx, "overwrite_key", "second_value"); err != nil {
		t.Fatalf("Set (overwrite) failed: %v", err)
	}

	// Get returns updated value
	setting, err := store.Get(ctx, "overwrite_key")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if setting == nil {
		t.Fatal("expected non-nil setting after overwrite")
	}
	if setting.Value != "second_value" {
		t.Errorf("Value = %q, want %q", setting.Value, "second_value")
	}
}
