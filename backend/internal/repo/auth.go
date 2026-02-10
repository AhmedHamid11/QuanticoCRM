package repo

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/fastcrm/backend/internal/db"
	"github.com/fastcrm/backend/internal/entity"
	"github.com/fastcrm/backend/internal/sfid"
)

// AuthRepo handles database operations for authentication
type AuthRepo struct {
	db db.DBConn
}

// NewAuthRepo creates a new AuthRepo
// Accepts db.DBConn interface which both *sql.DB and *db.TursoDB satisfy
func NewAuthRepo(conn db.DBConn) *AuthRepo {
	return &AuthRepo{db: conn}
}

// WithDB returns a new AuthRepo instance with the specified database connection
func (r *AuthRepo) WithDB(conn db.DBConn) *AuthRepo {
	return &AuthRepo{db: conn}
}

// parseTimestamp parses SQLite TEXT timestamps into time.Time
func parseTimestamp(s sql.NullString) time.Time {
	if !s.Valid {
		return time.Time{}
	}
	// Try multiple formats in order of likelihood
	formats := []string{
		time.RFC3339,                      // 2006-01-02T15:04:05Z07:00
		time.RFC3339Nano,                  // 2006-01-02T15:04:05.999999999Z07:00
		"2006-01-02 15:04:05.999999-07:00", // SQLite with microseconds and timezone
		"2006-01-02 15:04:05-07:00",       // SQLite with timezone
		"2006-01-02 15:04:05.999999+00:00", // UTC with microseconds
		"2006-01-02 15:04:05+00:00",       // UTC without microseconds
		"2006-01-02 15:04:05",             // Basic SQLite format
		"2006-01-02T15:04:05Z",            // ISO format with Z
	}
	for _, format := range formats {
		if t, err := time.Parse(format, s.String); err == nil {
			return t
		}
	}
	return time.Time{}
}

// parseTimestampPtr parses SQLite TEXT timestamps into *time.Time (for nullable fields)
func parseTimestampPtr(s sql.NullString) *time.Time {
	if !s.Valid {
		return nil
	}
	t := parseTimestamp(s)
	if t.IsZero() {
		return nil
	}
	return &t
}

// --- Organization Operations ---

