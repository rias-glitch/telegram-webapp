package game

import (
	"crypto/rand"
	"math/big"
)

// DiceGame represents a single dice roll game (1-6 dice)
type DiceGame struct {
	Target     int     `json:"target"`      // Target number (1-6)
	Result     int     `json:"result"`      // Roll result (1-6)
	Multiplier float64 `json:"multiplier"`  // Payout multiplier
	Won        bool    `json:"won"`         // Whether player won
	// Legacy fields for backward compatibility
	RollOver   bool    `json:"roll_over,omitempty"`
}

const (
	DiceMinTarget = 1
	DiceMaxTarget = 6
	DiceSides     = 6
	DiceMultiplier = 5.5 // 6 sides, fair odds would be 6x, house edge makes it 5.5x
)

// NewDiceGame creates a new dice game with the given parameters
func NewDiceGame(target int) *DiceGame {
	// Clamp target to valid range (1-6)
	if target < DiceMinTarget {
		target = DiceMinTarget
	}
	if target > DiceMaxTarget {
		target = DiceMaxTarget
	}

	g := &DiceGame{
		Target:     target,
		Multiplier: DiceMultiplier, // Fixed 5.5x for guessing correct number
	}
	return g
}

// CalculateMultiplier returns the payout multiplier (fixed 5.5x for 1-6 dice)
func (g *DiceGame) CalculateMultiplier() float64 {
	return DiceMultiplier
}

// WinChance returns the probability of winning (percentage)
func (g *DiceGame) WinChance() float64 {
	// 1 out of 6 sides = 16.67%
	return 100.0 / float64(DiceSides)
}

// Roll performs the dice roll and returns the result (1-6)
func (g *DiceGame) Roll() int {
	// Generate cryptographically secure random number (1-6)
	max := big.NewInt(DiceSides)
	n, err := rand.Int(rand.Reader, max)
	if err != nil {
		// Fallback - should never happen
		n = big.NewInt(0)
	}

	g.Result = int(n.Int64()) + 1 // Convert 0-5 to 1-6

	// Determine win/loss - you win if you guess the correct number
	g.Won = g.Result == g.Target

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
