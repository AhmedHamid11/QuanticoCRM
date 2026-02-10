// Bearing (Stage Progress Indicator) configuration types

export interface BearingConfig {
	id: string;
	orgId: string;
	entityType: string;
	name: string;
	sourcePicklist: string;
	displayOrder: number;
	active: boolean;
	confirmBackward: boolean;
	allowUpdates: boolean;
	createdAt: string;
	modifiedAt: string;
}

export interface PicklistOption {
	value: string;
	label: string;
	order: number;
}

export interface BearingWithStages extends BearingConfig {
	stages: PicklistOption[];
}

export interface BearingConfigCreateInput {
	name: string;
	sourcePicklist: string;
	displayOrder?: number;
	active?: boolean;
	confirmBackward?: boolean;
	allowUpdates?: boolean;
}

export interface BearingConfigUpdateInput {
	name?: string;
	sourcePicklist?: string;
	displayOrder?: number;
	active?: boolean;
	confirmBackward?: boolean;
	allowUpdates?: boolean;
}
