package entity

import "time"

// GoogleCalendarConnection stores OAuth tokens for a user's Google Calendar connection
type GoogleCalendarConnection struct {
	ID                    string    `json:"id"`
	OrgID                 string    `json:"orgId"`
	UserID                string    `json:"userId"`
	AccessTokenEncrypted  []byte    `json:"-"`
	RefreshTokenEncrypted []byte    `json:"-"`
	TokenExpiry           *time.Time `json:"tokenExpiry,omitempty"`
	CalendarID            string    `json:"calendarId"`
	ConnectedAt           *time.Time `json:"connectedAt,omitempty"`
	CreatedAt             time.Time  `json:"createdAt"`
	UpdatedAt             time.Time  `json:"updatedAt"`
}

// TimeWindow represents a time range (e.g., "09:00" to "17:00")
type TimeWindow struct {
	Start string `json:"start"` // "09:00"
	End   string `json:"end"`   // "17:00"
}

// SchedulingPage represents a user's scheduling page configuration
type SchedulingPage struct {
	ID               string    `json:"id"`
	OrgID            string    `json:"orgId"`
	UserID           string    `json:"userId"`
	Slug             string    `json:"slug"`
	Title            string    `json:"title"`
	Description      string    `json:"description"`
	DurationMinutes  int       `json:"durationMinutes"`
	AvailabilityJSON string    `json:"-"` // raw JSON stored in DB
	Timezone         string    `json:"timezone"`
	IsActive         bool      `json:"isActive"`
	BufferMinutes    int       `json:"bufferMinutes"`
	MaxDaysAhead     int       `json:"maxDaysAhead"`
	CreatedAt        time.Time `json:"createdAt"`
	UpdatedAt        time.Time `json:"updatedAt"`
}

// SchedulingPageCreateInput holds input for creating a scheduling page
type SchedulingPageCreateInput struct {
	Slug            string                        `json:"slug"`
	Title           string                        `json:"title"`
	Description     string                        `json:"description"`
	DurationMinutes int                           `json:"durationMinutes"`
	Availability    map[string][]TimeWindow       `json:"availability"`
	Timezone        string                        `json:"timezone"`
	BufferMinutes   int                           `json:"bufferMinutes"`
	MaxDaysAhead    int                           `json:"maxDaysAhead"`
}

// SchedulingPageUpdateInput holds input for updating a scheduling page (all optional)
type SchedulingPageUpdateInput struct {
	Slug            *string                        `json:"slug,omitempty"`
	Title           *string                        `json:"title,omitempty"`
	Description     *string                        `json:"description,omitempty"`
	DurationMinutes *int                           `json:"durationMinutes,omitempty"`
	Availability    map[string][]TimeWindow        `json:"availability,omitempty"`
	Timezone        *string                        `json:"timezone,omitempty"`
	IsActive        *bool                          `json:"isActive,omitempty"`
	BufferMinutes   *int                           `json:"bufferMinutes,omitempty"`
	MaxDaysAhead    *int                           `json:"maxDaysAhead,omitempty"`
}

// SchedulingBooking represents a booking made by an external visitor
type SchedulingBooking struct {
	ID                string    `json:"id"`
	OrgID             string    `json:"orgId"`
	SchedulingPageID  string    `json:"schedulingPageId"`
	UserID            string    `json:"userId"`
	GuestName         string    `json:"guestName"`
	GuestEmail        string    `json:"guestEmail"`
	GuestNotes        string    `json:"guestNotes"`
	StartTime         time.Time `json:"startTime"`
	EndTime           time.Time `json:"endTime"`
	Status            string    `json:"status"`
	GoogleEventID     string    `json:"googleEventId,omitempty"`
	CreatedAt         time.Time `json:"createdAt"`
}

// BookingCreateInput holds input for creating a booking
type BookingCreateInput struct {
	GuestName  string `json:"guestName"`
	GuestEmail string `json:"guestEmail"`
	GuestNotes string `json:"guestNotes"`
	StartTime  string `json:"startTime"` // ISO 8601
}

// AvailableSlot represents a single available time slot
type AvailableSlot struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

// SchedulingPagePublicView is the public-safe view of a scheduling page
type SchedulingPagePublicView struct {
	Slug            string                  `json:"slug"`
	Title           string                  `json:"title"`
	Description     string                  `json:"description"`
	DurationMinutes int                     `json:"durationMinutes"`
	Availability    map[string][]TimeWindow `json:"availability"`
	Timezone        string                  `json:"timezone"`
	MaxDaysAhead    int                     `json:"maxDaysAhead"`
	OwnerName       string                  `json:"ownerName"`
}
