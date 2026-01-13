package repository

import (
	"context"
	"time"

	"telegram_webapp/internal/domain"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type WithdrawalRepository struct {
	db *pgxpool.Pool
}

func NewWithdrawalRepository(db *pgxpool.Pool) *WithdrawalRepository {
	return &WithdrawalRepository{db: db}
}

// GetByID retrieves withdrawal by ID
func (r *WithdrawalRepository) GetByID(ctx context.Context, id int64) (*domain.Withdrawal, error) {
	row := r.db.QueryRow(ctx, `
		SELECT id, user_id, wallet_address, coins_amount, ton_amount_nano, fee_coins, exchange_rate,
		       status, tx_hash, tx_lt, admin_notes, created_at, processed_at, completed_at,
		       gems_amount, fee_gems
		FROM ton_withdrawals
		WHERE id = $1
	`, id)

	return scanWithdrawal(row)
}

// GetByUserID retrieves all withdrawals for a user
func (r *WithdrawalRepository) GetByUserID(ctx context.Context, userID int64, limit int) ([]domain.Withdrawal, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, user_id, wallet_address, coins_amount, ton_amount_nano, fee_coins, exchange_rate,
		       status, tx_hash, tx_lt, admin_notes, created_at, processed_at, completed_at,
		       gems_amount, fee_gems
		FROM ton_withdrawals
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2
	`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanWithdrawals(rows)
}

// GetPending retrieves all pending withdrawals awaiting processing
func (r *WithdrawalRepository) GetPending(ctx context.Context) ([]domain.Withdrawal, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, user_id, wallet_address, coins_amount, ton_amount_nano, fee_coins, exchange_rate,
		       status, tx_hash, tx_lt, admin_notes, created_at, processed_at, completed_at,
		       gems_amount, fee_gems
		FROM ton_withdrawals
		WHERE status = 'pending'
		ORDER BY created_at ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanWithdrawals(rows)
}

// Create creates a new withdrawal request (coins)
func (r *WithdrawalRepository) Create(ctx context.Context, w *domain.Withdrawal) error {
	return r.db.QueryRow(ctx, `
		INSERT INTO ton_withdrawals (user_id, wallet_address, coins_amount, ton_amount_nano, fee_coins, exchange_rate, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, created_at
	`, w.UserID, w.WalletAddress, w.CoinsAmount, w.TonAmountNano, w.FeeCoins, w.ExchangeRate, w.Status).Scan(&w.ID, &w.CreatedAt)
}

// UpdateStatus updates withdrawal status
func (r *WithdrawalRepository) UpdateStatus(ctx context.Context, id int64, status domain.WithdrawalStatus) error {
	_, err := r.db.Exec(ctx, `
		UPDATE ton_withdrawals SET status = $2 WHERE id = $1
	`, id, status)
	return err
}

// MarkProcessing marks withdrawal as being processed
func (r *WithdrawalRepository) MarkProcessing(ctx context.Context, id int64) error {
	now := time.Now()
	_, err := r.db.Exec(ctx, `
		UPDATE ton_withdrawals SET status = 'processing', processed_at = $2 WHERE id = $1
	`, id, now)
	return err
}

// MarkSent marks withdrawal as sent with tx hash
func (r *WithdrawalRepository) MarkSent(ctx context.Context, id int64, txHash string, txLt int64) error {
	_, err := r.db.Exec(ctx, `
		UPDATE ton_withdrawals SET status = 'sent', tx_hash = $2, tx_lt = $3 WHERE id = $1
	`, id, txHash, txLt)
	return err
}

// MarkCompleted marks withdrawal as completed
func (r *WithdrawalRepository) MarkCompleted(ctx context.Context, id int64) error {
	now := time.Now()
	_, err := r.db.Exec(ctx, `
		UPDATE ton_withdrawals SET status = 'completed', completed_at = $2 WHERE id = $1
	`, id, now)
	return err
}

// MarkFailed marks withdrawal as failed
func (r *WithdrawalRepository) MarkFailed(ctx context.Context, id int64, notes string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE ton_withdrawals SET status = 'failed', admin_notes = $2 WHERE id = $1
	`, id, notes)
	return err
}

// Cancel cancels a pending withdrawal
func (r *WithdrawalRepository) Cancel(ctx context.Context, id int64, userID int64) error {
	result, err := r.db.Exec(ctx, `
		UPDATE ton_withdrawals SET status = 'cancelled'
		WHERE id = $1 AND user_id = $2 AND status = 'pending'
	`, id, userID)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

// GetTotalWithdrawnToday returns total gems withdrawn by user today (legacy)
func (r *WithdrawalRepository) GetTotalWithdrawnToday(ctx context.Context, userID int64) (int64, error) {
	var total int64
	today := time.Now().Truncate(24 * time.Hour)
	err := r.db.QueryRow(ctx, `
		SELECT COALESCE(SUM(gems_amount), 0)
		FROM ton_withdrawals
		WHERE user_id = $1 AND created_at >= $2 AND status NOT IN ('failed', 'cancelled')
	`, userID, today).Scan(&total)
	return total, err
}

// GetTotalCoinsWithdrawnToday returns total coins withdrawn by user today
func (r *WithdrawalRepository) GetTotalCoinsWithdrawnToday(ctx context.Context, userID int64) (int64, error) {
	var total int64
	today := time.Now().Truncate(24 * time.Hour)
	err := r.db.QueryRow(ctx, `
		SELECT COALESCE(SUM(coins_amount), 0)
		FROM ton_withdrawals
		WHERE user_id = $1 AND created_at >= $2 AND status NOT IN ('failed', 'cancelled')
	`, userID, today).Scan(&total)
	return total, err
}

// GetTotalWithdrawn returns total gems withdrawn by user (all time) - legacy
func (r *WithdrawalRepository) GetTotalWithdrawn(ctx context.Context, userID int64) (int64, error) {
	var total int64
	err := r.db.QueryRow(ctx, `
		SELECT COALESCE(SUM(gems_amount), 0)
		FROM ton_withdrawals
		WHERE user_id = $1 AND status IN ('sent', 'completed')
	`, userID).Scan(&total)
	return total, err
}

// HasPendingWithdrawal checks if user has a pending withdrawal
func (r *WithdrawalRepository) HasPendingWithdrawal(ctx context.Context, userID int64) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx, `
		SELECT EXISTS(SELECT 1 FROM ton_withdrawals WHERE user_id = $1 AND status IN ('pending', 'processing'))
	`, userID).Scan(&exists)
	return exists, err
}

