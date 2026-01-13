package handlers

import (
	"telegram_webapp/internal/repository"
	"telegram_webapp/internal/service"

	"github.com/jackc/pgx/v5/pgxpool"
)

// HandlerConfig holds configuration for handler
type HandlerConfig struct {
	MinBet int64
	MaxBet int64
}

type Handler struct {
	DB              *pgxpool.Pool
	BotToken        string
	GameHistoryRepo *repository.GameHistoryRepository
	QuestRepo       *repository.QuestRepository
	TransactionRepo *repository.TransactionRepository
	MinesProService *service.MinesProService
	GameService     *service.GameService
	AuditService    *service.AuditService
}

func NewHandler(db *pgxpool.Pool, botToken string) *Handler {
	return &Handler{
		DB:              db,
		BotToken:        botToken,
		GameHistoryRepo: repository.NewGameHistoryRepository(db),
		QuestRepo:       repository.NewQuestRepository(db),
		TransactionRepo: repository.NewTransactionRepository(db),
		MinesProService: service.NewMinesProService(db),
		GameService:     service.NewGameService(db),
		AuditService:    service.NewAuditService(db),
	}
}

// NewHandlerWithConfig creates a handler with custom configuration
func NewHandlerWithConfig(db *pgxpool.Pool, botToken string, cfg HandlerConfig) *Handler {
	return &Handler{
		DB:              db,
		BotToken:        botToken,
		GameHistoryRepo: repository.NewGameHistoryRepository(db),
		QuestRepo:       repository.NewQuestRepository(db),
		TransactionRepo: repository.NewTransactionRepository(db),
		MinesProService: service.NewMinesProService(db),
		GameService:     service.NewGameServiceWithLimits(db, cfg.MinBet, cfg.MaxBet),
		AuditService:    service.NewAuditService(db),
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
