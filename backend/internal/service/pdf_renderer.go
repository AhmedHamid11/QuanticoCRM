package service

import (
	"context"

	"github.com/fastcrm/backend/internal/entity"
)

// PdfRenderer defines the interface for rendering HTML to PDF bytes
type PdfRenderer interface {
	RenderPDF(ctx context.Context, html string, opts entity.PdfRenderOptions) ([]byte, error)
}