// CreateOrganization creates a new organization
func (r *AuthRepo) CreateOrganization(ctx context.Context, input entity.OrganizationCreateInput) (*entity.Organization, error) {
	org := &entity.Organization{
		ID:             sfid.NewOrg(),
		Name:           input.Name,
		Slug:           input.Slug,
		CurrentVersion: input.CurrentVersion,
		Plan:           "free",
		IsActive:       true,
		Settings:       "{}",
		CreatedAt:      time.Now().UTC(),
		ModifiedAt:     time.Now().UTC(),
	}

	query := `
		INSERT INTO organizations (id, name, slug, current_version, plan, is_active, settings, database_url, database_token, database_name, created_at, modified_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := r.db.ExecContext(ctx, query,
		org.ID, org.Name, org.Slug, org.CurrentVersion, org.Plan, org.IsActive, org.Settings,
		org.DatabaseURL, org.DatabaseToken, org.DatabaseName,
		org.CreatedAt, org.ModifiedAt,
	)
	if err != nil {
		return nil, err
	}

	return org, nil
}

// CreateOrganizationWithDatabase creates an organization with database credentials
func (r *AuthRepo) CreateOrganizationWithDatabase(ctx context.Context, input entity.OrganizationCreateInput, dbURL, dbToken, dbName string) (*entity.Organization, error) {
	org := &entity.Organization{
		ID:             sfid.NewOrg(),
		Name:           input.Name,
		Slug:           input.Slug,
		CurrentVersion: input.CurrentVersion,
		Plan:           "free",
		IsActive:       true,
		Settings:       "{}",
		DatabaseURL:    dbURL,
		DatabaseToken:  dbToken,
		DatabaseName:   dbName,
		CreatedAt:      time.Now().UTC(),
		ModifiedAt:     time.Now().UTC(),
	}

	query := `
		INSERT INTO organizations (id, name, slug, current_version, plan, is_active, settings, database_url, database_token, database_name, created_at, modified_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := r.db.ExecContext(ctx, query,
		org.ID, org.Name, org.Slug, org.CurrentVersion, org.Plan, org.IsActive, org.Settings,
		org.DatabaseURL, org.DatabaseToken, org.DatabaseName,
		org.CreatedAt, org.ModifiedAt,
	)
	if err != nil {
		return nil, err
	}

	return org, nil
}

// UpdateOrganizationDatabase updates an organization's database credentials
func (r *AuthRepo) UpdateOrganizationDatabase(ctx context.Context, orgID, dbURL, dbToken, dbName string) error {
	query := `UPDATE organizations SET database_url = ?, database_token = ?, database_name = ?, modified_at = ? WHERE id = ?`
	_, err := r.db.ExecContext(ctx, query, dbURL, dbToken, dbName, time.Now().UTC(), orgID)
	return err
}

// GetOrganizationByID retrieves an organization by ID
func (r *AuthRepo) GetOrganizationByID(ctx context.Context, id string) (*entity.Organization, error) {
	org := &entity.Organization{}
	var createdAt, modifiedAt sql.NullString
	var dbURL, dbToken, dbName sql.NullString
	query := `SELECT id, name, slug, plan, is_active, settings, database_url, database_token, database_name, created_at, modified_at FROM organizations WHERE id = ?`

	// Use QueryContext for retry support instead of QueryRowContext
	rows, err := r.db.QueryContext(ctx, query, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	if !rows.Next() {
		return nil, sql.ErrNoRows
	}

	if err := rows.Scan(
		&org.ID, &org.Name, &org.Slug, &org.Plan, &org.IsActive, &org.Settings,
		&dbURL, &dbToken, &dbName,
		&createdAt, &modifiedAt,
	); err != nil {
		return nil, err
	}

	org.DatabaseURL = dbURL.String
	org.DatabaseToken = dbToken.String
	org.DatabaseName = dbName.String
	org.CreatedAt = parseTimestamp(createdAt)
	org.ModifiedAt = parseTimestamp(modifiedAt)

	return org, nil
}

// GetOrganizationBySlug retrieves an organization by slug
func (r *AuthRepo) GetOrganizationBySlug(ctx context.Context, slug string) (*entity.Organization, error) {
	org := &entity.Organization{}
	var createdAt, modifiedAt sql.NullString
	var dbURL, dbToken, dbName sql.NullString
	query := `SELECT id, name, slug, plan, is_active, settings, database_url, database_token, database_name, created_at, modified_at FROM organizations WHERE slug = ?`

	err := r.db.QueryRowContext(ctx, query, slug).Scan(
		&org.ID, &org.Name, &org.Slug, &org.Plan, &org.IsActive, &org.Settings,
		&dbURL, &dbToken, &dbName,
		&createdAt, &modifiedAt,
	)
	if err != nil {
		return nil, err
	}

	org.DatabaseURL = dbURL.String
	org.DatabaseToken = dbToken.String
	org.DatabaseName = dbName.String
	org.CreatedAt = parseTimestamp(createdAt)
	org.ModifiedAt = parseTimestamp(modifiedAt)

	return org, nil
}

// UpdateOrganization updates an organization's details (for platform admins)
func (r *AuthRepo) UpdateOrganization(ctx context.Context, id string, input entity.OrganizationUpdateInput) (*entity.Organization, error) {
	// Build dynamic update query
	updates := []string{}
	args := []interface{}{}

	if input.Name != nil {
		updates = append(updates, "name = ?")
		args = append(args, *input.Name)
	}
	if input.Plan != nil {
		updates = append(updates, "plan = ?")
		args = append(args, *input.Plan)
	}
	if input.IsActive != nil {
		updates = append(updates, "is_active = ?")
		args = append(args, *input.IsActive)
	}
	if input.Settings != nil {
		updates = append(updates, "settings = ?")
		args = append(args, *input.Settings)
	}

	if len(updates) == 0 {
		return r.GetOrganizationByID(ctx, id)
	}

	updates = append(updates, "modified_at = datetime('now')")
	args = append(args, id)

	query := "UPDATE organizations SET " + strings.Join(updates, ", ") + " WHERE id = ?"
	_, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}

	return r.GetOrganizationByID(ctx, id)
}

// ListOrganizations lists all organizations (for platform admins)
func (r *AuthRepo) ListOrganizations(ctx context.Context, page, pageSize int) (*entity.OrganizationListResponse, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	// Get total count - using QueryContext for retry support
	var total int
	countQuery := `SELECT COUNT(*) FROM organizations`
	countRows, err := r.db.QueryContext(ctx, countQuery)
	if err != nil {
		return nil, fmt.Errorf("failed to count organizations: %w", err)
	}
	if countRows.Next() {
		if err := countRows.Scan(&total); err != nil {
			countRows.Close()
			return nil, fmt.Errorf("failed to scan count: %w", err)
		}
	}
	countRows.Close()

	// Get organizations
	query := `
		SELECT id, name, slug, plan, is_active, settings, created_at, modified_at
		FROM organizations
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?
	`

	rows, err := r.db.QueryContext(ctx, query, pageSize, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var orgs []entity.Organization
	for rows.Next() {
		var org entity.Organization
		var createdAt, modifiedAt sql.NullString
		if err := rows.Scan(&org.ID, &org.Name, &org.Slug, &org.Plan, &org.IsActive, &org.Settings, &createdAt, &modifiedAt); err != nil {
			return nil, err
		}
		org.CreatedAt = parseTimestamp(createdAt)
		org.ModifiedAt = parseTimestamp(modifiedAt)
		orgs = append(orgs, org)
	}

	totalPages := (total + pageSize - 1) / pageSize

	return &entity.OrganizationListResponse{
		Data:       orgs,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	}, nil
}

// --- User Operations ---

// CreateUser creates a new user
func (r *AuthRepo) CreateUser(ctx context.Context, email, passwordHash, firstName, lastName string) (*entity.User, error) {
	user := &entity.User{
		ID:              sfid.NewUser(),
		Email:           email,
		PasswordHash:    passwordHash,
		FirstName:       firstName,
		LastName:        lastName,
		IsActive:        true,
		IsPlatformAdmin: false,
		EmailVerified:   false,
		CreatedAt:       time.Now().UTC(),
		ModifiedAt:      time.Now().UTC(),
	}

	query := `
		INSERT INTO users (id, email, password_hash, first_name, last_name, is_active, is_platform_admin, email_verified, created_at, modified_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := r.db.ExecContext(ctx, query,
		user.ID, user.Email, user.PasswordHash, user.FirstName, user.LastName,
		user.IsActive, user.IsPlatformAdmin, user.EmailVerified, user.CreatedAt, user.ModifiedAt,
	)
	if err != nil {
		return nil, err
	}

	return user, nil
}

// GetUserByID retrieves a user by ID
func (r *AuthRepo) GetUserByID(ctx context.Context, id string) (*entity.User, error) {
	user := &entity.User{}
	var lastLoginAt, createdAt, modifiedAt sql.NullString
	query := `
		SELECT id, email, password_hash, first_name, last_name, is_active, is_platform_admin, email_verified, last_login_at, created_at, modified_at
		FROM users WHERE id = ?
	`

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&user.ID, &user.Email, &user.PasswordHash, &user.FirstName, &user.LastName,
		&user.IsActive, &user.IsPlatformAdmin, &user.EmailVerified, &lastLoginAt, &createdAt, &modifiedAt,
	)
	if err != nil {
		return nil, err
	}

	user.LastLoginAt = parseTimestampPtr(lastLoginAt)
	user.CreatedAt = parseTimestamp(createdAt)
	user.ModifiedAt = parseTimestamp(modifiedAt)

	return user, nil
}

// GetUserByEmail retrieves a user by email
func (r *AuthRepo) GetUserByEmail(ctx context.Context, email string) (*entity.User, error) {
	user := &entity.User{}
	var lastLoginAt, createdAt, modifiedAt sql.NullString
	query := `
		SELECT id, email, password_hash, first_name, last_name, is_active, is_platform_admin, email_verified, last_login_at, created_at, modified_at
		FROM users WHERE email = ?
	`

	err := r.db.QueryRowContext(ctx, query, email).Scan(
		&user.ID, &user.Email, &user.PasswordHash, &user.FirstName, &user.LastName,
		&user.IsActive, &user.IsPlatformAdmin, &user.EmailVerified, &lastLoginAt, &createdAt, &modifiedAt,
	)
	if err != nil {
		return nil, err
	}

	// Parse timestamps
	user.LastLoginAt = parseTimestampPtr(lastLoginAt)
	user.CreatedAt = parseTimestamp(createdAt)
	user.ModifiedAt = parseTimestamp(modifiedAt)

	return user, nil
}

// UpdateUserLastLogin updates the last login timestamp
func (r *AuthRepo) UpdateUserLastLogin(ctx context.Context, userID string) error {
	query := `UPDATE users SET last_login_at = ?, modified_at = ? WHERE id = ?`
	now := time.Now().UTC()
	_, err := r.db.ExecContext(ctx, query, now, now, userID)
	return err
}

// UpdateUserPassword updates the user's password hash
func (r *AuthRepo) UpdateUserPassword(ctx context.Context, userID, passwordHash string) error {
	query := `UPDATE users SET password_hash = ?, modified_at = ? WHERE id = ?`
	_, err := r.db.ExecContext(ctx, query, passwordHash, time.Now().UTC(), userID)
	return err
}

// UpdateUserActiveStatus activates or deactivates a user
func (r *AuthRepo) UpdateUserActiveStatus(ctx context.Context, userID string, isActive bool) error {
	query := `UPDATE users SET is_active = ?, modified_at = ? WHERE id = ?`
	activeVal := 0
	if isActive {
		activeVal = 1
	}
	_, err := r.db.ExecContext(ctx, query, activeVal, time.Now().UTC(), userID)
	return err
}

// ListUsersByOrg lists all users in an organization
func (r *AuthRepo) ListUsersByOrg(ctx context.Context, orgID string, page, pageSize int) (*entity.UserListResponse, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	// Get total count
	var total int
	countQuery := `SELECT COUNT(*) FROM user_org_memberships WHERE org_id = ?`
	if err := r.db.QueryRowContext(ctx, countQuery, orgID).Scan(&total); err != nil {
		return nil, err
	}

	// Get users with their membership info
	query := `
		SELECT u.id, u.email, u.password_hash, u.first_name, u.last_name, u.is_active, u.is_platform_admin,
		       u.email_verified, u.last_login_at, u.created_at, u.modified_at, m.role, m.joined_at
		FROM users u
		INNER JOIN user_org_memberships m ON u.id = m.user_id
		WHERE m.org_id = ?
		ORDER BY m.joined_at DESC
		LIMIT ? OFFSET ?
	`

	rows, err := r.db.QueryContext(ctx, query, orgID, pageSize, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []entity.UserWithMembership
	for rows.Next() {
		var u entity.UserWithMembership
		var lastLoginAt, createdAt, modifiedAt, joinedAt sql.NullString
		if err := rows.Scan(
			&u.ID, &u.Email, &u.PasswordHash, &u.FirstName, &u.LastName, &u.IsActive, &u.IsPlatformAdmin,
			&u.EmailVerified, &lastLoginAt, &createdAt, &modifiedAt, &u.Role, &joinedAt,
		); err != nil {
			return nil, err
		}
		u.LastLoginAt = parseTimestampPtr(lastLoginAt)
		u.CreatedAt = parseTimestamp(createdAt)
		u.ModifiedAt = parseTimestamp(modifiedAt)
		u.JoinedAt = parseTimestamp(joinedAt)
		users = append(users, u)
	}

	totalPages := (total + pageSize - 1) / pageSize

	return &entity.UserListResponse{
		Data:       users,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	}, nil
}

// --- Membership Operations ---

// CreateMembership creates a new user-org membership
func (r *AuthRepo) CreateMembership(ctx context.Context, userID, orgID, role string, isDefault bool) (*entity.Membership, error) {
	membership := &entity.Membership{
		ID:        sfid.NewMembership(),
		UserID:    userID,
		OrgID:     orgID,
		Role:      role,
		IsDefault: isDefault,
		JoinedAt:  time.Now().UTC(),
	}

	query := `
		INSERT INTO user_org_memberships (id, user_id, org_id, role, is_default, joined_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`

	_, err := r.db.ExecContext(ctx, query,
		membership.ID, membership.UserID, membership.OrgID, membership.Role, membership.IsDefault, membership.JoinedAt,
	)
	if err != nil {
		return nil, err
	}

	return membership, nil
}

// GetMembership retrieves a user's membership in an organization
func (r *AuthRepo) GetMembership(ctx context.Context, userID, orgID string) (*entity.Membership, error) {
	membership := &entity.Membership{}
	var joinedAt sql.NullString
	query := `SELECT id, user_id, org_id, role, is_default, joined_at FROM user_org_memberships WHERE user_id = ? AND org_id = ?`

	err := r.db.QueryRowContext(ctx, query, userID, orgID).Scan(
		&membership.ID, &membership.UserID, &membership.OrgID, &membership.Role, &membership.IsDefault, &joinedAt,
	)
	if err != nil {
		return nil, err
	}

	membership.JoinedAt = parseTimestamp(joinedAt)
	return membership, nil
}

// GetUserMemberships retrieves all organizations a user belongs to
func (r *AuthRepo) GetUserMemberships(ctx context.Context, userID string) ([]entity.MembershipWithOrg, error) {
	query := `
		SELECT m.id, m.user_id, m.org_id, m.role, m.is_default, m.joined_at, o.name, o.slug
		FROM user_org_memberships m
		INNER JOIN organizations o ON m.org_id = o.id
		WHERE m.user_id = ?
		ORDER BY m.is_default DESC, m.joined_at ASC
	`

	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var memberships []entity.MembershipWithOrg
	for rows.Next() {
		var m entity.MembershipWithOrg
		var joinedAt sql.NullString
		if err := rows.Scan(&m.ID, &m.UserID, &m.OrgID, &m.Role, &m.IsDefault, &joinedAt, &m.OrgName, &m.OrgSlug); err != nil {
			return nil, err
		}
		m.JoinedAt = parseTimestamp(joinedAt)
		memberships = append(memberships, m)
	}

	return memberships, nil
}

// GetDefaultMembership retrieves the user's default organization membership
func (r *AuthRepo) GetDefaultMembership(ctx context.Context, userID string) (*entity.MembershipWithOrg, error) {
	membership := &entity.MembershipWithOrg{}
	var joinedAt sql.NullString
	query := `
		SELECT m.id, m.user_id, m.org_id, m.role, m.is_default, m.joined_at, o.name, o.slug
		FROM user_org_memberships m
		INNER JOIN organizations o ON m.org_id = o.id
		WHERE m.user_id = ? AND m.is_default = 1
	`

	err := r.db.QueryRowContext(ctx, query, userID).Scan(
		&membership.ID, &membership.UserID, &membership.OrgID, &membership.Role, &membership.IsDefault, &joinedAt,
		&membership.OrgName, &membership.OrgSlug,
	)
	if err != nil {
		// If no default, get the first membership
		if err == sql.ErrNoRows {
			query = `
				SELECT m.id, m.user_id, m.org_id, m.role, m.is_default, m.joined_at, o.name, o.slug
				FROM user_org_memberships m
				INNER JOIN organizations o ON m.org_id = o.id
				WHERE m.user_id = ?
				ORDER BY m.joined_at ASC
				LIMIT 1
			`
			err = r.db.QueryRowContext(ctx, query, userID).Scan(
				&membership.ID, &membership.UserID, &membership.OrgID, &membership.Role, &membership.IsDefault, &joinedAt,
				&membership.OrgName, &membership.OrgSlug,
			)
		}
		if err != nil {
			return nil, err
		}
	}

	// Parse timestamp
	membership.JoinedAt = parseTimestamp(joinedAt)

	return membership, nil
}

// SetDefaultMembership sets the default organization for a user
func (r *AuthRepo) SetDefaultMembership(ctx context.Context, userID, orgID string) error {
	// First, unset all defaults for this user
	_, err := r.db.ExecContext(ctx, `UPDATE user_org_memberships SET is_default = 0 WHERE user_id = ?`, userID)
	if err != nil {
		return err
	}

	// Then set the new default
	_, err = r.db.ExecContext(ctx, `UPDATE user_org_memberships SET is_default = 1 WHERE user_id = ? AND org_id = ?`, userID, orgID)
	return err
}

// DeleteMembership removes a user from an organization
func (r *AuthRepo) DeleteMembership(ctx context.Context, userID, orgID string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM user_org_memberships WHERE user_id = ? AND org_id = ?`, userID, orgID)
	return err
}

// UpdateMembershipRole updates the role for a user's membership in an organization
func (r *AuthRepo) UpdateMembershipRole(ctx context.Context, userID, orgID, role string) error {
	query := `UPDATE user_org_memberships SET role = ? WHERE user_id = ? AND org_id = ?`
	result, err := r.db.ExecContext(ctx, query, role, userID, orgID)
	if err != nil {
		return err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// GetUserByIDInOrg retrieves a user with their membership in a specific organization
func (r *AuthRepo) GetUserByIDInOrg(ctx context.Context, userID, orgID string) (*entity.UserWithMembership, error) {
	var u entity.UserWithMembership
	var lastLoginAt, createdAt, modifiedAt, joinedAt sql.NullString

	query := `
		SELECT u.id, u.email, u.password_hash, u.first_name, u.last_name, u.is_active, u.is_platform_admin,
		       u.email_verified, u.last_login_at, u.created_at, u.modified_at, m.role, m.joined_at
		FROM users u
		INNER JOIN user_org_memberships m ON u.id = m.user_id
		WHERE u.id = ? AND m.org_id = ?
	`

	err := r.db.QueryRowContext(ctx, query, userID, orgID).Scan(
		&u.ID, &u.Email, &u.PasswordHash, &u.FirstName, &u.LastName, &u.IsActive, &u.IsPlatformAdmin,
		&u.EmailVerified, &lastLoginAt, &createdAt, &modifiedAt, &u.Role, &joinedAt,
	)
	if err != nil {
		return nil, err
	}

	u.LastLoginAt = parseTimestampPtr(lastLoginAt)
	u.CreatedAt = parseTimestamp(createdAt)
	u.ModifiedAt = parseTimestamp(modifiedAt)
	u.JoinedAt = parseTimestamp(joinedAt)

	return &u, nil
}

// CountOrgOwners counts the number of owners in an organization
func (r *AuthRepo) CountOrgOwners(ctx context.Context, orgID string) (int, error) {
	var count int
	query := `SELECT COUNT(*) FROM user_org_memberships WHERE org_id = ? AND role = 'owner'`
	err := r.db.QueryRowContext(ctx, query, orgID).Scan(&count)
	return count, err
}

// CountOrgMembers counts all members in an organization (for tier enforcement)
func (r *AuthRepo) CountOrgMembers(ctx context.Context, orgID string) (int, error) {
	var count int
	query := `SELECT COUNT(*) FROM user_org_memberships WHERE org_id = ?`
	err := r.db.QueryRowContext(ctx, query, orgID).Scan(&count)
	return count, err
}

// CountPendingInvitations counts pending (non-accepted) invitations for an organization
func (r *AuthRepo) CountPendingInvitations(ctx context.Context, orgID string) (int, error) {
	var count int
	query := `SELECT COUNT(*) FROM org_invitations WHERE org_id = ? AND accepted_at IS NULL AND expires_at > datetime('now')`
	err := r.db.QueryRowContext(ctx, query, orgID).Scan(&count)
	return count, err
}

// --- Session Operations ---

// CreateSession creates a new session with auto-generated family ID
func (r *AuthRepo) CreateSession(ctx context.Context, userID, orgID, refreshTokenHash, userAgent, ipAddress string, isImpersonation bool, impersonatedBy *string, expiresAt time.Time) (*entity.Session, error) {
	// Auto-generate family ID for new sessions (login starts a new family)
	return r.CreateSessionWithFamily(ctx, userID, orgID, refreshTokenHash, "", userAgent, ipAddress, isImpersonation, impersonatedBy, expiresAt)
}

// CreateSessionWithFamily creates a session with an explicit family ID (for token rotation)
// If familyID is empty, a new family is generated
func (r *AuthRepo) CreateSessionWithFamily(ctx context.Context, userID, orgID, refreshTokenHash, familyID, userAgent, ipAddress string, isImpersonation bool, impersonatedBy *string, expiresAt time.Time) (*entity.Session, error) {
	if familyID == "" {
		familyID = sfid.NewTokenFamily()
	}

	now := time.Now().UTC()
	session := &entity.Session{
		ID:                     sfid.NewSession(),
		UserID:                 userID,
		OrgID:                  orgID,
		RefreshTokenHash:       refreshTokenHash,
		UserAgent:              userAgent,
		IPAddress:              ipAddress,
		IsImpersonation:        isImpersonation,
		ImpersonatedBy:         impersonatedBy,
		FamilyID:               familyID,
		IsRevoked:              false,
		ExpiresAt:              expiresAt,
		CreatedAt:              now,
		LastActivityAt:         now, // Initialize activity timestamp
		IdleTimeoutMinutes:     30,  // Default 30 min idle timeout
		AbsoluteTimeoutMinutes: 1440, // Default 24 hour absolute timeout
	}

	query := `
		INSERT INTO sessions (id, user_id, org_id, refresh_token_hash, user_agent, ip_address, is_impersonation, impersonated_by, family_id, is_revoked, expires_at, created_at, last_activity_at, idle_timeout_minutes, absolute_timeout_minutes)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := r.db.ExecContext(ctx, query,
		session.ID, session.UserID, session.OrgID, session.RefreshTokenHash, session.UserAgent, session.IPAddress,
		session.IsImpersonation, session.ImpersonatedBy, session.FamilyID, session.IsRevoked, session.ExpiresAt, session.CreatedAt,
		session.LastActivityAt, session.IdleTimeoutMinutes, session.AbsoluteTimeoutMinutes,
	)
	if err != nil {
		return nil, err
	}

	return session, nil
}

// GetSessionByRefreshToken retrieves a session by refresh token hash
// NOTE: Does NOT filter by is_revoked - the service layer needs to detect reuse
// of revoked tokens to trigger family-wide revocation
func (r *AuthRepo) GetSessionByRefreshToken(ctx context.Context, refreshTokenHash string) (*entity.Session, error) {
	session := &entity.Session{}
	var expiresAt, createdAt, lastActivityAt sql.NullString
	query := `
		SELECT id, user_id, org_id, refresh_token_hash, user_agent, ip_address, is_impersonation,
		       impersonated_by, family_id, is_revoked, expires_at, created_at, last_activity_at,
		       idle_timeout_minutes, absolute_timeout_minutes
		FROM sessions WHERE refresh_token_hash = ? AND expires_at > ?
	`

	err := r.db.QueryRowContext(ctx, query, refreshTokenHash, time.Now().UTC()).Scan(
		&session.ID, &session.UserID, &session.OrgID, &session.RefreshTokenHash, &session.UserAgent, &session.IPAddress,
		&session.IsImpersonation, &session.ImpersonatedBy, &session.FamilyID, &session.IsRevoked, &expiresAt, &createdAt, &lastActivityAt,
		&session.IdleTimeoutMinutes, &session.AbsoluteTimeoutMinutes,
	)
	if err != nil {
		return nil, err
	}

	session.ExpiresAt = parseTimestamp(expiresAt)
	session.CreatedAt = parseTimestamp(createdAt)
	session.LastActivityAt = parseTimestamp(lastActivityAt)

	return session, nil
}

// DeleteSession deletes a session
func (r *AuthRepo) DeleteSession(ctx context.Context, sessionID string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM sessions WHERE id = ?`, sessionID)
	return err
}

// DeleteSessionByRefreshToken deletes a session by refresh token hash
func (r *AuthRepo) DeleteSessionByRefreshToken(ctx context.Context, refreshTokenHash string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM sessions WHERE refresh_token_hash = ?`, refreshTokenHash)
	return err
}

// DeleteUserSessions deletes all sessions for a user
func (r *AuthRepo) DeleteUserSessions(ctx context.Context, userID string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM sessions WHERE user_id = ?`, userID)
	return err
}

// CleanExpiredSessions removes expired sessions
func (r *AuthRepo) CleanExpiredSessions(ctx context.Context) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM sessions WHERE expires_at < ?`, time.Now().UTC())
	return err
}

// RevokeSession marks a single session as revoked (used when rotating tokens)
func (r *AuthRepo) RevokeSession(ctx context.Context, sessionID string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE sessions SET is_revoked = 1 WHERE id = ?`, sessionID)
	return err
}

