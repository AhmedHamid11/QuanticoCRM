import { get, post, put, del } from '$lib/utils/api';

// Re-export common types and utilities from dedup.ts
export type {
	MatchResult,
	DuplicateMatch,
	PendingAlert
} from './dedup';

export {
	getBannerClass,
	getConfidenceBadgeClass,
	formatConfidence
} from './dedup';

// ===== Matching Rules =====

export interface DedupFieldConfig {
	fieldName: string;
	matchType: 'exact' | 'fuzzy' | 'phonetic' | 'email' | 'phone';
	weight: number;
	isBlockingField: boolean;
}

export interface MatchingRule {
	id: string;
	orgId: string;
	name: string;
	description: string;
	entityType: string;
	targetEntityType: string;  // For cross-entity matching (e.g., Contact→Lead)
	isEnabled: boolean;
	priority: number;  // Lower number = higher priority
	threshold: number;  // Min score for match (0-1)
	highConfidenceThreshold: number;  // Score for high confidence (0.95+)
	mediumConfidenceThreshold: number;  // Score for medium confidence (0.85+)
	blockingStrategy: 'first_letter' | 'domain' | 'area_code' | 'soundex' | 'none';
	fieldConfigs: DedupFieldConfig[];
	mergeDisplayFields: string[];  // Fields to show on merge wizard
	createdAt: string;
	updatedAt: string;
}

export interface MatchingRuleCreateInput {
	name: string;
	description: string;
	entityType: string;
	targetEntityType?: string;
	isEnabled?: boolean;
	priority?: number;
	threshold: number;
	highConfidenceThreshold?: number;
	mediumConfidenceThreshold?: number;
	blockingStrategy?: string;
	fieldConfigs: DedupFieldConfig[];
	mergeDisplayFields?: string[];
}

export interface MatchingRuleUpdateInput {
	name?: string;
	description?: string;
	isEnabled?: boolean;
	priority?: number;
	threshold?: number;
	highConfidenceThreshold?: number;
	mediumConfidenceThreshold?: number;
	blockingStrategy?: string;
	fieldConfigs?: DedupFieldConfig[];
	mergeDisplayFields?: string[];
}

// ===== Pending Alerts (Review Queue) =====

export interface PaginatedResponse<T> {
	data: T[];
	total: number;
	page: number;
	pageSize: number;
}

// ===== Merge Operations =====

export interface FieldDef {
	id: string;
	orgId: string;
	entityName: string;
	name: string;
	label: string;
	type: string;
	isRequired: boolean;
	isReadOnly: boolean;
	isAudited: boolean;
	isCustom: boolean;
}

export interface RelatedRecordCount {
	entityType: string;
	entityLabel: string;
	recordId: string;  // which record these belong to
	count: number;
	records?: Record<string, any>[];  // actual related records (for expandable preview)
}

export interface MergePreviewRequest {
	recordIds: string[];  // backend expects recordIds (lowercase r)
	entityType: string;
}

export interface MergePreview {
	records: Record<string, any>[];  // Full record data for all records
	suggestedSurvivorId: string;  // System recommendation based on completeness
	completenessScores: Record<string, number>;  // recordID -> 0.0-1.0
	relatedRecordCounts: RelatedRecordCount[];  // What will be transferred
	fields: FieldDef[];  // Field definitions for display
}

export interface MergeRequest {
	survivorId: string;
	duplicateIds: string[];
	mergedFields: Record<string, any>;  // backend expects mergedFields
	entityType: string;
}

export interface MergeResult {
	survivorId: string;
	snapshotId: string;  // For undo within 30 days
	mergedAt: string;
}

export interface MergeHistoryEntry {
	snapshotId: string;
	entityType: string;
	survivorId: string;
	duplicateIds: string[];
	mergedById: string;  // User ID
	canUndo: boolean;
	createdAt: string;
	expiresAt: string;  // 30 days from merge
}

// ===== Scan Jobs =====

export interface ScanSchedule {
	id: string;
	orgId: string;
	entityType: string;
	cronExpression: string;  // "0 2 * * *" (daily at 2am), etc.
	isEnabled: boolean;
	lastRunAt?: string;
	nextRunAt?: string;
	createdAt: string;
	updatedAt: string;
}

export interface ScheduleInput {
	cronExpression: string;
	isEnabled?: boolean;
}

export interface ScanCheckpoint {
	jobId: string;
	lastProcessedId: string;
	processedCount: number;
	totalCount: number;
	detectedCount: number;
	createdAt: string;
}

export interface ScanJob {
	id: string;
	orgId: string;
	entityType: string;
	status: 'queued' | 'running' | 'completed' | 'failed';
	totalRecords: number;
	processedRecords: number;
	alertsCreated: number;
	startedAt?: string;
	completedAt?: string;
	errorMessage?: string;
	checkpoint?: ScanCheckpoint;
}

