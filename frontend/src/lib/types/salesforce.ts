export interface SalesforceConfig {
	clientId: string;
	redirectUrl: string;
	instanceUrl: string;
	isEnabled: boolean;
	status: 'connected' | 'configured' | 'not_configured' | 'expired';
	connectedAt: string | null;
}

export interface SalesforceConfigInput {
	clientId: string;
	clientSecret: string;
	redirectUrl: string;
}

export interface SyncJob {
	id: string;
	orgId: string;
	batchId: string;
	entityType: string;
	status: 'pending' | 'running' | 'completed' | 'failed';
	totalInstructions: number;
	deliveredInstructions: number;
	failedInstructions: number;
	errorMessage: string | null;
	retryCount: number;
	triggerType: string;
	startedAt: string | null;
	completedAt: string | null;
	createdAt: string;
}

export interface SyncJobListResponse {
	jobs: SyncJob[];
	total: number;
}
