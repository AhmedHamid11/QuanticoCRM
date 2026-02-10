package handler

import (
	"encoding/json"
	"log"

	"github.com/fastcrm/backend/internal/flow"
	"github.com/fastcrm/backend/internal/repo"
	"github.com/gofiber/fiber/v2"
)

// FlowHandler handles HTTP requests for screen flows
type FlowHandler struct {
	engine   *flow.Engine
	flowRepo *repo.FlowRepo
}

// NewFlowHandler creates a new FlowHandler
func NewFlowHandler(engine *flow.Engine, flowRepo *repo.FlowRepo) *FlowHandler {
	return &FlowHandler{
		engine:   engine,
		flowRepo: flowRepo,
	}
}

// RegisterRoutes registers flow routes for authenticated users
func (h *FlowHandler) RegisterRoutes(router fiber.Router) {
	flows := router.Group("/flows")

	// Flow definition routes (read for all users)
	flows.Get("", h.ListFlows)
	flows.Get("/:id", h.GetFlow)

	// Flow execution routes
	flows.Post("/:id/start", h.StartFlow)
	flows.Get("/executions/:execId", h.GetExecution)
	flows.Post("/executions/:execId/submit", h.SubmitScreen)

	// Get available flows for an entity (for flow buttons)
	flows.Get("/entity/:entityType", h.ListFlowsForEntity)
}

// RegisterAdminRoutes registers admin-only flow management routes
func (h *FlowHandler) RegisterAdminRoutes(router fiber.Router) {
	flows := router.Group("/flows")

	// Flow definition CRUD (admin only)
	flows.Post("", h.CreateFlow)
	flows.Put("/:id", h.UpdateFlow)
	flows.Delete("/:id", h.DeleteFlow)

	// View execution history (admin only)
	flows.Get("/:id/executions", h.ListFlowExecutions)
}

// =============================================================================
// Flow Definition Endpoints
// =============================================================================

// ListFlows lists all flows for the organization
// GET /flows
func (h *FlowHandler) ListFlows(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)

	var params flow.FlowListParams
	if err := c.QueryParser(&params); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid query parameters",
		})
	}

	response, err := h.flowRepo.ListFlows(c.Context(), orgID, params)
	if err != nil {
		log.Printf("ListFlows error: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to list flows",
		})
	}

	return c.JSON(response)
}

// GetFlow retrieves a single flow definition
// GET /flows/:id
func (h *FlowHandler) GetFlow(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	flowID := c.Params("id")

	flowDef, err := h.flowRepo.GetFlow(c.Context(), flowID, orgID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Flow not found",
		})
	}

	return c.JSON(flowDef)
}

// ListFlowsForEntity returns flows available for a specific entity type
// GET /flows/entity/:entityType
func (h *FlowHandler) ListFlowsForEntity(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	entityType := c.Params("entityType")

	params := flow.FlowListParams{
		EntityType: entityType,
		PageSize:   50,
	}

	// Only show active flows
	active := true
	params.IsActive = &active

	response, err := h.flowRepo.ListFlows(c.Context(), orgID, params)
	if err != nil {
		log.Printf("ListFlowsForEntity error: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to list flows",
		})
	}

	// Transform to simpler response for UI buttons
	var buttons []fiber.Map
	for _, f := range response.Data {
		def, err := f.ParseDefinition()
		if err != nil {
			continue
		}

		buttons = append(buttons, fiber.Map{
			"id":                f.ID,
			"name":              f.Name,
			"buttonLabel":       def.Trigger.ButtonLabel,
			"showOn":            def.Trigger.ShowOn,
			"refreshOnComplete": def.RefreshOnComplete,
		})
	}

	return c.JSON(fiber.Map{
		"flows": buttons,
	})
}

// CreateFlow creates a new flow definition
// POST /flows (admin only)
func (h *FlowHandler) CreateFlow(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	userID := c.Locals("userID").(string)

	var input flow.FlowCreateInput
	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	if input.Name == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Name is required",
		})
	}

	// Serialize definition
	defJSON, err := json.Marshal(input.Definition)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid flow definition",
		})
	}

	isActive := true
	if input.IsActive != nil {
		isActive = *input.IsActive
	}

	flowDef := &flow.FlowDefinitionDB{
		OrgID:       orgID,
		Name:        input.Name,
		Description: input.Description,
		Definition:  string(defJSON),
		IsActive:    isActive,
		CreatedBy:   userID,
	}

	if err := h.flowRepo.CreateFlow(c.Context(), flowDef); err != nil {
		log.Printf("CreateFlow error: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to create flow",
		})
	}

	return c.Status(fiber.StatusCreated).JSON(flowDef)
}

