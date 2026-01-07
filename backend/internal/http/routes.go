package http

import (
	"os"
	"strconv"
	"time"

	"telegram_webapp/internal/http/handlers"
	"telegram_webapp/internal/http/middleware"
	"telegram_webapp/internal/repository"
	"telegram_webapp/internal/ws"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

func RegisterRoutes(r *gin.Engine, db *pgxpool.Pool, botToken string) {
	h := handlers.NewHandler(db, botToken)

	// read limits from env, with safe defaults
	apiRateLimit := 10
	if v := os.Getenv("API_RATE_LIMIT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			apiRateLimit = n
		}
	}
	apiRateWindow := time.Minute
	if v := os.Getenv("API_RATE_WINDOW_SECONDS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			apiRateWindow = time.Duration(n) * time.Second
		}
	}

	authRateLimit := 5
	if v := os.Getenv("AUTH_RATE_LIMIT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			authRateLimit = n
		}
	}
	authRateWindow := time.Minute
	if v := os.Getenv("AUTH_RATE_WINDOW_SECONDS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			authRateWindow = time.Duration(n) * time.Second
		}
	}

	api := r.Group("/api")
	api.Use(middleware.RedisRateLimit(apiRateLimit, apiRateWindow))

	// Health check for Render.com
	api.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	// Auth
	api.POST("/auth", middleware.RedisRateLimit(authRateLimit, authRateWindow), h.Auth)

	// User profile
	api.GET("/me", middleware.JWT(), h.Me)
	api.GET("/profile", middleware.JWT(), h.MyProfile)
	api.POST("/profile/balance", middleware.JWT(), h.UpdateBalance)
	api.POST("/profile/bonus", middleware.JWT(), h.ClaimBonus)
	api.GET("/profile/:id", h.Profile)

	// History
	api.POST("/history", middleware.JWT(), h.AddHistory)
	api.GET("/history", middleware.JWT(), h.GetHistory)

	// Games history and stats
	api.GET("/me/games", middleware.JWT(), h.MyGames)
	api.GET("/top", h.TopUsers)

	// Server-side game endpoints (PvE)
	api.POST("/game/coinflip", middleware.JWT(), h.CoinFlip)
	api.POST("/game/rps", middleware.JWT(), h.RPS)
	api.POST("/game/mines", middleware.JWT(), h.Mines)
	api.POST("/game/case", middleware.JWT(), h.CaseSpin)

	// Tasks (old system)
	api.GET("/tasks", h.ListTasks)
	api.POST("/tasks", middleware.JWT(), h.CreateTask)
	api.PATCH("/tasks/:id/complete", middleware.JWT(), h.CompleteTask)

	// Quests (new reward system)
	api.GET("/quests", h.GetQuests)
	api.GET("/me/quests", middleware.JWT(), h.GetMyQuests)
	api.POST("/quests/:id/claim", middleware.JWT(), h.ClaimQuestReward)

	// WebSocket for PvP games
	gameRepo := repository.NewGameRepository(db)
	gameHistoryRepo := repository.NewGameHistoryRepository(db)
	hub := ws.NewHub(gameRepo, gameHistoryRepo)
	hub.StartCleanup()
	r.GET("/ws", h.WS(hub))

	// Frontend static files
	r.StaticFS("/assets", gin.Dir("../frontend", false))
	r.NoRoute(func(c *gin.Context) {
		c.File("../frontend/index.html")
	})
}
