package game

import (
	"crypto/rand"
	"errors"
	"math"
	"math/big"
	"sync"
	"time"
)

// MinesPvEGame represents an advanced single-player mines game
// Player can reveal multiple cells and cash out at any time
type MinesPvEGame struct {
	ID            string    `json:"id"`
	UserID        int64     `json:"user_id"`
	BoardSize     int       `json:"board_size"`      // Default 25 (5x5)
	MinesCount    int       `json:"mines_count"`     // 1-24 mines
	Bet           int64     `json:"bet"`
	Mines         []int     `json:"-"`               // Mine positions (hidden from client)
	RevealedCells []int     `json:"revealed_cells"`  // Cells player has revealed
	Multiplier    float64   `json:"multiplier"`      // Current multiplier
	NextMultiplier float64  `json:"next_multiplier"` // Multiplier if next cell is safe
	Status        string    `json:"status"`          // active, cashed_out, exploded
	WinAmount     int64     `json:"win_amount"`      // Amount won (0 if exploded)
	CreatedAt     time.Time `json:"created_at"`
	FinishedAt    *time.Time `json:"finished_at,omitempty"`
	mu            sync.RWMutex
}

const (
	MinesProBoardSize       = 25 // 5x5 grid
	MinesProMinMines        = 1
	MinesProMaxMines        = 24
	MinesProStatusActive    = "active"
	MinesProStatusCashedOut = "cashed_out"
	MinesProStatusExploded  = "exploded"
)

// NewMinesPvEGame creates a new Mines Pro game
func NewMinesPvEGame(id string, userID int64, bet int64, minesCount int) (*MinesPvEGame, error) {
	if minesCount < MinesProMinMines || minesCount > MinesProMaxMines {
		return nil, errors.New("mines count must be between 1 and 24")
	}
	if bet <= 0 {
		return nil, errors.New("bet must be positive")
	}

	g := &MinesPvEGame{
		ID:            id,
		UserID:        userID,
		BoardSize:     MinesProBoardSize,
		MinesCount:    minesCount,
		Bet:           bet,
		RevealedCells: []int{},
		Multiplier:    1.0,
		Status:        MinesProStatusActive,
		CreatedAt:     time.Now(),
	}

	// Generate random mine positions
	g.Mines = g.generateMines()

	// Calculate initial next multiplier
	g.NextMultiplier = g.calculateNextMultiplier()

	return g, nil
}

// generateMines generates random mine positions
func (g *MinesPvEGame) generateMines() []int {
	mines := make([]int, 0, g.MinesCount)
	used := make(map[int]bool)

	for len(mines) < g.MinesCount {
		max := big.NewInt(int64(g.BoardSize))
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			n = big.NewInt(int64(len(mines))) // Fallback
		}
		pos := int(n.Int64())
		if !used[pos] {
			used[pos] = true
			mines = append(mines, pos)
		}
	}

	return mines
}

// calculateMultiplier calculates the multiplier based on revealed safe cells
func (g *MinesPvEGame) calculateMultiplier() float64 {
	if len(g.RevealedCells) == 0 {
		return 1.0
	}

	totalCells := g.BoardSize
	safeCells := totalCells - g.MinesCount
	revealed := len(g.RevealedCells)

	// Provably fair formula:
	// Multiplier = product of (totalRemaining / safeRemaining) for each reveal
	multiplier := 1.0
	for i := 0; i < revealed; i++ {
		totalRemaining := float64(totalCells - i)
		safeRemaining := float64(safeCells - i)
		if safeRemaining <= 0 {
			break
		}
		multiplier *= totalRemaining / safeRemaining
	}

	// Round to 2 decimal places
	return math.Floor(multiplier*100) / 100
}

// calculateNextMultiplier calculates what the multiplier would be if next cell is safe
func (g *MinesPvEGame) calculateNextMultiplier() float64 {
	totalCells := g.BoardSize
	safeCells := totalCells - g.MinesCount
	revealed := len(g.RevealedCells)

	// If all safe cells revealed, no more moves
	if revealed >= safeCells {
		return g.Multiplier
	}

	// Calculate multiplier after one more reveal
	multiplier := 1.0
	for i := 0; i <= revealed; i++ {
		totalRemaining := float64(totalCells - i)
		safeRemaining := float64(safeCells - i)
		if safeRemaining <= 0 {
			break
		}
		multiplier *= totalRemaining / safeRemaining
	}

	return math.Floor(multiplier*100) / 100
}

