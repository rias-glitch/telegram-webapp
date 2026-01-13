package game

import (
	"crypto/rand"
	"math/big"
)

// DiceGame represents a single dice roll game
type DiceGame struct {
	Target     float64 `json:"target"`      // Target number (1.00 - 98.99)
	RollOver   bool    `json:"roll_over"`   // true = win if result > target, false = win if result < target
	Result     float64 `json:"result"`      // Roll result (0.00 - 99.99)
	Multiplier float64 `json:"multiplier"`  // Payout multiplier
	Won        bool    `json:"won"`         // Whether player won
}

const (
	DiceMinTarget = 1.00
	DiceMaxTarget = 98.99
)

// NewDiceGame creates a new dice game with the given parameters
func NewDiceGame(target float64, rollOver bool) *DiceGame {
	// Clamp target to valid range
	if target < DiceMinTarget {
		target = DiceMinTarget
	}
	if target > DiceMaxTarget {
		target = DiceMaxTarget
	}

	g := &DiceGame{
		Target:   target,
		RollOver: rollOver,
	}
	g.Multiplier = g.CalculateMultiplier()
	return g
}

// CalculateMultiplier returns the payout multiplier based on win chance
func (g *DiceGame) CalculateMultiplier() float64 {
	winChance := g.WinChance()
	if winChance <= 0 {
		return 0
	}
	return 100.0 / winChance
}

// WinChance returns the probability of winning (0-100)
func (g *DiceGame) WinChance() float64 {
	if g.RollOver {
		return 99.99 - g.Target
	}
	return g.Target
}

// Roll performs the dice roll and returns the result
func (g *DiceGame) Roll() float64 {
	// Generate cryptographically secure random number
	// Result is 0.00 to 99.99 (10000 possibilities)
	max := big.NewInt(10000)
	n, err := rand.Int(rand.Reader, max)
	if err != nil {
		// Fallback - should never happen
		n = big.NewInt(5000)
	}

	g.Result = float64(n.Int64()) / 100.0

	// Determine win/loss
	if g.RollOver {
		g.Won = g.Result > g.Target
	} else {
		g.Won = g.Result < g.Target
	}

	return g.Result
}

// CalculateWinAmount returns the win amount for a given bet
func (g *DiceGame) CalculateWinAmount(bet int64) int64 {
	if !g.Won {
		return 0
	}
	return int64(float64(bet) * g.Multiplier)
}

// ToDetails returns game details for storage
func (g *DiceGame) ToDetails() map[string]interface{} {
	return map[string]interface{}{
		"target":     g.Target,
		"roll_over":  g.RollOver,
		"result":     g.Result,
		"multiplier": g.Multiplier,
		"win_chance": g.WinChance(),
	}
}
