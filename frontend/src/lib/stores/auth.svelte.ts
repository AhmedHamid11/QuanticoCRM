// Auth store for Quantico CRM using Svelte 5 runes
// SECURITY: Access tokens are stored only in memory (reactive state)
// SECURITY: Refresh tokens are in HttpOnly cookies - never accessible to JavaScript
import { PUBLIC_API_URL } from '$env/static/public';
import { initSessionTracking, stopSessionTracking } from './session.svelte.ts';
import type {
	AuthState,
	AuthResponse,
	UserWithOrgs,
	Membership,
	CurrentUser,
	RegisterInput,
	LoginInput,
	SwitchOrgInput,
	ImpersonateInput,
	ChangePasswordInput
} from '$lib/types/auth';

const STORAGE_KEY = 'quantico_auth';

// Mutex to prevent concurrent refresh attempts (causes token reuse detection)
let refreshPromise: Promise<boolean> | null = null;

// Cross-tab refresh coordination via localStorage lock.
// Prevents two browser tabs from refreshing simultaneously, which causes
// the backend to detect "token reuse" and revoke the entire session family.
const REFRESH_LOCK_KEY = 'quantico_refresh_lock';
const LOCK_TTL_MS = 10000; // 10s max lock hold time

function isRefreshLockedByOtherTab(): boolean {
	if (typeof window === 'undefined') return false;
	try {
		const lock = localStorage.getItem(REFRESH_LOCK_KEY);
		if (!lock) return false;
		return Date.now() - parseInt(lock, 10) < LOCK_TTL_MS;
	} catch {
		return false;
	}
}

function setRefreshLock(): void {
	try {
		localStorage.setItem(REFRESH_LOCK_KEY, Date.now().toString());
	} catch {}
}

function clearRefreshLock(): void {
	try {
		localStorage.removeItem(REFRESH_LOCK_KEY);
	} catch {}
}

function waitForRefreshLock(): Promise<void> {
	return new Promise((resolve) => {
		const maxWait = setTimeout(resolve, LOCK_TTL_MS);
		const interval = setInterval(() => {
			if (!isRefreshLockedByOtherTab()) {
				clearInterval(interval);
				clearTimeout(maxWait);
				resolve();
			}
		}, 100);
	});
}

// Silent refresh function - declared here so it can be used before definition
async function silentRefresh(): Promise<boolean> {
	// In-tab mutex: if this tab is already refreshing, wait for the same promise
	if (refreshPromise) {
		return refreshPromise;
	}

	// Cross-tab mutex: if another tab is refreshing, wait for it to finish
	// then refresh ourselves (each tab needs its own access token in memory)
	if (isRefreshLockedByOtherTab()) {
		await waitForRefreshLock();
	}

	refreshPromise = (async () => {
		setRefreshLock();
		try {
			return await doSilentRefresh();
		} finally {
			clearRefreshLock();
		}
	})();
	try {
		return await refreshPromise;
	} finally {
		refreshPromise = null;
	}
}

async function doSilentRefresh(): Promise<boolean> {
	try {
		// No body needed - refresh token is in HttpOnly cookie
		// Browser sends it automatically with credentials: 'include'
		const API_BASE = PUBLIC_API_URL || '/api/v1';
		const response = await fetch(`${API_BASE}/auth/refresh`, {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			credentials: 'include' // CRITICAL: Sends HttpOnly cookie
		});

		if (response.ok) {
			const data = await response.json();
			state.accessToken = data.accessToken;
			state.expiresAt = new Date(data.expiresAt);
			state.isAuthenticated = true;
			if (data.user) {
				state.user = data.user;
				const defaultOrg =
					data.user.memberships.find((m: Membership) => m.isDefault) || data.user.memberships[0];
				state.currentOrg = defaultOrg || null;
			}
			// Decode JWT to get mustChangePassword flag
			const claims = decodeJWT(data.accessToken);
			state.mustChangePassword = claims?.mustChangePassword === true;
			persistState();
			return true;
		}
		return false;
	} catch {
		return false;
	}
}

