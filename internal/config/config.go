package config

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	TelegramToken string
	AllowedUserID int64
	GeminiAPIKey  string
	GeminiModel   string
	GitRemoteURL  string
	GitUserEmail  string
	GitUserName   string
	NotesDir      string
}

func Load() (*Config, error) {
	cfg := &Config{}

	cfg.TelegramToken = os.Getenv("TELEGRAM_TOKEN")
	if cfg.TelegramToken == "" {
		return nil, fmt.Errorf("TELEGRAM_TOKEN is required")
	}

	rawUID := os.Getenv("ALLOWED_USER_ID")
	if rawUID == "" {
		return nil, fmt.Errorf("ALLOWED_USER_ID is required")
	}
	uid, err := strconv.ParseInt(rawUID, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("ALLOWED_USER_ID must be a number: %w", err)
	}
	cfg.AllowedUserID = uid

	cfg.GeminiAPIKey = os.Getenv("GEMINI_API_KEY")
	if cfg.GeminiAPIKey == "" {
		return nil, fmt.Errorf("GEMINI_API_KEY is required")
	}

	cfg.GeminiModel = os.Getenv("GEMINI_MODEL")
	if cfg.GeminiModel == "" {
		cfg.GeminiModel = "gemini-2.0-flash"
	}

	cfg.GitRemoteURL = os.Getenv("GIT_REMOTE_URL")
	if cfg.GitRemoteURL == "" {
		return nil, fmt.Errorf("GIT_REMOTE_URL is required")
	}

	cfg.GitUserEmail = os.Getenv("GIT_USER_EMAIL")
	if cfg.GitUserEmail == "" {
		return nil, fmt.Errorf("GIT_USER_EMAIL is required")
	}

	cfg.GitUserName = os.Getenv("GIT_USER_NAME")
	if cfg.GitUserName == "" {
		return nil, fmt.Errorf("GIT_USER_NAME is required")
	}

	cfg.NotesDir = os.Getenv("NOTES_DIR")
	if cfg.NotesDir == "" {
		cfg.NotesDir = "/app/notes"
	}

	return cfg, nil
}
