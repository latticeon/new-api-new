package model

import (
	"fmt"
	"github.com/go-sql-driver/mysql"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func arrivalFixture(t *testing.T, dialect string) {
	t.Helper()
	previousDB, previousLog := DB, LOG_DB
	previousMain, previousLogType := common.MainDatabaseType(), common.LogDatabaseType()
	previousPath := common.SQLitePath
	dsn := os.Getenv("TEST_MYSQL_DSN")
	databaseType := common.DatabaseTypeMySQL
	if dialect == "postgres" {
		dsn = os.Getenv("TEST_POSTGRES_DSN")
		databaseType = common.DatabaseTypePostgreSQL
	}
	if dialect == "sqlite" {
		dsn = "local"
		databaseType = common.DatabaseTypeSQLite
		common.SQLitePath = filepath.Join(t.TempDir(), "arrival.db")
	}
	if dsn == "" {
		t.Skip("TEST_MYSQL_DSN / TEST_POSTGRES_DSN not configured")
	}
	t.Setenv("ARRIVAL_TEST_DSN", dsn)
	db, _, err := chooseDB("ARRIVAL_TEST_DSN", false)
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	name := fmt.Sprintf("arrival_test_%d", time.Now().UnixNano())
	adminDB := db
	if dialect == "mysql" {
		require.NoError(t, adminDB.Exec("CREATE DATABASE "+name).Error)
		cfg, parseErr := mysql.ParseDSN(dsn)
		require.NoError(t, parseErr)
		cfg.DBName = name
		t.Setenv("ARRIVAL_TEST_DSN", cfg.FormatDSN())
	}
	if dialect == "postgres" {
		require.NoError(t, adminDB.Exec("CREATE SCHEMA "+name).Error)
		uri, parseErr := url.Parse(dsn)
		require.NoError(t, parseErr)
		query := uri.Query()
		query.Set("search_path", name)
		uri.RawQuery = query.Encode()
		t.Setenv("ARRIVAL_TEST_DSN", uri.String())
	}
	if dialect != "sqlite" {
		db, _, err = chooseDB("ARRIVAL_TEST_DSN", false)
		require.NoError(t, err)
		scopedPool, poolErr := db.DB()
		require.NoError(t, poolErr)
		scopedPool.SetMaxOpenConns(4)
		t.Cleanup(func() { _ = scopedPool.Close() })
	}
	DB, LOG_DB = db, db
	common.SetDatabaseTypes(databaseType, databaseType)
	initCol()
	t.Cleanup(func() {
		if dialect == "mysql" {
			adminDB.Exec("DROP DATABASE " + name)
		}
		if dialect == "postgres" {
			adminDB.Exec("DROP SCHEMA " + name + " CASCADE")
		}
		_ = sqlDB.Close()
		DB, LOG_DB = previousDB, previousLog
		common.SQLitePath = previousPath
		common.SetDatabaseTypes(previousMain, previousLogType)
		initCol()
	})
	versionQuery := "SELECT version()"
	if dialect == "sqlite" {
		versionQuery = "SELECT sqlite_version()"
	}
	var version string
	require.NoError(t, db.Raw(versionQuery).Scan(&version).Error)
	t.Logf("%s version %s", dialect, version)
	arrivalBefore := *operation_setting.GetArrivalSetting()
	rebateBefore := *operation_setting.GetRebateSetting()
	paymentBefore := *operation_setting.GetPaymentSetting()
	t.Cleanup(func() {
		*operation_setting.GetArrivalSetting() = arrivalBefore
		*operation_setting.GetRebateSetting() = rebateBefore
		*operation_setting.GetPaymentSetting() = paymentBefore
	})
	*operation_setting.GetArrivalSetting() = operation_setting.ArrivalSetting{Enabled: true, Mode: "cumulative", Threshold: 100, SourceGroup: "default", TargetGroup: "earned", IncludeAdmin: true}
	*operation_setting.GetRebateSetting() = operation_setting.RebateSetting{Enabled: true, Rule: "every", Percent: 10, QuotaPerYuan: 100, MaxPerInvitee: 250, Destination: "wallet", Online: true, Subscription: true, Redemption: true, Admin: true}
	operation_setting.GetPaymentSetting().ComplianceConfirmed = true
	operation_setting.GetPaymentSetting().ComplianceTermsVersion = operation_setting.CurrentComplianceTermsVersion
	require.NoError(t, DB.AutoMigrate(&User{}, &ArrivalEvent{}, &TopUp{}, &Redemption{}, &SubscriptionPlan{}, &SubscriptionOrder{}, &UserSubscription{}, &Log{}))
}
func arrivalUsers(t *testing.T) (User, User) {
	t.Helper()
	inviter := User{Username: "inviter", Password: "unused", AffCode: "inviter", Group: "default", Status: common.UserStatusEnabled}
	require.NoError(t, DB.Create(&inviter).Error)
	invitee := User{Username: "invitee", Password: "unused", AffCode: "invitee", InviterId: inviter.Id, Group: "default", Status: common.UserStatusEnabled}
	require.NoError(t, DB.Create(&invitee).Error)
	return inviter, invitee
}
func recordArrival(t *testing.T, input ArrivalInput) ArrivalEvent {
	t.Helper()
	require.NoError(t, DB.Transaction(func(tx *gorm.DB) error { return RecordArrivalTx(tx, input) }))
	var event ArrivalEvent
	require.NoError(t, DB.Where("source = ? AND reference = ?", input.Source, input.Reference).First(&event).Error)
	return event
}

func TestArrivalRebateDatabaseMatrix(t *testing.T) {
	for _, dialect := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
			arrivalFixture(t, dialect)
			t.Run("fresh_and_upgrade_migration", func(t *testing.T) {
				inviter, invitee := arrivalUsers(t)
				require.NoError(t, DB.Migrator().DropColumn(&User{}, "ArrivalTotal"))
				require.NoError(t, DB.Migrator().DropColumn(&User{}, "EarnedGroup"))

				require.NoError(t, DB.Migrator().DropColumn(&Redemption{}, "SalePrice"))
				require.NoError(t, DB.Migrator().DropColumn(&Redemption{}, "RebateEnabled"))
				require.NoError(t, DB.Migrator().DropColumn(&TopUp{}, "PaymentSnapshot"))
				require.NoError(t, DB.Migrator().DropColumn(&SubscriptionOrder{}, "PaymentSnapshot"))
				for range 2 {
					require.NoError(t, DB.AutoMigrate(&User{}, &ArrivalEvent{}, &TopUp{}, &Redemption{}, &SubscriptionOrder{}))
				}
				var restored User
				require.NoError(t, DB.First(&restored, invitee.Id).Error)
				assert.Equal(t, inviter.Id, restored.InviterId)
				assert.Zero(t, restored.ArrivalTotal)
				require.True(t, DB.Migrator().HasIndex(&ArrivalEvent{}, "idx_arrival_source"))
				require.NoError(t, DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Unscoped().Delete(&User{}).Error)
			})
			inviter, invitee := arrivalUsers(t)
			input := ArrivalInput{Source: "online", Reference: "first", UserId: invitee.Id, Quota: 60, Payment: NewPaymentReceipt(decimal.NewFromInt(10), "CNY", "provider_payment"), Participate: true}
			first := recordArrival(t, input)
			assert.Equal(t, 100, first.Reward)
			assert.Equal(t, "pending", first.Status)
			duplicate := recordArrival(t, input)
			assert.Equal(t, first.Id, duplicate.Id)
			require.NoError(t, PayArrivalRebate(first.Id))
			require.NoError(t, PayArrivalRebate(first.Id))
			var recipient User
			require.NoError(t, DB.First(&recipient, inviter.Id).Error)
			assert.Equal(t, 100, recipient.Quota)
			operation_setting.GetArrivalSetting().Enabled = false
			input.Reference = "disabled"
			recordArrival(t, input)
			operation_setting.GetArrivalSetting().Enabled = true
			input.Reference = "third"
			third := recordArrival(t, input)
			assert.Equal(t, 50, third.Reward)
			var earned User
			require.NoError(t, DB.First(&earned, invitee.Id).Error)
			assert.Equal(t, 120, earned.ArrivalTotal)
			assert.Equal(t, "earned", earned.Group)
			// Reservations include pending rewards and never exceed the lifetime cap.
			var errs [2]error
			var wg sync.WaitGroup
			for i := range 2 {
				wg.Go(func() {
					next := input
					next.Reference = fmt.Sprintf("concurrent-%d", i)
					errs[i] = DB.Transaction(func(tx *gorm.DB) error { return RecordArrivalTx(tx, next) })
				})
			}
			wg.Wait()
			for _, err := range errs {
				require.NoError(t, err)
			}
			var sum int64
			require.NoError(t, DB.Model(&ArrivalEvent{}).Select("SUM(reward)").Scan(&sum).Error)
			assert.Equal(t, int64(250), sum)
			// A full recipient wallet cannot cancel the original successful topup.
			require.NoError(t, DB.Model(&User{}).Where("id = ?", inviter.Id).Update("quota", common.MaxWalletQuota).Error)
			require.Error(t, PayArrivalRebate(third.Id))
			var failed ArrivalEvent
			require.NoError(t, DB.First(&failed, third.Id).Error)
			assert.Equal(t, "pending", failed.Status)
			assert.NotEmpty(t, failed.Reason)
			assert.Greater(t, failed.Attempts, 0)
			operation_setting.GetRebateSetting().Percent = 99
			operation_setting.GetRebateSetting().Destination = "affiliate"
			require.NoError(t, DB.Model(&User{}).Where("id = ?", inviter.Id).Update("quota", 0).Error)
			require.NoError(t, PayArrivalRebate(third.Id))
			require.NoError(t, DB.First(&recipient, inviter.Id).Error)
			assert.Equal(t, 50, recipient.Quota)
			assert.Zero(t, recipient.AffQuota)
		})
	}
}

