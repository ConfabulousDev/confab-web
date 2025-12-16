import { describe, it, expect } from 'vitest';
import { APIError, NetworkError, AuthenticationError } from './api';

describe('APIError', () => {
  it('should create APIError with correct properties', () => {
    const error = new APIError('Test error', 500, 'Internal Server Error', { detail: 'test' });

    expect(error.message).toBe('Test error');
    expect(error.status).toBe(500);
    expect(error.statusText).toBe('Internal Server Error');
    expect(error.data).toEqual({ detail: 'test' });
    expect(error.name).toBe('APIError');
  });

  it('should extract backend error message from data.error', () => {
    const error = new APIError('Request failed: Bad Request', 400, 'Bad Request', {
      error: 'Custom title exceeds maximum length of 255 characters',
    });

    expect(error.message).toBe('Custom title exceeds maximum length of 255 characters');
    expect(error.status).toBe(400);
    expect(error.data).toEqual({ error: 'Custom title exceeds maximum length of 255 characters' });
  });

  it('should fallback to generic message when data.error is not present', () => {
    const error = new APIError('Request failed: Internal Server Error', 500, 'Internal Server Error', {
      details: 'something went wrong',
    });

    expect(error.message).toBe('Request failed: Internal Server Error');
  });

  it('should fallback to generic message when data is not an object', () => {
    const error = new APIError('Request failed: Bad Request', 400, 'Bad Request', 'plain text error');

    expect(error.message).toBe('Request failed: Bad Request');
  });
});

describe('NetworkError', () => {
  it('should create NetworkError with correct properties', () => {
    const error = new NetworkError('Network failed');

    expect(error.message).toBe('Network failed');
    expect(error.name).toBe('NetworkError');
  });
});

describe('AuthenticationError', () => {
  it('should create AuthenticationError with default message', () => {
    const error = new AuthenticationError();

    expect(error.message).toBe('Authentication required');
    expect(error.status).toBe(401);
    expect(error.statusText).toBe('Unauthorized');
    expect(error.name).toBe('AuthenticationError');
  });

  it('should create AuthenticationError with custom message', () => {
    const error = new AuthenticationError('Custom auth error');

    expect(error.message).toBe('Custom auth error');
    expect(error.status).toBe(401);
  });
});
