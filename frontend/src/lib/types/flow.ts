// =============================================================================
// Flow Definition Types
// =============================================================================

export type TriggerType = 'manual' | 'record_create' | 'record_update' | 'scheduled';
export type StepType = 'screen' | 'decision' | 'assignment' | 'record_create' | 'record_update' | 'record_get' | 'n8n_webhook' | 'end';
export type FieldType = 'text' | 'textarea' | 'number' | 'currency' | 'percent' | 'date' | 'datetime' | 'select' | 'radio' | 'checkbox' | 'lookup' | 'email' | 'phone' | 'url' | 'display';
export type ExecutionStatus = 'running' | 'paused_at_screen' | 'completed' | 'failed';

export interface FlowTrigger {
	type: TriggerType;
	entityType?: string;
	buttonLabel?: string;
	showOn?: string[];
}

export interface FlowVariable {
	type: 'string' | 'number' | 'boolean' | 'date' | 'record' | 'list';
	default?: unknown;
}

export interface FlowDefinition {
	id: string;
	name: string;
	description?: string;
	version: number;
	trigger: FlowTrigger;
	variables?: Record<string, FlowVariable>;
	refreshOnComplete?: boolean; // Refresh page when flow completes
	steps: unknown[]; // Raw step data
}

// =============================================================================
// Screen Step Types
// =============================================================================

export interface ScreenHeader {
	type: string;
	variant: 'success' | 'warning' | 'error' | 'info';
	message: string;
}

export interface FieldOption {
	value: string;
	label: string;
}

export interface ScreenField {
	name: string;
	label: string;
	type: FieldType;
	required?: boolean;
	defaultValue?: string;
	value?: string; // For display fields - template like {{variableName}}
	placeholder?: string;
	helpText?: string;
	options?: FieldOption[];
	entity?: string;
	filter?: unknown;
	minValue?: number;
	maxValue?: number;
	minDate?: string;
	maxDate?: string;
	rows?: number;
	variant?: 'info' | 'warning' | 'error' | 'success'; // For display fields - styled message box
}

export interface ScreenStep {
	id: string;
	type: 'screen';
	name: string;
	next?: string;
	header?: ScreenHeader;
	fields: ScreenField[];
}

// =============================================================================
// End Step Types
// =============================================================================

export interface EndRedirect {
	entity: string;
	recordId: string;
}

export interface EndStep {
	id: string;
	type: 'end';
	name: string;
	message?: string;
	redirect?: EndRedirect;
}

// =============================================================================
// Flow Execution Types
// =============================================================================

export interface FlowExecution {
	id: string;
	flowId: string;
	flowName: string;
	flowVersion: number;
	orgId: string;
	userId: string;
	status: ExecutionStatus;
	currentStep: string;
	variables: Record<string, unknown>;
	record?: Record<string, unknown>;
	screenData: Record<string, unknown>;
	screenDef?: ScreenStep;
	endMessage?: string;
	redirect?: EndRedirect;
	error?: string;
	startedAt: string;
	completedAt?: string;
}

// =============================================================================
// API Request/Response Types
// =============================================================================

export interface StartFlowRequest {
	recordId?: string;
	entity?: string;
}

export interface SubmitScreenRequest {
	[key: string]: unknown;
}

export interface FlowListItem {
	id: string;
	orgId: string;
	name: string;
	description?: string;
	version: number;
	isActive: boolean;
	createdAt: string;
	modifiedAt: string;
}

export interface FlowListResponse {
	data: FlowListItem[];
	total: number;
	page: number;
	pageSize: number;
	totalPages: number;
}

// =============================================================================
// UI Helper Types
// =============================================================================

export interface FlowButton {
	flowId: string;
	label: string;
	entityType: string;
}

// =============================================================================
// Flow Builder Types
// =============================================================================

export interface DecisionOutcome {
	id: string;
	label: string;
	condition: string;
	next?: string;
}

export interface DecisionStep {
	id: string;
	type: 'decision';
	name: string;
	outcomes: DecisionOutcome[];
	defaultNext?: string;
}

export interface AssignmentOperation {
	variable: string;
	operation: 'set' | 'add' | 'subtract';
	value: string;
}

export interface AssignmentStep {
	id: string;
	type: 'assignment';
	name: string;
	assignments: AssignmentOperation[];
	next?: string;
}

export interface RecordCreateStep {
	id: string;
	type: 'record_create';
	name: string;
	entity: string;
	fieldMappings: Record<string, string>;
	storeResultAs?: string;
	next?: string;
}

