package handlers

import (
	"net/http"

	"telegram_webapp/internal/domain"
	"telegram_webapp/internal/game"
	"telegram_webapp/internal/repository"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
)

// DiceRequest represents the dice game request (1-6 dice)
type DiceRequest struct {
	Bet    int64 `json:"bet" binding:"required,min=1"`
	Target int   `json:"target" binding:"required,min=1,max=6"`
}

// DiceResponse represents the dice game response (1-6 dice)
type DiceResponse struct {
	Target     int     `json:"target"`
	Result     int     `json:"result"`
	Multiplier float64 `json:"multiplier"`
	WinChance  float64 `json:"win_chance"`
	Won        bool    `json:"won"`
	WinAmount  int64   `json:"win_amount"`
	Gems       int64   `json:"gems"`
}

// Dice handles the dice game endpoint
func (h *Handler) Dice(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not found"})
		return
	}

	var req DiceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: " + err.Error()})
		return
	}

	// Validate target range (1-6)
	if req.Target < game.DiceMinTarget || req.Target > game.DiceMaxTarget {
		c.JSON(http.StatusBadRequest, gin.H{"error": "target must be between 1 and 6"})
		return
	}

	ctx := c.Request.Context()

	// Start transaction
	tx, err := h.DB.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
		return
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Lock and check balance
	var balance int64
	if err := tx.QueryRow(ctx, `SELECT gems FROM users WHERE id=$1 FOR UPDATE`, userID).Scan(&balance); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
		return
	}
	if balance < req.Bet {
		c.JSON(http.StatusBadRequest, gin.H{"error": "insufficient balance"})
		return
	}

	// Deduct bet
	if _, err := tx.Exec(ctx, `UPDATE users SET gems = gems - $1 WHERE id=$2`, req.Bet, userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
		return
	}

	// Play the game (1-6 dice)
	diceGame := game.NewDiceGame(req.Target)
	diceGame.Roll()

	// Calculate winnings
	winAmount := diceGame.CalculateWinAmount(req.Bet)
	if winAmount > 0 {
		if _, err := tx.Exec(ctx, `UPDATE users SET gems = gems + $1 WHERE id=$2`, winAmount, userID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
			return
		}
	}

	// Record transaction
	netAmount := winAmount - req.Bet
	meta := diceGame.ToDetails()
	meta["bet"] = req.Bet
	meta["win_amount"] = winAmount
	txRecord := &domain.Transaction{
		UserID: userID,
		Type:   "dice",
		Amount: netAmount,
		Meta:   meta,
	}
	if err := h.TransactionRepo.CreateWithTx(ctx, tx, txRecord); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
		return
	}

	// Get new balance
	var newBalance int64
	if err := tx.QueryRow(ctx, `SELECT gems FROM users WHERE id=$1`, userID).Scan(&newBalance); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
		return
	}

	if err := tx.Commit(ctx); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
		return
	}

	// Record game history
	var gameResult domain.GameResult
	if diceGame.Won {
		gameResult = domain.GameResultWin
	} else {
		gameResult = domain.GameResultLose
	}
	go h.RecordGameResult(userID, domain.GameTypeDice, domain.GameModePVE, gameResult, req.Bet, netAmount, meta)

	c.JSON(http.StatusOK, DiceResponse{
		Target:     diceGame.Target,
		Result:     diceGame.Result,
		Multiplier: diceGame.Multiplier,
		WinChance:  diceGame.WinChance(),
		Won:        diceGame.Won,
		WinAmount:  winAmount,
		Gems:       newBalance,
	})
}

// DiceInfo returns dice game configuration info (1-6 dice)
func (h *Handler) DiceInfo(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"min_target":  game.DiceMinTarget,  // 1
		"max_target":  game.DiceMaxTarget,  // 6
		"sides":       game.DiceSides,      // 6
		"multiplier":  game.DiceMultiplier, // 5.5x
		"win_chance":  100.0 / float64(game.DiceSides), // 16.67%
		"description": "Pick a number 1-6. If dice matches your number, you win 5.5x!",
	})
}

