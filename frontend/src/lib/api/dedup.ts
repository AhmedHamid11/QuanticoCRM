import { get, post } from '$lib/utils/api';

// Types for duplicate detection
export interface MatchResult {
	score: number;
	confidenceTier: 'high' | 'medium' | 'low';
	fieldScores: Record<string, number>;
	matchingFields: string[];
	ruleId: string;
	ruleName: string;
}

export interface DuplicateMatch {
	recordId: string;
	recordName?: string;
	matchResult: MatchResult;
}

export interface PendingAlert {
	id: string;
	orgId: string;
	entityType: string;
	recordId: string;
	matches: DuplicateMatch[];
	totalMatchCount: number;
	highestConfidence: 'high' | 'medium' | 'low';
	isBlockMode: boolean;  // From matching rule configuration
	status: 'pending' | 'dismissed' | 'created_anyway' | 'merged';
	detectedAt: string;
	resolvedAt?: string;
	resolvedById?: string;
	overrideText?: string;
}

export type AlertResolution = 'dismissed' | 'created_anyway' | 'merged';

/**
 * Get pending duplicate alert for a specific record
 * Returns null if no pending alert exists
 */
export async function getPendingAlert(entityType: string, recordId: string): Promise<PendingAlert | null> {
	try {
		return await get<PendingAlert>(`/dedup/${entityType}/${recordId}/pending-alert`);
	} catch (error: any) {
		// 404 means no pending alert - this is normal
		if (error?.status === 404 || error?.message?.includes('404')) {
			return null;
		}
		throw error;
	}
}

/**
 * Resolve a pending duplicate alert
 */
export async function resolveAlert(
	entityType: string,
	recordId: string,
	status: AlertResolution,
	overrideText?: string
): Promise<void> {
	await post(`/dedup/${entityType}/${recordId}/resolve-alert`, {
		status,
		overrideText: overrideText || ''
	});
}

/**
 * Format confidence score as percentage
 */
export function formatConfidence(score: number): string {
	return `${Math.round(score * 100)}%`;
}

/**
 * Get CSS classes for confidence tier badge
 */
export function getConfidenceBadgeClass(tier: 'high' | 'medium' | 'low'): string {
	switch (tier) {
		case 'high':
			return 'bg-red-100 text-red-800 border-red-200';
		case 'medium':
			return 'bg-yellow-100 text-yellow-800 border-yellow-200';
		case 'low':
			return 'bg-blue-100 text-blue-800 border-blue-200';
		default:
			return 'bg-gray-100 text-gray-800 border-gray-200';
	}
}

/**
 * Get CSS classes for banner based on confidence
 */
export function getBannerClass(confidence: 'high' | 'medium' | 'low'): string {
	switch (confidence) {
		case 'high':
			return 'bg-red-50 border-red-200 text-red-800';
		case 'medium':
			return 'bg-yellow-50 border-yellow-200 text-yellow-800';
		case 'low':
			return 'bg-blue-50 border-blue-200 text-blue-800';
		default:
			return 'bg-gray-50 border-gray-200 text-gray-800';
	}
}
