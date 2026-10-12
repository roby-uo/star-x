package service

import (
	"context"
	"fmt"
	"math"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/paymentorder"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

func (s *PaymentService) checkDailyLimitCNY(ctx context.Context, tx *dbent.Tx, userID int64, amount, limit float64, currency string) error {
	if limit <= 0 {
		return nil
	}
	current, err := paymentCashAmountCNY(amount, currency)
	if err != nil {
		return err
	}
	orders, err := tx.PaymentOrder.Query().Where(paymentorder.UserIDEQ(userID), paymentorder.StatusIn(OrderStatusPaid, OrderStatusRecharging, OrderStatusCompleted), paymentorder.PaidAtGTE(psStartOfDayUTC(time.Now()))).All(ctx)
	if err != nil {
		return fmt.Errorf("query daily payment usage: %w", err)
	}
	var used float64
	for _, order := range orders {
		paid, conversionErr := paymentCashAmountCNY(order.PayAmount, PaymentOrderCurrency(order))
		if conversionErr != nil {
			return conversionErr
		}
		used += paid
	}
	if used+current > limit {
		return infraerrors.TooManyRequests("DAILY_LIMIT_EXCEEDED", "daily_limit_exceeded").WithMetadata(map[string]string{"remaining": fmt.Sprintf("%.2f", math.Max(0, limit-used)), "currency": "CNY"})
	}
	return nil
}
