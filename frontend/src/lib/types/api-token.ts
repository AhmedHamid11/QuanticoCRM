// API Token types

export interface APITokenListItem {
	id: string;
	name: string;
	tokenPrefix: string;
	scopes: string[];
	lastUsedAt: string | null;
	expiresAt: string | null;
	isActive: boolean;
	createdAt: string;
	createdBy: string;
}

export interface APITokenCreateInput {
	name: string;
	scopes?: string[];
	expiresIn?: number | null; // Days until expiration, null = never
}

export interface APITokenCreateResponse {
	token: string; // Full token - only shown once!
	id: string;
	name: string;
	scopes: string[];
	expiresAt: string | null;
	createdAt: string;
}

export interface APITokenListResponse {
	tokens: APITokenListItem[];
}

// Valid scopes
export const TOKEN_SCOPES = {
	READ: 'read',
	WRITE: 'write'
} as const;

export type TokenScope = (typeof TOKEN_SCOPES)[keyof typeof TOKEN_SCOPES];
