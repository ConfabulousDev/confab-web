import { describe, it, expect } from 'vitest';
import {
  emailSchema,
  sessionIdSchema,
  shareFormSchema,
  apiKeyNameSchema,
  createAPIKeySchema,
  validateForm,
  getFieldError,
} from './validation';

describe('validation schemas', () => {
  describe('emailSchema', () => {
    it('should accept valid emails', () => {
      expect(emailSchema.parse('test@example.com')).toBe('test@example.com');
      expect(emailSchema.parse('user.name+tag@example.co.uk')).toBe('user.name+tag@example.co.uk');
    });

    it('should trim whitespace', () => {
      expect(emailSchema.parse('  test@example.com  ')).toBe('test@example.com');
    });

    it('should reject invalid emails', () => {
      expect(() => emailSchema.parse('invalid')).toThrow();
      expect(() => emailSchema.parse('@example.com')).toThrow();
      expect(() => emailSchema.parse('user@')).toThrow();
      expect(() => emailSchema.parse('')).toThrow('Email is required');
    });

    it('should reject emails that are too long', () => {
      const longEmail = 'a'.repeat(250) + '@example.com';
      expect(() => emailSchema.parse(longEmail)).toThrow('Email is too long');
    });
  });

  describe('sessionIdSchema', () => {
    it('should accept valid session IDs', () => {
      expect(sessionIdSchema.parse('abc123')).toBe('abc123');
      expect(sessionIdSchema.parse('session-123_test')).toBe('session-123_test');
    });

    it('should reject invalid session IDs', () => {
      expect(() => sessionIdSchema.parse('')).toThrow('Session ID is required');
      expect(() => sessionIdSchema.parse('invalid@session')).toThrow('Invalid session ID format');
      expect(() => sessionIdSchema.parse('has spaces')).toThrow('Invalid session ID format');
    });
  });

  describe('shareFormSchema', () => {
    it('should accept valid public share', () => {
      const data = {
        visibility: 'public' as const,
        invited_emails: [],
        expires_in_days: 7,
      };

      expect(shareFormSchema.parse(data)).toEqual(data);
    });

    it('should accept valid private share with emails', () => {
      const data = {
        visibility: 'private' as const,
        invited_emails: ['user1@example.com', 'user2@example.com'],
        expires_in_days: 30,
      };

      expect(shareFormSchema.parse(data)).toEqual(data);
    });

    it('should reject private share without emails', () => {
      const data = {
        visibility: 'private' as const,
        invited_emails: [],
      };

      expect(() => shareFormSchema.parse(data)).toThrow('Private shares must have at least one invited email');
    });

    it('should accept null expires_in_days', () => {
      const data = {
        visibility: 'public' as const,
        expires_in_days: null,
      };

      const result = shareFormSchema.parse(data);
      expect(result.expires_in_days).toBeNull();
    });

    it('should reject invalid visibility', () => {
      const data = {
        visibility: 'invalid',
      };

      expect(() => shareFormSchema.parse(data)).toThrow();
    });

    it('should reject too many emails', () => {
      const data = {
        visibility: 'private' as const,
        invited_emails: Array(51).fill('test@example.com'),
      };

      expect(() => shareFormSchema.parse(data)).toThrow('Too many email addresses');
    });

    it('should reject invalid expiration days', () => {
      const data = {
        visibility: 'public' as const,
        expires_in_days: -1,
      };

      expect(() => shareFormSchema.parse(data)).toThrow('Expiration must be positive');
    });

    it('should reject expiration > 365 days', () => {
      const data = {
        visibility: 'public' as const,
        expires_in_days: 400,
      };

      expect(() => shareFormSchema.parse(data)).toThrow('Maximum expiration is 365 days');
    });
  });

  describe('apiKeyNameSchema', () => {
    it('should accept valid key names', () => {
      expect(apiKeyNameSchema.parse('Production Server')).toBe('Production Server');
      expect(apiKeyNameSchema.parse('my-laptop_2024')).toBe('my-laptop_2024');
    });

    it('should trim whitespace', () => {
      expect(apiKeyNameSchema.parse('  Test Key  ')).toBe('Test Key');
    });

    it('should reject empty names', () => {
      expect(() => apiKeyNameSchema.parse('')).toThrow('Key name is required');
      expect(() => apiKeyNameSchema.parse('   ')).toThrow('Key name is required');
    });

    it('should reject names that are too long', () => {
      const longName = 'a'.repeat(101);
      expect(() => apiKeyNameSchema.parse(longName)).toThrow('Key name is too long');
    });

    it('should reject invalid characters', () => {
      expect(() => apiKeyNameSchema.parse('test@key')).toThrow('Key name can only contain');
      expect(() => apiKeyNameSchema.parse('test!key')).toThrow('Key name can only contain');
    });
  });

  describe('createAPIKeySchema', () => {
    it('should accept valid API key data', () => {
      const data = { name: 'Test Key' };
      expect(createAPIKeySchema.parse(data)).toEqual(data);
    });

    it('should reject missing name', () => {
      expect(() => createAPIKeySchema.parse({})).toThrow();
    });
  });

  describe('validateForm', () => {
    it('should return success with valid data', () => {
      const data = { name: 'Test Key' };
      const result = validateForm(createAPIKeySchema, data);

      expect(result.success).toBe(true);
      if (result.success) {
        expect(result.data).toEqual(data);
      }
    });

    it('should return errors with invalid data', () => {
      const data = { name: '' };
      const result = validateForm(createAPIKeySchema, data);

      expect(result.success).toBe(false);
      if (!result.success) {
        expect(result.errors).toHaveProperty('name');
        expect(result.errors.name).toContain('Key name is required');
      }
    });

    it('should flatten nested errors', () => {
      const data = {
        visibility: 'private' as const,
        invited_emails: [],
      };
      const result = validateForm(shareFormSchema, data);

      expect(result.success).toBe(false);
      if (!result.success) {
        expect(result.errors).toHaveProperty('invited_emails');
      }
    });
  });

  describe('getFieldError', () => {
    it('should return first error for field', () => {
      const errors = {
        name: ['Error 1', 'Error 2'],
        email: ['Email error'],
      };

      expect(getFieldError(errors, 'name')).toBe('Error 1');
      expect(getFieldError(errors, 'email')).toBe('Email error');
    });

    it('should return undefined for missing field', () => {
      const errors = {
        name: ['Error 1'],
      };

      expect(getFieldError(errors, 'email')).toBeUndefined();
    });

    it('should handle undefined errors', () => {
      expect(getFieldError(undefined, 'name')).toBeUndefined();
    });
  });
});