export interface RecordUpdateStep {
	id: string;
	type: 'record_update';
	name: string;
	entity: string;
	recordId: string;
	fieldMappings: Record<string, string>;
	next?: string;
}

export interface RecordGetStep {
	id: string;
	type: 'record_get';
	name: string;
	entity: string;
	recordId: string;
	storeResultAs: string;
	next?: string;
}

export interface WebhookStep {
	id: string;
	type: 'n8n_webhook';
	name: string;
	url: string;
	method?: string;
	payload?: Record<string, string>;
	waitForResponse?: boolean;
	storeResultAs?: string;
	next?: string;
}

export type FlowStep = ScreenStep | DecisionStep | AssignmentStep | RecordCreateStep | RecordUpdateStep | RecordGetStep | WebhookStep | EndStep;

export interface FlowBuilderDefinition {
	trigger: FlowTrigger;
	variables?: Record<string, FlowVariable>;
	steps: FlowStep[];
}

export interface FlowCreateInput {
	name: string;
	description?: string;
	isActive?: boolean;
	definition: FlowBuilderDefinition;
}

export interface FlowUpdateInput {
	name?: string;
	description?: string;
	isActive?: boolean;
	definition?: FlowBuilderDefinition;
}

export const STEP_TYPES = [
	{ value: 'screen', label: 'Screen', icon: 'M4 5a1 1 0 011-1h14a1 1 0 011 1v2a1 1 0 01-1 1H5a1 1 0 01-1-1V5zM4 13a1 1 0 011-1h6a1 1 0 011 1v6a1 1 0 01-1 1H5a1 1 0 01-1-1v-6zM16 13a1 1 0 011-1h2a1 1 0 011 1v6a1 1 0 01-1 1h-2a1 1 0 01-1-1v-6z', description: 'Display a form to collect user input' },
	{ value: 'decision', label: 'Decision', icon: 'M9 3v2m6-2v2M9 19v2m6-2v2M5 9H3m2 6H3m18-6h-2m2 6h-2M7 19h10a2 2 0 002-2V7a2 2 0 00-2-2H7a2 2 0 00-2 2v10a2 2 0 002 2zM9 9h6v6H9V9z', description: 'Branch based on conditions' },
	{ value: 'assignment', label: 'Assignment', icon: 'M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z', description: 'Set or update variable values' },
	{ value: 'record_create', label: 'Create Record', icon: 'M9 13h6m-3-3v6m5 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z', description: 'Create a new record in the database' },
	{ value: 'record_update', label: 'Update Record', icon: 'M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z', description: 'Update an existing record' },
	{ value: 'record_get', label: 'Get Record', icon: 'M7 21h10a2 2 0 002-2V9.414a1 1 0 00-.293-.707l-5.414-5.414A1 1 0 0012.586 3H7a2 2 0 00-2 2v14a2 2 0 002 2z', description: 'Fetch a record from the database' },
	{ value: 'n8n_webhook', label: 'Webhook', icon: 'M13.828 10.172a4 4 0 00-5.656 0l-4 4a4 4 0 105.656 5.656l1.102-1.101m-.758-4.899a4 4 0 005.656 0l4-4a4 4 0 00-5.656-5.656l-1.1 1.1', description: 'Call an external webhook (n8n)' },
	{ value: 'end', label: 'End', icon: 'M21 12a9 9 0 11-18 0 9 9 0 0118 0z M9 10a1 1 0 011-1h4a1 1 0 011 1v4a1 1 0 01-1 1h-4a1 1 0 01-1-1v-4z', description: 'End the flow with a message' }
] as const;

export const FIELD_TYPES = [
	{ value: 'text', label: 'Text' },
	{ value: 'textarea', label: 'Text Area' },
	{ value: 'number', label: 'Number' },
	{ value: 'currency', label: 'Currency' },
	{ value: 'percent', label: 'Percent' },
	{ value: 'date', label: 'Date' },
	{ value: 'datetime', label: 'Date/Time' },
	{ value: 'select', label: 'Dropdown' },
	{ value: 'radio', label: 'Radio Buttons' },
	{ value: 'checkbox', label: 'Checkbox' },
	{ value: 'email', label: 'Email' },
	{ value: 'phone', label: 'Phone' },
	{ value: 'url', label: 'URL' },
	{ value: 'lookup', label: 'Lookup' }
] as const;
