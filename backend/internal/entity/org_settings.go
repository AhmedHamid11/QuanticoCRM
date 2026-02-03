package entity

// OrgSettings represents per-organization settings
type OrgSettings struct {
	OrgID        string `json:"orgId" db:"org_id"`
	HomePage     string `json:"homePage" db:"home_page"`
	SettingsJSON string `json:"-" db:"settings_json"`
}

// OrgSettingsUpdateInput represents input for updating org settings
type OrgSettingsUpdateInput struct {
	HomePage *string `json:"homePage"`
}
