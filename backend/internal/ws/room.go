package ws

import (
	"context"
	"encoding/json"
	"log"
	"sync"
	"time"

	"telegram_webapp/internal/domain"
	"telegram_webapp/internal/game"
	"telegram_webapp/internal/repository"
)
const (
	StateWaiting  = "waiting"
	StatePlaying  = "playing"
	StateFinished = "finished"
)


type Room struct {
	ID      string
	Clients map[int64]*Client

	Register   chan *Client
	Disconnect chan *Client

	mu         sync.RWMutex
	timer      *time.Timer
	createdAt  time.Time

	game     game.Game  // ← игра через интерфейс
	GameRepo *repository.GameRepository
	GameHistoryRepo *repository.GameHistoryRepository
	hub      *Hub       // ← ссылка на Hub для cleanup
}
func NewRoom(id string, g game.Game, hub *Hub) *Room {
	return &Room{
		ID:        id,
		Clients:   make(map[int64]*Client),
		Register:  make(chan *Client, 2),
		Disconnect: make(chan *Client, 2),
		createdAt: time.Now(),
		game:      g,
		hub:       hub,
	}
}

func NewRoomWithRepo(id string, g game.Game, gameRepo *repository.GameRepository, gameHistoryRepo *repository.GameHistoryRepository, hub *Hub) *Room {
	r := NewRoom(id, g, hub)
	r.GameRepo = gameRepo
	r.GameHistoryRepo = gameHistoryRepo
	return r
}



func (r *Room) Run() {
	log.Printf("Room.Run: starting room=%s", r.ID)

	setupDone := make(chan struct{})

	// Setup phase (если нужна для игры)
	if r.game.SetupTimeout() > 0 {
		log.Printf("Room.Run: room=%s has setup phase", r.ID)

		go func() {
			timer := time.NewTimer(r.game.SetupTimeout())
			defer timer.Stop()

			select {
			case <-timer.C:
				log.Printf("Room.Run: room=%s setup timeout", r.ID)
				r.completeSetup()
				close(setupDone)
			case <-setupDone:
				log.Printf("Room.Run: room=%s setup completed manually", r.ID)
			}
		}()
	} else {
		close(setupDone)
	}

	// Обработка событий
	for {
		select {
		case c := <-r.Register:
			log.Printf("Room.Run: room=%s received Register for user=%d", r.ID, c.UserID)
			r.handleRegister(c)

			// Если setup завершён и оба игрока подключены
			r.mu.RLock()
			clientsCount := len(r.Clients)
			r.mu.RUnlock()

			if r.game.IsSetupComplete() && clientsCount == 2 {
				// wait for both clients to signal Ready (with timeout)
				r.mu.RLock()
				clientsCopy := make([]*Client, 0, len(r.Clients))
				for _, cl := range r.Clients {
					clientsCopy = append(clientsCopy, cl)
				}
				r.mu.RUnlock()

				for _, cl := range clientsCopy {
					select {
					case <-cl.Ready:
						log.Printf("Room.Run: client %d Ready in room=%s", cl.UserID, r.ID)
					case <-time.After(1 * time.Second):
						log.Printf("Room.Run: timeout waiting for client %d Ready in room=%s", cl.UserID, r.ID)
					}
				}

				log.Printf("Room.Run: starting round in room=%s", r.ID)
				r.startRound()
			}

		case c := <-r.Disconnect:
			log.Printf("Room.Run: room=%s received Disconnect for user=%d", r.ID, c.UserID)
			r.handleDisconnect(c)

			// Если все отключились - выходим
			r.mu.RLock()
			clientsCount := len(r.Clients)
			r.mu.RUnlock()

			if clientsCount == 0 {
				log.Printf("Room.Run: room=%s is empty, exiting", r.ID)
				r.cleanup()
				return
			}
		}

		// Если игра закончена - выходим
		if r.game.IsFinished() {
			log.Printf("Room.Run: room=%s game finished", r.ID)
			r.saveResult()
			r.cleanup()
			return
		}
	}
}

func (r *Room) completeSetup() {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Для каждого игрока который не завершил setup - вызываем HandleMove с nil (бот сделает)
	for _, playerID := range r.game.Players() {
		if !r.game.IsSetupComplete() {
			r.game.HandleMove(playerID, nil)
		}
	}

	r.broadcast(Message{Type: "setup_complete"})
}

func (r *Room) startRound() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.broadcast(Message{Type: "start"})

	// Запускаем таймер на ход
	if r.timer != nil {
		r.timer.Stop()
	}

	r.timer = time.AfterFunc(r.game.TurnTimeout(), func() {
		r.handleRoundTimeout()
	})
}

