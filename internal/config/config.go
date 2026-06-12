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

	// ServiceName is the OpenTelemetry service.name resource attribute. Defaults
	// to "questionnairebot"; OTEL_SERVICE_NAME (or OTEL_RESOURCE_ATTRIBUTES) wins
	// via resource.WithFromEnv when telemetry is enabled.
	ServiceName string
	// TelemetryEnabled is true when any OTLP endpoint env var is set. When false,
	// telemetry.Setup installs no providers and the bot behaves identically to a
	// build without telemetry.
	TelemetryEnabled bool
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

	serviceName := strings.TrimSpace(os.Getenv("OTEL_SERVICE_NAME"))
	if serviceName == "" {
		serviceName = "questionnairebot"
	}

	telemetryEnabled := false
	for _, key := range []string{
		"OTEL_EXPORTER_OTLP_ENDPOINT",
		"OTEL_EXPORTER_OTLP_TRACES_ENDPOINT",
		"OTEL_EXPORTER_OTLP_METRICS_ENDPOINT",
		"OTEL_EXPORTER_OTLP_LOGS_ENDPOINT",
	} {
		if strings.TrimSpace(os.Getenv(key)) != "" {
			telemetryEnabled = true
			break
		}
	}

	return &Config{
		BotToken:         token,
		ChatID:           chatID,
		DataDir:          dataDir,
		ServiceName:      serviceName,
		TelemetryEnabled: telemetryEnabled,
	}, nil
}
