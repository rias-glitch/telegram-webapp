package service

import (
	"context"
	"errors"

	"telegram_webapp/internal/domain"
	"telegram_webapp/internal/repository"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrInsufficientFunds = errors.New("insufficient funds")
	ErrUserNotFound      = errors.New("user not found")
	ErrInvalidAmount     = errors.New("invalid amount")
)

// BalanceService handles all balance operations
type BalanceService struct {
	db              *pgxpool.Pool
	transactionRepo *repository.TransactionRepository
}

// NewBalanceService creates a new balance service
func NewBalanceService(db *pgxpool.Pool) *BalanceService {
	return &BalanceService{
		db:              db,
		transactionRepo: repository.NewTransactionRepository(db),
	}
}

// GetBalance returns user's current balance
func (s *BalanceService) GetBalance(ctx context.Context, userID int64) (int64, error) {
	var balance int64
	err := s.db.QueryRow(ctx, `SELECT gems FROM users WHERE id = $1`, userID).Scan(&balance)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, ErrUserNotFound
		}
		return 0, err
	}
	return balance, nil
}

// Debit deducts amount from user's balance (for bets, purchases, etc.)
func (s *BalanceService) Debit(ctx context.Context, userID int64, amount int64, txType string, meta map[string]interface{}) (newBalance int64, err error) {
	if amount <= 0 {
		return 0, ErrInvalidAmount
	}

	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Lock and check balance
	var balance int64
	err = tx.QueryRow(ctx, `SELECT gems FROM users WHERE id = $1 FOR UPDATE`, userID).Scan(&balance)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, ErrUserNotFound
		}
		return 0, err
	}

	if balance < amount {
		return 0, ErrInsufficientFunds
	}

	// Deduct
	err = tx.QueryRow(ctx, `UPDATE users SET gems = gems - $1 WHERE id = $2 RETURNING gems`, amount, userID).Scan(&newBalance)
	if err != nil {
		return 0, err
	}

	// Record transaction
	transaction := &domain.Transaction{
		UserID: userID,
		Type:   txType,
		Amount: -amount,
		Meta:   meta,
	}
	if err = s.transactionRepo.CreateWithTx(ctx, tx, transaction); err != nil {
		return 0, err
	}

	if err = tx.Commit(ctx); err != nil {
		return 0, err
	}

	return newBalance, nil
}

// Credit adds amount to user's balance (for winnings, deposits, bonuses, etc.)
func (s *BalanceService) Credit(ctx context.Context, userID int64, amount int64, txType string, meta map[string]interface{}) (newBalance int64, err error) {
	if amount <= 0 {
		return 0, ErrInvalidAmount
	}

	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Credit
	err = tx.QueryRow(ctx, `UPDATE users SET gems = gems + $1 WHERE id = $2 RETURNING gems`, amount, userID).Scan(&newBalance)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, ErrUserNotFound
		}
		return 0, err
	}

	// Record transaction
	transaction := &domain.Transaction{
		UserID: userID,
		Type:   txType,
		Amount: amount,
		Meta:   meta,
	}
	if err = s.transactionRepo.CreateWithTx(ctx, tx, transaction); err != nil {
		return 0, err
	}

	if err = tx.Commit(ctx); err != nil {
		return 0, err
	}

	return newBalance, nil
}

