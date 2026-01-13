package service

import (
	"context"
	"errors"
	"math/rand"
	"time"

	"telegram_webapp/internal/domain"
	"telegram_webapp/internal/repository"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrInsufficientBalance = errors.New("insufficient balance")
	ErrBetTooLow           = errors.New("bet below minimum")
	ErrBetTooHigh          = errors.New("bet exceeds maximum")
	ErrInvalidBet          = errors.New("invalid bet amount")
)

// GameLimits holds bet limits configuration
type GameLimits struct {
	MinBet int64
	MaxBet int64
}

// GameService handles game business logic
type GameService struct {
	db              *pgxpool.Pool
	transactionRepo *repository.TransactionRepository
	limits          GameLimits
}

// NewGameService creates a new game service
func NewGameService(db *pgxpool.Pool) *GameService {
	return &GameService{
		db:              db,
		transactionRepo: repository.NewTransactionRepository(db),
		limits:          GameLimits{MinBet: 10, MaxBet: 100000}, // defaults
	}
}

// NewGameServiceWithLimits creates a game service with custom limits
func NewGameServiceWithLimits(db *pgxpool.Pool, minBet, maxBet int64) *GameService {
	return &GameService{
		db:              db,
		transactionRepo: repository.NewTransactionRepository(db),
		limits:          GameLimits{MinBet: minBet, MaxBet: maxBet},
	}
}

// ValidateBet checks if bet is within allowed limits
func (s *GameService) ValidateBet(bet int64) error {
	if bet <= 0 {
		return ErrInvalidBet
	}
	if bet < s.limits.MinBet {
		return ErrBetTooLow
	}
	if bet > s.limits.MaxBet {
		return ErrBetTooHigh
	}
	return nil
}

// GetLimits returns current bet limits
func (s *GameService) GetLimits() GameLimits {
	return s.limits
}

// CoinFlipResult contains the result of a coin flip game
type CoinFlipResult struct {
	Win        bool  `json:"win"`
	Awarded    int64 `json:"awarded"`
	NewBalance int64 `json:"gems"`
}

// PlayCoinFlip performs a coin flip game
func (s *GameService) PlayCoinFlip(ctx context.Context, userID int64, bet int64) (*CoinFlipResult, map[string]interface{}, error) {
	if err := s.ValidateBet(bet); err != nil {
		return nil, nil, err
	}

	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Lock and check balance
	var balance int64
	if err := tx.QueryRow(ctx, `SELECT gems FROM users WHERE id=$1 FOR UPDATE`, userID).Scan(&balance); err != nil {
		return nil, nil, err
	}
	if balance < bet {
		return nil, nil, ErrInsufficientBalance
	}

	// Deduct bet
	if _, err := tx.Exec(ctx, `UPDATE users SET gems = gems - $1 WHERE id=$2`, bet, userID); err != nil {
		return nil, nil, err
	}

	// Coin flip
	win := rand.Int63n(2) == 0

	awarded := int64(0)
	if win {
		awarded = bet * 2
		if _, err := tx.Exec(ctx, `UPDATE users SET gems = gems + $1 WHERE id=$2`, awarded, userID); err != nil {
			return nil, nil, err
		}
	}

	// Record transaction
	meta := map[string]interface{}{"bet": bet, "awarded": awarded, "win": win}
	transaction := &domain.Transaction{
		UserID: userID,
		Type:   "coinflip",
		Amount: awarded - bet,
		Meta:   meta,
	}
	if err := s.transactionRepo.CreateWithTx(ctx, tx, transaction); err != nil {
		return nil, nil, err
	}

	// Get new balance
	var newBalance int64
	if err := tx.QueryRow(ctx, `SELECT gems FROM users WHERE id=$1`, userID).Scan(&newBalance); err != nil {
		return nil, nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, nil, err
	}

	return &CoinFlipResult{
		Win:        win,
		Awarded:    awarded,
		NewBalance: newBalance,
	}, meta, nil
}

// RPSResult contains the result of an RPS game
type RPSResult struct {
	UserMove   string `json:"move"`
	BotMove    string `json:"bot"`
	Result     int    `json:"result"` // 1=win, 0=draw, -1=lose
	Awarded    int64  `json:"awarded"`
	NewBalance int64  `json:"gems"`
}

