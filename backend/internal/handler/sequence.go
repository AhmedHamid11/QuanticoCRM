package handler

import (
	"log"
	"time"

	"github.com/fastcrm/backend/internal/entity"
	"github.com/fastcrm/backend/internal/middleware"
	"github.com/fastcrm/backend/internal/repo"
	"github.com/fastcrm/backend/internal/service"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)


// SequenceHandler handles HTTP requests for sequence CRUD, step management,
// and contact enrollment.
type SequenceHandler struct {
	sequenceService   *service.SequenceService
	sequenceRepo      *repo.SequenceRepo
	engagementRepo    *repo.EngagementRepo
	sequenceScheduler *service.SequenceScheduler
	abService         *service.ABService
}

// NewSequenceHandler creates a new SequenceHandler.
func NewSequenceHandler(
	svc *service.SequenceService,
	seqRepo *repo.SequenceRepo,
	engagementRepo *repo.EngagementRepo,
) *SequenceHandler {
	return &SequenceHandler{
		sequenceService: svc,
		sequenceRepo:    seqRepo,
		engagementRepo:  engagementRepo,
	}
}

// SetScheduler wires the SequenceScheduler so the handler can register org
// DBs when a new enrollment triggers polling for that org.
func (h *SequenceHandler) SetScheduler(scheduler *service.SequenceScheduler) {
	h.sequenceScheduler = scheduler
}

// SetABService wires the ABService for variant CRUD and stats endpoints.
func (h *SequenceHandler) SetABService(ab *service.ABService) {
	h.abService = ab
}

// RegisterRoutes registers all sequence routes on the admin-protected group.
func (h *SequenceHandler) RegisterRoutes(router fiber.Router) {
	log.Println("[STARTUP] Registering sequence routes")
	g := router.Group("/sequences")

	// Sequence CRUD
	g.Post("", h.CreateSequence)
	g.Get("", h.ListSequences)
	g.Get("/:id", h.GetSequence)
	g.Put("/:id", h.UpdateSequence)
	g.Delete("/:id", h.DeleteSequence)

	// Step management
	g.Post("/:id/steps", h.AddStep)
	g.Put("/:id/steps/:stepId", h.UpdateStep)
	g.Delete("/:id/steps/:stepId", h.RemoveStep)

	// Status transitions
	g.Post("/:id/activate", h.ActivateSequence)
	g.Post("/:id/pause", h.PauseSequence)

	// Enrollment
	g.Post("/:id/enroll", h.EnrollContact)
	g.Post("/:id/enroll-bulk", h.BulkEnroll)

	// Clone (Sequence Library)
	g.Post("/:id/clone", h.CloneSequence)

	// A/B variant management for email steps
	g.Get("/:id/steps/:stepId/variants", h.ListVariants)
	g.Post("/:id/steps/:stepId/variants", h.CreateVariant)
	g.Patch("/:id/steps/:stepId/variants/:variantId", h.UpdateVariant)
	g.Delete("/:id/steps/:stepId/variants/:variantId", h.DeleteVariant)
	g.Post("/:id/steps/:stepId/variants/:variantId/promote", h.PromoteVariant)
}

// RegisterTaskRoutes registers the PRA task queue routes on the user-accessible group.
// These routes are accessible to all authenticated users (not just admins).
func (h *SequenceHandler) RegisterTaskRoutes(router fiber.Router) {
	log.Println("[STARTUP] Registering task queue routes")
	g := router.Group("/engagement/tasks")
	g.Get("", h.ListTasks)
	g.Post("/:executionId/complete", h.CompleteTask)
	g.Post("/:executionId/skip", h.SkipTask)
	g.Put("/:executionId/reschedule", h.RescheduleTask)

	// Contact activity timeline — accessible by PRAs alongside task routes.
	router.Get("/contacts/:contactId/activity", h.GetContactActivity)
}

// ========== Sequence CRUD ==========

