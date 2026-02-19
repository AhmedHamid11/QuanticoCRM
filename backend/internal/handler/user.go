package handler

import (
	"database/sql"
	"log"

	"github.com/fastcrm/backend/internal/entity"
	"github.com/fastcrm/backend/internal/middleware"
	"github.com/fastcrm/backend/internal/repo"
	"github.com/fastcrm/backend/internal/service"
	"github.com/gofiber/fiber/v2"
)

// UserHandler handles HTTP requests for user management
type UserHandler struct {
	authRepo    *repo.AuthRepo
	auditLogger *service.AuditLogger
	contactRepo *repo.ContactRepo
	accountRepo *repo.AccountRepo
	taskRepo    *repo.TaskRepo
	quoteRepo   *repo.QuoteRepo
}

// NewUserHandler creates a new UserHandler
func NewUserHandler(authRepo *repo.AuthRepo, auditLogger *service.AuditLogger) *UserHandler {
	return &UserHandler{
		authRepo:    authRepo,
		auditLogger: auditLogger,
	}
}

// SetEntityRepos sets the entity repos used for owned-records counting and bulk reassignment
func (h *UserHandler) SetEntityRepos(contactRepo *repo.ContactRepo, accountRepo *repo.AccountRepo, taskRepo *repo.TaskRepo, quoteRepo *repo.QuoteRepo) {
	h.contactRepo = contactRepo
	h.accountRepo = accountRepo
	h.taskRepo = taskRepo
	h.quoteRepo = quoteRepo
}

// List returns all users in the current organization
// GET /users
func (h *UserHandler) List(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)

	page := c.QueryInt("page", 1)
	pageSize := c.QueryInt("pageSize", 20)

	response, err := h.authRepo.ListUsersByOrg(c.Context(), orgID, page, pageSize)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to list users",
		})
	}

	return c.JSON(response)
}

// Get returns a specific user in the current organization
// GET /users/:id
func (h *UserHandler) Get(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	userID := c.Params("id")

	user, err := h.authRepo.GetUserByIDInOrg(c.Context(), userID, orgID)
	if err != nil {
		if err == sql.ErrNoRows {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "User not found in this organization",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to get user",
		})
	}

	return c.JSON(user)
}

// UpdateRoleInput represents the input for updating a user's role
type UpdateRoleInput struct {
	Role string `json:"role"`
}

// UpdateStatusInput represents the input for updating a user's active status
type UpdateStatusInput struct {
	IsActive   bool    `json:"isActive"`
	ReassignTo *string `json:"reassignTo,omitempty"`
}

// UpdateRole updates a user's role in the current organization
// PUT /users/:id/role
func (h *UserHandler) UpdateRole(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	currentUserID := c.Locals("userID").(string)
	currentUserRole := c.Locals("role").(string)
	isPlatformAdmin := c.Locals("isPlatformAdmin").(bool)
	targetUserID := c.Params("id")

	var input UpdateRoleInput
	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Validate the role
	if input.Role != entity.RoleOwner && input.Role != entity.RoleAdmin && input.Role != entity.RoleUser {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid role. Must be 'owner', 'admin', or 'user'",
		})
	}

	// Get the target user's current membership
	targetUser, err := h.authRepo.GetUserByIDInOrg(c.Context(), targetUserID, orgID)
	if err != nil {
		if err == sql.ErrNoRows {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "User not found in this organization",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to get user",
		})
	}

	// Capture old role for audit logging
	oldRole := targetUser.Role

	// Permission checks (platform admins bypass these)
	if !isPlatformAdmin {
		// Only owners and admins can change roles
		if !entity.IsAdminRole(currentUserRole) {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": "Only organization owners and admins can change user roles",
			})
		}

		// Admins have restricted permissions
		if currentUserRole == entity.RoleAdmin {
			// Admins cannot promote anyone to owner
			if input.Role == entity.RoleOwner {
				return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
					"error": "Only owners can promote users to owner",
				})
			}
			// Admins cannot change owner roles
			if targetUser.Role == entity.RoleOwner {
				return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
					"error": "Admins cannot change owner roles",
				})
			}
		}

		// Prevent demoting yourself if you're the last owner
		if currentUserID == targetUserID && targetUser.Role == entity.RoleOwner && input.Role != entity.RoleOwner {
			ownerCount, err := h.authRepo.CountOrgOwners(c.Context(), orgID)
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
					"error": "Failed to check owner count",
				})
			}
			if ownerCount <= 1 {
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
					"error": "Cannot demote the last owner. Transfer ownership first.",
				})
			}
		}
	}

	// Update the role
	if err := h.authRepo.UpdateMembershipRole(c.Context(), targetUserID, orgID, input.Role); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to update user role",
		})
	}

	// Audit log the role change
	go h.auditLogger.LogRoleChange(
		c.Context(),
		currentUserID,
		c.Locals("email").(string),
		targetUserID,
		targetUser.Email,
		orgID,
		oldRole,
		input.Role,
		c.IP(),
	)

	// Return the updated user
	updatedUser, err := h.authRepo.GetUserByIDInOrg(c.Context(), targetUserID, orgID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Role updated but failed to fetch user",
		})
	}

	return c.JSON(updatedUser)
}

