// Centralized API client with error handling, interceptors, and Zod validation
// All API responses are validated at runtime to ensure type safety
import { z } from 'zod';
import { getCSRFToken, initCSRF, clearCSRFToken, updateCSRFTokenFromResponse } from './csrf';
import { shouldSkip401Redirect } from '@/utils/sessionErrors';
import {
  SessionDetailSchema,
  SessionListSchema,
  SessionShareListSchema,
  APIKeyListSchema,
  CreateAPIKeyResponseSchema,
  CreateShareResponseSchema,
  UserSchema,
  GitHubLinkSchema,
  GitHubLinksResponseSchema,
  SessionAnalyticsSchema,
  TrendsResponseSchema,
  validateResponse,
  type Session,
  type SessionDetail,
  type SessionShare,
  type APIKey,
  type CreateAPIKeyResponse,
  type CreateShareResponse,
  type User,
  type GitHubLink,
  type GitHubLinksResponse,
  type SessionAnalytics,
  type TrendsResponse,
} from '@/schemas/api';

// Re-export types for consumers
export type { GitHubLink, SessionAnalytics } from '@/schemas/api';

/**
 * Handles authentication failures by clearing cached state and redirecting to home.
 * Call this when a 401 response is received.
 */
function handleAuthFailure(): void {
  clearCSRFToken();
  window.location.href = '/';
}

export class APIError extends Error {
  status: number;
  statusText: string;
  data?: unknown;

  constructor(message: string, status: number, statusText: string, data?: unknown) {
    // Extract backend error message if available (format: {"error": "message"})
    const backendMessage = extractErrorMessage(data);
    super(backendMessage || message);
    this.name = 'APIError';
    this.status = status;
    this.statusText = statusText;
    this.data = data;
  }
}

/**
 * Type guard for backend error response format.
 */
function isErrorResponse(data: unknown): data is { error: string } {
  return (
    data !== null &&
    typeof data === 'object' &&
    'error' in data &&
    typeof data.error === 'string'
  );
}

/**
 * Extract error message from backend response data.
 * Backend returns errors as {"error": "message"}.
 */
