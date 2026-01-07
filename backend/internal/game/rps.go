package game

import (
	"errors"
	"log"
	"sync"
	"time"
)

type RPSGame struct {
	id      string
	players [2]int64
	moves   map[int64]string
	result  *GameResult
	mu      sync.RWMutex
}

func NewRPSGame(id string, players [2]int64) *RPSGame {
	return &RPSGame{
		id:      id,
		players: players,
		moves:   make(map[int64]string),
	}
}

func (g *RPSGame) Type() GameType {
	return TypeRPS
}

func (g *RPSGame) Players() [2]int64 {
	return g.players
}

func (g *RPSGame) SetupTimeout() time.Duration {
	return 0 // RPS doesn't have setup phase
}

func (g *RPSGame) HandleSetup(playerID int64, data interface{}) error {
	return nil // no setup needed
}

func (g *RPSGame) IsSetupComplete() bool {
	return true // always ready
}

func (g *RPSGame) TurnTimeout() time.Duration {
	return 20 * time.Second
}

func (g *RPSGame) HandleMove(playerID int64, data interface{}) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if _, ok := g.moves[playerID]; ok {
		return errors.New("already moved")
	}

	move, ok := data.(string)
	if !ok {
		return errors.New("invalid move type")
	}

	if move != "rock" && move != "paper" && move != "scissors" {
		return errors.New("invalid move value")
	}

	g.moves[playerID] = move
	log.Printf("RPSGame.HandleMove: game=%s player=%d move=%s moves=%v", g.id, playerID, move, g.moves)
	return nil
}

func (g *RPSGame) IsRoundComplete() bool {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return len(g.moves) == 2
}

func (g *RPSGame) CheckResult() *GameResult {
	g.mu.Lock()
	defer g.mu.Unlock()

	if len(g.moves) < 2 {
		log.Printf("RPSGame.CheckResult: not enough moves yet: moves=%v", g.moves)
		return nil
	}

	p1, p2 := g.players[0], g.players[1]
	move1, move2 := g.moves[p1], g.moves[p2]

	log.Printf("RPSGame.CheckResult: p1=%d move=%s, p2=%d move=%s", p1, move1, p2, move2)

	outcome := decide(move1, move2)

	var winner *int64
	if outcome == "win" {
		winner = &p1
		log.Printf("RPSGame.CheckResult: p1 wins")
	} else if outcome == "lose" {
		winner = &p2
		log.Printf("RPSGame.CheckResult: p2 wins")
	} else {
		log.Printf("RPSGame.CheckResult: draw")
	}

	g.result = &GameResult{
		WinnerID: winner,
		Reason:   "game_complete",
		Details: map[string]interface{}{
			"moves": g.moves,
		},
	}

	return g.result
}

func (g *RPSGame) IsFinished() bool {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.result != nil
}

func (g *RPSGame) SerializeState(playerID int64) interface{} {
	g.mu.RLock()
	defer g.mu.RUnlock()

	return map[string]interface{}{
		"type":   "rps",
		"moves":  len(g.moves),
		"result": g.result,
	}
}

func decide(moveA, moveB string) string {
	if moveA == moveB {
		return "draw"
	}

	switch moveA {
	case "rock":
		if moveB == "scissors" {
			return "win"
		}
	case "paper":
		if moveB == "rock" {
			return "win"
		}
	case "scissors":
		if moveB == "paper" {
			return "win"
		}
	}

	return "lose"
}