// CreateSequence creates a new sequence in draft status.
// POST /sequences
func (h *SequenceHandler) CreateSequence(c *fiber.Ctx) error {
	orgID, userID := getSequenceContext(c)
	tenantDB := middleware.GetTenantDBConn(c)

	var input struct {
		Name               string                    `json:"name"`
		Description        string                    `json:"description"`
		Timezone           string                    `json:"timezone"`
		BusinessHoursStart string                    `json:"businessHoursStart"`
		BusinessHoursEnd   string                    `json:"businessHoursEnd"`
		SuppressionRules   []service.SuppressionRule `json:"suppressionRules"`
	}
	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	if input.Name == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "name is required"})
	}

	timezone := input.Timezone
	if timezone == "" {
		timezone = "America/New_York"
	}

	now := time.Now().UTC()
	seq := &entity.Sequence{
		ID:        uuid.New().String(),
		OrgID:     orgID,
		Name:      input.Name,
		Status:    entity.SequenceStatusDraft,
		Timezone:  timezone,
		CreatedBy: userID,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if input.Description != "" {
		seq.Description = &input.Description
	}
	if input.BusinessHoursStart != "" {
		seq.BusinessHoursStart = &input.BusinessHoursStart
	}
	if input.BusinessHoursEnd != "" {
		seq.BusinessHoursEnd = &input.BusinessHoursEnd
	}

	if err := h.sequenceRepo.WithDB(tenantDB).CreateSequence(c.Context(), seq); err != nil {
		log.Printf("[Sequence] CreateSequence error for org %s: %v", orgID, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to create sequence"})
	}

	return c.Status(fiber.StatusCreated).JSON(seq)
}

// ListSequences returns all sequences for the org.
// GET /sequences
func (h *SequenceHandler) ListSequences(c *fiber.Ctx) error {
	orgID, _ := getSequenceContext(c)
	tenantDB := middleware.GetTenantDBConn(c)

	sequences, err := h.sequenceRepo.WithDB(tenantDB).ListSequences(c.Context(), orgID)
	if err != nil {
		log.Printf("[Sequence] ListSequences error for org %s: %v", orgID, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to list sequences"})
	}
	if sequences == nil {
		sequences = []*entity.Sequence{}
	}
	return c.JSON(sequences)
}

// GetSequence returns a single sequence with all its steps.
// GET /sequences/:id
func (h *SequenceHandler) GetSequence(c *fiber.Ctx) error {
	orgID, _ := getSequenceContext(c)
	id := c.Params("id")
	tenantDB := middleware.GetTenantDBConn(c)
	r := h.sequenceRepo.WithDB(tenantDB)

	seq, err := r.GetSequence(c.Context(), orgID, id)
	if err != nil {
		log.Printf("[Sequence] GetSequence error for org %s id %s: %v", orgID, id, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to get sequence"})
	}
	if seq == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "sequence not found"})
	}

	steps, err := r.ListStepsBySequence(c.Context(), id)
	if err != nil {
		log.Printf("[Sequence] ListStepsBySequence error for seq %s: %v", id, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to get sequence steps"})
	}
	if steps == nil {
		steps = []*entity.SequenceStep{}
	}

	return c.JSON(fiber.Map{
		"sequence": seq,
		"steps":    steps,
	})
}

// UpdateSequence updates a sequence's mutable fields.
// Only allowed if status=draft or status=paused.
// PUT /sequences/:id
func (h *SequenceHandler) UpdateSequence(c *fiber.Ctx) error {
	orgID, _ := getSequenceContext(c)
	id := c.Params("id")
	tenantDB := middleware.GetTenantDBConn(c)
	r := h.sequenceRepo.WithDB(tenantDB)

	existing, err := r.GetSequence(c.Context(), orgID, id)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to fetch sequence"})
	}
	if existing == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "sequence not found"})
	}
	if existing.Status != entity.SequenceStatusDraft && existing.Status != entity.SequenceStatusPaused {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "sequence can only be updated when in draft or paused status"})
	}

	var input struct {
		Name               string                    `json:"name"`
		Description        string                    `json:"description"`
		Timezone           string                    `json:"timezone"`
		BusinessHoursStart string                    `json:"businessHoursStart"`
		BusinessHoursEnd   string                    `json:"businessHoursEnd"`
		SuppressionRules   []service.SuppressionRule `json:"suppressionRules"`
	}
	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	if input.Name != "" {
		existing.Name = input.Name
	}
	if input.Description != "" {
		existing.Description = &input.Description
	}
	if input.Timezone != "" {
		existing.Timezone = input.Timezone
	}
	if input.BusinessHoursStart != "" {
		existing.BusinessHoursStart = &input.BusinessHoursStart
	}
	if input.BusinessHoursEnd != "" {
		existing.BusinessHoursEnd = &input.BusinessHoursEnd
	}

	if err := r.UpdateSequence(c.Context(), existing); err != nil {
		log.Printf("[Sequence] UpdateSequence error for org %s id %s: %v", orgID, id, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to update sequence"})
	}

	return c.JSON(existing)
}

