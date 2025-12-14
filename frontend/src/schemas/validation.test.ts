import { describe, it, expect } from 'vitest';
import {
  emailSchema,
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

  describe('shareFormSchema', () => {
    it('should accept valid public share', () => {
      const data = {
        is_public: true,
        recipients: [],
        expires_in_days: 7,
      };

      expect(shareFormSchema.parse(data)).toEqual(data);
    });

    it('should accept valid non-public share with recipients', () => {
      const data = {
        is_public: false,
        recipients: ['user1@example.com', 'user2@example.com'],
        expires_in_days: 30,
      };

      expect(shareFormSchema.parse(data)).toEqual(data);
    });

    it('should reject non-public share without recipients', () => {
      const data = {
        is_public: false,
        recipients: [],
      };

      expect(() => shareFormSchema.parse(data)).toThrow('Non-public shares must have at least one recipient email');
    });

    it('should accept null expires_in_days', () => {
      const data = {
        is_public: true,
        expires_in_days: null,
      };

      const result = shareFormSchema.parse(data);
      expect(result.expires_in_days).toBeNull();
    });

    it('should reject too many recipients', () => {
      const data = {
        is_public: false,
        recipients: Array(51).fill('test@example.com'),
      };

      expect(() => shareFormSchema.parse(data)).toThrow('Too many email addresses');
    });

    it('should reject invalid expiration days', () => {
      const data = {
        is_public: true,
        expires_in_days: -1,
      };

      expect(() => shareFormSchema.parse(data)).toThrow('Expiration must be positive');
    });

    it('should reject expiration > 365 days', () => {
      const data = {
        is_public: true,
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
        is_public: false,
        recipients: [],
      };
      const result = validateForm(shareFormSchema, data);

      expect(result.success).toBe(false);
      if (!result.success) {
        expect(result.errors).toHaveProperty('recipients');
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
