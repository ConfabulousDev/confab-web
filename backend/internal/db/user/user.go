package user

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/lib/pq"

	"github.com/ConfabulousDev/confab-web/internal/db"
	"github.com/ConfabulousDev/confab-web/internal/models"
)

// GetUserByID retrieves a user by ID
func (s *Store) GetUserByID(ctx context.Context, userID int64) (*models.User, error) {
	query := `SELECT id, email, name, avatar_url, status, read_only, is_admin, created_at, updated_at FROM users WHERE id = $1`

	var user models.User
	err := s.conn().QueryRowContext(ctx, query, userID).Scan(
		&user.ID,
		&user.Email,
		&user.Name,
		&user.AvatarURL,
		&user.Status,
		&user.ReadOnly,
		&user.IsAdmin,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, db.ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return &user, nil
}

// UpsertDemoIdentity provisions or refreshes the demo user row idempotently.
// Sets name='Demo', status='active', is_admin=false, read_only=true. Returns
// the resulting user and whether the row pre-existed (caller logs WARN if so,
// because flipping a real user is a significant operator action). CF-483.
func (s *Store) UpsertDemoIdentity(ctx context.Context, email string) (*models.User, bool, error) {
	query := `
		INSERT INTO users (email, name, status, is_admin, read_only, created_at, updated_at)
		VALUES ($1, 'Demo', 'active', false, true, NOW(), NOW())
		ON CONFLICT (email) DO UPDATE
			SET name = 'Demo',
			    status = 'active',
			    is_admin = false,
			    read_only = true,
			    updated_at = NOW()
		RETURNING id, email, name, avatar_url, status, read_only, created_at, updated_at,
		          (xmax <> 0) AS pre_existed
	`
	var user models.User
	var preExisted bool
	err := s.conn().QueryRowContext(ctx, query, email).Scan(
		&user.ID, &user.Email, &user.Name, &user.AvatarURL,
		&user.Status, &user.ReadOnly, &user.CreatedAt, &user.UpdatedAt,
		&preExisted,
	)
	if err != nil {
		return nil, false, fmt.Errorf("upsert demo identity: %w", err)
	}
	return &user, preExisted, nil
}

// DeletePasswordIdentitiesForUser removes every password-provider
// identity (and its credentials via ON DELETE CASCADE on identity_passwords)
// for the given user. Used by CF-483 bootstrap so the demo identity
// cannot be logged in via password even if it inherited a hash from
// a pre-existing real user. Idempotent.
func (s *Store) DeletePasswordIdentitiesForUser(ctx context.Context, userID int64) error {
	query := `DELETE FROM user_identities WHERE user_id = $1 AND provider = 'password'`
	if _, err := s.conn().ExecContext(ctx, query, userID); err != nil {
		return fmt.Errorf("delete password identities: %w", err)
	}
	return nil
}

// CountUsers returns the total number of users in the system
func (s *Store) CountUsers(ctx context.Context) (int, error) {
	query := `SELECT COUNT(*) FROM users`
	var count int
	err := s.conn().QueryRowContext(ctx, query).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count users: %w", err)
	}
	return count, nil
}

// ListEffectiveAdminIDs returns the IDs of active users who can reach the admin
// panel: those with is_admin=true OR whose (lowercased) email is in
// superAdminEmails (the normalized SUPER_ADMIN_EMAILS set). Inactive users and
// env super-admins without a matching user row are excluded — they can't
// actually log in. Used by the last-effective-admin guard (g0bq).
func (s *Store) ListEffectiveAdminIDs(ctx context.Context, superAdminEmails []string) ([]int64, error) {
	query := `
		SELECT id FROM users
		WHERE status = 'active' AND (is_admin = true OR LOWER(email) = ANY($1))`
	rows, err := s.conn().QueryContext(ctx, query, pq.Array(superAdminEmails))
	if err != nil {
		return nil, fmt.Errorf("failed to list effective admin ids: %w", err)
	}
	defer rows.Close()

	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("failed to scan effective admin id: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("effective admin id rows: %w", err)
	}
	return ids, nil
}

