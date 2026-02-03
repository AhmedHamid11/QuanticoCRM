import { PUBLIC_API_URL } from '$env/static/public';
import type { FieldValidationError } from '$lib/types/validation';

const API_BASE = PUBLIC_API_URL || '/api/v1';
const STORAGE_KEY = 'quantico_auth';

interface ApiOptions {
	method?: string;
	body?: unknown;
	headers?: Record<string, string>;
	signal?: AbortSignal;
}

// Extended Error class to include field validation errors
export class ApiError extends Error {
	fieldErrors?: FieldValidationError[];
	status?: number;

	constructor(message: string, status?: number, fieldErrors?: FieldValidationError[]) {
		super(message);
		this.name = 'ApiError';
		this.status = status;
		this.fieldErrors = fieldErrors;
	}
}

function getAuthToken(): string | null {
	if (typeof window === 'undefined') return null;
	try {
		const stored = localStorage.getItem(STORAGE_KEY);
		if (stored) {
			const data = JSON.parse(stored);
			return data.accessToken || null;
		}
	} catch {
		// Ignore parse errors
	}
	return null;
}

function handleSessionExpired(): void {
	if (typeof window === 'undefined') return;

	// Clear auth state from localStorage
	localStorage.removeItem(STORAGE_KEY);

	// Redirect to login with message
	window.location.href = '/login?session_expired=true';
}

// Check if an error is retryable (network/CORS failures)
function isRetryableError(error: unknown): boolean {
	if (error instanceof Error) {
		// Network errors, CORS failures, connection refused
		const message = error.message.toLowerCase();
		return (
			message.includes('failed to fetch') ||
			message.includes('network') ||
			message.includes('cors') ||
			message.includes('err_failed')
		);
	}
	return false;
}

// Delay helper for retry backoff
function delay(ms: number): Promise<void> {
	return new Promise(resolve => setTimeout(resolve, ms));
}

export async function api<T>(endpoint: string, options: ApiOptions = {}): Promise<T> {
	const { method = 'GET', body, headers = {}, signal } = options;

	const maxRetries = 3;
	const baseDelay = 200; // Start with 200ms delay

	const authToken = getAuthToken();
	const authHeaders: Record<string, string> = {};
	if (authToken) {
		authHeaders['Authorization'] = `Bearer ${authToken}`;
	}

	const config: RequestInit = {
		method,
		headers: {
			'Content-Type': 'application/json',
			...authHeaders,
			...headers
		},
		signal
	};

	if (body) {
		config.body = JSON.stringify(body);
	}

	let lastError: Error | null = null;

	for (let attempt = 0; attempt < maxRetries; attempt++) {
		try {
			// Check if request was aborted before attempting
			if (signal?.aborted) {
				throw new DOMException('Aborted', 'AbortError');
			}

			const response = await fetch(`${API_BASE}${endpoint}`, config);

			if (!response.ok) {
				const error = await response.json().catch(() => ({ error: 'Unknown error' }));
				const errorMessage = error.error || `HTTP ${response.status}`;

				// Handle 401 Unauthorized - session expired (don't retry)
				if (response.status === 401 && errorMessage.toLowerCase().includes('invalid or expired token')) {
					handleSessionExpired();
					throw new ApiError('Session expired', 401);
				}

				// Handle 422 Unprocessable Entity - validation errors (don't retry)
				if (response.status === 422 && error.fieldErrors) {
					throw new ApiError(errorMessage, 422, error.fieldErrors);
				}

				// Don't retry 4xx client errors (except for specific cases)
				if (response.status >= 400 && response.status < 500) {
					throw new ApiError(errorMessage, response.status);
				}

				// Retry 5xx server errors
				if (response.status >= 500 && attempt < maxRetries - 1) {
					lastError = new ApiError(errorMessage, response.status);
					await delay(baseDelay * Math.pow(2, attempt));
					continue;
				}

				throw new ApiError(errorMessage, response.status);
			}

			// Handle empty responses (204 No Content)
			if (response.status === 204) {
				return undefined as T;
			}

			return response.json();
		} catch (error) {
			// Don't retry abort errors
			if (error instanceof Error && error.name === 'AbortError') {
				throw error;
			}

			// Don't retry ApiErrors (already handled above)
			if (error instanceof ApiError) {
				throw error;
			}

			// Retry network/CORS errors
			if (isRetryableError(error) && attempt < maxRetries - 1) {
				lastError = error as Error;
				await delay(baseDelay * Math.pow(2, attempt));
				continue;
			}

			throw error;
		}
	}

	// If we exhausted retries, throw the last error
	throw lastError || new Error('Request failed after retries');
}

// Check if an error is an abort error (request was cancelled)
export function isAbortError(error: unknown): boolean {
	if (error instanceof Error) {
		// Only treat as abort if it's specifically an AbortError
		// Don't treat "Failed to fetch" as abort - that could be a real network error
		return error.name === 'AbortError';
	}
	return false;
}

// Convenience methods
export const get = <T>(endpoint: string, signal?: AbortSignal) => api<T>(endpoint, { signal });
export const post = <T>(endpoint: string, body: unknown, signal?: AbortSignal) => api<T>(endpoint, { method: 'POST', body, signal });
export const put = <T>(endpoint: string, body: unknown, signal?: AbortSignal) => api<T>(endpoint, { method: 'PUT', body, signal });
export const patch = <T>(endpoint: string, body: unknown, signal?: AbortSignal) => api<T>(endpoint, { method: 'PATCH', body, signal });
export const del = <T>(endpoint: string, signal?: AbortSignal) => api<T>(endpoint, { method: 'DELETE', signal });