// Initial state - SECURITY: tokens are NOT restored from localStorage
function getInitialState(): AuthState {
	if (typeof window === 'undefined') {
		return {
			user: null,
			currentOrg: null,
			accessToken: null, // Memory only - NOT restored from storage
			expiresAt: null,
			isAuthenticated: false,
			isLoading: true, // Start true - we'll try silent refresh
			isImpersonation: false,
			impersonatedBy: null,
			mustChangePassword: false
		};
	}

	// Restore only non-sensitive data from localStorage
	try {
		const stored = localStorage.getItem(STORAGE_KEY);
		if (stored) {
			const data = JSON.parse(stored);
			return {
				user: data.user,
				currentOrg: data.currentOrg,
				accessToken: null, // DO NOT restore - memory only
				expiresAt: null, // Must re-validate via refresh
				isAuthenticated: false, // Not authenticated until refresh succeeds
				isLoading: true, // Will attempt silent refresh
				isImpersonation: data.isImpersonation || false,
				impersonatedBy: data.impersonatedBy || null,
				mustChangePassword: false // Will be set on login/refresh
			};
		}
	} catch (e) {
		console.error('Failed to restore auth state:', e);
	}

	return {
		user: null,
		currentOrg: null,
		accessToken: null,
		expiresAt: null,
		isAuthenticated: false,
		isLoading: false,
		isImpersonation: false,
		impersonatedBy: null,
		mustChangePassword: false
	};
}

// Create reactive state
let state = $state<AuthState>(getInitialState());

// Persist to localStorage - SECURITY: NEVER persist tokens
function persistState() {
	if (typeof window === 'undefined') return;

	try {
		// SECURITY: Only persist non-sensitive data
		// Access token: memory only (XSS protection)
		// Refresh token: HttpOnly cookie only (not accessible to JS)
		const toStore = {
			user: state.user,
			currentOrg: state.currentOrg,
			isImpersonation: state.isImpersonation,
			impersonatedBy: state.impersonatedBy
			// NO tokens persisted
		};
		localStorage.setItem(STORAGE_KEY, JSON.stringify(toStore));
	} catch (e) {
		console.error('Failed to persist auth state:', e);
	}
}

// Clear persisted state
function clearPersistedState() {
	if (typeof window === 'undefined') return;
	localStorage.removeItem(STORAGE_KEY);
}

// Get CSRF token from cookie for state-changing requests
function getCSRFToken(): string | null {
	if (typeof document === 'undefined') return null;
	const match = document.cookie.match(/(?:^|; )csrf_token=([^;]*)/);
	return match ? decodeURIComponent(match[1]) : null;
}

// API helper with auth - CRITICAL: credentials: 'include' for cookie transmission
async function authFetch<T>(
	endpoint: string,
	options: {
		method?: string;
		body?: unknown;
		requiresAuth?: boolean;
	} = {}
): Promise<T> {
	const { method = 'GET', body, requiresAuth = true } = options;

	const headers: Record<string, string> = {
		'Content-Type': 'application/json'
	};

	if (requiresAuth && state.accessToken) {
		headers['Authorization'] = `Bearer ${state.accessToken}`;
	}

	// CSRF token required for state-changing requests (POST, PUT, PATCH, DELETE)
	if (method !== 'GET' && method !== 'HEAD' && method !== 'OPTIONS') {
		const csrfToken = getCSRFToken();
		if (csrfToken) {
			headers['X-CSRF-Token'] = csrfToken;
		}
	}

	const API_BASE = PUBLIC_API_URL || '/api/v1';
	const response = await fetch(`${API_BASE}${endpoint}`, {
		method,
		headers,
		body: body ? JSON.stringify(body) : undefined,
		credentials: 'include' // CRITICAL: Send HttpOnly cookies
	});

	if (!response.ok) {
		const error = await response.json().catch(() => ({ error: 'Unknown error' }));

		// Handle PASSWORD_CHANGE_REQUIRED - redirect to change-password
		if (response.status === 403 && error.code === 'PASSWORD_CHANGE_REQUIRED') {
			if (typeof window !== 'undefined') {
				window.location.href = '/change-password?required=true';
			}
			throw new Error(error.message || 'Password change required');
		}

		// Handle 401 - try silent refresh
		if (response.status === 401 && requiresAuth) {
			const refreshed = await silentRefresh();
			if (refreshed) {
				// Retry with new token (and fresh CSRF token)
				headers['Authorization'] = `Bearer ${state.accessToken}`;
				const freshCsrfToken = getCSRFToken();
				if (freshCsrfToken && method !== 'GET' && method !== 'HEAD' && method !== 'OPTIONS') {
					headers['X-CSRF-Token'] = freshCsrfToken;
				}
				const retryResponse = await fetch(`${API_BASE}${endpoint}`, {
					method,
					headers,
					body: body ? JSON.stringify(body) : undefined,
					credentials: 'include'
				});
				if (retryResponse.ok) {
					if (retryResponse.status === 204) return undefined as T;
					return retryResponse.json();
				}
			}
			await logoutWithSessionExpired();
			throw new Error('Session expired');
		}

		throw new Error(error.error || `HTTP ${response.status}`);
	}

	if (response.status === 204) {
		return undefined as T;
	}

	return response.json();
}

