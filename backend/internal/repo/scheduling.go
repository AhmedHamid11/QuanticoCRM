package repo

import (
	"context"
	"database/sql"
	"time"

	"github.com/fastcrm/backend/internal/db"
	"github.com/fastcrm/backend/internal/entity"
)

// SchedulingRepo handles database operations for scheduling tables
type SchedulingRepo struct {
	db db.DBConn
}

// NewSchedulingRepo creates a new SchedulingRepo
func NewSchedulingRepo(conn db.DBConn) *SchedulingRepo {
	return &SchedulingRepo{db: conn}
}

// WithDB returns a new SchedulingRepo using the specified database connection
func (r *SchedulingRepo) WithDB(conn db.DBConn) *SchedulingRepo {
	if conn == nil {
		return r
	}
	return &SchedulingRepo{db: conn}
}

// ========== Google Calendar Connection ==========

// GetGoogleConnection retrieves the Google Calendar connection for a user
func (r *SchedulingRepo) GetGoogleConnection(ctx context.Context, orgID, userID string) (*entity.GoogleCalendarConnection, error) {
	query := `
		SELECT id, org_id, user_id, access_token_encrypted, refresh_token_encrypted,
		       token_expiry, calendar_id, connected_at, created_at, updated_at
		FROM google_calendar_connections
		WHERE org_id = ? AND user_id = ?
	`
	row := r.db.QueryRowContext(ctx, query, orgID, userID)

	var conn entity.GoogleCalendarConnection
	var tokenExpiry, connectedAt, createdAt, updatedAt sql.NullString
	err := row.Scan(
		&conn.ID, &conn.OrgID, &conn.UserID,
		&conn.AccessTokenEncrypted, &conn.RefreshTokenEncrypted,
		&tokenExpiry, &conn.CalendarID, &connectedAt, &createdAt, &updatedAt,
	)
	if err != nil {
		return nil, err
	}

	if tokenExpiry.Valid {
		t, err := time.Parse("2006-01-02T15:04:05Z", tokenExpiry.String)
		if err == nil {
			conn.TokenExpiry = &t
		}
	}
	if connectedAt.Valid {
		t, err := time.Parse("2006-01-02T15:04:05Z", connectedAt.String)
		if err == nil {
			conn.ConnectedAt = &t
		}
	}

	return &conn, nil
}

// UpsertGoogleConnection inserts or updates a Google Calendar connection
func (r *SchedulingRepo) UpsertGoogleConnection(ctx context.Context, conn *entity.GoogleCalendarConnection) error {
	query := `
		INSERT INTO google_calendar_connections
		    (id, org_id, user_id, access_token_encrypted, refresh_token_encrypted,
		     token_expiry, calendar_id, connected_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		ON CONFLICT(org_id, user_id) DO UPDATE SET
		    access_token_encrypted = excluded.access_token_encrypted,
		    refresh_token_encrypted = excluded.refresh_token_encrypted,
		    token_expiry = excluded.token_expiry,
		    calendar_id = excluded.calendar_id,
		    connected_at = excluded.connected_at,
		    updated_at = CURRENT_TIMESTAMP
	`

	var tokenExpiry interface{}
	if conn.TokenExpiry != nil {
		tokenExpiry = conn.TokenExpiry.UTC().Format("2006-01-02T15:04:05Z")
	}

	var connectedAt interface{}
	if conn.ConnectedAt != nil {
		connectedAt = conn.ConnectedAt.UTC().Format("2006-01-02T15:04:05Z")
	}

	_, err := r.db.ExecContext(ctx, query,
		conn.ID, conn.OrgID, conn.UserID,
		conn.AccessTokenEncrypted, conn.RefreshTokenEncrypted,
		tokenExpiry, conn.CalendarID, connectedAt,
	)
	return err
}

// DeleteGoogleConnection removes a Google Calendar connection
func (r *SchedulingRepo) DeleteGoogleConnection(ctx context.Context, orgID, userID string) error {
	_, err := r.db.ExecContext(ctx,
		"DELETE FROM google_calendar_connections WHERE org_id = ? AND user_id = ?",
		orgID, userID,
	)
	return err
}

// ========== Scheduling Pages ==========

// CreatePage inserts a new scheduling page
func (r *SchedulingRepo) CreatePage(ctx context.Context, page *entity.SchedulingPage) error {
	isActiveInt := 0
	if page.IsActive {
		isActiveInt = 1
	}
	query := `
		INSERT INTO scheduling_pages
		    (id, org_id, user_id, slug, title, description, duration_minutes,
		     availability_json, timezone, is_active, buffer_minutes, max_days_ahead,
		     created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`
	_, err := r.db.ExecContext(ctx, query,
		page.ID, page.OrgID, page.UserID, page.Slug, page.Title, page.Description,
		page.DurationMinutes, page.AvailabilityJSON, page.Timezone,
		isActiveInt, page.BufferMinutes, page.MaxDaysAhead,
	)
	return err
}