// WheelRequest represents the wheel game request
type WheelRequest struct {
	Bet int64 `json:"bet" binding:"required,min=1"`
}

// WheelResponse represents the wheel game response
type WheelResponse struct {
	SegmentID  int     `json:"segment_id"`
	Multiplier float64 `json:"multiplier"`
	Color      string  `json:"color"`
	Label      string  `json:"label"`
	SpinAngle  float64 `json:"spin_angle"`
	WinAmount  int64   `json:"win_amount"`
	Gems       int64   `json:"gems"`
}

// Wheel handles the wheel of fortune game endpoint
func (h *Handler) Wheel(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not found"})
		return
	}

	var req WheelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: " + err.Error()})
		return
	}

	ctx := c.Request.Context()

	// Start transaction
	tx, err := h.DB.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
		return
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Lock and check balance
	var balance int64
	if err := tx.QueryRow(ctx, `SELECT gems FROM users WHERE id=$1 FOR UPDATE`, userID).Scan(&balance); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
		return
	}
	if balance < req.Bet {
		c.JSON(http.StatusBadRequest, gin.H{"error": "insufficient balance"})
		return
	}

	// Deduct bet
	if _, err := tx.Exec(ctx, `UPDATE users SET gems = gems - $1 WHERE id=$2`, req.Bet, userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
		return
	}

	// Play the game
	wheelGame := game.NewWheelGame()
	result := wheelGame.Spin()

	// Calculate winnings
	winAmount := wheelGame.CalculateWinAmount(req.Bet)
	if winAmount > 0 {
		if _, err := tx.Exec(ctx, `UPDATE users SET gems = gems + $1 WHERE id=$2`, winAmount, userID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
			return
		}
	}

	// Record transaction
	netAmount := winAmount - req.Bet
	meta := wheelGame.ToDetails()
	meta["bet"] = req.Bet
	meta["win_amount"] = winAmount
	txRecord := &domain.Transaction{
		UserID: userID,
		Type:   "wheel",
		Amount: netAmount,
		Meta:   meta,
	}
	if err := h.TransactionRepo.CreateWithTx(ctx, tx, txRecord); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
		return
	}

	// Get new balance
	var newBalance int64
	if err := tx.QueryRow(ctx, `SELECT gems FROM users WHERE id=$1`, userID).Scan(&newBalance); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
		return
	}

	if err := tx.Commit(ctx); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
		return
	}

	// Record game history
	var gameResult domain.GameResult
	if result.Multiplier >= 1.0 {
		gameResult = domain.GameResultWin
	} else {
		gameResult = domain.GameResultLose
	}
	go h.RecordGameResult(userID, domain.GameTypeWheel, domain.GameModePVE, gameResult, req.Bet, netAmount, meta)

	c.JSON(http.StatusOK, WheelResponse{
		SegmentID:  result.ID,
		Multiplier: result.Multiplier,
		Color:      result.Color,
		Label:      result.Label,
		SpinAngle:  wheelGame.SpinAngle,
		WinAmount:  winAmount,
		Gems:       newBalance,
	})
}

// WheelInfo returns wheel configuration for frontend
func (h *Handler) WheelInfo(c *gin.Context) {
	wheelGame := game.NewWheelGame()

	c.JSON(http.StatusOK, gin.H{
		"segments":        wheelGame.Segments,
		"expected_return": wheelGame.GetExpectedReturn(),
	})
}

// ============ MINES PRO ============

// MinesProStartRequest represents the start game request
type MinesProStartRequest struct {
	Bet        int64 `json:"bet" binding:"required,min=1"`
	MinesCount int   `json:"mines_count" binding:"required,min=1,max=24"`
}