// DeleteSequence removes a sequence. Only allowed if status=draft.
// DELETE /sequences/:id
func (h *SequenceHandler) DeleteSequence(c *fiber.Ctx) error {
	orgID, _ := getSequenceContext(c)
	id := c.Params("id")
	tenantDB := middleware.GetTenantDBConn(c)
	r := h.sequenceRepo.WithDB(tenantDB)

	existing, err := r.GetSequence(c.Context(), orgID, id)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to fetch sequence"})
	}
	if existing == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "sequence not found"})
	}
	if existing.Status != entity.SequenceStatusDraft {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "only draft sequences can be deleted"})
	}

	if err := r.DeleteSequence(c.Context(), orgID, id); err != nil {
		log.Printf("[Sequence] DeleteSequence error for org %s id %s: %v", orgID, id, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to delete sequence"})
	}

	return c.JSON(fiber.Map{"status": "deleted"})
}

// ========== Step Management ==========

// AddStep adds a step to a sequence.
// POST /sequences/:id/steps
func (h *SequenceHandler) AddStep(c *fiber.Ctx) error {
	orgID, _ := getSequenceContext(c)
	sequenceID := c.Params("id")
	tenantDB := middleware.GetTenantDBConn(c)
	r := h.sequenceRepo.WithDB(tenantDB)

	// Verify sequence exists and belongs to this org
	seq, err := r.GetSequence(c.Context(), orgID, sequenceID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to fetch sequence"})
	}
	if seq == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "sequence not found"})
	}

	var input struct {
		StepNumber int    `json:"stepNumber"`
		StepType   string `json:"stepType"`
		DelayDays  int    `json:"delayDays"`
		DelayHours int    `json:"delayHours"`
		TemplateID string `json:"templateId"`
		ConfigJSON string `json:"configJson"`
	}
	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	// Validate step type
	if !isValidStepType(input.StepType) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid stepType — must be one of: email, call, sms, linkedin, custom",
		})
	}

	now := time.Now().UTC()
	step := &entity.SequenceStep{
		ID:         uuid.New().String(),
		SequenceID: sequenceID,
		StepNumber: input.StepNumber,
		StepType:   input.StepType,
		DelayDays:  input.DelayDays,
		DelayHours: input.DelayHours,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if input.TemplateID != "" {
		step.TemplateID = &input.TemplateID
	}
	if input.ConfigJSON != "" {
		step.ConfigJSON = &input.ConfigJSON
	}

	if err := r.CreateStep(c.Context(), step); err != nil {
		log.Printf("[Sequence] AddStep error for seq %s: %v", sequenceID, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to add step"})
	}

	return c.Status(fiber.StatusCreated).JSON(step)
}

// UpdateStep updates a step's configuration.
// PUT /sequences/:id/steps/:stepId
func (h *SequenceHandler) UpdateStep(c *fiber.Ctx) error {
	orgID, _ := getSequenceContext(c)
	sequenceID := c.Params("id")
	stepID := c.Params("stepId")
	tenantDB := middleware.GetTenantDBConn(c)
	r := h.sequenceRepo.WithDB(tenantDB)

	// Verify sequence exists and belongs to this org
	seq, err := r.GetSequence(c.Context(), orgID, sequenceID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to fetch sequence"})
	}
	if seq == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "sequence not found"})
	}

	var input struct {
		StepNumber int    `json:"stepNumber"`
		StepType   string `json:"stepType"`
		DelayDays  int    `json:"delayDays"`
		DelayHours int    `json:"delayHours"`
		TemplateID string `json:"templateId"`
		ConfigJSON string `json:"configJson"`
	}
	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	if input.StepType != "" && !isValidStepType(input.StepType) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid stepType — must be one of: email, call, sms, linkedin, custom",
		})
	}

	step := &entity.SequenceStep{
		ID:         stepID,
		SequenceID: sequenceID,
		StepNumber: input.StepNumber,
		StepType:   input.StepType,
		DelayDays:  input.DelayDays,
		DelayHours: input.DelayHours,
	}
	if input.TemplateID != "" {
		step.TemplateID = &input.TemplateID
	}
	if input.ConfigJSON != "" {
		step.ConfigJSON = &input.ConfigJSON
	}

	if err := r.UpdateStep(c.Context(), step); err != nil {
		log.Printf("[Sequence] UpdateStep error for step %s: %v", stepID, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to update step"})
	}

	return c.JSON(step)
}

