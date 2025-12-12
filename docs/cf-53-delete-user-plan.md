# CF-53: Delete User Feature - Implementation Plan

## Overview

The feature requires:
1. A new `status` field on users (active/inactive/deleted conceptually, though deleted = removed from DB)
2. Admin-only HTML UI served by backend to list/manage users
3. Soft-delete (inactive) and hard-delete capabilities
4. Proper cascade cleanup including S3 objects

---

## Phase 1: Database Schema Changes

**Add user status field:**
- Add `status` column to `users` table: `VARCHAR(20) DEFAULT 'active'`
- Values: `active`, `inactive`
- Add index on status for filtering

**Migration file:** `backend/internal/db/migrations/XXXXXX_add_user_status.up.sql`

---

## Phase 2: Core User Management Functions

**File:** `backend/internal/db/db.go`

Add new functions:
- `ListAllUsers(ctx) ([]User, error)` - List all users with status
- `UpdateUserStatus(ctx, userID, status) error` - Set user to active/inactive
- `DeleteUser(ctx, userID) error` - Hard delete user from DB
- `GetUserSessions(ctx, userID) ([]Session, error)` - Get all sessions for S3 cleanup

**File:** `backend/internal/models/models.go`

Update User struct to include `Status` field.

---

## Phase 3: S3 Cleanup for User Deletion

**File:** `backend/internal/storage/s3.go`

Add new function:
- `DeleteAllUserChunks(ctx, userID) error` - Delete all S3 objects under `{userID}/` prefix

This must be called **before** database deletion to avoid orphaned objects.

---

## Phase 4: Auth Middleware Updates

**File:** `backend/internal/auth/auth.go`

Modify `Middleware` and `SessionMiddleware` to:
1. After validating credentials, check user status
2. If status = `inactive`, return 403 Forbidden with message "Account deactivated"
3. This blocks all API and web access for inactive users

---

## Phase 5: Admin Authentication & Routes

**File:** `backend/internal/admin/admin.go` (new)

Create admin package with:
- `IsSuperAdmin(email string) bool` - Check if email is in allowed list
- Super admin list stored in environment variable: `SUPER_ADMIN_EMAILS` (comma-separated)

**File:** `backend/internal/api/server.go`

Add admin routes (protected by super admin check):
```
GET  /admin/users                  - HTML page listing all users
POST /admin/users/{id}/deactivate  - Set user to inactive
POST /admin/users/{id}/activate    - Set user back to active
POST /admin/users/{id}/delete      - Hard delete user
```

---

## Phase 6: Admin HTML UI

**File:** `backend/internal/admin/templates/users.html` (new)

Simple HTML template with:
- Table of all users (id, email, name, status, created_at)
- Action buttons per user: Activate/Deactivate, Delete
- Confirmation dialogs for destructive actions
- CSRF protection on forms
- Basic styling (inline CSS, no external dependencies)

**File:** `backend/internal/admin/handlers.go` (new)

Handlers for:
- `HandleAdminUsers` - Render user list page
- `HandleDeactivateUser` - Set status to inactive
- `HandleActivateUser` - Set status to active
- `HandleDeleteUser` - Delete S3 objects, then delete from DB

---

## Phase 7: Share Access Enforcement

**File:** `backend/internal/api/shares.go`

Update share access handlers to check if the **session owner** is inactive:
- If owner is inactive, return 403 "This session is no longer available"
- This makes shares immediately inaccessible when owner is deactivated

---

## Phase 8: Help Page for Account Deletion Requests

**File:** `backend/internal/api/server.go`

Add public route:
```
GET /help/delete-account - Static HTML page with deletion instructions
```

**File:** `backend/internal/admin/templates/delete_account_help.html` (new)

Simple page explaining:
- "To delete your account, please email support@confabulous.dev"
- Include user's email in the request
- What data will be deleted

---

## File Summary

| File | Action |
|------|--------|
| `backend/internal/db/migrations/XXXXXX_add_user_status.up.sql` | New |
| `backend/internal/db/migrations/XXXXXX_add_user_status.down.sql` | New |
| `backend/internal/models/models.go` | Modify |
| `backend/internal/db/db.go` | Modify |
| `backend/internal/storage/s3.go` | Modify |
| `backend/internal/auth/auth.go` | Modify |
| `backend/internal/admin/admin.go` | New |
| `backend/internal/admin/handlers.go` | New |
| `backend/internal/admin/templates/users.html` | New |
| `backend/internal/admin/templates/delete_account_help.html` | New |
| `backend/internal/api/server.go` | Modify |
| `backend/internal/api/shares.go` | Modify |

---

## Testing Plan

1. **Unit tests** for new DB functions
2. **Integration tests** for admin endpoints
3. **Manual testing:**
   - Deactivate user → verify they can't login/access API
   - Deactivate user → verify their shares are inaccessible
   - Reactivate user → verify access restored
   - Delete user → verify S3 objects and DB records removed