// MinesProRevealRequest represents the reveal cell request
type MinesProRevealRequest struct {
	Cell *int `json:"cell" binding:"required,min=0,max=24"`
}

// MinesProStart starts a new Mines Pro game
func (h *Handler) MinesProStart(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not found"})
		return
	}

	var req MinesProStartRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: " + err.Error()})
		return
	}

	ctx := c.Request.Context()
	g, err := h.MinesProService.StartGame(ctx, userID, req.Bet, req.MinesCount)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, g.GetState())
}

// MinesProReveal reveals a cell in the active game
func (h *Handler) MinesProReveal(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not found"})
		return
	}

	var req MinesProRevealRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: " + err.Error()})
		return
	}

	if req.Cell == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cell is required"})
		return
	}

	ctx := c.Request.Context()
	hitMine, g, err := h.MinesProService.RevealCell(ctx, userID, *req.Cell)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	state := g.GetState()
	state["hit_mine"] = hitMine

	// Record game if finished
	if !g.IsActive() {
		var result domain.GameResult
		if g.Status == game.MinesProStatusCashedOut {
			result = domain.GameResultWin
		} else {
			result = domain.GameResultLose
		}
		go h.RecordGameResult(userID, domain.GameTypeMinesPro, domain.GameModePVE, result, g.Bet, g.GetProfit(), g.ToDetails())

		// Record transaction
		meta := g.ToDetails()
		meta["bet"] = g.Bet
		meta["win_amount"] = g.WinAmount
		txRecord := &domain.Transaction{
			UserID: userID,
			Type:   "mines_pro",
			Amount: g.GetProfit(),
			Meta:   meta,
		}
		_ = h.TransactionRepo.Create(ctx, txRecord)
	}

	// Get current balance
	user, _ := repository.NewUserRepository(h.DB).GetByID(ctx, userID)
	var balance int64
	if user != nil {
		balance = user.Gems
	}
	state["gems"] = balance

	c.JSON(http.StatusOK, state)
}

// MinesProCashOut cashes out the active game
func (h *Handler) MinesProCashOut(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not found"})
		return
	}

	ctx := c.Request.Context()
	g, err := h.MinesProService.CashOut(ctx, userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Record game history
	go h.RecordGameResult(userID, domain.GameTypeMinesPro, domain.GameModePVE, domain.GameResultWin, g.Bet, g.GetProfit(), g.ToDetails())

	// Record transaction
	meta := g.ToDetails()
	meta["bet"] = g.Bet
	meta["win_amount"] = g.WinAmount
	txRecord := &domain.Transaction{
		UserID: userID,
		Type:   "mines_pro",
		Amount: g.GetProfit(),
		Meta:   meta,
	}
	_ = h.TransactionRepo.Create(ctx, txRecord)

	// Get current balance
	user, _ := repository.NewUserRepository(h.DB).GetByID(ctx, userID)
	var balance int64
	if user != nil {
		balance = user.Gems
	}

	state := g.GetState()
	state["gems"] = balance

	c.JSON(http.StatusOK, state)
}

// MinesProState returns the current game state
func (h *Handler) MinesProState(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not found"})
		return
	}

	g := h.MinesProService.GetActiveGame(userID)
	if g == nil {
		c.JSON(http.StatusOK, gin.H{"active": false})
		return
	}

	state := g.GetState()
	state["active"] = true
	c.JSON(http.StatusOK, state)
}

// MinesProInfo returns game configuration
func (h *Handler) MinesProInfo(c *gin.Context) {
	// Multiplier tables for different mine counts
	tables := make(map[int][]float64)
	for mines := 1; mines <= 24; mines++ {
		tables[mines] = game.MultiplierTable(mines)
	}

	c.JSON(http.StatusOK, gin.H{
		"board_size":        game.MinesProBoardSize,
		"min_mines":         game.MinesProMinMines,
		"max_mines":         game.MinesProMaxMines,
		"multiplier_tables": tables,
	})
}