// UpdatePage updates an existing scheduling page
func (r *SchedulingRepo) UpdatePage(ctx context.Context, page *entity.SchedulingPage) error {
	isActiveInt := 0
	if page.IsActive {
		isActiveInt = 1
	}
	query := `
		UPDATE scheduling_pages SET
		    slug = ?, title = ?, description = ?, duration_minutes = ?,
		    availability_json = ?, timezone = ?, is_active = ?,
		    buffer_minutes = ?, max_days_ahead = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ? AND org_id = ?
	`
	_, err := r.db.ExecContext(ctx, query,
		page.Slug, page.Title, page.Description, page.DurationMinutes,
		page.AvailabilityJSON, page.Timezone, isActiveInt,
		page.BufferMinutes, page.MaxDaysAhead,
		page.ID, page.OrgID,
	)
	return err
}

// GetPage retrieves a scheduling page by ID
func (r *SchedulingRepo) GetPage(ctx context.Context, id string) (*entity.SchedulingPage, error) {
	query := `
		SELECT id, org_id, user_id, slug, title, description, duration_minutes,
		       availability_json, timezone, is_active, buffer_minutes, max_days_ahead,
		       created_at, updated_at
		FROM scheduling_pages WHERE id = ?
	`
	return r.scanPage(r.db.QueryRowContext(ctx, query, id))
}

// GetPageBySlug retrieves a scheduling page by slug (globally unique, no org filter)
func (r *SchedulingRepo) GetPageBySlug(ctx context.Context, slug string) (*entity.SchedulingPage, error) {
	query := `
		SELECT id, org_id, user_id, slug, title, description, duration_minutes,
		       availability_json, timezone, is_active, buffer_minutes, max_days_ahead,
		       created_at, updated_at
		FROM scheduling_pages WHERE slug = ?
	`
	return r.scanPage(r.db.QueryRowContext(ctx, query, slug))
}

