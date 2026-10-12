package service

import (
	"fmt"
	"math"

	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/shopspring/decimal"
)

const defaultBalanceRechargeMultiplier = 1.0

func normalizeBalanceRechargeMultiplier(multiplier float64) float64 {
	if math.IsNaN(multiplier) || math.IsInf(multiplier, 0) || multiplier <= 0 {
		return defaultBalanceRechargeMultiplier
	}
	return multiplier
}

// normalizeSubscriptionUSDToCNYRate 将非法值归一为 0（换算关闭）。
// 与余额倍率不同，0 是合法状态：表示订阅保持 price 直付的存量行为。
func normalizeSubscriptionUSDToCNYRate(rate float64) float64 {
	if math.IsNaN(rate) || math.IsInf(rate, 0) || rate < 0 {
		return 0
	}
	return rate
}

func calculateCreditedBalance(paymentAmount, multiplier float64) float64 {
	return decimal.NewFromFloat(paymentAmount).
		Mul(decimal.NewFromFloat(normalizeBalanceRechargeMultiplier(multiplier))).
		Round(paymentLedgerPrecision).
		InexactFloat64()
}

// The legacy multiplier measures ledger units per payment unit. In simple mode
// the separately configured multiplier is only a gift ratio; CNY is first
// converted to the existing USD ledger and USD is already in that ledger.
func calculateCreditedBalanceForCurrency(paymentAmount float64, currency string, cfg *PaymentConfig) (float64, error) {
	if !cfg.SimpleBillingEnabled {
		return calculateCreditedBalance(paymentAmount, cfg.BalanceRechargeMultiplier), nil
	}
	if currency != "CNY" && currency != "USD" {
		return 0, fmt.Errorf("simple billing does not support recharge currency %s", currency)
	}
	value := decimal.NewFromFloat(paymentAmount).
		Mul(decimal.NewFromFloat(normalizeBalanceRechargeMultiplier(cfg.RechargeBonusMultiplier)))
	if currency == "CNY" {
		value = value.Div(decimal.NewFromFloat(FixedUSDToCNYRate))
	}
	return value.Round(paymentLedgerPrecision).InexactFloat64(), nil
}

func calculateGatewayRefundAmount(orderAmount, payAmount, refundAmount float64, currency string) float64 {
	if orderAmount <= 0 || payAmount <= 0 || refundAmount <= 0 {
		return 0
	}
	fractionDigits := int32(payment.CurrencyMaxFractionDigits(currency))
	if math.Abs(refundAmount-orderAmount) <= math.Pow10(-paymentLedgerPrecision)/2 {
		return decimal.NewFromFloat(payAmount).Round(fractionDigits).InexactFloat64()
	}
	return decimal.NewFromFloat(payAmount).
		Mul(decimal.NewFromFloat(refundAmount)).
		Div(decimal.NewFromFloat(orderAmount)).
		Round(fractionDigits).
		InexactFloat64()
}