// RevokeTokenFamily revokes all sessions in a token family (used on reuse detection)
// This is a security measure - if a token is reused after rotation, it indicates
// potential token theft, so we invalidate the entire family
func (r *AuthRepo) RevokeTokenFamily(ctx context.Context, familyID string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE sessions SET is_revoked = 1 WHERE family_id = ?`, familyID)
	return err
}

// GetSessionByUserAndOrg retrieves an active session for a user in an organization
func (r *AuthRepo) GetSessionByUserAndOrg(ctx context.Context, userID, orgID string) (*entity.Session, error) {
	session := &entity.Session{}
	var expiresAt, createdAt, lastActivityAt sql.NullString
	query := `
		SELECT id, user_id, org_id, refresh_token_hash, user_agent, ip_address, is_impersonation,
		       impersonated_by, family_id, is_revoked, expires_at, created_at, last_activity_at,
		       idle_timeout_minutes, absolute_timeout_minutes
		FROM sessions
		WHERE user_id = ? AND org_id = ? AND is_revoked = 0 AND expires_at > ?
		ORDER BY created_at DESC
		LIMIT 1
	`

	err := r.db.QueryRowContext(ctx, query, userID, orgID, time.Now().UTC()).Scan(
		&session.ID, &session.UserID, &session.OrgID, &session.RefreshTokenHash, &session.UserAgent, &session.IPAddress,
		&session.IsImpersonation, &session.ImpersonatedBy, &session.FamilyID, &session.IsRevoked, &expiresAt, &createdAt, &lastActivityAt,
		&session.IdleTimeoutMinutes, &session.AbsoluteTimeoutMinutes,
	)
	if err != nil {
		return nil, err
	}

	session.ExpiresAt = parseTimestamp(expiresAt)
	session.CreatedAt = parseTimestamp(createdAt)
	session.LastActivityAt = parseTimestamp(lastActivityAt)

	return session, nil
}

// UpdateLastActivity updates the last_activity_at timestamp for a session
func (r *AuthRepo) UpdateLastActivity(ctx context.Context, sessionID string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE sessions SET last_activity_at = ? WHERE id = ?`, time.Now().UTC(), sessionID)
	return err
}

