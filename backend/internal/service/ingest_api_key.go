package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/fastcrm/backend/internal/entity"
	"github.com/fastcrm/backend/internal/repo"
)

var (
	ErrIngestKeyNotFound      = errors.New("ingest API key not found")
	ErrIngestKeyInactive      = errors.New("ingest API key is inactive")
	ErrInvalidIngestKeyName   = errors.New("ingest key name must not be empty")
)

// IngestAPIKeyService handles ingest API key business logic
type IngestAPIKeyService struct {
	repo *repo.IngestAPIKeyRepo
}

// NewIngestAPIKeyService creates a new IngestAPIKeyService
func NewIngestAPIKeyService(repo *repo.IngestAPIKeyRepo) *IngestAPIKeyService {
	return &IngestAPIKeyService{repo: repo}
}

// Create generates a new ingest API key for an organization
func (s *IngestAPIKeyService) Create(ctx context.Context, orgID, createdBy string, input entity.IngestAPIKeyCreateInput) (*entity.IngestAPIKeyCreateResponse, error) {
	// Validate name
	if input.Name == "" {
		return nil, ErrInvalidIngestKeyName
	}

	// Set default rate limit if not provided
	rateLimit := 500
	if input.RateLimit != nil && *input.RateLimit > 0 {
		rateLimit = *input.RateLimit
	}

	// Generate the key: qik_<random 32 bytes hex>
	// qik = "quantico ingest key"
	keyBytes := make([]byte, 32)
	if _, err := rand.Read(keyBytes); err != nil {
		return nil, fmt.Errorf("generating key: %w", err)
	}
	key := "qik_" + hex.EncodeToString(keyBytes)

	// Extract prefix for display (first 8 chars: qik_xxxx)
	keyPrefix := key[:8]

	// Hash the key for storage
	keyHash := s.hashKey(key)

	// Create the key in database
	ingestKey, err := s.repo.Create(ctx, orgID, createdBy, input.Name, keyHash, keyPrefix, rateLimit)
	if err != nil {
		return nil, fmt.Errorf("creating ingest key: %w", err)
	}

	// Return response with the full key (only shown once!)
	return &entity.IngestAPIKeyCreateResponse{
		Key:       key,
		ID:        ingestKey.ID,
		Name:      ingestKey.Name,
		OrgID:     ingestKey.OrgID,
		RateLimit: ingestKey.RateLimit,
		CreatedAt: ingestKey.CreatedAt,
	}, nil
}

// ValidateKey validates an ingest API key and returns the key entity
func (s *IngestAPIKeyService) ValidateKey(ctx context.Context, rawKey string) (*entity.IngestAPIKey, error) {
	// Verify key format
	if len(rawKey) < 8 || rawKey[:4] != "qik_" {
		return nil, ErrIngestKeyNotFound
	}

	// Hash the key
	keyHash := s.hashKey(rawKey)

	// Look up the key
	ingestKey, err := s.repo.GetByHash(ctx, keyHash)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrIngestKeyNotFound
		}
		return nil, fmt.Errorf("looking up ingest key: %w", err)
	}

	// Check if active
	if !ingestKey.IsActive {
		return nil, ErrIngestKeyInactive
	}

	return ingestKey, nil
}

// List returns all ingest keys for an organization
func (s *IngestAPIKeyService) List(ctx context.Context, orgID string) ([]*entity.IngestAPIKey, error) {
	keys, err := s.repo.ListByOrg(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("listing ingest keys: %w", err)
	}
	return keys, nil
}

// Deactivate deactivates a key (keeps it in DB for audit trail)
func (s *IngestAPIKeyService) Deactivate(ctx context.Context, id, orgID string) error {
	err := s.repo.Deactivate(ctx, id, orgID)
	if err != nil {
		if err == sql.ErrNoRows {
			return ErrIngestKeyNotFound
		}
		return fmt.Errorf("deactivating ingest key: %w", err)
	}
	return nil
}

// Delete permanently removes a key
func (s *IngestAPIKeyService) Delete(ctx context.Context, id, orgID string) error {
	err := s.repo.Delete(ctx, id, orgID)
	if err != nil {
		if err == sql.ErrNoRows {
			return ErrIngestKeyNotFound
		}
		return fmt.Errorf("deleting ingest key: %w", err)
	}
	return nil
}

// hashKey creates a SHA256 hash of the key
func (s *IngestAPIKeyService) hashKey(key string) string {
	hash := sha256.Sum256([]byte(key))
	return hex.EncodeToString(hash[:])
}
