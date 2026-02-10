export interface Quote {
	id: string;
	orgId: string;
	name: string;
	quoteNumber: string;
	status: string;
	accountId: string | null;
	accountName: string;
	contactId: string | null;
	contactName: string;
	validUntil: string;
	subtotal: number;
	discountPercent: number;
	discountAmount: number;
	taxPercent: number;
	taxAmount: number;
	shippingAmount: number;
	grandTotal: number;
	currency: string;
	billingAddressStreet: string;
	billingAddressCity: string;
	billingAddressState: string;
	billingAddressCountry: string;
	billingAddressPostalCode: string;
	shippingAddressStreet: string;
	shippingAddressCity: string;
	shippingAddressState: string;
	shippingAddressCountry: string;
	shippingAddressPostalCode: string;
	description: string;
	terms: string;
	notes: string;
	assignedUserId: string | null;
	createdById: string | null;
	createdByName: string;
	modifiedById: string | null;
	modifiedByName: string;
	createdAt: string;
	modifiedAt: string;
	deleted: boolean;
	lineItems: QuoteLineItem[];
}

export interface QuoteLineItem {
	id: string;
	orgId: string;
	quoteId: string;
	name: string;
	description: string;
	sku: string;
	quantity: number;
	unitPrice: number;
	discountPercent: number;
	discountAmount: number;
	taxPercent: number;
	total: number;
	sortOrder: number;
	createdAt: string;
	modifiedAt: string;
}

export interface QuoteLineItemInput {
	id?: string;
	name: string;
	description: string;
	sku: string;
	quantity: number;
	unitPrice: number;
	discountPercent: number;
	discountAmount: number;
	taxPercent: number;
	sortOrder: number;
}

export interface QuoteCreateInput {
	name: string;
	status?: string;
	accountId?: string | null;
	accountName?: string;
	contactId?: string | null;
	contactName?: string;
	validUntil?: string;
	discountPercent?: number;
	discountAmount?: number;
	taxPercent?: number;
	shippingAmount?: number;
	currency?: string;
	billingAddressStreet?: string;
	billingAddressCity?: string;
	billingAddressState?: string;
	billingAddressCountry?: string;
	billingAddressPostalCode?: string;
	shippingAddressStreet?: string;
	shippingAddressCity?: string;
	shippingAddressState?: string;
	shippingAddressCountry?: string;
	shippingAddressPostalCode?: string;
	description?: string;
	terms?: string;
	notes?: string;
	assignedUserId?: string | null;
	lineItems?: QuoteLineItemInput[];
}

export interface QuoteListResponse {
	data: Quote[];
	total: number;
	page: number;
	pageSize: number;
	totalPages: number;
}

export const QUOTE_STATUSES = [
	'Draft',
	'Needs Review',
	'Approved',
	'Sent',
	'Accepted',
	'Declined',
	'Expired'
] as const;

export function getStatusColor(status: string): string {
	switch (status) {
		case 'Draft': return 'bg-gray-100 text-gray-800';
		case 'Needs Review': return 'bg-yellow-100 text-yellow-800';
		case 'Approved': return 'bg-blue-100 text-blue-800';
		case 'Sent': return 'bg-indigo-100 text-indigo-800';
		case 'Accepted': return 'bg-green-100 text-green-800';
		case 'Declined': return 'bg-red-100 text-red-800';
		case 'Expired': return 'bg-orange-100 text-orange-800';
		default: return 'bg-gray-100 text-gray-800';
	}
}

export function formatCurrency(amount: number, currency = 'USD'): string {
	return new Intl.NumberFormat('en-US', {
		style: 'currency',
		currency,
		minimumFractionDigits: 2
	}).format(amount);
}
