package entity

import "time"

// Contact represents a CRM contact (person)
// Based on EspoCRM Contact entity with simplified, storable fields
type Contact struct {
	ID              string                 `json:"id" db:"id"`
	OrgID           string                 `json:"orgId" db:"org_id"`
	SalutationName  string                 `json:"salutationName" db:"salutation_name"`
	FirstName       string                 `json:"firstName" db:"first_name"`
	LastName        string                 `json:"lastName" db:"last_name"`
	EmailAddress    string                 `json:"emailAddress" db:"email_address"`
	PhoneNumber     string                 `json:"phoneNumber" db:"phone_number"`
	PhoneNumberType string                 `json:"phoneNumberType" db:"phone_number_type"`
	DoNotCall       bool                   `json:"doNotCall" db:"do_not_call"`
	Description     string                 `json:"description" db:"description"`
	AddressStreet   string                 `json:"addressStreet" db:"address_street"`
	AddressCity     string                 `json:"addressCity" db:"address_city"`
	AddressState    string                 `json:"addressState" db:"address_state"`
	AddressCountry  string                 `json:"addressCountry" db:"address_country"`
	AddressPostal   string                 `json:"addressPostalCode" db:"address_postal_code"`
	AccountID       *string                `json:"accountId" db:"account_id"`
	AccountName     string                 `json:"accountName" db:"account_name"`
	AssignedUserID   *string                `json:"assignedUserId" db:"assigned_user_id"`
	AssignedUserName string                 `json:"assignedUserName" db:"-"`
	CreatedByID      *string                `json:"createdById" db:"created_by_id"`
	CreatedByName    string                 `json:"createdByName" db:"-"`
	ModifiedByID     *string                `json:"modifiedById" db:"modified_by_id"`
	ModifiedByName   string                 `json:"modifiedByName" db:"-"`
	CreatedAt       time.Time              `json:"createdAt" db:"created_at"`
	ModifiedAt      time.Time              `json:"modifiedAt" db:"modified_at"`
	Deleted         bool                   `json:"deleted" db:"deleted"`
	CustomFieldsRaw string                 `json:"-" db:"custom_fields"`
	CustomFields    map[string]interface{} `json:"customFields,omitempty" db:"-"`
}

// Name returns the full name of the contact
func (c *Contact) Name() string {
	if c.FirstName == "" {
		return c.LastName
	}
	if c.LastName == "" {
		return c.FirstName
	}
	return c.FirstName + " " + c.LastName
}

// ContactCreateInput represents the input for creating a contact
type ContactCreateInput struct {
	SalutationName  string                 `json:"salutationName"`
	FirstName       string                 `json:"firstName"`
	LastName        string                 `json:"lastName" validate:"required"`
	EmailAddress    string                 `json:"emailAddress"`
	PhoneNumber     string                 `json:"phoneNumber"`
	PhoneNumberType string                 `json:"phoneNumberType"`
	DoNotCall       bool                   `json:"doNotCall"`
	Description     string                 `json:"description"`
	AddressStreet   string                 `json:"addressStreet"`
	AddressCity     string                 `json:"addressCity"`
	AddressState    string                 `json:"addressState"`
	AddressCountry  string                 `json:"addressCountry"`
	AddressPostal   string                 `json:"addressPostalCode"`
	AccountID       *string                `json:"accountId"`
	AccountName     string                 `json:"accountName"`
	AssignedUserID  *string                `json:"assignedUserId"`
	CustomFields    map[string]interface{} `json:"customFields"`
}

// ContactUpdateInput represents the input for updating a contact
type ContactUpdateInput struct {
	SalutationName  *string                `json:"salutationName"`
	FirstName       *string                `json:"firstName"`
	LastName        *string                `json:"lastName"`
	EmailAddress    *string                `json:"emailAddress"`
	PhoneNumber     *string                `json:"phoneNumber"`
	PhoneNumberType *string                `json:"phoneNumberType"`
	DoNotCall       *bool                  `json:"doNotCall"`
	Description     *string                `json:"description"`
	AddressStreet   *string                `json:"addressStreet"`
	AddressCity     *string                `json:"addressCity"`
	AddressState    *string                `json:"addressState"`
	AddressCountry  *string                `json:"addressCountry"`
	AddressPostal   *string                `json:"addressPostalCode"`
	AccountID       *string                `json:"accountId"`
	AccountName     *string                `json:"accountName"`
	AssignedUserID  *string                `json:"assignedUserId"`
	CustomFields    map[string]interface{} `json:"customFields"`
}

// ContactListParams represents query parameters for listing contacts
type ContactListParams struct {
	Search     string `query:"search"`
	SortBy     string `query:"sortBy"`
	SortDir    string `query:"sortDir"`
	Page       int    `query:"page"`
	PageSize   int    `query:"pageSize"`
	Filter     string `query:"filter"`
	KnownTotal int    `query:"knownTotal"`
	Owner      string `query:"owner"` // "me", "unassigned", or a user ID
}

// ContactListResponse represents the response for listing contacts
type ContactListResponse struct {
	Data       []Contact `json:"data"`
	Total      int       `json:"total"`
	Page       int       `json:"page"`
	PageSize   int       `json:"pageSize"`
	TotalPages int       `json:"totalPages"`
}