func TestArrivalRebateGlobalFirstRule(t *testing.T) {
	arrivalFixture(t, "sqlite")
	inviter, invitee := arrivalUsers(t)
	cfg := operation_setting.GetRebateSetting()
	cfg.Enabled, cfg.Rule = false, "first"
	input := ArrivalInput{Source: "online", Reference: "disabled-first", UserId: invitee.Id, Quota: 1, Payment: NewPaymentReceipt(decimal.NewFromInt(10), "CNY", "provider_payment"), Participate: true}
	first := recordArrival(t, input)
	assert.True(t, first.RebateQualified)
	assert.Equal(t, "disabled", first.Reason)
	cfg.Enabled = true
	input.Reference = "enabled-later"
	assert.Equal(t, "not_first_qualified_success", recordArrival(t, input).Reason)
	cfg.Rule, cfg.Percent = "every", 0
	input.Reference = "zero"
	assert.Zero(t, recordArrival(t, input).Reward)
	cfg.Percent = 10
	input.Reference = "wallet"
	event := recordArrival(t, input)
	require.NoError(t, PayArrivalRebate(event.Id))
	cfg.Destination = "affiliate"
	input.Reference = "affiliate"
	event = recordArrival(t, input)
	require.NoError(t, PayArrivalRebate(event.Id))
	var recipient User
	require.NoError(t, DB.First(&recipient, inviter.Id).Error)
	assert.Equal(t, 100, recipient.Quota)
	assert.Equal(t, 100, recipient.AffQuota)
	assert.Equal(t, 200, recipient.AffHistoryQuota)
}