// --- Invitation Operations ---

// CreateInvitation creates a new organization invitation
func (r *AuthRepo) CreateInvitation(ctx context.Context, orgID, email, role, token, invitedBy string, expiresAt time.Time) (*entity.Invitation, error) {
	invitation := &entity.Invitation{
		ID:        sfid.NewInvitation(),
		OrgID:     orgID,
		Email:     email,
		Role:      role,
		Token:     token,
		InvitedBy: invitedBy,
		ExpiresAt: expiresAt,
		CreatedAt: time.Now().UTC(),
	}

	query := `
		INSERT INTO org_invitations (id, org_id, email, role, token, invited_by, expires_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := r.db.ExecContext(ctx, query,
		invitation.ID, invitation.OrgID, invitation.Email, invitation.Role, invitation.Token, invitation.InvitedBy,
		invitation.ExpiresAt, invitation.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	return invitation, nil
}

// GetInvitationByToken retrieves an invitation by token
func (r *AuthRepo) GetInvitationByToken(ctx context.Context, token string) (*entity.InvitationWithDetails, error) {
	invitation := &entity.InvitationWithDetails{}
	var expiresAt, acceptedAt, createdAt sql.NullString
	query := `
		SELECT i.id, i.org_id, i.email, i.role, i.token, i.invited_by, i.expires_at, i.accepted_at, i.created_at,
		       o.name, u.first_name || ' ' || u.last_name, u.email
		FROM org_invitations i
		INNER JOIN organizations o ON i.org_id = o.id
		INNER JOIN users u ON i.invited_by = u.id
		WHERE i.token = ? AND i.accepted_at IS NULL AND i.expires_at > ?
	`

	err := r.db.QueryRowContext(ctx, query, token, time.Now().UTC()).Scan(
		&invitation.ID, &invitation.OrgID, &invitation.Email, &invitation.Role, &invitation.Token, &invitation.InvitedBy,
		&expiresAt, &acceptedAt, &createdAt,
		&invitation.OrgName, &invitation.InviterName, &invitation.InviterEmail,
	)
	if err != nil {
		return nil, err
	}

	invitation.ExpiresAt = parseTimestamp(expiresAt)
	invitation.AcceptedAt = parseTimestampPtr(acceptedAt)
	invitation.CreatedAt = parseTimestamp(createdAt)

	return invitation, nil
}

// MarkInvitationAccepted marks an invitation as accepted
func (r *AuthRepo) MarkInvitationAccepted(ctx context.Context, invitationID string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE org_invitations SET accepted_at = ? WHERE id = ?`, time.Now().UTC(), invitationID)
	return err
}

