export type ConditionType = 'ISNEW' | 'ISCHANGED' | 'ISDELETED' | 'FIELD_EQUALS' | 'FIELD_CHANGED_TO';

export interface TripwireCondition {
	id: string;
	type: ConditionType;
	fieldName?: string;
	value?: string;
	fromValue?: string;
	toValue?: string;
}

export interface Tripwire {
	id: string;
	orgId: string;
	name: string;
	description?: string;
	entityType: string;
	endpointUrl: string;
	enabled: boolean;
	conditionLogic: string;
	conditions: TripwireCondition[];
	createdAt: string;
	modifiedAt: string;
	createdBy?: string;
	modifiedBy?: string;
}

export interface TripwireCreateInput {
	name: string;
	description?: string;
	entityType: string;
	endpointUrl: string;
	enabled?: boolean;
	conditionLogic?: string;
	conditions: TripwireCondition[];
}

export interface TripwireUpdateInput {
	name?: string;
	description?: string;
	entityType?: string;
	endpointUrl?: string;
	enabled?: boolean;
	conditionLogic?: string;
	conditions?: TripwireCondition[];
}

export interface TripwireListParams {
	search?: string;
	entityType?: string;
	enabled?: boolean;
	sortBy?: string;
	sortDir?: 'asc' | 'desc';
	page?: number;
	pageSize?: number;
}

export interface TripwireListResponse {
	data: Tripwire[];
	total: number;
	page: number;
	pageSize: number;
	totalPages: number;
}

export type WebhookAuthType = 'none' | 'api_key' | 'bearer' | 'custom_header';

export interface OrgWebhookSettings {
	id?: string;
	orgId: string;
	authType: WebhookAuthType;
	apiKey?: string;
	bearerToken?: string;
	customHeaderName?: string;
	customHeaderValue?: string;
	timeoutMs: number;
	createdAt?: string;
	modifiedAt?: string;
}

export interface OrgWebhookSettingsInput {
	authType: WebhookAuthType;
	apiKey?: string;
	bearerToken?: string;
	customHeaderName?: string;
	customHeaderValue?: string;
	timeoutMs?: number;
}

export interface TripwireLog {
	id: string;
	tripwireId: string;
	tripwireName?: string;
	orgId: string;
	recordId: string;
	entityType: string;
	eventType: string;
	status: 'success' | 'failed' | 'timeout';
	responseCode?: number;
	errorMessage?: string;
	durationMs?: number;
	executedAt: string;
}

export interface TripwireLogListParams {
	tripwireId?: string;
	status?: string;
	eventType?: string;
	sortBy?: string;
	sortDir?: 'asc' | 'desc';
	page?: number;
	pageSize?: number;
}

export interface TripwireLogListResponse {
	data: TripwireLog[];
	total: number;
	page: number;
	pageSize: number;
	totalPages: number;
}

export const CONDITION_TYPES: { value: ConditionType; label: string; description: string }[] = [
	{ value: 'ISNEW', label: 'Record Created', description: 'Trigger when a new record is created' },
	{ value: 'ISCHANGED', label: 'Field Changed', description: 'Trigger when a specific field is modified' },
	{ value: 'ISDELETED', label: 'Record Deleted', description: 'Trigger when a record is deleted' },
	{ value: 'FIELD_EQUALS', label: 'Field Equals', description: 'Trigger when a field equals a specific value' },
	{ value: 'FIELD_CHANGED_TO', label: 'Field Changed To', description: 'Trigger when a field changes from one value to another' }
];

export const CONDITION_LOGIC_OPTIONS = [
	{ value: 'AND', label: 'All conditions must match (AND)' },
	{ value: 'OR', label: 'Any condition must match (OR)' }
];
