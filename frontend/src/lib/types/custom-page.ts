// Component types supported on custom pages
export type ComponentType = 'iframe' | 'text' | 'markdown' | 'html' | 'entity_list' | 'link_group' | 'stats';

// Component width in the grid layout
export type ComponentWidth = 'full' | '1/2' | '1/3' | '2/3';

// Base component interface
export interface PageComponent {
	id: string;
	type: ComponentType;
	title?: string;
	width: ComponentWidth;
	order: number;
	config: IframeConfig | TextConfig | HTMLConfig | EntityListConfig | LinkGroupConfig | StatsConfig;
}

// Component-specific configurations
export interface IframeConfig {
	url: string;
	height?: number;
	sandbox?: string;
}

export interface TextConfig {
	content: string;
}

export interface HTMLConfig {
	content: string;
}

export interface EntityListConfig {
	entity: string;
	filters?: Record<string, unknown>;
	columns?: string[];
	pageSize?: number;
	sortBy?: string;
	sortDir?: 'asc' | 'desc';
}

export interface LinkItem {
	label: string;
	href: string;
	icon?: string;
	description?: string;
	external?: boolean;
}

export interface LinkGroupConfig {
	links: LinkItem[];
}

export interface StatItem {
	label: string;
	value: string; // Can contain templates like {{count:contacts}}
	icon?: string;
	color?: string;
}

export interface StatsConfig {
	items: StatItem[];
}

// Custom page interfaces
export interface CustomPage {
	id: string;
	orgId: string;
	slug: string;
	title: string;
	description?: string;
	icon: string;
	isEnabled: boolean;
	isPublic: boolean;
	layout: 'single' | 'grid';
	components: PageComponent[];
	sortOrder: number;
	createdAt: string;
	modifiedAt: string;
	createdBy?: string;
	modifiedBy?: string;
}

export interface CustomPageListItem {
	id: string;
	slug: string;
	title: string;
	description?: string;
	icon: string;
	isEnabled: boolean;
	isPublic: boolean;
	sortOrder: number;
	modifiedAt: string;
}

export interface CustomPageCreateInput {
	slug: string;
	title: string;
	description?: string;
	icon?: string;
	isEnabled?: boolean;
	isPublic?: boolean;
	layout?: 'single' | 'grid';
	components?: PageComponent[];
	sortOrder?: number;
}

export interface CustomPageUpdateInput {
	slug?: string;
	title?: string;
	description?: string;
	icon?: string;
	isEnabled?: boolean;
	isPublic?: boolean;
	layout?: 'single' | 'grid';
	components?: PageComponent[];
	sortOrder?: number;
}

// Helper constants
export const COMPONENT_TYPES: { value: ComponentType; label: string; description: string; icon: string }[] = [
	{ value: 'iframe', label: 'Iframe', description: 'Embed an external website or app', icon: 'globe' },
	{ value: 'text', label: 'Text', description: 'Plain text content', icon: 'align-left' },
	{ value: 'markdown', label: 'Markdown', description: 'Rich text with markdown formatting', icon: 'file-text' },
	{ value: 'html', label: 'HTML', description: 'Custom HTML content', icon: 'code' },
	{ value: 'entity_list', label: 'Entity List', description: 'Display records from an entity', icon: 'list' },
	{ value: 'link_group', label: 'Quick Links', description: 'Group of shortcut links', icon: 'link' },
	{ value: 'stats', label: 'Stats', description: 'Display key metrics', icon: 'bar-chart-2' }
];

export const COMPONENT_WIDTHS: { value: ComponentWidth; label: string }[] = [
	{ value: 'full', label: 'Full Width' },
	{ value: '1/2', label: 'Half (1/2)' },
	{ value: '1/3', label: 'One Third (1/3)' },
	{ value: '2/3', label: 'Two Thirds (2/3)' }
];

export const PAGE_ICONS: string[] = [
	'file', 'folder', 'home', 'dashboard', 'chart', 'bar-chart', 'pie-chart',
	'users', 'user', 'settings', 'star', 'heart', 'bookmark', 'flag',
	'calendar', 'clock', 'mail', 'phone', 'globe', 'map', 'link',
	'document', 'clipboard', 'archive', 'briefcase', 'dollar-sign', 'percent'
];