// ListPendingInvitations lists pending invitations for an organization
func (r *AuthRepo) ListPendingInvitations(ctx context.Context, orgID string) ([]entity.InvitationWithDetails, error) {
	query := `
		SELECT i.id, i.org_id, i.email, i.role, i.token, i.invited_by, i.expires_at, i.accepted_at, i.created_at,
		       o.name, u.first_name || ' ' || u.last_name, u.email
		FROM org_invitations i
		INNER JOIN organizations o ON i.org_id = o.id
		INNER JOIN users u ON i.invited_by = u.id
		WHERE i.org_id = ? AND i.accepted_at IS NULL AND i.expires_at > ?
		ORDER BY i.created_at DESC
	`

	rows, err := r.db.QueryContext(ctx, query, orgID, time.Now().UTC())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var invitations []entity.InvitationWithDetails
	for rows.Next() {
		var inv entity.InvitationWithDetails
		var expiresAt, acceptedAt, createdAt sql.NullString
		if err := rows.Scan(
			&inv.ID, &inv.OrgID, &inv.Email, &inv.Role, &inv.Token, &inv.InvitedBy,
			&expiresAt, &acceptedAt, &createdAt,
			&inv.OrgName, &inv.InviterName, &inv.InviterEmail,
		); err != nil {
			return nil, err
		}
		inv.ExpiresAt = parseTimestamp(expiresAt)
		inv.AcceptedAt = parseTimestampPtr(acceptedAt)
		inv.CreatedAt = parseTimestamp(createdAt)
		invitations = append(invitations, inv)
	}

	return invitations, nil
}

