package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/fastcrm/backend/internal/entity"
	"github.com/fastcrm/backend/internal/repo"
	"github.com/google/uuid"
)

var (
	ErrInvalidSlug        = errors.New("slug must be 3-50 characters: lowercase alphanumeric and hyphens only")
	ErrSlugTaken          = errors.New("slug is already in use")
	ErrPageNotFound       = errors.New("scheduling page not found")
	ErrPageInactive       = errors.New("scheduling page is not active")
	ErrSlotNotAvailable   = errors.New("the requested time slot is not available")
	ErrInvalidTimeFormat  = errors.New("invalid time format; expected HH:MM")
	ErrInvalidTimeWindow  = errors.New("availability window start must be before end")

	slugPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,48}[a-z0-9]$`)
	timePattern = regexp.MustCompile(`^([0-1][0-9]|2[0-3]):[0-5][0-9]$`)
)

// SchedulingService handles business logic for scheduling pages and bookings
type SchedulingService struct {
	repo        *repo.SchedulingRepo
	gcalService *GoogleCalendarService
}

// NewSchedulingService creates a new SchedulingService
func NewSchedulingService(r *repo.SchedulingRepo, gcalService *GoogleCalendarService) *SchedulingService {
	return &SchedulingService{
		repo:        r,
		gcalService: gcalService,
	}
}

// CreatePage creates a new scheduling page
func (s *SchedulingService) CreatePage(ctx context.Context, orgID, userID string, input entity.SchedulingPageCreateInput) (*entity.SchedulingPage, error) {
	// Validate slug
	slug := strings.ToLower(strings.TrimSpace(input.Slug))
	if len(slug) < 3 || !slugPattern.MatchString(slug) {
		return nil, ErrInvalidSlug
	}

	// Validate availability windows
	if err := validateAvailability(input.Availability); err != nil {
		return nil, err
	}

	// Validate duration
	if input.DurationMinutes <= 0 {
		input.DurationMinutes = 30
	}

	// Set defaults
	if input.Timezone == "" {
		input.Timezone = "America/New_York"
	}
	if input.MaxDaysAhead <= 0 {
		input.MaxDaysAhead = 30
	}

	// Marshal availability to JSON
	availJSON, err := json.Marshal(input.Availability)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal availability: %w", err)
	}

	page := &entity.SchedulingPage{
		ID:               uuid.New().String(),
		OrgID:            orgID,
		UserID:           userID,
		Slug:             slug,
		Title:            input.Title,
		Description:      input.Description,
		DurationMinutes:  input.DurationMinutes,
		AvailabilityJSON: string(availJSON),
		Timezone:         input.Timezone,
		IsActive:         true,
		BufferMinutes:    input.BufferMinutes,
		MaxDaysAhead:     input.MaxDaysAhead,
	}

	if err := s.repo.CreatePage(ctx, page); err != nil {
		if strings.Contains(err.Error(), "UNIQUE") || strings.Contains(err.Error(), "unique") {
			return nil, ErrSlugTaken
		}
		return nil, fmt.Errorf("failed to create scheduling page: %w", err)
	}

	return page, nil
}

// UpdatePage updates an existing scheduling page
func (s *SchedulingService) UpdatePage(ctx context.Context, orgID, userID, pageID string, input entity.SchedulingPageUpdateInput) (*entity.SchedulingPage, error) {
	page, err := s.repo.GetPage(ctx, pageID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrPageNotFound
		}
		return nil, fmt.Errorf("failed to load page: %w", err)
	}

	// Verify ownership
	if page.OrgID != orgID || page.UserID != userID {
		return nil, ErrPageNotFound
	}

	// Apply updates
	if input.Slug != nil {
		slug := strings.ToLower(strings.TrimSpace(*input.Slug))
		if len(slug) < 3 || !slugPattern.MatchString(slug) {
			return nil, ErrInvalidSlug
		}
		page.Slug = slug
	}
	if input.Title != nil {
		page.Title = *input.Title
	}
	if input.Description != nil {
		page.Description = *input.Description
	}
	if input.DurationMinutes != nil {
		page.DurationMinutes = *input.DurationMinutes
	}
	if input.Availability != nil {
		if err := validateAvailability(input.Availability); err != nil {
			return nil, err
		}
		availJSON, err := json.Marshal(input.Availability)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal availability: %w", err)
		}
		page.AvailabilityJSON = string(availJSON)
	}
	if input.Timezone != nil {
		page.Timezone = *input.Timezone
	}
	if input.IsActive != nil {
		page.IsActive = *input.IsActive
	}
	if input.BufferMinutes != nil {
		page.BufferMinutes = *input.BufferMinutes
	}
	if input.MaxDaysAhead != nil {
		page.MaxDaysAhead = *input.MaxDaysAhead
	}

	if err := s.repo.UpdatePage(ctx, page); err != nil {
		if strings.Contains(err.Error(), "UNIQUE") || strings.Contains(err.Error(), "unique") {
			return nil, ErrSlugTaken
		}
		return nil, fmt.Errorf("failed to update page: %w", err)
	}

	return page, nil
}

// GetPublicPageInfo returns public-safe page info by slug
func (s *SchedulingService) GetPublicPageInfo(ctx context.Context, slug string) (*entity.SchedulingPagePublicView, error) {
	page, err := s.repo.GetPageBySlug(ctx, slug)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrPageNotFound
		}
		return nil, fmt.Errorf("failed to load page: %w", err)
	}

	if !page.IsActive {
		return nil, ErrPageInactive
	}

	availability, err := unmarshalAvailability(page.AvailabilityJSON)
	if err != nil {
		availability = map[string][]entity.TimeWindow{}
	}

	return &entity.SchedulingPagePublicView{
		Slug:            page.Slug,
		Title:           page.Title,
		Description:     page.Description,
		DurationMinutes: page.DurationMinutes,
		Availability:    availability,
		Timezone:        page.Timezone,
		MaxDaysAhead:    page.MaxDaysAhead,
	}, nil
}

// GetAvailableSlots returns available booking slots for a specific date
func (s *SchedulingService) GetAvailableSlots(ctx context.Context, slug, dateStr string) ([]entity.AvailableSlot, error) {
	page, err := s.repo.GetPageBySlug(ctx, slug)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrPageNotFound
		}
		return nil, fmt.Errorf("failed to load page: %w", err)
	}

	if !page.IsActive {
		return nil, ErrPageInactive
	}

	// Parse date
	loc, err := time.LoadLocation(page.Timezone)
	if err != nil {
		loc = time.UTC
	}

	date, err := time.ParseInLocation("2006-01-02", dateStr, loc)
	if err != nil {
		return nil, fmt.Errorf("invalid date format; expected YYYY-MM-DD")
	}

	// Check date is not too far ahead
	maxDate := time.Now().In(loc).AddDate(0, 0, page.MaxDaysAhead)
	if date.After(maxDate) {
		return nil, fmt.Errorf("date is beyond max days ahead (%d days)", page.MaxDaysAhead)
	}

	// Get day of week
	dayName := strings.ToLower(date.Weekday().String())

	// Load availability for that day
	availability, err := unmarshalAvailability(page.AvailabilityJSON)
	if err != nil {
		return nil, fmt.Errorf("failed to parse availability: %w", err)
	}

	windows, ok := availability[dayName]
	if !ok || len(windows) == 0 {
		return []entity.AvailableSlot{}, nil // No availability on this day
	}

	// Generate all candidate slots from availability windows
	var candidates []entity.AvailableSlot
	slotDuration := time.Duration(page.DurationMinutes) * time.Minute
	bufferDuration := time.Duration(page.BufferMinutes) * time.Minute

	for _, window := range windows {
		windowStart, err1 := parseTimeOnDate(date, window.Start, loc)
		windowEnd, err2 := parseTimeOnDate(date, window.End, loc)
		if err1 != nil || err2 != nil {
			continue
		}

		for slotStart := windowStart; slotStart.Add(slotDuration).Before(windowEnd) || slotStart.Add(slotDuration).Equal(windowEnd); slotStart = slotStart.Add(slotDuration + bufferDuration) {
			slotEnd := slotStart.Add(slotDuration)
			candidates = append(candidates, entity.AvailableSlot{
				Start: slotStart,
				End:   slotEnd,
			})
		}
	}

	// Define the full day range for queries
	dayStart := date
	dayEnd := date.Add(24 * time.Hour)

	// Get Google Calendar busy slots (if connected)
	var busySlots []BusySlot
	if s.gcalService != nil {
		busySlots, _ = s.gcalService.GetFreeBusySlots(ctx, page.OrgID, page.UserID, dayStart, dayEnd)
	}

	// Get existing bookings for the day
	existingBookings, _ := s.repo.ListBookingsByUserInRange(ctx, page.UserID, page.OrgID, dayStart, dayEnd)

	// Filter out slots that overlap with busy times or existing bookings
	now := time.Now()
	var available []entity.AvailableSlot
	for _, slot := range candidates {
		// Skip past slots
		if slot.Start.Before(now) {
			continue
		}

		// Check Google Calendar busy times
		overlaps := false
		for _, busy := range busySlots {
			if timesOverlap(slot.Start, slot.End, busy.Start, busy.End) {
				overlaps = true
				break
			}
		}
		if overlaps {
			continue
		}

		// Check existing bookings
		for _, booking := range existingBookings {
			if timesOverlap(slot.Start, slot.End, booking.StartTime, booking.EndTime) {
				overlaps = true
				break
			}
		}
		if !overlaps {
			available = append(available, slot)
		}
	}

	if available == nil {
		available = []entity.AvailableSlot{}
	}
	return available, nil
}

// BookSlot creates a booking for an available time slot
func (s *SchedulingService) BookSlot(ctx context.Context, slug string, input entity.BookingCreateInput) (*entity.SchedulingBooking, error) {
	page, err := s.repo.GetPageBySlug(ctx, slug)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrPageNotFound
		}
		return nil, fmt.Errorf("failed to load page: %w", err)
	}

	if !page.IsActive {
		return nil, ErrPageInactive
	}

	// Parse start time
	startTime, err := time.Parse(time.RFC3339, input.StartTime)
	if err != nil {
		return nil, fmt.Errorf("invalid startTime format; expected ISO 8601")
	}

	endTime := startTime.Add(time.Duration(page.DurationMinutes) * time.Minute)

	// Re-verify slot availability to prevent race conditions
	dateStr := startTime.In(mustLoadLocation(page.Timezone)).Format("2006-01-02")
	available, err := s.GetAvailableSlots(ctx, slug, dateStr)
	if err != nil {
		return nil, err
	}

	slotFound := false
	for _, slot := range available {
		if slot.Start.Equal(startTime) {
			slotFound = true
			break
		}
	}
	if !slotFound {
		return nil, ErrSlotNotAvailable
	}

	booking := &entity.SchedulingBooking{
		ID:               uuid.New().String(),
		OrgID:            page.OrgID,
		SchedulingPageID: page.ID,
		UserID:           page.UserID,
		GuestName:        input.GuestName,
		GuestEmail:       input.GuestEmail,
		GuestNotes:       input.GuestNotes,
		StartTime:        startTime,
		EndTime:          endTime,
		Status:           "confirmed",
	}

	// Create Google Calendar event if connected
	if s.gcalService != nil {
		eventID, err := s.gcalService.CreateEvent(ctx, page.OrgID, page.UserID, CalendarEvent{
			Summary:     fmt.Sprintf("Meeting with %s", input.GuestName),
			Description: fmt.Sprintf("Booked via QuanticoCRM scheduling\n\nGuest: %s <%s>", input.GuestName, input.GuestEmail),
			StartTime:   startTime,
			EndTime:     endTime,
			GuestEmail:  input.GuestEmail,
			GuestName:   input.GuestName,
			Timezone:    page.Timezone,
		})
		if err == nil {
			booking.GoogleEventID = eventID
		}
		// Non-fatal: log but continue if calendar event creation fails
	}

	if err := s.repo.CreateBooking(ctx, booking); err != nil {
		return nil, fmt.Errorf("failed to create booking: %w", err)
	}

	return booking, nil
}

// ========== Helper functions ==========

func validateAvailability(availability map[string][]entity.TimeWindow) error {
	validDays := map[string]bool{
		"monday": true, "tuesday": true, "wednesday": true, "thursday": true,
		"friday": true, "saturday": true, "sunday": true,
	}

	for day, windows := range availability {
		if !validDays[day] {
			return fmt.Errorf("invalid day: %s", day)
		}
		for _, w := range windows {
			if !timePattern.MatchString(w.Start) || !timePattern.MatchString(w.End) {
				return ErrInvalidTimeFormat
			}
			if w.Start >= w.End {
				return ErrInvalidTimeWindow
			}
		}
	}
	return nil
}

func unmarshalAvailability(jsonStr string) (map[string][]entity.TimeWindow, error) {
	var availability map[string][]entity.TimeWindow
	if jsonStr == "" || jsonStr == "{}" {
		return map[string][]entity.TimeWindow{}, nil
	}
	err := json.Unmarshal([]byte(jsonStr), &availability)
	return availability, err
}

func parseTimeOnDate(date time.Time, timeStr string, loc *time.Location) (time.Time, error) {
	var hour, minute int
	if _, err := fmt.Sscanf(timeStr, "%d:%d", &hour, &minute); err != nil {
		return time.Time{}, fmt.Errorf("invalid time: %s", timeStr)
	}
	return time.Date(date.Year(), date.Month(), date.Day(), hour, minute, 0, 0, loc), nil
}

func timesOverlap(start1, end1, start2, end2 time.Time) bool {
	return start1.Before(end2) && end1.After(start2)
}

func mustLoadLocation(timezone string) *time.Location {
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		return time.UTC
	}
	return loc
}
