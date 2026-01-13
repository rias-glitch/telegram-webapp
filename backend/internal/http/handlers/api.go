package handlers

import (
	"context"
	"errors"
	"net/http"
	"time"

	"telegram_webapp/internal/domain"
	"telegram_webapp/internal/repository"
	"telegram_webapp/internal/service"

	"github.com/gin-gonic/gin"
)

// MyProfile returns current user's profile including gems
func (h *Handler) MyProfile(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not found"})
		return
	}

	repo := repository.NewUserRepository(h.DB)
	ctx := c.Request.Context()
	user, err := repo.GetByID(ctx, userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}

	// fetch transactions using repository
	transactions, _ := h.TransactionRepo.GetByUserID(ctx, userID, 100)
	var history []map[string]interface{}
	for _, tx := range transactions {
		history = append(history, map[string]interface{}{
			"type":   tx.Type,
			"amount": tx.Amount,
			"meta":   tx.Meta,
			"date":   tx.CreatedAt,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"id":         user.ID,
		"tg_id":      user.TgID,
		"username":   user.Username,
		"first_name": user.FirstName,
		"created_at": user.CreatedAt,
		"gems":       user.Gems,
		"history":    history,
	})
}

// UpdateBalance adjusts user's gems balance by delta (can be negative)
func (h *Handler) UpdateBalance(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not found"})
		return
	}

	var req struct {
		Delta int64 `json:"delta"`
	}
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "bad request"})
		return
	}

	ctx := c.Request.Context()
	result, err := h.GameService.UpdateBalance(ctx, userID, req.Delta)
	if err != nil {
		if errors.Is(err, service.ErrInsufficientBalance) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "insufficient balance"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"gems": result.NewBalance})
}

// AddHistory records a transaction/history entry
func (h *Handler) AddHistory(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not found"})
		return
	}

	var req struct {
		Type   string                 `json:"type"`
		Amount int64                  `json:"amount"`
		Meta   map[string]interface{} `json:"meta"`
	}
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "bad request"})
		return
	}

	ctx := c.Request.Context()
	if err := h.GameService.AddTransaction(ctx, userID, req.Type, req.Amount, req.Meta); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// GetHistory returns recent transactions for the current user
func (h *Handler) GetHistory(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not found"})
		return
	}

	ctx := c.Request.Context()
	transactions, err := h.GameService.GetTransactionHistory(ctx, userID, 200)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
		return
	}

	var out []map[string]interface{}
	for _, tx := range transactions {
		out = append(out, map[string]interface{}{
			"id":     tx.ID,
			"type":   tx.Type,
			"amount": tx.Amount,
			"meta":   tx.Meta,
			"date":   tx.CreatedAt,
		})
	}
	c.JSON(http.StatusOK, gin.H{"history": out})
}

// CoinFlip performs a server-side coin flip: 50/50. Expects {bet:int}
func (h *Handler) CoinFlip(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not found"})
		return
	}

	var req struct {
		Bet int64 `json:"bet"`
	}
	if err := c.BindJSON(&req); err != nil || req.Bet <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid bet"})
		return
	}

	ctx := c.Request.Context()
	result, meta, err := h.GameService.PlayCoinFlip(ctx, userID, req.Bet)
	if err != nil {
		if errors.Is(err, service.ErrInsufficientBalance) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "insufficient balance"})
			return
		}
		if errors.Is(err, service.ErrBetTooLow) || errors.Is(err, service.ErrBetTooHigh) || errors.Is(err, service.ErrInvalidBet) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
		return
	}

	// Record game history
	var gameResult domain.GameResult
	if result.Win {
		gameResult = domain.GameResultWin
	} else {
		gameResult = domain.GameResultLose
	}
	go h.RecordGameResultWithTimeout(userID, domain.GameTypeCoinflip, domain.GameModePVE, gameResult, req.Bet, result.Awarded-req.Bet, meta)

	// Audit log
	h.AuditService.LogGame(ctx, userID, "coinflip", req.Bet, result.Awarded-req.Bet, result.Win, meta)

	c.JSON(http.StatusOK, gin.H{"win": result.Win, "awarded": result.Awarded, "gems": result.NewBalance})
}

// RPS: server-side rock-paper-scissors PvE
func (h *Handler) RPS(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not found"})
		return
	}

	var req struct {
		Move string `json:"move"`
		Bet  int64  `json:"bet"`
	}
	if err := c.BindJSON(&req); err != nil || (req.Move != "rock" && req.Move != "paper" && req.Move != "scissors") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	ctx := c.Request.Context()
	result, meta, err := h.GameService.PlayRPS(ctx, userID, req.Move, req.Bet)
	if err != nil {
		if errors.Is(err, service.ErrInsufficientBalance) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "insufficient balance"})
			return
		}
		if errors.Is(err, service.ErrBetTooLow) || errors.Is(err, service.ErrBetTooHigh) || errors.Is(err, service.ErrInvalidBet) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
		return
	}

	// Record game history
	var gameResult domain.GameResult
	if result.Result == 1 {
		gameResult = domain.GameResultWin
	} else if result.Result == 0 {
		gameResult = domain.GameResultDraw
	} else {
		gameResult = domain.GameResultLose
	}
	netAmount := result.Awarded - req.Bet
	go h.RecordGameResultWithTimeout(userID, domain.GameTypeRPS, domain.GameModePVE, gameResult, req.Bet, netAmount, meta)

	// Audit log
	h.AuditService.LogGame(ctx, userID, "rps", req.Bet, netAmount, result.Result == 1, meta)

	c.JSON(http.StatusOK, gin.H{
		"move":    result.UserMove,
		"bot":     result.BotMove,
		"result":  result.Result,
		"awarded": result.Awarded,
		"gems":    result.NewBalance,
	})
}

