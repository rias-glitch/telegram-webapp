package domain

import "time"

type Task struct {
    ID          int64     `db:"id"`
    UserID      int64     `db:"user_id"`
    Title       string    `db:"title"`
    Description string    `db:"description"`
    Completed   bool      `db:"completed"`
    CreatedAt   time.Time `db:"created_at"`
}
