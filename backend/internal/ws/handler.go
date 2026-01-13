package ws

import (
	"context"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"telegram_webapp/internal/domain"
	"telegram_webapp/internal/repository"
	"telegram_webapp/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// WSHandler holds dependencies for WebSocket handling
type WSHandler struct {
	Hub      *Hub
	UserRepo *repository.UserRepository
}

func NewWSHandler(hub *Hub, userRepo *repository.UserRepository) *WSHandler {
	return &WSHandler{
		Hub:      hub,
		UserRepo: userRepo,
	}
}

func (h *WSHandler) HandleWS() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.Query("token")
		if token == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "token required"})
			return
		}

		userID, err := service.ParseJWT(token)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			return
		}

		// Get game type from query (default: rps)
		gameType := c.Query("game")
		if gameType == "" {
			gameType = "rps"
		}

		// Get bet amount from query (default: 0 for free play)
		betAmount := int64(0)
		if betStr := c.Query("bet"); betStr != "" {
			if bet, err := strconv.ParseInt(betStr, 10, 64); err == nil && bet > 0 {
				betAmount = bet
			}
		}

		// Get currency from query (default: gems)
		currency := c.Query("currency")
		if currency != string(domain.CurrencyCoins) {
			currency = string(domain.CurrencyGems)
		}

		// Validate user has enough balance for the bet
		if betAmount > 0 && h.UserRepo != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			user, err := h.UserRepo.GetByID(ctx, userID)
			if err != nil || user == nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "user not found"})
				return
			}

			var balance int64
			if currency == string(domain.CurrencyCoins) {
				balance = user.Coins
			} else {
				balance = user.Gems
			}

			if balance < betAmount {
				c.JSON(http.StatusBadRequest, gin.H{"error": "insufficient balance"})
				return
			}

			// Deduct bet from user balance (reserve it)
			if currency == string(domain.CurrencyCoins) {
				if _, err := h.UserRepo.UpdateCoins(ctx, userID, -betAmount); err != nil {
					log.Printf("HandleWS: failed to deduct coins: %v", err)
					c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to reserve bet"})
					return
				}
			} else {
				if _, err := h.UserRepo.UpdateGems(ctx, userID, -betAmount); err != nil {
					log.Printf("HandleWS: failed to deduct gems: %v", err)
					c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to reserve bet"})
					return
				}
			}
		}

		allowedOrigin := os.Getenv("ALLOWED_ORIGIN")
		upgrader := websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				if allowedOrigin == "" {
					return true
				}
				return r.Header.Get("Origin") == allowedOrigin
			},
		}

		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			log.Println("ws upgrade error:", err)
			// Refund the bet if WebSocket upgrade fails
			if betAmount > 0 && h.UserRepo != nil {
				ctx := context.Background()
				if currency == string(domain.CurrencyCoins) {
					h.UserRepo.UpdateCoins(ctx, userID, betAmount)
				} else {
					h.UserRepo.UpdateGems(ctx, userID, betAmount)
				}
			}
			return
		}

		// create client and start its pumps and matchmaking
		client := NewClient(userID, conn, h.Hub, gameType, betAmount, currency)
		go client.Run()
	}
}

// Legacy handler for backwards compatibility (no betting)
func HandleWS(hub *Hub) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.Query("token")
		if token == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "token required"})
			return
		}

		userID, err := service.ParseJWT(token)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			return
		}

		// Get game type from query (default: rps)
		gameType := c.Query("game")
		if gameType == "" {
			gameType = "rps"
		}

		allowedOrigin := os.Getenv("ALLOWED_ORIGIN")
		upgrader := websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				if allowedOrigin == "" {
					return true
				}
				return r.Header.Get("Origin") == allowedOrigin
			},
		}

		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			log.Println("ws upgrade error:", err)
			return
		}

		// create client and start its pumps and matchmaking (free play)
		client := NewClient(userID, conn, hub, gameType, 0, string(domain.CurrencyGems))
		go client.Run()
	}
}
