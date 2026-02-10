export interface Account {
    id: string;
    orgId: string;
    name: string;
    website: string;
    emailAddress: string;
    phoneNumber: string;
    type: string;
    industry: string;
    sicCode: string;
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
    stage: string | null;
    assignedUserId: string | null;
    createdById: string | null;
    createdByName: string;
    modifiedById: string | null;
    modifiedByName: string;
    createdAt: string;
    modifiedAt: string;
    deleted: boolean;
    customFields?: Record<string, unknown>;
}

export interface AccountListResponse {
    data: Account[];
    total: number;
    page: number;
    pageSize: number;
    totalPages: number;
}

export interface AccountCreate {
    name: string;
    website?: string;
    emailAddress?: string;
    phoneNumber?: string;
    type?: string;
    industry?: string;
    sicCode?: string;
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
    assignedUserId?: string;
}

export interface AccountUpdate {
    name?: string;
    website?: string;
    emailAddress?: string;
    phoneNumber?: string;
    type?: string;
    industry?: string;
    sicCode?: string;
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
    assignedUserId?: string;
}
