package config

import (
	"fmt"
	"os"
	"strconv"
)

// Config holds application runtime configuration.
type Config struct {
	HTTPAddr              string
	BoltDBPath            string
	TelegramAllowedChatID int64
	TelegramBotToken      string
	LLMBaseURL            string
	LLMAPIKey             string
	LLMModel              string
}

// TelegramConfigured reports whether Telegram ingestion is configured.
func (c Config) TelegramConfigured() bool {
	return c.TelegramBotToken != ""
}

// LLMConfigured reports whether LLM-backed ingestion is configured.
func (c Config) LLMConfigured() bool {
	return c.LLMModel != ""
}

// Load reads configuration from environment variables.
func Load() (Config, error) {
	allowedChatID, err := parseInt64Env("TELEGRAM_ALLOWED_CHAT_ID", 0)
	if err != nil {
		return Config{}, err
	}

	cfg := Config{
		HTTPAddr:              getenv("HTTP_ADDR", ":8080"),
		BoltDBPath:            getenv("BOLTDB_PATH", "data/calorie-tracker.db"),
		TelegramAllowedChatID: allowedChatID,
		TelegramBotToken:      os.Getenv("TELEGRAM_BOT_TOKEN"),
		LLMBaseURL:            getenv("LLM_BASE_URL", ""),
		LLMAPIKey:             os.Getenv("LLM_API_KEY"),
		LLMModel:              getenv("LLM_MODEL", ""),
	}

	if cfg.HTTPAddr == "" {
		return Config{}, fmt.Errorf("http address is empty")
	}

	if cfg.BoltDBPath == "" {
		return Config{}, fmt.Errorf("boltdb path is empty")
	}

	return cfg, nil
}

func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}

	return fallback
}

func parseInt64Env(key string, fallback int64) (int64, error) {
	value := os.Getenv(key)
	if value == "" {
		return fallback, nil
	}

	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parsing %s: %w", key, err)
	}

	return parsed, nil
}
