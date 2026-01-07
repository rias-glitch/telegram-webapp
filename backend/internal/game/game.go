package game

import "time"

type GameType string

const (
	TypeRPS   GameType = "rps"
	TypeMines GameType = "mines"
)

type Game interface {
	Type() GameType
	Players() [2]int64

	// Setup phase (optional for some games)
	SetupTimeout() time.Duration
	HandleSetup(playerID int64, data interface{}) error
	IsSetupComplete() bool

	// Playing phase
	TurnTimeout() time.Duration
	HandleMove(playerID int64, data interface{}) error
	IsRoundComplete() bool

	// Result checking
	CheckResult() *GameResult
	IsFinished() bool

	// Serialization for client
	SerializeState(playerID int64) interface{}
}

type GameResult struct {
	WinnerID  *int64
	Reason    string
	Details   map[string]interface{}
}