func TestArrivalRebateFourSourceHooks(t *testing.T) {
	for _, dialect := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
			arrivalFixture(t, dialect)
			sqlDB, dbErr := DB.DB()
			require.NoError(t, dbErr)
			sqlDB.SetMaxOpenConns(2)
			inviter, invitee := arrivalUsers(t)
			cfg := operation_setting.GetRebateSetting()
			cfg.MaxPerInvitee = 0
			receipt := NewPaymentReceipt(decimal.NewFromInt(10), "CNY", "provider_payment")
			topup := TopUp{UserId: invitee.Id, Amount: 1, Money: 10, TradeNo: "epay-hook", PaymentProvider: PaymentProviderEpay, Status: common.TopUpStatusPending, PaymentSnapshot: PaymentReceiptJSON(receipt)}
			require.NoError(t, topup.Insert())
			_, err := RechargeEpay(topup.TradeNo, "", "test")
			require.NoError(t, err)
			_, err = RechargeEpay(topup.TradeNo, "", "test")
			require.NoError(t, err)
			price := 10.0
			code := Redemption{Key: common.GetUUID(), Quota: 10, SalePrice: &price, RebateEnabled: true, Status: common.RedemptionCodeStatusEnabled}
			require.NoError(t, code.Insert())
			_, err = Redeem(code.Key, invitee.Id)
			require.NoError(t, err)
			_, err = AdjustUserQuota(invitee.Id, common.RoleRootUser, "add", 10, AdminArrivalOptions{Reference: "admin-hook", ReceivedAmount: &price, RebateEnabled: true})
			require.NoError(t, err)
			_, err = AdjustUserQuota(invitee.Id, common.RoleRootUser, "add", 10, AdminArrivalOptions{Reference: "admin-hook", ReceivedAmount: &price, RebateEnabled: true})
			require.Error(t, err)
			plan := SubscriptionPlan{Title: "plan", PriceAmount: 0.1, DurationUnit: SubscriptionDurationMonth, DurationValue: 1, Enabled: true, TotalAmount: 100}
			require.NoError(t, DB.Create(&plan).Error)
			order := SubscriptionOrder{UserId: invitee.Id, PlanId: plan.Id, Money: 10, TradeNo: "subscription-hook", PaymentProvider: PaymentProviderEpay, Status: common.TopUpStatusPending, PaymentSnapshot: PaymentReceiptJSON(receipt)}
			require.NoError(t, order.Insert())
			require.NoError(t, CompleteSubscriptionOrder(order.TradeNo, "", PaymentProviderEpay, ""))
			require.NoError(t, CompleteSubscriptionOrder(order.TradeNo, "", PaymentProviderEpay, ""))
			require.NoError(t, PurchaseSubscriptionWithBalance(invitee.Id, plan.Id))
			// Provider-success amounts override checkout quotes without changing
			// credited quota, and every provider remains independently idempotent.
			for _, provider := range []string{PaymentProviderStripe, PaymentProviderCreem, PaymentProviderWaffo, PaymentProviderWaffoPancake} {
				order := TopUp{UserId: invitee.Id, Amount: 1, Money: 0.1, TradeNo: "provider-" + provider, PaymentProvider: provider, Status: common.TopUpStatusPending}
				require.NoError(t, order.Insert())
				for range 2 {
					switch provider {
					case PaymentProviderStripe:
						err = Recharge(order.TradeNo, "", "test", receipt)
					case PaymentProviderCreem:
						err = RechargeCreem(order.TradeNo, "", "", "test", receipt)
					case PaymentProviderWaffo:
						err = RechargeWaffo(order.TradeNo, "test", receipt)
					case PaymentProviderWaffoPancake:
						err = RechargeWaffoPancake(order.TradeNo, receipt)
					}
					require.NoError(t, err)
				}
			}
			var events []ArrivalEvent
			require.NoError(t, DB.Order("id").Find(&events).Error)
			require.Len(t, events, 8)
			for _, event := range events {
				assert.Equal(t, "paid", event.Status)
				assert.Equal(t, 100, event.Reward)
			}
			assert.Equal(t, "registered_sale", events[1].AmountEvidence)
			assert.Equal(t, "registered_receipt", events[2].AmountEvidence)
			assert.False(t, events[3].GroupCounted)
			var user User
			require.NoError(t, DB.First(&user, inviter.Id).Error)
			assert.Equal(t, 800, user.Quota)
		})
	}
}

