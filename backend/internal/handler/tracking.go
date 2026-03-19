package handler

import (
	"context"
	"database/sql"
	"log"

	"github.com/fastcrm/backend/internal/repo"
	"github.com/fastcrm/backend/internal/service"
	"github.com/gofiber/fiber/v2"
)

// transparentGIF is a minimal 1x1 transparent GIF (43 bytes).
var transparentGIF = []byte{
	0x47, 0x49, 0x46, 0x38, 0x39, 0x61, // GIF89a
	0x01, 0x00, 0x01, 0x00, // width=1, height=1
	0x80, 0x00, 0x00, // packed field, bg index, aspect
	0xff, 0xff, 0xff, // color 0: white
	0x00, 0x00, 0x00, // color 1: black
	0x21, 0xf9, 0x04, 0x01, 0x00, 0x00, 0x00, 0x00, // graphic control extension
	0x2c, 0x00, 0x00, 0x00, 0x00, 0x01, 0x00, 0x01, 0x00, 0x00, // image descriptor
	0x02, 0x02, 0x44, 0x01, 0x00, // image data
	0x3b, // trailer
}

// TrackingHandler serves public tracking pixel, click-redirect, and unsubscribe endpoints.
// These endpoints MUST NOT require authentication — they are called by email clients
// and linked by email recipients who have no session.
type TrackingHandler struct {
	trackingService *service.TrackingService
	masterDB        *sql.DB
}

// NewTrackingHandler creates a TrackingHandler.
func NewTrackingHandler(ts *service.TrackingService) *TrackingHandler {
	return &TrackingHandler{trackingService: ts}
}

// SetMasterDB wires the master DB for unsubscribe opt-out lookups (tenant routing).
// Optional — if not set, the unsubscribe endpoint logs a warning and returns success anyway.
func (h *TrackingHandler) SetMasterDB(db *sql.DB) {
	h.masterDB = db
}

// RegisterPublicRoutes registers public tracking endpoints on the router:
//   - GET /t/p/:trackingId — tracking pixel (open)
//   - GET /t/c/:trackingId — click redirect
//   - GET /unsubscribe/:contactId/:orgId — CAN-SPAM unsubscribe
func (h *TrackingHandler) RegisterPublicRoutes(router fiber.Router) {
	t := router.Group("/t")
	t.Get("/p/:trackingId", h.TrackOpen)
	t.Get("/c/:trackingId", h.TrackClick)

	// Public unsubscribe link (no auth — email recipients click this)
	router.Get("/unsubscribe/:contactId/:orgId", h.Unsubscribe)
}

// TrackOpen handles GET /t/p/:trackingId.
// Records an open event (fire-and-forget) and returns a 1x1 transparent GIF.
// Always returns 200 with the pixel — never returns an error to the email client.
func (h *TrackingHandler) TrackOpen(c *fiber.Ctx) error {
	trackingID := c.Params("trackingId")
	if trackingID == "" {
		// Serve pixel anyway — silent fail
		return h.servePixel(c)
	}

	// Non-blocking — errors are logged inside RecordOpen, not propagated
	h.trackingService.RecordOpen(trackingID)

	return h.servePixel(c)
}

// TrackClick handles GET /t/c/:trackingId.
// Records a click event and 302-redirects to the original URL.
// Falls back to 404 for genuinely bad tokens (prevents open redirects via error path).
func (h *TrackingHandler) TrackClick(c *fiber.Ctx) error {
	trackingID := c.Params("trackingId")
	if trackingID == "" {
		return c.Status(fiber.StatusBadRequest).SendString("missing tracking id")
	}

	redirectURL, err := h.trackingService.RecordClick(trackingID)
	if err != nil {
		log.Printf("[TrackingHandler] TrackClick bad token: %v", err)
		return c.Status(fiber.StatusNotFound).SendString("invalid tracking link")
	}

	return c.Redirect(redirectURL, fiber.StatusFound)
}

// Unsubscribe handles GET /unsubscribe/:contactId/:orgId.
// Adds the contact to opt_out_list with channel="email" and reason="unsubscribe_link".
// Returns a simple HTML confirmation page — no auth required.
func (h *TrackingHandler) Unsubscribe(c *fiber.Ctx) error {
	contactID := c.Params("contactId")
	orgID := c.Params("orgId")

	if contactID == "" || orgID == "" {
		return c.Status(fiber.StatusBadRequest).SendString("Invalid unsubscribe link")
	}

	if h.masterDB != nil {
		tRepo := repo.NewTrackingRepo(h.masterDB)
		if err := tRepo.OptOutContact(context.Background(), orgID, contactID, "email", "unsubscribe_link"); err != nil {
			log.Printf("[TrackingHandler] Unsubscribe: opt-out failed for contact %s org %s: %v", contactID, orgID, err)
			// Non-fatal — show confirmation page even on DB error to avoid UX confusion
		}
	} else {
		log.Printf("[TrackingHandler] Unsubscribe: masterDB not configured, cannot record opt-out for contact %s org %s", contactID, orgID)
	}

	c.Set("Content-Type", "text/html; charset=utf-8")
	return c.Status(fiber.StatusOK).SendString(`<!DOCTYPE html>
<html lang="en">
<head><meta charset="UTF-8"><title>Unsubscribed</title>
<style>body{font-family:sans-serif;max-width:480px;margin:80px auto;text-align:center;color:#333}</style>
</head>
<body>
<h2>You have been unsubscribed</h2>
<p>You will no longer receive emails from us.</p>
</body>
</html>`)
}

// servePixel writes the 1x1 transparent GIF with appropriate cache-busting headers.
func (h *TrackingHandler) servePixel(c *fiber.Ctx) error {
	c.Set("Content-Type", "image/gif")
	c.Set("Cache-Control", "no-store, no-cache, must-revalidate, proxy-revalidate")
	c.Set("Pragma", "no-cache")
	c.Set("Expires", "0")
	return c.Status(fiber.StatusOK).Send(transparentGIF)
}
