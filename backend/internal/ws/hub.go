package ws

import (
	"fmt"
	"log"
	"strconv"
	"sync"
	"time"

	"telegram_webapp/internal/game"
	"telegram_webapp/internal/repository"
)

// WaitingKey uniquely identifies a matchmaking queue
// Players are matched by game type, bet amount, and currency
type WaitingKey struct {
	GameType  game.GameType
	BetAmount int64
	Currency  string
}

func (k WaitingKey) String() string {
	return fmt.Sprintf("%s_%d_%s", k.GameType, k.BetAmount, k.Currency)
}

type Hub struct {
	Rooms    map[string]*Room
	UserRoom map[int64]string
	mu       sync.RWMutex
	roomSeq  int64
	// Separate waiting queues for each game type + bet + currency
	WaitingByKey map[WaitingKey]*Client
	// Legacy: for backwards compatibility
	WaitingByGame   map[game.GameType]*Client
	GameRepo        *repository.GameRepository
	GameHistoryRepo *repository.GameHistoryRepository
	UserRepo        *repository.UserRepository
}

func NewHub(gameRepo *repository.GameRepository, gameHistoryRepo *repository.GameHistoryRepository) *Hub {
	return &Hub{
		Rooms:           make(map[string]*Room),
		UserRoom:        make(map[int64]string),
		WaitingByKey:    make(map[WaitingKey]*Client),
		WaitingByGame:   make(map[game.GameType]*Client),
		GameRepo:        gameRepo,
		GameHistoryRepo: gameHistoryRepo,
	}
}

func NewHubWithUserRepo(gameRepo *repository.GameRepository, gameHistoryRepo *repository.GameHistoryRepository, userRepo *repository.UserRepository) *Hub {
	hub := NewHub(gameRepo, gameHistoryRepo)
	hub.UserRepo = userRepo
	return hub
}