func TestArrivalTemporarySubscriptionPreservesEarnedGroup(t *testing.T) {
	for _, dialect := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
			arrivalFixture(t, dialect)
			_, invitee := arrivalUsers(t)
			require.NoError(t, DB.Model(&User{}).Where("id = ?", invitee.Id).Update("group", "temporary").Error)
			sub := UserSubscription{UserId: invitee.Id, Status: "active", StartTime: common.GetTimestamp() - 10, EndTime: common.GetTimestamp() + 100, UpgradeGroup: "temporary", PrevUserGroup: "default", DowngradeGroup: "default"}
			require.NoError(t, DB.Create(&sub).Error)
			recordArrival(t, ArrivalInput{Source: "redemption", Reference: "earned-during-sub", UserId: invitee.Id, Quota: 100})
			var user User
			require.NoError(t, DB.First(&user, invitee.Id).Error)
			assert.Equal(t, "temporary", user.Group)
			assert.Equal(t, "earned", user.EarnedGroup)
			require.NoError(t, DB.Model(&UserSubscription{}).Where("id = ?", sub.Id).Update("end_time", common.GetTimestamp()-1).Error)
			_, err := ExpireDueSubscriptions(100)
			require.NoError(t, err)
			require.NoError(t, DB.First(&user, invitee.Id).Error)
			assert.Equal(t, "earned", user.Group)
		})
	}
}

