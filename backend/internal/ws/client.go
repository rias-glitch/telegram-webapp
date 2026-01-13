package ws

import (
	"bytes"
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	writeWait  = 10 * time.Second
	pongWait   = 30 * time.Second
	pingPeriod = 25 * time.Second
)

type Client struct {
	UserID   int64
	GameType string
	Conn     *websocket.Conn
	Send     chan []byte

	// Betting info
	BetAmount int64
	Currency  string // "gems" or "coins"

	Hub        *Hub
	Room       *Room
	Ready      chan struct{}
	Registered chan struct{}
	ResultAck  chan struct{}
	Done       chan struct{}
	pendingMu  sync.Mutex
	pending    [][]byte
}

func NewClient(userID int64, conn *websocket.Conn, hub *Hub, gameType string, betAmount int64, currency string) *Client {
	return &Client{
		UserID:     userID,
		GameType:   gameType,
		Conn:       conn,
		Send:       make(chan []byte, 1024),
		BetAmount:  betAmount,
		Currency:   currency,
		Hub:        hub,
		Ready:      make(chan struct{}),
		Registered: make(chan struct{}, 1),
		ResultAck:  make(chan struct{}, 1),
		Done:       make(chan struct{}),
	}
}

func (c *Client) Run() {
	// стартуем writer first so room registration can observe readiness
	go c.writePump()
	// signal that writePump has been started
	close(c.Ready)

	// send explicit ready handshake so tests/clients can wait for it
	readyMsg := []byte(`{"type":"ready"}`)
	select {
	case c.Send <- readyMsg:
		log.Printf("Client.Run: user=%d ready message queued", c.UserID)
	case <-time.After(500 * time.Millisecond):
		log.Printf("Client.Run: timeout queuing ready for user=%d", c.UserID)
	}

	// start readPump early so we don't miss messages while matchmaking
	go func() {
		log.Printf("Client.Run: starting readPump (goroutine) for user=%d", c.UserID)
		c.readPump()
	}()

	// назначаем комнату (матчмейкинг / реконнект)
	c.Room = c.Hub.AssignClient(c)

	if c.Room == nil {
		log.Printf("Client.Run: failed to assign room for user=%d", c.UserID)
		c.Conn.Close()
		return
	}

	log.Printf("Client.Run: user=%d assigned to room=%s", c.UserID, c.Room.ID)

	// wait for readPump to finish (disconnect)
	<-c.Done
}

//read
func (c *Client) readPump() {
	log.Printf("Client.readPump: START for user=%d", c.UserID)
	defer func() {
		c.disconnect()
		close(c.Done)
	}()

	c.Conn.SetReadLimit(4096)
	c.Conn.SetReadDeadline(time.Now().Add(pongWait))
	c.Conn.SetPongHandler(func(string) error {
		c.Conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, msg, err := c.Conn.ReadMessage()
		if err != nil {
			log.Println("read error:", err)
			break
		}
		log.Printf("Client.readPump: user=%d received %d bytes: %s", c.UserID, len(msg), string(msg))
		if c.Room != nil {
			c.Room.HandleMessage(c, msg)
		} else {
			// buffer message until room is assigned
			c.pendingMu.Lock()
			c.pending = append(c.pending, append([]byte(nil), msg...))
			c.pendingMu.Unlock()
			log.Printf("Client.readPump: user=%d buffered %d bytes (no room yet)", c.UserID, len(msg))
		}
	}
}

//write
func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.Conn.Close()
	}()

	for {
		select {
		case msg, ok := <-c.Send:
			c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				_ = c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.Conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				log.Printf("Client.writePump: user=%d write error: %v", c.UserID, err)
				return
			}
			log.Printf("Client.writePump: user=%d wrote %d bytes: %s", c.UserID, len(msg), string(msg))

			// if this was a result message, ack it so server can wait for delivery
			if bytes.Contains(msg, []byte(`"type":"result"`)) {
				select {
				case c.ResultAck <- struct{}{}:
				default:
				}
			}

		case <-ticker.C:
			c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

//disconnect
func (c *Client) disconnect() {
	if c.Room != nil {
		c.Hub.OnDisconnect(c)
	}
	_ = c.Conn.Close()
}
