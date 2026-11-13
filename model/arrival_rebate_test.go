package model

import (
	"fmt"
	"github.com/go-sql-driver/mysql"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
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
	paymentBefore := *operation_setting.GetPaymentSetting()
	t.Cleanup(func() {
		*operation_setting.GetArrivalSetting() = arrivalBefore
		*operation_setting.GetPaymentSetting() = paymentBefore
	})
	*operation_setting.GetArrivalSetting() = operation_setting.ArrivalSetting{Enabled: true, Mode: "cumulative", Threshold: 100, SourceGroup: "default", TargetGroup: "earned", IncludeAdmin: true}
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

func TestArrivalGroupingDatabaseMatrix(t *testing.T) {
	for _, dialect := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
			arrivalFixture(t, dialect)
			_, invitee := arrivalUsers(t)
			require.NoError(t, DB.Migrator().DropColumn(&User{}, "ArrivalTotal"))
			require.NoError(t, DB.Migrator().DropColumn(&User{}, "EarnedGroup"))
			for range 2 {
				require.NoError(t, DB.AutoMigrate(&User{}, &ArrivalEvent{}))
			}
			cfg := operation_setting.GetArrivalSetting()
			input := ArrivalInput{Source: "online", Reference: "first", UserId: invitee.Id, Quota: 60}
			first := recordArrival(t, input)
			assert.Equal(t, first.Id, recordArrival(t, input).Id)
			cfg.Enabled = false
			input.Reference = "disabled"
			assert.False(t, recordArrival(t, input).GroupCounted)
			cfg.Enabled = true
			input.Reference = "second"
			recordArrival(t, input)
			var user User
			require.NoError(t, DB.First(&user, invitee.Id).Error)
			assert.Equal(t, 120, user.ArrivalTotal)
			assert.Equal(t, "earned", user.Group)
			cfg.IncludeAdmin = false
			recordArrival(t, ArrivalInput{Source: "admin", Reference: "excluded", UserId: invitee.Id, Quota: 200})
			recordArrival(t, ArrivalInput{Source: "subscription", Reference: "external-not-wallet", UserId: invitee.Id, Quota: 200})
			require.NoError(t, DB.First(&user, invitee.Id).Error)
			assert.Equal(t, 120, user.ArrivalTotal)
			cfg.Mode, cfg.TargetGroup, cfg.SourceGroup = "single", "single-earned", ""
			recordArrival(t, ArrivalInput{Source: "redemption", Reference: "small", UserId: invitee.Id, Quota: 99})
			require.NoError(t, DB.First(&user, invitee.Id).Error)
			assert.Equal(t, "earned", user.Group)
			recordArrival(t, ArrivalInput{Source: "redemption", Reference: "large", UserId: invitee.Id, Quota: 100})
			require.NoError(t, DB.First(&user, invitee.Id).Error)
			assert.Equal(t, "single-earned", user.Group)
			require.True(t, DB.Migrator().HasIndex(&ArrivalEvent{}, "idx_arrival_source"))
		})
	}
}
func TestArrivalGroupingSourceHooks(t *testing.T) {
	arrivalFixture(t, "sqlite")
	_, invitee := arrivalUsers(t)
	topup := TopUp{UserId: invitee.Id, Amount: 1, Money: 10, TradeNo: "group-online", PaymentProvider: PaymentProviderEpay, Status: common.TopUpStatusPending}
	require.NoError(t, topup.Insert())
	_, err := RechargeEpay(topup.TradeNo, "", "test")
	require.NoError(t, err)
	_, err = RechargeEpay(topup.TradeNo, "", "test")
	require.NoError(t, err)
	code := Redemption{Key: common.GetUUID(), Quota: 10, Status: common.RedemptionCodeStatusEnabled}
	require.NoError(t, code.Insert())
	_, err = Redeem(code.Key, invitee.Id)
	require.NoError(t, err)
	_, err = AdjustUserQuota(invitee.Id, common.RoleRootUser, "add", 10)
	require.NoError(t, err)
	var events []ArrivalEvent
	require.NoError(t, DB.Order("id").Find(&events).Error)
	require.Len(t, events, 3)
	assert.Equal(t, "online", events[0].Source)
	assert.Equal(t, "redemption", events[1].Source)
	assert.Equal(t, "admin", events[2].Source)
	var user User
	require.NoError(t, DB.First(&user, invitee.Id).Error)
	assert.Equal(t, events[0].Quota+20, user.ArrivalTotal)
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