func TestArrivalRebateConcurrentMatrix(t *testing.T) {
	for _, dialect := range []string{"mysql", "postgres"} {
		for _, rule := range []string{"every", "first"} {
			t.Run(dialect+"/"+rule, func(t *testing.T) {
				arrivalFixture(t, dialect)
				inviter, invitee := arrivalUsers(t)
				cfg := operation_setting.GetRebateSetting()
				cfg.Rule = rule
				cfg.MaxPerInvitee = 150
				input := ArrivalInput{Source: "online", UserId: invitee.Id, Quota: 60, Payment: NewPaymentReceipt(decimal.NewFromInt(10), "CNY", "provider_payment"), Participate: true}
				var ready sync.WaitGroup
				ready.Add(2)
				release := make(chan struct{})
				errs := make(chan error, 2)
				for i := range 2 {
					go func() {
						errs <- DB.Transaction(func(tx *gorm.DB) error {
							// Establish an old consistent snapshot before either obtains the user lock.
							var count int64
							err := tx.Model(&ArrivalEvent{}).Count(&count).Error
							ready.Done()
							<-release
							if err != nil {
								return err
							}
							next := input
							next.Reference = fmt.Sprintf("cap-%d", i)
							return RecordArrivalTx(tx, next)
						})
					}()
				}
				ready.Wait()
				close(release)
				for range 2 {
					require.NoError(t, <-errs)
				}
				var events []ArrivalEvent
				require.NoError(t, DB.Order("id").Find(&events).Error)
				require.Len(t, events, 2)
				expectedReward := 150
				if rule == "first" {
					expectedReward = 100
				}
				assert.Equal(t, expectedReward, events[0].Reward+events[1].Reward)
				var user User
				require.NoError(t, DB.First(&user, invitee.Id).Error)
				assert.Equal(t, 120, user.ArrivalTotal)
				assert.Equal(t, "earned", user.EarnedGroup)
				var payments sync.WaitGroup
				for range 2 {
					payments.Go(func() { errs <- PayArrivalRebate(events[0].Id) })
				}
				payments.Wait()
				for range 2 {
					require.NoError(t, <-errs)
				}
				var recipient User
				require.NoError(t, DB.First(&recipient, inviter.Id).Error)
				assert.Equal(t, events[0].Reward, recipient.Quota)
			})
		}
	}
}

