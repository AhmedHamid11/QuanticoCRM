package repo

import (
	"context"
	"database/sql"
	"time"

	"github.com/fastcrm/backend/internal/db"
	"github.com/fastcrm/backend/internal/entity"
)

// SequenceRepo handles all database operations for sequences, steps, enrollments,
// and step executions. Uses the DBConn interface for compatibility with both local
// SQLite and Turso connections.
type SequenceRepo struct {
	db db.DBConn
}

// NewSequenceRepo creates a new SequenceRepo with the given connection.
func NewSequenceRepo(conn db.DBConn) *SequenceRepo {
	return &SequenceRepo{db: conn}
}

// WithDB returns a new SequenceRepo using the provided tenant database connection.
// This is the standard tenant-routing pattern used throughout the codebase.
func (r *SequenceRepo) WithDB(conn db.DBConn) *SequenceRepo {
	if conn == nil {
		return r
	}
	return &SequenceRepo{db: conn}
}

// ========== Sequences ==========

// CreateSequence inserts a new sequence record.
func (r *SequenceRepo) CreateSequence(ctx context.Context, s *entity.Sequence) error {
	query := `
		INSERT INTO sequences
		    (id, org_id, name, description, status, timezone,
		     business_hours_start, business_hours_end, created_by,
		     created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`
	var desc interface{}
	if s.Description != nil {
		desc = *s.Description
	}
	bhStart := "09:00"
	if s.BusinessHoursStart != nil {
		bhStart = *s.BusinessHoursStart
	}
	bhEnd := "17:00"
	if s.BusinessHoursEnd != nil {
		bhEnd = *s.BusinessHoursEnd
	}

	_, err := r.db.ExecContext(ctx, query,
		s.ID, s.OrgID, s.Name, desc, s.Status, s.Timezone,
		bhStart, bhEnd, s.CreatedBy,
	)
	return err
}

