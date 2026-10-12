//go:build unit

package service

import (
	"context"
	"strconv"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/stretchr/testify/require"
)

func TestSimplePaymentConfigUsesSharedSwitchAndSeparateGiftRatio(t *testing.T) {
	for _, tc := range []struct {
		name, raw string
		enabled   bool
	}{
		{"new config", "", true},
		{"existing cost book", `{"version":3,"usd_to_cny_rate":7.2}`, true},
		{"explicit legacy", `{"simple_billing_enabled":false}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &paymentConfigSettingRepoStub{values: map[string]string{
				"billing_management_config_v1": tc.raw,
				SettingBalanceRechargeMult:     "10",
			}}
			cfg, err := (&PaymentConfigService{settingRepo: repo}).GetPaymentConfig(context.Background())
			require.NoError(t, err)
			require.Equal(t, tc.enabled, cfg.SimpleBillingEnabled)
			require.Equal(t, 1.0, cfg.RechargeBonusMultiplier)
			require.Equal(t, 10.0, cfg.BalanceRechargeMultiplier)
			require.Equal(t, FixedUSDToCNYRate, cfg.USDToCNYRate)
		})
	}
	_, err := (&PaymentConfigService{settingRepo: &paymentConfigSettingRepoStub{values: map[string]string{
		"billing_management_config_v1": `{broken`,
	}}}).GetPaymentConfig(context.Background())
	require.ErrorContains(t, err, "invalid persisted billing management config")
}

func TestSimpleRechargeCurrencyAndSmallestCNYUnit(t *testing.T) {
	for _, tc := range []struct {
		name, currency                     string
		amount, bonus, wantLedger, wantCNY float64
	}{
		{"CNY", "CNY", 100, 1, 16.129032258065, 100},
		{"one fen", "CNY", 0.01, 1, 0.001612903226, 0.01},
		{"USD", "USD", 100, 1, 100, 620},
		{"gift", "CNY", 100, 1.2, 19.354838709677, 120},
	} {
		t.Run(tc.name, func(t *testing.T) {
			amount, err := calculateCreditedBalanceForCurrency(tc.amount, tc.currency, &PaymentConfig{
				SimpleBillingEnabled: true, BalanceRechargeMultiplier: 10, RechargeBonusMultiplier: tc.bonus,
			})
			require.NoError(t, err)
			require.InDelta(t, tc.wantLedger, amount, 1e-10)
			require.Equal(t, tc.wantCNY, paymentLedgerAmountCNY(amount))
		})
	}
	legacy, err := calculateCreditedBalanceForCurrency(100, "CNY", &PaymentConfig{BalanceRechargeMultiplier: 1})
	require.NoError(t, err)
	require.Equal(t, 100.0, legacy)
	_, err = calculateCreditedBalanceForCurrency(100, "EUR", &PaymentConfig{SimpleBillingEnabled: true})
	require.Error(t, err)
}

func TestSimpleGiftRatioPatchPreservesPaymentConfiguration(t *testing.T) {
	ctx := context.Background()
	repo := &paymentConfigSettingRepoStub{values: map[string]string{
		SettingPaymentEnabled: "true", SettingEnabledPaymentTypes: "alipay,wxpay", SettingBalanceRechargeMult: "10", SettingSubscriptionUSDToCNYRate: "7.15",
	}}
	svc := &PaymentConfigService{settingRepo: repo}
	bonus := 1.2
	require.NoError(t, svc.UpdatePaymentConfig(ctx, UpdatePaymentConfigRequest{RechargeBonusMultiplier: &bonus}))
	require.Equal(t, map[string]string{SettingRechargeBonusMultiplier: "1.2"}, repo.updates)
	cfg, err := svc.GetPaymentConfig(ctx)
	require.NoError(t, err)
	require.True(t, cfg.Enabled)
	require.Equal(t, []string{"alipay", "wxpay"}, cfg.EnabledTypes)
	require.Equal(t, 10.0, cfg.BalanceRechargeMultiplier)
	require.Equal(t, 7.15, cfg.SubscriptionUSDToCNYRate)
	require.Equal(t, 1.2, cfg.RechargeBonusMultiplier)
	disabled, zero, empty := false, 0.0, ""
	require.NoError(t, svc.UpdatePaymentConfig(ctx, UpdatePaymentConfigRequest{Enabled: &disabled, RechargeFeeRate: &zero, HelpText: &empty, EnabledTypes: []string{}}))
	require.Equal(t, "false", repo.values[SettingPaymentEnabled])
	require.Equal(t, "", repo.values[SettingEnabledPaymentTypes])
	require.Equal(t, "0.00", repo.values[SettingRechargeFeeRate])
}

func TestSimpleSubscriptionCurrencyDoesNotDoubleConvert(t *testing.T) {
	cash := 100.0
	for _, tc := range []struct {
		name, planCurrency, paymentCurrency string
		rate, wantBase, wantLedger          float64
		cashPrice                           *float64
	}{
		{"new CNY cash ignores old conversion", "CNY", "CNY", 6.2, 100, 16.129032258065, &cash},
		{"new cash ignores USD display label", "USD", "CNY", 7.2, 100, 16.129032258065, &cash},
		{"new CNY cash to USD", "USD", "USD", 7.2, 16.13, 16.13, &cash},
		{"old USD label CNY unchanged", "USD", "CNY", 0, 100, 16.129032258065, nil},
		{"old CNY label conversion preserved", "CNY", "CNY", 7.15, 715, 115.322580645161, nil},
		{"old USD label USD unchanged", "USD", "USD", 7.15, 100, 100, nil},
		{"unlabelled CNY unchanged", "", "CNY", 0, 100, 16.129032258065, nil},
		{"unlabelled existing conversion", "", "CNY", 7.15, 715, 115.322580645161, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ledger, base, err := calculatePaymentOrderAmounts(CreateOrderRequest{OrderType: payment.OrderTypeSubscription}, &dbent.SubscriptionPlan{Price: 100, Currency: tc.planCurrency, CashPriceCny: tc.cashPrice}, &PaymentConfig{SimpleBillingEnabled: true, SubscriptionUSDToCNYRate: tc.rate}, tc.paymentCurrency)
			require.NoError(t, err)
			require.Equal(t, tc.wantBase, base)
			require.InDelta(t, tc.wantLedger, ledger, 1e-10)
		})
	}
	ledger, base, err := calculatePaymentOrderAmounts(CreateOrderRequest{}, &dbent.SubscriptionPlan{Price: 100, Currency: "CNY"}, &PaymentConfig{SubscriptionUSDToCNYRate: 7.15}, "CNY")
	require.NoError(t, err)
	require.Equal(t, 100.0, ledger)
	require.Equal(t, 715.0, base, "explicitly disabled mode retains legacy label-only behavior")
}

func TestSimpleRefundNearFullAmountRemainsPartial(t *testing.T) {
	orderAmount := paymentAmountToLedger(100, "CNY")
	nearFull := paymentAmountToLedger(99.98, "CNY")
	require.Equal(t, 99.98, calculateGatewayRefundAmount(orderAmount, 100, nearFull, "CNY"))
	require.Equal(t, 100.0, calculateGatewayRefundAmount(orderAmount, 100, orderAmount, "CNY"))
	require.Equal(t, 0.01, calculateGatewayRefundAmount(paymentAmountToLedger(0.01, "CNY"), 0.01, paymentAmountToLedger(0.01, "CNY"), "CNY"))
}

func TestSimplePlanCashPriceCRUDKeepsLegacyUneditedPlans(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	svc := &PaymentConfigService{entClient: client}
	legacy, err := svc.CreatePlan(ctx, CreatePlanRequest{GroupID: 1, Name: "Old display label", Price: 100, Currency: "USD", ValidityDays: 30, ValidityUnit: "day"})
	require.NoError(t, err)
	require.Nil(t, legacy.CashPriceCny)
	price, _ := SubscriptionPlanPriceCNY(legacy, &PaymentConfig{SimpleBillingEnabled: true})
	require.NotNil(t, price)
	require.Equal(t, 100.0, *price)
	name := "Only change the name"
	legacy, err = svc.UpdatePlan(ctx, legacy.ID, UpdatePlanRequest{Name: &name})
	require.NoError(t, err)
	require.Nil(t, legacy.CashPriceCny)
	cash, original := 100.0, 120.0
	newPlan, err := svc.CreatePlan(ctx, CreatePlanRequest{GroupID: 1, Name: "Explicit cash price", Price: 999, Currency: "USD", CashPriceCNY: &cash, CashOriginalPriceCNY: &original, ValidityDays: 30, ValidityUnit: "day"})
	require.NoError(t, err)
	newPlan, err = svc.GetPlan(ctx, newPlan.ID)
	require.NoError(t, err)
	price, originalPrice := SubscriptionPlanPriceCNY(newPlan, &PaymentConfig{SimpleBillingEnabled: true, SubscriptionUSDToCNYRate: 7.15})
	require.Equal(t, 100.0, *price)
	require.Equal(t, 120.0, *originalPrice)
	cash = 150
	legacy, err = svc.UpdatePlan(ctx, legacy.ID, UpdatePlanRequest{CashPriceCNY: &cash})
	require.NoError(t, err)
	price, _ = SubscriptionPlanPriceCNY(legacy, &PaymentConfig{SimpleBillingEnabled: true, SubscriptionUSDToCNYRate: 7.15})
	require.Equal(t, 150.0, *price)
	invalid := 0.001
	_, err = svc.UpdatePlan(ctx, legacy.ID, UpdatePlanRequest{CashPriceCNY: &invalid})
	require.Error(t, err)
}

type simplePaymentRedeemRepo struct {
	paymentOrderLifecycleRedeemRepo
}

func (r *simplePaymentRedeemRepo) Create(_ context.Context, code *RedeemCode) error {
	code.ID = int64(len(r.codesByCode) + 1)
	copyCode := *code
	r.codesByCode[code.Code] = &copyCode
	return nil
}

type simplePaymentRefundProvider struct {
	refundProviderTestDouble
	refundAmounts []string
	queryAmounts  []string
}

func (p *simplePaymentRefundProvider) Refund(_ context.Context, req payment.RefundRequest) (*payment.RefundResponse, error) {
	p.refundAmounts = append(p.refundAmounts, req.Amount)
	return &payment.RefundResponse{Status: payment.ProviderStatusSuccess}, nil
}

func (p *simplePaymentRefundProvider) QueryRefund(_ context.Context, req payment.RefundQueryRequest) (*payment.RefundResponse, error) {
	p.queryAmounts = append(p.queryAmounts, req.Amount)
	return &payment.RefundResponse{Status: payment.ProviderStatusSuccess}, nil
}

func TestSimplePaymentPersistFulfillReplayAndRefund(t *testing.T) {
	for _, paymentAmount := range []float64{100, 0.01} {
		t.Run(strconv.FormatFloat(paymentAmount, 'f', 2, 64), func(t *testing.T) {
			ctx := context.Background()
			client := newPaymentConfigServiceTestClient(t)
			user, err := client.User.Create().SetEmail("simple-payment@example.com").SetPasswordHash("hash").SetBalance(100).Save(ctx)
			require.NoError(t, err)
			instance, err := client.PaymentProviderInstance.Create().SetProviderKey(payment.TypeStripe).SetName("Test CNY cash").SetConfig("{}").SetSupportedTypes(payment.TypeStripe).SetEnabled(true).SetRefundEnabled(true).Save(ctx)
			require.NoError(t, err)
			userRepo := &mockUserRepo{getByIDUser: &User{ID: user.ID, Email: user.Email, Balance: 100}}
			var credits, deductions int
			userRepo.updateBalanceFn = func(_ context.Context, _ int64, amount float64) error {
				credits++
				userRepo.getByIDUser.Balance += amount
				return nil
			}
			userRepo.deductBalanceFn = func(_ context.Context, _ int64, amount float64) error {
				deductions++
				userRepo.getByIDUser.Balance -= amount
				return nil
			}
			redeemRepo := &simplePaymentRedeemRepo{paymentOrderLifecycleRedeemRepo{codesByCode: map[string]*RedeemCode{}}}
			svc := &PaymentService{entClient: client, userRepo: userRepo, loadBalancer: &captureLoadBalancer{}, redeemService: NewRedeemService(redeemRepo, userRepo, nil, nil, nil, client, nil, nil)}
			cfg := &PaymentConfig{SimpleBillingEnabled: true, RechargeBonusMultiplier: 1, BalanceRechargeMultiplier: 10, MaxPendingOrders: 3}
			req := CreateOrderRequest{UserID: user.ID, Amount: paymentAmount, OrderType: payment.OrderTypeBalance, PaymentType: payment.TypeStripe}
			ledger, base, err := calculatePaymentOrderAmounts(req, nil, cfg, "CNY")
			require.NoError(t, err)
			_, pay, err := calculateCreateOrderPayAmount(base, 0, "CNY")
			require.NoError(t, err)
			order, err := svc.createOrderInTx(ctx, req, userRepo.getByIDUser, nil, cfg, ledger, base, 0, pay, &payment.InstanceSelection{InstanceID: strconv.FormatInt(instance.ID, 10), ProviderKey: payment.TypeStripe, Config: map[string]string{"currency": "CNY"}})
			require.NoError(t, err)
			require.Equal(t, paymentAmount, order.PayAmount)
			require.InDelta(t, ledger, order.Amount, 1e-10)
			require.Equal(t, "USD", order.ProviderSnapshot["ledger_currency"])
			require.NoError(t, svc.confirmPayment(ctx, order.ID, "test-paid-trade", pay, payment.TypeStripe, nil))
			require.NoError(t, svc.confirmPayment(ctx, order.ID, "test-paid-trade", pay, payment.TypeStripe, nil))
			require.Equal(t, 1, credits)
			require.Equal(t, 620+paymentAmount, paymentLedgerAmountCNY(userRepo.getByIDUser.Balance))
			paid, err := client.PaymentOrder.Get(ctx, order.ID)
			require.NoError(t, err)
			require.Equal(t, OrderStatusCompleted, paid.Status)
			provider := &simplePaymentRefundProvider{}
			restore := replacePaymentProviderFactoryForTest(t, provider)
			defer restore()
			plan, result, err := svc.PrepareRefund(ctx, order.ID, 0, "test full refund", false, true)
			require.NoError(t, err)
			require.Nil(t, result)
			require.Equal(t, paymentAmount, plan.GatewayAmount)
			result, err = svc.ExecuteRefund(ctx, plan)
			require.NoError(t, err)
			require.True(t, result.Success)
			require.Equal(t, 1, deductions)
			require.Equal(t, []string{strconv.FormatFloat(paymentAmount, 'f', 2, 64)}, provider.refundAmounts)
			require.InDelta(t, 100, userRepo.getByIDUser.Balance, 1e-10)
			refunded, err := client.PaymentOrder.Get(ctx, order.ID)
			require.NoError(t, err)
			require.Equal(t, OrderStatusRefunded, refunded.Status)
			require.Equal(t, paymentAmount, refunded.PayAmount, "historical cash is not rewritten")
			require.InDelta(t, ledger, refunded.RefundAmount, 1e-10)
		})
	}
}

func TestSimplePendingRefundQueryUsesProviderCash(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	order := createPendingRefundOrderForTest(t, ctx, client, "simple-pending-refund")
	ledger := paymentAmountToLedger(100, "CNY")
	order, err := client.PaymentOrder.UpdateOneID(order.ID).SetAmount(ledger).SetPayAmount(100).SetRefundAmount(ledger).Save(ctx)
	require.NoError(t, err)
	provider := &simplePaymentRefundProvider{}
	restore := replacePaymentProviderFactoryForTest(t, provider)
	defer restore()
	svc := &PaymentService{entClient: client, loadBalancer: &captureLoadBalancer{}, userRepo: &mockUserRepo{deductBalanceFn: func(context.Context, int64, float64) error { return nil }}}
	result, err := svc.QueryAndFinalizeRefund(ctx, order.ID)
	require.NoError(t, err)
	require.True(t, result.Success)
	require.Equal(t, []string{"100.00"}, provider.queryAmounts)
}

func TestSimplePaymentCashStatsDoNotMixCurrenciesOrRewriteOrders(t *testing.T) {
	paid := time.Now()
	orders := []*dbent.PaymentOrder{
		{PayAmount: 100, PaidAt: &paid, ProviderSnapshot: map[string]any{"currency": "CNY"}},
		{PayAmount: 10, PaidAt: &paid, ProviderSnapshot: map[string]any{"currency": "USD"}},
	}
	normalized, err := paymentOrdersForCNYStats(orders)
	require.NoError(t, err)
	stats := &DashboardStats{}
	computeBasicStats(stats, normalized, paid.Add(-time.Second))
	require.Equal(t, 162.0, stats.TotalAmount)
	require.Equal(t, 10.0, orders[1].PayAmount)
	_, err = paymentOrdersForCNYStats([]*dbent.PaymentOrder{{PayAmount: 10, ProviderSnapshot: map[string]any{"currency": "EUR"}}})
	require.Error(t, err)
}
