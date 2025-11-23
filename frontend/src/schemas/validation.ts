// Zod validation schemas for forms and API responses
import { z } from 'zod';

// ============================================================================
// Common Schemas
// ============================================================================

export const emailSchema = z
  .string()
  .trim()
  .min(1, 'Email is required')
  .email('Invalid email address')
  .max(255, 'Email is too long');

export const sessionIdSchema = z
  .string()
  .min(1, 'Session ID is required')
  .regex(/^[a-zA-Z0-9_-]+$/, 'Invalid session ID format');

// ============================================================================
// Share Form Schemas
// ============================================================================

export const shareVisibilitySchema = z.enum(['public', 'private'], {
  message: 'Visibility must be either "public" or "private"',
});

export const shareFormSchema = z
  .object({
    visibility: shareVisibilitySchema,
    invited_emails: z
      .array(emailSchema)
      .max(50, 'Too many email addresses (max 50)')
      .optional()
      .default([]),
    expires_in_days: z
      .number()
      .int('Expiration must be a whole number')
      .positive('Expiration must be positive')
      .max(365, 'Maximum expiration is 365 days')
      .nullable()
      .optional(),
  })
  .refine(
    (data) => {
      // If visibility is private, must have at least one email
      if (data.visibility === 'private') {
        return data.invited_emails && data.invited_emails.length > 0;
      }
      return true;
    },
    {
      message: 'Private shares must have at least one invited email',
      path: ['invited_emails'],
    }
  );

export type ShareFormData = z.infer<typeof shareFormSchema>;

// ============================================================================
// API Key Schemas
// ============================================================================

export const apiKeyNameSchema = z
  .string()
  .trim()
  .min(1, 'Key name is required')
  .max(100, 'Key name is too long (max 100 characters)')
  .regex(/^[a-zA-Z0-9\s_-]+$/, 'Key name can only contain letters, numbers, spaces, hyphens, and underscores');

export const createAPIKeySchema = z.object({
  name: apiKeyNameSchema,
});

export type CreateAPIKeyData = z.infer<typeof createAPIKeySchema>;

// ============================================================================
// API Response Schemas (for runtime validation)
// ============================================================================

export const userSchema = z.object({
  name: z.string(),
  email: emailSchema,
  avatar_url: z.string().url().optional(),
});

export const gitInfoSchema = z.object({
  repo_url: z.string().optional(),
  branch: z.string().optional(),
  commit_sha: z.string().optional(),
  commit_message: z.string().optional(),
  author: z.string().optional(),
  is_dirty: z.boolean().optional(),
});

export const fileDetailSchema = z.object({
  id: z.number(),
  file_path: z.string(),
  file_type: z.string(),
  size_bytes: z.number(),
  s3_key: z.string().optional(),
  s3_uploaded_at: z.string().optional(),
});

export const runDetailSchema = z.object({
  id: z.number(),
  end_timestamp: z.string(),
  cwd: z.string(),
  reason: z.string(),
  transcript_path: z.string(),
  s3_uploaded: z.boolean(),
  git_info: gitInfoSchema.optional(),
  files: z.array(fileDetailSchema),
});

export const sessionDetailSchema = z.object({
  session_id: sessionIdSchema,
  first_seen: z.string(),
  runs: z.array(runDetailSchema),
});

export const sessionSchema = z.object({
  session_id: sessionIdSchema,
  first_seen: z.string(),
  run_count: z.number(),
  last_run_time: z.string(),
  title: z.string().optional(),
  session_type: z.string(),
  max_transcript_size: z.number(),
  git_repo: z.string().optional(),
  git_branch: z.string().optional(),
});

export const sessionShareSchema = z.object({
  id: z.number(),
  share_token: z.string(),
  visibility: shareVisibilitySchema,
  invited_emails: z.array(z.string()).optional(),
  expires_at: z.string().optional(),
  created_at: z.string(),
  last_accessed_at: z.string().optional(),
});

export const apiKeySchema = z.object({
  id: z.number(),
  name: z.string(),
  created_at: z.string(),
});

// ============================================================================
// Utility Functions
// ============================================================================

/**
 * Validate data and return typed result with errors
 */
export function validateForm<T>(
  schema: z.ZodSchema<T>,
  data: unknown
): { success: true; data: T } | { success: false; errors: Record<string, string[]> } {
  const result = schema.safeParse(data);

  if (result.success) {
    return { success: true, data: result.data };
  }

  // Flatten Zod errors into field-level errors
  const errors: Record<string, string[]> = {};

  // Zod v4 uses issues instead of errors
  const issues = result.error?.issues || [];

  issues.forEach((err) => {
    const path = err.path.join('.');
    if (!errors[path]) {
      errors[path] = [];
    }
    errors[path].push(err.message);
  });

  return { success: false, errors };
}

/**
 * Get first error message for a field
 */
export function getFieldError(errors: Record<string, string[]> | undefined, field: string): string | undefined {
  return errors?.[field]?.[0];
}

/**
 * Validate API response data
 */
export function validateResponse<T>(schema: z.ZodSchema<T>, data: unknown): T {
  return schema.parse(data);
}
