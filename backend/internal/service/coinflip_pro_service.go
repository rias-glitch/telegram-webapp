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

// CoinFlipProService manages active CoinFlip Pro games
type CoinFlipProService struct {
	db          *pgxpool.Pool
	activeGames map[int64]*game.CoinFlipProGame // userID -> game
	mu          sync.RWMutex
}

// NewCoinFlipProService creates a new CoinFlip Pro service
func NewCoinFlipProService(db *pgxpool.Pool) *CoinFlipProService {
	s := &CoinFlipProService{
		db:          db,
		activeGames: make(map[int64]*game.CoinFlipProGame),
	}

	// Start cleanup goroutine for expired games
	go s.cleanupExpiredGames()

	return s
}

// StartGame starts a new CoinFlip Pro game
func (s *CoinFlipProService) StartGame(ctx context.Context, userID int64, bet int64) (*game.CoinFlipProGame, error) {
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
	g, err := game.NewCoinFlipProGame(gameID, userID, bet)
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
func (s *CoinFlipProService) GetActiveGame(userID int64) *game.CoinFlipProGame {
	s.mu.RLock()
	defer s.mu.RUnlock()

	g, ok := s.activeGames[userID]
	if !ok || !g.IsActive() {
		return nil
	}
	return g
}

// Flip performs a coin flip in user's active game
func (s *CoinFlipProService) Flip(ctx context.Context, userID int64) (win bool, g *game.CoinFlipProGame, err error) {
	s.mu.Lock()
	g, ok := s.activeGames[userID]
	if !ok || !g.IsActive() {
		s.mu.Unlock()
		return false, nil, errors.New("no active game")
	}
	s.mu.Unlock()

	win, err = g.Flip()
	if err != nil {
		return false, g, err
	}

	// If game is over, clean up and credit winnings if won
	if !g.IsActive() {
		s.mu.Lock()
		delete(s.activeGames, userID)
		s.mu.Unlock()

		// If won (auto-cashed out at max rounds), credit winnings
		if g.Status == game.CoinFlipProStatusCashedOut {
			_, _ = s.db.Exec(ctx, `UPDATE users SET gems = gems + $1 WHERE id=$2`, g.WinAmount, userID)
		}
	}

	return win, g, nil
}

// CashOut cashes out user's active game
func (s *CoinFlipProService) CashOut(ctx context.Context, userID int64) (*game.CoinFlipProGame, error) {
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
func (s *CoinFlipProService) cleanupExpiredGames() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		s.mu.Lock()
		now := time.Now()
		for userID, g := range s.activeGames {
			if now.Sub(g.CreatedAt) > time.Hour {
				delete(s.activeGames, userID)
			}
		}
		s.mu.Unlock()
	}
}

// GetActiveGamesCount returns the number of active games
func (s *CoinFlipProService) GetActiveGamesCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.activeGames)
}
