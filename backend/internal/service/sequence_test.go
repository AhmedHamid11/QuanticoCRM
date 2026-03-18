package service_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"github.com/fastcrm/backend/internal/entity"
	"github.com/fastcrm/backend/internal/repo"
	"github.com/fastcrm/backend/internal/service"
)

// setupTestDB creates an in-memory SQLite DB with the required engagement tables.
func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory sqlite: %v", err)
	}

	schema := `
	CREATE TABLE IF NOT EXISTS sequences (
		id TEXT PRIMARY KEY,
		org_id TEXT NOT NULL,
		name TEXT NOT NULL,
		description TEXT DEFAULT '',
		status TEXT NOT NULL DEFAULT 'draft',
		timezone TEXT NOT NULL DEFAULT 'America/New_York',
		business_hours_start TEXT,
		business_hours_end TEXT,
		created_by TEXT NOT NULL,
		created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS sequence_steps (
		id TEXT PRIMARY KEY,
		sequence_id TEXT NOT NULL,
		step_number INTEGER NOT NULL,
		step_type TEXT NOT NULL,
		delay_days INTEGER NOT NULL DEFAULT 0,
		delay_hours INTEGER NOT NULL DEFAULT 0,
		template_id TEXT,
		config_json TEXT,
		created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS sequence_enrollments (
		id TEXT PRIMARY KEY,
		sequence_id TEXT NOT NULL,
		contact_id TEXT NOT NULL,
		org_id TEXT NOT NULL,
		enrolled_by TEXT NOT NULL,
		status TEXT NOT NULL DEFAULT 'enrolled',
		current_step INTEGER NOT NULL DEFAULT 0,
		ab_variant_id TEXT,
		enrolled_at TEXT,
		finished_at TEXT,
		paused_at TEXT,
		created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(sequence_id, contact_id)
	);

	CREATE TABLE IF NOT EXISTS sequence_step_executions (
		id TEXT PRIMARY KEY,
		enrollment_id TEXT NOT NULL,
		step_id TEXT NOT NULL,
		org_id TEXT NOT NULL,
		status TEXT NOT NULL DEFAULT 'pending',
		scheduled_at TEXT,
		executed_at TEXT,
		error_message TEXT,
		created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS opt_out_list (
		id TEXT PRIMARY KEY,
		org_id TEXT NOT NULL,
		contact_id TEXT NOT NULL,
		channel TEXT NOT NULL,
		reason TEXT,
		opted_out_at TEXT,
		created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(org_id, contact_id, channel)
	);

	CREATE TABLE IF NOT EXISTS contacts (
		id TEXT PRIMARY KEY,
		org_id TEXT NOT NULL,
		first_name TEXT,
		last_name TEXT,
		email_address TEXT,
		phone_number TEXT,
		account_name TEXT,
		status TEXT,
		created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
	);
	`

	if _, err := db.Exec(schema); err != nil {
		t.Fatalf("failed to create schema: %v", err)
	}
	return db
}

// insertEnrollment inserts a raw enrollment into the test DB.
func insertEnrollment(t *testing.T, db *sql.DB, id, sequenceID, contactID, orgID, status string) {
	t.Helper()
	_, err := db.Exec(`
		INSERT INTO sequence_enrollments (id, sequence_id, contact_id, org_id, enrolled_by, status, enrolled_at)
		VALUES (?, ?, ?, ?, 'user1', ?, CURRENT_TIMESTAMP)
	`, id, sequenceID, contactID, orgID, status)
	if err != nil {
		t.Fatalf("failed to insert enrollment: %v", err)
	}
}

// insertSequence inserts a raw sequence into the test DB.
func insertSequence(t *testing.T, db *sql.DB, id, orgID, name, status string) {
	t.Helper()
	_, err := db.Exec(`
		INSERT INTO sequences (id, org_id, name, status, created_by)
		VALUES (?, ?, ?, ?, 'user1')
	`, id, orgID, name, status)
	if err != nil {
		t.Fatalf("failed to insert sequence: %v", err)
	}
}

// insertStep inserts a raw step into the test DB.
func insertStep(t *testing.T, db *sql.DB, id, sequenceID string, stepNumber int) {
	t.Helper()
	_, err := db.Exec(`
		INSERT INTO sequence_steps (id, sequence_id, step_number, step_type)
		VALUES (?, ?, ?, 'email')
	`, id, sequenceID, stepNumber)
	if err != nil {
		t.Fatalf("failed to insert step: %v", err)
	}
}