// Transfer moves amount from one user to another
func (s *BalanceService) Transfer(ctx context.Context, fromUserID, toUserID int64, amount int64, meta map[string]interface{}) error {
	if amount <= 0 {
		return ErrInvalidAmount
	}

	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Lock both users (order by ID to prevent deadlocks)
	firstID, secondID := fromUserID, toUserID
	if firstID > secondID {
		firstID, secondID = secondID, firstID
	}

	var balance1, balance2 int64
	err = tx.QueryRow(ctx, `SELECT gems FROM users WHERE id = $1 FOR UPDATE`, firstID).Scan(&balance1)
	if err != nil {
		return err
	}
	err = tx.QueryRow(ctx, `SELECT gems FROM users WHERE id = $1 FOR UPDATE`, secondID).Scan(&balance2)
	if err != nil {
		return err
	}

	// Check sender's balance
	var senderBalance int64
	if fromUserID == firstID {
		senderBalance = balance1
	} else {
		senderBalance = balance2
	}

	if senderBalance < amount {
		return ErrInsufficientFunds
	}

	// Execute transfer
	_, err = tx.Exec(ctx, `UPDATE users SET gems = gems - $1 WHERE id = $2`, amount, fromUserID)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `UPDATE users SET gems = gems + $1 WHERE id = $2`, amount, toUserID)
	if err != nil {
		return err
	}

	// Record transactions
	fromTx := &domain.Transaction{
		UserID: fromUserID,
		Type:   "transfer_out",
		Amount: -amount,
		Meta:   meta,
	}
	if err = s.transactionRepo.CreateWithTx(ctx, tx, fromTx); err != nil {
		return err
	}

	toMeta := make(map[string]interface{})
	for k, v := range meta {
		toMeta[k] = v
	}
	toMeta["from_user_id"] = fromUserID

	toTx := &domain.Transaction{
		UserID: toUserID,
		Type:   "transfer_in",
		Amount: amount,
		Meta:   toMeta,
	}
	if err = s.transactionRepo.CreateWithTx(ctx, tx, toTx); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// DebitWithTx deducts amount within an existing transaction
func (s *BalanceService) DebitWithTx(ctx context.Context, tx pgx.Tx, userID int64, amount int64) (newBalance int64, err error) {
	if amount <= 0 {
		return 0, ErrInvalidAmount
	}

	// Check and deduct
	err = tx.QueryRow(ctx,
		`UPDATE users SET gems = gems - $1 WHERE id = $2 AND gems >= $1 RETURNING gems`,
		amount, userID,
	).Scan(&newBalance)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Could be not found or insufficient funds, check which
			var exists bool
			_ = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE id = $1)`, userID).Scan(&exists)
			if !exists {
				return 0, ErrUserNotFound
			}
			return 0, ErrInsufficientFunds
		}
		return 0, err
	}

	return newBalance, nil
}

// CreditWithTx adds amount within an existing transaction
func (s *BalanceService) CreditWithTx(ctx context.Context, tx pgx.Tx, userID int64, amount int64) (newBalance int64, err error) {
	if amount <= 0 {
		return 0, ErrInvalidAmount
	}

	err = tx.QueryRow(ctx,
		`UPDATE users SET gems = gems + $1 WHERE id = $2 RETURNING gems`,
		amount, userID,
	).Scan(&newBalance)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, ErrUserNotFound
		}
		return 0, err
	}

	return newBalance, nil
}

// ClaimBonus gives bonus gems to user if balance is low
func (s *BalanceService) ClaimBonus(ctx context.Context, userID int64, bonusAmount int64, minBalanceThreshold int64) (newBalance int64, err error) {
	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Check current balance
	var balance int64
	err = tx.QueryRow(ctx, `SELECT gems FROM users WHERE id = $1 FOR UPDATE`, userID).Scan(&balance)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, ErrUserNotFound
		}
		return 0, err
	}

	if balance >= minBalanceThreshold {
		return balance, errors.New("balance too high for bonus")
	}

	// Add bonus
	err = tx.QueryRow(ctx, `UPDATE users SET gems = gems + $1 WHERE id = $2 RETURNING gems`, bonusAmount, userID).Scan(&newBalance)
	if err != nil {
		return 0, err
	}

	// Record transaction
	transaction := &domain.Transaction{
		UserID: userID,
		Type:   "bonus",
		Amount: bonusAmount,
		Meta:   map[string]interface{}{"reason": "low_balance_bonus"},
	}
	if err = s.transactionRepo.CreateWithTx(ctx, tx, transaction); err != nil {
		return 0, err
	}

	return newBalance, tx.Commit(ctx)
}

// GetTransactionHistory returns user's transaction history
func (s *BalanceService) GetTransactionHistory(ctx context.Context, userID int64, limit int) ([]*domain.Transaction, error) {
	return s.transactionRepo.GetByUserID(ctx, userID, limit)
}
