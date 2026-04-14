// Package service implements the application's business logic.
package service

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/joshua-lamorey/calorie-counter/internal/llm"
	"github.com/joshua-lamorey/calorie-counter/internal/model"
	"github.com/joshua-lamorey/calorie-counter/internal/store"
)

// IngestService turns free-text food messages into persisted entries.
type IngestService struct {
	logger *slog.Logger
	llm    llm.Client
	store  *store.Store
}

// NewIngestService creates an ingestion service.
func NewIngestService(logger *slog.Logger, llmClient llm.Client, st *store.Store) *IngestService {
	return &IngestService{
		logger: logger,
		llm:    llmClient,
		store:  st,
	}
}

// IngestMessage estimates nutrition for a message and stores the resulting entry.
func (s *IngestService) IngestMessage(ctx context.Context, source, description string, timestamp time.Time) (model.Entry, error) {
	description = strings.TrimSpace(description)
	if description == "" {
		return model.Entry{}, fmt.Errorf("message is empty")
	}

	entry, err := s.llm.EstimateEntry(ctx, description)
	if err != nil {
		return model.Entry{}, fmt.Errorf("estimating entry: %w", err)
	}

	entry.Description = description
	entry.Timestamp = timestamp.UTC().Unix()
	if entry.ID == "" {
		entry.ID = fmt.Sprintf("%d", timestamp.UTC().UnixNano())
	}

	if err := s.store.SaveEntry(ctx, entry); err != nil {
		return model.Entry{}, fmt.Errorf("saving entry: %w", err)
	}

	s.logger.InfoContext(ctx, "entry ingested",
		"source", source,
		"entry_id", entry.ID,
		"description", entry.Description,
		"kcal", entry.Kcal,
	)

	return entry, nil
}