// GetSequence retrieves a sequence by orgID and ID, including all its steps.
// Returns nil (not an error) when no record is found.
func (r *SequenceRepo) GetSequence(ctx context.Context, orgID, id string) (*entity.Sequence, error) {
	query := `
		SELECT id, org_id, name, description, status, timezone,
		       business_hours_start, business_hours_end, created_by,
		       created_at, updated_at
		FROM sequences
		WHERE org_id = ? AND id = ?
	`
	row := r.db.QueryRowContext(ctx, query, orgID, id)

	s, err := scanSequence(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return s, nil
}

// ListSequences returns all sequences for an org, ordered by updated_at DESC.
// Each row includes step_count and enrollment_count via correlated COUNT subqueries.
func (r *SequenceRepo) ListSequences(ctx context.Context, orgID string) ([]*entity.Sequence, error) {
	query := `
		SELECT s.id, s.org_id, s.name, s.description, s.status, s.timezone,
		       s.business_hours_start, s.business_hours_end, s.created_by,
		       s.created_at, s.updated_at,
		       (SELECT COUNT(*) FROM sequence_steps ss WHERE ss.sequence_id = s.id) AS step_count,
		       (SELECT COUNT(*) FROM sequence_enrollments se WHERE se.sequence_id = s.id AND se.status IN ('enrolled', 'active', 'paused')) AS enrollment_count
		FROM sequences s
		WHERE s.org_id = ?
		ORDER BY s.updated_at DESC
	`
	rows, err := r.db.QueryContext(ctx, query, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sequences []*entity.Sequence
	for rows.Next() {
		s, err := scanSequenceRow(rows)
		if err != nil {
			return nil, err
		}
		sequences = append(sequences, s)
	}
	return sequences, rows.Err()
}

// UpdateSequence updates the mutable fields of a sequence.
func (r *SequenceRepo) UpdateSequence(ctx context.Context, s *entity.Sequence) error {
	query := `
		UPDATE sequences
		SET name = ?, description = ?, timezone = ?,
		    business_hours_start = ?, business_hours_end = ?,
		    updated_at = CURRENT_TIMESTAMP
		WHERE id = ? AND org_id = ?
	`
	var desc interface{}
	if s.Description != nil {
		desc = *s.Description
	}
	bhStart := "09:00"
	if s.BusinessHoursStart != nil {
		bhStart = *s.BusinessHoursStart
	}
	bhEnd := "17:00"
	if s.BusinessHoursEnd != nil {
		bhEnd = *s.BusinessHoursEnd
	}

	_, err := r.db.ExecContext(ctx, query,
		s.Name, desc, s.Timezone, bhStart, bhEnd, s.ID, s.OrgID,
	)
	return err
}

// DeleteSequence removes a sequence. Only allowed if status=draft.
func (r *SequenceRepo) DeleteSequence(ctx context.Context, orgID, id string) error {
	_, err := r.db.ExecContext(ctx,
		"DELETE FROM sequences WHERE id = ? AND org_id = ? AND status = 'draft'",
		id, orgID,
	)
	return err
}

// ActivateSequence transitions a sequence from draft to active.
func (r *SequenceRepo) ActivateSequence(ctx context.Context, orgID, id string) error {
	_, err := r.db.ExecContext(ctx,
		"UPDATE sequences SET status = 'active', updated_at = CURRENT_TIMESTAMP WHERE id = ? AND org_id = ? AND status IN ('draft', 'paused')",
		id, orgID,
	)
	return err
}

// PauseSequence transitions a sequence from active to paused.
func (r *SequenceRepo) PauseSequence(ctx context.Context, orgID, id string) error {
	_, err := r.db.ExecContext(ctx,
		"UPDATE sequences SET status = 'paused', updated_at = CURRENT_TIMESTAMP WHERE id = ? AND org_id = ? AND status = 'active'",
		id, orgID,
	)
	return err
}

// ========== Steps ==========

// CreateStep inserts a new sequence step.
func (r *SequenceRepo) CreateStep(ctx context.Context, step *entity.SequenceStep) error {
	query := `
		INSERT INTO sequence_steps
		    (id, sequence_id, step_number, step_type, delay_days, delay_hours,
		     template_id, config_json, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`
	var templateID, configJSON interface{}
	if step.TemplateID != nil {
		templateID = *step.TemplateID
	}
	if step.ConfigJSON != nil {
		configJSON = *step.ConfigJSON
	}

	_, err := r.db.ExecContext(ctx, query,
		step.ID, step.SequenceID, step.StepNumber, step.StepType,
		step.DelayDays, step.DelayHours, templateID, configJSON,
	)
	return err
}

// UpdateStep updates the mutable fields of a sequence step.
func (r *SequenceRepo) UpdateStep(ctx context.Context, step *entity.SequenceStep) error {
	query := `
		UPDATE sequence_steps
		SET step_number = ?, step_type = ?, delay_days = ?, delay_hours = ?,
		    template_id = ?, config_json = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ? AND sequence_id = ?
	`
	var templateID, configJSON interface{}
	if step.TemplateID != nil {
		templateID = *step.TemplateID
	}
	if step.ConfigJSON != nil {
		configJSON = *step.ConfigJSON
	}

	_, err := r.db.ExecContext(ctx, query,
		step.StepNumber, step.StepType, step.DelayDays, step.DelayHours,
		templateID, configJSON, step.ID, step.SequenceID,
	)
	return err
}

// DeleteStep removes a step from a sequence.
func (r *SequenceRepo) DeleteStep(ctx context.Context, sequenceID, stepID string) error {
	_, err := r.db.ExecContext(ctx,
		"DELETE FROM sequence_steps WHERE id = ? AND sequence_id = ?",
		stepID, sequenceID,
	)
	return err
}

// ListStepsBySequence returns all steps for a sequence, ordered by step_number.
func (r *SequenceRepo) ListStepsBySequence(ctx context.Context, sequenceID string) ([]*entity.SequenceStep, error) {
	query := `
		SELECT id, sequence_id, step_number, step_type, delay_days, delay_hours,
		       template_id, config_json, created_at, updated_at
		FROM sequence_steps
		WHERE sequence_id = ?
		ORDER BY step_number ASC
	`
	rows, err := r.db.QueryContext(ctx, query, sequenceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var steps []*entity.SequenceStep
	for rows.Next() {
		step, err := scanStepRow(rows)
		if err != nil {
			return nil, err
		}
		steps = append(steps, step)
	}
	return steps, rows.Err()
}

// ========== Enrollments ==========

// CreateEnrollment inserts a new sequence enrollment.
func (r *SequenceRepo) CreateEnrollment(ctx context.Context, e *entity.SequenceEnrollment) error {
	query := `
		INSERT INTO sequence_enrollments
		    (id, sequence_id, contact_id, org_id, enrolled_by, status, current_step,
		     ab_variant_id, enrolled_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`
	var abVariantID interface{}
	if e.ABVariantID != nil {
		abVariantID = *e.ABVariantID
	}
	enrolledAt := e.EnrolledAt.UTC().Format("2006-01-02T15:04:05Z")

	_, err := r.db.ExecContext(ctx, query,
		e.ID, e.SequenceID, e.ContactID, e.OrgID, e.EnrolledBy,
		e.Status, e.CurrentStep, abVariantID, enrolledAt,
	)
	return err
}

// GetEnrollment retrieves a single enrollment by ID.
// Returns nil (not an error) when not found.
func (r *SequenceRepo) GetEnrollment(ctx context.Context, id string) (*entity.SequenceEnrollment, error) {
	query := `
		SELECT id, sequence_id, contact_id, org_id, enrolled_by, status, current_step,
		       ab_variant_id, enrolled_at, finished_at, paused_at, created_at, updated_at
		FROM sequence_enrollments
		WHERE id = ?
	`
	row := r.db.QueryRowContext(ctx, query, id)
	e, err := scanEnrollment(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return e, err
}

// UpdateEnrollmentStatus updates the status (and optional timestamp fields) of an enrollment.
// UpdateEnrollmentVariant sets the ab_variant_id for an enrollment.
// Called after enrollment creation when A/B variants are configured.
func (r *SequenceRepo) UpdateEnrollmentVariant(ctx context.Context, enrollmentID, variantID string) error {
	_, err := r.db.ExecContext(ctx,
		"UPDATE sequence_enrollments SET ab_variant_id = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?",
		variantID, enrollmentID,
	)
	return err
}

func (r *SequenceRepo) UpdateEnrollmentStatus(ctx context.Context, id, status string) error {
	now := time.Now().UTC().Format("2006-01-02T15:04:05Z")
	query := `
		UPDATE sequence_enrollments
		SET status = ?, updated_at = CURRENT_TIMESTAMP,
		    finished_at = CASE WHEN ? IN ('finished', 'replied', 'bounced', 'opted_out') THEN ? ELSE finished_at END,
		    paused_at   = CASE WHEN ? = 'paused' THEN ? ELSE paused_at END
		WHERE id = ?
	`
	_, err := r.db.ExecContext(ctx, query,
		status,
		status, now,
		status, now,
		id,
	)
	return err
}

// GetActiveEnrollmentsByContact returns all active/enrolled enrollments for a contact
// across ALL sequences. Used for overlap detection.
func (r *SequenceRepo) GetActiveEnrollmentsByContact(ctx context.Context, orgID, contactID string) ([]*entity.SequenceEnrollment, error) {
	query := `
		SELECT e.id, e.sequence_id, e.contact_id, e.org_id, e.enrolled_by, e.status, e.current_step,
		       e.ab_variant_id, e.enrolled_at, e.finished_at, e.paused_at, e.created_at, e.updated_at
		FROM sequence_enrollments e
		WHERE e.org_id = ? AND e.contact_id = ?
		  AND e.status IN ('enrolled', 'active', 'paused')
	`
	rows, err := r.db.QueryContext(ctx, query, orgID, contactID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var enrollments []*entity.SequenceEnrollment
	for rows.Next() {
		e, err := scanEnrollmentRow(rows)
		if err != nil {
			return nil, err
		}
		enrollments = append(enrollments, e)
	}
	return enrollments, rows.Err()
}

// GetActiveEnrollmentBySequenceAndContact returns the enrollment for a specific contact
// in a specific sequence if it exists and is still active. Used for UNIQUE guard.
// Returns nil (not an error) when not found.
func (r *SequenceRepo) GetActiveEnrollmentBySequenceAndContact(ctx context.Context, sequenceID, contactID string) (*entity.SequenceEnrollment, error) {
	query := `
		SELECT id, sequence_id, contact_id, org_id, enrolled_by, status, current_step,
		       ab_variant_id, enrolled_at, finished_at, paused_at, created_at, updated_at
		FROM sequence_enrollments
		WHERE sequence_id = ? AND contact_id = ?
		  AND status IN ('enrolled', 'active', 'paused')
	`
	row := r.db.QueryRowContext(ctx, query, sequenceID, contactID)
	e, err := scanEnrollment(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return e, err
}

// GetEnrollmentsBySequenceAndContacts returns enrollments for a set of contacts in a
// specific sequence. Used for bulk enrollment skip logic.
func (r *SequenceRepo) GetEnrollmentsBySequenceAndContacts(ctx context.Context, sequenceID string, contactIDs []string) (map[string]bool, error) {
	if len(contactIDs) == 0 {
		return map[string]bool{}, nil
	}

	// Build query with placeholders
	placeholders := make([]byte, 0, len(contactIDs)*2)
	args := make([]interface{}, 0, len(contactIDs)+1)
	args = append(args, sequenceID)
	for i, id := range contactIDs {
		if i > 0 {
			placeholders = append(placeholders, ',')
		}
		placeholders = append(placeholders, '?')
		args = append(args, id)
	}

	query := "SELECT contact_id FROM sequence_enrollments WHERE sequence_id = ? AND contact_id IN (" + string(placeholders) + ")"
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	enrolled := make(map[string]bool)
	for rows.Next() {
		var contactID string
		if err := rows.Scan(&contactID); err != nil {
			return nil, err
		}
		enrolled[contactID] = true
	}
	return enrolled, rows.Err()
}

// BulkCreateEnrollments inserts multiple enrollments in a single transaction.
func (r *SequenceRepo) BulkCreateEnrollments(ctx context.Context, enrollments []*entity.SequenceEnrollment) error {
	if len(enrollments) == 0 {
		return nil
	}
	for _, e := range enrollments {
		if err := r.CreateEnrollment(ctx, e); err != nil {
			return err
		}
	}
	return nil
}

// ========== Suppression ==========

// IsContactOptedOut checks whether a contact has opted out of email (channel='email' or channel='all').
func (r *SequenceRepo) IsContactOptedOut(ctx context.Context, orgID, contactID string) (bool, error) {
	query := `
		SELECT COUNT(*)
		FROM opt_out_list
		WHERE org_id = ? AND contact_id = ? AND channel IN ('email', 'all')
	`
	var count int
	err := r.db.QueryRowContext(ctx, query, orgID, contactID).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// GetContactFieldValue retrieves the value of a single field for a contact.
// Returns empty string if the contact doesn't exist or the field is NULL.
func (r *SequenceRepo) GetContactFieldValue(ctx context.Context, orgID, contactID, field string) (string, error) {
	// We use a parameterised column name via a CASE approach to avoid SQL injection.
	// Only pre-approved fields are queried.
	// NOTE: field is validated by the caller to be a known safe column name.
	query := "SELECT COALESCE(" + sanitizeFieldName(field) + ", '') FROM contacts WHERE org_id = ? AND id = ?"
	var value string
	err := r.db.QueryRowContext(ctx, query, orgID, contactID).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return value, err
}

// ========== Step Executions ==========

// CreateStepExecution inserts a new step execution record.
func (r *SequenceRepo) CreateStepExecution(ctx context.Context, exec *entity.StepExecution) error {
	query := `
		INSERT INTO sequence_step_executions
		    (id, enrollment_id, step_id, org_id, status, scheduled_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`
	var scheduledAt interface{}
	if exec.ScheduledAt != nil {
		scheduledAt = exec.ScheduledAt.UTC().Format("2006-01-02T15:04:05Z")
	}

	_, err := r.db.ExecContext(ctx, query,
		exec.ID, exec.EnrollmentID, exec.StepID, exec.OrgID, exec.Status, scheduledAt,
	)
	return err
}

// GetDueExecutions returns up to 100 step executions that are due to run for the
// given org. "Due" means status='scheduled' AND scheduled_at <= before, ordered
// by scheduled_at ascending.
func (r *SequenceRepo) GetDueExecutions(ctx context.Context, orgID string, before time.Time) ([]*entity.StepExecution, error) {
	query := `
		SELECT id, enrollment_id, step_id, org_id, status,
		       scheduled_at, executed_at, error_message, created_at, updated_at
		FROM sequence_step_executions
		WHERE org_id = ? AND status = 'scheduled' AND scheduled_at <= ?
		ORDER BY scheduled_at ASC
		LIMIT 100
	`
	cutoff := before.UTC().Format("2006-01-02T15:04:05Z")
	rows, err := r.db.QueryContext(ctx, query, orgID, cutoff)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var executions []*entity.StepExecution
	for rows.Next() {
		exec, err := scanStepExecutionRow(rows)
		if err != nil {
			return nil, err
		}
		executions = append(executions, exec)
	}
	return executions, rows.Err()
}

// UpdateStepExecution updates mutable fields of a step execution record.
func (r *SequenceRepo) UpdateStepExecution(ctx context.Context, exec *entity.StepExecution) error {
	query := `
		UPDATE sequence_step_executions
		SET status = ?, scheduled_at = ?, executed_at = ?, error_message = ?,
		    updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`
	var scheduledAt, executedAt, errorMessage interface{}
	if exec.ScheduledAt != nil {
		scheduledAt = exec.ScheduledAt.UTC().Format("2006-01-02T15:04:05Z")
	}
	if exec.ExecutedAt != nil {
		executedAt = exec.ExecutedAt.UTC().Format("2006-01-02T15:04:05Z")
	}
	if exec.ErrorMessage != nil {
		errorMessage = *exec.ErrorMessage
	}

	_, err := r.db.ExecContext(ctx, query,
		exec.Status, scheduledAt, executedAt, errorMessage, exec.ID,
	)
	return err
}

// ClaimStepExecution atomically transitions a step execution from scheduled to executing.
// Returns true if the row was successfully claimed (rows affected > 0).
func (r *SequenceRepo) ClaimStepExecution(ctx context.Context, id string) (bool, error) {
	result, err := r.db.ExecContext(ctx,
		"UPDATE sequence_step_executions SET status = 'executing', updated_at = CURRENT_TIMESTAMP WHERE id = ? AND status = 'scheduled'",
		id,
	)
	if err != nil {
		return false, err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// ========== Task Queue (Manual Steps) ==========

// TaskView is a flattened view of a due manual step execution joined with
// its sequence step config, enrollment, contact info, and engagement signals.
type TaskView struct {
	ExecutionID  string
	StepType     string
	StepNumber   int
	ConfigJSON   *string
	ScheduledAt  time.Time
	SequenceID   string
	SequenceName string
	ContactID    string
	ContactName  string
	ContactEmail string
	ContactPhone *string
	EnrollmentID string
	LastOpenAt   *time.Time
	LastReplyAt  *time.Time
}

// GetTasksForUser returns all due manual step executions (call, linkedin, custom)
// for the given user (enrolled_by), joined with contact and sequence info.
// Tasks are sorted by scheduled_at ASC (overdue first).
func (r *SequenceRepo) GetTasksForUser(ctx context.Context, orgID, userID string, now time.Time) ([]TaskView, error) {
	nowStr := now.UTC().Format("2006-01-02T15:04:05Z")
	query := `
		SELECT
			sse.id                                      AS execution_id,
			ss.step_type,
			ss.step_number,
			ss.config_json,
			sse.scheduled_at,
			s.id                                        AS sequence_id,
			s.name                                      AS sequence_name,
			se.contact_id,
			COALESCE(c.first_name || ' ' || c.last_name, c.email_address, se.contact_id) AS contact_name,
			COALESCE(c.email_address, '')               AS contact_email,
			c.phone_number                              AS contact_phone,
			se.id                                       AS enrollment_id,
			MAX(CASE WHEN te.event_type = 'open'  THEN te.occurred_at ELSE NULL END) AS last_open_at,
			MAX(CASE WHEN te.event_type = 'reply' THEN te.occurred_at ELSE NULL END) AS last_reply_at
		FROM sequence_step_executions sse
		JOIN sequence_steps ss
			ON ss.id = sse.step_id
		JOIN sequence_enrollments se
			ON se.id = sse.enrollment_id
		JOIN sequences s
			ON s.id = se.sequence_id
		LEFT JOIN contacts c
			ON c.id = se.contact_id AND c.org_id = ?
		LEFT JOIN email_tracking_events te
			ON te.enrollment_id = se.id
		WHERE sse.org_id = ?
		  AND se.enrolled_by = ?
		  AND ss.step_type IN ('call', 'linkedin', 'custom')
		  AND sse.status = 'scheduled'
		  AND sse.scheduled_at <= ?
		GROUP BY sse.id
		ORDER BY sse.scheduled_at ASC
	`
	rows, err := r.db.QueryContext(ctx, query, orgID, orgID, userID, nowStr)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []TaskView
	for rows.Next() {
		var t TaskView
		var scheduledAt, lastOpenAt, lastReplyAt sql.NullString
		var phone sql.NullString
		var configJSON sql.NullString

		err := rows.Scan(
			&t.ExecutionID,
			&t.StepType,
			&t.StepNumber,
			&configJSON,
			&scheduledAt,
			&t.SequenceID,
			&t.SequenceName,
			&t.ContactID,
			&t.ContactName,
			&t.ContactEmail,
			&phone,
			&t.EnrollmentID,
			&lastOpenAt,
			&lastReplyAt,
		)
		if err != nil {
			return nil, err
		}
		if configJSON.Valid {
			t.ConfigJSON = &configJSON.String
		}
		if scheduledAt.Valid {
			if ts, err := parseDBTime(scheduledAt.String); err == nil {
				t.ScheduledAt = ts
			}
		}
		if phone.Valid {
			t.ContactPhone = &phone.String
		}
		if lastOpenAt.Valid {
			if ts, err := parseDBTime(lastOpenAt.String); err == nil {
				t.LastOpenAt = &ts
			}
		}
		if lastReplyAt.Valid {
			if ts, err := parseDBTime(lastReplyAt.String); err == nil {
				t.LastReplyAt = &ts
			}
		}
		tasks = append(tasks, t)
	}
	return tasks, rows.Err()
}

// CompleteStepExecution marks a step execution as completed.
func (r *SequenceRepo) CompleteStepExecution(ctx context.Context, execID string) error {
	now := time.Now().UTC().Format("2006-01-02T15:04:05Z")
	_, err := r.db.ExecContext(ctx,
		"UPDATE sequence_step_executions SET status = 'completed', executed_at = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?",
		now, execID,
	)
	return err
}

// SkipStepExecution marks a step execution as skipped.
func (r *SequenceRepo) SkipStepExecution(ctx context.Context, execID string) error {
	_, err := r.db.ExecContext(ctx,
		"UPDATE sequence_step_executions SET status = 'skipped', updated_at = CURRENT_TIMESTAMP WHERE id = ?",
		execID,
	)
	return err
}

// RescheduleStepExecution updates the scheduled_at time for a step execution.
// Status remains 'scheduled'.
func (r *SequenceRepo) RescheduleStepExecution(ctx context.Context, execID string, newScheduledAt time.Time) error {
	newTime := newScheduledAt.UTC().Format("2006-01-02T15:04:05Z")
	_, err := r.db.ExecContext(ctx,
		"UPDATE sequence_step_executions SET scheduled_at = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?",
		newTime, execID,
	)
	return err
}

// GetStepExecutionByID retrieves a step execution by its ID.
// Returns nil (not an error) when not found.
func (r *SequenceRepo) GetStepExecutionByID(ctx context.Context, id string) (*entity.StepExecution, error) {
	query := `
		SELECT id, enrollment_id, step_id, org_id, status, scheduled_at, executed_at, error_message, created_at, updated_at
		FROM sequence_step_executions
		WHERE id = ?
	`
	row := r.db.QueryRowContext(ctx, query, id)
	var exec entity.StepExecution
	var scheduledAt, executedAt, errorMsg, createdAt, updatedAt sql.NullString

	err := row.Scan(
		&exec.ID, &exec.EnrollmentID, &exec.StepID, &exec.OrgID, &exec.Status,
		&scheduledAt, &executedAt, &errorMsg, &createdAt, &updatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if scheduledAt.Valid {
		if t, err := parseDBTime(scheduledAt.String); err == nil {
			exec.ScheduledAt = &t
		}
	}
	if executedAt.Valid {
		if t, err := parseDBTime(executedAt.String); err == nil {
			exec.ExecutedAt = &t
		}
	}
	if errorMsg.Valid {
		exec.ErrorMessage = &errorMsg.String
	}
	return &exec, nil
}

// ListEnrollmentSteps returns all step executions for an enrollment ordered by step_id.
// Used to find the next scheduled step.
func (r *SequenceRepo) ListEnrollmentSteps(ctx context.Context, enrollmentID string) ([]*entity.StepExecution, error) {
	query := `
		SELECT id, enrollment_id, step_id, org_id, status, scheduled_at, executed_at, error_message, created_at, updated_at
		FROM sequence_step_executions
		WHERE enrollment_id = ?
		ORDER BY scheduled_at ASC
	`
	rows, err := r.db.QueryContext(ctx, query, enrollmentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var execs []*entity.StepExecution
	for rows.Next() {
		var exec entity.StepExecution
		var scheduledAt, executedAt, errorMsg, createdAt, updatedAt sql.NullString
		err := rows.Scan(
			&exec.ID, &exec.EnrollmentID, &exec.StepID, &exec.OrgID, &exec.Status,
			&scheduledAt, &executedAt, &errorMsg, &createdAt, &updatedAt,
		)
		if err != nil {
			return nil, err
		}
		if scheduledAt.Valid {
			if t, err := parseDBTime(scheduledAt.String); err == nil {
				exec.ScheduledAt = &t
			}
		}
		if executedAt.Valid {
			if t, err := parseDBTime(executedAt.String); err == nil {
				exec.ExecutedAt = &t
			}
		}
		if errorMsg.Valid {
			exec.ErrorMessage = &errorMsg.String
		}
		execs = append(execs, &exec)
	}
	return execs, rows.Err()
}

// ========== Enrollment Triggers ==========

// ListEnrollmentTriggersByEntity returns active enrollment triggers for the given
// org and target entity. Only returns triggers whose sequence is currently active.
func (r *SequenceRepo) ListEnrollmentTriggersByEntity(ctx context.Context, orgID, targetEntity string) ([]entity.EnrollmentTrigger, error) {
	query := `
		SELECT id, sequence_id, org_id, target_entity, field_name, operator, value, created_at, updated_at
		FROM sequence_enrollment_triggers
		WHERE org_id = ? AND target_entity = ?
		  AND sequence_id IN (
		      SELECT id FROM sequences WHERE org_id = ? AND status = 'active'
		  )
		ORDER BY created_at ASC
	`
	rows, err := r.db.QueryContext(ctx, query, orgID, targetEntity, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var triggers []entity.EnrollmentTrigger
	for rows.Next() {
		t, err := scanEnrollmentTriggerRow(rows)
		if err != nil {
			return nil, err
		}
		triggers = append(triggers, t)
	}
	return triggers, rows.Err()
}

// ListEnrollmentTriggersBySequence returns all enrollment triggers for a specific sequence.
func (r *SequenceRepo) ListEnrollmentTriggersBySequence(ctx context.Context, sequenceID string) ([]entity.EnrollmentTrigger, error) {
	query := `
		SELECT id, sequence_id, org_id, target_entity, field_name, operator, value, created_at, updated_at
		FROM sequence_enrollment_triggers
		WHERE sequence_id = ?
		ORDER BY created_at ASC
	`
	rows, err := r.db.QueryContext(ctx, query, sequenceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var triggers []entity.EnrollmentTrigger
	for rows.Next() {
		t, err := scanEnrollmentTriggerRow(rows)
		if err != nil {
			return nil, err
		}
		triggers = append(triggers, t)
	}
	return triggers, rows.Err()
}

// CreateEnrollmentTrigger inserts a new enrollment trigger.
func (r *SequenceRepo) CreateEnrollmentTrigger(ctx context.Context, t *entity.EnrollmentTrigger) error {
	query := `
		INSERT INTO sequence_enrollment_triggers
		    (id, sequence_id, org_id, target_entity, field_name, operator, value,
		     created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`
	_, err := r.db.ExecContext(ctx, query,
		t.ID, t.SequenceID, t.OrgID, t.TargetEntity, t.FieldName, t.Operator, t.Value,
	)
	return err
}

// DeleteEnrollmentTrigger removes an enrollment trigger by ID.
func (r *SequenceRepo) DeleteEnrollmentTrigger(ctx context.Context, triggerID string) error {
	_, err := r.db.ExecContext(ctx,
		"DELETE FROM sequence_enrollment_triggers WHERE id = ?",
		triggerID,
	)
	return err
}

// AddToOptOutList adds a contact to the opt-out list for the given channel and reason.
// Uses INSERT OR IGNORE to be idempotent — silently skips if already opted out.
func (r *SequenceRepo) AddToOptOutList(ctx context.Context, orgID, contactID, channel, reason string) error {
	id := orgID + "_" + contactID + "_" + channel // deterministic ID for idempotency
	_, err := r.db.ExecContext(ctx, `
		INSERT OR IGNORE INTO opt_out_list (id, org_id, contact_id, channel, reason, opted_out_at, created_at)
		VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`, id, orgID, contactID, channel, reason)
	return err
}

// scanEnrollmentTriggerRow scans a single EnrollmentTrigger from *sql.Rows.
func scanEnrollmentTriggerRow(rows *sql.Rows) (entity.EnrollmentTrigger, error) {
	var t entity.EnrollmentTrigger
	var createdAt, updatedAt sql.NullString

	err := rows.Scan(
		&t.ID, &t.SequenceID, &t.OrgID, &t.TargetEntity,
		&t.FieldName, &t.Operator, &t.Value,
		&createdAt, &updatedAt,
	)
	if err != nil {
		return t, err
	}
	if createdAt.Valid {
		if ts, err := parseDBTime(createdAt.String); err == nil {
			t.CreatedAt = ts
		}
	}
	if updatedAt.Valid {
		if ts, err := parseDBTime(updatedAt.String); err == nil {
			t.UpdatedAt = ts
		}
	}
	return t, nil
}

// ========== Helpers ==========

// sanitizeFieldName converts a field name to its snake_case DB column equivalent,
// allowing only known-safe column names to prevent SQL injection.
// Returns the field as-is if it's a known column, otherwise returns 'id' as a safe fallback.
func sanitizeFieldName(field string) string {
	allowed := map[string]string{
		"status":       "status",
		"email":        "email_address",
		"email_address": "email_address",
		"phone":        "phone_number",
		"phone_number": "phone_number",
		"first_name":   "first_name",
		"last_name":    "last_name",
		"account_name": "account_name",
	}
	if col, ok := allowed[field]; ok {
		return col
	}
	return "id" // Safe fallback (always non-null)
}

// scanSequence scans a single sequence from a *sql.Row.
func scanSequence(row *sql.Row) (*entity.Sequence, error) {
	var s entity.Sequence
	var desc, bhStart, bhEnd, createdAt, updatedAt sql.NullString

	err := row.Scan(
		&s.ID, &s.OrgID, &s.Name, &desc, &s.Status, &s.Timezone,
		&bhStart, &bhEnd, &s.CreatedBy, &createdAt, &updatedAt,
	)
	if err != nil {
		return nil, err
	}
	if desc.Valid {
		s.Description = &desc.String
	}
	if bhStart.Valid {
		s.BusinessHoursStart = &bhStart.String
	}
	if bhEnd.Valid {
		s.BusinessHoursEnd = &bhEnd.String
	}
	parseSequenceTimes(&s, createdAt, updatedAt)
	return &s, nil
}

// scanSequenceRow scans a single sequence from *sql.Rows.
// Expects 13 columns: the 11 base columns plus step_count and enrollment_count.
func scanSequenceRow(rows *sql.Rows) (*entity.Sequence, error) {
	var s entity.Sequence
	var desc, bhStart, bhEnd, createdAt, updatedAt sql.NullString
	var stepCount, enrollmentCount int

	err := rows.Scan(
		&s.ID, &s.OrgID, &s.Name, &desc, &s.Status, &s.Timezone,
		&bhStart, &bhEnd, &s.CreatedBy, &createdAt, &updatedAt,
		&stepCount, &enrollmentCount,
	)
	if err != nil {
		return nil, err
	}
	s.StepCount = stepCount
	s.EnrollmentCount = enrollmentCount
	if desc.Valid {
		s.Description = &desc.String
	}
	if bhStart.Valid {
		s.BusinessHoursStart = &bhStart.String
	}
	if bhEnd.Valid {
		s.BusinessHoursEnd = &bhEnd.String
	}
	parseSequenceTimes(&s, createdAt, updatedAt)
	return &s, nil
}

func parseSequenceTimes(s *entity.Sequence, createdAt, updatedAt sql.NullString) {
	if createdAt.Valid {
		if t, err := parseDBTime(createdAt.String); err == nil {
			s.CreatedAt = t
		}
	}
	if updatedAt.Valid {
		if t, err := parseDBTime(updatedAt.String); err == nil {
			s.UpdatedAt = t
		}
	}
}

// scanStepRow scans a single step from *sql.Rows.
func scanStepRow(rows *sql.Rows) (*entity.SequenceStep, error) {
	var step entity.SequenceStep
	var templateID, configJSON, createdAt, updatedAt sql.NullString

	err := rows.Scan(
		&step.ID, &step.SequenceID, &step.StepNumber, &step.StepType,
		&step.DelayDays, &step.DelayHours, &templateID, &configJSON,
		&createdAt, &updatedAt,
	)
	if err != nil {
		return nil, err
	}
	if templateID.Valid {
		step.TemplateID = &templateID.String
	}
	if configJSON.Valid {
		step.ConfigJSON = &configJSON.String
	}
	if createdAt.Valid {
		if t, err := parseDBTime(createdAt.String); err == nil {
			step.CreatedAt = t
		}
	}
	if updatedAt.Valid {
		if t, err := parseDBTime(updatedAt.String); err == nil {
			step.UpdatedAt = t
		}
	}
	return &step, nil
}

// scanEnrollment scans a single enrollment from *sql.Row.
func scanEnrollment(row *sql.Row) (*entity.SequenceEnrollment, error) {
	var e entity.SequenceEnrollment
	var abVariantID, enrolledAt, finishedAt, pausedAt, createdAt, updatedAt sql.NullString

	err := row.Scan(
		&e.ID, &e.SequenceID, &e.ContactID, &e.OrgID, &e.EnrolledBy,
		&e.Status, &e.CurrentStep, &abVariantID,
		&enrolledAt, &finishedAt, &pausedAt, &createdAt, &updatedAt,
	)
	if err != nil {
		return nil, err
	}
	parseEnrollmentOptionals(&e, abVariantID, enrolledAt, finishedAt, pausedAt, createdAt, updatedAt)
	return &e, nil
}

// scanEnrollmentRow scans a single enrollment from *sql.Rows.
func scanEnrollmentRow(rows *sql.Rows) (*entity.SequenceEnrollment, error) {
	var e entity.SequenceEnrollment
	var abVariantID, enrolledAt, finishedAt, pausedAt, createdAt, updatedAt sql.NullString

	err := rows.Scan(
		&e.ID, &e.SequenceID, &e.ContactID, &e.OrgID, &e.EnrolledBy,
		&e.Status, &e.CurrentStep, &abVariantID,
		&enrolledAt, &finishedAt, &pausedAt, &createdAt, &updatedAt,
	)
	if err != nil {
		return nil, err
	}
	parseEnrollmentOptionals(&e, abVariantID, enrolledAt, finishedAt, pausedAt, createdAt, updatedAt)
	return &e, nil
}

func parseEnrollmentOptionals(e *entity.SequenceEnrollment, abVariantID, enrolledAt, finishedAt, pausedAt, createdAt, updatedAt sql.NullString) {
	if abVariantID.Valid {
		e.ABVariantID = &abVariantID.String
	}
	if enrolledAt.Valid {
		if t, err := parseDBTime(enrolledAt.String); err == nil {
			e.EnrolledAt = t
		}
	}
	if finishedAt.Valid {
		if t, err := parseDBTime(finishedAt.String); err == nil {
			e.FinishedAt = &t
		}
	}
	if pausedAt.Valid {
		if t, err := parseDBTime(pausedAt.String); err == nil {
			e.PausedAt = &t
		}
	}
	if createdAt.Valid {
		if t, err := parseDBTime(createdAt.String); err == nil {
			e.CreatedAt = t
		}
	}
	if updatedAt.Valid {
		if t, err := parseDBTime(updatedAt.String); err == nil {
			e.UpdatedAt = t
		}
	}
}

// scanStepExecutionRow scans a single StepExecution from *sql.Rows.
func scanStepExecutionRow(rows *sql.Rows) (*entity.StepExecution, error) {
	var exec entity.StepExecution
	var scheduledAt, executedAt, errorMessage, createdAt, updatedAt sql.NullString

	err := rows.Scan(
		&exec.ID, &exec.EnrollmentID, &exec.StepID, &exec.OrgID, &exec.Status,
		&scheduledAt, &executedAt, &errorMessage, &createdAt, &updatedAt,
	)
	if err != nil {
		return nil, err
	}
	if scheduledAt.Valid {
		if t, err := parseDBTime(scheduledAt.String); err == nil {
			exec.ScheduledAt = &t
		}
	}
	if executedAt.Valid {
		if t, err := parseDBTime(executedAt.String); err == nil {
			exec.ExecutedAt = &t
		}
	}
	if errorMessage.Valid {
		exec.ErrorMessage = &errorMessage.String
	}
	if createdAt.Valid {
		if t, err := parseDBTime(createdAt.String); err == nil {
			exec.CreatedAt = t
		}
	}
	if updatedAt.Valid {
		if t, err := parseDBTime(updatedAt.String); err == nil {
			exec.UpdatedAt = t
		}
	}
	return &exec, nil
}

// parseDBTime parses SQLite timestamp strings into time.Time.
func parseDBTime(s string) (time.Time, error) {
	formats := []string{
		"2006-01-02T15:04:05Z",
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05",
	}
	for _, format := range formats {
		if t, err := time.Parse(format, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, nil
}
