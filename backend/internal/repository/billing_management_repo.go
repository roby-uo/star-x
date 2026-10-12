package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

const billingManagementSettingKey = "billing_management_config_v1"

type billingManagementRepository struct{ db *sql.DB }

func NewBillingManagementRepository(db *sql.DB) service.BillingManagementRepository {
	return &billingManagementRepository{db: db}
}

func (r *billingManagementRepository) Get(ctx context.Context) (string, error) {
	var raw string
	err := r.db.QueryRowContext(ctx, `SELECT value FROM settings WHERE key = $1`, billingManagementSettingKey).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return raw, err
}

// Initial inserts and subsequent updates both reject racing writes. The JSON
// value is intentionally stored as text, so exact equality is a CAS token.
func (r *billingManagementRepository) CompareAndSwap(ctx context.Context, expected, next string) (bool, error) {
	var result sql.Result
	var err error
	if expected == "" {
		result, err = r.db.ExecContext(ctx, `INSERT INTO settings (key, value, updated_at) VALUES ($1, $2, NOW()) ON CONFLICT (key) DO NOTHING`, billingManagementSettingKey, next)
	} else {
		result, err = r.db.ExecContext(ctx, `UPDATE settings SET value = $1, updated_at = NOW() WHERE key = $2 AND value = $3`, next, billingManagementSettingKey, expected)
	}
	if err != nil {
		return false, err
	}
	rows, err := result.RowsAffected()
	return rows == 1, err
}
