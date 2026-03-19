package handler

import (
	"encoding/json"
	"log"

	"github.com/fastcrm/backend/internal/entity"
	"github.com/fastcrm/backend/internal/middleware"
	"github.com/fastcrm/backend/internal/repo"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

// CloneSequence creates an independent draft copy of a sequence with all its steps.
// POST /sequences/:id/clone
func (h *SequenceHandler) CloneSequence(c *fiber.Ctx) error {
	orgID, userID := getSequenceContext(c)
	sourceID := c.Params("id")
	tenantDB := middleware.GetTenantDBConn(c)
	r := h.sequenceRepo.WithDB(tenantDB)

	// Fetch source sequence to derive the clone name
	src, err := r.GetSequence(c.Context(), orgID, sourceID)
	if err != nil {
		log.Printf("[Sequence] CloneSequence: fetch source error for org %s id %s: %v", orgID, sourceID, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to fetch source sequence"})
	}
	if src == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "sequence not found"})
	}

	newID := uuid.New().String()
	newName := repo.BuildCloneName(src.Name)

	if err := r.CloneSequence(c.Context(), orgID, sourceID, newID, newName, userID); err != nil {
		log.Printf("[Sequence] CloneSequence error for org %s src %s: %v", orgID, sourceID, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to clone sequence"})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"id":   newID,
		"name": newName,
	})
}

// createVersionSnapshot takes a snapshot of the current steps and stores it in
// sequence_versions. Called from ActivateSequence after status update succeeds.
// Non-fatal: errors are logged but do not fail the activation.
func createVersionSnapshot(c *fiber.Ctx, r *repo.SequenceRepo, orgID, sequenceID string, steps []*entity.SequenceStep) error {
	// Marshal steps to JSON for the snapshot
	stepsJSON, err := json.Marshal(steps)
	if err != nil {
		return err
	}

	// Determine next version number
	latestNum, err := r.GetLatestVersionNumber(c.Context(), sequenceID)
	if err != nil {
		return err
	}
	nextVersion := latestNum + 1
	versionID := uuid.New().String()

	return r.CreateSequenceVersion(c.Context(), orgID, sequenceID, versionID, nextVersion, string(stepsJSON))
}