// Decode JWT to extract claims (client-side only, no verification)
function decodeJWT(token: string): Record<string, unknown> | null {
	try {
		const base64Url = token.split('.')[1];
		const base64 = base64Url.replace(/-/g, '+').replace(/_/g, '/');
		const jsonPayload = decodeURIComponent(
			atob(base64)
				.split('')
				.map((c) => '%' + ('00' + c.charCodeAt(0).toString(16)).slice(-2))
				.join('')
		);
		return JSON.parse(jsonPayload);
	} catch {
		return null;
	}
}

// Set auth state from response - no refreshToken (it's in cookie)
function setAuthState(response: AuthResponse) {
	const defaultOrg =
		response.user.memberships.find((m) => m.isDefault) || response.user.memberships[0];

	state.user = response.user;
	state.currentOrg = defaultOrg || null;
	state.accessToken = response.accessToken;
	// No refreshToken - it's in HttpOnly cookie
	state.expiresAt = new Date(response.expiresAt);
	state.isAuthenticated = true;
	state.isLoading = false;

	// Decode JWT to get mustChangePassword flag
	const claims = decodeJWT(response.accessToken);
	state.mustChangePassword = claims?.mustChangePassword === true;

	persistState();

	// Initialize session tracking with org settings (defaults if not present)
	const idleTimeout = (response.user as any).orgSettings?.idleTimeoutMinutes ?? 30;
	const absoluteTimeout = (response.user as any).orgSettings?.absoluteTimeoutMinutes ?? 1440;
	initSessionTracking(idleTimeout, absoluteTimeout);
}

// Auth actions
export async function register(input: RegisterInput): Promise<void> {
	state.isLoading = true;
	try {
		const response = await authFetch<AuthResponse>('/auth/register', {
			method: 'POST',
			body: input,
			requiresAuth: false
		});
		setAuthState(response);
	} finally {
		state.isLoading = false;
	}
}

export async function login(input: LoginInput): Promise<void> {
	state.isLoading = true;
	try {
		const response = await authFetch<AuthResponse>('/auth/login', {
			method: 'POST',
			body: input,
			requiresAuth: false
		});
		setAuthState(response);
	} finally {
		state.isLoading = false;
	}
}

// Logout - no body needed, server reads refresh token from cookie
export async function logout(): Promise<void> {
	try {
		// No body needed - server reads refresh token from cookie
		await authFetch('/auth/logout', {
			method: 'POST'
		}).catch(() => {});
	} finally {
		stopSessionTracking();
		state.user = null;
		state.currentOrg = null;
		state.accessToken = null;
		state.expiresAt = null;
		state.isAuthenticated = false;
		state.isImpersonation = false;
		state.impersonatedBy = null;
		state.mustChangePassword = false;
		clearPersistedState();
	}
}

