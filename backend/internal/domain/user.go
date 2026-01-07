package domain

import "time"

type User struct {
	ID        int64     `db:"id"`
	TgID      int64     `db:"tg_id"`
	Username  string    `db:"username"`
	FirstName string    `db:"first_name"`
	CreatedAt time.Time `db:"created_at"`
	Gems      int64     `db:"gems" json:"gems"`
}
