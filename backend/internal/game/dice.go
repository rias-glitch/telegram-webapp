package game

import (
	"crypto/rand"
	"math/big"
)

// DiceGame represents a single dice roll game (1-6 dice)
type DiceGame struct {
	Target     int     `json:"target"`      // Target number (1-6) or range indicator
	Result     int     `json:"result"`      // Roll result (1-6)
	Mode       string  `json:"mode"`        // "exact", "low" (1-3), "high" (4-6)
	Multiplier float64 `json:"multiplier"`  // Payout multiplier
	Won        bool    `json:"won"`         // Whether player won
	// Legacy fields for backward compatibility
	RollOver   bool    `json:"roll_over,omitempty"`
}

const (
	DiceMinTarget = 1
	DiceMaxTarget = 6
	DiceSides     = 6
	DiceModeExact = "exact"  // Guess exact number
	DiceModeLow   = "low"    // 1-3 range
	DiceModeHigh  = "high"   // 4-6 range

	DiceMultiplierExact = 5.5  // 1/6 chance = 5.5x
	DiceMultiplierRange = 1.8  // 1/2 chance = 1.8x (house edge)
)

// NewDiceGame creates a new dice game with the given parameters
func NewDiceGame(target int, mode string) *DiceGame {
	// Validate mode
	if mode != DiceModeExact && mode != DiceModeLow && mode != DiceModeHigh {
		mode = DiceModeExact // Default to exact mode
	}

	// For range modes, target is not used
	if mode == DiceModeLow || mode == DiceModeHigh {
		target = 0 // Not applicable for range bets
	} else {
		// Clamp target to valid range (1-6) for exact mode
		if target < DiceMinTarget {
			target = DiceMinTarget
		}
		if target > DiceMaxTarget {
			target = DiceMaxTarget
		}
	}

	// Calculate multiplier based on mode
	var multiplier float64
	if mode == DiceModeExact {
		multiplier = DiceMultiplierExact // 5.5x for exact number
	} else {
		multiplier = DiceMultiplierRange // 1.8x for range
	}

	g := &DiceGame{
		Target:     target,
		Mode:       mode,
		Multiplier: multiplier,
	}
	return g
}

// CalculateMultiplier returns the payout multiplier (fixed 5.5x for 1-6 dice)
func (g *DiceGame) CalculateMultiplier() float64 {
	return DiceMultiplier
}

// WinChance returns the probability of winning (percentage)
func (g *DiceGame) WinChance() float64 {
	switch g.Mode {
	case DiceModeExact:
		// 1 out of 6 = 16.67%
		return 100.0 / float64(DiceSides)
	case DiceModeLow, DiceModeHigh:
		// 3 out of 6 = 50%
		return 50.0
	default:
		return 0
	}
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

	// Determine win/loss based on mode
	switch g.Mode {
	case DiceModeExact:
		// Win if exact number matches
		g.Won = g.Result == g.Target
	case DiceModeLow:
		// Win if result is 1, 2, or 3
		g.Won = g.Result >= 1 && g.Result <= 3
	case DiceModeHigh:
		// Win if result is 4, 5, or 6
		g.Won = g.Result >= 4 && g.Result <= 6
	default:
		g.Won = false
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
		"mode":       g.Mode,
		"result":     g.Result,
		"multiplier": g.Multiplier,
		"win_chance": g.WinChance(),
	}
}
