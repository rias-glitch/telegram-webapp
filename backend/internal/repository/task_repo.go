package repository

import (
	"context"

	"telegram_webapp/internal/domain"

	"github.com/jackc/pgx/v5/pgxpool"
)

type TaskRepository struct{
    db *pgxpool.Pool
}

func NewTaskRepository(db *pgxpool.Pool) *TaskRepository {
    return &TaskRepository{db: db}
}

func (r *TaskRepository) List(ctx context.Context) ([]*domain.Task, error) {
    rows, err := r.db.Query(ctx, `SELECT id, user_id, title, description, completed, created_at FROM tasks ORDER BY created_at DESC LIMIT 100`)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var res []*domain.Task
    for rows.Next() {
        var t domain.Task
        if err := rows.Scan(&t.ID, &t.UserID, &t.Title, &t.Description, &t.Completed, &t.CreatedAt); err != nil {
            return nil, err
        }
        res = append(res, &t)
    }
    return res, nil
}

func (r *TaskRepository) Create(ctx context.Context, t *domain.Task) error {
    return r.db.QueryRow(ctx, `INSERT INTO tasks (user_id, title, description, completed) VALUES ($1,$2,$3,$4) RETURNING id, created_at`, t.UserID, t.Title, t.Description, t.Completed).Scan(&t.ID, &t.CreatedAt)
}

func (r *TaskRepository) SetCompleted(ctx context.Context, id int64, completed bool) error {
    _, err := r.db.Exec(ctx, `UPDATE tasks SET completed = $1 WHERE id = $2`, completed, id)
    return err
}
