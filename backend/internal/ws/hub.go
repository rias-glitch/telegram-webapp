package ws

import (
	"log"
	"strconv"
	"sync"
	"time"

	"telegram_webapp/internal/game"
	"telegram_webapp/internal/repository"
)

type Hub struct {
	Rooms    map[string]*Room
	UserRoom map[int64]string
	mu       sync.RWMutex
	roomSeq  int64
	// Separate waiting queues for each game type
	WaitingByGame map[game.GameType]*Client
	GameRepo        *repository.GameRepository
	GameHistoryRepo *repository.GameHistoryRepository
}

func NewHub(gameRepo *repository.GameRepository, gameHistoryRepo *repository.GameHistoryRepository) *Hub {
	return &Hub{
		Rooms:         make(map[string]*Room),
		UserRoom:      make(map[int64]string),
		WaitingByGame: make(map[game.GameType]*Client),
		GameRepo:        gameRepo,
		GameHistoryRepo: gameHistoryRepo,
	}
}

func (h *Hub) AssignClient(c *Client) *Room {
	h.mu.Lock()

	// Convert string game type to GameType
	gameType := game.GameType(c.GameType)
	if gameType != game.TypeRPS && gameType != game.TypeMines {
		gameType = game.TypeRPS // default
	}

	log.Printf("Hub.AssignClient: user=%d game=%s - assign via waiting slot (rooms=%d)", c.UserID, gameType, len(h.Rooms))

	// If there is a waiting client for this game type, attempt to pair
	waiting := h.WaitingByGame[gameType]
	if waiting != nil {
		// don't pair with self
		if waiting.UserID != c.UserID {
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
						log.Printf("Hub.AssignClient: pairing user=%d with waiting user=%d in room=%s game=%s", c.UserID, waiting.UserID, foundRoom.ID, gameType)

						// create new game with both players
						oldPlayers := foundRoom.game.Players()
						newPlayers := [2]int64{oldPlayers[0], c.UserID}

						factory := game.NewFactory()
						g, err := factory.CreateGame(gameType, foundRoom.ID, newPlayers)
						if err != nil {
							log.Printf("Hub.AssignClient: failed to create game: %v", err)
							h.mu.Unlock()
							return nil
						}

						// reserve second client in room before releasing locks
						foundRoom.mu.Lock()
						foundRoom.game = g
						foundRoom.Clients[c.UserID] = c
						foundRoom.mu.Unlock()

						h.UserRoom[c.UserID] = foundRoom.ID
						// clear waiting slot for this game type
						delete(h.WaitingByGame, gameType)
						h.mu.Unlock()

						log.Printf("Hub.AssignClient: about to register user=%d to room=%s", c.UserID, foundRoom.ID)
						foundRoom.Register <- c
						log.Printf("Hub.AssignClient: registered user=%d to room=%s", c.UserID, foundRoom.ID)

						return foundRoom
					}
					// if waiting client not present in room, clear stale waiting
					log.Printf("Hub.AssignClient: found stale waiting client=%d (not in room), clearing waiting slot", waiting.UserID)
					delete(h.WaitingByGame, gameType)
				} else {
					// room missing, clear stale waiting
					log.Printf("Hub.AssignClient: waiting client's room missing (id=%s), clearing waiting slot", roomID)
					delete(h.WaitingByGame, gameType)
				}
			} else {
				// no mapping for waiting user, clear
				log.Printf("Hub.AssignClient: waiting user mapped to no room, clearing waiting slot")
				delete(h.WaitingByGame, gameType)
			}
		} else {
			// waiting is same user - clear and fallthrough to create room
			log.Printf("Hub.AssignClient: waiting user is same as current user=%d, clearing waiting slot", c.UserID)
			delete(h.WaitingByGame, gameType)
		}
	}

	// Create new room for this game type
	players := [2]int64{c.UserID, 0}
	room := h.newRoom(gameType, players)

	if room == nil {
		log.Printf("Hub.AssignClient: failed to create room for user=%d", c.UserID)
		h.mu.Unlock()
		return nil
	}

	log.Printf("Hub.AssignClient: user=%d created new room=%s game=%s", c.UserID, room.ID, gameType)
	// reserve the slot for this client immediately to avoid race with another AssignClient
	room.mu.Lock()
	room.Clients[c.UserID] = c
	room.mu.Unlock()

	log.Printf("Hub.AssignClient: reserved user=%d in room=%s (pre-register)", c.UserID, room.ID)

	h.UserRoom[c.UserID] = room.ID
	// mark this client as waiting for a peer for this game type
	h.WaitingByGame[gameType] = c

	h.mu.Unlock()

	log.Printf("Hub.AssignClient: registering user=%d to NEW room=%s", c.UserID, room.ID)
	room.Register <- c

	return room
}

func (h *Hub) newRoom(gameType game.GameType, players [2]int64) *Room {
	h.roomSeq++
	id := strconv.FormatInt(h.roomSeq, 10)

	factory := game.NewFactory()
	g, err := factory.CreateGame(gameType, id, players)
	if err != nil {
		log.Printf("Hub.newRoom: failed to create game: %v", err)
		return nil
	}

	room := NewRoomWithRepo(id, g, h.GameRepo, h.GameHistoryRepo, h)
	h.Rooms[id] = room

	log.Printf("Hub.newRoom: created room=%s game=%s, starting Run()", id, gameType)
	go room.Run()

	return room
}

func (h *Hub) OnDisconnect(c *Client) {
	h.mu.Lock()
	// clear waiting slot if this was the waiting client for any game type
	for gt, waiting := range h.WaitingByGame {
		if waiting != nil && waiting.UserID == c.UserID {
			delete(h.WaitingByGame, gt)
		}
	}
	defer h.mu.Unlock()

	if roomID, ok := h.UserRoom[c.UserID]; ok {
		log.Printf("Hub.OnDisconnect: user=%d room=%s", c.UserID, roomID)
		if room, ok := h.Rooms[roomID]; ok {
			room.Disconnect <- c
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