func (r *Room) handleRoundTimeout() {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Для каждого игрока который не сходил - бот делает ход
	for _, playerID := range r.game.Players() {
		// Проверяем сходил ли игрок (это зависит от реализации игры)
		// Для простоты просто вызываем HandleMove с nil
		r.game.HandleMove(playerID, nil)
	}

	if r.game.IsRoundComplete() {
		r.checkRound()
	}
}

func (r *Room) checkRound() {
	// debug: log players and connected clients
	r.mu.RLock()
	clients := make([]int64, 0, len(r.Clients))
	for uid := range r.Clients {
		clients = append(clients, uid)
	}
	r.mu.RUnlock()
	log.Printf("Room.checkRound: room=%s players=%v clients=%v", r.ID, r.game.Players(), clients)

	result := r.game.CheckResult()
	log.Printf("Room.checkRound: room=%s check result=%v", r.ID, result)

	if result == nil {
		// Раунд ещё не завершён
		return
	}

	// Отправляем результат
	r.broadcastResult(result)

	if r.game.IsFinished() {
		// Игра полностью закончена
		log.Printf("Room.checkRound: game finished in room %s", r.ID)
		return
	}

	// Игра продолжается - следующий раунд
	log.Printf("Room.checkRound: starting next round in room %s", r.ID)
	r.startRound()
}

func (r *Room) cleanup() {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.timer != nil {
		r.timer.Stop()
	}

	// Удаляем себя из Hub
	if r.hub != nil {
		r.hub.mu.Lock()
		delete(r.hub.Rooms, r.ID)
		for uid := range r.Clients {
			delete(r.hub.UserRoom, uid)
		}
		r.hub.mu.Unlock()
	}
}

func (r *Room) handleRegister(c *Client) {
	r.mu.Lock()

	r.Clients[c.UserID] = c

	log.Printf("Room.handleRegister: room=%s user=%d players=%d game_type=%s", r.ID, c.UserID, len(r.Clients), r.game.Type())

	// Check if the client's writePump has started; do not block here
	if c != nil {
		select {
		case <-c.Ready:
			log.Printf("Room.handleRegister: client %d already ready in room=%s", c.UserID, r.ID)
		default:
			log.Printf("Room.handleRegister: client %d not ready yet in room=%s", c.UserID, r.ID)
		}
	}

	// acknowledge registration to the handler by closing the channel (idempotent)
	if c != nil && c.Registered != nil {
		// close to signal registration (safe because handleRegister called once per client)
		close(c.Registered)
		log.Printf("Room.handleRegister: closed Registered for user=%d room=%s", c.UserID, r.ID)
	}



	if len(r.Clients) == 2 {
		log.Printf("Room.handleRegister: room=%s BOTH PLAYERS REGISTERED; will send matched messages", r.ID)

		// Collect data while holding lock
		players := r.game.Players()
		p1, p2 := players[0], players[1]
		c1 := r.Clients[p1]
		c2 := r.Clients[p2]

		// Release lock before sending to avoid deadlock
		r.mu.Unlock()

		// Send matched to both players
		if c1 != nil {
			data1, _ := json.Marshal(Message{
				Type: "matched",
				Payload: map[string]any{
					"room_id":  r.ID,
					"opponent": map[string]any{"id": p2},
				},
			})
			select {
			case c1.Send <- data1:
				log.Printf("Room.handleRegister: sent matched to p1=%d", p1)
			case <-time.After(1 * time.Second):
				log.Printf("Room.handleRegister: timeout sending matched to p1=%d", p1)
			}
		}

		if c2 != nil {
			data2, _ := json.Marshal(Message{
				Type: "matched",
				Payload: map[string]any{
					"room_id":  r.ID,
					"opponent": map[string]any{"id": p1},
				},
			})
			select {
			case c2.Send <- data2:
				log.Printf("Room.handleRegister: sent matched to p2=%d", p2)
			case <-time.After(1 * time.Second):
				log.Printf("Room.handleRegister: timeout sending matched to p2=%d", p2)
			}
		}

		// Re-acquire lock
		r.mu.Lock()
	} else {
		log.Printf("Room.handleRegister: room=%s waiting for second player (have %d)", r.ID, len(r.Clients))
	}

	// drain any pending messages that the client sent before registration
	c.pendingMu.Lock()
	pending := c.pending
	c.pending = nil
	c.pendingMu.Unlock()

	log.Printf("Room.handleRegister: room=%s user=%d pending_count=%d", r.ID, c.UserID, len(pending))

	// release room lock before processing pending messages to avoid deadlocks
	r.mu.Unlock()

	// send state now that lock is released
	r.send(c.UserID, Message{
		Type: "state",
		Payload: map[string]any{
			"room_id":   r.ID,
			"players":   len(r.Clients),
			"game_type": string(r.game.Type()),
		},
	})

	for i, m := range pending {
		log.Printf("Room.handleRegister: replaying pending[%d] for user=%d: %s", i, c.UserID, string(m))
		r.HandleMessage(c, m)
	}

	// after replaying pending messages, try to check the round a few times synchronously
	for i := 0; i < 5; i++ {
		time.Sleep(20 * time.Millisecond)
		r.checkRound()
		if r.game.IsFinished() {
			break
		}
	}
}

