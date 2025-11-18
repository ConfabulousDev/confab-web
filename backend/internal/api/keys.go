package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/santaclaude2025/confab/backend/internal/auth"
	"github.com/santaclaude2025/confab/backend/internal/db"
	"github.com/santaclaude2025/confab/backend/internal/models"
)

// CreateAPIKeyRequest is the request body for creating an API key
type CreateAPIKeyRequest struct {
	Name string `json:"name"`
}

// CreateAPIKeyResponse is the response for creating an API key
type CreateAPIKeyResponse struct {
	ID        int64  `json:"id"`
	Key       string `json:"key"` // Only returned once
	Name      string `json:"name"`
	CreatedAt string `json:"created_at"`
}

// HandleCreateAPIKey creates a new API key for the authenticated user
func HandleCreateAPIKey(database *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// Get user ID from context (set by SessionMiddleware)
		userID, ok := auth.GetUserID(ctx)
		if !ok {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// Parse request body
		var req CreateAPIKeyRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		if req.Name == "" {
			req.Name = "API Key"
		}

		// Generate API key
		apiKey, keyHash, err := auth.GenerateAndHashAPIKey()
		if err != nil {
			http.Error(w, "Failed to generate API key", http.StatusInternalServerError)
			return
		}

		// Store in database
		keyID, createdAt, err := database.CreateAPIKeyWithReturn(ctx, userID, keyHash, req.Name)
		if err != nil {
			fmt.Printf("Error creating API key: %v\n", err)
			http.Error(w, "Failed to create API key", http.StatusInternalServerError)
			return
		}

		// Return response (key is only shown once)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(CreateAPIKeyResponse{
			ID:        keyID,
			Key:       apiKey,
			Name:      req.Name,
			CreatedAt: createdAt.Format("2006-01-02 15:04:05"),
		})
	}
}

// HandleListAPIKeys lists all API keys for the authenticated user
func HandleListAPIKeys(database *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// Get user ID from context
		userID, ok := auth.GetUserID(ctx)
		if !ok {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// Get keys from database
		keys, err := database.ListAPIKeys(ctx, userID)
		if err != nil {
			http.Error(w, "Failed to list API keys", http.StatusInternalServerError)
			return
		}

		// Return empty array if no keys
		if keys == nil {
			keys = []models.APIKey{}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(keys)
	}
}

// HandleDeleteAPIKey deletes an API key
func HandleDeleteAPIKey(database *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// Get user ID from context
		userID, ok := auth.GetUserID(ctx)
		if !ok {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// Get key ID from URL
		keyIDStr := chi.URLParam(r, "id")
		keyID, err := strconv.ParseInt(keyIDStr, 10, 64)
		if err != nil {
			http.Error(w, "Invalid key ID", http.StatusBadRequest)
			return
		}

		// Delete key
		if err := database.DeleteAPIKey(ctx, userID, keyID); err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}
