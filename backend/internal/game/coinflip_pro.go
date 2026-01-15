package game

import (
	"crypto/rand"
	"errors"
	"math/big"
	"sync"
	"time"
)

// CoinFlipProGame represents a multi-round coin flip game
type CoinFlipProGame struct {
	ID           string    `json:"id"`
	UserID       int64     `json:"user_id"`
	Bet          int64     `json:"bet"`
	CurrentRound int       `json:"current_round"`
	MaxRounds    int       `json:"max_rounds"`
	Multiplier   float64   `json:"multiplier"`
	Status       string    `json:"status"` // active, cashed_out, lost
	WinAmount    int64     `json:"win_amount"`
	FlipHistory  []bool    `json:"flip_history"` // true = win, false = lose
	CreatedAt    time.Time `json:"created_at"`
	FinishedAt   *time.Time `json:"finished_at,omitempty"`
	mu           sync.RWMutex
}

const (
	CoinFlipProMaxRounds     = 10
	CoinFlipProStatusActive  = "active"
	CoinFlipProStatusCashedOut = "cashed_out"
	CoinFlipProStatusLost    = "lost"
)

// Multipliers for each round (beautiful round numbers)
var CoinFlipProMultipliers = []float64{
	1.0,   // Start (no flips yet)
	1.5,   // Round 1
	2.0,   // Round 2
	3.0,   // Round 3
	5.0,   // Round 4
	8.0,   // Round 5
	12.0,  // Round 6
	20.0,  // Round 7
	35.0,  // Round 8
	60.0,  // Round 9
	100.0, // Round 10
}

// NewCoinFlipProGame creates a new multi-round coin flip game
func NewCoinFlipProGame(id string, userID int64, bet int64) (*CoinFlipProGame, error) {
	if bet <= 0 {
		return nil, errors.New("bet must be positive")
	}

	return &CoinFlipProGame{
		ID:           id,
		UserID:       userID,
		Bet:          bet,
		CurrentRound: 0,
		MaxRounds:    CoinFlipProMaxRounds,
		Multiplier:   1.0,
		Status:       CoinFlipProStatusActive,
		FlipHistory:  []bool{},
		CreatedAt:    time.Now(),
	}, nil
}

// Flip performs a coin flip for the current round
func (g *CoinFlipProGame) Flip() (win bool, err error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.Status != CoinFlipProStatusActive {
		return false, errors.New("game is not active")
	}

	if g.CurrentRound >= g.MaxRounds {
		return false, errors.New("all rounds completed")
	}

	// 50/50 coin flip using crypto/rand
	n, err := rand.Int(rand.Reader, big.NewInt(2))
	if err != nil {
		return false, err
	}
	win = n.Int64() == 0

	g.FlipHistory = append(g.FlipHistory, win)

	if win {
		g.CurrentRound++
		g.Multiplier = CoinFlipProMultipliers[g.CurrentRound]

		// Auto cashout if reached max rounds
		if g.CurrentRound >= g.MaxRounds {
			g.Status = CoinFlipProStatusCashedOut
			g.WinAmount = int64(float64(g.Bet) * g.Multiplier)
			now := time.Now()
			g.FinishedAt = &now
		}
	} else {
		// Lost - game over
		g.Status = CoinFlipProStatusLost
		g.WinAmount = 0
		now := time.Now()
		g.FinishedAt = &now
	}

	return win, nil
}

// CashOut cashes out current winnings
func (g *CoinFlipProGame) CashOut() (int64, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.Status != CoinFlipProStatusActive {
		return 0, errors.New("game is not active")
	}

	if g.CurrentRound == 0 {
		return 0, errors.New("must win at least one round before cashing out")
	}

	g.Status = CoinFlipProStatusCashedOut
	g.WinAmount = int64(float64(g.Bet) * g.Multiplier)
	now := time.Now()
	g.FinishedAt = &now

	return g.WinAmount, nil
}

// GetState returns the current game state (safe for client)
func (g *CoinFlipProGame) GetState() map[string]interface{} {
	g.mu.RLock()
	defer g.mu.RUnlock()

	nextMultiplier := 0.0
	if g.CurrentRound < g.MaxRounds {
		nextMultiplier = CoinFlipProMultipliers[g.CurrentRound+1]
	}

	return map[string]interface{}{
		"id":              g.ID,
		"bet":             g.Bet,
		"current_round":   g.CurrentRound,
		"max_rounds":      g.MaxRounds,
		"multiplier":      g.Multiplier,
		"next_multiplier": nextMultiplier,
		"status":          g.Status,
		"win_amount":      g.WinAmount,
		"potential_win":   int64(float64(g.Bet) * g.Multiplier),
		"flip_history":    g.FlipHistory,
	}
}

// IsActive returns whether the game is still active
func (g *CoinFlipProGame) IsActive() bool {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.Status == CoinFlipProStatusActive
}

// GetProfit returns net profit (win - bet)
func (g *CoinFlipProGame) GetProfit() int64 {
	g.mu.RLock()
	defer g.mu.RUnlock()
	if g.Status == CoinFlipProStatusCashedOut {
		return g.WinAmount - g.Bet
	}
	return -g.Bet // Lost
}

// GetMultiplierTable returns the multiplier table for display
func GetCoinFlipProMultiplierTable() []map[string]interface{} {
	table := make([]map[string]interface{}, CoinFlipProMaxRounds)
	for i := 0; i < CoinFlipProMaxRounds; i++ {
		table[i] = map[string]interface{}{
			"round":      i + 1,
			"multiplier": CoinFlipProMultipliers[i+1],
		}
	}
	return table
}
