package service

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// AdminService provides admin statistics and operations
type AdminService struct {
	db *pgxpool.Pool
}

// NewAdminService creates a new admin service
func NewAdminService(db *pgxpool.Pool) *AdminService {
	return &AdminService{db: db}
}

// Stats represents platform statistics
type Stats struct {
	TotalUsers       int64 `json:"total_users"`
	ActiveUsersToday int64 `json:"active_users_today"`
	ActiveUsersWeek  int64 `json:"active_users_week"`
	TotalGamesPlayed int64 `json:"total_games_played"`
	GamesToday       int64 `json:"games_today"`
	TotalGems        int64 `json:"total_gems"`        // Total gems in circulation
	TotalCoins       int64 `json:"total_coins"`       // Total coins in circulation
	TotalWagered     int64 `json:"total_wagered"`     // All-time wagered
	WageredToday     int64 `json:"wagered_today"`     // Today's wagered
	PendingWithdraws int   `json:"pending_withdraws"` // Pending withdrawal requests
	TotalDeposited   int64 `json:"total_deposited"`   // Total TON deposited (in gems)
	TotalWithdrawn   int64 `json:"total_withdrawn"`   // Total withdrawn (in gems)
	// Coins purchased stats
	CoinsPurchasedToday int64 `json:"coins_purchased_today"`
	CoinsPurchasedWeek  int64 `json:"coins_purchased_week"`
	CoinsPurchasedMonth int64 `json:"coins_purchased_month"`
	CoinsPurchasedTotal int64 `json:"coins_purchased_total"`
}

// GetStats returns platform statistics
func (s *AdminService) GetStats(ctx context.Context) (*Stats, error) {
	stats := &Stats{}
	today := time.Now().Truncate(24 * time.Hour)
	weekAgo := today.Add(-7 * 24 * time.Hour)
	monthAgo := today.Add(-30 * 24 * time.Hour)

	// Total users
	_ = s.db.QueryRow(ctx, `SELECT COUNT(*) FROM users`).Scan(&stats.TotalUsers)

	// Active users today (played at least one game)
	_ = s.db.QueryRow(ctx, `
		SELECT COUNT(DISTINCT user_id) FROM game_history WHERE created_at >= $1
	`, today).Scan(&stats.ActiveUsersToday)

	// Active users this week
	_ = s.db.QueryRow(ctx, `
		SELECT COUNT(DISTINCT user_id) FROM game_history WHERE created_at >= $1
	`, weekAgo).Scan(&stats.ActiveUsersWeek)

	// Total games played
	_ = s.db.QueryRow(ctx, `SELECT COUNT(*) FROM game_history`).Scan(&stats.TotalGamesPlayed)

	// Games today
	_ = s.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM game_history WHERE created_at >= $1
	`, today).Scan(&stats.GamesToday)

	// Total gems in circulation
	_ = s.db.QueryRow(ctx, `SELECT COALESCE(SUM(gems), 0) FROM users`).Scan(&stats.TotalGems)

	// Total coins in circulation
	_ = s.db.QueryRow(ctx, `SELECT COALESCE(SUM(coins), 0) FROM users`).Scan(&stats.TotalCoins)

	// Total wagered (all time)
	_ = s.db.QueryRow(ctx, `
		SELECT COALESCE(SUM(bet_amount), 0) FROM game_history
	`).Scan(&stats.TotalWagered)

	// Wagered today
	_ = s.db.QueryRow(ctx, `
		SELECT COALESCE(SUM(bet_amount), 0) FROM game_history WHERE created_at >= $1
	`, today).Scan(&stats.WageredToday)

	// Pending withdrawals
	_ = s.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM withdrawals WHERE status IN ('pending', 'processing')
	`).Scan(&stats.PendingWithdraws)

	// Total deposited
	_ = s.db.QueryRow(ctx, `
		SELECT COALESCE(SUM(gems_credited), 0) FROM deposits WHERE status = 'confirmed'
	`).Scan(&stats.TotalDeposited)

	// Total withdrawn
	_ = s.db.QueryRow(ctx, `
		SELECT COALESCE(SUM(gems_amount), 0) FROM withdrawals WHERE status IN ('sent', 'completed')
	`).Scan(&stats.TotalWithdrawn)

	// Coins purchased stats (from deposits table)
	_ = s.db.QueryRow(ctx, `
		SELECT COALESCE(SUM(coins_credited), 0) FROM deposits WHERE status = 'confirmed' AND created_at >= $1
	`, today).Scan(&stats.CoinsPurchasedToday)

	_ = s.db.QueryRow(ctx, `
		SELECT COALESCE(SUM(coins_credited), 0) FROM deposits WHERE status = 'confirmed' AND created_at >= $1
	`, weekAgo).Scan(&stats.CoinsPurchasedWeek)

	_ = s.db.QueryRow(ctx, `
		SELECT COALESCE(SUM(coins_credited), 0) FROM deposits WHERE status = 'confirmed' AND created_at >= $1
	`, monthAgo).Scan(&stats.CoinsPurchasedMonth)

	_ = s.db.QueryRow(ctx, `
		SELECT COALESCE(SUM(coins_credited), 0) FROM deposits WHERE status = 'confirmed'
	`).Scan(&stats.CoinsPurchasedTotal)

	return stats, nil
}

