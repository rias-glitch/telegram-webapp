package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)


type Config struct {
	AppPort     string
	DatabaseURL string
	BotToken    string
	JWTSecret   string
}

func Load() *Config {
	// загружаем .env
	_ = godotenv.Load()

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL is not set")
	}
jwtSecret := os.Getenv("JWT_SECRET")
if jwtSecret == "" {
	log.Fatal("JWT_SECRET is not set")
}

	botToken := os.Getenv("BOT_TOKEN")
	if botToken == "" {
		log.Fatal("BOT_TOKEN is not set")
	}

	port := os.Getenv("APP_PORT")
	if port == "" {
		port = "8080"
	}

	return &Config{
		AppPort:     port,
		DatabaseURL: dbURL,
		BotToken:    botToken,
		JWTSecret: jwtSecret,

	}
}