// PlayRPS performs an RPS game
func (s *GameService) PlayRPS(ctx context.Context, userID int64, move string, bet int64) (*RPSResult, map[string]interface{}, error) {
	if move != "rock" && move != "paper" && move != "scissors" {
		return nil, nil, errors.New("invalid move")
	}

	// Validate bet if provided
	if bet > 0 {
		if err := s.ValidateBet(bet); err != nil {
			return nil, nil, err
		}
	}

	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Handle bet deduction if bet > 0
	if bet > 0 {
		var balance int64
		if err := tx.QueryRow(ctx, `SELECT gems FROM users WHERE id=$1 FOR UPDATE`, userID).Scan(&balance); err != nil {
			return nil, nil, err
		}
		if balance < bet {
			return nil, nil, ErrInsufficientBalance
		}
		if _, err := tx.Exec(ctx, `UPDATE users SET gems = gems - $1 WHERE id=$2`, bet, userID); err != nil {
			return nil, nil, err
		}
	}

	// Bot move
	moves := []string{"rock", "paper", "scissors"}
	botMove := moves[rand.Intn(3)]

	// Determine winner: 1=user win, 0=draw, -1=bot win
	result := 0
	if move == botMove {
		result = 0
	} else if (move == "rock" && botMove == "scissors") ||
		(move == "paper" && botMove == "rock") ||
		(move == "scissors" && botMove == "paper") {
		result = 1
	} else {
		result = -1
	}

	awarded := int64(0)
	if result == 1 && bet > 0 {
		awarded = bet * 2
		if _, err := tx.Exec(ctx, `UPDATE users SET gems = gems + $1 WHERE id=$2`, awarded, userID); err != nil {
			return nil, nil, err
		}
	}

	// Record transaction
	meta := map[string]interface{}{"move": move, "bot": botMove, "result": result}
	netAmount := awarded - bet
	transaction := &domain.Transaction{
		UserID: userID,
		Type:   "rps",
		Amount: netAmount,
		Meta:   meta,
	}
	if err := s.transactionRepo.CreateWithTx(ctx, tx, transaction); err != nil {
		return nil, nil, err
	}

	var newBalance int64
	if err := tx.QueryRow(ctx, `SELECT gems FROM users WHERE id=$1`, userID).Scan(&newBalance); err != nil {
		return nil, nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, nil, err
	}

	return &RPSResult{
		UserMove:   move,
		BotMove:    botMove,
		Result:     result,
		Awarded:    awarded,
		NewBalance: newBalance,
	}, meta, nil
}

// MinesResult contains the result of a mines game
type MinesResult struct {
	Win        bool       `json:"win"`
	Awarded    int64      `json:"awarded"`
	NewBalance int64      `json:"gems"`
	Mines      map[int]bool `json:"-"`
}

// PlayMines performs a mines game
func (s *GameService) PlayMines(ctx context.Context, userID int64, pick int, bet int64) (*MinesResult, map[string]interface{}, error) {
	if pick < 1 || pick > 12 {
		return nil, nil, errors.New("invalid pick")
	}
	if err := s.ValidateBet(bet); err != nil {
		return nil, nil, err
	}

	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var balance int64
	if err := tx.QueryRow(ctx, `SELECT gems FROM users WHERE id=$1 FOR UPDATE`, userID).Scan(&balance); err != nil {
		return nil, nil, err
	}
	if balance < bet {
		return nil, nil, ErrInsufficientBalance
	}

	if _, err := tx.Exec(ctx, `UPDATE users SET gems = gems - $1 WHERE id=$2`, bet, userID); err != nil {
		return nil, nil, err
	}

	// Place 4 unique mines
	mines := map[int]bool{}
	for len(mines) < 4 {
		n := rand.Intn(12) + 1
		mines[n] = true
	}

	pickIsMine := mines[pick]
	awarded := int64(0)
	if !pickIsMine {
		awarded = bet * 2
		if _, err := tx.Exec(ctx, `UPDATE users SET gems = gems + $1 WHERE id=$2`, awarded, userID); err != nil {
			return nil, nil, err
		}
	}

	meta := map[string]interface{}{"pick": pick, "mines": mines, "win": !pickIsMine}
	netAmount := awarded - bet
	transaction := &domain.Transaction{
		UserID: userID,
		Type:   "mines",
		Amount: netAmount,
		Meta:   meta,
	}
	if err := s.transactionRepo.CreateWithTx(ctx, tx, transaction); err != nil {
		return nil, nil, err
	}

	var newBalance int64
	if err := tx.QueryRow(ctx, `SELECT gems FROM users WHERE id=$1`, userID).Scan(&newBalance); err != nil {
		return nil, nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, nil, err
	}

	return &MinesResult{
		Win:        !pickIsMine,
		Awarded:    awarded,
		NewBalance: newBalance,
		Mines:      mines,
	}, meta, nil
}

