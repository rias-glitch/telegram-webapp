package ws

const (
	// client - server
	MsgMove = "move"
	MsgPing = "ping"

	// server - client
	MsgMatchFound = "match_found"
	MsgResult     = "result"
	MsgError      = "error"
)
