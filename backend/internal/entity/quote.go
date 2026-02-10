package entity

import "time"

// Quote represents a CRM quote/proposal
type Quote struct {
	ID                        string                 `json:"id" db:"id"`
	OrgID                     string                 `json:"orgId" db:"org_id"`
	Name                      string                 `json:"name" db:"name"`
	QuoteNumber               string                 `json:"quoteNumber" db:"quote_number"`
	Status                    string                 `json:"status" db:"status"`
	AccountID                 *string                `json:"accountId" db:"account_id"`
	AccountName               string                 `json:"accountName" db:"account_name"`
	ContactID                 *string                `json:"contactId" db:"contact_id"`
	ContactName               string                 `json:"contactName" db:"contact_name"`
	ValidUntil                string                 `json:"validUntil" db:"valid_until"`
	Subtotal                  float64                `json:"subtotal" db:"subtotal"`
	DiscountPercent           float64                `json:"discountPercent" db:"discount_percent"`
	DiscountAmount            float64                `json:"discountAmount" db:"discount_amount"`
	TaxPercent                float64                `json:"taxPercent" db:"tax_percent"`
	TaxAmount                 float64                `json:"taxAmount" db:"tax_amount"`
	ShippingAmount            float64                `json:"shippingAmount" db:"shipping_amount"`
	GrandTotal                float64                `json:"grandTotal" db:"grand_total"`
	Currency                  string                 `json:"currency" db:"currency"`
	BillingAddressStreet      string                 `json:"billingAddressStreet" db:"billing_address_street"`
	BillingAddressCity        string                 `json:"billingAddressCity" db:"billing_address_city"`
	BillingAddressState       string                 `json:"billingAddressState" db:"billing_address_state"`
	BillingAddressCountry     string                 `json:"billingAddressCountry" db:"billing_address_country"`
	BillingAddressPostalCode  string                 `json:"billingAddressPostalCode" db:"billing_address_postal_code"`
	ShippingAddressStreet     string                 `json:"shippingAddressStreet" db:"shipping_address_street"`
	ShippingAddressCity       string                 `json:"shippingAddressCity" db:"shipping_address_city"`
	ShippingAddressState      string                 `json:"shippingAddressState" db:"shipping_address_state"`
	ShippingAddressCountry    string                 `json:"shippingAddressCountry" db:"shipping_address_country"`
	ShippingAddressPostalCode string                 `json:"shippingAddressPostalCode" db:"shipping_address_postal_code"`
	Description               string                 `json:"description" db:"description"`
	Terms                     string                 `json:"terms" db:"terms"`
	Notes                     string                 `json:"notes" db:"notes"`
	AssignedUserID            *string                `json:"assignedUserId" db:"assigned_user_id"`
	CreatedByID               *string                `json:"createdById" db:"created_by_id"`
	CreatedByName             string                 `json:"createdByName" db:"-"`
	ModifiedByID              *string                `json:"modifiedById" db:"modified_by_id"`
	ModifiedByName            string                 `json:"modifiedByName" db:"-"`
	CreatedAt                 time.Time              `json:"createdAt" db:"created_at"`
	ModifiedAt                time.Time              `json:"modifiedAt" db:"modified_at"`
	Deleted                   bool                   `json:"deleted" db:"deleted"`
	CustomFieldsRaw           string                 `json:"-" db:"custom_fields"`
	CustomFields              map[string]interface{} `json:"customFields,omitempty" db:"-"`
	LineItems                 []QuoteLineItem        `json:"lineItems,omitempty" db:"-"`
}

// QuoteLineItem represents a single line item in a quote
type QuoteLineItem struct {
	ID              string  `json:"id" db:"id"`
	OrgID           string  `json:"orgId" db:"org_id"`
	QuoteID         string  `json:"quoteId" db:"quote_id"`
	Name            string  `json:"name" db:"name"`
	Description     string  `json:"description" db:"description"`
	SKU             string  `json:"sku" db:"sku"`
	Quantity        float64 `json:"quantity" db:"quantity"`
	UnitPrice       float64 `json:"unitPrice" db:"unit_price"`
	DiscountPercent float64 `json:"discountPercent" db:"discount_percent"`
	DiscountAmount  float64 `json:"discountAmount" db:"discount_amount"`
	TaxPercent      float64 `json:"taxPercent" db:"tax_percent"`
	Total           float64 `json:"total" db:"total"`
	SortOrder       int     `json:"sortOrder" db:"sort_order"`
	CreatedAt       string  `json:"createdAt" db:"created_at"`
	ModifiedAt      string  `json:"modifiedAt" db:"modified_at"`
}

