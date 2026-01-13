package service

import (
	"context"
	"fmt"
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
	TotalUsers       int64   `json:"total_users"`
	ActiveUsersToday int64   `json:"active_users_today"`
	ActiveUsersWeek  int64   `json:"active_users_week"`
	TotalGamesPlayed int64   `json:"total_games_played"`
	GamesToday       int64   `json:"games_today"`
	TotalGems        int64   `json:"total_gems"`        // Total gems in circulation
	TotalWagered     int64   `json:"total_wagered"`     // All-time wagered
	WageredToday     int64   `json:"wagered_today"`     // Today's wagered
	HouseProfit      int64   `json:"house_profit"`      // House profit (wagered - won)
	ProfitToday      int64   `json:"profit_today"`
	PendingWithdraws int     `json:"pending_withdraws"` // Pending withdrawal requests
	TotalDeposited   int64   `json:"total_deposited"`   // Total TON deposited (in gems)
	TotalWithdrawn   int64   `json:"total_withdrawn"`   // Total withdrawn (in gems)
}

// GetStats returns platform statistics
func (s *AdminService) GetStats(ctx context.Context) (*Stats, error) {
	stats := &Stats{}
	today := time.Now().Truncate(24 * time.Hour)
	weekAgo := today.Add(-7 * 24 * time.Hour)

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

	// Total wagered (all time)
	_ = s.db.QueryRow(ctx, `
		SELECT COALESCE(SUM(bet_amount), 0) FROM game_history
	`).Scan(&stats.TotalWagered)

	// Wagered today
	_ = s.db.QueryRow(ctx, `
		SELECT COALESCE(SUM(bet_amount), 0) FROM game_history WHERE created_at >= $1
	`, today).Scan(&stats.WageredToday)

	// House profit (wagered - won)
	var totalWon int64
	_ = s.db.QueryRow(ctx, `
		SELECT COALESCE(SUM(CASE WHEN win_amount > 0 THEN win_amount ELSE 0 END), 0) FROM game_history
	`).Scan(&totalWon)
	stats.HouseProfit = stats.TotalWagered - totalWon

	// Profit today
	var wonToday int64
	_ = s.db.QueryRow(ctx, `
		SELECT COALESCE(SUM(CASE WHEN win_amount > 0 THEN win_amount ELSE 0 END), 0)
		FROM game_history WHERE created_at >= $1
	`, today).Scan(&wonToday)
	stats.ProfitToday = stats.WageredToday - wonToday

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

	return stats, nil
}

// UserInfo represents user information for admin
type UserInfo struct {
	ID        int64     `json:"id"`
	TgID      int64     `json:"tg_id"`
	Username  string    `json:"username"`
	FirstName string    `json:"first_name"`
	Gems      int64     `json:"gems"`
	CreatedAt time.Time `json:"created_at"`
	GamesPlayed int64   `json:"games_played"`
	TotalWon    int64   `json:"total_won"`
	TotalLost   int64   `json:"total_lost"`
}

// GetUser returns user info by ID or telegram ID
func (s *AdminService) GetUser(ctx context.Context, identifier string) (*UserInfo, error) {
	var user UserInfo

	// Try to find by ID first, then by tg_id, then by username
	err := s.db.QueryRow(ctx, `
		SELECT id, tg_id, username, first_name, gems, created_at
		FROM users
		WHERE id::text = $1 OR tg_id::text = $1 OR LOWER(username) = LOWER($1)
	`, identifier).Scan(&user.ID, &user.TgID, &user.Username, &user.FirstName, &user.Gems, &user.CreatedAt)

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
	rows, err := s.db.Query(ctx, `SELECT tg_id FROM users WHERE gems >= 0`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			continue
		}
		ids = append(ids, id)
	}
	return ids, nil
}
