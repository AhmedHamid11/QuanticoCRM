export interface PdfTemplate {
	id: string;
	orgId: string;
	name: string;
	entityType: string;
	isDefault: boolean;
	isSystem: boolean;
	baseDesign: string;
	branding: PdfBranding | null;
	sections: SectionConfig[];
	pageSize: string;
	orientation: string;
	margins: string;
	createdAt: string;
	modifiedAt: string;
}

export interface PdfBranding {
	logoUrl: string;
	companyName: string;
	primaryColor: string;
	accentColor: string;
	fontFamily: string;
}

export interface SectionConfig {
	id: string;
	label: string;
	enabled: boolean;
	fields: string[];
}

export interface AvailableField {
	name: string;
	label: string;
	section: string;
}

export const BASE_DESIGNS = [
	{ value: 'professional', label: 'Professional', description: 'Blue header bar, structured table, corporate' },
	{ value: 'minimal', label: 'Minimal', description: 'Clean white, thin borders, modern typography' },
	{ value: 'modern', label: 'Modern', description: 'Gradient accents, rounded cards, contemporary' }
] as const;

export const PAGE_SIZES = ['A4', 'Letter', 'Legal'] as const;
export const ORIENTATIONS = ['portrait', 'landscape'] as const;
export const FONT_FAMILIES = [
	'Helvetica, Arial, sans-serif',
	"'Segoe UI', system-ui, sans-serif",
	"'Inter', system-ui, sans-serif",
	"Georgia, serif",
	"'Courier New', monospace"
] as const;
