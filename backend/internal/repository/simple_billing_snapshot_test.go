//go:build unit

package repository

import (
	"context"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestSimpleBillingPriceSnapshotAndBalanceAreOneTransaction(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(map[bool]string{false: "success", true: "snapshot insert failure rolls back without deduction"}[fail], func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer func() { _ = db.Close() }()
			ctx := context.Background()
			mock.ExpectBegin()
			tx, err := db.BeginTx(ctx, nil)
			require.NoError(t, err)
			expect := mock.ExpectExec("INSERT INTO usage_billing_price_snapshots").WithArgs("frozen-price", int64(22), int64(11), int64(7), sqlmock.AnyArg())
			if fail {
				expect.WillReturnError(errors.New("snapshot storage unavailable"))
				mock.ExpectRollback()
			} else {
				expect.WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectQuery(conditionalBalanceDeductSQL).WithArgs(.2, int64(11)).WillReturnRows(sqlmock.NewRows([]string{"balance"}).AddRow(9.8))
				mock.ExpectCommit()
			}
			result := &service.UsageBillingApplyResult{Applied: true}
			err = (&usageBillingRepository{}).applyUsageBillingEffects(ctx, tx, &service.UsageBillingCommand{RequestID: "frozen-price", APIKeyID: 22, UserID: 11, BalanceCost: .2, PriceSnapshot: &service.SimpleBillingSnapshot{Version: 7, Source: "manual", Hash: "frozen-v7"}}, result)
			if fail {
				require.Error(t, err)
				require.NoError(t, tx.Rollback())
				require.Nil(t, result.NewBalance)
			} else {
				require.NoError(t, err)
				require.NoError(t, tx.Commit())
				require.InDelta(t, 9.8, *result.NewBalance, 1e-12)
			}
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
