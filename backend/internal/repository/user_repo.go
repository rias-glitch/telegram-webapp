package repository

import (
	"context"

	"telegram_webapp/internal/domain"

	"github.com/jackc/pgx/v5/pgxpool"
)

type UserRepository struct {
	db *pgxpool.Pool
}

func NewUserRepository(db *pgxpool.Pool) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) GetByTgID(ctx context.Context, tgID int64) (*domain.User, error) {
	row := r.db.QueryRow(ctx,
		`SELECT id, tg_id, username, first_name, gems, coins, created_at
		 FROM users
		 WHERE tg_id = $1`,
		tgID,
	)

	var u domain.User
	if err := row.Scan(
		&u.ID,
		&u.TgID,
		&u.Username,
		&u.FirstName,
		&u.Gems,
		&u.Coins,
		&u.CreatedAt,
	); err != nil {
		return nil, err
	}

	return &u, nil
}

func (r *UserRepository) Create(ctx context.Context, u *domain.User) error {
	// Начальный баланс для новых пользователей
	const initialGems = 10000

	return r.db.QueryRow(ctx,
		`INSERT INTO users (tg_id, username, first_name, gems)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id`,
		u.TgID,
		u.Username,
		u.FirstName,
		initialGems,
	).Scan(&u.ID)
}

func (r *UserRepository) GetByID(ctx context.Context, id int64) (*domain.User, error) {
	row := r.db.QueryRow(ctx,
		`SELECT id, tg_id, username, first_name, gems, coins, created_at
		 FROM users
		 WHERE id = $1`,
		id,
	)

	var u domain.User
	if err := row.Scan(
		&u.ID,
		&u.TgID,
		&u.Username,
		&u.FirstName,
		&u.Gems,
		&u.Coins,
		&u.CreatedAt,
	); err != nil {
		return nil, err
	}

	return &u, nil
}

// GetTopByWins returns users ordered by win count desc
func (r *UserRepository) GetTopByWins(ctx context.Context, limit int) ([]struct {
	User domain.User `json:"user"`
	Wins int64       `json:"wins"`
}, error) {
	rows, err := r.db.Query(ctx, `
		SELECT u.id, u.tg_id, u.username, u.first_name, u.gems, u.coins, u.created_at, COALESCE(wc.wins,0) as wins
		FROM users u
		LEFT JOIN (
			SELECT winner_id, COUNT(*) as wins FROM games WHERE winner_id IS NOT NULL GROUP BY winner_id
		) wc ON wc.winner_id = u.id
		ORDER BY wins DESC
		LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var res []struct {
		User domain.User `json:"user"`
		Wins int64       `json:"wins"`
	}

	for rows.Next() {
		var u domain.User
		var wins int64
		if err := rows.Scan(&u.ID, &u.TgID, &u.Username, &u.FirstName, &u.Gems, &u.Coins, &u.CreatedAt, &wins); err != nil {
			return nil, err
		}
		res = append(res, struct {
			User domain.User `json:"user"`
			Wins int64       `json:"wins"`
		}{User: u, Wins: wins})
	}
	return res, nil
}

// UpdateCoins updates user's coins balance
func (r *UserRepository) UpdateCoins(ctx context.Context, userID int64, delta int64) (int64, error) {
	var newBalance int64
	err := r.db.QueryRow(ctx,
		`UPDATE users SET coins = coins + $1 WHERE id = $2 AND coins + $1 >= 0 RETURNING coins`,
		delta, userID,
	).Scan(&newBalance)
	return newBalance, err
}

// GetCoins returns user's coins balance
func (r *UserRepository) GetCoins(ctx context.Context, userID int64) (int64, error) {
	var coins int64
	err := r.db.QueryRow(ctx, `SELECT coins FROM users WHERE id = $1`, userID).Scan(&coins)
	return coins, err
}

// UpdateGems updates user's gems balance
func (r *UserRepository) UpdateGems(ctx context.Context, userID int64, delta int64) (int64, error) {
	var newBalance int64
	err := r.db.QueryRow(ctx,
		`UPDATE users SET gems = gems + $1 WHERE id = $2 AND gems + $1 >= 0 RETURNING gems`,
		delta, userID,
	).Scan(&newBalance)
	return newBalance, err
}

// GetGems returns user's gems balance
func (r *UserRepository) GetGems(ctx context.Context, userID int64) (int64, error) {
	var gems int64
	err := r.db.QueryRow(ctx, `SELECT gems FROM users WHERE id = $1`, userID).Scan(&gems)
	return gems, err
}
