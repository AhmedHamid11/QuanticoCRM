// Related List configuration types

export interface FieldConfig {
	field: string;
	label?: string;
	width?: number;
	position: number;
}

export interface RelatedListConfig {
	id: string;
	orgId: string;
	entityType: string;
	relatedEntity: string;
	lookupField: string;
	label: string;
	enabled: boolean;
	editInList?: boolean;
	displayFields: FieldConfig[];
	sortOrder: number;
	defaultSort: string;
	defaultSortDir: 'asc' | 'desc';
	pageSize: number;
	createdAt: string;
	modifiedAt: string;
}

export interface PossibleRelatedList {
	relatedEntity: string;
	lookupField: string;
	suggestedLabel: string;
	fieldLabel: string;
}

export interface RelatedListConfigCreateInput {
	relatedEntity: string;
	lookupField: string;
	label: string;
	enabled: boolean;
	editInList?: boolean;
	displayFields: FieldConfig[];
	sortOrder?: number;
	defaultSort?: string;
	defaultSortDir?: 'asc' | 'desc';
	pageSize?: number;
}

export interface RelatedListConfigBulkInput {
	configs: RelatedListConfigCreateInput[];
}

export interface RelatedRecordsResponse {
	records: Record<string, unknown>[];
	total: number;
	page: number;
	pageSize: number;
	totalPages: number;
}
