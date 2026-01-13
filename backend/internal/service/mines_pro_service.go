package service

import (
	"context"
	"errors"
	"sync"
	"time"

	"telegram_webapp/internal/game"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// MinesProService manages active Mines Pro games
type MinesProService struct {
	db          *pgxpool.Pool
	activeGames map[int64]*game.MinesPvEGame // userID -> game
	mu          sync.RWMutex
}

// NewMinesProService creates a new Mines Pro service
func NewMinesProService(db *pgxpool.Pool) *MinesProService {
	s := &MinesProService{
		db:          db,
		activeGames: make(map[int64]*game.MinesPvEGame),
	}

	// Start cleanup goroutine for expired games
	go s.cleanupExpiredGames()

	return s
}

// StartGame starts a new Mines Pro game
func (s *MinesProService) StartGame(ctx context.Context, userID int64, bet int64, minesCount int) (*game.MinesPvEGame, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if user already has an active game
	if existing, ok := s.activeGames[userID]; ok && existing.IsActive() {
		return nil, errors.New("you already have an active game")
	}

	// Start transaction
	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Check and deduct balance
	var balance int64
	if err := tx.QueryRow(ctx, `SELECT gems FROM users WHERE id=$1 FOR UPDATE`, userID).Scan(&balance); err != nil {
		return nil, err
	}
	if balance < bet {
		return nil, errors.New("insufficient balance")
	}

	if _, err := tx.Exec(ctx, `UPDATE users SET gems = gems - $1 WHERE id=$2`, bet, userID); err != nil {
		return nil, err
	}

	// Create game
	gameID := uuid.New().String()[:8]
	g, err := game.NewMinesPvEGame(gameID, userID, bet, minesCount)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	s.activeGames[userID] = g
	return g, nil
}

// GetActiveGame returns user's active game
func (s *MinesProService) GetActiveGame(userID int64) *game.MinesPvEGame {
	s.mu.RLock()
	defer s.mu.RUnlock()

	g, ok := s.activeGames[userID]
	if !ok || !g.IsActive() {
		return nil
	}
	return g
}

// RevealCell reveals a cell in user's active game
func (s *MinesProService) RevealCell(ctx context.Context, userID int64, cell int) (hitMine bool, g *game.MinesPvEGame, err error) {
	s.mu.Lock()
	g, ok := s.activeGames[userID]
	if !ok || !g.IsActive() {
		s.mu.Unlock()
		return false, nil, errors.New("no active game")
	}
	s.mu.Unlock()

	hitMine, err = g.Reveal(cell)
	if err != nil {
		return false, g, err
	}

	// If game is over (exploded or all revealed), clean up
	if !g.IsActive() {
		s.mu.Lock()
		delete(s.activeGames, userID)
		s.mu.Unlock()

		// If auto-cashed out (revealed all), credit winnings
		if g.Status == game.MinesProStatusCashedOut {
			_, _ = s.db.Exec(ctx, `UPDATE users SET gems = gems + $1 WHERE id=$2`, g.WinAmount, userID)
		}
	}

	return hitMine, g, nil
}

// CashOut cashes out user's active game
func (s *MinesProService) CashOut(ctx context.Context, userID int64) (*game.MinesPvEGame, error) {
	s.mu.Lock()
	g, ok := s.activeGames[userID]
	if !ok || !g.IsActive() {
		s.mu.Unlock()
		return nil, errors.New("no active game")
	}
	s.mu.Unlock()

	winAmount, err := g.CashOut()
	if err != nil {
		return g, err
	}

	// Credit winnings
	if _, err := s.db.Exec(ctx, `UPDATE users SET gems = gems + $1 WHERE id=$2`, winAmount, userID); err != nil {
		return g, err
	}

	// Clean up
	s.mu.Lock()
	delete(s.activeGames, userID)
	s.mu.Unlock()

	return g, nil
}

// cleanupExpiredGames removes games older than 1 hour
func (s *MinesProService) cleanupExpiredGames() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		s.mu.Lock()
		now := time.Now()
		for userID, g := range s.activeGames {
			// Games older than 1 hour are considered abandoned
			if now.Sub(g.CreatedAt) > time.Hour {
				delete(s.activeGames, userID)
			}
		}
		s.mu.Unlock()
	}
}

// GetActiveGamesCount returns the number of active games
func (s *MinesProService) GetActiveGamesCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.activeGames)
}
