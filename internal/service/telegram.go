package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/joshua-lamorey/calorie-counter/internal/telegram"
)

// TelegramHandler handles Telegram messages by ingesting their text.
type TelegramHandler struct {
	ingest *IngestService
}

// NewTelegramHandler creates a Telegram message handler.
func NewTelegramHandler(ingest *IngestService) *TelegramHandler {
	return &TelegramHandler{ingest: ingest}
}

// HandleMessage ingests a Telegram text message.
func (h *TelegramHandler) HandleMessage(ctx context.Context, msg telegram.IncomingMessage) error {
	text := strings.TrimSpace(msg.Text)
	if text == "" {
		return nil
	}

	_, err := h.ingest.IngestMessage(ctx, "telegram", text, msg.Timestamp())
	if err != nil {
		return fmt.Errorf("ingesting telegram message: %w", err)
	}

	return nil
}
