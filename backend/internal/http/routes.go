package http

import (
	"os"
	"strconv"
	"time"

	"telegram_webapp/internal/config"
	"telegram_webapp/internal/http/handlers"
	"telegram_webapp/internal/http/middleware"
	"telegram_webapp/internal/repository"
	"telegram_webapp/internal/ws"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Global reference to ton handler for setting callbacks
var globalTonHandler *handlers.TonHandler

func RegisterRoutes(r *gin.Engine, db *pgxpool.Pool, botToken string, version string) {
	RegisterRoutesWithConfig(r, db, botToken, version, nil)
}

// SetWithdrawalNotifyCallback sets the callback for withdrawal notifications
func SetWithdrawalNotifyCallback(callback handlers.WithdrawalNotifyFunc) {
	if globalTonHandler != nil {
		globalTonHandler.OnWithdrawalCreate = callback
	}
}

func RegisterRoutesWithConfig(r *gin.Engine, db *pgxpool.Pool, botToken string, version string, cfg *config.Config) {
	var h *handlers.Handler
	if cfg != nil {
		h = handlers.NewHandlerWithConfig(db, botToken, handlers.HandlerConfig{
			MinBet: cfg.MinBet,
			MaxBet: cfg.MaxBet,
		})
	} else {
		h = handlers.NewHandler(db, botToken)
	}
	healthHandler := handlers.NewHealthHandler(db, version)

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

	// Health checks (no rate limiting)
	r.GET("/health", healthHandler.Health)
	r.GET("/healthz", healthHandler.Liveness)
	r.GET("/readyz", healthHandler.Readiness)

	// Game rate limiting (per user)
	gameRateLimit := 60
	gameRateWindow := time.Minute
	if cfg != nil {
		gameRateLimit = cfg.GameRateLimit
		gameRateWindow = time.Duration(cfg.GameRateWindow) * time.Second
	}

	// API v1 routes
	v1 := r.Group("/api/v1")
	v1.Use(middleware.RedisRateLimit(apiRateLimit, apiRateWindow))
	registerAPIRoutes(v1, h, authRateLimit, authRateWindow, gameRateLimit, gameRateWindow)

	// Legacy /api routes (redirect to v1 for backward compatibility)
	api := r.Group("/api")
	api.Use(middleware.RedisRateLimit(apiRateLimit, apiRateWindow))

	// Keep old health endpoint for backward compatibility
	api.GET("/health", healthHandler.Health)
	registerAPIRoutes(api, h, authRateLimit, authRateWindow, gameRateLimit, gameRateWindow)

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

func registerAPIRoutes(api *gin.RouterGroup, h *handlers.Handler, authRateLimit int, authRateWindow time.Duration, gameRateLimit int, gameRateWindow time.Duration) {
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

	// Game rate limiter middleware (per user, not per IP)
	gameRL := middleware.GameRateLimit(gameRateLimit, gameRateWindow)

	// Server-side game endpoints (PvE) with game rate limiting
	api.POST("/game/coinflip", middleware.JWT(), gameRL, h.CoinFlip)
	api.POST("/game/rps", middleware.JWT(), gameRL, h.RPS)
	api.POST("/game/mines", middleware.JWT(), gameRL, h.Mines)
	api.POST("/game/case", middleware.JWT(), gameRL, h.CaseSpin)

	// New PvE games with game rate limiting
	api.POST("/game/dice", middleware.JWT(), gameRL, h.Dice)
	api.GET("/game/dice/info", h.DiceInfo)
	api.POST("/game/wheel", middleware.JWT(), gameRL, h.Wheel)
	api.GET("/game/wheel/info", h.WheelInfo)

	// Mines Pro (advanced multi-round mines) with game rate limiting
	api.POST("/game/mines-pro/start", middleware.JWT(), gameRL, h.MinesProStart)
	api.POST("/game/mines-pro/reveal", middleware.JWT(), gameRL, h.MinesProReveal)
	api.POST("/game/mines-pro/cashout", middleware.JWT(), h.MinesProCashOut)
	api.GET("/game/mines-pro/state", middleware.JWT(), h.MinesProState)
	api.GET("/game/mines-pro/info", h.MinesProInfo)

	// CoinFlip Pro (multi-round coinflip) with game rate limiting
	api.POST("/game/coinflip-pro/start", middleware.JWT(), gameRL, h.CoinFlipProStart)
	api.POST("/game/coinflip-pro/flip", middleware.JWT(), gameRL, h.CoinFlipProFlip)
	api.POST("/game/coinflip-pro/cashout", middleware.JWT(), h.CoinFlipProCashOut)
	api.GET("/game/coinflip-pro/state", middleware.JWT(), h.CoinFlipProState)
	api.GET("/game/coinflip-pro/info", h.CoinFlipProInfo)

	// Game limits info endpoint
	api.GET("/game/limits", h.GameLimits)

	// Tasks (old system)
	api.GET("/tasks", h.ListTasks)
	api.POST("/tasks", middleware.JWT(), h.CreateTask)
	api.PATCH("/tasks/:id/complete", middleware.JWT(), h.CompleteTask)

	// Quests (new reward system)
	api.GET("/quests", h.GetQuests)
	api.GET("/me/quests", middleware.JWT(), h.GetMyQuests)
	api.POST("/quests/:id/claim", middleware.JWT(), h.ClaimQuestReward)

	// Referral system
	referralRepo := repository.NewReferralRepository(h.DB)
	botUsername := os.Getenv("BOT_USERNAME")
	if botUsername == "" {
		botUsername = "CryptoGamesBot"
	}
	referralHandler := handlers.NewReferralHandler(referralRepo, botUsername)
	referral := api.Group("/referral")
	referral.Use(middleware.JWT())
	{
		referral.GET("/code", referralHandler.GetReferralCode)
		referral.GET("/link", referralHandler.GetReferralLink)
		referral.GET("/stats", referralHandler.GetReferralStats)
		referral.POST("/apply", referralHandler.ApplyReferralCode)
	}

	// Upgrade system (character levels, GK currency)
	userRepo := repository.NewUserRepository(h.DB)
	upgradeHandler := handlers.NewUpgradeHandler(userRepo, referralRepo)
	upgrade := api.Group("/upgrade")
	{
		upgrade.GET("/info", upgradeHandler.GetUpgradeInfo)
		upgrade.GET("/status", middleware.JWT(), upgradeHandler.GetMyUpgradeStatus)
		upgrade.POST("/level", middleware.JWT(), upgradeHandler.UpgradeCharacter)
		upgrade.POST("/claim-reward", middleware.JWT(), upgradeHandler.ClaimReferralReward)
	}

	// Leaderboard (monthly top 100 + user rank)
	api.GET("/leaderboard", h.GetLeaderboard)
	api.GET("/leaderboard/rank", middleware.JWT(), h.GetMyRank)

	// TON Connect & Payments
	tonHandler := handlers.NewTonHandler(h)
	globalTonHandler = tonHandler // Store for callback setup
	ton := api.Group("/ton")
	{
		// Wallet management
		ton.GET("/config", tonHandler.GetTonConfig)
		ton.GET("/wallet", middleware.JWT(), tonHandler.GetWallet)
		ton.POST("/wallet/connect", middleware.JWT(), tonHandler.ConnectWallet)
		ton.DELETE("/wallet", middleware.JWT(), tonHandler.DisconnectWallet)

		// Deposits
		ton.GET("/deposit/info", middleware.JWT(), tonHandler.GetDepositInfo)
		ton.GET("/deposits", middleware.JWT(), tonHandler.GetDeposits)
		ton.POST("/deposit/manual", middleware.JWT(), func(c *gin.Context) {
			tonHandler.RecordManualDeposit(c, h)
		})

		// Withdrawals
		ton.POST("/withdraw/estimate", middleware.JWT(), tonHandler.GetWithdrawEstimate)
		ton.POST("/withdraw", middleware.JWT(), func(c *gin.Context) {
			tonHandler.RequestWithdrawal(c, nil)
		})
		ton.GET("/withdrawals", middleware.JWT(), tonHandler.GetWithdrawals)
		ton.POST("/withdraw/cancel", middleware.JWT(), tonHandler.CancelWithdrawal)
	}
}
