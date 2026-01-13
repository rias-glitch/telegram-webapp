package service

import (
	"context"

	"telegram_webapp/internal/domain"
	"telegram_webapp/internal/logger"
	"telegram_webapp/internal/repository"

	"github.com/jackc/pgx/v5/pgxpool"
)

// AuditService handles audit logging
type AuditService struct {
	repo *repository.AuditRepository
}

// NewAuditService creates a new audit service
func NewAuditService(db *pgxpool.Pool) *AuditService {
	return &AuditService{
		repo: repository.NewAuditRepository(db),
	}
}

// Log creates a new audit log entry
func (s *AuditService) Log(ctx context.Context, userID int64, action, category string, details map[string]interface{}) {
	log := &domain.AuditLog{
		UserID:   userID,
		Action:   action,
		Category: category,
		Details:  details,
	}

	if err := s.repo.Create(ctx, log); err != nil {
		logger.Error("failed to create audit log", "error", err, "action", action, "user_id", userID)
	}
}

// LogWithRequest creates an audit log with request info (IP, User-Agent)
func (s *AuditService) LogWithRequest(ctx context.Context, userID int64, action, category, ip, userAgent string, details map[string]interface{}) {
	log := &domain.AuditLog{
		UserID:    userID,
		Action:    action,
		Category:  category,
		Details:   details,
		IP:        ip,
		UserAgent: userAgent,
	}

	if err := s.repo.Create(ctx, log); err != nil {
		logger.Error("failed to create audit log", "error", err, "action", action, "user_id", userID)
	}
}

// LogGame logs a game action
func (s *AuditService) LogGame(ctx context.Context, userID int64, gameType string, bet, result int64, win bool, details map[string]interface{}) {
	action := domain.AuditActionGameLose
	if win {
		action = domain.AuditActionGameWin
	}

	if details == nil {
		details = make(map[string]interface{})
	}
	details["game_type"] = gameType
	details["bet"] = bet
	details["result"] = result
	details["win"] = win

	s.Log(ctx, userID, action, domain.AuditCategoryGame, details)
}

// LogDeposit logs a deposit action
func (s *AuditService) LogDeposit(ctx context.Context, userID int64, amount int64, txHash string, details map[string]interface{}) {
	if details == nil {
		details = make(map[string]interface{})
	}
	details["amount"] = amount
	details["tx_hash"] = txHash

	s.Log(ctx, userID, domain.AuditActionDeposit, domain.AuditCategoryPayment, details)
}

// LogWithdrawRequest logs a withdrawal request
func (s *AuditService) LogWithdrawRequest(ctx context.Context, userID int64, amount int64, walletAddress string) {
	details := map[string]interface{}{
		"amount":         amount,
		"wallet_address": walletAddress,
	}

	s.Log(ctx, userID, domain.AuditActionWithdrawRequest, domain.AuditCategoryWithdrawal, details)
}

// LogWithdrawApprove logs a withdrawal approval
func (s *AuditService) LogWithdrawApprove(ctx context.Context, userID, withdrawalID int64, txHash string) {
	details := map[string]interface{}{
		"withdrawal_id": withdrawalID,
		"tx_hash":       txHash,
	}

	s.Log(ctx, userID, domain.AuditActionWithdrawApprove, domain.AuditCategoryWithdrawal, details)
}

// LogWithdrawReject logs a withdrawal rejection
func (s *AuditService) LogWithdrawReject(ctx context.Context, userID, withdrawalID int64, reason string) {
	details := map[string]interface{}{
		"withdrawal_id": withdrawalID,
		"reason":        reason,
	}

	s.Log(ctx, userID, domain.AuditActionWithdrawReject, domain.AuditCategoryWithdrawal, details)
}

// LogAdminAction logs an admin action
func (s *AuditService) LogAdminAction(ctx context.Context, adminID int64, action string, targetUserID int64, details map[string]interface{}) {
	if details == nil {
		details = make(map[string]interface{})
	}
	details["admin_id"] = adminID
	details["target_user_id"] = targetUserID

	s.Log(ctx, targetUserID, action, domain.AuditCategoryAdmin, details)
}

// LogLogin logs a user login
func (s *AuditService) LogLogin(ctx context.Context, userID int64, ip, userAgent string) {
	s.LogWithRequest(ctx, userID, domain.AuditActionLogin, domain.AuditCategoryAuth, ip, userAgent, nil)
}

// LogBalanceChange logs a balance change
func (s *AuditService) LogBalanceChange(ctx context.Context, userID int64, change int64, reason string, details map[string]interface{}) {
	action := domain.AuditActionBalanceCredit
	if change < 0 {
		action = domain.AuditActionBalanceDebit
	}

	if details == nil {
		details = make(map[string]interface{})
	}
	details["change"] = change
	details["reason"] = reason

	s.Log(ctx, userID, action, domain.AuditCategoryBalance, details)
}

// GetUserAuditLogs returns audit logs for a user
func (s *AuditService) GetUserAuditLogs(ctx context.Context, userID int64, limit int) ([]*domain.AuditLog, error) {
	return s.repo.GetByUserID(ctx, userID, limit)
}

// GetRecentLogs returns recent audit logs
func (s *AuditService) GetRecentLogs(ctx context.Context, limit int) ([]*domain.AuditLog, error) {
	return s.repo.GetRecent(ctx, limit)
}

// GetLogsByCategory returns logs by category
func (s *AuditService) GetLogsByCategory(ctx context.Context, category string, limit int) ([]*domain.AuditLog, error) {
	return s.repo.GetByCategory(ctx, category, limit)
}