// UpdateStatus activates or deactivates a user
// PUT /users/:id/status
func (h *UserHandler) UpdateStatus(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	currentUserID := c.Locals("userID").(string)
	currentUserRole := c.Locals("role").(string)
	isPlatformAdmin := c.Locals("isPlatformAdmin").(bool)
	targetUserID := c.Params("id")

	var input UpdateStatusInput
	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Get the target user's current membership
	targetUser, err := h.authRepo.GetUserByIDInOrg(c.Context(), targetUserID, orgID)
	if err != nil {
		if err == sql.ErrNoRows {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "User not found in this organization",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to get user",
		})
	}

	// Permission checks (platform admins bypass these)
	if !isPlatformAdmin {
		// Only owners and admins can change user status
		if !entity.IsAdminRole(currentUserRole) {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": "Only organization owners and admins can change user status",
			})
		}

		// Cannot deactivate yourself
		if currentUserID == targetUserID && !input.IsActive {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Cannot deactivate yourself",
			})
		}

		// Admins cannot deactivate owners
		if currentUserRole == entity.RoleAdmin && targetUser.Role == entity.RoleOwner && !input.IsActive {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": "Admins cannot deactivate organization owners",
			})
		}

		// Cannot deactivate the last owner
		if targetUser.Role == entity.RoleOwner && !input.IsActive {
			ownerCount, err := h.authRepo.CountOrgOwners(c.Context(), orgID)
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
					"error": "Failed to check owner count",
				})
			}
			if ownerCount <= 1 {
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
					"error": "Cannot deactivate the last owner",
				})
			}
		}
	}

	// Bulk reassign records if deactivating and reassignTo is provided
	if !input.IsActive && input.ReassignTo != nil && *input.ReassignTo != "" {
		// Validate target user exists in org and is active
		targetReassignUser, err := h.authRepo.GetUserByIDInOrg(c.Context(), *input.ReassignTo, orgID)
		if err != nil || targetReassignUser == nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Reassign target user not found in this organization",
			})
		}
		if !targetReassignUser.IsActive {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Cannot reassign to an inactive user",
			})
		}

		// Use tenant DB for entity operations
		if h.contactRepo != nil && h.accountRepo != nil && h.taskRepo != nil && h.quoteRepo != nil {
			tenantDB := middleware.GetTenantDBConn(c)
			contactRepo := h.contactRepo
			accountRepo := h.accountRepo
			taskRepo := h.taskRepo
			quoteRepo := h.quoteRepo
			if tenantDB != nil {
				contactRepo = contactRepo.WithDB(tenantDB)
				accountRepo = accountRepo.WithDB(tenantDB)
				taskRepo = taskRepo.WithDB(tenantDB)
				quoteRepo = quoteRepo.WithDB(tenantDB)
			}

			ctx := c.Context()
			cCount, _ := contactRepo.BulkReassignByAssignedUser(ctx, orgID, targetUserID, *input.ReassignTo, currentUserID)
			aCount, _ := accountRepo.BulkReassignByAssignedUser(ctx, orgID, targetUserID, *input.ReassignTo, currentUserID)
			tCount, _ := taskRepo.BulkReassignByAssignedUser(ctx, orgID, targetUserID, *input.ReassignTo, currentUserID)
			qCount, _ := quoteRepo.BulkReassignByAssignedUser(ctx, orgID, targetUserID, *input.ReassignTo, currentUserID)
			log.Printf("[UserHandler] Reassigned records from %s to %s: contacts=%d accounts=%d tasks=%d quotes=%d",
				targetUserID, *input.ReassignTo, cCount, aCount, tCount, qCount)
		}
	}

	// Update the user's active status
	if err := h.authRepo.UpdateUserActiveStatus(c.Context(), targetUserID, input.IsActive); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to update user status",
		})
	}

	// Audit log the status change
	go h.auditLogger.LogUserStatusChange(
		c.Context(),
		currentUserID,
		c.Locals("email").(string),
		targetUserID,
		targetUser.Email,
		orgID,
		input.IsActive,
		c.IP(),
	)

	// If deactivating, also delete all their sessions to log them out immediately
	if !input.IsActive {
		_ = h.authRepo.DeleteUserSessions(c.Context(), targetUserID)
	}

	// Return the updated user
	updatedUser, err := h.authRepo.GetUserByIDInOrg(c.Context(), targetUserID, orgID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Status updated but failed to fetch user",
		})
	}

	return c.JSON(updatedUser)
}