// DeleteInvitation deletes an invitation
func (r *AuthRepo) DeleteInvitation(ctx context.Context, invitationID string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM org_invitations WHERE id = ?`, invitationID)
	return err
}

// DeleteOrganization deletes an organization and all related data
func (r *AuthRepo) DeleteOrganization(ctx context.Context, orgID string) error {
	// Delete in order of dependencies

	// Delete sessions for users in this org
	_, err := r.db.ExecContext(ctx, `DELETE FROM sessions WHERE org_id = ?`, orgID)
	if err != nil {
		return err
	}

	// Delete invitations
	_, err = r.db.ExecContext(ctx, `DELETE FROM org_invitations WHERE org_id = ?`, orgID)
	if err != nil {
		return err
	}

	// Delete memberships
	_, err = r.db.ExecContext(ctx, `DELETE FROM user_org_memberships WHERE org_id = ?`, orgID)
	if err != nil {
		return err
	}

	// Delete metadata (entities, fields, layouts, navigation)
	_, err = r.db.ExecContext(ctx, `DELETE FROM layout_defs WHERE org_id = ?`, orgID)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx, `DELETE FROM field_defs WHERE org_id = ?`, orgID)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx, `DELETE FROM entity_defs WHERE org_id = ?`, orgID)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx, `DELETE FROM navigation_tabs WHERE org_id = ?`, orgID)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx, `DELETE FROM relationship_defs WHERE org_id = ?`, orgID)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx, `DELETE FROM related_list_configs WHERE org_id = ?`, orgID)
	if err != nil {
		return err
	}

	// Helper: tolerate "no such table" / "no such column" errors for org-specific
	// tables that may not exist in the master DB (they live in per-org Turso DBs).
	tryDelete := func(query string, args ...interface{}) error {
		_, e := r.db.ExecContext(ctx, query, args...)
		if e != nil {
			msg := e.Error()
			if strings.Contains(msg, "no such table") || strings.Contains(msg, "no such column") {
				return nil
			}
			return e
		}
		return nil
	}

	// Delete CRM data (contacts, accounts, tasks, quotes)
	// quote_line_items has ON DELETE CASCADE from quotes, but delete explicitly in case FK not enabled
	if err = tryDelete(`DELETE FROM quote_line_items WHERE org_id = ?`, orgID); err != nil {
		return err
	}
	if err = tryDelete(`DELETE FROM quotes WHERE org_id = ?`, orgID); err != nil {
		return err
	}
	if err = tryDelete(`DELETE FROM contacts WHERE org_id = ?`, orgID); err != nil {
		return err
	}
	if err = tryDelete(`DELETE FROM accounts WHERE org_id = ?`, orgID); err != nil {
		return err
	}
	if err = tryDelete(`DELETE FROM tasks WHERE org_id = ?`, orgID); err != nil {
		return err
	}

	// Delete data quality / dedup tables
	if err = tryDelete(`DELETE FROM scan_checkpoints WHERE org_id = ?`, orgID); err != nil {
		return err
	}
	if err = tryDelete(`DELETE FROM scan_jobs WHERE org_id = ?`, orgID); err != nil {
		return err
	}
	if err = tryDelete(`DELETE FROM scan_schedules WHERE org_id = ?`, orgID); err != nil {
		return err
	}
	if err = tryDelete(`DELETE FROM pending_duplicate_alerts WHERE org_id = ?`, orgID); err != nil {
		return err
	}
	if err = tryDelete(`DELETE FROM matching_rule_fields WHERE rule_id IN (SELECT id FROM matching_rules WHERE org_id = ?)`, orgID); err != nil {
		return err
	}
	if err = tryDelete(`DELETE FROM matching_rules WHERE org_id = ?`, orgID); err != nil {
		return err
	}
	if err = tryDelete(`DELETE FROM merge_snapshots WHERE org_id = ?`, orgID); err != nil {
		return err
	}
	if err = tryDelete(`DELETE FROM notifications WHERE org_id = ?`, orgID); err != nil {
		return err
	}

	// Delete other org-specific data
	if err = tryDelete(`DELETE FROM tripwires WHERE org_id = ?`, orgID); err != nil {
		return err
	}
	if err = tryDelete(`DELETE FROM bearing_configs WHERE org_id = ?`, orgID); err != nil {
		return err
	}
	if err = tryDelete(`DELETE FROM validation_rules WHERE org_id = ?`, orgID); err != nil {
		return err
	}
	if err = tryDelete(`DELETE FROM api_tokens WHERE org_id = ?`, orgID); err != nil {
		return err
	}
	if err = tryDelete(`DELETE FROM custom_pages WHERE org_id = ?`, orgID); err != nil {
		return err
	}
	if err = tryDelete(`DELETE FROM flow_executions WHERE org_id = ?`, orgID); err != nil {
		return err
	}
	if err = tryDelete(`DELETE FROM flow_definitions WHERE org_id = ?`, orgID); err != nil {
		return err
	}

	// Delete migration tracking (has FK to organizations without CASCADE)
	if err = tryDelete(`DELETE FROM migration_runs WHERE org_id = ?`, orgID); err != nil {
		return err
	}

	// Finally delete the organization
	_, err = r.db.ExecContext(ctx, `DELETE FROM organizations WHERE id = ?`, orgID)
	return err
}

// --- Password Reset Token Operations ---

// CreatePasswordResetToken creates a new password reset token
func (r *AuthRepo) CreatePasswordResetToken(ctx context.Context, userID, tokenHash string, expiresAt time.Time) error {
	id := sfid.NewPasswordResetToken()
	query := `
		INSERT INTO password_reset_tokens (id, user_id, token_hash, expires_at, created_at)
		VALUES (?, ?, ?, ?, ?)
	`
	_, err := r.db.ExecContext(ctx, query, id, userID, tokenHash, expiresAt, time.Now().UTC())
	return err
}

// GetPasswordResetToken retrieves a password reset token by hash
func (r *AuthRepo) GetPasswordResetToken(ctx context.Context, tokenHash string) (*entity.PasswordResetToken, error) {
	token := &entity.PasswordResetToken{}
	var expiresAt, usedAt, createdAt sql.NullString
	query := `
		SELECT id, user_id, token_hash, expires_at, used_at, created_at
		FROM password_reset_tokens WHERE token_hash = ?
	`

	err := r.db.QueryRowContext(ctx, query, tokenHash).Scan(
		&token.ID, &token.UserID, &token.TokenHash, &expiresAt, &usedAt, &createdAt,
	)
	if err != nil {
		return nil, err
	}

	token.ExpiresAt = parseTimestamp(expiresAt)
	token.UsedAt = parseTimestampPtr(usedAt)
	token.CreatedAt = parseTimestamp(createdAt)

	return token, nil
}

// MarkPasswordResetTokenUsed marks a password reset token as used
func (r *AuthRepo) MarkPasswordResetTokenUsed(ctx context.Context, tokenID string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE password_reset_tokens SET used_at = ? WHERE id = ?`, time.Now().UTC(), tokenID)
	return err
}

