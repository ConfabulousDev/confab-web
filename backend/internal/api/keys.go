package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/santaclaude2025/confab/backend/internal/auth"
	"github.com/santaclaude2025/confab/backend/internal/db"
	"github.com/santaclaude2025/confab/backend/internal/logger"
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
		// Get user ID from context (set by SessionMiddleware)
		userID, ok := auth.GetUserID(r.Context())
		if !ok {
			respondError(w, http.StatusUnauthorized, "Unauthorized")
			return
		}

		// Parse request body
		var req CreateAPIKeyRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, http.StatusBadRequest, "Invalid request body")
			return
		}

		if req.Name == "" {
			req.Name = "API Key"
		}

		// Generate API key
		apiKey, keyHash, err := auth.GenerateAndHashAPIKey()
		if err != nil {
			respondError(w, http.StatusInternalServerError, "Failed to generate API key")
			return
		}

		// Create context with timeout for database operation
		ctx, cancel := context.WithTimeout(r.Context(), DatabaseTimeout)
		defer cancel()

		// Store in database
		keyID, createdAt, err := database.CreateAPIKeyWithReturn(ctx, userID, keyHash, req.Name)
		if err != nil {
			logger.Error("Failed to create API key in database", "error", err, "user_id", userID, "name", req.Name)
			respondError(w, http.StatusInternalServerError, "Failed to create API key")
			return
		}

		// Audit log: API key created
		logger.Info("API key created", "user_id", userID, "key_id", keyID, "name", req.Name)

		// Return response (key is only shown once)
		respondJSON(w, http.StatusOK, CreateAPIKeyResponse{
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
		// Get user ID from context
		userID, ok := auth.GetUserID(r.Context())
		if !ok {
			respondError(w, http.StatusUnauthorized, "Unauthorized")
			return
		}

		// Create context with timeout for database operation
		ctx, cancel := context.WithTimeout(r.Context(), DatabaseTimeout)
		defer cancel()

		// Get keys from database
		keys, err := database.ListAPIKeys(ctx, userID)
		if err != nil {
			logger.Error("Failed to list API keys", "error", err, "user_id", userID)
			respondError(w, http.StatusInternalServerError, "Failed to list API keys")
			return
		}

		// Success log
		logger.Info("API keys listed", "user_id", userID, "count", len(keys))

		// Return empty array if no keys
		if keys == nil {
			keys = []models.APIKey{}
		}

		respondJSON(w, http.StatusOK, keys)
	}
}

// HandleDeleteAPIKey deletes an API key
func HandleDeleteAPIKey(database *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Get user ID from context
		userID, ok := auth.GetUserID(r.Context())
		if !ok {
			respondError(w, http.StatusUnauthorized, "Unauthorized")
			return
		}

		// Get key ID from URL
		keyIDStr := chi.URLParam(r, "id")
		keyID, err := strconv.ParseInt(keyIDStr, 10, 64)
		if err != nil {
			respondError(w, http.StatusBadRequest, "Invalid key ID")
			return
		}

		// Create context with timeout for database operation
		ctx, cancel := context.WithTimeout(r.Context(), DatabaseTimeout)
		defer cancel()

		// Delete key
		if err := database.DeleteAPIKey(ctx, userID, keyID); err != nil {
			logger.Error("Failed to delete API key", "error", err, "user_id", userID, "key_id", keyID)
			respondError(w, http.StatusNotFound, "API key not found")
			return
		}

		// Audit log: API key deleted
		logger.Info("API key deleted", "user_id", userID, "key_id", keyID)

		w.WriteHeader(http.StatusNoContent)
	}
}