func (r *Room) handleDisconnect(c *Client) {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.Clients, c.UserID)

	log.Printf("Room.handleDisconnect: room=%s user=%d", r.ID, c.UserID)

	// Если остался 1 игрок - он победил
	if len(r.Clients) == 1 {
		for uid := range r.Clients {
			r.send(uid, Message{
				Type: "result",
				Payload: map[string]string{
					"you":    "win",
					"reason": "opponent_left",
				},
			})
		}

		// Завершаем игру (устанавливаем результат принудительно)
		// Это можно сделать через reflection или добавить метод ForceFinish в интерфейс
		// Для простоты просто cleanup
		r.cleanup()
	}
}

func (r *Room) HandleMessage(c *Client, raw []byte) {
	var msg struct {
		Type  string      `json:"type"`
		Value interface{} `json:"value"`
	}

	if err := json.Unmarshal(raw, &msg); err != nil {
		log.Printf("Room.HandleMessage: failed to unmarshal: %v", err)
		return
	}

	log.Printf("Room.HandleMessage: room=%s user=%d type=%s value=%v valueType=%T", r.ID, c.UserID, msg.Type, msg.Value, msg.Value)

	// Convert value to appropriate type for the game
	var moveValue interface{} = msg.Value

	// For RPS, value should be a string
	// For Mines setup, value should be []int
	// For Mines move, value should be int
	if r.game.Type() == "mines" {
		// Handle setup (array of mine positions)
		if arr, ok := msg.Value.([]interface{}); ok {
			intArr := make([]int, len(arr))
			for i, v := range arr {
				if num, ok := v.(float64); ok {
					intArr[i] = int(num)
				}
			}
			moveValue = intArr
			log.Printf("Room.HandleMessage: converted mines setup value to []int: %v", intArr)
		}
		// Handle move (single cell number)
		if num, ok := msg.Value.(float64); ok {
			moveValue = int(num)
			log.Printf("Room.HandleMessage: converted mines move value to int: %d", int(num))
		}
	}

	// Обрабатываем ход через игру
	if err := r.game.HandleMove(c.UserID, moveValue); err != nil {
		log.Printf("Room.HandleMessage: invalid move from user=%d: %v", c.UserID, err)
		r.send(c.UserID, Message{
			Type: "error",
			Payload: map[string]string{"message": err.Error()},
		})
		return
	}

	// Если раунд завершён - проверяем результат
	if r.game.IsRoundComplete() {
		log.Printf("Room.HandleMessage: round complete in room=%s", r.ID)

		r.mu.Lock()
		if r.timer != nil {
			r.timer.Stop()
			r.timer = nil
		}
		r.mu.Unlock()

		// call checkRound, but retry briefly to avoid races between game state updates
		for i := 0; i < 10; i++ {
			log.Printf("Room.HandleMessage: invoking checkRound attempt=%d room=%s", i, r.ID)
			r.checkRound()
			if r.game.IsFinished() {
				log.Printf("Room.HandleMessage: game finished after checkRound attempt=%d room=%s", i, r.ID)
				break
			}
			// small backoff
			time.Sleep(20 * time.Millisecond)
		}
	} else {
		log.Printf("Room.HandleMessage: waiting for other player in room=%s", r.ID)
	}
}