// ===========================
// State Machine Tests
// ===========================

// TestEnrollmentTransition_LegalTransitions verifies all valid FSM transitions succeed.
func TestEnrollmentTransition_LegalTransitions(t *testing.T) {
	legalTransitions := []struct {
		from string
		to   string
	}{
		{entity.EnrollmentStatusEnrolled, entity.EnrollmentStatusActive},
		{entity.EnrollmentStatusActive, entity.EnrollmentStatusFinished},
		{entity.EnrollmentStatusActive, entity.EnrollmentStatusPaused},
		{entity.EnrollmentStatusActive, entity.EnrollmentStatusReplied},
		{entity.EnrollmentStatusActive, entity.EnrollmentStatusBounced},
		{entity.EnrollmentStatusActive, entity.EnrollmentStatusOptedOut},
		{entity.EnrollmentStatusPaused, entity.EnrollmentStatusActive},
		{entity.EnrollmentStatusPaused, entity.EnrollmentStatusFinished},
	}

	svc := service.NewSequenceService(nil) // repo not needed for FSM logic

	for _, tc := range legalTransitions {
		t.Run(tc.from+"->"+tc.to, func(t *testing.T) {
			enrollment := &entity.SequenceEnrollment{Status: tc.from}
			err := svc.TransitionEnrollment(enrollment, tc.to)
			if err != nil {
				t.Errorf("expected legal transition %s->%s to succeed, got error: %v", tc.from, tc.to, err)
			}
			if enrollment.Status != tc.to {
				t.Errorf("expected status to be %s after transition, got %s", tc.to, enrollment.Status)
			}
		})
	}
}

// TestEnrollmentTransition_IllegalTransitions verifies invalid FSM transitions are rejected.
func TestEnrollmentTransition_IllegalTransitions(t *testing.T) {
	illegalTransitions := []struct {
		from string
		to   string
	}{
		{entity.EnrollmentStatusFinished, entity.EnrollmentStatusActive},
		{entity.EnrollmentStatusOptedOut, entity.EnrollmentStatusActive},
		{entity.EnrollmentStatusEnrolled, entity.EnrollmentStatusFinished},
	}

	svc := service.NewSequenceService(nil)

	for _, tc := range illegalTransitions {
		t.Run(tc.from+"->"+tc.to, func(t *testing.T) {
			enrollment := &entity.SequenceEnrollment{Status: tc.from}
			err := svc.TransitionEnrollment(enrollment, tc.to)
			if err == nil {
				t.Errorf("expected illegal transition %s->%s to fail, but got no error", tc.from, tc.to)
			}
		})
	}
}

// ===========================
// Bulk Enrollment Tests
// ===========================

// TestBulkEnroll_SkipsAlreadyEnrolled verifies that bulk enrollment skips contacts
// already enrolled in the sequence and returns the correct summary.
func TestBulkEnroll_SkipsAlreadyEnrolled(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	orgID := "org1"
	sequenceID := "seq1"
	insertSequence(t, db, sequenceID, orgID, "Sequence 1", entity.SequenceStatusActive)
	insertStep(t, db, "step1", sequenceID, 1)

	// Pre-enroll contacts 4 and 5
	insertEnrollment(t, db, "enr4", sequenceID, "contact4", orgID, entity.EnrollmentStatusActive)
	insertEnrollment(t, db, "enr5", sequenceID, "contact5", orgID, entity.EnrollmentStatusActive)

	seqRepo := repo.NewSequenceRepo(db)
	svc := service.NewSequenceService(seqRepo)

	contactIDs := []string{"contact1", "contact2", "contact3", "contact4", "contact5"}
	result, err := svc.BulkEnroll(context.Background(), orgID, sequenceID, contactIDs, "user1")
	if err != nil {
		t.Fatalf("BulkEnroll returned unexpected error: %v", err)
	}

	if result.Enrolled != 3 {
		t.Errorf("expected 3 enrolled, got %d", result.Enrolled)
	}
	if result.Skipped != 2 {
		t.Errorf("expected 2 skipped, got %d", result.Skipped)
	}
	if len(result.SkippedContacts) != 2 {
		t.Errorf("expected 2 skipped contacts, got %d", len(result.SkippedContacts))
	}
}

// ===========================
// Enrollment Overlap Tests
// ===========================