// UserExistsByEmail checks if a user exists with the given email
func (s *Store) UserExistsByEmail(ctx context.Context, email string) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM users WHERE email = $1)`
	var exists bool
	err := s.conn().QueryRowContext(ctx, query, email).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check user exists: %w", err)
	}
	return exists, nil
}

// ListAllUsers returns all users in the system with stats, ordered by ID
func (s *Store) ListAllUsers(ctx context.Context) ([]models.AdminUserStats, error) {
	query := `
		SELECT
			u.id, u.email, u.name, u.avatar_url, u.status, u.created_at, u.updated_at, u.is_admin,
			COUNT(DISTINCT s.id) AS session_count,
			MAX(ak.last_used_at) AS last_api_key_used,
			MAX(ws.created_at) AS last_logged_in
		FROM users u
		LEFT JOIN sessions s ON s.user_id = u.id
		LEFT JOIN api_keys ak ON ak.user_id = u.id
		LEFT JOIN web_sessions ws ON ws.user_id = u.id
		GROUP BY u.id
		ORDER BY u.id`

	rows, err := s.conn().QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list users: %w", err)
	}
	defer rows.Close()

	var users []models.AdminUserStats
	for rows.Next() {
		var user models.AdminUserStats
		err := rows.Scan(
			&user.ID,
			&user.Email,
			&user.Name,
			&user.AvatarURL,
			&user.Status,
			&user.CreatedAt,
			&user.UpdatedAt,
			&user.IsAdmin,
			&user.SessionCount,
			&user.LastAPIKeyUsed,
			&user.LastLoggedIn,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan user: %w", err)
		}
		users = append(users, user)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating users: %w", err)
	}

	return users, nil
}

// UpdateUserStatus updates the status of a user (active/inactive)
func (s *Store) UpdateUserStatus(ctx context.Context, userID int64, status models.UserStatus) error {
	query := `UPDATE users SET status = $1, updated_at = NOW() WHERE id = $2`

	result, err := s.conn().ExecContext(ctx, query, status, userID)
	if err != nil {
		return fmt.Errorf("failed to update user status: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return db.ErrUserNotFound
	}

	return nil
}

// DeleteUser permanently deletes a user and all associated data (via CASCADE)
// Note: S3 objects must be deleted separately before calling this function
func (s *Store) DeleteUser(ctx context.Context, userID int64) error {
	query := `DELETE FROM users WHERE id = $1`

	result, err := s.conn().ExecContext(ctx, query, userID)
	if err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return db.ErrUserNotFound
	}

	return nil
}

// HasOwnSessions checks if a user has any sessions they own
func (s *Store) HasOwnSessions(ctx context.Context, userID int64) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM sessions WHERE user_id = $1)`
	var exists bool
	err := s.conn().QueryRowContext(ctx, query, userID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check user sessions: %w", err)
	}
	return exists, nil
}

// HasAPIKeys checks if a user has any API keys
func (s *Store) HasAPIKeys(ctx context.Context, userID int64) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM api_keys WHERE user_id = $1)`
	var exists bool
	err := s.conn().QueryRowContext(ctx, query, userID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check user API keys: %w", err)
	}
	return exists, nil
}

// GetUserSessionIDs returns all session IDs (UUIDs) for a user
func (s *Store) GetUserSessionIDs(ctx context.Context, userID int64) ([]string, error) {
	query := `SELECT id FROM sessions WHERE user_id = $1`

	rows, err := s.conn().QueryContext(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user sessions: %w", err)
	}
	defer rows.Close()

	var sessionIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("failed to scan session ID: %w", err)
		}
		sessionIDs = append(sessionIDs, id)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating sessions: %w", err)
	}

	return sessionIDs, nil
}

// SetUserAdmin sets the users.is_admin column (5k4v). Returns ErrUserNotFound
// when no row matches. This controls one half of the admin union; the
// SUPER_ADMIN_EMAILS env half is unaffected.
func (s *Store) SetUserAdmin(ctx context.Context, userID int64, isAdmin bool) error {
	query := `UPDATE users SET is_admin = $1, updated_at = NOW() WHERE id = $2`

	result, err := s.conn().ExecContext(ctx, query, isAdmin, userID)
	if err != nil {
		return fmt.Errorf("failed to set user admin: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return db.ErrUserNotFound
	}

	return nil
}