func (r *Room) broadcastResult(result *game.GameResult) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	players := r.game.Players()
	p1, p2 := players[0], players[1]

	log.Printf("Room.broadcastResult: room=%s winner=%v", r.ID, result.WinnerID)

	// Определяем результат для каждого игрока
	var result1, result2 string

	if result.WinnerID == nil {
		result1 = "draw"
		result2 = "draw"
	} else if *result.WinnerID == p1 {
		result1 = "win"
		result2 = "lose"
	} else {
		result1 = "lose"
		result2 = "win"
	}

	log.Printf("Room.broadcastResult: sending results - p1=%s p2=%s", result1, result2)

	// Отправляем каждому игроку
	log.Printf("Room.broadcastResult: sending result to p1=%d", p1)
	r.send(p1, Message{
		Type: "result",
		Payload: map[string]any{
			"you":     result1,
			"reason":  result.Reason,
			"details": result.Details,
		},
	})
	log.Printf("Room.broadcastResult: sent to p1=%d, sending to p2=%d", p1, p2)

	log.Printf("Room.broadcastResult: sending result to p2=%d", p2)
	r.send(p2, Message{
		Type: "result",
		Payload: map[string]any{
			"you":     result2,
			"reason":  result.Reason,
			"details": result.Details,
		},
	})
	log.Printf("Room.broadcastResult: sent to p2=%d", p2)
}


func (r *Room) send(userID int64, msg Message) {
	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("Room.send: marshal error: %v", err)
		return
	}

	r.mu.RLock()
	c, ok := r.Clients[userID]
	r.mu.RUnlock()

	if ok {
		// blocking send with generous timeout to improve reliability in tests
		select {
		case c.Send <- data:
			log.Printf("Room.send: ✅ sent to user=%d type=%s", userID, msg.Type)
		case <-time.After(2 * time.Second):
			log.Printf("Room.send: ❌ timeout sending to user=%d type=%s", userID, msg.Type)
		}
	} else {
		log.Printf("Room.send: ❌ user=%d not in room", userID)
	}

	// if this was a result message, wait for client writePump ack
	if ok && msg.Type == "result" && c != nil && c.ResultAck != nil {
		select {
		case <-c.ResultAck:
			log.Printf("Room.send: delivery ack received for user=%d type=%s", userID, msg.Type)
		case <-time.After(2 * time.Second):
			log.Printf("Room.send: delivery ack TIMEOUT for user=%d type=%s", userID, msg.Type)
		}
	}
}

func (r *Room) broadcast(msg Message) {
	for _, c := range r.Clients {
		// use blocking send to ensure clients receive broadcast (with timeout)
		r.send(c.UserID, msg)
	}
}
func (r *Room) saveResult() {
	result := r.game.CheckResult()
	if result == nil {
		return
	}

	players := r.game.Players()
	p1, p2 := players[0], players[1]

	log.Printf("Room.saveResult: room=%s storing game", r.ID)

	// Save to old games table (for backwards compatibility)
	if r.GameRepo != nil {
		g := &domain.Game{
			RoomID:    r.ID,
			PlayerAID: p1,
			PlayerBID: p2,
			Moves:     make(map[int64]string),
			WinnerID:  result.WinnerID,
		}
		go func(game *domain.Game) {
			if err := r.GameRepo.Create(context.Background(), game); err != nil {
				log.Printf("Room.saveResult: game store failed: %v", err)
			}
		}(g)
	}

	// Save to new game_history table
	if r.GameHistoryRepo != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)

		gameType := string(r.game.Type())
		details := result.Details

		// Determine results for each player
		var result1, result2 domain.GameResult
		var winAmount1, winAmount2 int64

		if result.WinnerID == nil {
			result1 = domain.GameResultDraw
			result2 = domain.GameResultDraw
		} else if *result.WinnerID == p1 {
			result1 = domain.GameResultWin
			result2 = domain.GameResultLose
		} else {
			result1 = domain.GameResultLose
			result2 = domain.GameResultWin
		}

		// Save for player 1
		gh1 := &domain.GameHistory{
			UserID:     p1,
			GameType:   domain.GameType(gameType),
			Mode:       domain.GameModePVP,
			OpponentID: &p2,
			RoomID:     &r.ID,
			Result:     result1,
			BetAmount:  0,
			WinAmount:  winAmount1,
			Details:    details,
		}
		go func() {
			defer cancel()
			if err := r.GameHistoryRepo.Create(ctx, gh1); err != nil {
				log.Printf("Room.saveResult: game_history p1 failed: %v", err)
			}
		}()

		// Save for player 2
		gh2 := &domain.GameHistory{
			UserID:     p2,
			GameType:   domain.GameType(gameType),
			Mode:       domain.GameModePVP,
			OpponentID: &p1,
			RoomID:     &r.ID,
			Result:     result2,
			BetAmount:  0,
			WinAmount:  winAmount2,
			Details:    details,
		}
		go func() {
			if err := r.GameHistoryRepo.Create(ctx, gh2); err != nil {
				log.Printf("Room.saveResult: game_history p2 failed: %v", err)
			}
		}()
	}
}
