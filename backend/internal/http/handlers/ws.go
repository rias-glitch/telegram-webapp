package handlers

import (
	"log"
	"net/http"
	"os"

	"telegram_webapp/internal/service"
	"telegram_webapp/internal/ws"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

func (h *Handler) WS(hub *ws.Hub) gin.HandlerFunc {
	return func(c *gin.Context) {
		// JWT from query
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

		// WebSocket upgrade
		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			log.Println("ws upgrade error:", err)
			return
		}

		// Create client with game type
		client := ws.NewClient(userID, conn, hub, gameType)

		// Start client (matchmaking, room, read/write)
		go client.Run()
	}
}