// DeletePasswordResetTokens deletes all password reset tokens for a user
func (r *AuthRepo) DeletePasswordResetTokens(ctx context.Context, userID string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM password_reset_tokens WHERE user_id = ?`, userID)
	return err
}

// GetUserNamesByIDs returns a map of user ID to full name for the given IDs
// This queries the platform database (users table) to resolve user names
func (r *AuthRepo) GetUserNamesByIDs(ctx context.Context, userIDs []string) (map[string]string, error) {
	if len(userIDs) == 0 {
		return map[string]string{}, nil
	}

	// Build query with placeholders
	placeholders := make([]string, len(userIDs))
	args := make([]interface{}, len(userIDs))
	for i, id := range userIDs {
		placeholders[i] = "?"
		args[i] = id
	}

	query := fmt.Sprintf(`
		SELECT id, first_name, last_name
		FROM users
		WHERE id IN (%s)
	`, strings.Join(placeholders, ","))

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]string)
	for rows.Next() {
		var id, firstName, lastName string
		if err := rows.Scan(&id, &firstName, &lastName); err != nil {
			continue
		}
		// Build full name (same as User.Name() method)
		name := strings.TrimSpace(firstName + " " + lastName)
		if name == "" {
			name = "Unknown User"
		}
		result[id] = name
	}

	return result, nil
}
