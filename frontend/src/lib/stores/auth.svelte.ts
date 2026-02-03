// Auth store for Quantico CRM using Svelte 5 runes
import { PUBLIC_API_URL } from '$env/static/public';
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

// Initial state
function getInitialState(): AuthState {
	if (typeof window === 'undefined') {
		return {
			user: null,
			currentOrg: null,
			accessToken: null,
			refreshToken: null,
			expiresAt: null,
			isAuthenticated: false,
			isLoading: true,
			isImpersonation: false,
			impersonatedBy: null
		};
	}

	// Try to restore from localStorage
	try {
		const stored = localStorage.getItem(STORAGE_KEY);
		if (stored) {
			const data = JSON.parse(stored);
			return {
				...data,
				expiresAt: data.expiresAt ? new Date(data.expiresAt) : null,
				isLoading: false,
				isAuthenticated: !!data.accessToken
			};
		}
	} catch (e) {
		console.error('Failed to restore auth state:', e);
	}

	return {
		user: null,
		currentOrg: null,
		accessToken: null,
		refreshToken: null,
		expiresAt: null,
		isAuthenticated: false,
		isLoading: false,
		isImpersonation: false,
		impersonatedBy: null
	};
}

// Create reactive state
let state = $state<AuthState>(getInitialState());

// Persist to localStorage
function persistState() {
	if (typeof window === 'undefined') return;

	try {
		const toStore = {
			user: state.user,
			currentOrg: state.currentOrg,
			accessToken: state.accessToken,
			refreshToken: state.refreshToken,
			expiresAt: state.expiresAt?.toISOString() ?? null,
			isImpersonation: state.isImpersonation,
			impersonatedBy: state.impersonatedBy
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

// API helper with auth
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

	const config: RequestInit = {
		method,
		headers
	};

	if (body) {
		config.body = JSON.stringify(body);
	}

	const API_BASE = PUBLIC_API_URL || '/api/v1';
	const response = await fetch(`${API_BASE}${endpoint}`, config);

	if (!response.ok) {
		// Handle 401 - try to refresh token
		if (response.status === 401 && requiresAuth && state.refreshToken) {
			const refreshed = await refreshTokens();
			if (refreshed) {
				// Retry the request with new token
				headers['Authorization'] = `Bearer ${state.accessToken}`;
				const retryResponse = await fetch(`${API_BASE}${endpoint}`, {
					...config,
					headers
				});
				if (retryResponse.ok) {
					if (retryResponse.status === 204) return undefined as T;
					return retryResponse.json();
				}
			}
			// Refresh failed, logout and redirect to login
			await logoutWithSessionExpired();
			throw new Error('Session expired');
		}

		const error = await response.json().catch(() => ({ error: 'Unknown error' }));
		throw new Error(error.error || `HTTP ${response.status}`);
	}

	if (response.status === 204) {
		return undefined as T;
	}

	return response.json();
}

// Set auth state from response
function setAuthState(response: AuthResponse) {
	const defaultOrg = response.user.memberships.find((m) => m.isDefault) || response.user.memberships[0];

	state.user = response.user;
	state.currentOrg = defaultOrg || null;
	state.accessToken = response.accessToken;
	state.refreshToken = response.refreshToken;
	state.expiresAt = new Date(response.expiresAt);
	state.isAuthenticated = true;
	state.isLoading = false;

	persistState();
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

export async function logout(): Promise<void> {
	try {
		if (state.refreshToken) {
			await authFetch('/auth/logout', {
				method: 'POST',
				body: { refreshToken: state.refreshToken }
			}).catch(() => {});
		}
	} finally {
		state.user = null;
		state.currentOrg = null;
		state.accessToken = null;
		state.refreshToken = null;
		state.expiresAt = null;
		state.isAuthenticated = false;
		state.isImpersonation = false;
		state.impersonatedBy = null;
		clearPersistedState();
	}
}

// Logout and redirect to login with session expired message
export async function logoutWithSessionExpired(): Promise<void> {
	state.user = null;
	state.currentOrg = null;
	state.accessToken = null;
	state.refreshToken = null;
	state.expiresAt = null;
	state.isAuthenticated = false;
	state.isImpersonation = false;
	state.impersonatedBy = null;
	clearPersistedState();

	// Redirect to login with session expired flag
	if (typeof window !== 'undefined') {
		window.location.href = '/login?session_expired=true';
	}
}

export async function refreshTokens(): Promise<boolean> {
	if (!state.refreshToken) return false;

	try {
		const response = await authFetch<AuthResponse>('/auth/refresh', {
			method: 'POST',
			body: { refreshToken: state.refreshToken },
			requiresAuth: false
		});
		setAuthState(response);
		return true;
	} catch (e) {
		console.error('Failed to refresh token:', e);
		return false;
	}
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
	await authFetch('/auth/change-password', {
		method: 'POST',
		body: input
	});
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

// Initialize auth state on page load
export function initAuth(): void {
	state = getInitialState();
	state.isLoading = false;

	// Check if token is expired and try to refresh
	if (state.accessToken && state.expiresAt) {
		const now = new Date();
		const expiresIn = state.expiresAt.getTime() - now.getTime();

		// If token expires in less than 5 minutes, refresh it
		if (expiresIn < 5 * 60 * 1000) {
			refreshTokens();
		}
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
		// Platform admins and org admins/owners can access setup
		return state.user?.isPlatformAdmin || this.isAdmin;
	}
};

// Export the authFetch for use in other parts of the app
export { authFetch };