// CaseSpinResult contains the result of a case spin game
type CaseSpinResult struct {
	CaseID     int   `json:"case_id"`
	Prize      int64 `json:"prize"`
	NewBalance int64 `json:"gems"`
}

// PlayCaseSpin performs a case spin game
func (s *GameService) PlayCaseSpin(ctx context.Context, userID int64) (*CaseSpinResult, map[string]interface{}, error) {
	const cost int64 = 100
	// Prize distribution
	cases := []struct {
		ID     int
		Amount int64
		Prob   float64
	}{
		{1, 250, 0.5},
		{2, 500, 0.2},
		{3, 750, 0.15},
		{4, 1000, 0.10},
		{5, 5000, 0.05},
	}

	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Lock balance
	var balance int64
	if err := tx.QueryRow(ctx, `SELECT gems FROM users WHERE id=$1 FOR UPDATE`, userID).Scan(&balance); err != nil {
		return nil, nil, err
	}
	if balance < cost {
		return nil, nil, ErrInsufficientBalance
	}

	if _, err := tx.Exec(ctx, `UPDATE users SET gems = gems - $1 WHERE id=$2`, cost, userID); err != nil {
		return nil, nil, err
	}

	// Weighted pick
	r := rand.Float64()
	acc := 0.0
	var picked struct {
		ID     int
		Amount int64
		Prob   float64
	}
	for _, cs := range cases {
		acc += cs.Prob
		if r <= acc {
			picked = cs
			break
		}
	}
	if picked.Amount == 0 {
		picked = cases[len(cases)-1]
	}

	awarded := picked.Amount
	if awarded > 0 {
		if _, err := tx.Exec(ctx, `UPDATE users SET gems = gems + $1 WHERE id=$2`, awarded, userID); err != nil {
			return nil, nil, err
		}
	}

	netAmount := awarded - cost
	meta := map[string]interface{}{"case_id": picked.ID, "prize": awarded, "cost": cost}
	transaction := &domain.Transaction{
		UserID: userID,
		Type:   "case",
		Amount: netAmount,
		Meta:   meta,
	}
	if err := s.transactionRepo.CreateWithTx(ctx, tx, transaction); err != nil {
		return nil, nil, err
	}

	var newBalance int64
	if err := tx.QueryRow(ctx, `SELECT gems FROM users WHERE id=$1`, userID).Scan(&newBalance); err != nil {
		return nil, nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, nil, err
	}

	return &CaseSpinResult{
		CaseID:     picked.ID,
		Prize:      awarded,
		NewBalance: newBalance,
	}, meta, nil
}

// UpdateBalanceResult contains the result of balance update
type UpdateBalanceResult struct {
	NewBalance int64 `json:"gems"`
}

// UpdateBalance adjusts user's balance
func (s *GameService) UpdateBalance(ctx context.Context, userID int64, delta int64) (*UpdateBalanceResult, error) {
	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var newGems int64
	if delta < 0 {
		// Attempt atomic deduction only if enough balance
		row := tx.QueryRow(ctx, `UPDATE users SET gems = gems + $1 WHERE id=$2 AND gems + $1 >= 0 RETURNING gems`, delta, userID)
		if err := row.Scan(&newGems); err != nil {
			if err == pgx.ErrNoRows {
				return nil, ErrInsufficientBalance
			}
			return nil, err
		}
	} else {
		row := tx.QueryRow(ctx, `UPDATE users SET gems = gems + $1 WHERE id=$2 RETURNING gems`, delta, userID)
		if err := row.Scan(&newGems); err != nil {
			return nil, err
		}
	}

	// Insert transaction record
	transaction := &domain.Transaction{
		UserID: userID,
		Type:   "balance_adjust",
		Amount: delta,
		Meta:   map[string]interface{}{"reason": "manual"},
	}
	if err := s.transactionRepo.CreateWithTx(ctx, tx, transaction); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return &UpdateBalanceResult{NewBalance: newGems}, nil
}

// GetTransactionHistory returns user's transaction history
func (s *GameService) GetTransactionHistory(ctx context.Context, userID int64, limit int) ([]*domain.Transaction, error) {
	return s.transactionRepo.GetByUserID(ctx, userID, limit)
}

// AddTransaction adds a transaction record
func (s *GameService) AddTransaction(ctx context.Context, userID int64, txType string, amount int64, meta map[string]interface{}) error {
	transaction := &domain.Transaction{
		UserID: userID,
		Type:   txType,
		Amount: amount,
		Meta:   meta,
	}
	return s.transactionRepo.Create(ctx, transaction)
}

func init() {
	rand.Seed(time.Now().UnixNano())
}