// UserInfo represents user information for admin
type UserInfo struct {
	ID          int64     `json:"id"`
	TgID        int64     `json:"tg_id"`
	Username    string    `json:"username"`
	FirstName   string    `json:"first_name"`
	Gems        int64     `json:"gems"`
	Coins       int64     `json:"coins"`
	CreatedAt   time.Time `json:"created_at"`
	GamesPlayed int64     `json:"games_played"`
	TotalWon    int64     `json:"total_won"`
	TotalLost   int64     `json:"total_lost"`
}

// GetUser returns user info by ID or telegram ID
func (s *AdminService) GetUser(ctx context.Context, identifier string) (*UserInfo, error) {
	var user UserInfo

	// Try to find by ID first, then by tg_id, then by username
	err := s.db.QueryRow(ctx, `
		SELECT id, tg_id, username, first_name, gems, coins, created_at
		FROM users
		WHERE id::text = $1 OR tg_id::text = $1 OR LOWER(username) = LOWER($1)
	`, identifier).Scan(&user.ID, &user.TgID, &user.Username, &user.FirstName, &user.Gems, &user.Coins, &user.CreatedAt)

	if err != nil {
		return nil, err
	}

	// Get game stats
	_ = s.db.QueryRow(ctx, `SELECT COUNT(*) FROM game_history WHERE user_id = $1`, user.ID).Scan(&user.GamesPlayed)
	_ = s.db.QueryRow(ctx, `
		SELECT COALESCE(SUM(win_amount), 0) FROM game_history WHERE user_id = $1 AND win_amount > 0
	`, user.ID).Scan(&user.TotalWon)
	_ = s.db.QueryRow(ctx, `
		SELECT COALESCE(SUM(ABS(win_amount)), 0) FROM game_history WHERE user_id = $1 AND win_amount < 0
	`, user.ID).Scan(&user.TotalLost)

	return &user, nil
}

// SetUserGems sets user's gems balance
func (s *AdminService) SetUserGems(ctx context.Context, userID int64, gems int64) error {
	_, err := s.db.Exec(ctx, `UPDATE users SET gems = $1 WHERE id = $2`, gems, userID)
	return err
}

// AddUserGems adds gems to user's balance
func (s *AdminService) AddUserGems(ctx context.Context, userID int64, amount int64) (int64, error) {
	var newBalance int64
	err := s.db.QueryRow(ctx, `
		UPDATE users SET gems = gems + $1 WHERE id = $2 RETURNING gems
	`, amount, userID).Scan(&newBalance)
	return newBalance, err
}

// BanUser bans a user (sets gems to -1 as a marker)
func (s *AdminService) BanUser(ctx context.Context, userID int64) error {
	_, err := s.db.Exec(ctx, `UPDATE users SET gems = -1 WHERE id = $1`, userID)
	return err
}

// UnbanUser unbans a user
func (s *AdminService) UnbanUser(ctx context.Context, userID int64) error {
	_, err := s.db.Exec(ctx, `UPDATE users SET gems = 0 WHERE id = $1 AND gems = -1`, userID)
	return err
}

