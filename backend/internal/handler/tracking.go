package handler

import (
	"log"

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

// TrackingHandler serves public tracking pixel and click-redirect endpoints.
// These endpoints MUST NOT require authentication — they are called by email clients
// and linked by email recipients who have no session.
type TrackingHandler struct {
	trackingService *service.TrackingService
}

// NewTrackingHandler creates a TrackingHandler.
func NewTrackingHandler(ts *service.TrackingService) *TrackingHandler {
	return &TrackingHandler{trackingService: ts}
}

// RegisterPublicRoutes registers /t/p/:trackingId and /t/c/:trackingId on the router.
// Both routes are intentionally public — no auth middleware applied.
func (h *TrackingHandler) RegisterPublicRoutes(router fiber.Router) {
	t := router.Group("/t")
	t.Get("/p/:trackingId", h.TrackOpen)
	t.Get("/c/:trackingId", h.TrackClick)
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

// servePixel writes the 1x1 transparent GIF with appropriate cache-busting headers.
func (h *TrackingHandler) servePixel(c *fiber.Ctx) error {
	c.Set("Content-Type", "image/gif")
	c.Set("Cache-Control", "no-store, no-cache, must-revalidate, proxy-revalidate")
	c.Set("Pragma", "no-cache")
	c.Set("Expires", "0")
	return c.Status(fiber.StatusOK).Send(transparentGIF)
}
