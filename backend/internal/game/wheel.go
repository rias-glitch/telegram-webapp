package game

import (
	"crypto/rand"
	"math/big"
)

// WheelSegment represents a single segment on the wheel
type WheelSegment struct {
	ID          int     `json:"id"`
	Multiplier  float64 `json:"multiplier"`
	Color       string  `json:"color"`
	Probability float64 `json:"probability"` // 0.0 - 1.0
	Label       string  `json:"label"`
}

// WheelGame represents a single wheel spin
type WheelGame struct {
	Segments []WheelSegment `json:"segments"`
	Result   *WheelSegment  `json:"result"`
	SpinAngle float64       `json:"spin_angle"` // Final angle for frontend animation
}

// DefaultWheelSegments returns the default wheel configuration
func DefaultWheelSegments() []WheelSegment {
	return []WheelSegment{
		{ID: 1, Multiplier: 0.0, Color: "#4a4a4a", Probability: 0.30, Label: "0x"},
		{ID: 2, Multiplier: 0.5, Color: "#e74c3c", Probability: 0.25, Label: "0.5x"},
		{ID: 3, Multiplier: 1.0, Color: "#f39c12", Probability: 0.22, Label: "1x"},
		{ID: 4, Multiplier: 1.5, Color: "#2ecc71", Probability: 0.09, Label: "1.5x"},
		{ID: 5, Multiplier: 2.0, Color: "#3498db", Probability: 0.06, Label: "2x"},
		{ID: 6, Multiplier: 3.0, Color: "#9b59b6", Probability: 0.035, Label: "3x"},
		{ID: 7, Multiplier: 5.0, Color: "#e67e22", Probability: 0.025, Label: "5x"},
		{ID: 8, Multiplier: 10.0, Color: "#f1c40f", Probability: 0.02, Label: "10x"},
	}
}

// NewWheelGame creates a new wheel game with default segments
func NewWheelGame() *WheelGame {
	return &WheelGame{
		Segments: DefaultWheelSegments(),
	}
}

// NewWheelGameWithSegments creates a wheel game with custom segments
func NewWheelGameWithSegments(segments []WheelSegment) *WheelGame {
	return &WheelGame{
		Segments: segments,
	}
}

// Spin performs the wheel spin and returns the winning segment
func (g *WheelGame) Spin() *WheelSegment {
	// Generate cryptographically secure random number
	max := big.NewInt(1000000) // 0.000001 precision
	n, err := rand.Int(rand.Reader, max)
	if err != nil {
		n = big.NewInt(500000)
	}

	random := float64(n.Int64()) / 1000000.0 // 0.0 - 0.999999

	// Find winning segment based on probability distribution
	cumulative := 0.0
	for i := range g.Segments {
		cumulative += g.Segments[i].Probability
		if random < cumulative {
			g.Result = &g.Segments[i]
			break
		}
	}

	// Fallback to last segment if something went wrong
	if g.Result == nil {
		g.Result = &g.Segments[len(g.Segments)-1]
	}

	// Calculate spin angle for frontend animation
	// Each segment takes 360/numSegments degrees
	segmentAngle := 360.0 / float64(len(g.Segments))
	baseAngle := float64(g.Result.ID-1) * segmentAngle

	// Add random offset within segment + multiple full rotations
	offsetMax := big.NewInt(int64(segmentAngle * 100))
	offsetN, _ := rand.Int(rand.Reader, offsetMax)
	offset := float64(offsetN.Int64()) / 100.0

	rotations := 5 // Number of full rotations for animation
	g.SpinAngle = float64(rotations*360) + baseAngle + offset

	return g.Result
}

// CalculateWinAmount returns the win amount for a given bet
func (g *WheelGame) CalculateWinAmount(bet int64) int64 {
	if g.Result == nil {
		return 0
	}
	return int64(float64(bet) * g.Result.Multiplier)
}

// ToDetails returns game details for storage
func (g *WheelGame) ToDetails() map[string]interface{} {
	result := map[string]interface{}{
		"spin_angle": g.SpinAngle,
	}
	if g.Result != nil {
		result["segment_id"] = g.Result.ID
		result["multiplier"] = g.Result.Multiplier
		result["color"] = g.Result.Color
		result["label"] = g.Result.Label
	}
	return result
}

// GetExpectedReturn calculates the expected return of the wheel
func (g *WheelGame) GetExpectedReturn() float64 {
	expected := 0.0
	for _, seg := range g.Segments {
		expected += seg.Probability * seg.Multiplier
	}
	return expected
}