// RemoveStep removes a step from a sequence.
// DELETE /sequences/:id/steps/:stepId
func (h *SequenceHandler) RemoveStep(c *fiber.Ctx) error {
	orgID, _ := getSequenceContext(c)
	sequenceID := c.Params("id")
	stepID := c.Params("stepId")
	tenantDB := middleware.GetTenantDBConn(c)
	r := h.sequenceRepo.WithDB(tenantDB)

	// Verify sequence exists and belongs to this org
	seq, err := r.GetSequence(c.Context(), orgID, sequenceID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to fetch sequence"})
	}
	if seq == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "sequence not found"})
	}

	if err := r.DeleteStep(c.Context(), sequenceID, stepID); err != nil {
		log.Printf("[Sequence] RemoveStep error for step %s: %v", stepID, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to remove step"})
	}

	return c.JSON(fiber.Map{"status": "deleted"})
}

// ========== Status Transitions ==========

// ActivateSequence transitions a sequence from draft to active.
// Requires at least 1 step to exist.
// POST /sequences/:id/activate
func (h *SequenceHandler) ActivateSequence(c *fiber.Ctx) error {
	orgID, _ := getSequenceContext(c)
	id := c.Params("id")
	tenantDB := middleware.GetTenantDBConn(c)
	r := h.sequenceRepo.WithDB(tenantDB)

	seq, err := r.GetSequence(c.Context(), orgID, id)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to fetch sequence"})
	}
	if seq == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "sequence not found"})
	}
	if seq.Status != entity.SequenceStatusDraft && seq.Status != entity.SequenceStatusPaused {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "sequence must be in draft or paused status to activate"})
	}

	steps, err := r.ListStepsBySequence(c.Context(), id)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to check steps"})
	}
	if len(steps) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "sequence must have at least one step before activation"})
	}

	if err := r.ActivateSequence(c.Context(), orgID, id); err != nil {
		log.Printf("[Sequence] ActivateSequence error for org %s id %s: %v", orgID, id, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to activate sequence"})
	}

	// Snapshot current steps into sequence_versions for enrollment pinning.
	// Non-fatal: if version creation fails, activation still succeeds.
	if err := createVersionSnapshot(c, r, orgID, id, steps); err != nil {
		log.Printf("[Sequence] ActivateSequence: version snapshot failed for seq %s (non-fatal): %v", id, err)
	}

	return c.JSON(fiber.Map{"status": "active", "id": id})
}

// PauseSequence transitions a sequence from active to paused.
// POST /sequences/:id/pause
func (h *SequenceHandler) PauseSequence(c *fiber.Ctx) error {
	orgID, _ := getSequenceContext(c)
	id := c.Params("id")
	tenantDB := middleware.GetTenantDBConn(c)
	r := h.sequenceRepo.WithDB(tenantDB)

	seq, err := r.GetSequence(c.Context(), orgID, id)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to fetch sequence"})
	}
	if seq == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "sequence not found"})
	}
	if seq.Status != entity.SequenceStatusActive {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "sequence must be in active status to pause"})
	}

	if err := r.PauseSequence(c.Context(), orgID, id); err != nil {
		log.Printf("[Sequence] PauseSequence error for org %s id %s: %v", orgID, id, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to pause sequence"})
	}

	return c.JSON(fiber.Map{"status": "paused", "id": id})
}

// ========== Enrollment ==========

