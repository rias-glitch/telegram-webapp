package repository

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Referral struct {
	ID          int64     `json:"id"`
	ReferrerID  int64     `json:"referrer_id"`
	ReferredID  int64     `json:"referred_id"`
	BonusClaimed bool     `json:"bonus_claimed"`
	CreatedAt   time.Time `json:"created_at"`
}

type ReferralStats struct {
	TotalReferrals  int   `json:"total_referrals"`
	TotalEarned     int64 `json:"total_earned"`
}

type ReferralRepository struct {
	db *pgxpool.Pool
}

func NewReferralRepository(db *pgxpool.Pool) *ReferralRepository {
	return &ReferralRepository{db: db}
}

// GenerateReferralCode generates a unique referral code
func GenerateReferralCode() string {
	bytes := make([]byte, 6)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

// GetOrCreateReferralCode gets existing or creates new referral code for user
func (r *ReferralRepository) GetOrCreateReferralCode(ctx context.Context, userID int64) (string, error) {
	// First try to get existing code
	var code string
	err := r.db.QueryRow(ctx,
		`SELECT referral_code FROM users WHERE id = $1`,
		userID,
	).Scan(&code)

	if err == nil && code != "" {
		return code, nil
	}

	// Generate new code
	for i := 0; i < 5; i++ { // Try up to 5 times in case of collision
		code = GenerateReferralCode()
		_, err = r.db.Exec(ctx,
			`UPDATE users SET referral_code = $1 WHERE id = $2`,
			code, userID,
		)
		if err == nil {
			return code, nil
		}
	}

	return "", err
}

// GetUserByReferralCode finds user by their referral code
func (r *ReferralRepository) GetUserByReferralCode(ctx context.Context, code string) (int64, error) {
	var userID int64
	err := r.db.QueryRow(ctx,
		`SELECT id FROM users WHERE referral_code = $1`,
		code,
	).Scan(&userID)
	return userID, err
}

// CreateReferral creates a new referral relationship
func (r *ReferralRepository) CreateReferral(ctx context.Context, referrerID, referredID int64) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO referrals (referrer_id, referred_id)
		 VALUES ($1, $2)
		 ON CONFLICT (referred_id) DO NOTHING`,
		referrerID, referredID,
	)
	if err != nil {
		return err
	}

	// Also update the referred_by field in users table
	_, err = r.db.Exec(ctx,
		`UPDATE users SET referred_by = $1 WHERE id = $2 AND referred_by IS NULL`,
		referrerID, referredID,
	)
	return err
}

// GetReferralsByUser returns all referrals made by a user
func (r *ReferralRepository) GetReferralsByUser(ctx context.Context, userID int64) ([]Referral, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, referrer_id, referred_id, bonus_claimed, created_at
		 FROM referrals
		 WHERE referrer_id = $1
		 ORDER BY created_at DESC`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var referrals []Referral
	for rows.Next() {
		var ref Referral
		if err := rows.Scan(&ref.ID, &ref.ReferrerID, &ref.ReferredID, &ref.BonusClaimed, &ref.CreatedAt); err != nil {
			continue
		}
		referrals = append(referrals, ref)
	}

	return referrals, nil
}

// GetReferralStats returns referral statistics for a user
func (r *ReferralRepository) GetReferralStats(ctx context.Context, userID int64) (*ReferralStats, error) {
	stats := &ReferralStats{}

	// Count total referrals
	err := r.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM referrals WHERE referrer_id = $1`,
		userID,
	).Scan(&stats.TotalReferrals)
	if err != nil {
		return nil, err
	}

	// Calculate total earned (500 gems per referral)
	stats.TotalEarned = int64(stats.TotalReferrals) * 500

	return stats, nil
}

// ClaimReferralBonus marks bonus as claimed and gives rewards
func (r *ReferralRepository) ClaimReferralBonus(ctx context.Context, referralID int64, referrerID int64) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// Mark as claimed
	result, err := tx.Exec(ctx,
		`UPDATE referrals SET bonus_claimed = true
		 WHERE id = $1 AND referrer_id = $2 AND bonus_claimed = false`,
		referralID, referrerID,
	)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return nil // Already claimed or not found
	}

	// Add bonus gems to referrer
	_, err = tx.Exec(ctx,
		`UPDATE users SET gems = gems + 500 WHERE id = $1`,
		referrerID,
	)
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// IsReferred checks if a user was referred by someone
func (r *ReferralRepository) IsReferred(ctx context.Context, userID int64) (bool, error) {
	var count int
	err := r.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM referrals WHERE referred_id = $1`,
		userID,
	).Scan(&count)
	return count > 0, err
}
