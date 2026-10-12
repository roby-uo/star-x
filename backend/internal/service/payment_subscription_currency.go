package service

import (
	"fmt"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/shopspring/decimal"
)

// A historical currency label is never interpreted as a checkout currency.
// Only a newly saved explicit cash price overrides the legacy provider policy.
func subscriptionPaymentBaseAmount(plan *dbent.SubscriptionPlan, cfg *PaymentConfig, paymentCurrency string) (float64, error) {
	if plan.CashPriceCny == nil {
		return calculateSubscriptionGatewayBaseAmount(plan.Price, cfg.SubscriptionUSDToCNYRate, paymentCurrency), nil
	}
	if paymentCurrency != "CNY" && paymentCurrency != "USD" {
		return 0, fmt.Errorf("simple billing does not support subscription payment currency %s", paymentCurrency)
	}
	value := decimal.NewFromFloat(*plan.CashPriceCny)
	if paymentCurrency == "USD" {
		value = value.Div(decimal.NewFromFloat(FixedUSDToCNYRate))
	}
	return value.Round(int32(payment.CurrencyMaxFractionDigits(paymentCurrency))).InexactFloat64(), nil
}

// SubscriptionPlanPriceCNY quotes the current CNY checkout base, excluding any
// recharge fee. It is not a conversion of historical payment records.
func SubscriptionPlanPriceCNY(plan *dbent.SubscriptionPlan, cfg *PaymentConfig) (*float64, *float64) {
	price, err := subscriptionPaymentBaseAmount(plan, cfg, "CNY")
	if err != nil {
		return nil, nil
	}
	var original *float64
	if plan.CashOriginalPriceCny != nil {
		value := *plan.CashOriginalPriceCny
		original = &value
	} else if plan.OriginalPrice != nil && plan.CashPriceCny == nil {
		copyPlan := *plan
		copyPlan.Price = *plan.OriginalPrice
		if value, originalErr := subscriptionPaymentBaseAmount(&copyPlan, cfg, "CNY"); originalErr == nil {
			original = &value
		}
	}
	return &price, original
}

func calculatePaymentOrderAmounts(req CreateOrderRequest, plan *dbent.SubscriptionPlan, cfg *PaymentConfig, currency string) (ledgerAmount, paymentBase float64, err error) {
	if plan == nil {
		ledgerAmount, err = calculateCreditedBalanceForCurrency(req.Amount, currency, cfg)
		return ledgerAmount, req.Amount, err
	}
	paymentBase, err = subscriptionPaymentBaseAmount(plan, cfg, currency)
	if err != nil {
		return 0, 0, err
	}
	if !cfg.SimpleBillingEnabled && plan.CashPriceCny == nil {
		return plan.Price, paymentBase, nil
	}
	if currency != "CNY" && currency != "USD" {
		return 0, 0, fmt.Errorf("simple billing does not support payment currency %s", currency)
	}
	return paymentAmountToLedger(paymentBase, currency), paymentBase, nil
}