// EnrollContact enrolls a single contact in a sequence.
// Returns a warning if the contact is already in another active sequence.
// POST /sequences/:id/enroll
func (h *SequenceHandler) EnrollContact(c *fiber.Ctx) error {
	orgID, userID := getSequenceContext(c)
	sequenceID := c.Params("id")
	tenantDB := middleware.GetTenantDBConn(c)

	var input struct {
		ContactID   string `json:"contactId"`
		ForceEnroll bool   `json:"forceEnroll"`
	}
	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	if input.ContactID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "contactId is required"})
	}

	svc := service.NewSequenceService(h.sequenceRepo.WithDB(tenantDB))

	result, err := svc.EnrollContact(c.Context(), orgID, sequenceID, input.ContactID, userID, input.ForceEnroll)
	if err != nil {
		log.Printf("[Sequence] EnrollContact error for org %s seq %s contact %s: %v",
			orgID, sequenceID, input.ContactID, err)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(result)
}

// BulkEnroll enrolls multiple contacts in a sequence.
// Contacts already enrolled are skipped.
// POST /sequences/:id/enroll-bulk
func (h *SequenceHandler) BulkEnroll(c *fiber.Ctx) error {
	orgID, userID := getSequenceContext(c)
	sequenceID := c.Params("id")
	tenantDB := middleware.GetTenantDBConn(c)

	var input struct {
		ContactIDs []string `json:"contactIds"`
	}
	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	if len(input.ContactIDs) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "contactIds is required and must not be empty"})
	}

	svc := service.NewSequenceService(h.sequenceRepo.WithDB(tenantDB))

	result, err := svc.BulkEnroll(c.Context(), orgID, sequenceID, input.ContactIDs, userID)
	if err != nil {
		log.Printf("[Sequence] BulkEnroll error for org %s seq %s: %v", orgID, sequenceID, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(result)
}

// ========== Task Queue ==========

// ListTasks returns all due manual step executions for the current user.
// GET /engagement/tasks
func (h *SequenceHandler) ListTasks(c *fiber.Ctx) error {
	orgID, userID := getSequenceContext(c)
	tenantDB := middleware.GetTenantDBConn(c)
	r := h.sequenceRepo.WithDB(tenantDB)

	tasks, err := r.GetTasksForUser(c.Context(), orgID, userID, time.Now())
	if err != nil {
		log.Printf("[Sequence] ListTasks error for org %s user %s: %v", orgID, userID, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to list tasks"})
	}
	if tasks == nil {
		tasks = []repo.TaskView{}
	}
	return c.JSON(tasks)
}

// CompleteTask marks a step execution as complete.
// Optionally accepts disposition, notes, and contactId in the body for call steps.
// When disposition is provided, a call_dispositions row is written (log error but
// do not fail the completion — disposition is supplementary).
// POST /engagement/tasks/:executionId/complete
func (h *SequenceHandler) CompleteTask(c *fiber.Ctx) error {
	orgID, userID := getSequenceContext(c)
	execID := c.Params("executionId")
	tenantDB := middleware.GetTenantDBConn(c)
	r := h.sequenceRepo.WithDB(tenantDB)

	var input struct {
		Disposition string `json:"disposition"`
		Notes       string `json:"notes"`
		ContactID   string `json:"contactId"`
	}
	// Body parsing is optional — fields may be absent for non-call steps.
	_ = c.BodyParser(&input)

	exec, err := r.GetStepExecutionByID(c.Context(), execID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to fetch step execution"})
	}
	if exec == nil || exec.OrgID != orgID {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "task not found"})
	}

	if err := r.CompleteStepExecution(c.Context(), execID); err != nil {
		log.Printf("[Sequence] CompleteTask error for exec %s: %v", execID, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to complete task"})
	}

	// If a disposition was provided, write the call_dispositions row.
	// This is supplementary — failures are logged but do not abort the response.
	if input.Disposition != "" {
		now := time.Now().UTC()
		disp := &entity.CallDisposition{
			ID:              uuid.New().String(),
			OrgID:           orgID,
			StepExecutionID: execID,
			ContactID:       input.ContactID,
			EnrolledBy:      userID,
			Disposition:     input.Disposition,
			CalledAt:        now,
			CreatedAt:       now,
		}
		if input.Notes != "" {
			disp.Notes = &input.Notes
		}
		if dispErr := r.CreateCallDisposition(c.Context(), disp); dispErr != nil {
			log.Printf("[Sequence] CompleteTask: failed to write call disposition for exec %s: %v", execID, dispErr)
		}
	}

	return c.JSON(fiber.Map{
		"status":      "completed",
		"executionId": execID,
		"disposition": input.Disposition,
	})
}

// SkipTask marks a step execution as skipped.
// POST /engagement/tasks/:executionId/skip
func (h *SequenceHandler) SkipTask(c *fiber.Ctx) error {
	orgID, _ := getSequenceContext(c)
	execID := c.Params("executionId")
	tenantDB := middleware.GetTenantDBConn(c)
	r := h.sequenceRepo.WithDB(tenantDB)

	exec, err := r.GetStepExecutionByID(c.Context(), execID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to fetch step execution"})
	}
	if exec == nil || exec.OrgID != orgID {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "task not found"})
	}

	if err := r.SkipStepExecution(c.Context(), execID); err != nil {
		log.Printf("[Sequence] SkipTask error for exec %s: %v", execID, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to skip task"})
	}

	return c.JSON(fiber.Map{"status": "skipped", "executionId": execID})
}