func (h *Hub) AssignClient(c *Client) *Room {
	h.mu.Lock()

	// Convert string game type to GameType
	gameType := game.GameType(c.GameType)
	if gameType != game.TypeRPS && gameType != game.TypeMines {
		gameType = game.TypeRPS // default
	}

	// Create waiting key for matchmaking by game type + bet + currency
	waitingKey := WaitingKey{
		GameType:  gameType,
		BetAmount: c.BetAmount,
		Currency:  c.Currency,
	}

	log.Printf("Hub.AssignClient: user=%d game=%s bet=%d currency=%s - assign via waiting slot (rooms=%d)",
		c.UserID, gameType, c.BetAmount, c.Currency, len(h.Rooms))

	// Clean up any stale state for this user (e.g., from previous game/reconnect)
	if oldRoomID, exists := h.UserRoom[c.UserID]; exists {
		log.Printf("Hub.AssignClient: user=%d has stale room mapping to %s, cleaning up", c.UserID, oldRoomID)
		delete(h.UserRoom, c.UserID)
		// If user was in WaitingByKey, clear it
		for key, waiting := range h.WaitingByKey {
			if waiting != nil && waiting.UserID == c.UserID {
				log.Printf("Hub.AssignClient: clearing stale waiting slot for user=%d key=%s", c.UserID, key)
				delete(h.WaitingByKey, key)
			}
		}
		// Legacy: also check WaitingByGame
		if waiting := h.WaitingByGame[gameType]; waiting != nil && waiting.UserID == c.UserID {
			log.Printf("Hub.AssignClient: clearing stale waiting slot (legacy) for user=%d", c.UserID)
			delete(h.WaitingByGame, gameType)
		}
	}

	// If there is a waiting client for this exact key (game + bet + currency), attempt to pair
	waiting := h.WaitingByKey[waitingKey]
	if waiting != nil {
		// don't pair with self
		if waiting.UserID != c.UserID {
			// Check if waiting client's connection is still alive
			// by trying a non-blocking send to its Send channel
			waitingAlive := false
			select {
			case waiting.Send <- []byte(`{"type":"ping"}`):
				waitingAlive = true
			default:
				// Channel is full or closed - client may be dead
				log.Printf("Hub.AssignClient: waiting client=%d Send channel blocked, may be dead", waiting.UserID)
			}

			if !waitingAlive {
				log.Printf("Hub.AssignClient: waiting client=%d appears dead, clearing waiting slot", waiting.UserID)
				delete(h.WaitingByKey, waitingKey)
				// Fall through to create new room
			} else {
				// find the waiting client's room id
				roomID, ok := h.UserRoom[waiting.UserID]
				if ok {
					foundRoom, ok2 := h.Rooms[roomID]
					if ok2 {
						// ensure waiting client still appears in the room's clients
						foundRoom.mu.RLock()
						_, stillThere := foundRoom.Clients[waiting.UserID]
						foundRoom.mu.RUnlock()
						if stillThere {
							log.Printf("Hub.AssignClient: pairing user=%d with waiting user=%d in room=%s game=%s bet=%d currency=%s",
								c.UserID, waiting.UserID, foundRoom.ID, gameType, c.BetAmount, c.Currency)

							// Update existing game with second player (preserves setup state for Mines)
							foundRoom.mu.Lock()
							foundRoom.game.SetSecondPlayer(c.UserID)
							foundRoom.Clients[c.UserID] = c
							foundRoom.mu.Unlock()

							h.UserRoom[c.UserID] = foundRoom.ID
							// clear waiting slot for this key
							delete(h.WaitingByKey, waitingKey)
							h.mu.Unlock()

							log.Printf("Hub.AssignClient: about to register user=%d to room=%s", c.UserID, foundRoom.ID)

							// Non-blocking send to avoid deadlock
							select {
							case foundRoom.Register <- c:
								log.Printf("Hub.AssignClient: registered user=%d to room=%s", c.UserID, foundRoom.ID)
							case <-time.After(5 * time.Second):
								log.Printf("Hub.AssignClient: TIMEOUT registering user=%d to room=%s", c.UserID, foundRoom.ID)
								return nil
							}

							return foundRoom
						}
						// if waiting client not present in room, clear stale waiting
						log.Printf("Hub.AssignClient: found stale waiting client=%d (not in room), clearing waiting slot", waiting.UserID)
						delete(h.WaitingByKey, waitingKey)
					} else {
						// room missing, clear stale waiting
						log.Printf("Hub.AssignClient: waiting client's room missing (id=%s), clearing waiting slot", roomID)
						delete(h.WaitingByKey, waitingKey)
					}
				} else {
					// no mapping for waiting user, clear
					log.Printf("Hub.AssignClient: waiting user mapped to no room, clearing waiting slot")
					delete(h.WaitingByKey, waitingKey)
				}
			}
		} else {
			// waiting is same user - clear and fallthrough to create room
			log.Printf("Hub.AssignClient: waiting user is same as current user=%d, clearing waiting slot", c.UserID)
			delete(h.WaitingByKey, waitingKey)
		}
	}

	// Create new room for this game type with bet info
	players := [2]int64{c.UserID, 0}
	room := h.newRoomWithBet(gameType, players, c.BetAmount, c.Currency)

	if room == nil {
		log.Printf("Hub.AssignClient: failed to create room for user=%d", c.UserID)
		h.mu.Unlock()
		return nil
	}

	log.Printf("Hub.AssignClient: user=%d created new room=%s game=%s bet=%d currency=%s",
		c.UserID, room.ID, gameType, c.BetAmount, c.Currency)
	// reserve the slot for this client immediately to avoid race with another AssignClient
	room.mu.Lock()
	room.Clients[c.UserID] = c
	room.mu.Unlock()

	log.Printf("Hub.AssignClient: reserved user=%d in room=%s (pre-register)", c.UserID, room.ID)

	h.UserRoom[c.UserID] = room.ID
	// mark this client as waiting for a peer with same bet
	h.WaitingByKey[waitingKey] = c

	h.mu.Unlock()

	log.Printf("Hub.AssignClient: registering user=%d to NEW room=%s", c.UserID, room.ID)

	// Non-blocking send to avoid deadlock if room.Run() has exited
	select {
	case room.Register <- c:
		log.Printf("Hub.AssignClient: successfully registered user=%d to room=%s", c.UserID, room.ID)
	case <-time.After(5 * time.Second):
		log.Printf("Hub.AssignClient: TIMEOUT registering user=%d to room=%s - room may have exited", c.UserID, room.ID)
		return nil
	}

	return room
}

func (h *Hub) newRoom(gameType game.GameType, players [2]int64) *Room {
	return h.newRoomWithBet(gameType, players, 0, "gems")
}

func (h *Hub) newRoomWithBet(gameType game.GameType, players [2]int64, betAmount int64, currency string) *Room {
	h.roomSeq++
	id := strconv.FormatInt(h.roomSeq, 10)

	factory := game.NewFactory()
	g, err := factory.CreateGame(gameType, id, players)
	if err != nil {
		log.Printf("Hub.newRoom: failed to create game: %v", err)
		return nil
	}

	room := NewRoomWithRepo(id, g, h.GameRepo, h.GameHistoryRepo, h)
	room.BetAmount = betAmount
	room.Currency = currency
	room.UserRepo = h.UserRepo
	h.Rooms[id] = room

	log.Printf("Hub.newRoom: created room=%s game=%s bet=%d currency=%s, starting Run()", id, gameType, betAmount, currency)
	go room.Run()

	return room
}

