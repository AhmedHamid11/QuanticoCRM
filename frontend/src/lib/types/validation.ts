// Validation operators
export type ValidationOperator =
	| 'EQUALS'
	| 'NOT_EQUALS'
	| 'IN'
	| 'NOT_IN'
	| 'IS_EMPTY'
	| 'IS_NOT_EMPTY'
	| 'GREATER_THAN'
	| 'LESS_THAN'
	| 'GREATER_EQUAL'
	| 'LESS_EQUAL'
	| 'CHANGED'
	| 'CHANGED_TO'
	| 'CHANGED_FROM'
	| 'IS_TRUE'
	| 'IS_FALSE'
	| 'CONTAINS'
	| 'STARTS_WITH'
	| 'ENDS_WITH';

// Action types
export type ValidationActionType =
	| 'BLOCK_SAVE'
	| 'LOCK_FIELDS'
	| 'REQUIRE_VALUE'
	| 'ENFORCE_VALUE'
	| 'SET_VALUE';

export interface ValidationCondition {
	id: string;
	fieldName: string;
	operator: ValidationOperator;
	value?: string | number | boolean;
	values?: string[];
}

export interface ValidationAction {
	type: ValidationActionType;
	fields?: string[];
	fieldName?: string;
	value?: string | number | boolean;
	errorMessage?: string;
}

export interface ValidationRule {
	id: string;
	orgId: string;
	name: string;
	description?: string;
	entityType: string;
	enabled: boolean;
	triggerOnCreate: boolean;
	triggerOnUpdate: boolean;
	triggerOnDelete: boolean;
	conditionLogic: string;
	conditions: ValidationCondition[];
	actions: ValidationAction[];
	errorMessage?: string;
	priority: number;
	createdAt: string;
	modifiedAt: string;
	createdBy?: string;
	modifiedBy?: string;
}

export interface ValidationRuleCreateInput {
	name: string;
	description?: string;
	entityType: string;
	enabled?: boolean;
	triggerOnCreate?: boolean;
	triggerOnUpdate?: boolean;
	triggerOnDelete?: boolean;
	conditionLogic?: string;
	conditions: ValidationCondition[];
	actions: ValidationAction[];
	errorMessage?: string;
	priority?: number;
}

export interface ValidationRuleUpdateInput {
	name?: string;
	description?: string;
	entityType?: string;
	enabled?: boolean;
	triggerOnCreate?: boolean;
	triggerOnUpdate?: boolean;
	triggerOnDelete?: boolean;
	conditionLogic?: string;
	conditions?: ValidationCondition[];
	actions?: ValidationAction[];
	errorMessage?: string;
	priority?: number;
}

export interface ValidationRuleListParams {
	search?: string;
	entityType?: string;
	enabled?: boolean;
	sortBy?: string;
	sortDir?: 'asc' | 'desc';
	page?: number;
	pageSize?: number;
}

export interface ValidationRuleListResponse {
	data: ValidationRule[];
	total: number;
	page: number;
	pageSize: number;
	totalPages: number;
}

export interface FieldValidationError {
	field: string;
	message: string;
	ruleId: string;
}

export interface ValidationResult {
	valid: boolean;
	message?: string;
	fieldErrors?: FieldValidationError[];
}

export interface TestValidationInput {
	rule: ValidationRuleCreateInput;
	operation: 'CREATE' | 'UPDATE' | 'DELETE';
	oldRecord?: Record<string, unknown>;
	newRecord?: Record<string, unknown>;
}