export interface ProgressEvent {
	jobId: string;
	percentage: number;
	status: string;
}

// ===== API Functions =====

// --- Rules ---

export async function listRules(entityType?: string): Promise<{ data: MatchingRule[] }> {
	const query = entityType ? `?entityType=${encodeURIComponent(entityType)}` : '';
	return get<{ data: MatchingRule[] }>(`/dedup/rules${query}`);
}

export async function getRule(id: string): Promise<MatchingRule> {
	return get<MatchingRule>(`/dedup/rules/${id}`);
}

export async function createRule(input: MatchingRuleCreateInput): Promise<MatchingRule> {
	return post<MatchingRule>('/dedup/rules', input);
}

export async function updateRule(id: string, input: MatchingRuleUpdateInput): Promise<MatchingRule> {
	return put<MatchingRule>(`/dedup/rules/${id}`, input);
}

export async function deleteRule(id: string): Promise<void> {
	return del<void>(`/dedup/rules/${id}`);
}

export async function checkDuplicates(
	entityType: string,
	recordData: Record<string, unknown>
): Promise<{ duplicates: DuplicateMatch[]; count: number }> {
	return post<{ duplicates: DuplicateMatch[]; count: number }>(
		`/dedup/${entityType}/check`,
		recordData
	);
}

// --- Pending Alerts (Review Queue) ---

export async function listPendingAlerts(params?: {
	entityType?: string;
	page?: number;
	pageSize?: number;
}): Promise<PaginatedResponse<PendingAlert>> {
	const queryParams = new URLSearchParams();
	if (params?.entityType) queryParams.set('entityType', params.entityType);
	if (params?.page) queryParams.set('page', params.page.toString());
	if (params?.pageSize) queryParams.set('pageSize', params.pageSize.toString());

	const query = queryParams.toString() ? `?${queryParams.toString()}` : '';
	return get<PaginatedResponse<PendingAlert>>(`/dedup/pending-alerts${query}`);
}

// --- Merge ---

export async function mergePreview(req: MergePreviewRequest): Promise<MergePreview> {
	return post<MergePreview>('/merge/preview', req);
}

export async function mergeExecute(req: MergeRequest): Promise<MergeResult> {
	return post<MergeResult>('/merge/execute', req);
}

export async function mergeUndo(snapshotId: string): Promise<void> {
	return post<void>(`/merge/undo/${snapshotId}`, {});
}

export async function mergeHistory(params?: {
	entityType?: string;
	page?: number;
	pageSize?: number;
}): Promise<PaginatedResponse<MergeHistoryEntry>> {
	const queryParams = new URLSearchParams();
	if (params?.entityType) queryParams.set('entityType', params.entityType);
	if (params?.page) queryParams.set('page', params.page.toString());
	if (params?.pageSize) queryParams.set('pageSize', params.pageSize.toString());

	const query = queryParams.toString() ? `?${queryParams.toString()}` : '';
	return get<PaginatedResponse<MergeHistoryEntry>>(`/merge/history${query}`);
}

// --- Scan Jobs ---

export async function listSchedules(): Promise<ScanSchedule[]> {
	const response = await get<{ data: ScanSchedule[] }>('/scan-jobs/schedules');
	return response.data;
}

export async function upsertSchedule(
	entityType: string,
	input: ScheduleInput
): Promise<ScanSchedule> {
	return put<ScanSchedule>(`/scan-jobs/schedules/${entityType}`, input);
}

export async function deleteSchedule(entityType: string): Promise<void> {
	return del<void>(`/scan-jobs/schedules/${entityType}`);
}

export async function listJobs(params?: {
	entityType?: string;
	page?: number;
	pageSize?: number;
}): Promise<PaginatedResponse<ScanJob>> {
	const queryParams = new URLSearchParams();
	if (params?.entityType) queryParams.set('entityType', params.entityType);
	if (params?.page) queryParams.set('page', params.page.toString());
	if (params?.pageSize) queryParams.set('pageSize', params.pageSize.toString());

	const query = queryParams.toString() ? `?${queryParams.toString()}` : '';
	return get<PaginatedResponse<ScanJob>>(`/scan-jobs${query}`);
}

export async function triggerManualScan(
	entityType: string
): Promise<{ jobId: string; status: string }> {
	return post<{ jobId: string; status: string }>(`/scan-jobs/${entityType}/trigger`, {});
}

export async function retryJob(jobId: string): Promise<{ jobId: string; status: string }> {
	return post<{ jobId: string; status: string }>(`/scan-jobs/${jobId}/retry`, {});
}