// RescheduleTask updates the scheduled_at of a step execution.
// PUT /engagement/tasks/:executionId/reschedule
func (h *SequenceHandler) RescheduleTask(c *fiber.Ctx) error {
	orgID, _ := getSequenceContext(c)
	execID := c.Params("executionId")
	tenantDB := middleware.GetTenantDBConn(c)
	r := h.sequenceRepo.WithDB(tenantDB)

	var input struct {
		ScheduledAt string `json:"scheduledAt"`
	}
	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	if input.ScheduledAt == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "scheduledAt is required"})
	}

	newTime, err := time.Parse(time.RFC3339, input.ScheduledAt)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "scheduledAt must be a valid ISO 8601 timestamp"})
	}

	exec, err := r.GetStepExecutionByID(c.Context(), execID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to fetch step execution"})
	}
	if exec == nil || exec.OrgID != orgID {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "task not found"})
	}

	if err := r.RescheduleStepExecution(c.Context(), execID, newTime); err != nil {
		log.Printf("[Sequence] RescheduleTask error for exec %s: %v", execID, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to reschedule task"})
	}

	return c.JSON(fiber.Map{"status": "rescheduled", "executionId": execID, "scheduledAt": newTime.UTC().Format(time.RFC3339)})
}

// GetContactActivity returns a merged timeline of call and SMS activity for a contact.
// GET /contacts/:contactId/activity
func (h *SequenceHandler) GetContactActivity(c *fiber.Ctx) error {
	orgID, _ := getSequenceContext(c)
	contactID := c.Params("contactId")
	tenantDB := middleware.GetTenantDBConn(c)
	r := h.sequenceRepo.WithDB(tenantDB)

	items, err := r.GetContactActivity(c.Context(), orgID, contactID, 50)
	if err != nil {
		log.Printf("[Sequence] GetContactActivity error for contact %s: %v", contactID, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to fetch contact activity"})
	}
	if items == nil {
		items = []repo.ActivityItem{}
	}
	return c.JSON(items)
}

// ========== A/B Variant Endpoints ==========

// ListVariants returns all A/B variants for a step with their tracking stats.
// GET /sequences/:id/steps/:stepId/variants
func (h *SequenceHandler) ListVariants(c *fiber.Ctx) error {
	orgID, _ := getSequenceContext(c)
	stepID := c.Params("stepId")

	if h.abService == nil {
		return c.JSON([]service.ABVariantWithStats{})
	}

	results, err := h.abService.GetVariantsWithStats(c.Context(), orgID, stepID)
	if err != nil {
		log.Printf("[Sequence] ListVariants error for step %s: %v", stepID, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to list variants"})
	}
	if results == nil {
		results = []service.ABVariantWithStats{}
	}
	return c.JSON(results)
}

// CreateVariant creates a new A/B test variant for an email step.
// POST /sequences/:id/steps/:stepId/variants
func (h *SequenceHandler) CreateVariant(c *fiber.Ctx) error {
	orgID, _ := getSequenceContext(c)
	stepID := c.Params("stepId")

	if h.abService == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "A/B service not available"})
	}

	var input struct {
		VariantLabel     string `json:"variantLabel"`
		SubjectOverride  string `json:"subjectOverride"`
		BodyHTMLOverride string `json:"bodyHtmlOverride"`
		TrafficPct       int    `json:"trafficPct"`
	}
	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	if input.VariantLabel == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "variantLabel is required"})
	}

	now := time.Now().UTC()
	v := &entity.ABTestVariant{
		ID:           uuid.New().String(),
		StepID:       stepID,
		VariantLabel: input.VariantLabel,
		TrafficPct:   input.TrafficPct,
		IsWinner:     0,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if input.SubjectOverride != "" {
		v.SubjectOverride = &input.SubjectOverride
	}
	if input.BodyHTMLOverride != "" {
		v.BodyHTMLOverride = &input.BodyHTMLOverride
	}

	if err := h.abService.CreateVariant(c.Context(), orgID, v); err != nil {
		log.Printf("[Sequence] CreateVariant error for step %s: %v", stepID, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to create variant"})
	}

	return c.Status(fiber.StatusCreated).JSON(v)
}

// UpdateVariant updates a variant's label, overrides, or traffic percentage.
// PATCH /sequences/:id/steps/:stepId/variants/:variantId
func (h *SequenceHandler) UpdateVariant(c *fiber.Ctx) error {
	orgID, _ := getSequenceContext(c)
	variantID := c.Params("variantId")

	if h.abService == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "A/B service not available"})
	}

	var input struct {
		VariantLabel     string `json:"variantLabel"`
		SubjectOverride  string `json:"subjectOverride"`
		BodyHTMLOverride string `json:"bodyHtmlOverride"`
		TrafficPct       int    `json:"trafficPct"`
	}
	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	existing, err := h.abService.GetVariantByID(c.Context(), orgID, variantID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to fetch variant"})
	}
	if existing == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "variant not found"})
	}

	if input.VariantLabel != "" {
		existing.VariantLabel = input.VariantLabel
	}
	if input.SubjectOverride != "" {
		existing.SubjectOverride = &input.SubjectOverride
	}
	if input.BodyHTMLOverride != "" {
		existing.BodyHTMLOverride = &input.BodyHTMLOverride
	}
	if input.TrafficPct >= 0 {
		existing.TrafficPct = input.TrafficPct
	}
	existing.UpdatedAt = time.Now().UTC()

	if err := h.abService.UpdateVariant(c.Context(), orgID, existing); err != nil {
		log.Printf("[Sequence] UpdateVariant error for variant %s: %v", variantID, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to update variant"})
	}

	return c.JSON(existing)
}

