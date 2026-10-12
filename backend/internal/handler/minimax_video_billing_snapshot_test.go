package handler

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestSelectedMiniMaxQuoteRequiresSameChargeAndConfigVersion(t *testing.T) {
	quote := &service.VideoQuote{Currency: "USD", UnitPrice: .5 / 6.2, Multiplier: 1.4, Total: 3.5 / 6.2, SimpleBilling: &service.SimpleBillingSnapshot{Version: 4, AccountID: 1}}
	selected := *quote
	selected.SimpleBilling = &service.SimpleBillingSnapshot{Version: 4, AccountID: 2}
	require.True(t, sameMiniMaxVideoCharge(quote, &selected), "same-priced supplier failover is allowed")
	selected.Total += .01
	require.False(t, sameMiniMaxVideoCharge(quote, &selected), "a changed charge cannot use the old reservation")
	selected.Total = quote.Total
	selected.SimpleBilling.Version++
	require.False(t, sameMiniMaxVideoCharge(quote, &selected), "a changed price configuration requires a fresh public quote")
	selected.SimpleBilling = nil
	require.False(t, sameMiniMaxVideoCharge(quote, &selected), "do not fall back to a legacy quote after a pricing switch")
}