// Reveal attempts to reveal a cell
func (g *MinesPvEGame) Reveal(cell int) (hitMine bool, err error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.Status != MinesProStatusActive {
		return false, errors.New("game is not active")
	}

	if cell < 0 || cell >= g.BoardSize {
		return false, errors.New("invalid cell position")
	}

	// Check if already revealed
	for _, c := range g.RevealedCells {
		if c == cell {
			return false, errors.New("cell already revealed")
		}
	}

	// Check if hit mine
	for _, m := range g.Mines {
		if m == cell {
			// BOOM!
			g.Status = MinesProStatusExploded
			g.WinAmount = 0
			now := time.Now()
			g.FinishedAt = &now
			return true, nil
		}
	}

	// Safe cell
	g.RevealedCells = append(g.RevealedCells, cell)
	g.Multiplier = g.calculateMultiplier()
	g.NextMultiplier = g.calculateNextMultiplier()

	// Check if all safe cells revealed (auto cashout)
	safeCells := g.BoardSize - g.MinesCount
	if len(g.RevealedCells) >= safeCells {
		g.Status = MinesProStatusCashedOut
		g.WinAmount = int64(float64(g.Bet) * g.Multiplier)
		now := time.Now()
		g.FinishedAt = &now
	}

	return false, nil
}

// CashOut cashes out current winnings
func (g *MinesPvEGame) CashOut() (int64, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.Status != MinesProStatusActive {
		return 0, errors.New("game is not active")
	}

	if len(g.RevealedCells) == 0 {
		return 0, errors.New("must reveal at least one cell before cashing out")
	}

	g.Status = MinesProStatusCashedOut
	g.WinAmount = int64(float64(g.Bet) * g.Multiplier)
	now := time.Now()
	g.FinishedAt = &now

	return g.WinAmount, nil
}

// GetState returns the current game state (safe for client)
func (g *MinesPvEGame) GetState() map[string]interface{} {
	g.mu.RLock()
	defer g.mu.RUnlock()

	state := map[string]interface{}{
		"id":              g.ID,
		"board_size":      g.BoardSize,
		"mines_count":     g.MinesCount,
		"bet":             g.Bet,
		"revealed_cells":  g.RevealedCells,
		"multiplier":      g.Multiplier,
		"next_multiplier": g.NextMultiplier,
		"status":          g.Status,
		"win_amount":      g.WinAmount,
		"potential_win":   int64(float64(g.Bet) * g.Multiplier),
	}

	// Only reveal mines if game is over
	if g.Status != MinesProStatusActive {
		state["mines"] = g.Mines
	}

	return state
}

// IsActive returns whether the game is still active
func (g *MinesPvEGame) IsActive() bool {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.Status == MinesProStatusActive
}

// GetProfit returns net profit (win - bet)
func (g *MinesPvEGame) GetProfit() int64 {
	g.mu.RLock()
	defer g.mu.RUnlock()
	if g.Status == MinesProStatusCashedOut {
		return g.WinAmount - g.Bet
	}
	return -g.Bet // Lost
}

// ToDetails returns game details for storage
func (g *MinesPvEGame) ToDetails() map[string]interface{} {
	g.mu.RLock()
	defer g.mu.RUnlock()

	return map[string]interface{}{
		"board_size":     g.BoardSize,
		"mines_count":    g.MinesCount,
		"mines":          g.Mines,
		"revealed_cells": g.RevealedCells,
		"multiplier":     g.Multiplier,
		"status":         g.Status,
	}
}

// MultiplierTable returns a table of multipliers for different reveal counts
func MultiplierTable(minesCount int) []float64 {
	boardSize := MinesProBoardSize
	safeCells := boardSize - minesCount

	table := make([]float64, safeCells)

	for reveals := 1; reveals <= safeCells; reveals++ {
		multiplier := 1.0
		for i := 0; i < reveals; i++ {
			totalRemaining := float64(boardSize - i)
			safeRemaining := float64(safeCells - i)
			multiplier *= totalRemaining / safeRemaining
		}
		table[reveals-1] = math.Floor(multiplier*100) / 100
	}

	return table
}
