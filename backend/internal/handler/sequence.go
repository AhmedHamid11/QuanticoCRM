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
	sequenceService *service.SequenceService
	sequenceRepo    *repo.SequenceRepo
	engagementRepo  *repo.EngagementRepo
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
	if seq.Status != entity.SequenceStatusDraft {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "sequence must be in draft status to activate"})
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
