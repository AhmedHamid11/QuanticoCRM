export type FieldType =
	| 'varchar'
	| 'text'
	| 'int'
	| 'float'
	| 'bool'
	| 'date'
	| 'datetime'
	| 'email'
	| 'phone'
	| 'url'
	| 'enum'
	| 'multiEnum'
	| 'link'
	| 'linkMultiple'
	| 'currency'
	| 'address'
	| 'rollup'
	| 'textBlock'
	| 'stream';

export type TextBlockVariant = 'info' | 'warning' | 'error' | 'success';

export interface FieldParam {
	name: string;
	type: string;
	label: string;
	default?: unknown;
	required?: boolean;
	min?: number;
	max?: number;
	options?: string[];
}

export interface FieldTypeInfo {
	name: FieldType;
	label: string;
	description: string;
	params: FieldParam[];
}

export interface EntityDef {
	id: string;
	name: string;
	label: string;
	labelPlural: string;
	icon: string;
	color: string;
	isCustom: boolean;
	isCustomizable: boolean;
	hasStream: boolean;
	hasActivities: boolean;
	displayField: string;
	searchFields: string;
	createdAt: string;
	modifiedAt: string;
}

export interface FieldDef {
	id: string;
	entityName: string;
	name: string;
	label: string;
	type: FieldType;
	isRequired: boolean;
	isReadOnly: boolean;
	isAudited: boolean;
	isCustom: boolean;
	defaultValue?: string | null;
	options?: string | null;
	maxLength?: number | null;
	minValue?: number | null;
	maxValue?: number | null;
	pattern?: string | null;
	tooltip?: string | null;
	linkEntity?: string | null;
	rollupQuery?: string | null;
	rollupResultType?: 'numeric' | 'text' | null;
	rollupDecimalPlaces?: number | null;
	defaultToToday?: boolean;
	variant?: TextBlockVariant | null;
	content?: string | null;
	sortOrder: number;
	createdAt: string;
	modifiedAt: string;
}

export interface FieldDefCreateInput {
	name: string;
	label: string;
	type: FieldType;
	isRequired?: boolean;
	isReadOnly?: boolean;
	isAudited?: boolean;
	defaultValue?: string;
	options?: string;
	maxLength?: number;
	minValue?: number;
	maxValue?: number;
	pattern?: string;
	tooltip?: string;
	linkEntity?: string;
	rollupQuery?: string;
	rollupResultType?: 'numeric' | 'text';
	rollupDecimalPlaces?: number;
	defaultToToday?: boolean;
	variant?: TextBlockVariant;
	content?: string;
}

export interface FieldDefUpdateInput {
	label?: string;
	isRequired?: boolean;
	isReadOnly?: boolean;
	isAudited?: boolean;
	defaultValue?: string;
	options?: string;
	maxLength?: number;
	minValue?: number;
	maxValue?: number;
	pattern?: string;
	tooltip?: string;
	sortOrder?: number;
	defaultToToday?: boolean;
	variant?: TextBlockVariant;
	content?: string;
}

export interface EntityDefCreateInput {
	name: string;
	label: string;
	labelPlural?: string;
	icon?: string;
	color?: string;
	hasStream?: boolean;
	hasActivities?: boolean;
}

export interface EntityDefUpdateInput {
	label?: string;
	labelPlural?: string;
	icon?: string;
	color?: string;
	hasStream?: boolean;
	hasActivities?: boolean;
}
