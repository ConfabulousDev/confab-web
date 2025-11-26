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

export const FileDetailSchema = z.object({
  id: z.number(),
  file_path: z.string(),
  file_type: z.string(),
  size_bytes: z.number(),
  s3_key: z.string().optional(),
  s3_uploaded_at: z.string().optional(),
});

export const RunDetailSchema = z.object({
  id: z.number(),
  end_timestamp: z.string(),
  cwd: z.string(),
  reason: z.string(),
  transcript_path: z.string(),
  s3_uploaded: z.boolean(),
  git_info: GitInfoSchema.optional(),
  files: z.array(FileDetailSchema),
});

// ============================================================================
// Session Schemas
// ============================================================================

export const SessionSchema = z.object({
  id: z.string(),
  external_id: z.string(),
  first_seen: z.string(),
  run_count: z.number(),
  last_run_time: z.string(),
  title: z.string().optional(),
  session_type: z.string(),
  max_transcript_size: z.number(),
  git_repo: z.string().optional(),
  git_branch: z.string().optional(),
  is_owner: z.boolean(),
  access_type: z.enum(['owner', 'private_share', 'public_share']),
  share_token: z.string().optional(),
  shared_by_email: z.string().optional(),
});

export const SessionDetailSchema = z.object({
  id: z.string(),
  external_id: z.string(),
  first_seen: z.string(),
  runs: z.array(RunDetailSchema),
});

export const SessionShareSchema = z.object({
  id: z.number(),
  session_id: z.string(),
  external_id: z.string(),
  session_title: z.string().optional(),
  share_token: z.string(),
  visibility: z.enum(['public', 'private']),
  invited_emails: z.array(z.string()).optional(),
  expires_at: z.string().optional(),
  created_at: z.string(),
  last_accessed_at: z.string().optional(),
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
  last_used_at: z.string().optional(),
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
export type FileDetail = z.infer<typeof FileDetailSchema>;
export type RunDetail = z.infer<typeof RunDetailSchema>;
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
