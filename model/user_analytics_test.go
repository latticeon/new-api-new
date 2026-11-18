package model

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestGetUserAnalyticsIncludesRedemptionsAndDeduplicatesPayers(t *testing.T) {
	appDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	appSQLDB, err := appDB.DB()
	require.NoError(t, err)
	appSQLDB.SetMaxOpenConns(1)
	t.Cleanup(func() { require.NoError(t, appSQLDB.Close()) })

	logDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	logSQLDB, err := logDB.DB()
	require.NoError(t, err)
	logSQLDB.SetMaxOpenConns(1)
	t.Cleanup(func() { require.NoError(t, logSQLDB.Close()) })
	require.NoError(t, appDB.AutoMigrate(&User{}, &TopUp{}, &Redemption{}))
	require.NoError(t, logDB.AutoMigrate(&AuditLog{}))

	previousDB, previousLogDB := DB, LOG_DB
	DB, LOG_DB = appDB, logDB
	t.Cleanup(func() { DB, LOG_DB = previousDB, previousLogDB })

	now := time.Now().Unix()
	day := localDayStart(now, 0) - 86400
	user := User{Username: "analytics-redeem", Password: "x", Status: common.UserStatusEnabled, AffCode: "anaredeem", CreatedAt: day + 100}
	require.NoError(t, appDB.Create(&user).Error)
	require.NoError(t, appDB.Create(&TopUp{
		UserId: user.Id, TradeNo: "analytics-paid-order", Money: 12,
		Status: common.TopUpStatusSuccess, CompleteTime: day + 200,
	}).Error)
	paidPrice, freePrice := 3.5, 0.0
	codes := []Redemption{
		{Key: "analytics-paid-code", Status: common.RedemptionCodeStatusUsed, Quota: 100, UsedUserId: user.Id, RedeemedTime: day + 300, SalePrice: &paidPrice},
		{Key: "analytics-free-code", Status: common.RedemptionCodeStatusUsed, Quota: 50, UsedUserId: user.Id, RedeemedTime: day + 400, SalePrice: &freePrice},
		{Key: "analytics-free-code-unknown", Status: common.RedemptionCodeStatusUsed, Quota: 25, UsedUserId: user.Id, RedeemedTime: day + 500},
	}
	require.NoError(t, appDB.Create(&codes).Error)
	freeUser := User{Username: "analytics-free-only", Password: "x", Status: common.UserStatusEnabled, AffCode: "anafree", CreatedAt: day + 110}
	require.NoError(t, appDB.Create(&freeUser).Error)
	require.NoError(t, appDB.Create(&Redemption{Key: "analytics-free-only-code", Status: common.RedemptionCodeStatusUsed, Quota: 10, UsedUserId: freeUser.Id, RedeemedTime: day + 450}).Error)
	require.NoError(t, logDB.Create([]AuditLog{
		{EventId: "login-success", UserId: user.Id, Category: AuditCategoryLogin, Success: true, CreatedAt: day + 600},
		{EventId: "login-failure-anonymous", Category: AuditCategoryLogin, Success: false, CreatedAt: day + 700},
	}).Error)

	got, err := GetUserAnalytics(day, day+86399, 0)
	require.NoError(t, err)
	require.Len(t, got.Daily, 1)
	require.EqualValues(t, 1, got.TopupOrders)
	require.EqualValues(t, 4, got.Redemptions)
	require.EqualValues(t, 185, got.RedemptionQuota)
	require.Equal(t, 3.5, got.RedemptionRevenueCNY)
	require.EqualValues(t, 1, got.PayingUsers)
	require.EqualValues(t, 4, got.Daily[0].Redemptions)
	require.Equal(t, 12.0, got.Daily[0].RevenueByCurrency[unknownTopupCurrency])
	require.EqualValues(t, 1, got.Daily[0].ActiveUsers)
	require.EqualValues(t, 1, got.Daily[0].FailedLogins)
	require.EqualValues(t, 1, got.FailedLogins)
}
