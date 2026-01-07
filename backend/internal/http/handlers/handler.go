package handlers

import (
	"telegram_webapp/internal/repository"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Handler struct {
	DB              *pgxpool.Pool
	BotToken        string
	GameHistoryRepo *repository.GameHistoryRepository
	QuestRepo       *repository.QuestRepository
}

func NewHandler(db *pgxpool.Pool, botToken string) *Handler {
	return &Handler{
		DB:              db,
		BotToken:        botToken,
		GameHistoryRepo: repository.NewGameHistoryRepository(db),
		QuestRepo:       repository.NewQuestRepository(db),
	}
}

// getUserID извлекает user_id из контекста Gin
func getUserID(c interface{ Get(string) (any, bool) }) (int64, bool) {
	uidVal, ok := c.Get("user_id")
	if !ok {
		return 0, false
	}
	switch v := uidVal.(type) {
	case int64:
		return v, true
	case float64:
		return int64(v), true
	default:
		return 0, false
	}
}
