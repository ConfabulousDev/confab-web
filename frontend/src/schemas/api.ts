// Zod schemas for API response validation
// Validates all data received from backend APIs
import { z } from 'zod';

// ============================================================================
// Common Schemas
// ============================================================================

export const GitInfoSchema = z.object({
  repo_url: z.string().optional(),
  branch: z.string().optional(),
  commit_sha: z.string().optional(),
  commit_message: z.string().optional(),
  author: z.string().optional(),
  is_dirty: z.boolean().optional(),
});

export const SyncFileDetailSchema = z.object({
  file_name: z.string(),
  file_type: z.string(),
  last_synced_line: z.number(),
  updated_at: z.string(),
});

// ============================================================================
// Session Schemas
// ============================================================================

export const SessionSchema = z.object({
  id: z.string(),
  external_id: z.string(),
  first_seen: z.string(),
  file_count: z.number(),
  last_sync_time: z.string().nullable().optional(),
  summary: z.string().nullable().optional(),
  first_user_message: z.string().nullable().optional(),
  session_type: z.string(),
  total_lines: z.number(),
  git_repo: z.string().nullable().optional(),
  git_branch: z.string().nullable().optional(),
  is_owner: z.boolean(),
  access_type: z.enum(['owner', 'private_share', 'public_share', 'system_share']),
  share_token: z.string().nullable().optional(),
  shared_by_email: z.string().nullable().optional(),
});

export const SessionDetailSchema = z.object({
  id: z.string(),
  external_id: z.string(),
  summary: z.string().nullable().optional(),
  first_user_message: z.string().nullable().optional(),
  first_seen: z.string(),
  cwd: z.string().nullable().optional(),
  transcript_path: z.string().nullable().optional(),
  git_info: GitInfoSchema.nullable().optional(),
  last_sync_at: z.string().nullable().optional(),
  files: z.array(SyncFileDetailSchema),
});

export const SessionShareSchema = z.object({
  id: z.number(),
  session_id: z.string(),
  external_id: z.string(),
  session_summary: z.string().nullable().optional(),
  session_first_user_message: z.string().nullable().optional(),
  share_token: z.string(),
  is_public: z.boolean(),
  recipients: z.array(z.string()).nullable().optional(),
  expires_at: z.string().nullable().optional(),
  created_at: z.string(),
  last_accessed_at: z.string().nullable().optional(),
});

// ============================================================================
// Auth Schemas
// ============================================================================

export const UserSchema = z.object({
  name: z.string(),
  email: z.string(),
  avatar_url: z.string(),
});

// ============================================================================
// API Key Schemas
// ============================================================================

export const APIKeySchema = z.object({
  id: z.number(),
  name: z.string(),
  created_at: z.string(),
  last_used_at: z.string().nullable().optional(),
});

export const CreateAPIKeyResponseSchema = z.object({
  id: z.number(),
  key: z.string(),
  name: z.string(),
  created_at: z.string(),
});

// ============================================================================
// Share Schemas
// ============================================================================

export const CreateShareResponseSchema = z.object({
  share_url: z.string(),
});

// ============================================================================
// Array Response Schemas
// ============================================================================

export const SessionListSchema = z.array(SessionSchema);
export const SessionShareListSchema = z.array(SessionShareSchema);
export const APIKeyListSchema = z.array(APIKeySchema);

// ============================================================================
// Inferred Types
// ============================================================================

export type GitInfo = z.infer<typeof GitInfoSchema>;
export type SyncFileDetail = z.infer<typeof SyncFileDetailSchema>;
export type Session = z.infer<typeof SessionSchema>;
export type SessionDetail = z.infer<typeof SessionDetailSchema>;
export type SessionShare = z.infer<typeof SessionShareSchema>;
export type User = z.infer<typeof UserSchema>;
export type APIKey = z.infer<typeof APIKeySchema>;
export type CreateAPIKeyResponse = z.infer<typeof CreateAPIKeyResponseSchema>;
export type CreateShareResponse = z.infer<typeof CreateShareResponseSchema>;

// ============================================================================
// Validation Functions
// ============================================================================

/**
 * Validate API response data against a schema.
 * Throws ZodError with detailed messages on failure.
 */
export function validateResponse<T>(schema: z.ZodType<T>, data: unknown, endpoint: string): T {
  const result = schema.safeParse(data);
  if (!result.success) {
    console.error(`API validation failed for ${endpoint}:`, result.error.issues);
    throw new APIValidationError(endpoint, result.error);
  }
  return result.data;
}

/**
 * Custom error class for API validation failures
 */
export class APIValidationError extends Error {
  endpoint: string;
  zodError: z.ZodError;

  constructor(endpoint: string, zodError: z.ZodError) {
    const issues = zodError.issues.map((i) => `${i.path.join('.')}: ${i.message}`).join('; ');
    super(`Invalid API response from ${endpoint}: ${issues}`);
    this.name = 'APIValidationError';
    this.endpoint = endpoint;
    this.zodError = zodError;
  }
}