// TestEnrollContact_OverlapWarning verifies that enrolling a contact already in an active
// sequence returns a structured warning (not an error).
func TestEnrollContact_OverlapWarning(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	orgID := "org1"
	seqID1 := "seq1"
	seqID2 := "seq2"

	insertSequence(t, db, seqID1, orgID, "Existing Sequence", entity.SequenceStatusActive)
	insertSequence(t, db, seqID2, orgID, "New Sequence", entity.SequenceStatusActive)
	insertStep(t, db, "step1", seqID2, 1)

	// Contact is already active in seqID1
	insertEnrollment(t, db, "enr1", seqID1, "contact1", orgID, entity.EnrollmentStatusActive)

	seqRepo := repo.NewSequenceRepo(db)
	svc := service.NewSequenceService(seqRepo)

	result, err := svc.EnrollContact(context.Background(), orgID, seqID2, "contact1", "user1", false)
	if err != nil {
		t.Fatalf("EnrollContact returned unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if !result.Warning {
		t.Errorf("expected warning=true for overlap, got false")
	}
	if result.ExistingSequence == "" {
		t.Errorf("expected non-empty existingSequence name in warning")
	}
}

// ===========================
// Suppression Tests
// ===========================

// TestCheckSuppression_OptOutList verifies that a contact on the opt-out list is suppressed.
func TestCheckSuppression_OptOutList(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	orgID := "org1"
	contactID := "contact1"

	// Add contact to opt-out list for email
	_, err := db.Exec(`
		INSERT INTO opt_out_list (id, org_id, contact_id, channel, opted_out_at)
		VALUES ('optout1', ?, ?, 'email', CURRENT_TIMESTAMP)
	`, orgID, contactID)
	if err != nil {
		t.Fatalf("failed to insert opt-out: %v", err)
	}

	seqRepo := repo.NewSequenceRepo(db)
	svc := service.NewSequenceService(seqRepo)

	result, err := svc.CheckSuppression(context.Background(), orgID, contactID, nil)
	if err != nil {
		t.Fatalf("CheckSuppression returned unexpected error: %v", err)
	}
	if !result.Suppressed {
		t.Errorf("expected contact to be suppressed via opt-out list")
	}
}

// TestCheckSuppression_StatusFieldRule verifies that a contact matching a status-field
// suppression rule is suppressed.
func TestCheckSuppression_StatusFieldRule(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	orgID := "org1"
	contactID := "contact1"

	// Insert a contact with status "customer"
	_, err := db.Exec(`
		INSERT INTO contacts (id, org_id, first_name, last_name, status)
		VALUES (?, ?, 'John', 'Doe', 'customer')
	`, contactID, orgID)
	if err != nil {
		t.Fatalf("failed to insert contact: %v", err)
	}

	seqRepo := repo.NewSequenceRepo(db)
	svc := service.NewSequenceService(seqRepo)

	rules := []service.SuppressionRule{
		{Field: "status", Operator: "eq", Value: "customer"},
	}

	result, err := svc.CheckSuppression(context.Background(), orgID, contactID, rules)
	if err != nil {
		t.Fatalf("CheckSuppression returned unexpected error: %v", err)
	}
	if !result.Suppressed {
		t.Errorf("expected contact to be suppressed via status-field rule")
	}
}

// TestCheckSuppression_NoMatch verifies that a contact with no opt-out and no matching
// suppression rule is NOT suppressed.
func TestCheckSuppression_NoMatch(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	orgID := "org1"
	contactID := "contact1"

	// Insert a contact with status "prospect"
	_, err := db.Exec(`
		INSERT INTO contacts (id, org_id, first_name, last_name, status)
		VALUES (?, ?, 'Jane', 'Smith', 'prospect')
	`, contactID, orgID)
	if err != nil {
		t.Fatalf("failed to insert contact: %v", err)
	}

	seqRepo := repo.NewSequenceRepo(db)
	svc := service.NewSequenceService(seqRepo)

	rules := []service.SuppressionRule{
		{Field: "status", Operator: "eq", Value: "customer"},
	}

	result, err := svc.CheckSuppression(context.Background(), orgID, contactID, rules)
	if err != nil {
		t.Fatalf("CheckSuppression returned unexpected error: %v", err)
	}
	if result.Suppressed {
		t.Errorf("expected contact NOT to be suppressed")
	}
}

// Ensure time import is used.
var _ = time.Now