// GetTopUsers returns top users by gems
func (s *AdminService) GetTopUsers(ctx context.Context, limit int) ([]UserInfo, error) {
	rows, err := s.db.Query(ctx, `
		SELECT id, tg_id, username, first_name, gems, created_at
		FROM users
		WHERE gems >= 0
		ORDER BY gems DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []UserInfo
	for rows.Next() {
		var u UserInfo
		if err := rows.Scan(&u.ID, &u.TgID, &u.Username, &u.FirstName, &u.Gems, &u.CreatedAt); err != nil {
			continue
		}
		users = append(users, u)
	}
	return users, nil
}

// GetRecentGames returns recent games
func (s *AdminService) GetRecentGames(ctx context.Context, limit int) ([]map[string]interface{}, error) {
	rows, err := s.db.Query(ctx, `
		SELECT gh.id, gh.user_id, u.username, gh.game_type, gh.mode, gh.result,
		       gh.bet_amount, gh.win_amount, gh.created_at
		FROM game_history gh
		JOIN users u ON u.id = gh.user_id
		ORDER BY gh.created_at DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var games []map[string]interface{}
	for rows.Next() {
		var id, userID, betAmount, winAmount int64
		var username, gameType, mode, result string
		var createdAt time.Time

		if err := rows.Scan(&id, &userID, &username, &gameType, &mode, &result, &betAmount, &winAmount, &createdAt); err != nil {
			continue
		}

		games = append(games, map[string]interface{}{
			"id":         id,
			"user_id":    userID,
			"username":   username,
			"game_type":  gameType,
			"mode":       mode,
			"result":     result,
			"bet_amount": betAmount,
			"win_amount": winAmount,
			"created_at": createdAt,
		})
	}
	return games, nil
}

// PendingWithdrawal represents a pending withdrawal
type PendingWithdrawal struct {
	ID            int64     `json:"id"`
	UserID        int64     `json:"user_id"`
	Username      string    `json:"username"`
	WalletAddress string    `json:"wallet_address"`
	GemsAmount    int64     `json:"gems_amount"`
	TonAmount     string    `json:"ton_amount"`
	Status        string    `json:"status"`
	CreatedAt     time.Time `json:"created_at"`
}

// GetPendingWithdrawals returns pending withdrawal requests
func (s *AdminService) GetPendingWithdrawals(ctx context.Context) ([]PendingWithdrawal, error) {
	rows, err := s.db.Query(ctx, `
		SELECT w.id, w.user_id, u.username, w.wallet_address, w.gems_amount,
		       w.ton_amount_nano, w.status, w.created_at
		FROM withdrawals w
		JOIN users u ON u.id = w.user_id
		WHERE w.status IN ('pending', 'processing')
		ORDER BY w.created_at ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var withdrawals []PendingWithdrawal
	for rows.Next() {
		var w PendingWithdrawal
		var tonNano int64
		if err := rows.Scan(&w.ID, &w.UserID, &w.Username, &w.WalletAddress,
			&w.GemsAmount, &tonNano, &w.Status, &w.CreatedAt); err != nil {
			continue
		}
		w.TonAmount = fmt.Sprintf("%.4f TON", float64(tonNano)/1e9)
		withdrawals = append(withdrawals, w)
	}
	return withdrawals, nil
}

// ApproveWithdrawal marks withdrawal as sent (after manual sending)
func (s *AdminService) ApproveWithdrawal(ctx context.Context, id int64, txHash string) error {
	_, err := s.db.Exec(ctx, `
		UPDATE withdrawals
		SET status = 'sent', tx_hash = $2, processed_at = NOW()
		WHERE id = $1 AND status IN ('pending', 'processing')
	`, id, txHash)
	return err
}

// RejectWithdrawal rejects a withdrawal and refunds gems
func (s *AdminService) RejectWithdrawal(ctx context.Context, id int64, reason string) error {
	// Start transaction
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// Get withdrawal info
	var userID, gemsAmount int64
	err = tx.QueryRow(ctx, `
		SELECT user_id, gems_amount FROM withdrawals WHERE id = $1 AND status = 'pending'
	`, id).Scan(&userID, &gemsAmount)
	if err != nil {
		return err
	}

	// Refund gems
	_, err = tx.Exec(ctx, `UPDATE users SET gems = gems + $1 WHERE id = $2`, gemsAmount, userID)
	if err != nil {
		return err
	}

	// Update withdrawal status
	_, err = tx.Exec(ctx, `
		UPDATE withdrawals SET status = 'cancelled', admin_notes = $2 WHERE id = $1
	`, id, reason)
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// Broadcast sends a message to all users (returns count)
// Note: This just stores the message - actual sending happens via bot
func (s *AdminService) GetAllUserTgIDs(ctx context.Context) ([]int64, error) {
	query := `SELECT tg_id FROM users WHERE tg_id IS NOT NULL ORDER BY id`
	rows, err := s.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	defer rows.Close()

	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan failed: %w", err)
		}
		ids = append(ids, id)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration failed: %w", err)
	}

	return ids, nil
}

// GameRecord represents a single game record
type GameRecord struct {
	ID        int64     `json:"id"`
	GameType  string    `json:"game_type"`
	Mode      string    `json:"mode"`
	Result    string    `json:"result"`
	BetAmount int64     `json:"bet_amount"`
	WinAmount int64     `json:"win_amount"`
	Currency  string    `json:"currency"`
	CreatedAt time.Time `json:"created_at"`
}

// GetUserGamesByTgID returns last games for user by telegram ID
func (s *AdminService) GetUserGamesByTgID(ctx context.Context, tgID int64, currency string, limit int) ([]GameRecord, error) {
	rows, err := s.db.Query(ctx, `
		SELECT gh.id, gh.game_type, gh.mode, gh.result, gh.bet_amount, gh.win_amount,
		       COALESCE(gh.currency, 'gems') as currency, gh.created_at
		FROM game_history gh
		JOIN users u ON u.id = gh.user_id
		WHERE u.tg_id = $1 AND COALESCE(gh.currency, 'gems') = $2
		ORDER BY gh.created_at DESC
		LIMIT $3
	`, tgID, currency, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var games []GameRecord
	for rows.Next() {
		var g GameRecord
		if err := rows.Scan(&g.ID, &g.GameType, &g.Mode, &g.Result, &g.BetAmount, &g.WinAmount, &g.Currency, &g.CreatedAt); err != nil {
			continue
		}
		games = append(games, g)
	}
	return games, nil
}

// UserGameStats represents user wins statistics
type UserGameStats struct {
	UserID    int64  `json:"user_id"`
	TgID      int64  `json:"tg_id"`
	Username  string `json:"username"`
	GemsWins  int64  `json:"gems_wins"`
	CoinsWins int64  `json:"coins_wins"`
}

// GetTopUsersByWins returns users sorted by wins
func (s *AdminService) GetTopUsersByWins(ctx context.Context, limit int) ([]UserGameStats, error) {
	rows, err := s.db.Query(ctx, `
		SELECT
			u.id,
			u.tg_id,
			COALESCE(u.username, u.first_name) as username,
			COALESCE(SUM(CASE WHEN gh.result = 'win' AND COALESCE(gh.currency, 'gems') = 'gems' THEN 1 ELSE 0 END), 0) as gems_wins,
			COALESCE(SUM(CASE WHEN gh.result = 'win' AND gh.currency = 'coins' THEN 1 ELSE 0 END), 0) as coins_wins
		FROM users u
		LEFT JOIN game_history gh ON gh.user_id = u.id
		GROUP BY u.id, u.tg_id, u.username, u.first_name
		HAVING COALESCE(SUM(CASE WHEN gh.result = 'win' THEN 1 ELSE 0 END), 0) > 0
		ORDER BY (COALESCE(SUM(CASE WHEN gh.result = 'win' AND COALESCE(gh.currency, 'gems') = 'gems' THEN 1 ELSE 0 END), 0) +
		          COALESCE(SUM(CASE WHEN gh.result = 'win' AND gh.currency = 'coins' THEN 1 ELSE 0 END), 0)) DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stats []UserGameStats
	for rows.Next() {
		var s UserGameStats
		if err := rows.Scan(&s.UserID, &s.TgID, &s.Username, &s.GemsWins, &s.CoinsWins); err != nil {
			continue
		}
		stats = append(stats, s)
	}
	return stats, nil
}

// AddUserCoins adds coins to user's balance by tg_id
func (s *AdminService) AddUserCoins(ctx context.Context, tgID int64, amount int64) (int64, error) {
	var newBalance int64
	err := s.db.QueryRow(ctx, `
		UPDATE users SET coins = coins + $1 WHERE tg_id = $2 RETURNING coins
	`, amount, tgID).Scan(&newBalance)
	return newBalance, err
}

// GetUserByTgID returns user info by telegram ID
func (s *AdminService) GetUserByTgID(ctx context.Context, tgID int64) (*UserInfo, error) {
	var user UserInfo
	err := s.db.QueryRow(ctx, `
		SELECT id, tg_id, username, first_name, gems, created_at
		FROM users
		WHERE tg_id = $1
	`, tgID).Scan(&user.ID, &user.TgID, &user.Username, &user.FirstName, &user.Gems, &user.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// ReferralStat represents referral statistics for a user
type ReferralStat struct {
	UserID    int64  `json:"user_id"`
	Username  string `json:"username"`
	FirstName string `json:"first_name"`
	Count     int    `json:"count"`
}

// GetReferralStats returns users with their referral counts
func (s *AdminService) GetReferralStats(ctx context.Context, limit int) ([]ReferralStat, error) {
	rows, err := s.db.Query(ctx, `
		SELECT u.id, COALESCE(u.username, ''), COALESCE(u.first_name, ''), COUNT(r.id) as ref_count
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

	var stats []ReferralStat
	for rows.Next() {
		var s ReferralStat
		if err := rows.Scan(&s.UserID, &s.Username, &s.FirstName, &s.Count); err != nil {
			continue
		}
		stats = append(stats, s)
	}
	return stats, nil
}

// UserListItem represents a user in the users list
type UserListItem struct {
	ID        int64  `json:"id"`
	TgID      int64  `json:"tg_id"`
	Username  string `json:"username"`
	FirstName string `json:"first_name"`
	Gems      int64  `json:"gems"`
	Coins     int64  `json:"coins"`
}

// GetAllUsers returns all users with pagination
func (s *AdminService) GetAllUsers(ctx context.Context, limit, offset int) ([]UserListItem, int, error) {
	var total int
	_ = s.db.QueryRow(ctx, `SELECT COUNT(*) FROM users`).Scan(&total)

	rows, err := s.db.Query(ctx, `
		SELECT id, tg_id, COALESCE(username, ''), COALESCE(first_name, ''), gems, COALESCE(coins, 0)
		FROM users
		ORDER BY id DESC
		LIMIT $1 OFFSET $2
	`, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var users []UserListItem
	for rows.Next() {
		var u UserListItem
		if err := rows.Scan(&u.ID, &u.TgID, &u.Username, &u.FirstName, &u.Gems, &u.Coins); err != nil {
			continue
		}
		users = append(users, u)
	}
	return users, total, nil
}

// GetUserByUsername returns user by username (without @)
func (s *AdminService) GetUserByUsername(ctx context.Context, username string) (*UserInfo, error) {
	// Remove @ if present
	username = strings.TrimPrefix(username, "@")

	var user UserInfo
	err := s.db.QueryRow(ctx, `
		SELECT id, tg_id, COALESCE(username, ''), COALESCE(first_name, ''), gems, COALESCE(coins, 0), created_at
		FROM users
		WHERE LOWER(username) = LOWER($1)
	`, username).Scan(&user.ID, &user.TgID, &user.Username, &user.FirstName, &user.Gems, &user.Coins, &user.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// ResolveUserIdentifier resolves @username or tg_id to internal user ID
func (s *AdminService) ResolveUserIdentifier(ctx context.Context, identifier string) (int64, error) {
	// Remove @ if present
	identifier = strings.TrimPrefix(identifier, "@")

	var userID int64

	// First try to parse as number (tg_id)
	if tgID, err := strconv.ParseInt(identifier, 10, 64); err == nil {
		err = s.db.QueryRow(ctx, `SELECT id FROM users WHERE tg_id = $1`, tgID).Scan(&userID)
		if err == nil {
			return userID, nil
		}
	}

	// Try as username
	err := s.db.QueryRow(ctx, `SELECT id FROM users WHERE LOWER(username) = LOWER($1)`, identifier).Scan(&userID)
	return userID, err
}

// GetWithdrawalInfo returns withdrawal details for notification
type WithdrawalNotification struct {
	ID            int64
	UserID        int64
	Username      string
	TgID          int64
	WalletAddress string
	CoinsAmount   int64
	TonAmount     float64
}

// GetWithdrawalNotification returns withdrawal info for admin notification
func (s *AdminService) GetWithdrawalNotification(ctx context.Context, withdrawalID int64) (*WithdrawalNotification, error) {
	var w WithdrawalNotification
	var tonNano int64
	err := s.db.QueryRow(ctx, `
		SELECT w.id, w.user_id, COALESCE(u.username, u.first_name, ''), u.tg_id,
		       w.wallet_address, w.coins_amount, w.ton_amount_nano
		FROM withdrawals w
		JOIN users u ON u.id = w.user_id
		WHERE w.id = $1
	`, withdrawalID).Scan(&w.ID, &w.UserID, &w.Username, &w.TgID, &w.WalletAddress, &w.CoinsAmount, &tonNano)
	if err != nil {
		return nil, err
	}
	w.TonAmount = float64(tonNano) / 1e9
	return &w, nil
}