// Operator definitions for the UI
export const VALIDATION_OPERATORS: {
	value: ValidationOperator;
	label: string;
	description: string;
	requiresValue: boolean;
	requiresValues: boolean;
	forUpdateOnly: boolean;
}[] = [
	{
		value: 'EQUALS',
		label: 'Equals',
		description: 'Field value equals the specified value',
		requiresValue: true,
		requiresValues: false,
		forUpdateOnly: false
	},
	{
		value: 'NOT_EQUALS',
		label: 'Not Equals',
		description: 'Field value does not equal the specified value',
		requiresValue: true,
		requiresValues: false,
		forUpdateOnly: false
	},
	{
		value: 'IN',
		label: 'In List',
		description: 'Field value is one of the specified values',
		requiresValue: false,
		requiresValues: true,
		forUpdateOnly: false
	},
	{
		value: 'NOT_IN',
		label: 'Not In List',
		description: 'Field value is not one of the specified values',
		requiresValue: false,
		requiresValues: true,
		forUpdateOnly: false
	},
	{
		value: 'IS_EMPTY',
		label: 'Is Empty',
		description: 'Field has no value',
		requiresValue: false,
		requiresValues: false,
		forUpdateOnly: false
	},
	{
		value: 'IS_NOT_EMPTY',
		label: 'Is Not Empty',
		description: 'Field has a value',
		requiresValue: false,
		requiresValues: false,
		forUpdateOnly: false
	},
	{
		value: 'GREATER_THAN',
		label: 'Greater Than',
		description: 'Field value is greater than the specified number',
		requiresValue: true,
		requiresValues: false,
		forUpdateOnly: false
	},
	{
		value: 'LESS_THAN',
		label: 'Less Than',
		description: 'Field value is less than the specified number',
		requiresValue: true,
		requiresValues: false,
		forUpdateOnly: false
	},
	{
		value: 'GREATER_EQUAL',
		label: 'Greater or Equal',
		description: 'Field value is greater than or equal to the specified number',
		requiresValue: true,
		requiresValues: false,
		forUpdateOnly: false
	},
	{
		value: 'LESS_EQUAL',
		label: 'Less or Equal',
		description: 'Field value is less than or equal to the specified number',
		requiresValue: true,
		requiresValues: false,
		forUpdateOnly: false
	},
	{
		value: 'CHANGED',
		label: 'Changed',
		description: 'Field value was changed (UPDATE only)',
		requiresValue: false,
		requiresValues: false,
		forUpdateOnly: true
	},
	{
		value: 'CHANGED_TO',
		label: 'Changed To',
		description: 'Field value changed to the specified value (UPDATE only)',
		requiresValue: true,
		requiresValues: false,
		forUpdateOnly: true
	},
	{
		value: 'CHANGED_FROM',
		label: 'Changed From',
		description: 'Field value changed from the specified value (UPDATE only)',
		requiresValue: true,
		requiresValues: false,
		forUpdateOnly: true
	},
	{
		value: 'IS_TRUE',
		label: 'Is True',
		description: 'Boolean field is true',
		requiresValue: false,
		requiresValues: false,
		forUpdateOnly: false
	},
	{
		value: 'IS_FALSE',
		label: 'Is False',
		description: 'Boolean field is false',
		requiresValue: false,
		requiresValues: false,
		forUpdateOnly: false
	},
	{
		value: 'CONTAINS',
		label: 'Contains',
		description: 'Field value contains the specified text',
		requiresValue: true,
		requiresValues: false,
		forUpdateOnly: false
	},
	{
		value: 'STARTS_WITH',
		label: 'Starts With',
		description: 'Field value starts with the specified text',
		requiresValue: true,
		requiresValues: false,
		forUpdateOnly: false
	},
	{
		value: 'ENDS_WITH',
		label: 'Ends With',
		description: 'Field value ends with the specified text',
		requiresValue: true,
		requiresValues: false,
		forUpdateOnly: false
	}
];

// Action type definitions for the UI
export const VALIDATION_ACTION_TYPES: {
	value: ValidationActionType;
	label: string;
	description: string;
	requiresFields: boolean;
	requiresFieldName: boolean;
	requiresValue: boolean;
}[] = [
	{
		value: 'BLOCK_SAVE',
		label: 'Block Save',
		description: 'Prevent the save operation entirely',
		requiresFields: false,
		requiresFieldName: false,
		requiresValue: false
	},
	{
		value: 'LOCK_FIELDS',
		label: 'Lock Fields',
		description: 'Prevent modification of specific fields',
		requiresFields: true,
		requiresFieldName: false,
		requiresValue: false
	},
	{
		value: 'REQUIRE_VALUE',
		label: 'Require Value',
		description: 'Ensure specific fields have non-empty values',
		requiresFields: true,
		requiresFieldName: false,
		requiresValue: false
	},
	{
		value: 'ENFORCE_VALUE',
		label: 'Enforce Value',
		description: 'Require a specific field to have a specific value',
		requiresFields: false,
		requiresFieldName: true,
		requiresValue: true
	}
];

// Condition logic options
export const CONDITION_LOGIC_OPTIONS = [
	{ value: 'AND', label: 'All conditions must match (AND)' },
	{ value: 'OR', label: 'Any condition must match (OR)' }
];
