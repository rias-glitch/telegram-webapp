package repository

import (
	"context"

	"telegram_webapp/internal/domain"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type WalletRepository struct {
	db *pgxpool.Pool
}

func NewWalletRepository(db *pgxpool.Pool) *WalletRepository {
	return &WalletRepository{db: db}
}

// GetByUserID retrieves wallet by user ID
func (r *WalletRepository) GetByUserID(ctx context.Context, userID int64) (*domain.Wallet, error) {
	row := r.db.QueryRow(ctx, `
		SELECT id, user_id, address, raw_address, linked_at, is_verified, last_proof_timestamp
		FROM wallets
		WHERE user_id = $1
	`, userID)

	var w domain.Wallet
	var rawAddr *string
	var lastProofTs *int64

	if err := row.Scan(
		&w.ID, &w.UserID, &w.Address, &rawAddr, &w.LinkedAt, &w.IsVerified, &lastProofTs,
	); err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	if rawAddr != nil {
		w.RawAddress = *rawAddr
	}
	if lastProofTs != nil {
		w.LastProofTimestamp = *lastProofTs
	}

	return &w, nil
}

// GetByAddress retrieves wallet by TON address
func (r *WalletRepository) GetByAddress(ctx context.Context, address string) (*domain.Wallet, error) {
	row := r.db.QueryRow(ctx, `
		SELECT id, user_id, address, raw_address, linked_at, is_verified, last_proof_timestamp
		FROM wallets
		WHERE address = $1 OR raw_address = $1
	`, address)

	var w domain.Wallet
	var rawAddr *string
	var lastProofTs *int64

	if err := row.Scan(
		&w.ID, &w.UserID, &w.Address, &rawAddr, &w.LinkedAt, &w.IsVerified, &lastProofTs,
	); err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	if rawAddr != nil {
		w.RawAddress = *rawAddr
	}
	if lastProofTs != nil {
		w.LastProofTimestamp = *lastProofTs
	}

	return &w, nil
}

// Create creates a new wallet link
func (r *WalletRepository) Create(ctx context.Context, w *domain.Wallet) error {
	return r.db.QueryRow(ctx, `
		INSERT INTO wallets (user_id, address, raw_address, is_verified, last_proof_timestamp)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, linked_at
	`, w.UserID, w.Address, w.RawAddress, w.IsVerified, w.LastProofTimestamp).Scan(&w.ID, &w.LinkedAt)
}

// Update updates wallet info
func (r *WalletRepository) Update(ctx context.Context, w *domain.Wallet) error {
	_, err := r.db.Exec(ctx, `
		UPDATE wallets
		SET address = $2, raw_address = $3, is_verified = $4, last_proof_timestamp = $5
		WHERE id = $1
	`, w.ID, w.Address, w.RawAddress, w.IsVerified, w.LastProofTimestamp)
	return err
}

// Delete removes wallet link
func (r *WalletRepository) Delete(ctx context.Context, userID int64) error {
	_, err := r.db.Exec(ctx, `DELETE FROM wallets WHERE user_id = $1`, userID)
	return err
}

// SetVerified marks wallet as verified
func (r *WalletRepository) SetVerified(ctx context.Context, userID int64, verified bool) error {
	_, err := r.db.Exec(ctx, `
		UPDATE wallets SET is_verified = $2 WHERE user_id = $1
	`, userID, verified)
	return err
}

// Exists checks if user already has a linked wallet
func (r *WalletRepository) Exists(ctx context.Context, userID int64) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx, `
		SELECT EXISTS(SELECT 1 FROM wallets WHERE user_id = $1)
	`, userID).Scan(&exists)
	return exists, err
}

// AddressExists checks if address is already linked to any user
func (r *WalletRepository) AddressExists(ctx context.Context, address string) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx, `
		SELECT EXISTS(SELECT 1 FROM wallets WHERE address = $1 OR raw_address = $1)
	`, address).Scan(&exists)
	return exists, err
}
