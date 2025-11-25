// Centralized API client with error handling and interceptors
import { getCSRFToken, initCSRF, clearCSRFToken } from './csrf';
import type { z } from 'zod';

/**
 * Handles authentication failures by clearing cached state and redirecting to home.
 * Call this when a 401 response is received.
 */
export function handleAuthFailure(): void {
  clearCSRFToken();
  window.location.href = '/';
}

export class APIError extends Error {
  status: number;
  statusText: string;
  data?: unknown;

  constructor(
    message: string,
    status: number,
    statusText: string,
    data?: unknown
  ) {
    super(message);
    this.name = 'APIError';
    this.status = status;
    this.statusText = statusText;
    this.data = data;
  }
}

export class NetworkError extends Error {
  constructor(message: string) {
    super(message);
    this.name = 'NetworkError';
  }
}

export class AuthenticationError extends APIError {
  constructor(message = 'Authentication required') {
    super(message, 401, 'Unauthorized');
    this.name = 'AuthenticationError';
  }
}

interface RequestOptions extends RequestInit {
  skipAuth?: boolean;
  skipCSRF?: boolean;
  validateResponse?: z.ZodSchema<unknown>;
}

class APIClient {
  private baseURL: string;

  constructor(baseURL = '/api/v1') {
    this.baseURL = baseURL;
  }

  private async handleResponse<T>(response: Response, endpoint: string): Promise<T> {
    // Handle authentication errors
    if (response.status === 401) {
      // Don't redirect for /me - it's expected to return 401 when not logged in
      // The useAuth hook handles this gracefully
      if (!endpoint.endsWith('/me')) {
        handleAuthFailure();
      }
      throw new AuthenticationError();
    }

    // Handle other HTTP errors
    if (!response.ok) {
      let errorData: unknown;
      try {
        errorData = await response.json();
      } catch {
        errorData = await response.text();
      }

      throw new APIError(
        `Request failed: ${response.statusText}`,
        response.status,
        response.statusText,
        errorData
      );
    }

    // Handle empty responses
    const contentType = response.headers.get('content-type');
    if (!contentType) {
      return undefined as T;
    }

    // Parse JSON responses
    if (contentType.includes('application/json')) {
      return response.json();
    }

    // Return text for other content types
    return response.text() as T;
  }

  async request<T>(
    endpoint: string,
    options: RequestOptions = {}
  ): Promise<T> {
    const { skipAuth, skipCSRF, validateResponse, ...fetchOptions } = options;

    const url = endpoint.startsWith('http')
      ? endpoint
      : `${this.baseURL}${endpoint}`;

    const headers = new Headers(fetchOptions.headers);

    // Add CSRF token for state-changing operations
    const method = (options.method || 'GET').toUpperCase();
    if (!skipCSRF && ['POST', 'PUT', 'DELETE', 'PATCH'].includes(method)) {
      let token = getCSRFToken();

      // Initialize CSRF token if not present
      if (!token) {
        await initCSRF();
        token = getCSRFToken();
      }

      if (token) {
        headers.set('X-CSRF-Token', token);
      }
    }

    // Add JSON content type and stringify if body is an object
    let body: BodyInit | undefined;
    if (fetchOptions.body && typeof fetchOptions.body === 'object') {
      headers.set('Content-Type', 'application/json');
      body = JSON.stringify(fetchOptions.body);
    } else {
      body = fetchOptions.body ?? undefined;
    }

    const config: RequestInit = {
      ...fetchOptions,
      headers,
      body,
      credentials: skipAuth ? 'omit' : 'include',
    };

    try {
      const response = await fetch(url, config);
      const data = await this.handleResponse<T>(response, endpoint);

      // Optional runtime validation with Zod
      if (validateResponse) {
        try {
          return validateResponse.parse(data) as T;
        } catch (validationError) {
          throw new APIError(
            'Invalid response data from server',
            response.status,
            response.statusText,
            validationError
          );
        }
      }

      return data;
    } catch (error) {
      if (error instanceof APIError || error instanceof AuthenticationError) {
        throw error;
      }

      // Network or other errors
      if (error instanceof TypeError) {
        throw new NetworkError('Network request failed. Please check your connection.');
      }

      throw error;
    }
  }

  async get<T>(endpoint: string, options?: RequestOptions): Promise<T> {
    return this.request<T>(endpoint, { ...options, method: 'GET' });
  }

  async post<T>(
    endpoint: string,
    data?: unknown,
    options?: RequestOptions
  ): Promise<T> {
    return this.request<T>(endpoint, {
      ...options,
      method: 'POST',
      body: data as BodyInit,
    });
  }

  async put<T>(
    endpoint: string,
    data?: unknown,
    options?: RequestOptions
  ): Promise<T> {
    return this.request<T>(endpoint, {
      ...options,
      method: 'PUT',
      body: data as BodyInit,
    });
  }

  async delete<T>(endpoint: string, options?: RequestOptions): Promise<T> {
    return this.request<T>(endpoint, { ...options, method: 'DELETE' });
  }

  async patch<T>(
    endpoint: string,
    data?: unknown,
    options?: RequestOptions
  ): Promise<T> {
    return this.request<T>(endpoint, {
      ...options,
      method: 'PATCH',
      body: data as BodyInit,
    });
  }
}

// Export singleton instance
export const api = new APIClient();

// Export type-safe API methods for common endpoints
export const sessionsAPI = {
  list: (includeShared = false) =>
    api.get<Array<import('@/types').Session>>(
      `/sessions${includeShared ? '?include_shared=true' : ''}`
    ),

  get: (sessionId: string) =>
    api.get<import('@/types').SessionDetail>(`/sessions/${sessionId}`),

  getShares: (sessionId: string) =>
    api.get<Array<import('@/types').SessionShare>>(`/sessions/${sessionId}/shares`),

  createShare: (
    sessionId: string,
    data: {
      visibility: 'public' | 'private';
      invited_emails?: string[];
      expires_in_days?: number | null;
    }
  ) =>
    api.post<{ share_url: string }>(`/sessions/${sessionId}/share`, data),

  revokeShare: (shareToken: string) =>
    api.delete(`/shares/${shareToken}`),
};

export const authAPI = {
  me: () => api.get<{ name: string; email: string; avatar_url: string }>('/me'),
};

export const filesAPI = {
  getContent: (runId: number, fileId: number) =>
    api.get<string>(`/runs/${runId}/files/${fileId}/content`),

  getSharedContent: (sessionId: string, shareToken: string, fileId: number) =>
    api.get<string>(`/sessions/${sessionId}/shared/${shareToken}/files/${fileId}/content`),
};

export const keysAPI = {
  list: () => api.get<Array<import('@/types').APIKey>>('/keys'),

  create: (name: string) =>
    api.post<{ key: string; api_key: import('@/types').APIKey }>('/keys', { name }),

  delete: (keyId: number) => api.delete(`/keys/${keyId}`),
};
