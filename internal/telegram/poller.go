package telegram

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const maxMessageRetries = 3

// MessageHandler processes an incoming Telegram message.
type MessageHandler interface {
	HandleMessage(ctx context.Context, msg IncomingMessage) error
}

// OffsetStore persists Telegram polling offsets and retry metadata.
type OffsetStore interface {
	ClearTelegramFailureCount(ctx context.Context, updateID int64) error
	IncrementTelegramFailureCount(ctx context.Context, updateID int64) (int, error)
	SaveTelegramOffset(ctx context.Context, offset int64) error
	TelegramOffset(ctx context.Context) (int64, error)
}

// Poller polls Telegram for new bot messages.
type Poller struct {
	allowedChatID int64
	handler       MessageHandler
	httpClient    *http.Client
	logger        *slog.Logger
	offset        int64
	offsetStore   OffsetStore
	token         string
}

// NewPoller creates a Telegram poller.
func NewPoller(logger *slog.Logger, token string, allowedChatID int64, offsetStore OffsetStore, handler MessageHandler) *Poller {
	return &Poller{
		allowedChatID: allowedChatID,
		handler:       handler,
		httpClient: &http.Client{
			Timeout: 70 * time.Second,
		},
		logger:      logger,
		offsetStore: offsetStore,
		token:       token,
	}
}

// Run starts the polling loop and blocks until the context is cancelled.
func (p *Poller) Run(ctx context.Context) error {
	if p.token == "" {
		p.logger.InfoContext(ctx, "telegram poller disabled", "reason", "missing bot token")
		<-ctx.Done()
		return nil
	}

	if p.handler == nil {
		return fmt.Errorf("telegram message handler is nil")
	}

	if p.offsetStore == nil {
		return fmt.Errorf("telegram offset store is nil")
	}

	offset, err := p.offsetStore.TelegramOffset(ctx)
	if err != nil {
		return fmt.Errorf("loading telegram offset: %w", err)
	}
	p.offset = offset

	p.logger.InfoContext(ctx, "telegram poller started", "offset", p.offset)

	for {
		updates, err := p.getUpdates(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}

			p.logger.ErrorContext(ctx, "telegram getUpdates failed", "error", err)

			select {
			case <-ctx.Done():
				return nil
			case <-time.After(5 * time.Second):
			}
			continue
		}

		for _, update := range updates {
			nextOffset := update.UpdateID + 1

			if update.Message == nil {
				if err := p.persistOffset(ctx, nextOffset); err != nil {
					return err
				}
				continue
			}

			if p.allowedChatID != 0 && update.Message.Chat.ID != p.allowedChatID {
				p.logger.WarnContext(ctx, "ignoring telegram message from unauthorized chat",
					"chat_id", update.Message.Chat.ID,
				)
				if err := p.persistOffset(ctx, nextOffset); err != nil {
					return err
				}
				continue
			}

			text := strings.TrimSpace(update.Message.Text)
			if text == "" {
				if err := p.persistOffset(ctx, nextOffset); err != nil {
					return err
				}
				continue
			}

			if err := p.handler.HandleMessage(ctx, *update.Message); err != nil {
				failureCount, failureErr := p.offsetStore.IncrementTelegramFailureCount(ctx, update.UpdateID)
				if failureErr != nil {
					return fmt.Errorf("incrementing telegram failure count: %w", failureErr)
				}

				p.logger.ErrorContext(ctx, "handling telegram message failed",
					"error", err,
					"chat_id", update.Message.Chat.ID,
					"message_id", update.Message.MessageID,
					"update_id", update.UpdateID,
					"failure_count", failureCount,
				)

				if failureCount >= maxMessageRetries {
					p.logger.WarnContext(ctx, "telegram message reached retry limit, skipping",
						"update_id", update.UpdateID,
						"message_id", update.Message.MessageID,
						"failure_count", failureCount,
					)
					if err := p.offsetStore.ClearTelegramFailureCount(ctx, update.UpdateID); err != nil {
						return fmt.Errorf("clearing telegram failure count: %w", err)
					}
					if err := p.persistOffset(ctx, nextOffset); err != nil {
						return err
					}
				}
				continue
			}

			if err := p.offsetStore.ClearTelegramFailureCount(ctx, update.UpdateID); err != nil {
				return fmt.Errorf("clearing telegram failure count: %w", err)
			}

			if err := p.persistOffset(ctx, nextOffset); err != nil {
				return err
			}
		}
	}
}

func (p *Poller) persistOffset(ctx context.Context, offset int64) error {
	if offset <= p.offset {
		return nil
	}

	if err := p.offsetStore.SaveTelegramOffset(ctx, offset); err != nil {
		return fmt.Errorf("persisting telegram offset: %w", err)
	}

	p.offset = offset
	return nil
}

func (p *Poller) getUpdates(ctx context.Context) ([]Update, error) {
	endpoint := fmt.Sprintf("https://api.telegram.org/bot%s/getUpdates", p.token)
	values := url.Values{}
	values.Set("timeout", "60")
	values.Set("allowed_updates", `["message"]`)
	if p.offset > 0 {
		values.Set("offset", strconv.FormatInt(p.offset, 10))
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(values.Encode()))
	if err != nil {
		return nil, fmt.Errorf("creating getUpdates request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("performing getUpdates request: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("telegram getUpdates returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var payload struct {
		OK     bool     `json:"ok"`
		Result []Update `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decoding getUpdates response: %w", err)
	}

	if !payload.OK {
		return nil, fmt.Errorf("telegram getUpdates returned ok=false")
	}

	return payload.Result, nil
}
