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

// GetReferrerID returns the referrer's user ID for a given user
func (r *ReferralRepository) GetReferrerID(ctx context.Context, userID int64) (int64, error) {
	var referrerID int64
	err := r.db.QueryRow(ctx,
		`SELECT referred_by FROM users WHERE id = $1 AND referred_by IS NOT NULL`,
		userID,
	).Scan(&referrerID)
	return referrerID, err
}

// GetClaimedGKRewards returns list of claimed GK reward thresholds for a user
func (r *ReferralRepository) GetClaimedGKRewards(ctx context.Context, userID int64) ([]int, error) {
	rows, err := r.db.Query(ctx,
		`SELECT threshold FROM gk_rewards_claimed WHERE user_id = $1`,
		userID,
	)
	if err != nil {
		return []int{}, nil // Return empty if table doesn't exist yet
	}
	defer rows.Close()

	var thresholds []int
	for rows.Next() {
		var t int
		if err := rows.Scan(&t); err != nil {
			continue
		}
		thresholds = append(thresholds, t)
	}
	return thresholds, nil
}

// IsGKRewardClaimed checks if a GK reward threshold has been claimed
func (r *ReferralRepository) IsGKRewardClaimed(ctx context.Context, userID int64, threshold int) (bool, error) {
	var count int
	err := r.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM gk_rewards_claimed WHERE user_id = $1 AND threshold = $2`,
		userID, threshold,
	).Scan(&count)
	if err != nil {
		return false, nil // Assume not claimed if table doesn't exist
	}
	return count > 0, nil
}

// ClaimGKReward claims a GK reward and adds GK to user balance
func (r *ReferralRepository) ClaimGKReward(ctx context.Context, userID int64, threshold int, reward int64) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// Record the claim
	_, err = tx.Exec(ctx,
		`INSERT INTO gk_rewards_claimed (user_id, threshold, reward, claimed_at)
		 VALUES ($1, $2, $3, NOW())
		 ON CONFLICT (user_id, threshold) DO NOTHING`,
		userID, threshold, reward,
	)
	if err != nil {
		return err
	}

	// Add GK to user
	_, err = tx.Exec(ctx,
		`UPDATE users SET gk = COALESCE(gk, 0) + $1 WHERE id = $2`,
		reward, userID,
	)
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// GetAllReferralStats returns all users with their referral counts (for admin)
func (r *ReferralRepository) GetAllReferralStats(ctx context.Context, limit int) ([]struct {
	UserID    int64  `json:"user_id"`
	Username  string `json:"username"`
	FirstName string `json:"first_name"`
	Count     int    `json:"count"`
}, error) {
	rows, err := r.db.Query(ctx, `
		SELECT u.id, u.username, u.first_name, COUNT(r.id) as ref_count
		FROM users u
		LEFT JOIN referrals r ON r.referrer_id = u.id
		GROUP BY u.id, u.username, u.first_name
		HAVING COUNT(r.id) > 0
		ORDER BY ref_count DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []struct {
		UserID    int64  `json:"user_id"`
		Username  string `json:"username"`
		FirstName string `json:"first_name"`
		Count     int    `json:"count"`
	}

	for rows.Next() {
		var r struct {
			UserID    int64  `json:"user_id"`
			Username  string `json:"username"`
			FirstName string `json:"first_name"`
			Count     int    `json:"count"`
		}
		if err := rows.Scan(&r.UserID, &r.Username, &r.FirstName, &r.Count); err != nil {
			continue
		}
		results = append(results, r)
	}
	return results, nil
}