// DeleteVariant removes a variant from a step.
// DELETE /sequences/:id/steps/:stepId/variants/:variantId
func (h *SequenceHandler) DeleteVariant(c *fiber.Ctx) error {
	orgID, _ := getSequenceContext(c)
	variantID := c.Params("variantId")

	if h.abService == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "A/B service not available"})
	}

	if err := h.abService.DeleteVariant(c.Context(), orgID, variantID); err != nil {
		log.Printf("[Sequence] DeleteVariant error for variant %s: %v", variantID, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to delete variant"})
	}

	return c.JSON(fiber.Map{"status": "deleted", "variantId": variantID})
}

// PromoteVariant promotes a variant to 100% winner status.
// POST /sequences/:id/steps/:stepId/variants/:variantId/promote
func (h *SequenceHandler) PromoteVariant(c *fiber.Ctx) error {
	orgID, _ := getSequenceContext(c)
	variantID := c.Params("variantId")

	if h.abService == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "A/B service not available"})
	}

	if err := h.abService.PromoteWinner(c.Context(), orgID, variantID); err != nil {
		log.Printf("[Sequence] PromoteVariant error for variant %s: %v", variantID, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to promote winner: " + err.Error()})
	}

	return c.JSON(fiber.Map{"status": "promoted", "variantId": variantID})
}

// ========== Helpers ==========

// getSequenceContext extracts orgID and userID from the Fiber context.
func getSequenceContext(c *fiber.Ctx) (string, string) {
	orgID, _ := c.Locals("orgID").(string)
	userID, _ := c.Locals("userID").(string)
	return orgID, userID
}

// isValidStepType checks whether a step type string is one of the entity constants.
func isValidStepType(t string) bool {
	switch t {
	case entity.StepTypeEmail, entity.StepTypeCall, entity.StepTypeSMS,
		entity.StepTypeLinkedIn, entity.StepTypeCustom:
		return true
	}
	return false
}

