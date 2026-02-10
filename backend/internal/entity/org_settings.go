package entity

// OrgSettings represents per-organization settings
type OrgSettings struct {
	OrgID                  string `json:"orgId" db:"org_id"`
	HomePage               string `json:"homePage" db:"home_page"`
	IdleTimeoutMinutes     int    `json:"idleTimeoutMinutes" db:"idle_timeout_minutes"`
	AbsoluteTimeoutMinutes int    `json:"absoluteTimeoutMinutes" db:"absolute_timeout_minutes"`
	SettingsJSON           string `json:"-" db:"settings_json"`
}

// OrgSettingsUpdateInput represents input for updating org settings
type OrgSettingsUpdateInput struct {
	HomePage               *string `json:"homePage"`
	IdleTimeoutMinutes     *int    `json:"idleTimeoutMinutes"`
	AbsoluteTimeoutMinutes *int    `json:"absoluteTimeoutMinutes"`
}

// SessionTimeoutBounds defines valid ranges
const (
	MinIdleTimeout     = 15   // 15 minutes
	MaxIdleTimeout     = 60   // 60 minutes
	MinAbsoluteTimeout = 480  // 8 hours
	MaxAbsoluteTimeout = 4320 // 72 hours
	DefaultIdleTimeout     = 30
	DefaultAbsoluteTimeout = 1440 // 24 hours
)
