// Package handler provides HTTP handlers for the application.
package handler

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/joshua-lamorey/calorie-counter/internal/config"
	"github.com/joshua-lamorey/calorie-counter/internal/service"
	"github.com/joshua-lamorey/calorie-counter/internal/store"
)

// API serves JSON endpoints.
type API struct {
	cfg    config.Config
	ingest *service.IngestService
	logger *slog.Logger
	store  *store.Store
}

// NewAPI creates API handlers.
func NewAPI(cfg config.Config, logger *slog.Logger, store *store.Store, ingest *service.IngestService) *API {
	return &API{
		cfg:    cfg,
		ingest: ingest,
		logger: logger,
		store:  store,
	}
}

// Register registers API routes on the provided mux.
func (a *API) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/health", a.handleHealth)
	mux.HandleFunc("GET /api/configured", a.handleConfigured)
	mux.HandleFunc("GET /api/entries", a.handleEntries)
	mux.HandleFunc("POST /api/entries", a.handleCreateEntry)
	mux.HandleFunc("GET /api/summary", a.handleSummary)
}

func (a *API) handleHealth(w http.ResponseWriter, r *http.Request) {
	a.writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
		"services": map[string]any{
			"telegram": map[string]any{
				"configured": a.cfg.TelegramConfigured(),
			},
			"llm": map[string]any{
				"configured": a.cfg.LLMConfigured(),
				"provider":   a.cfg.LLMProvider,
				"model":      a.cfg.LLMModel,
			},
		},
	})
}

func (a *API) handleConfigured(w http.ResponseWriter, r *http.Request) {
	a.writeJSON(w, http.StatusOK, map[string]any{
		"telegram": map[string]any{
			"configured":      a.cfg.TelegramConfigured(),
			"allowed_chat_id": a.cfg.TelegramAllowedChatID,
		},
		"llm": map[string]any{
			"configured": a.cfg.LLMConfigured(),
			"provider":   a.cfg.LLMProvider,
			"base_url":   a.cfg.LLMBaseURL,
			"model":      a.cfg.LLMModel,
		},
	})
}

func (a *API) handleEntries(w http.ResponseWriter, r *http.Request) {
	date := r.URL.Query().Get("date")
	if date == "" {
		a.writeError(w, http.StatusBadRequest, "missing date query parameter")
		return
	}

	if _, err := time.Parse(time.DateOnly, date); err != nil {
		a.writeError(w, http.StatusBadRequest, "invalid date format, expected YYYY-MM-DD")
		return
	}

	entries, err := a.store.EntriesByDate(r.Context(), date)
	if err != nil {
		a.logger.ErrorContext(r.Context(), "listing entries failed", "error", err, "date", date)
		a.writeError(w, http.StatusInternalServerError, "failed to list entries")
		return
	}

	a.writeJSON(w, http.StatusOK, map[string]any{
		"date":    date,
		"entries": entries,
	})
}

func (a *API) handleCreateEntry(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Text      string `json:"text"`
		Timestamp string `json:"timestamp,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		a.writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}

	text := strings.TrimSpace(request.Text)
	if text == "" {
		a.writeError(w, http.StatusBadRequest, "text is required")
		return
	}

	timestamp := time.Now().UTC()
	if request.Timestamp != "" {
		parsed, err := time.Parse(time.RFC3339, request.Timestamp)
		if err != nil {
			a.writeError(w, http.StatusBadRequest, "invalid timestamp, expected RFC3339")
			return
		}
		timestamp = parsed.UTC()
	}

	entry, err := a.ingest.IngestMessage(r.Context(), "api", text, timestamp)
	if err != nil {
		a.logger.ErrorContext(r.Context(), "creating entry failed", "error", err)
		a.writeError(w, http.StatusInternalServerError, "failed to create entry")
		return
	}

	a.writeJSON(w, http.StatusCreated, map[string]any{"entry": entry})
}

func (a *API) handleSummary(w http.ResponseWriter, r *http.Request) {
	rangeValue := r.URL.Query().Get("range")
	if rangeValue == "" {
		rangeValue = "7d"
	}

	days, err := parseDayRange(rangeValue)
	if err != nil {
		a.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	end := time.Now().UTC()
	start := end.AddDate(0, 0, -(days - 1)).Truncate(24 * time.Hour)
	summary, err := a.store.Summary(r.Context(), start, end)
	if err != nil {
		a.logger.ErrorContext(r.Context(), "building summary failed", "error", err, "range", rangeValue)
		a.writeError(w, http.StatusInternalServerError, "failed to build summary")
		return
	}

	a.writeJSON(w, http.StatusOK, map[string]any{
		"range":   rangeValue,
		"start":   start.Format(time.DateOnly),
		"end":     end.Format(time.DateOnly),
		"summary": summary,
	})
}

func parseDayRange(value string) (int, error) {
	switch value {
	case "7d":
		return 7, nil
	case "30d":
		return 30, nil
	default:
		return 0, fmt.Errorf("invalid range, expected one of 7d or 30d")
	}
}

func (a *API) writeError(w http.ResponseWriter, status int, message string) {
	a.writeJSON(w, status, map[string]string{"error": message})
}

func (a *API) writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		a.logger.Error("writing json response failed", "error", err)
	}
}
