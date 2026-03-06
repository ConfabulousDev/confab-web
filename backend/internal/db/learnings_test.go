// ABOUTME: Integration tests for the learnings CRUD database layer.
// ABOUTME: Requires PostgreSQL via testcontainers; skipped with -short flag.
package db_test

import (
	"context"
	"errors"
	"testing"

	"github.com/ConfabulousDev/confab-web/internal/db"
	"github.com/ConfabulousDev/confab-web/internal/models"
	"github.com/ConfabulousDev/confab-web/internal/testutil"
)

// =============================================================================
// CreateLearning Tests
// =============================================================================

func TestCreateLearning_Success(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	user := testutil.CreateTestUser(t, env, "learner@test.com", "Learner")
	sessionID := testutil.CreateTestSession(t, env, user.ID, "learn-session-1")

	ctx := context.Background()

	req := &models.CreateLearningRequest{
		Title:      "How to debug OCP routes",
		Body:       "When a route returns 503, check the backend pod readiness probe first.",
		Tags:       []string{"openshift", "debugging", "routes"},
		Source:     models.LearningSourceManualSession,
		SessionIDs: []string{sessionID},
	}

	learning, err := env.DB.CreateLearning(ctx, user.ID, req)
	if err != nil {
		t.Fatalf("CreateLearning failed: %v", err)
	}

	// Verify returned fields
	if learning.ID == "" {
		t.Error("expected non-empty ID")
	}
	if learning.UserID != user.ID {
		t.Errorf("UserID = %d, want %d", learning.UserID, user.ID)
	}
	if learning.Title != req.Title {
		t.Errorf("Title = %q, want %q", learning.Title, req.Title)
	}
	if learning.Body != req.Body {
		t.Errorf("Body = %q, want %q", learning.Body, req.Body)
	}
	if len(learning.Tags) != 3 {
		t.Errorf("expected 3 tags, got %d", len(learning.Tags))
	}
	if learning.Status != models.LearningStatusDraft {
		t.Errorf("Status = %q, want %q", learning.Status, models.LearningStatusDraft)
	}
	if learning.Source != models.LearningSourceManualSession {
		t.Errorf("Source = %q, want %q", learning.Source, models.LearningSourceManualSession)
	}
	if len(learning.SessionIDs) != 1 || learning.SessionIDs[0] != sessionID {
		t.Errorf("SessionIDs = %v, want [%s]", learning.SessionIDs, sessionID)
	}
	if learning.CreatedAt.IsZero() {
		t.Error("expected non-zero CreatedAt")
	}
	if learning.UpdatedAt.IsZero() {
		t.Error("expected non-zero UpdatedAt")
	}
}

// =============================================================================
// ListLearnings Tests
// =============================================================================

func TestListLearnings_FilterByStatus(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	user := testutil.CreateTestUser(t, env, "filter@test.com", "Filter User")
	ctx := context.Background()

	// Create 2 drafts and 1 confirmed learning
	for i := 0; i < 2; i++ {
		_, err := env.DB.CreateLearning(ctx, user.ID, &models.CreateLearningRequest{
			Title:  "Draft learning",
			Body:   "Some draft content",
			Tags:   []string{"draft"},
			Source: models.LearningSourceManualReview,
		})
		if err != nil {
			t.Fatalf("CreateLearning (draft %d) failed: %v", i, err)
		}
	}

	// Create a confirmed learning via create + update
	confirmed, err := env.DB.CreateLearning(ctx, user.ID, &models.CreateLearningRequest{
		Title:  "Confirmed learning",
		Body:   "Confirmed content",
		Tags:   []string{"confirmed"},
		Source: models.LearningSourceManualSession,
	})
	if err != nil {
		t.Fatalf("CreateLearning (confirmed) failed: %v", err)
	}
	confirmedStatus := models.LearningStatusConfirmed
	_, err = env.DB.UpdateLearning(ctx, confirmed.ID, user.ID, &models.UpdateLearningRequest{
		Status: &confirmedStatus,
	})
	if err != nil {
		t.Fatalf("UpdateLearning (to confirmed) failed: %v", err)
	}

	// Filter by draft status
	draftStatus := models.LearningStatusDraft
	drafts, err := env.DB.ListLearnings(ctx, user.ID, &db.LearningFilters{
		Status: &draftStatus,
	})
	if err != nil {
		t.Fatalf("ListLearnings (draft filter) failed: %v", err)
	}
	if len(drafts) != 2 {
		t.Errorf("expected 2 drafts, got %d", len(drafts))
	}

	// Filter by confirmed status
	confirmedFilter, err := env.DB.ListLearnings(ctx, user.ID, &db.LearningFilters{
		Status: &confirmedStatus,
	})
	if err != nil {
		t.Fatalf("ListLearnings (confirmed filter) failed: %v", err)
	}
	if len(confirmedFilter) != 1 {
		t.Errorf("expected 1 confirmed, got %d", len(confirmedFilter))
	}

	// No filter should return all 3
	all, err := env.DB.ListLearnings(ctx, user.ID, nil)
	if err != nil {
		t.Fatalf("ListLearnings (no filter) failed: %v", err)
	}
	if len(all) != 3 {
		t.Errorf("expected 3 total learnings, got %d", len(all))
	}

	// Verify owner_email is populated
	if all[0].OwnerEmail == "" {
		t.Error("expected OwnerEmail to be populated")
	}
}

