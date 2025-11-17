// CSRF Token Management
// Provides utilities to fetch and use CSRF tokens for secure API requests

let csrfToken: string = '';

/**
 * Fetches a new CSRF token from the backend
 * Should be called on app initialization
 */
export async function initCSRF(): Promise<void> {
	try {
		const response = await fetch('/api/v1/csrf-token', {
			credentials: 'include'
		});

		if (!response.ok) {
			console.error('Failed to fetch CSRF token:', response.statusText);
			return;
		}

		const data = await response.json();
		csrfToken = data.csrf_token;

		// Also check for token in response header
		const headerToken = response.headers.get('X-CSRF-Token');
		if (headerToken) {
			csrfToken = headerToken;
		}

		console.log('CSRF token initialized');
	} catch (error) {
		console.error('Error initializing CSRF token:', error);
	}
}

/**
 * Gets the current CSRF token
 * @returns The current CSRF token or empty string if not initialized
 */
export function getCSRFToken(): string {
	return csrfToken;
}

/**
 * Makes a fetch request with CSRF token automatically included
 * Use this for all POST, PUT, DELETE requests
 */
export async function fetchWithCSRF(
	url: string,
	options: RequestInit = {}
): Promise<Response> {
	// Ensure credentials are included
	const fetchOptions: RequestInit = {
		...options,
		credentials: 'include'
	};

	// Add CSRF token header for state-changing operations
	const method = (options.method || 'GET').toUpperCase();
	if (method !== 'GET' && method !== 'HEAD' && method !== 'OPTIONS') {
		fetchOptions.headers = {
			...fetchOptions.headers,
			'X-CSRF-Token': csrfToken
		};
	}

	return fetch(url, fetchOptions);
}