func TestArrivalSourceEvidenceAndSettings(t *testing.T) {
	arrivalFixture(t, "sqlite")
	_, invitee := arrivalUsers(t)
	operation_setting.GetRebateSetting().Rule = "first"
	input := ArrivalInput{Source: "online", Reference: "unsupported-currency", UserId: invitee.Id, Quota: 40, Participate: true, Payment: NewPaymentReceipt(decimal.NewFromInt(20), "EUR", "provider_payment")}
	first := recordArrival(t, input)
	assert.True(t, first.RebateQualified)
	assert.Equal(t, "20", first.Amount)
	assert.Empty(t, first.AmountCNY)
	assert.Equal(t, "currency_conversion_unavailable", first.Reason)
	input.Reference = "later-convertible"
	input.Payment = NewPaymentReceipt(decimal.NewFromInt(20), "CNY", "provider_payment")
	assert.Equal(t, "not_first_qualified_success", recordArrival(t, input).Reason)
	// A manual completion may grant actual wallet arrival but never turns an
	// unverified checkout quote into a provider payment or rebate entitlement.
	topup := TopUp{UserId: invitee.Id, Amount: 1, Money: 20, TradeNo: "manual-quote", PaymentProvider: PaymentProviderEpay, Status: common.TopUpStatusPending, PaymentSnapshot: PaymentReceiptJSON(input.Payment)}
	require.NoError(t, topup.Insert())
	require.NoError(t, ManualCompleteTopUp(topup.TradeNo, "test"))
	var manual ArrivalEvent
	require.NoError(t, DB.Where("reference = ?", topup.TradeNo).First(&manual).Error)
	assert.Equal(t, "unavailable", manual.AmountEvidence)
	assert.False(t, manual.RebateQualified)
	cfg := operation_setting.GetArrivalSetting()
	cfg.Mode = "single"
	cfg.TargetGroup = "single-earned"
	cfg.SourceGroup = ""
	cfg.IncludeAdmin = false
	before := User{}
	require.NoError(t, DB.First(&before, invitee.Id).Error)
	recordArrival(t, ArrivalInput{Source: "admin", Reference: "not-counted-admin", UserId: invitee.Id, Quota: 200})
	recordArrival(t, ArrivalInput{Source: "redemption", Reference: "below-single", UserId: invitee.Id, Quota: 99})
	var after User
	require.NoError(t, DB.First(&after, invitee.Id).Error)
	assert.Equal(t, before.ArrivalTotal+99, after.ArrivalTotal)
	assert.Equal(t, before.EarnedGroup, after.EarnedGroup)
	recordArrival(t, ArrivalInput{Source: "redemption", Reference: "meets-single", UserId: invitee.Id, Quota: 100})
	require.NoError(t, DB.First(&after, invitee.Id).Error)
	assert.Equal(t, "single-earned", after.EarnedGroup)
	for _, tc := range []struct {
		key, value string
		valid      bool
	}{
		{"arrival_setting.threshold", "-1", false},
		{"arrival_setting.mode", "single", true},
		{"rebate_setting.percent", "NaN", false},
		{"rebate_setting.quota_per_yuan", "+Inf", false},
		{"rebate_setting.percent", "0", true},
		{"rebate_setting.rule", "every", true},
		{"rebate_setting.destination", "wallet", true},
		{"rebate_setting.enabled", "false", true},
		{"rebate_setting.unknown", "true", false},
	} {
		err := operation_setting.ValidateArrivalRebateOption(tc.key, tc.value)
		assert.Equal(t, tc.valid, err == nil, "%s=%s", tc.key, tc.value)
	}
}
