package service

import (
	"fmt"
	"strings"

	"github.com/shopspring/decimal"
)

// FixedUSDToCNYRate is the site's accounting display conversion. Existing
// balances, holds and gateway charges continue to use the internal USD ledger.
const FixedUSDToCNYRate = 6.2

const paymentLedgerPrecision = 12

func paymentAmountToLedger(amount float64, currency string) float64 {
	value := decimal.NewFromFloat(amount)
	if strings.EqualFold(strings.TrimSpace(currency), "CNY") || strings.EqualFold(strings.TrimSpace(currency), "RMB") {
		value = value.Div(decimal.NewFromFloat(FixedUSDToCNYRate))
	}
	return value.Round(paymentLedgerPrecision).InexactFloat64()
}

func paymentLedgerAmountCNY(amount float64) float64 {
	return decimal.NewFromFloat(amount).Mul(decimal.NewFromFloat(FixedUSDToCNYRate)).Round(2).InexactFloat64()
}

func paymentCashAmountCNY(amount float64, currency string) (float64, error) {
	switch strings.ToUpper(strings.TrimSpace(currency)) {
	case "CNY", "RMB":
		return amount, nil
	case "USD":
		return paymentLedgerAmountCNY(amount), nil
	default:
		return 0, fmt.Errorf("no configured CNY conversion for payment currency %s", currency)
	}
}
