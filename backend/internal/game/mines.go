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
	// History of moves for each player: playerID -> []MoveResult
	moveHistory map[int64][]MoveResult
	// Last round result for sending to clients
	lastRoundResult *RoundResult
}

type MoveResult struct {
	Cell   int  `json:"cell"`
	HitMine bool `json:"hit_mine"`
	Round  int  `json:"round"`
}

type RoundResult struct {
	Round       int                  `json:"round"`
	PlayerMoves map[int64]MoveResult `json:"player_moves"`
}

type Board struct {
	mines [12]bool
}

func NewMinesGame(id string, players [2]int64) *MinesGame {
	g := &MinesGame{
		id:          id,
		players:     players,
		boards:      make(map[int64]*Board),
		moves:       make(map[int64]int),
		moveHistory: make(map[int64][]MoveResult),
	}
	// Don't initialize boards here - they get created during setup phase
	// Initialize empty move history for both players
	g.moveHistory[players[0]] = []MoveResult{}
	g.moveHistory[players[1]] = []MoveResult{}
	return g
}

func (g *MinesGame) Type() GameType { return TypeMines }
func (g *MinesGame) Players() [2]int64 { return g.players }
func (g *MinesGame) SetupTimeout() time.Duration { return 10 * time.Second }
func (g *MinesGame) TurnTimeout() time.Duration { return 10 * time.Second }

func (g *MinesGame) SetSecondPlayer(playerID int64) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.players[1] = playerID
	// Initialize move history for the new player
	if g.moveHistory[playerID] == nil {
		g.moveHistory[playerID] = []MoveResult{}
	}
}

func (g *MinesGame) IsSetupComplete() bool {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.isSetupCompleteUnlocked()
}

// isSetupCompleteUnlocked - internal version without locking (caller must hold lock)
func (g *MinesGame) isSetupCompleteUnlocked() bool {
	// Both players must have boards with mines placed
	if len(g.boards) < 2 {
		return false
	}
	for _, board := range g.boards {
		if board == nil {
			return false
		}
		// Check if at least one mine is placed
		hasMine := false
		for _, m := range board.mines {
			if m {
				hasMine = true
				break
			}
		}
		if !hasMine {
			return false
		}
	}
	return true
}

func (g *MinesGame) HandleSetup(playerID int64, data interface{}) error {
	return g.HandleMove(playerID, data)  // Setup = тот же HandleMove
}

func (g *MinesGame) HandleMove(playerID int64, data interface{}) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	log.Printf("MinesGame.HandleMove: player=%d data=%v dataType=%T setupComplete=%v", playerID, data, data, g.isSetupCompleteUnlocked())

	// Setup phase - placing mines
	if !g.isSetupCompleteUnlocked() {
		positions, ok := data.([]int)
		if !ok || len(positions) != 4 {
			log.Printf("MinesGame.HandleMove: invalid setup data, using bot positions")
			// Бот расставляет мины случайно
			positions = []int{}
			used := make(map[int]bool)
			for len(positions) < 4 {
				pos := rand.Intn(12) + 1
				if !used[pos] {
					used[pos] = true
					positions = append(positions, pos)
				}
			}
		}

		board := &Board{}
		for _, pos := range positions {
			if pos >= 1 && pos <= 12 {
				board.mines[pos-1] = true
			}
		}
		g.boards[playerID] = board
		log.Printf("MinesGame.HandleMove: player=%d placed mines at %v, boards=%d", playerID, positions, len(g.boards))
		return nil
	}

	// Playing phase - selecting cell on opponent's board
	position, ok := data.(int)
	if !ok || position < 1 || position > 12 {
		log.Printf("MinesGame.HandleMove: invalid move data, using random position")
		// Бот выбирает случайную клетку
		position = rand.Intn(12) + 1
	}

	g.moves[playerID] = position
	log.Printf("MinesGame.HandleMove: player=%d selected cell %d, moves=%v", playerID, position, g.moves)
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

	// Save move history for both players
	move1 := MoveResult{Cell: g.moves[p1], HitMine: hit1, Round: g.round}
	move2 := MoveResult{Cell: g.moves[p2], HitMine: hit2, Round: g.round}
	g.moveHistory[p1] = append(g.moveHistory[p1], move1)
	g.moveHistory[p2] = append(g.moveHistory[p2], move2)

	// Save last round result for clients
	g.lastRoundResult = &RoundResult{
		Round: g.round,
		PlayerMoves: map[int64]MoveResult{
			p1: move1,
			p2: move2,
		},
	}

	// Один подорвался
	if hit1 && !hit2 {
		log.Printf("MinesGame.CheckResult: p2 wins (p1 hit mine)")
		g.result = &GameResult{WinnerID: &p2, Reason: "opponent_hit_mine", Details: g.getResultDetails()}
		return g.result
	}
	if hit2 && !hit1 {
		log.Printf("MinesGame.CheckResult: p1 wins (p2 hit mine)")
		g.result = &GameResult{WinnerID: &p1, Reason: "opponent_hit_mine", Details: g.getResultDetails()}
		return g.result
	}

	// Оба подорвались - ничья в раунде, но игра продолжается
	if hit1 && hit2 {
		log.Printf("MinesGame.CheckResult: both hit mines, round draw")
	}

	// 5 раундов прошло
	if g.round >= 5 {
		log.Printf("MinesGame.CheckResult: draw (5 rounds)")
		g.result = &GameResult{WinnerID: nil, Reason: "draw", Details: g.getResultDetails()}
		return g.result
	}

	// Продолжаем
	log.Printf("MinesGame.CheckResult: continue to round %d", g.round+1)
	g.moves = make(map[int64]int)
	return nil
}

// getResultDetails returns game details for the result
func (g *MinesGame) getResultDetails() map[string]interface{} {
	return map[string]interface{}{
		"rounds":      g.round,
		"moveHistory": g.moveHistory,
	}
}

// GetLastRoundResult returns the last round result (for sending to clients)
func (g *MinesGame) GetLastRoundResult() *RoundResult {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.lastRoundResult
}

// GetMoveHistory returns move history for a player
func (g *MinesGame) GetMoveHistory(playerID int64) []MoveResult {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.moveHistory[playerID]
}

// GetRound returns current round number
func (g *MinesGame) GetRound() int {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.round
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