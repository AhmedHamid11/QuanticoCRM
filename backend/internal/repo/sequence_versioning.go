package repo

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"time"

	"github.com/fastcrm/backend/internal/entity"
	"github.com/google/uuid"
)

// ========== Sequence Versioning (Phase 37) ==========

// CreateSequenceVersion inserts a snapshot of sequence steps at the time of activation.
// versionNumber must be unique per sequence (enforced by DB UNIQUE constraint).
func (r *SequenceRepo) CreateSequenceVersion(ctx context.Context, orgID, sequenceID, versionID string, versionNumber int, stepsJSON string) error {
	now := time.Now().UTC().Format("2006-01-02T15:04:05Z")
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO sequence_versions (id, sequence_id, version_number, steps_snapshot_json, created_at)
		VALUES (?, ?, ?, ?, ?)
	`, versionID, sequenceID, versionNumber, stepsJSON, now)
	return err
}

// GetLatestVersionNumber returns the highest version_number for a sequence.
// Returns 0 if no versions exist (sequence was never activated with versioning).
func (r *SequenceRepo) GetLatestVersionNumber(ctx context.Context, sequenceID string) (int, error) {
	var maxVersion sql.NullInt64
	err := r.db.QueryRowContext(ctx,
		"SELECT MAX(version_number) FROM sequence_versions WHERE sequence_id = ?",
		sequenceID,
	).Scan(&maxVersion)
	if err != nil {
		return 0, err
	}
	if !maxVersion.Valid {
		return 0, nil
	}
	return int(maxVersion.Int64), nil
}

// GetLatestVersionID returns the id of the most recent version for a sequence.
// Returns "" if no versions exist (backward compatible — pre-versioning sequences).
func (r *SequenceRepo) GetLatestVersionID(ctx context.Context, sequenceID string) (string, error) {
	var id sql.NullString
	err := r.db.QueryRowContext(ctx,
		"SELECT id FROM sequence_versions WHERE sequence_id = ? ORDER BY version_number DESC LIMIT 1",
		sequenceID,
	).Scan(&id)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	if !id.Valid {
		return "", nil
	}
	return id.String, nil
}

// GetVersionedSteps fetches and deserializes the steps snapshot stored in a version.
// Returns an empty slice if the version does not exist.
func (r *SequenceRepo) GetVersionedSteps(ctx context.Context, versionID string) ([]entity.SequenceStep, error) {
	var snapshotJSON sql.NullString
	err := r.db.QueryRowContext(ctx,
		"SELECT steps_snapshot_json FROM sequence_versions WHERE id = ?",
		versionID,
	).Scan(&snapshotJSON)
	if err == sql.ErrNoRows {
		return []entity.SequenceStep{}, nil
	}
	if err != nil {
		return nil, err
	}
	if !snapshotJSON.Valid || snapshotJSON.String == "" {
		return []entity.SequenceStep{}, nil
	}
	var steps []entity.SequenceStep
	if jsonErr := json.Unmarshal([]byte(snapshotJSON.String), &steps); jsonErr != nil {
		return nil, jsonErr
	}
	return steps, nil
}

// SetEnrollmentVersionID pins an enrollment to a specific sequence version.
// Called after enrollment creation to record which snapshot the contact enrolled on.
func (r *SequenceRepo) SetEnrollmentVersionID(ctx context.Context, enrollmentID, versionID string) error {
	_, err := r.db.ExecContext(ctx,
		"UPDATE sequence_enrollments SET version_id = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?",
		versionID, enrollmentID,
	)
	return err
}

// ========== Sequence Cloning (Phase 37) ==========

// CloneSequence creates an independent deep copy of a sequence and all its steps.
// The clone is created in draft status with the provided name and clonedBy as creator.
// AB test variants for email steps are also cloned.
// Runs in a transaction when the underlying connection is *sql.DB.
func (r *SequenceRepo) CloneSequence(ctx context.Context, orgID, sourceID, newID, newName, clonedBy string) error {
	sqlDB, ok := r.db.(*sql.DB)
	if !ok {
		return r.cloneSequenceStmts(ctx, orgID, sourceID, newID, newName, clonedBy)
	}

	tx, err := sqlDB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck

	if err := doClone(ctx, tx, orgID, sourceID, newID, newName, clonedBy); err != nil {
		return err
	}
	return tx.Commit()
}

// cloneSequenceStmts is the non-transactional fallback for CloneSequence when the
// underlying connection does not support BeginTx (e.g. Turso wrapper).
func (r *SequenceRepo) cloneSequenceStmts(ctx context.Context, orgID, sourceID, newID, newName, clonedBy string) error {
	return doClone(ctx, r.db, orgID, sourceID, newID, newName, clonedBy)
}

// dbExecer abstracts both *sql.DB and *sql.Tx so doClone can work with either.
type dbExecer interface {
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row
}

// doClone performs the actual clone operations against any dbExecer.
func doClone(ctx context.Context, conn dbExecer, orgID, sourceID, newID, newName, clonedBy string) error {
	now := time.Now().UTC().Format("2006-01-02T15:04:05Z")

	// Fetch source sequence fields needed for the clone
	var srcTimezone, srcBHStart, srcBHEnd string
	var srcDesc sql.NullString
	err := conn.QueryRowContext(ctx, `
		SELECT COALESCE(timezone, 'America/New_York'),
		       COALESCE(business_hours_start, '09:00'),
		       COALESCE(business_hours_end, '17:00'),
		       description
		FROM sequences WHERE id = ? AND org_id = ?
	`, sourceID, orgID).Scan(&srcTimezone, &srcBHStart, &srcBHEnd, &srcDesc)
	if err != nil {
		return err
	}

	// Insert cloned sequence in draft status
	var descArg interface{}
	if srcDesc.Valid {
		descArg = srcDesc.String
	}
	_, err = conn.ExecContext(ctx, `
		INSERT INTO sequences
		    (id, org_id, name, description, status, timezone,
		     business_hours_start, business_hours_end, created_by,
		     created_at, updated_at)
		VALUES (?, ?, ?, ?, 'draft', ?, ?, ?, ?, ?, ?)
	`, newID, orgID, newName, descArg, srcTimezone, srcBHStart, srcBHEnd, clonedBy, now, now)
	if err != nil {
		return err
	}

	// Clone steps and collect email step ID mappings for variant cloning
	stepRows, err := conn.QueryContext(ctx, `
		SELECT id, step_number, step_type, delay_days, delay_hours, template_id, config_json
		FROM sequence_steps WHERE sequence_id = ? ORDER BY step_number ASC
	`, sourceID)
	if err != nil {
		return err
	}
	defer stepRows.Close()

	type stepMap struct{ oldID, newID string }
	var emailStepMaps []stepMap

	for stepRows.Next() {
		var oldStepID, stepType string
		var stepNumber, delayDays, delayHours int
		var templateID, configJSON sql.NullString
		if err := stepRows.Scan(&oldStepID, &stepNumber, &stepType, &delayDays, &delayHours, &templateID, &configJSON); err != nil {
			return err
		}
		newStepID := uuid.New().String()
		var tplArg, cfgArg interface{}
		if templateID.Valid {
			tplArg = templateID.String
		}
		if configJSON.Valid {
			cfgArg = configJSON.String
		}
		_, err = conn.ExecContext(ctx, `
			INSERT INTO sequence_steps
			    (id, sequence_id, step_number, step_type, delay_days, delay_hours,
			     template_id, config_json, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, newStepID, newID, stepNumber, stepType, delayDays, delayHours, tplArg, cfgArg, now, now)
		if err != nil {
			return err
		}
		if stepType == entity.StepTypeEmail {
			emailStepMaps = append(emailStepMaps, stepMap{oldID: oldStepID, newID: newStepID})
		}
	}
	if err := stepRows.Err(); err != nil {
		return err
	}

	// Clone AB test variants for each email step
	for _, sm := range emailStepMaps {
		vRows, err := conn.QueryContext(ctx, `
			SELECT variant_label, traffic_pct, subject_override, body_html_override
			FROM ab_test_variants WHERE step_id = ?
		`, sm.oldID)
		if err != nil {
			return err
		}
		for vRows.Next() {
			var label string
			var trafficPct int
			var subjectOverride, bodyHTMLOverride sql.NullString
			if err := vRows.Scan(&label, &trafficPct, &subjectOverride, &bodyHTMLOverride); err != nil {
				vRows.Close()
				return err
			}
			var subArg, bodyArg interface{}
			if subjectOverride.Valid {
				subArg = subjectOverride.String
			}
			if bodyHTMLOverride.Valid {
				bodyArg = bodyHTMLOverride.String
			}
			_, err = conn.ExecContext(ctx, `
				INSERT INTO ab_test_variants
				    (id, step_id, variant_label, traffic_pct, subject_override,
				     body_html_override, is_winner, created_at, updated_at)
				VALUES (?, ?, ?, ?, ?, ?, 0, ?, ?)
			`, uuid.New().String(), sm.newID, label, trafficPct, subArg, bodyArg, now, now)
			if err != nil {
				vRows.Close()
				return err
			}
		}
		vRows.Close()
		if err := vRows.Err(); err != nil {
			return err
		}
	}

	return nil
}

// BuildCloneName returns the clone name for a sequence.
// If the source already ends in " (Copy)", strips that suffix before appending
// to prevent runaway suffixes like "Foo (Copy) (Copy)".
func BuildCloneName(sourceName string) string {
	return strings.TrimSuffix(sourceName, " (Copy)") + " (Copy)"
}
