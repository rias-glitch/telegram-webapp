package integration

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"telegram_webapp/internal/domain"
	"telegram_webapp/internal/repository"

	"github.com/jackc/pgx/v5/pgxpool"
)

func applyMigrations(t *testing.T, db *pgxpool.Pool) {
    t.Helper()
    migDir := filepath.Join("..", "..", "internal", "migrations")
    files, err := os.ReadDir(migDir)
    if err != nil {
        t.Fatalf("read migrations: %v", err)
    }
    for _, f := range files {
        b, err := os.ReadFile(filepath.Join(migDir, f.Name()))
        if err != nil {
            t.Fatalf("read file: %v", err)
        }
        if _, err := db.Exec(context.Background(), string(b)); err != nil {
            t.Fatalf("apply migration %s: %v", f.Name(), err)
        }
    }
}

func TestGameRepository_Create_GetByUser(t *testing.T) {
    dsn := os.Getenv("DATABASE_URL")
    if dsn == "" {
        t.Skip("DATABASE_URL not set")
    }

    db, err := pgxpool.New(context.Background(), dsn)
    if err != nil {
        t.Fatalf("connect db: %v", err)
    }
    defer db.Close()

    applyMigrations(t, db)

    repo := repository.NewGameRepository(db)

    g := &domain.Game{
        RoomID: "r1",
        PlayerAID: 1,
        PlayerBID: 2,
        Moves: map[int64]string{1: "rock", 2: "scissors"},
    }

    if err := repo.Create(context.Background(), g); err != nil {
        t.Fatalf("create game: %v", err)
    }

    games, err := repo.GetByUser(context.Background(), 1)
    if err != nil {
        t.Fatalf("get by user: %v", err)
    }
    if len(games) == 0 {
        t.Fatalf("expected games, got 0")
    }
}