// Logout and redirect to login with session expired message
export async function logoutWithSessionExpired(): Promise<void> {
	stopSessionTracking();
	state.user = null;
	state.currentOrg = null;
	state.accessToken = null;
	state.expiresAt = null;
	state.isAuthenticated = false;
	state.isImpersonation = false;
	state.impersonatedBy = null;
	state.mustChangePassword = false;
	clearPersistedState();

	// Redirect to login with session expired flag
	if (typeof window !== 'undefined') {
		window.location.href = '/login?session_expired=true';
	}
}

// Refresh tokens - uses HttpOnly cookie automatically
export async function refreshTokens(): Promise<boolean> {
	return silentRefresh();
}

export async function switchOrg(input: SwitchOrgInput): Promise<void> {
	state.isLoading = true;
	try {
		const response = await authFetch<AuthResponse>('/auth/switch-org', {
			method: 'POST',
			body: input
		});
		setAuthState(response);
	} finally {
		state.isLoading = false;
	}
}

export async function getCurrentUser(): Promise<CurrentUser> {
	return authFetch<CurrentUser>('/auth/me');
}

export async function changePassword(input: ChangePasswordInput): Promise<void> {
	const response = await authFetch<AuthResponse>('/auth/change-password', {
		method: 'POST',
		body: input
	});
	// Update auth state with new tokens (mustChangePassword will be false)
	setAuthState(response);
}

// Platform admin actions
export async function impersonate(input: ImpersonateInput): Promise<void> {
	state.isLoading = true;
	try {
		const response = await authFetch<AuthResponse>('/auth/impersonate', {
			method: 'POST',
			body: input
		});
		state.isImpersonation = true;
		state.impersonatedBy = state.user?.id || null;
		setAuthState(response);
	} finally {
		state.isLoading = false;
	}
}

export async function stopImpersonation(): Promise<void> {
	state.isLoading = true;
	try {
		const response = await authFetch<AuthResponse>('/auth/stop-impersonate', {
			method: 'POST'
		});
		state.isImpersonation = false;
		state.impersonatedBy = null;
		setAuthState(response);
	} finally {
		state.isLoading = false;
	}
}

// Initialize auth state on page load - attempts silent refresh via HttpOnly cookie
export async function initAuth(): Promise<void> {
	state = getInitialState();

	// If we have user info from localStorage, attempt silent refresh
	// to restore the session using the HttpOnly cookie
	if (state.user) {
		state.isLoading = true;
		const refreshed = await silentRefresh();
		if (!refreshed) {
			// Refresh failed - clear state
			state.user = null;
			state.currentOrg = null;
			state.isImpersonation = false;
			state.impersonatedBy = null;
			clearPersistedState();
		}
		state.isLoading = false;
	} else {
		// No user info - attempt silent refresh anyway
		// (browser may have valid cookie from previous session)
		state.isLoading = true;
		await silentRefresh();
		state.isLoading = false;
	}
}

// Export reactive getters
export const auth = {
	get user() {
		return state.user;
	},
	get currentOrg() {
		return state.currentOrg;
	},
	get accessToken() {
		return state.accessToken;
	},
	get isAuthenticated() {
		return state.isAuthenticated;
	},
	get isLoading() {
		return state.isLoading;
	},
	get isPlatformAdmin() {
		return state.user?.isPlatformAdmin ?? false;
	},
	get isImpersonation() {
		return state.isImpersonation;
	},
	get memberships() {
		return state.user?.memberships ?? [];
	},
	// Role helpers for permission checks
	get role() {
		return state.currentOrg?.role ?? 'user';
	},
	get isOwner() {
		return state.currentOrg?.role === 'owner';
	},
	get isAdmin() {
		const role = state.currentOrg?.role;
		return role === 'admin' || role === 'owner';
	},
	get canAccessSetup() {
		// Platform admins can always access setup
		// During impersonation, the impersonating user (platform admin) should retain access
		// Org admins/owners can access setup for their org
		return state.user?.isPlatformAdmin || state.isImpersonation || this.isAdmin;
	},
	get mustChangePassword() {
		return state.mustChangePassword;
	}
};

// Export the authFetch for use in other parts of the app
export { authFetch };
