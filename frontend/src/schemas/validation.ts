// Zod validation schemas for form input validation
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

// ============================================================================
// Share Form Schemas
// ============================================================================

export const shareFormSchema = z
  .object({
    is_public: z.boolean(),
    recipients: z
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
      // If not public, must have at least one recipient
      if (!data.is_public) {
        return data.recipients && data.recipients.length > 0;
      }
      return true;
    },
    {
      message: 'Non-public shares must have at least one recipient email',
      path: ['recipients'],
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
