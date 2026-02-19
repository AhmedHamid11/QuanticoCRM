package entity

import "time"

// Account represents a CRM account (company/organization)
type Account struct {
	ID                        string                 `json:"id" db:"id"`
	OrgID                     string                 `json:"orgId" db:"org_id"`
	Name                      string                 `json:"name" db:"name"`
	Website                   string                 `json:"website" db:"website"`
	EmailAddress              string                 `json:"emailAddress" db:"email_address"`
	PhoneNumber               string                 `json:"phoneNumber" db:"phone_number"`
	Type                      string                 `json:"type" db:"type"`
	Industry                  string                 `json:"industry" db:"industry"`
	SicCode                   string                 `json:"sicCode" db:"sic_code"`
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
	Stage                     *string                `json:"stage" db:"stage"`
	AssignedUserID            *string                `json:"assignedUserId" db:"assigned_user_id"`
	AssignedUserName          string                 `json:"assignedUserName" db:"-"`
	CreatedByID               *string                `json:"createdById" db:"created_by_id"`
	CreatedByName             string                 `json:"createdByName" db:"-"`
	ModifiedByID              *string                `json:"modifiedById" db:"modified_by_id"`
	ModifiedByName            string                 `json:"modifiedByName" db:"-"`
	CreatedAt                 time.Time              `json:"createdAt" db:"created_at"`
	ModifiedAt                time.Time              `json:"modifiedAt" db:"modified_at"`
	Deleted                   bool                   `json:"deleted" db:"deleted"`
	CustomFieldsRaw           string                 `json:"-" db:"custom_fields"`
	CustomFields              map[string]interface{} `json:"customFields,omitempty" db:"-"`
}

// AccountCreateInput represents the input for creating an account
type AccountCreateInput struct {
	Name                      string                 `json:"name" validate:"required"`
	Website                   string                 `json:"website"`
	EmailAddress              string                 `json:"emailAddress"`
	PhoneNumber               string                 `json:"phoneNumber"`
	Type                      string                 `json:"type"`
	Industry                  string                 `json:"industry"`
	SicCode                   string                 `json:"sicCode"`
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
	AssignedUserID            *string                `json:"assignedUserId"`
	CustomFields              map[string]interface{} `json:"customFields"`
}

// AccountUpdateInput represents the input for updating an account
type AccountUpdateInput struct {
	Name                      *string                `json:"name"`
	Website                   *string                `json:"website"`
	EmailAddress              *string                `json:"emailAddress"`
	PhoneNumber               *string                `json:"phoneNumber"`
	Type                      *string                `json:"type"`
	Industry                  *string                `json:"industry"`
	SicCode                   *string                `json:"sicCode"`
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
	Stage                     *string                `json:"stage"`
	AssignedUserID            *string                `json:"assignedUserId"`
	CustomFields              map[string]interface{} `json:"customFields"`
}

// AccountListParams represents query parameters for listing accounts
type AccountListParams struct {
	Search     string `query:"search"`
	SortBy     string `query:"sortBy"`
	SortDir    string `query:"sortDir"`
	Page       int    `query:"page"`
	PageSize   int    `query:"pageSize"`
	Filter     string `query:"filter"`
	KnownTotal int    `query:"knownTotal"`
	Owner      string `query:"owner"` // "me", "unassigned", or a user ID
}

// AccountListResponse represents the response for listing accounts
type AccountListResponse struct {
	Data       []Account `json:"data"`
	Total      int       `json:"total"`
	Page       int       `json:"page"`
	PageSize   int       `json:"pageSize"`
	TotalPages int       `json:"totalPages"`
}
