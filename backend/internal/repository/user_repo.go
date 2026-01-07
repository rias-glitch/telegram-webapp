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
		`SELECT id, tg_id, username, first_name, gems, created_at
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
		`SELECT id, tg_id, username, first_name, gems, created_at
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
		&u.CreatedAt,
	); err != nil {
		return nil, err
	}

	return &u, nil
}

// GetTopByWins returns users ordered by win count desc
func (r *UserRepository) GetTopByWins(ctx context.Context, limit int) ([]struct{
	User  domain.User `json:"user"`
	Wins  int64       `json:"wins"`
}, error) {
	rows, err := r.db.Query(ctx, `
		SELECT u.id, u.tg_id, u.username, u.first_name, u.created_at, COALESCE(wc.wins,0) as wins
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

	var res []struct{
		User domain.User `json:"user"`
		Wins int64       `json:"wins"`
	}

	for rows.Next() {
		var u domain.User
		var wins int64
		if err := rows.Scan(&u.ID, &u.TgID, &u.Username, &u.FirstName, &u.CreatedAt, &wins); err != nil {
			return nil, err
		}
		res = append(res, struct{
			User domain.User `json:"user"`
			Wins int64       `json:"wins"`
		}{User: u, Wins: wins})
	}
	return res, nil
}