// ListPagesByUser retrieves all scheduling pages for a user
func (r *SchedulingRepo) ListPagesByUser(ctx context.Context, orgID, userID string) ([]*entity.SchedulingPage, error) {
	query := `
		SELECT id, org_id, user_id, slug, title, description, duration_minutes,
		       availability_json, timezone, is_active, buffer_minutes, max_days_ahead,
		       created_at, updated_at
		FROM scheduling_pages WHERE org_id = ? AND user_id = ?
		ORDER BY created_at DESC
	`
	rows, err := r.db.QueryContext(ctx, query, orgID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var pages []*entity.SchedulingPage
	for rows.Next() {
		page, err := r.scanPageFromRows(rows)
		if err != nil {
			return nil, err
		}
		pages = append(pages, page)
	}
	return pages, rows.Err()
}

// DeletePage removes a scheduling page
func (r *SchedulingRepo) DeletePage(ctx context.Context, id, orgID string) error {
	_, err := r.db.ExecContext(ctx,
		"DELETE FROM scheduling_pages WHERE id = ? AND org_id = ?",
		id, orgID,
	)
	return err
}

// ========== Scheduling Bookings ==========

// CreateBooking inserts a new booking
func (r *SchedulingRepo) CreateBooking(ctx context.Context, booking *entity.SchedulingBooking) error {
	query := `
		INSERT INTO scheduling_bookings
		    (id, org_id, scheduling_page_id, user_id, guest_name, guest_email,
		     guest_notes, start_time, end_time, status, google_event_id, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
	`
	_, err := r.db.ExecContext(ctx, query,
		booking.ID, booking.OrgID, booking.SchedulingPageID, booking.UserID,
		booking.GuestName, booking.GuestEmail, booking.GuestNotes,
		booking.StartTime.UTC().Format("2006-01-02T15:04:05Z"),
		booking.EndTime.UTC().Format("2006-01-02T15:04:05Z"),
		booking.Status, booking.GoogleEventID,
	)
	return err
}

// ListBookingsByPage retrieves all bookings for a scheduling page
func (r *SchedulingRepo) ListBookingsByPage(ctx context.Context, pageID string) ([]*entity.SchedulingBooking, error) {
	query := `
		SELECT id, org_id, scheduling_page_id, user_id, guest_name, guest_email,
		       guest_notes, start_time, end_time, status, COALESCE(google_event_id, ''), created_at
		FROM scheduling_bookings WHERE scheduling_page_id = ?
		ORDER BY start_time DESC
	`
	rows, err := r.db.QueryContext(ctx, query, pageID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var bookings []*entity.SchedulingBooking
	for rows.Next() {
		b, err := r.scanBookingFromRows(rows)
		if err != nil {
			return nil, err
		}
		bookings = append(bookings, b)
	}
	return bookings, rows.Err()
}

// ListBookingsByUserInRange retrieves bookings for a user within a time range
func (r *SchedulingRepo) ListBookingsByUserInRange(ctx context.Context, userID, orgID string, start, end time.Time) ([]*entity.SchedulingBooking, error) {
	query := `
		SELECT id, org_id, scheduling_page_id, user_id, guest_name, guest_email,
		       guest_notes, start_time, end_time, status, COALESCE(google_event_id, ''), created_at
		FROM scheduling_bookings
		WHERE user_id = ? AND org_id = ? AND status = 'confirmed'
		  AND start_time < ? AND end_time > ?
		ORDER BY start_time
	`
	rows, err := r.db.QueryContext(ctx, query, userID, orgID,
		end.UTC().Format("2006-01-02T15:04:05Z"),
		start.UTC().Format("2006-01-02T15:04:05Z"),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var bookings []*entity.SchedulingBooking
	for rows.Next() {
		b, err := r.scanBookingFromRows(rows)
		if err != nil {
			return nil, err
		}
		bookings = append(bookings, b)
	}
	return bookings, rows.Err()
}

// GetBooking retrieves a booking by ID
func (r *SchedulingRepo) GetBooking(ctx context.Context, id string) (*entity.SchedulingBooking, error) {
	query := `
		SELECT id, org_id, scheduling_page_id, user_id, guest_name, guest_email,
		       guest_notes, start_time, end_time, status, COALESCE(google_event_id, ''), created_at
		FROM scheduling_bookings WHERE id = ?
	`
	row := r.db.QueryRowContext(ctx, query, id)
	return r.scanBooking(row)
}

// ========== Scanner helpers ==========

type rowScanner interface {
	Scan(dest ...interface{}) error
}

func (r *SchedulingRepo) scanPage(row rowScanner) (*entity.SchedulingPage, error) {
	var page entity.SchedulingPage
	var isActiveInt int
	var createdAt, updatedAt string
	err := row.Scan(
		&page.ID, &page.OrgID, &page.UserID, &page.Slug, &page.Title,
		&page.Description, &page.DurationMinutes, &page.AvailabilityJSON,
		&page.Timezone, &isActiveInt, &page.BufferMinutes, &page.MaxDaysAhead,
		&createdAt, &updatedAt,
	)
	if err != nil {
		return nil, err
	}
	page.IsActive = isActiveInt == 1
	return &page, nil
}

func (r *SchedulingRepo) scanPageFromRows(rows *sql.Rows) (*entity.SchedulingPage, error) {
	var page entity.SchedulingPage
	var isActiveInt int
	var createdAt, updatedAt string
	err := rows.Scan(
		&page.ID, &page.OrgID, &page.UserID, &page.Slug, &page.Title,
		&page.Description, &page.DurationMinutes, &page.AvailabilityJSON,
		&page.Timezone, &isActiveInt, &page.BufferMinutes, &page.MaxDaysAhead,
		&createdAt, &updatedAt,
	)
	if err != nil {
		return nil, err
	}
	page.IsActive = isActiveInt == 1
	return &page, nil
}

func (r *SchedulingRepo) scanBooking(row rowScanner) (*entity.SchedulingBooking, error) {
	var b entity.SchedulingBooking
	var startTime, endTime, createdAt string
	err := row.Scan(
		&b.ID, &b.OrgID, &b.SchedulingPageID, &b.UserID,
		&b.GuestName, &b.GuestEmail, &b.GuestNotes,
		&startTime, &endTime, &b.Status, &b.GoogleEventID, &createdAt,
	)
	if err != nil {
		return nil, err
	}
	b.StartTime, _ = time.Parse("2006-01-02T15:04:05Z", startTime)
	b.EndTime, _ = time.Parse("2006-01-02T15:04:05Z", endTime)
	b.CreatedAt, _ = time.Parse("2006-01-02T15:04:05Z", createdAt)
	return &b, nil
}

func (r *SchedulingRepo) scanBookingFromRows(rows *sql.Rows) (*entity.SchedulingBooking, error) {
	var b entity.SchedulingBooking
	var startTime, endTime, createdAt string
	err := rows.Scan(
		&b.ID, &b.OrgID, &b.SchedulingPageID, &b.UserID,
		&b.GuestName, &b.GuestEmail, &b.GuestNotes,
		&startTime, &endTime, &b.Status, &b.GoogleEventID, &createdAt,
	)
	if err != nil {
		return nil, err
	}
	b.StartTime, _ = time.Parse("2006-01-02T15:04:05Z", startTime)
	b.EndTime, _ = time.Parse("2006-01-02T15:04:05Z", endTime)
	b.CreatedAt, _ = time.Parse("2006-01-02T15:04:05Z", createdAt)
	return &b, nil
}