// =============================================================================
// SearchLearnings Tests
// =============================================================================

func TestSearchLearnings(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	user := testutil.CreateTestUser(t, env, "search@test.com", "Search User")
	ctx := context.Background()

	// Create two learnings with distinct keywords
	_, err := env.DB.CreateLearning(ctx, user.ID, &models.CreateLearningRequest{
		Title:  "Kubernetes networking fundamentals",
		Body:   "Understanding CNI plugins and service mesh patterns in OpenShift.",
		Tags:   []string{"kubernetes", "networking"},
		Source: models.LearningSourceManualSession,
	})
	if err != nil {
		t.Fatalf("CreateLearning (networking) failed: %v", err)
	}

	_, err = env.DB.CreateLearning(ctx, user.ID, &models.CreateLearningRequest{
		Title:  "PostgreSQL query optimization",
		Body:   "Using EXPLAIN ANALYZE and proper indexing for better database performance.",
		Tags:   []string{"postgresql", "performance"},
		Source: models.LearningSourceAIExtracted,
	})
	if err != nil {
		t.Fatalf("CreateLearning (postgres) failed: %v", err)
	}

	// Search for "kubernetes" — should match only the first
	queryKube := "kubernetes"
	results, err := env.DB.ListLearnings(ctx, user.ID, &db.LearningFilters{
		Query: &queryKube,
	})
	if err != nil {
		t.Fatalf("ListLearnings (search kubernetes) failed: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 result for 'kubernetes', got %d", len(results))
	}
	if len(results) > 0 && results[0].Title != "Kubernetes networking fundamentals" {
		t.Errorf("Title = %q, want 'Kubernetes networking fundamentals'", results[0].Title)
	}

	// Search for "indexing" — should match only the second (body content)
	queryIndex := "indexing"
	results, err = env.DB.ListLearnings(ctx, user.ID, &db.LearningFilters{
		Query: &queryIndex,
	})
	if err != nil {
		t.Fatalf("ListLearnings (search indexing) failed: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 result for 'indexing', got %d", len(results))
	}

	// Search for a term that matches neither
	queryNone := "ansible"
	results, err = env.DB.ListLearnings(ctx, user.ID, &db.LearningFilters{
		Query: &queryNone,
	})
	if err != nil {
		t.Fatalf("ListLearnings (search ansible) failed: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results for 'ansible', got %d", len(results))
	}
}

// =============================================================================
// DeleteLearning Tests
// =============================================================================

func TestDeleteLearning_Success(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	user := testutil.CreateTestUser(t, env, "delete@test.com", "Delete User")
	ctx := context.Background()

	// Create a learning
	learning, err := env.DB.CreateLearning(ctx, user.ID, &models.CreateLearningRequest{
		Title:  "Ephemeral learning",
		Body:   "This will be deleted.",
		Tags:   []string{"temporary"},
		Source: models.LearningSourceManualReview,
	})
	if err != nil {
		t.Fatalf("CreateLearning failed: %v", err)
	}

	// Delete it
	err = env.DB.DeleteLearning(ctx, learning.ID, user.ID)
	if err != nil {
		t.Fatalf("DeleteLearning failed: %v", err)
	}

	// Verify it's gone
	_, err = env.DB.GetLearning(ctx, learning.ID, user.ID)
	if !errors.Is(err, db.ErrLearningNotFound) {
		t.Errorf("expected ErrLearningNotFound after delete, got %v", err)
	}

	// Deleting again should return not found
	err = env.DB.DeleteLearning(ctx, learning.ID, user.ID)
	if !errors.Is(err, db.ErrLearningNotFound) {
		t.Errorf("expected ErrLearningNotFound on second delete, got %v", err)
	}
}
