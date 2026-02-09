// Auth types for Quantico CRM

export interface User {
	id: string;
	email: string;
	firstName: string;
	lastName: string;
	isActive: boolean;
	isPlatformAdmin: boolean;
	emailVerified: boolean;
	lastLoginAt: string | null;
	createdAt: string;
	modifiedAt: string;
}

export interface Organization {
	id: string;
	name: string;
	slug: string;
	plan: string;
	isActive: boolean;
	createdAt: string;
	modifiedAt: string;
}

export interface Membership {
	id: string;
	userId: string;
	orgId: string;
	role: 'owner' | 'admin' | 'user';
	isDefault: boolean;
	joinedAt: string;
	orgName: string;
	orgSlug: string;
}

export interface UserWithOrgs {
	id: string;
	email: string;
	firstName: string;
	lastName: string;
	isActive: boolean;
	isPlatformAdmin: boolean;
	emailVerified: boolean;
	memberships: Membership[];
}

export interface CurrentUser {
	id: string;
	email: string;
	firstName: string;
	lastName: string;
	orgId: string;
	orgName: string;
	orgSlug: string;
	role: 'owner' | 'admin' | 'user';
	isPlatformAdmin: boolean;
	isImpersonation: boolean;
	impersonatedBy?: string;
}

export interface AuthResponse {
	accessToken: string;
	refreshToken: string;
	expiresAt: string;
	user: UserWithOrgs;
}

export interface RegisterInput {
	email: string;
	password: string;
	firstName?: string;
	lastName?: string;
	orgName: string;
}

export interface LoginInput {
	email: string;
	password: string;
}

export interface RefreshInput {
	refreshToken: string;
}

export interface SwitchOrgInput {
	orgId: string;
}

export interface ImpersonateInput {
	orgId: string;
	userId?: string;
}

export interface InvitationInput {
	email: string;
	role?: 'owner' | 'admin' | 'user';
}

// UserWithMembership represents a user with their membership in a specific org
export interface UserWithMembership {
	id: string;
	email: string;
	firstName: string;
	lastName: string;
	isActive: boolean;
	isPlatformAdmin: boolean;
	emailVerified: boolean;
	lastLoginAt: string | null;
	createdAt: string;
	modifiedAt: string;
	role: 'owner' | 'admin' | 'user';
	joinedAt: string;
}

// UserListResponse represents the paginated user list
export interface UserListResponse {
	data: UserWithMembership[];
	total: number;
	page: number;
	pageSize: number;
	totalPages: number;
}

export interface AcceptInvitationInput {
	token: string;
	password: string;
	firstName?: string;
	lastName?: string;
}

export interface ChangePasswordInput {
	currentPassword: string;
	newPassword: string;
}

export interface AuthState {
	user: UserWithOrgs | null;
	currentOrg: Membership | null;
	accessToken: string | null;
	refreshToken: string | null;
	expiresAt: Date | null;
	isAuthenticated: boolean;
	isLoading: boolean;
	isImpersonation: boolean;
	impersonatedBy: string | null;
	mustChangePassword: boolean;
}
