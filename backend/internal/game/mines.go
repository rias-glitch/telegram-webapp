package game

import (
	"log"
	"math/rand"
	"sync"
	"time"
)

type MinesGame struct {
	id       string
	players  [2]int64
	boards   map[int64]*Board
	moves    map[int64]int
	round    int
	result   *GameResult
	mu       sync.RWMutex
}

type Board struct {
	mines [12]bool
}

func NewMinesGame(id string, players [2]int64) *MinesGame {
	g := &MinesGame{
		id:      id,
		players: players,
		boards:  make(map[int64]*Board),
		moves:   make(map[int64]int),
	}

	// Инициализируем пустые поля для обоих игроков
	g.boards[players[0]] = &Board{}
	g.boards[players[1]] = &Board{}

	return g
}

func (g *MinesGame) Type() GameType { return TypeMines }
func (g *MinesGame) Players() [2]int64 { return g.players }
func (g *MinesGame) SetupTimeout() time.Duration { return 10 * time.Second }
func (g *MinesGame) TurnTimeout() time.Duration { return 10 * time.Second }

func (g *MinesGame) IsSetupComplete() bool {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return len(g.boards) == 2
}
func (g *MinesGame) HandleSetup(playerID int64, data interface{}) error {
	return g.HandleMove(playerID, data)  // Setup = тот же HandleMove
}
func (g *MinesGame) HandleMove(playerID int64, data interface{}) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	// Setup phase
	if !g.IsSetupComplete() {
		positions, ok := data.([]int)
		if !ok || len(positions) != 4 {
			// Бот расставляет мины на 1,2,3,4
			positions = []int{1, 2, 3, 4}
		}

		board := &Board{}
		for _, pos := range positions {
			if pos >= 1 && pos <= 12 {
				board.mines[pos-1] = true
			}
		}
		g.boards[playerID] = board
		return nil
	}

	// Playing phase
	position, ok := data.(int)
	if !ok || position < 1 || position > 12 {
		// Бот выбирает случайную клетку
		position = rand.Intn(12) + 1
	}

	g.moves[playerID] = position
	return nil
}

func (g *MinesGame) IsRoundComplete() bool {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return len(g.moves) == 2
}

func (g *MinesGame) CheckResult() *GameResult {
	g.mu.Lock()
	defer g.mu.Unlock()

	if len(g.moves) < 2 {
		log.Printf("MinesGame.CheckResult: waiting for moves (have %d)", len(g.moves))
		return nil
	}

	g.round++

	p1, p2 := g.players[0], g.players[1]

	log.Printf("MinesGame.CheckResult: round=%d p1=%d pos=%d, p2=%d pos=%d", g.round, p1, g.moves[p1], p2, g.moves[p2])

	hit1 := g.boards[p2].mines[g.moves[p1]-1]
	hit2 := g.boards[p1].mines[g.moves[p2]-1]

	log.Printf("MinesGame.CheckResult: p1_hit=%v p2_hit=%v", hit1, hit2)

	// Один подорвался
	if hit1 && !hit2 {
		log.Printf("MinesGame.CheckResult: p2 wins (p1 hit mine)")
		g.result = &GameResult{WinnerID: &p2, Reason: "opponent_hit_mine"}
		return g.result
	}
	if hit2 && !hit1 {
		log.Printf("MinesGame.CheckResult: p1 wins (p2 hit mine)")
		g.result = &GameResult{WinnerID: &p1, Reason: "opponent_hit_mine"}
		return g.result
	}

	// 5 раундов прошло
	if g.round >= 5 {
		log.Printf("MinesGame.CheckResult: draw (5 rounds)")
		g.result = &GameResult{WinnerID: nil, Reason: "draw"}
		return g.result
	}

	// Продолжаем
	log.Printf("MinesGame.CheckResult: continue to round %d", g.round+1)
	g.moves = make(map[int64]int)
	return nil
}


func (g *MinesGame) IsFinished() bool {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.result != nil
}

// ← Добавь этот метод
func (g *MinesGame) SerializeState(playerID int64) interface{} {
	g.mu.RLock()
	defer g.mu.RUnlock()

	return map[string]interface{}{
		"type":  "mines",
		"round": g.round,
		"result": g.result,
	}
}