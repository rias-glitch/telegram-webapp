package ws

import (
	"log"
	"net/http"
	"os"

	"telegram_webapp/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

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

		// create client and start its pumps and matchmaking
		client := NewClient(userID, conn, hub, gameType)
		go client.Run()
	}
}