// Mines PvE simple: user picks a cell 1..12; server places 4 mines. If pick is safe, user wins bet*2, else loses.
func (h *Handler) Mines(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not found"})
		return
	}

	var req struct {
		Pick int   `json:"pick"`
		Bet  int64 `json:"bet"`
	}
	if err := c.BindJSON(&req); err != nil || req.Pick < 1 || req.Pick > 12 || req.Bet <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	ctx := c.Request.Context()
	result, meta, err := h.GameService.PlayMines(ctx, userID, req.Pick, req.Bet)
	if err != nil {
		if errors.Is(err, service.ErrInsufficientBalance) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "insufficient balance"})
			return
		}
		if errors.Is(err, service.ErrBetTooLow) || errors.Is(err, service.ErrBetTooHigh) || errors.Is(err, service.ErrInvalidBet) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
		return
	}

	// Record game history
	var gameResult domain.GameResult
	if result.Win {
		gameResult = domain.GameResultWin
	} else {
		gameResult = domain.GameResultLose
	}
	netAmount := result.Awarded - req.Bet
	go h.RecordGameResultWithTimeout(userID, domain.GameTypeMines, domain.GameModePVE, gameResult, req.Bet, netAmount, meta)

	// Audit log
	h.AuditService.LogGame(ctx, userID, "mines", req.Bet, netAmount, result.Win, meta)

	c.JSON(http.StatusOK, gin.H{"win": result.Win, "awarded": result.Awarded, "gems": result.NewBalance})
}

// CaseSpin performs a server-side case/roulette spin with fixed prize distribution
func (h *Handler) CaseSpin(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not found"})
		return
	}

	const cost int64 = 100
	ctx := c.Request.Context()
	result, meta, err := h.GameService.PlayCaseSpin(ctx, userID)
	if err != nil {
		if errors.Is(err, service.ErrInsufficientBalance) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "insufficient balance"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
		return
	}

	// Record game history
	netAmount := result.Prize - cost
	var gameResult domain.GameResult
	if netAmount >= 0 {
		gameResult = domain.GameResultWin
	} else {
		gameResult = domain.GameResultLose
	}
	go h.RecordGameResultWithTimeout(userID, domain.GameTypeCase, domain.GameModeSolo, gameResult, cost, netAmount, meta)

	// Audit log
	h.AuditService.LogGame(ctx, userID, "case", cost, netAmount, netAmount >= 0, meta)

	c.JSON(http.StatusOK, gin.H{"prize": result.Prize, "case_id": result.CaseID, "gems": result.NewBalance})
}

// RecordGameResultWithTimeout records game result with proper context timeout
func (h *Handler) RecordGameResultWithTimeout(userID int64, gameType domain.GameType, mode domain.GameMode, result domain.GameResult, bet int64, winAmount int64, details map[string]interface{}) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	gh := &domain.GameHistory{
		UserID:    userID,
		GameType:  gameType,
		Mode:      mode,
		Result:    result,
		BetAmount: bet,
		WinAmount: winAmount,
		Details:   details,
	}

	_ = h.GameHistoryRepo.Create(ctx, gh)

	// Update quests
	h.updateQuestsWithContext(ctx, userID, string(gameType), string(result))
}

// updateQuestsWithContext updates quests with provided context
func (h *Handler) updateQuestsWithContext(ctx context.Context, userID int64, gameType string, result string) {
	// Получаем все активные квесты
	quests, err := h.QuestRepo.GetActiveQuests(ctx)
	if err != nil {
		return
	}

	for _, quest := range quests {
		// Проверяем соответствие типа игры
		if quest.GameType != nil && *quest.GameType != "any" && *quest.GameType != gameType {
			continue
		}

		// Проверяем тип действия
		shouldIncrement := false
		switch quest.ActionType {
		case domain.ActionTypePlay:
			shouldIncrement = true
		case domain.ActionTypeWin:
			shouldIncrement = (result == "win")
		case domain.ActionTypeLose:
			shouldIncrement = (result == "lose")
		}

		if shouldIncrement {
			_ = h.QuestRepo.IncrementProgress(ctx, userID, quest, 1)
		}
	}
}

// GameLimits returns current bet limits for games
func (h *Handler) GameLimits(c *gin.Context) {
	limits := h.GameService.GetLimits()
	c.JSON(http.StatusOK, gin.H{
		"min_bet": limits.MinBet,
		"max_bet": limits.MaxBet,
	})
}
