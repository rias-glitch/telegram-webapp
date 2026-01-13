package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"telegram_webapp/internal/bot"
	"telegram_webapp/internal/config"
	"telegram_webapp/internal/db"
	httpServer "telegram_webapp/internal/http"
	"telegram_webapp/internal/http/middleware"
	"telegram_webapp/internal/logger"
	"telegram_webapp/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Version is set at build time
var Version = "dev"

func main() {
	cfg := config.Load()

	// Initialize structured logger
	jsonLogs := os.Getenv("LOG_FORMAT") == "json"
	logLevel := os.Getenv("LOG_LEVEL")
	if logLevel == "" {
		logLevel = "info"
	}
	logger.Init(logLevel, jsonLogs)
	log := logger.Get()

	service.InitJWT()

	dbPool := db.Connect(cfg.DatabaseURL)
	defer dbPool.Close()

	r := gin.Default()

	// CORS for production (frontend on different domain)
	r.Use(func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")
		if origin != "" {
			c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
			c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
			c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		}
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	})

	middleware.InitRedisRateLimiter("", "", 0)
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))

	httpServer.RegisterRoutesWithConfig(r, dbPool, cfg.BotToken, Version, cfg)

	srv := &http.Server{
		Addr:    ":" + cfg.AppPort,
		Handler: r,
	}

	go func() {
		log.Info("server started", "port", cfg.AppPort, "version", Version)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("listen failed", "error", err)
		}
	}()

	// Start admin bot if enabled
	var adminBot *bot.AdminBot
	if cfg.AdminBotEnabled && len(cfg.AdminTelegramIDs) > 0 {
		adminService := service.NewAdminService(dbPool)
		var err error
		adminBot, err = bot.NewAdminBot(cfg.BotToken, adminService, cfg.AdminTelegramIDs)
		if err != nil {
			log.Error("failed to start admin bot", "error", err)
		} else {
			go adminBot.Start()
			log.Info("admin bot started", "admin_ids", cfg.AdminTelegramIDs)
		}
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("shutting down server...")

	// Graceful shutdown for bot
	if adminBot != nil {
		adminBot.Stop()
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Fatal("server forced to shutdown", "error", err)
	}

	log.Info("server exited")
}
