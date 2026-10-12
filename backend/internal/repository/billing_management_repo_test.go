package repository

import (
	"context"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestBillingCostBookCASRejectsConcurrentInitialInsertAndUpdate(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := NewBillingManagementRepository(db)
	query := regexp.QuoteMeta(`INSERT INTO settings (key, value, updated_at) VALUES ($1, $2, NOW()) ON CONFLICT (key) DO NOTHING`)
	mock.ExpectExec(query).WithArgs(billingManagementSettingKey, `{"version":1}`).WillReturnResult(sqlmock.NewResult(0, 0))
	ok, err := repo.CompareAndSwap(context.Background(), "", `{"version":1}`)
	require.NoError(t, err)
	require.False(t, ok)
	query = regexp.QuoteMeta(`UPDATE settings SET value = $1, updated_at = NOW() WHERE key = $2 AND value = $3`)
	mock.ExpectExec(query).WithArgs(`{"version":2}`, billingManagementSettingKey, `{"version":1}`).WillReturnResult(sqlmock.NewResult(0, 0))
	ok, err = repo.CompareAndSwap(context.Background(), `{"version":1}`, `{"version":2}`)
	require.NoError(t, err)
	require.False(t, ok)
	mock.ExpectExec(query).WithArgs(`{"version":2}`, billingManagementSettingKey, `{"version":1}`).WillReturnResult(sqlmock.NewResult(0, 1))
	ok, err = repo.CompareAndSwap(context.Background(), `{"version":1}`, `{"version":2}`)
	require.NoError(t, err)
	require.True(t, ok)
	require.NoError(t, mock.ExpectationsWereMet())
}