// QuoteCreateInput represents the input for creating a quote
type QuoteCreateInput struct {
	Name                      string                 `json:"name"`
	Status                    string                 `json:"status"`
	AccountID                 *string                `json:"accountId"`
	AccountName               string                 `json:"accountName"`
	ContactID                 *string                `json:"contactId"`
	ContactName               string                 `json:"contactName"`
	ValidUntil                string                 `json:"validUntil"`
	DiscountPercent           float64                `json:"discountPercent"`
	DiscountAmount            float64                `json:"discountAmount"`
	TaxPercent                float64                `json:"taxPercent"`
	ShippingAmount            float64                `json:"shippingAmount"`
	Currency                  string                 `json:"currency"`
	BillingAddressStreet      string                 `json:"billingAddressStreet"`
	BillingAddressCity        string                 `json:"billingAddressCity"`
	BillingAddressState       string                 `json:"billingAddressState"`
	BillingAddressCountry     string                 `json:"billingAddressCountry"`
	BillingAddressPostalCode  string                 `json:"billingAddressPostalCode"`
	ShippingAddressStreet     string                 `json:"shippingAddressStreet"`
	ShippingAddressCity       string                 `json:"shippingAddressCity"`
	ShippingAddressState      string                 `json:"shippingAddressState"`
	ShippingAddressCountry    string                 `json:"shippingAddressCountry"`
	ShippingAddressPostalCode string                 `json:"shippingAddressPostalCode"`
	Description               string                 `json:"description"`
	Terms                     string                 `json:"terms"`
	Notes                     string                 `json:"notes"`
	AssignedUserID            *string                `json:"assignedUserId"`
	CustomFields              map[string]interface{} `json:"customFields"`
	LineItems                 []QuoteLineItemInput   `json:"lineItems"`
}

// QuoteUpdateInput represents the input for updating a quote
type QuoteUpdateInput struct {
	Name                      *string                `json:"name"`
	Status                    *string                `json:"status"`
	AccountID                 *string                `json:"accountId"`
	AccountName               *string                `json:"accountName"`
	ContactID                 *string                `json:"contactId"`
	ContactName               *string                `json:"contactName"`
	ValidUntil                *string                `json:"validUntil"`
	DiscountPercent           *float64               `json:"discountPercent"`
	DiscountAmount            *float64               `json:"discountAmount"`
	TaxPercent                *float64               `json:"taxPercent"`
	ShippingAmount            *float64               `json:"shippingAmount"`
	Currency                  *string                `json:"currency"`
	BillingAddressStreet      *string                `json:"billingAddressStreet"`
	BillingAddressCity        *string                `json:"billingAddressCity"`
	BillingAddressState       *string                `json:"billingAddressState"`
	BillingAddressCountry     *string                `json:"billingAddressCountry"`
	BillingAddressPostalCode  *string                `json:"billingAddressPostalCode"`
	ShippingAddressStreet     *string                `json:"shippingAddressStreet"`
	ShippingAddressCity       *string                `json:"shippingAddressCity"`
	ShippingAddressState      *string                `json:"shippingAddressState"`
	ShippingAddressCountry    *string                `json:"shippingAddressCountry"`
	ShippingAddressPostalCode *string                `json:"shippingAddressPostalCode"`
	Description               *string                `json:"description"`
	Terms                     *string                `json:"terms"`
	Notes                     *string                `json:"notes"`
	AssignedUserID            *string                `json:"assignedUserId"`
	CustomFields              map[string]interface{} `json:"customFields"`
	LineItems                 []QuoteLineItemInput   `json:"lineItems,omitempty"`
}

// QuoteLineItemInput represents a line item in create/update
type QuoteLineItemInput struct {
	ID              string  `json:"id"`
	Name            string  `json:"name"`
	Description     string  `json:"description"`
	SKU             string  `json:"sku"`
	Quantity        float64 `json:"quantity"`
	UnitPrice       float64 `json:"unitPrice"`
	DiscountPercent float64 `json:"discountPercent"`
	DiscountAmount  float64 `json:"discountAmount"`
	TaxPercent      float64 `json:"taxPercent"`
	SortOrder       int     `json:"sortOrder"`
}

// QuoteListParams represents query parameters for listing quotes
type QuoteListParams struct {
	Search     string `query:"search"`
	SortBy     string `query:"sortBy"`
	SortDir    string `query:"sortDir"`
	Page       int    `query:"page"`
	PageSize   int    `query:"pageSize"`
	Filter     string `query:"filter"`
	KnownTotal int    `query:"knownTotal"`
}

// QuoteListResponse represents the response for listing quotes
type QuoteListResponse struct {
	Data       []Quote `json:"data"`
	Total      int     `json:"total"`
	Page       int     `json:"page"`
	PageSize   int     `json:"pageSize"`
	TotalPages int     `json:"totalPages"`
}