// Remove removes a user from the current organization
// DELETE /users/:id
func (h *UserHandler) Remove(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	currentUserID := c.Locals("userID").(string)
	currentUserRole := c.Locals("role").(string)
	isPlatformAdmin := c.Locals("isPlatformAdmin").(bool)
	targetUserID := c.Params("id")

	// Get the target user's current membership
	targetUser, err := h.authRepo.GetUserByIDInOrg(c.Context(), targetUserID, orgID)
	if err != nil {
		if err == sql.ErrNoRows {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "User not found in this organization",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to get user",
		})
	}

	// Permission checks (platform admins bypass these)
	if !isPlatformAdmin {
		// Admins and owners can remove users
		if !entity.IsAdminRole(currentUserRole) {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": "Only organization admins and owners can remove users",
			})
		}

		// Admins cannot remove owners
		if currentUserRole == entity.RoleAdmin && targetUser.Role == entity.RoleOwner {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": "Admins cannot remove organization owners",
			})
		}

		// Prevent removing yourself if you're the last owner
		if currentUserID == targetUserID && targetUser.Role == entity.RoleOwner {
			ownerCount, err := h.authRepo.CountOrgOwners(c.Context(), orgID)
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
					"error": "Failed to check owner count",
				})
			}
			if ownerCount <= 1 {
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
					"error": "Cannot remove the last owner. Transfer ownership first.",
				})
			}
		}
	}

	// Remove the membership
	if err := h.authRepo.DeleteMembership(c.Context(), targetUserID, orgID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to remove user from organization",
		})
	}

	// Audit log the user removal
	go h.auditLogger.LogUserDelete(
		c.Context(),
		currentUserID,
		c.Locals("email").(string),
		targetUserID,
		targetUser.Email,
		orgID,
		map[string]interface{}{"action": "removed_from_org"},
	)

	return c.JSON(fiber.Map{
		"message": "User removed from organization",
	})
}

// GetOwnedRecordsCount returns the count of records assigned to a user
// GET /users/:id/owned-records-count
func (h *UserHandler) GetOwnedRecordsCount(c *fiber.Ctx) error {
	orgID := c.Locals("orgID").(string)
	targetUserID := c.Params("id")

	if h.contactRepo == nil || h.accountRepo == nil || h.taskRepo == nil || h.quoteRepo == nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Entity repos not configured",
		})
	}

	// Use tenant DB for entity operations
	tenantDB := middleware.GetTenantDBConn(c)
	contactRepo := h.contactRepo
	accountRepo := h.accountRepo
	taskRepo := h.taskRepo
	quoteRepo := h.quoteRepo
	if tenantDB != nil {
		contactRepo = contactRepo.WithDB(tenantDB)
		accountRepo = accountRepo.WithDB(tenantDB)
		taskRepo = taskRepo.WithDB(tenantDB)
		quoteRepo = quoteRepo.WithDB(tenantDB)
	}

	ctx := c.Context()
	contacts, _ := contactRepo.CountByAssignedUser(ctx, orgID, targetUserID)
	accounts, _ := accountRepo.CountByAssignedUser(ctx, orgID, targetUserID)
	tasks, _ := taskRepo.CountByAssignedUser(ctx, orgID, targetUserID)
	quotes, _ := quoteRepo.CountByAssignedUser(ctx, orgID, targetUserID)

	return c.JSON(fiber.Map{
		"contacts": contacts,
		"accounts": accounts,
		"tasks":    tasks,
		"quotes":   quotes,
		"total":    contacts + accounts + tasks + quotes,
	})
}

// RegisterRoutes registers the user management routes
func (h *UserHandler) RegisterRoutes(app fiber.Router) {
	users := app.Group("/users")
	users.Get("/", h.List)
	users.Get("/:id", h.Get)
}

// RegisterAdminRoutes registers the admin-only user management routes
func (h *UserHandler) RegisterAdminRoutes(app fiber.Router) {
	users := app.Group("/users")
	users.Get("/:id/owned-records-count", h.GetOwnedRecordsCount)
	users.Put("/:id/role", h.UpdateRole)
	users.Put("/:id/status", h.UpdateStatus)
	users.Delete("/:id", h.Remove)
}
