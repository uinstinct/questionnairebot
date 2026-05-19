// Package config loads runtime configuration from environment variables
// (optionally seeded from a .env file in the working directory).
package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

// Config holds the resolved runtime configuration.
type Config struct {
	BotToken string
	ChatID   int64
	DataDir  string
}

// Load reads configuration from the environment (and an optional .env file),
// validating that required values are present and that DATA_DIR is a directory.
func Load() (*Config, error) {
	if err := godotenv.Load(); err != nil && !errors.Is(err, os.ErrNotExist) {
		if _, statErr := os.Stat(".env"); statErr == nil {
			return nil, fmt.Errorf(".env: %w", err)
		}
	}

	token := strings.TrimSpace(os.Getenv("TELEGRAM_BOT_TOKEN"))
	if token == "" {
		return nil, errors.New("TELEGRAM_BOT_TOKEN is required")
	}

	chatRaw := strings.TrimSpace(os.Getenv("TELEGRAM_CHAT_ID"))
	if chatRaw == "" {
		return nil, errors.New("TELEGRAM_CHAT_ID is required")
	}
	chatID, err := strconv.ParseInt(chatRaw, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("TELEGRAM_CHAT_ID must be an integer: %w", err)
	}

	dataDir := strings.TrimSpace(os.Getenv("DATA_DIR"))
	if dataDir == "" {
		dataDir = "./data"
	}
	info, err := os.Stat(dataDir)
	if err != nil {
		return nil, fmt.Errorf("DATA_DIR %q: %w", dataDir, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("DATA_DIR %q is not a directory", dataDir)
	}

	return &Config{
		BotToken: token,
		ChatID:   chatID,
		DataDir:  dataDir,
	}, nil
}