func (h *Hub) OnDisconnect(c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	log.Printf("Hub.OnDisconnect: user=%d gameType=%s bet=%d currency=%s", c.UserID, c.GameType, c.BetAmount, c.Currency)

	// clear waiting slot if this was the waiting client for any key
	for key, waiting := range h.WaitingByKey {
		if waiting != nil && waiting.UserID == c.UserID {
			log.Printf("Hub.OnDisconnect: clearing waiting slot for user=%d key=%s", c.UserID, key)
			delete(h.WaitingByKey, key)
		}
	}
	// Legacy: also check WaitingByGame
	for gt, waiting := range h.WaitingByGame {
		if waiting != nil && waiting.UserID == c.UserID {
			log.Printf("Hub.OnDisconnect: clearing waiting slot (legacy) for user=%d game=%s", c.UserID, gt)
			delete(h.WaitingByGame, gt)
		}
	}

	if roomID, ok := h.UserRoom[c.UserID]; ok {
		log.Printf("Hub.OnDisconnect: user=%d room=%s", c.UserID, roomID)
		if room, ok := h.Rooms[roomID]; ok {
			// Non-blocking send to avoid deadlock if room.Run() exited
			select {
			case room.Disconnect <- c:
			default:
				log.Printf("Hub.OnDisconnect: room=%s Disconnect channel full/closed", roomID)
			}
		}
	}
}

func (h *Hub) StartCleanup() {
	go func() {
		ticker := time.NewTicker(10 * time.Minute)
		defer ticker.Stop()

		for range ticker.C {
			h.cleanupStaleRooms()
		}
	}()

	// More frequent cleanup for waiting slots (every 30 seconds)
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			h.cleanupStaleWaiting()
		}
	}()
}

func (h *Hub) cleanupStaleWaiting() {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Cleanup WaitingByKey
	for key, waiting := range h.WaitingByKey {
		if waiting == nil {
			continue
		}

		// Check if waiting client is still alive
		alive := false
		select {
		case waiting.Send <- []byte(`{"type":"ping"}`):
			alive = true
		default:
			// Channel blocked - client may be dead
		}

		if !alive {
			log.Printf("Hub.cleanupStaleWaiting: removing stale waiting client=%d key=%s", waiting.UserID, key)
			delete(h.WaitingByKey, key)

			// Also cleanup UserRoom mapping
			if roomID, ok := h.UserRoom[waiting.UserID]; ok {
				if room, ok := h.Rooms[roomID]; ok {
					room.mu.Lock()
					delete(room.Clients, waiting.UserID)
					clientsLeft := len(room.Clients)
					room.mu.Unlock()

					if clientsLeft == 0 {
						delete(h.Rooms, roomID)
						log.Printf("Hub.cleanupStaleWaiting: removed empty room=%s", roomID)
					}
				}
				delete(h.UserRoom, waiting.UserID)
			}
		}
	}

	// Legacy: also cleanup WaitingByGame
	for gameType, waiting := range h.WaitingByGame {
		if waiting == nil {
			continue
		}

		alive := false
		select {
		case waiting.Send <- []byte(`{"type":"ping"}`):
			alive = true
		default:
		}

		if !alive {
			log.Printf("Hub.cleanupStaleWaiting: removing stale waiting client=%d game=%s (legacy)", waiting.UserID, gameType)
			delete(h.WaitingByGame, gameType)

			if roomID, ok := h.UserRoom[waiting.UserID]; ok {
				if room, ok := h.Rooms[roomID]; ok {
					room.mu.Lock()
					delete(room.Clients, waiting.UserID)
					clientsLeft := len(room.Clients)
					room.mu.Unlock()

					if clientsLeft == 0 {
						delete(h.Rooms, roomID)
						log.Printf("Hub.cleanupStaleWaiting: removed empty room=%s", roomID)
					}
				}
				delete(h.UserRoom, waiting.UserID)
			}
		}
	}
}

func (h *Hub) cleanupStaleRooms() {
	h.mu.Lock()
	defer h.mu.Unlock()

	now := time.Now()

	for roomID, room := range h.Rooms {
		room.mu.RLock()
		clientsCount := len(room.Clients)
		createdAt := room.createdAt
		room.mu.RUnlock()

		if clientsCount == 0 && now.Sub(createdAt) > time.Hour {
			delete(h.Rooms, roomID)

			for uid, rid := range h.UserRoom {
				if rid == roomID {
					delete(h.UserRoom, uid)
				}
			}

			log.Printf("cleaned up stale room: %s", roomID)
		}
	}
}
