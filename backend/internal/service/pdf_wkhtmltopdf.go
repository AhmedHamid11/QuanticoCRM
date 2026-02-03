package service

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"time"

	"github.com/fastcrm/backend/internal/entity"
)

// SECURITY: Whitelist of allowed values for PDF options to prevent command injection
var (
	allowedPageSizes = map[string]bool{
		"A4":      true,
		"A3":      true,
		"A5":      true,
		"Letter":  true,
		"Legal":   true,
		"Tabloid": true,
	}
	allowedOrientations = map[string]bool{
		"portrait":  true,
		"landscape": true,
	}
	// Regex for valid margin values: number followed by unit (mm, cm, in, pt, px)
	marginRegex = regexp.MustCompile(`^\d+(\.\d+)?(mm|cm|in|pt|px)$`)
)

// sanitizePdfOptions validates and sanitizes PDF render options
// SECURITY: Prevents command injection via wkhtmltopdf arguments
func sanitizePdfOptions(opts entity.PdfRenderOptions) entity.PdfRenderOptions {
	sanitized := entity.PdfRenderOptions{
		PageSize:     "A4",      // Safe default
		Orientation:  "portrait", // Safe default
		MarginTop:    "10mm",
		MarginBottom: "10mm",
		MarginLeft:   "10mm",
		MarginRight:  "10mm",
	}

	// Validate page size against whitelist
	if opts.PageSize != "" && allowedPageSizes[opts.PageSize] {
		sanitized.PageSize = opts.PageSize
	}

	// Validate orientation against whitelist
	if opts.Orientation != "" && allowedOrientations[opts.Orientation] {
		sanitized.Orientation = opts.Orientation
	}

	// Validate margins with regex (number + unit only)
	if opts.MarginTop != "" && marginRegex.MatchString(opts.MarginTop) {
		sanitized.MarginTop = opts.MarginTop
	}
	if opts.MarginBottom != "" && marginRegex.MatchString(opts.MarginBottom) {
		sanitized.MarginBottom = opts.MarginBottom
	}
	if opts.MarginLeft != "" && marginRegex.MatchString(opts.MarginLeft) {
		sanitized.MarginLeft = opts.MarginLeft
	}
	if opts.MarginRight != "" && marginRegex.MatchString(opts.MarginRight) {
		sanitized.MarginRight = opts.MarginRight
	}

	return sanitized
}

// WkhtmltopdfRenderer renders HTML to PDF using wkhtmltopdf
type WkhtmltopdfRenderer struct {
	binaryPath string
}

// NewWkhtmltopdfRenderer creates a new WkhtmltopdfRenderer
func NewWkhtmltopdfRenderer() *WkhtmltopdfRenderer {
	binaryPath := os.Getenv("WKHTMLTOPDF_PATH")
	if binaryPath == "" {
		binaryPath = "wkhtmltopdf"
	}
	return &WkhtmltopdfRenderer{binaryPath: binaryPath}
}

// RenderPDF converts HTML to PDF bytes using wkhtmltopdf
func (r *WkhtmltopdfRenderer) RenderPDF(ctx context.Context, html string, opts entity.PdfRenderOptions) ([]byte, error) {
	// SECURITY: Sanitize all options to prevent command injection
	safeOpts := sanitizePdfOptions(opts)

	// Create temp directory for files
	tmpDir, err := os.MkdirTemp("", "pdf-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// Write HTML to temp file
	htmlPath := filepath.Join(tmpDir, "input.html")
	if err := os.WriteFile(htmlPath, []byte(html), 0600); err != nil {
		return nil, fmt.Errorf("failed to write HTML: %w", err)
	}

	// Output PDF path
	pdfPath := filepath.Join(tmpDir, "output.pdf")

	// Build wkhtmltopdf command with safe options only
	// SECURITY: Removed --enable-local-file-access to prevent local file disclosure
	// If local file access is needed, use base64-encoded inline images in HTML instead
	args := []string{
		"--quiet",
		"--print-media-type",
		"--encoding", "UTF-8",
		"--disable-local-file-access", // SECURITY: Explicitly disable local file access
		"--disable-javascript",        // SECURITY: Disable JS to prevent XSS in PDF
	}

	// Use sanitized options (all validated against whitelist)
	args = append(args, "--page-size", safeOpts.PageSize)
	args = append(args, "--orientation", safeOpts.Orientation)
	args = append(args, "--margin-top", safeOpts.MarginTop)
	args = append(args, "--margin-bottom", safeOpts.MarginBottom)
	args = append(args, "--margin-left", safeOpts.MarginLeft)
	args = append(args, "--margin-right", safeOpts.MarginRight)

	args = append(args, htmlPath, pdfPath)

	// Execute with timeout
	timeout := 30 * time.Second
	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(execCtx, r.binaryPath, args...)
	cmd.Dir = tmpDir

	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("wkhtmltopdf failed: %w (output: %s)", err, string(output))
	}

	// Read PDF output
	pdfBytes, err := os.ReadFile(pdfPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read PDF output: %w", err)
	}

	return pdfBytes, nil
}
