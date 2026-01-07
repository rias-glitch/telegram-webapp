package ws

// client → server
type MovePayload struct {
	Move string `json:"move"` // rock | paper | scissors
}

// server → client
type MatchFoundPayload struct {
	RoomID     string `json:"room_id"`
	OpponentID int64  `json:"opponent_id"`
}

type ResultPayload struct {
	Winner int64            `json:"winner"`
	Moves  map[int64]string `json:"moves"`
}

type ErrorPayload struct {
	Message string `json:"message"`
}