func scanWithdrawal(row pgx.Row) (*domain.Withdrawal, error) {
	var w domain.Withdrawal
	var txHash, adminNotes *string
	var txLt *int64
	var processedAt, completedAt *time.Time

	if err := row.Scan(
		&w.ID, &w.UserID, &w.WalletAddress, &w.CoinsAmount, &w.TonAmountNano, &w.FeeCoins, &w.ExchangeRate,
		&w.Status, &txHash, &txLt, &adminNotes, &w.CreatedAt, &processedAt, &completedAt,
		&w.GemsAmount, &w.FeeGems,
	); err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	if txHash != nil {
		w.TxHash = *txHash
	}
	if txLt != nil {
		w.TxLt = *txLt
	}
	if adminNotes != nil {
		w.AdminNotes = *adminNotes
	}
	w.ProcessedAt = processedAt
	w.CompletedAt = completedAt

	return &w, nil
}

func scanWithdrawals(rows pgx.Rows) ([]domain.Withdrawal, error) {
	var withdrawals []domain.Withdrawal

	for rows.Next() {
		var w domain.Withdrawal
		var txHash, adminNotes *string
		var txLt *int64
		var processedAt, completedAt *time.Time

		if err := rows.Scan(
			&w.ID, &w.UserID, &w.WalletAddress, &w.CoinsAmount, &w.TonAmountNano, &w.FeeCoins, &w.ExchangeRate,
			&w.Status, &txHash, &txLt, &adminNotes, &w.CreatedAt, &processedAt, &completedAt,
			&w.GemsAmount, &w.FeeGems,
		); err != nil {
			return nil, err
		}

		if txHash != nil {
			w.TxHash = *txHash
		}
		if txLt != nil {
			w.TxLt = *txLt
		}
		if adminNotes != nil {
			w.AdminNotes = *adminNotes
		}
		w.ProcessedAt = processedAt
		w.CompletedAt = completedAt

		withdrawals = append(withdrawals, w)
	}

	return withdrawals, nil
}
