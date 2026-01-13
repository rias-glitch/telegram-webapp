package repository

import (
	"context"
	"encoding/json"

	"telegram_webapp/internal/domain"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// AuditRepository handles audit log database operations
type AuditRepository struct {
	db *pgxpool.Pool
}

// NewAuditRepository creates a new audit repository
func NewAuditRepository(db *pgxpool.Pool) *AuditRepository {
	return &AuditRepository{db: db}
}

// Create inserts a new audit log entry
func (r *AuditRepository) Create(ctx context.Context, log *domain.AuditLog) error {
	detailsJSON, err := json.Marshal(log.Details)
	if err != nil {
		detailsJSON = []byte("{}")
	}

	_, err = r.db.Exec(ctx, `
		INSERT INTO audit_logs (user_id, action, category, details, ip, user_agent)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, log.UserID, log.Action, log.Category, detailsJSON, log.IP, log.UserAgent)
	return err
}

// CreateWithTx inserts a new audit log entry within a transaction
func (r *AuditRepository) CreateWithTx(ctx context.Context, tx pgx.Tx, log *domain.AuditLog) error {
	detailsJSON, err := json.Marshal(log.Details)
	if err != nil {
		detailsJSON = []byte("{}")
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO audit_logs (user_id, action, category, details, ip, user_agent)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, log.UserID, log.Action, log.Category, detailsJSON, log.IP, log.UserAgent)
	return err
}

// GetByUserID returns audit logs for a user
func (r *AuditRepository) GetByUserID(ctx context.Context, userID int64, limit int) ([]*domain.AuditLog, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, user_id, action, category, details, ip, user_agent, created_at
		FROM audit_logs
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2
	`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanAuditLogs(rows)
}

// GetByCategory returns audit logs by category
func (r *AuditRepository) GetByCategory(ctx context.Context, category string, limit int) ([]*domain.AuditLog, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, user_id, action, category, details, ip, user_agent, created_at
		FROM audit_logs
		WHERE category = $1
		ORDER BY created_at DESC
		LIMIT $2
	`, category, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanAuditLogs(rows)
}

// GetByAction returns audit logs by action
func (r *AuditRepository) GetByAction(ctx context.Context, action string, limit int) ([]*domain.AuditLog, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, user_id, action, category, details, ip, user_agent, created_at
		FROM audit_logs
		WHERE action = $1
		ORDER BY created_at DESC
		LIMIT $2
	`, action, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanAuditLogs(rows)
}

// GetRecent returns the most recent audit logs
func (r *AuditRepository) GetRecent(ctx context.Context, limit int) ([]*domain.AuditLog, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, user_id, action, category, details, ip, user_agent, created_at
		FROM audit_logs
		ORDER BY created_at DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanAuditLogs(rows)
}

func scanAuditLogs(rows pgx.Rows) ([]*domain.AuditLog, error) {
	var logs []*domain.AuditLog
	for rows.Next() {
		var log domain.AuditLog
		var detailsJSON []byte
		if err := rows.Scan(&log.ID, &log.UserID, &log.Action, &log.Category, &detailsJSON, &log.IP, &log.UserAgent, &log.CreatedAt); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(detailsJSON, &log.Details); err != nil {
			log.Details = make(map[string]interface{})
		}
		logs = append(logs, &log)
	}
	return logs, rows.Err()
}