function extractErrorMessage(data: unknown): string | null {
  if (isErrorResponse(data)) {
    return data.error;
  }
  return null;
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

interface RequestOptions extends Omit<RequestInit, 'body'> {
  skipAuth?: boolean;
  skipCSRF?: boolean;
  body?: unknown;
}

class APIClient {
  private baseURL: string;

  constructor(baseURL = '/api/v1') {
    this.baseURL = baseURL;
  }

  private async handleResponse(response: Response, endpoint: string): Promise<unknown> {
    // Update CSRF token from response header (keeps token fresh for next request)
    updateCSRFTokenFromResponse(response);

    // Handle authentication errors
    if (response.status === 401) {
      // Some endpoints handle 401 gracefully (e.g., showing login prompt)
      if (!shouldSkip401Redirect(endpoint)) {
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

      throw new APIError(`Request failed: ${response.statusText}`, response.status, response.statusText, errorData);
    }

    // Handle empty responses
    const contentType = response.headers.get('content-type');
    if (!contentType) {
      return undefined;
    }

    // Parse JSON responses
    if (contentType.includes('application/json')) {
      return response.json();
    }

    // Return text for other content types
    return response.text();
  }

  /**
   * Make an HTTP request and return the raw response.
   * Callers must validate/narrow the response type.
   */
  private async requestRaw(endpoint: string, options: RequestOptions = {}): Promise<unknown> {
    const { skipAuth, skipCSRF, body: requestBody, ...fetchOptions } = options;

    const url = endpoint.startsWith('http') ? endpoint : `${this.baseURL}${endpoint}`;

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
    if (requestBody !== undefined && requestBody !== null && typeof requestBody === 'object') {
      headers.set('Content-Type', 'application/json');
      body = JSON.stringify(requestBody);
    } else if (typeof requestBody === 'string') {
      body = requestBody;
    }

    const config: RequestInit = {
      ...fetchOptions,
      headers,
      body,
      credentials: skipAuth ? 'omit' : 'include',
    };

    try {
      const response = await fetch(url, config);
      return this.handleResponse(response, endpoint);
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

  /**
   * DELETE request that returns void
   */
  async deleteVoid(endpoint: string, options?: RequestOptions): Promise<void> {
    await this.requestRaw(endpoint, { ...options, method: 'DELETE' });
  }

  /**
   * GET request that returns string (for file content, etc.)
   */
  async getString(endpoint: string, options?: RequestOptions): Promise<string> {
    const result = await this.requestRaw(endpoint, { ...options, method: 'GET' });
    if (typeof result !== 'string') {
      throw new Error(`Expected string response from ${endpoint}`);
    }
    return result;
  }

  /**
   * Make a validated GET request
   * @param endpoint - API endpoint
   * @param schema - Zod schema to validate response
   */
  async getValidated<T>(endpoint: string, schema: z.ZodType<T>, options?: RequestOptions): Promise<T> {
    const data = await this.requestRaw(endpoint, { ...options, method: 'GET' });
    return validateResponse(schema, data, endpoint);
  }

  /**
   * Make a validated POST request
   * @param endpoint - API endpoint
   * @param schema - Zod schema to validate response
   * @param data - Request body
   */
  async postValidated<T>(
    endpoint: string,
    schema: z.ZodType<T>,
    data?: unknown,
    options?: RequestOptions
  ): Promise<T> {
    const response = await this.requestRaw(endpoint, {
      ...options,
      method: 'POST',
      body: data,
    });
    return validateResponse(schema, response, endpoint);
  }

  /**
   * Make a validated PATCH request
   * @param endpoint - API endpoint
   * @param schema - Zod schema to validate response
   * @param data - Request body
   */
  async patchValidated<T>(
    endpoint: string,
    schema: z.ZodType<T>,
    data?: unknown,
    options?: RequestOptions
  ): Promise<T> {
    const response = await this.requestRaw(endpoint, {
      ...options,
      method: 'PATCH',
      body: data,
    });
    return validateResponse(schema, response, endpoint);
  }

}

// Singleton instance
const api = new APIClient();

// Export validated API methods for common endpoints
// All responses are validated with Zod schemas at runtime

export const sessionsAPI = {
  list: (): Promise<Session[]> =>
    api.getValidated('/sessions', SessionListSchema),

  get: (sessionId: string): Promise<SessionDetail> =>
    api.getValidated(`/sessions/${sessionId}`, SessionDetailSchema),

  /**
   * Update the custom title for a session.
   * Pass null to clear the custom title and revert to auto-derived title.
   * @param sessionId - The session UUID
   * @param customTitle - The new title (max 255 chars) or null to clear
   */
  updateTitle: (sessionId: string, customTitle: string | null): Promise<SessionDetail> =>
    api.patchValidated(`/sessions/${sessionId}/title`, SessionDetailSchema, { custom_title: customTitle }),

  getShares: (sessionId: string): Promise<SessionShare[]> =>
    api.getValidated(`/sessions/${sessionId}/shares`, SessionShareListSchema),

  createShare: (
    sessionId: string,
    data: {
      is_public: boolean;
      recipients?: string[];
      expires_in_days?: number | null;
    }
  ): Promise<CreateShareResponse> =>
    api.postValidated(`/sessions/${sessionId}/share`, CreateShareResponseSchema, data),

  revokeShare: (shareId: number): Promise<void> => api.deleteVoid(`/shares/${shareId}`),
};

export const authAPI = {
  me: (): Promise<User> => api.getValidated('/me', UserSchema),
};

/**
 * Sync file API - access file content via sync API.
 * Uses canonical session endpoint which handles all access types
 * (owner, recipient share, system share, public share).
 */
export const syncFilesAPI = {
  /**
   * Get file content for a session.
   * Works for all access types - the backend determines access based on
   * session ownership, share status, and user authentication.
   * @param sessionId - The session UUID
   * @param fileName - Name of the file (e.g., "transcript.jsonl")
   * @param lineOffset - Optional: Return only lines after this line number (for incremental fetching)
   */
  getContent: (sessionId: string, fileName: string, lineOffset?: number): Promise<string> => {
    let url = `/sessions/${encodeURIComponent(sessionId)}/sync/file?file_name=${encodeURIComponent(fileName)}`;
    if (lineOffset !== undefined && lineOffset > 0) {
      url += `&line_offset=${lineOffset}`;
    }
    return api.getString(url);
  },
};

export const keysAPI = {
  list: (): Promise<APIKey[]> => api.getValidated('/keys', APIKeyListSchema),

  create: (name: string): Promise<CreateAPIKeyResponse> =>
    api.postValidated('/keys', CreateAPIKeyResponseSchema, { name }),

  delete: (keyId: number): Promise<void> => api.deleteVoid(`/keys/${keyId}`),
};

export const sharesAPI = {
  list: (): Promise<SessionShare[]> => api.getValidated('/shares', SessionShareListSchema),
};

export const githubLinksAPI = {
  /**
   * List GitHub links for a session.
   * Works for any user with session access (owner, shared, public).
   */
  list: (sessionId: string): Promise<GitHubLinksResponse> =>
    api.getValidated(`/sessions/${sessionId}/github-links`, GitHubLinksResponseSchema),

  /**
   * Create a new GitHub link for a session.
   * Requires session ownership.
   */
  create: (
    sessionId: string,
    data: {
      url: string;
      title?: string;
      source: 'cli_hook' | 'manual';
    }
  ): Promise<GitHubLink> =>
    api.postValidated(`/sessions/${sessionId}/github-links`, GitHubLinkSchema, data),

  /**
   * Delete a GitHub link.
   * Requires session ownership.
   */
  delete: (sessionId: string, linkId: number): Promise<void> =>
    api.deleteVoid(`/sessions/${sessionId}/github-links/${linkId}`),
};

export const analyticsAPI = {
  /**
   * Get analytics for a session with conditional request support.
   * Works for any user with session access (owner, shared, public).
   * Analytics are cached on the backend and recomputed when stale.
   *
   * @param sessionId - The session UUID
   * @param asOfLine - Optional line count client already has analytics for.
   *                   If provided and >= current line count, returns null (304 Not Modified).
   * @returns SessionAnalytics or null if no new data available
   */
  get: async (sessionId: string, asOfLine?: number): Promise<SessionAnalytics | null> => {
    let url = `/sessions/${sessionId}/analytics`;
    const hasCacheBustingParam = asOfLine !== undefined && asOfLine > 0;
    if (hasCacheBustingParam) {
      url += `?as_of_line=${asOfLine}`;
    }

    const fullUrl = `${api['baseURL']}${url}`;

    const response = await fetch(fullUrl, {
      method: 'GET',
      credentials: 'include',
      // Bypass browser cache when not using as_of_line param (e.g., during Smart Recap generation)
      // This ensures we get fresh data instead of a cached "generating" response
      ...(hasCacheBustingParam ? {} : { cache: 'no-store' as const }),
    });

    // Handle 304 Not Modified - no new data
    if (response.status === 304) {
      return null;
    }

    // Handle authentication errors
    if (response.status === 401) {
      handleAuthFailure();
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
      throw new APIError(`Request failed: ${response.statusText}`, response.status, response.statusText, errorData);
    }

    // Parse and validate response
    const data = await response.json();
    return validateResponse(SessionAnalyticsSchema, data, url);
  },

  /**
   * Force regeneration of the Smart Recap for a session.
   * Only available to session owners. Bypasses staleness check.
   *
   * @param sessionId - The session UUID
   * @returns SessionAnalytics with the smart_recap in "generating" state
   */
  regenerateSmartRecap: async (sessionId: string): Promise<SessionAnalytics> => {
    const url = `/sessions/${sessionId}/analytics/smart-recap/regenerate`;
    const fullUrl = `${api['baseURL']}${url}`;

    // Need CSRF token for POST
    await initCSRF();
    const csrfToken = getCSRFToken();

    const response = await fetch(fullUrl, {
      method: 'POST',
      credentials: 'include',
      headers: {
        'Content-Type': 'application/json',
        ...(csrfToken ? { 'X-CSRF-Token': csrfToken } : {}),
      },
    });

    // Handle authentication errors
    if (response.status === 401) {
      handleAuthFailure();
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
      throw new APIError(`Request failed: ${response.statusText}`, response.status, response.statusText, errorData);
    }

    // Parse and validate response
    const data = await response.json();
    return validateResponse(SessionAnalyticsSchema, data, url);
  },
};

export interface TrendsParams {
  startDate?: string; // YYYY-MM-DD
  endDate?: string;   // YYYY-MM-DD
  repos?: string[];
  includeNoRepo?: boolean;
}

export const trendsAPI = {
  /**
   * Get aggregated analytics trends across multiple sessions.
   *
   * @param params - Optional filter parameters
   * @returns TrendsResponse with aggregated card data
   */
  get: async (params: TrendsParams = {}): Promise<TrendsResponse> => {
    const searchParams = new URLSearchParams();
    if (params.startDate) searchParams.set('start_date', params.startDate);
    if (params.endDate) searchParams.set('end_date', params.endDate);
    if (params.repos && params.repos.length > 0) {
      searchParams.set('repos', params.repos.join(','));
    }
    if (params.includeNoRepo !== undefined) {
      searchParams.set('include_no_repo', String(params.includeNoRepo));
    }

    const queryString = searchParams.toString();
    const url = `/trends${queryString ? `?${queryString}` : ''}`;
    const fullUrl = `${api['baseURL']}${url}`;

    const response = await fetch(fullUrl, {
      method: 'GET',
      credentials: 'include',
    });

    // Handle authentication errors
    if (response.status === 401) {
      handleAuthFailure();
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
      throw new APIError(`Request failed: ${response.statusText}`, response.status, response.statusText, errorData);
    }

    // Parse and validate response
    const data = await response.json();
    return validateResponse(TrendsResponseSchema, data, url);
  },
};
