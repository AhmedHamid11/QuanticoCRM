export interface Contact {
	id: string;
	orgId: string;
	salutationName: string;
	firstName: string;
	lastName: string;
	emailAddress: string;
	phoneNumber: string;
	phoneNumberType: string;
	doNotCall: boolean;
	description: string;
	addressStreet: string;
	addressCity: string;
	addressState: string;
	addressCountry: string;
	addressPostalCode: string;
	accountId: string | null;
	accountName: string;
	assignedUserId: string | null;
	createdById: string | null;
	createdByName: string;
	modifiedById: string | null;
	modifiedByName: string;
	createdAt: string;
	modifiedAt: string;
	deleted: boolean;
}

export interface ContactListResponse {
	data: Contact[];
	total: number;
	page: number;
	pageSize: number;
	totalPages: number;
}

export interface ContactCreateInput {
	salutationName?: string;
	firstName?: string;
	lastName: string;
	emailAddress?: string;
	phoneNumber?: string;
	phoneNumberType?: string;
	doNotCall?: boolean;
	description?: string;
	addressStreet?: string;
	addressCity?: string;
	addressState?: string;
	addressCountry?: string;
	addressPostalCode?: string;
	accountId?: string | null;
	accountName?: string;
	assignedUserId?: string | null;
}

export interface ContactUpdateInput {
	salutationName?: string;
	firstName?: string;
	lastName?: string;
	emailAddress?: string;
	phoneNumber?: string;
	phoneNumberType?: string;
	doNotCall?: boolean;
	description?: string;
	addressStreet?: string;
	addressCity?: string;
	addressState?: string;
	addressCountry?: string;
	addressPostalCode?: string;
	accountId?: string | null;
	accountName?: string;
	assignedUserId?: string | null;
}
