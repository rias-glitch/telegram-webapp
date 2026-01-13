package config

import (
	"os"
	"strconv"
	"strings"

	"telegram_webapp/internal/logger"

	"github.com/joho/godotenv"
)

type Config struct {
	AppPort          string
	DatabaseURL      string
	BotToken         string
	JWTSecret        string
	AdminTelegramIDs []int64 // Telegram user IDs for admin access
	AdminBotEnabled  bool

	// Game limits
	MaxBet         int64 // Maximum bet amount for all games
	MinBet         int64 // Minimum bet amount
	GameRateLimit  int   // Max games per window
	GameRateWindow int   // Window in seconds
}

// Load loads configuration from environment variables
func Load() *Config {
	_ = godotenv.Load()

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		logger.Fatal("DATABASE_URL is not set")
	}

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		logger.Fatal("JWT_SECRET is not set")
	}

	botToken := os.Getenv("BOT_TOKEN")
	if botToken == "" {
		logger.Fatal("BOT_TOKEN is not set")
	}

	port := os.Getenv("APP_PORT")
	if port == "" {
		port = "8080"
	}

	// Parse admin telegram IDs (comma-separated)
	var adminIDs []int64
	adminIDsStr := os.Getenv("ADMIN_TELEGRAM_IDS")
	if adminIDsStr != "" {
		for _, idStr := range strings.Split(adminIDsStr, ",") {
			idStr = strings.TrimSpace(idStr)
			if id, err := strconv.ParseInt(idStr, 10, 64); err == nil {
				adminIDs = append(adminIDs, id)
			}
		}
	}

	adminBotEnabled := os.Getenv("ADMIN_BOT_ENABLED") == "true"

	// Game limits with defaults
	maxBet := int64(100000) // Default 100k gems
	if v := os.Getenv("MAX_BET"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil && n > 0 {
			maxBet = n
		}
	}

	minBet := int64(10) // Default 10 gems
	if v := os.Getenv("MIN_BET"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil && n > 0 {
			minBet = n
		}
	}

	gameRateLimit := 60 // Default 60 games per window
	if v := os.Getenv("GAME_RATE_LIMIT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			gameRateLimit = n
		}
	}

	gameRateWindow := 60 // Default 60 seconds
	if v := os.Getenv("GAME_RATE_WINDOW"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			gameRateWindow = n
		}
	}

	return &Config{
		AppPort:          port,
		DatabaseURL:      dbURL,
		BotToken:         botToken,
		JWTSecret:        jwtSecret,
		AdminTelegramIDs: adminIDs,
		AdminBotEnabled:  adminBotEnabled,
		MaxBet:           maxBet,
		MinBet:           minBet,
		GameRateLimit:    gameRateLimit,
		GameRateWindow:   gameRateWindow,
	}
}