// UpdateFlow updates an existing flow definition
// PUT /flows/:id (admin only)
func (h *FlowHandler) UpdateFlow(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	userID := c.Locals("userID").(string)
	flowID := c.Params("id")

	// Get existing flow
	existing, err := h.flowRepo.GetFlow(c.Context(), flowID, orgID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Flow not found",
		})
	}

	var input flow.FlowUpdateInput
	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Apply updates
	if input.Name != nil {
		existing.Name = *input.Name
	}
	if input.Description != nil {
		existing.Description = input.Description
	}
	if input.IsActive != nil {
		existing.IsActive = *input.IsActive
	}
	if input.Definition != nil {
		defJSON, err := json.Marshal(input.Definition)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid flow definition",
			})
		}
		existing.Definition = string(defJSON)
	}

	existing.ModifiedBy = &userID

	if err := h.flowRepo.UpdateFlow(c.Context(), existing); err != nil {
		log.Printf("UpdateFlow error: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to update flow",
		})
	}

	return c.JSON(existing)
}

// DeleteFlow deletes a flow definition
// DELETE /flows/:id (admin only)
func (h *FlowHandler) DeleteFlow(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	flowID := c.Params("id")

	if err := h.flowRepo.DeleteFlow(c.Context(), flowID, orgID); err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Flow not found",
		})
	}

	return c.JSON(fiber.Map{"deleted": true})
}

// =============================================================================
// Flow Execution Endpoints
// =============================================================================

// StartFlow begins execution of a flow
// POST /flows/:id/start
func (h *FlowHandler) StartFlow(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	userID := c.Locals("userID").(string)
	flowID := c.Params("id")

	// Parse request
	var req flow.StartFlowRequest
	if err := c.BodyParser(&req); err != nil && err.Error() != "Unprocessable Entity" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Get flow definition
	flowDef, err := h.flowRepo.GetFlow(c.Context(), flowID, orgID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Flow not found",
		})
	}

	if !flowDef.IsActive {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Flow is not active",
		})
	}

	// Parse definition
	def, err := flowDef.ParseDefinition()
	if err != nil {
		log.Printf("Failed to parse flow definition: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Invalid flow definition",
		})
	}

	// Get source record if provided
	var record map[string]interface{}
	if req.RecordID != "" && req.Entity != "" {
		// TODO: Fetch record from entity service
		// For now, just store the record ID in variables
		record = map[string]interface{}{
			"id": req.RecordID,
		}
	}

	// Start the flow
	exec, err := h.engine.StartFlow(c.Context(), def, orgID, userID, record)
	if err != nil {
		log.Printf("StartFlow error: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	// Save execution to database
	dbExec, err := repo.ToDBExecution(exec, req.Entity, req.RecordID)
	if err != nil {
		log.Printf("ToDBExecution error: %v", err)
	} else {
		if err := h.flowRepo.SaveExecution(c.Context(), dbExec); err != nil {
			log.Printf("SaveExecution error: %v", err)
		}
	}

	return c.JSON(exec)
}

// GetExecution retrieves the current state of a flow execution
// GET /flows/executions/:execId
func (h *FlowHandler) GetExecution(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	execID := c.Params("execId")

	// Try in-memory cache first
	exec, err := h.engine.GetExecution(execID)
	if err == nil && exec.OrgID == orgID {
		return c.JSON(exec)
	}

	// Fall back to database
	dbExec, err := h.flowRepo.GetExecution(c.Context(), execID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Execution not found",
		})
	}

	// Verify org ownership
	if dbExec.OrgID != orgID {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Execution not found",
		})
	}

	return c.JSON(dbExec)
}

// SubmitScreen submits screen data and continues flow execution
// POST /flows/executions/:execId/submit
func (h *FlowHandler) SubmitScreen(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	execID := c.Params("execId")

	// Parse screen data
	var screenData map[string]interface{}
	if err := c.BodyParser(&screenData); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Get current execution
	exec, err := h.engine.GetExecution(execID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Execution not found",
		})
	}

	// Verify ownership
	if exec.OrgID != orgID {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Execution not found",
		})
	}

	// Get flow definition
	flowDef, err := h.flowRepo.GetFlow(c.Context(), exec.FlowID, orgID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Flow not found",
		})
	}

	def, err := flowDef.ParseDefinition()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Invalid flow definition",
		})
	}

	// Resume the flow
	exec, err = h.engine.ResumeFlow(c.Context(), execID, screenData, def)
	if err != nil {
		log.Printf("ResumeFlow error: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	// Update execution in database
	dbExec, err := repo.ToDBExecution(exec, "", "")
	if err != nil {
		log.Printf("ToDBExecution error: %v", err)
	} else {
		if err := h.flowRepo.SaveExecution(c.Context(), dbExec); err != nil {
			log.Printf("SaveExecution error: %v", err)
		}
	}

	return c.JSON(exec)
}

// ListFlowExecutions lists executions for a specific flow
// GET /flows/:id/executions (admin only)
func (h *FlowHandler) ListFlowExecutions(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	flowID := c.Params("id")

	status := c.Query("status")
	limit := c.QueryInt("limit", 20)

	var statusPtr *string
	if status != "" {
		statusPtr = &status
	}

	executions, err := h.flowRepo.ListExecutions(c.Context(), orgID, &flowID, statusPtr, limit)
	if err != nil {
		log.Printf("ListFlowExecutions error: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to list executions",
		})
	}

	return c.JSON(fiber.Map{
		"data": executions,
	})
}
